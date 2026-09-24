"""Subset one master font (static or variable) for one EPUB font target, then verify it.

Order of operations (fast on 30k-65k glyph CJK masters, same result):
1. read master facts (cmap, UVS, vertical substitutions, axes) before anything changes;
2. subset with every OpenType feature kept, so vert/vrt2, locl, palt, ruby,
   FeatureVariations alternates etc. stay reachable;
3. optionally instance: "instance" pins every axis (static font, the handbook
   §4.6 default), "limit" narrows axis ranges (still variable), "keep" leaves
   variations untouched;
4. save with the flavor implied by the target extension and re-read the bytes
   for the checks.
"""

from __future__ import annotations

import hashlib
import io
import logging
import numbers
from dataclasses import dataclass

from fontTools import subset
from fontTools.pens.areaPen import AreaPen
from fontTools.pens.boundsPen import BoundsPen
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

FLAVOR_BY_EXT = {".ttf": None, ".otf": None, ".woff": "woff", ".woff2": "woff2"}
VARIATION_MODES = ("keep", "instance", "limit")
VERTICAL_FEATURES = ("vert", "vrt2")


class FontJobError(Exception):
    """The font job cannot run safely; the whole run must stop (exit 2)."""


@dataclass
class VariationSpec:
    mode: str = "keep"
    axes: dict | None = None


@dataclass
class FontFacts:
    cmap: dict
    uvs: set
    vertical_inputs: set
    axes: list
    outline: str
    glyph_count: int
    tables: list
    flavor: str | None


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def cp_label(cp: int) -> str:
    return f"U+{cp:04X} {chr(cp)}"


def is_variation_selector(cp: int) -> bool:
    return 0xFE00 <= cp <= 0xFE0F or 0xE0100 <= cp <= 0xE01EF or 0x180B <= cp <= 0x180F


def target_extension(target: str) -> str:
    dot = target.rfind(".")
    return target[dot:].lower() if dot > target.rfind("/") else ""


def parse_variation(raw) -> VariationSpec:
    if raw is None:
        return VariationSpec("keep", {})
    if not isinstance(raw, dict):
        raise FontJobError("variation must be an object")
    unknown = set(raw) - {"mode", "axes"}
    if unknown:
        raise FontJobError(f"variation has unknown keys: {sorted(unknown)}")
    mode = raw.get("mode", "keep")
    if mode not in VARIATION_MODES:
        raise FontJobError(f"variation.mode must be one of {VARIATION_MODES}, got {mode!r}")
    axes = raw.get("axes", {})
    if not isinstance(axes, dict):
        raise FontJobError("variation.axes must be an object")
    if mode == "keep" and axes:
        raise FontJobError("variation.mode 'keep' takes no axes")
    return VariationSpec(mode, axes)


def load_font(data: bytes, label: str) -> TTFont:
    if data[:4] == b"ttcf":
        raise FontJobError(f"{label}: font collections (.ttc/.otc) are not supported; extract one font first")
    try:
        font = TTFont(io.BytesIO(data), recalcTimestamp=False)
        font.getGlyphOrder()
    except ImportError as exc:
        raise FontJobError(f"{label}: missing optional module ({exc}); run `uv sync` (fonttools[woff] provides brotli)") from exc
    except Exception as exc:  # fontTools raises many exception types for bad input
        raise FontJobError(f"{label}: cannot parse font: {exc}") from exc
    return font


def outline_kind(font: TTFont) -> str:
    for tag in ("glyf", "CFF2", "CFF "):
        if tag in font:
            return tag.strip()
    return "unknown"


def axes_of(font: TTFont) -> list:
    if "fvar" not in font:
        return []
    return [
        {"tag": a.axisTag, "min": a.minValue, "default": a.defaultValue, "max": a.maxValue}
        for a in font["fvar"].axes
    ]


def uvs_pairs(font: TTFont) -> set:
    pairs = set()
    if "cmap" not in font:
        return pairs
    for table in font["cmap"].tables:
        if table.format == 14:
            for selector, entries in table.uvsDict.items():
                for base, _glyph in entries:
                    pairs.add((base, selector))
    return pairs


def vertical_inputs(font: TTFont) -> set:
    """Glyph names that a vert/vrt2 single substitution replaces."""
    if "GSUB" not in font:
        return set()
    gsub = font["GSUB"].table
    if not gsub.FeatureList or not gsub.LookupList:
        return set()
    indices = set()
    for record in gsub.FeatureList.FeatureRecord:
        if record.FeatureTag in VERTICAL_FEATURES:
            indices.update(record.Feature.LookupListIndex)
    inputs = set()
    for index in indices:
        for sub in gsub.LookupList.Lookup[index].SubTable:
            real = getattr(sub, "ExtSubTable", sub)
            if type(real).__name__ == "SingleSubst":
                inputs.update(real.mapping)
    return inputs


def font_facts(font: TTFont) -> FontFacts:
    return FontFacts(
        cmap=dict(font.getBestCmap() or {}),
        uvs=uvs_pairs(font),
        vertical_inputs=vertical_inputs(font),
        axes=axes_of(font),
        outline=outline_kind(font),
        glyph_count=len(font.getGlyphOrder()),
        tables=sorted(tag for tag in font.keys() if tag != "GlyphOrder"),
        flavor=font.flavor,
    )


def check_target_format(target: str, outline: str) -> None:
    ext = target_extension(target)
    if ext not in FLAVOR_BY_EXT:
        raise FontJobError(f"{target}: unsupported font extension {ext!r} (use .ttf/.otf/.woff/.woff2)")
    if ext == ".ttf" and outline != "glyf":
        raise FontJobError(f"{target}: .ttf target needs TrueType (glyf) outlines, master has {outline}")
    if ext == ".otf" and outline not in ("CFF", "CFF2"):
        raise FontJobError(f"{target}: .otf target needs CFF/CFF2 outlines, master has {outline}")


def _number(value, what: str) -> float:
    if isinstance(value, bool) or not isinstance(value, numbers.Real):
        raise FontJobError(f"{what} must be a number, got {value!r}")
    return float(value)


def axis_limits(facts: FontFacts, spec: VariationSpec, label: str) -> dict | None:
    """Translate the config into fontTools instancer limits (None = keep variations)."""
    if spec.mode == "keep":
        return None
    if not facts.axes:
        raise FontJobError(f"{label}: variation.mode {spec.mode!r} needs a variable font, but the master has no fvar table")
    by_tag = {a["tag"]: a for a in facts.axes}
    unknown = set(spec.axes) - set(by_tag)
    if unknown:
        raise FontJobError(f"{label}: unknown axes {sorted(unknown)}; master axes are {sorted(by_tag)}")
    limits = {}
    for tag, axis in by_tag.items():
        lo, hi = axis["min"], axis["max"]
        raw = spec.axes.get(tag)
        if spec.mode == "instance":
            value = axis["default"] if raw is None else _number(raw, f"axes.{tag}")
            if not lo <= value <= hi:
                raise FontJobError(f"{label}: axes.{tag}={value:g} is outside {lo:g}..{hi:g}")
            limits[tag] = value
            continue
        if raw is None:
            continue
        if isinstance(raw, list) and len(raw) == 2:
            a, b = _number(raw[0], f"axes.{tag}[0]"), _number(raw[1], f"axes.{tag}[1]")
            if not lo <= a <= b <= hi:
                raise FontJobError(f"{label}: axes.{tag}=[{a:g}, {b:g}] must satisfy {lo:g} <= min <= max <= {hi:g}")
            limits[tag] = (a, b)
        else:
            value = _number(raw, f"axes.{tag}")
            if not lo <= value <= hi:
                raise FontJobError(f"{label}: axes.{tag}={value:g} is outside {lo:g}..{hi:g}")
            limits[tag] = value
    if spec.mode == "limit" and not limits:
        raise FontJobError(f"{label}: variation.mode 'limit' needs at least one axis")
    return limits


def target_location(facts: FontFacts, spec: VariationSpec, limits: dict | None) -> dict | None:
    """Design-space location whose outlines the output's default instance must match."""
    if spec.mode == "keep" or not facts.axes:
        return None
    location = {}
    for axis in facts.axes:
        want = limits.get(axis["tag"])
        if want is None:
            location[axis["tag"]] = axis["default"]
        elif isinstance(want, tuple):  # "limit": the new default is the old one clamped into the range
            location[axis["tag"]] = min(max(axis["default"], want[0]), want[1])
        else:
            location[axis["tag"]] = want
    return location


def glyph_shapes(font: TTFont, cmap: dict, codepoints, location: dict | None = None) -> dict:
    """{codepoint: (advance, bounds, area)} drawn at `location` (None = default instance)."""
    glyphset = font.getGlyphSet(location=location, normalized=False) if location else font.getGlyphSet()
    shapes = {}
    for cp in codepoints:
        name = cmap.get(cp)
        if name is None:
            continue
        glyph = glyphset[name]
        bounds = BoundsPen(glyphset)
        glyph.draw(bounds)
        area = AreaPen(glyphset)
        glyph.draw(area)
        shapes[cp] = (glyph.width, bounds.bounds, area.value)
    return shapes


def compare_shapes(master_shapes: dict, out_shapes: dict, units_per_em: int) -> dict:
    """Per glyph: advance within 1 unit, bounds within 1% em, area within 10% (catches a wrong
    glyph or a broken outline). Whole set: total area within 0.5% (catches a wrong axis location:
    rounding errors cancel out, a wrong weight moves every glyph the same way).

    Why 1% em: instancing rounds coordinates; for CFF2 fontTools rounds relative operands,
    so points can drift about 0.7% em on complex CJK glyphs (invisible at reading sizes)."""
    bounds_tolerance = units_per_em * 0.01
    changed, total_master, total_out, compared = [], 0.0, 0.0, 0
    for cp, (w0, b0, a0) in sorted(master_shapes.items()):
        if cp not in out_shapes:
            continue  # reported by cmap-coverage
        w1, b1, a1 = out_shapes[cp]
        compared += 1
        total_master += abs(a0)
        total_out += abs(a1)
        same = abs(w0 - w1) <= 1 and (b0 is None) == (b1 is None)
        if same and b0 is not None:
            same = all(abs(x - y) <= bounds_tolerance for x, y in zip(b0, b1))
        if same:
            same = abs(abs(a0) - abs(a1)) <= max(2.0, abs(a0) * 0.10)
        if not same:
            changed.append(cp)
    drift = abs(total_out - total_master) / total_master if total_master else 0.0
    return {"ok": not changed and drift <= 0.005, "compared": compared, "areaDrift": round(drift, 5),
            "changed": [cp_label(cp) for cp in changed[:50]]}


def subset_options() -> subset.Options:
    options = subset.Options()
    options.layout_features = ["*"]  # keep vert/vrt2/locl/palt/ruby/... and their closure
    options.name_IDs = ["*"]  # fvar/STAT names, copyright and license records
    options.name_languages = ["*"]
    options.name_legacy = True
    options.notdef_outline = True  # missing glyphs show a visible box, not blank space
    options.legacy_kern = True
    options.recalc_timestamp = False  # same input -> same bytes
    return options


def _instance(font: TTFont, limits: dict, label: str, warnings: list) -> TTFont:
    is_cff2 = "CFF2" in font
    if is_cff2 and "VORG" in font and "VVAR" in font:
        warnings.append(
            f"{label}: CFF2 master has VORG; fontTools does not re-compute VORG for the instance, "
            "so vertical-text origins keep the default-instance values; check vertical pages in a reader "
            "or use a TrueType (glyf) master for vertical books"
        )
    try:
        return instancer.instantiateVariableFont(
            font, limits, updateFontNames="STAT" in font, downgradeCFF2=is_cff2
        )
    except Exception as exc:  # updateFontNames needs matching STAT records
        if "STAT" not in font:
            raise
        warnings.append(f"{label}: name table not updated from STAT ({exc}); internal names keep the master's names")
        return instancer.instantiateVariableFont(font, limits, downgradeCFF2=is_cff2)


def make_subset(master: TTFont, unicodes: set, spec: VariationSpec, limits: dict | None,
                flavor: str | None, label: str) -> tuple[bytes, list]:
    """Subset (and maybe instance) `master` in place; return the saved bytes and warnings."""
    warnings: list[str] = []
    subsetter = subset.Subsetter(subset_options())
    subsetter.populate(unicodes=sorted(unicodes))
    logging.getLogger("fontTools").setLevel(logging.ERROR)
    subsetter.subset(master)
    font = master
    if spec.mode == "instance":
        font = _instance(master, limits, label, warnings)
    elif spec.mode == "limit":
        font = instancer.instantiateVariableFont(master, limits)
    if axes_of(font):
        warnings.append(
            f"{label}: output is still a variable font; reader support is not verified "
            "(handbook §4.6 recommends static instances) - test it in the target readers"
        )
    font.flavor = flavor
    buffer = io.BytesIO()
    font.save(buffer)
    return buffer.getvalue(), warnings


def verify(master: FontFacts, out: FontFacts, required: set, spec: VariationSpec,
           limits: dict | None, target: str, out_font: TTFont, master_shapes: dict) -> tuple[dict, list]:
    """Return (checks, not_in_master). Every check has ok: bool.

    master_shapes comes from glyph_shapes(master, ..., target_location(...)) computed
    BEFORE make_subset, because make_subset changes the master font in place.
    """
    checks = {}
    wanted = {cp for cp in required if cp in master.cmap}
    not_in_master = sorted(cp for cp in required if cp not in master.cmap and not is_variation_selector(cp))

    missing = sorted(cp for cp in wanted if cp not in out.cmap)
    checks["cmap-coverage"] = {"ok": not missing, "wanted": len(wanted),
                               "missing": [cp_label(cp) for cp in missing[:50]]}

    wanted_uvs = {(base, sel) for base, sel in master.uvs if base in required and sel in required}
    lost_uvs = sorted(wanted_uvs - out.uvs)
    checks["uvs-sequences"] = {"ok": not lost_uvs, "wanted": len(wanted_uvs),
                               "lost": [f"U+{b:04X} U+{s:04X}" for b, s in lost_uvs[:50]]}

    vertical = sorted(cp for cp in wanted if master.cmap[cp] in master.vertical_inputs)
    lost_vertical = [cp for cp in vertical if out.cmap.get(cp) not in out.vertical_inputs]
    checks["vertical-alternates"] = {"ok": not lost_vertical, "wanted": len(vertical),
                                     "lost": [cp_label(cp) for cp in lost_vertical[:50]]}

    out_shapes = glyph_shapes(out_font, out.cmap, master_shapes)
    checks["outlines"] = compare_shapes(master_shapes, out_shapes, out_font["head"].unitsPerEm)

    checks["variation"] = _check_variation(master, out, spec, limits, out_font)

    ext = target_extension(target)
    expected_flavor = FLAVOR_BY_EXT.get(ext)
    outline_ok = (ext in (".woff", ".woff2")) or (ext == ".ttf") == (out.outline == "glyf")
    checks["format"] = {"ok": out.flavor == expected_flavor and outline_ok,
                        "extension": ext, "flavor": out.flavor, "outline": out.outline}
    return checks, not_in_master


def _close(axis: dict, bounds: tuple) -> bool:
    """fvar stores 16.16 fixed-point values, so compare with a small tolerance."""
    return abs(axis["min"] - bounds[0]) < 0.01 and abs(axis["max"] - bounds[1]) < 0.01


def _check_variation(master: FontFacts, out: FontFacts, spec: VariationSpec,
                     limits: dict | None, out_font: TTFont) -> dict:
    if spec.mode == "keep":
        ok = out.axes == master.axes
        if master.axes:
            ok = ok and ("gvar" in out.tables if out.outline == "glyf" else out.outline == "CFF2")
        return {"ok": ok, "mode": "keep", "axes": out.axes}
    if spec.mode == "instance":
        detail = {"mode": "instance", "location": limits, "axesLeft": out.axes, "outline": out.outline}
        ok = not out.axes and "gvar" not in out.tables and out.outline != "CFF2"
        if "wght" in limits:
            expected = min(1000, max(1, round(limits["wght"])))
            detail["usWeightClass"] = out_font["OS/2"].usWeightClass
            ok = ok and detail["usWeightClass"] == expected
        detail["ok"] = ok
        return detail
    ok = True
    by_tag = {a["tag"]: a for a in out.axes}
    for axis in master.axes:
        tag = axis["tag"]
        want = limits.get(tag)
        got = by_tag.get(tag)
        if want is None:
            ok = ok and got is not None and _close(got, (axis["min"], axis["max"]))
        elif isinstance(want, tuple):
            ok = ok and got is not None and _close(got, want)
        else:
            ok = ok and got is None
    return {"ok": ok, "mode": "limit", "limits": {k: list(v) if isinstance(v, tuple) else v for k, v in limits.items()},
            "axes": out.axes}
