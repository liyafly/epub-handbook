"""Command line: subset every manifest font, applying per-font config overrides.

    epub-font subset BOOK.epub --out NEW.epub [--config fonts.json]

Exit codes: 0 = every check passed; 1 = a check failed (report written, no
candidate EPUB); 2 = bad input or unsupported font (nothing trustworthy written).
The candidate EPUB only replaces the bytes of the configured font entries; the
OPF, CSS, XHTML and every other entry are copied unchanged, in the same order
and with the same compression method.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import zipfile
from importlib.metadata import version as package_version
from pathlib import Path

import fontTools

from . import check, epubtext, fontops

CONFIG_KEYS = {"version", "fonts"}
FONT_KEYS = {"target", "variation", "extraText"}
DEPRECATED_FONT_KEYS = {"action"}
DEPRECATED_ACTION_REMOVAL_VERSION = "3.0.0"


class UsageError(Exception):
    pass


def _load_config(path: Path) -> dict:
    try:
        config = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise UsageError(f"cannot read config {path}: {exc}") from exc
    except UnicodeDecodeError as exc:
        raise UsageError(f"config {path} is not valid UTF-8: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise UsageError(f"config {path} is not valid JSON: {exc}") from exc
    if not isinstance(config, dict):
        raise UsageError("config must be a JSON object")
    unknown = set(config) - CONFIG_KEYS
    if unknown:
        raise UsageError(f"config has unknown keys: {sorted(unknown)}")
    if config.get("version") != 1:
        raise UsageError("config.version must be 1")
    fonts = config.get("fonts")
    if not isinstance(fonts, list) or not fonts:
        raise UsageError("config.fonts must be a non-empty list")
    seen = set()
    for index, job in enumerate(fonts):
        if not isinstance(job, dict):
            raise UsageError(f"fonts[{index}] must be an object")
        unknown = set(job) - FONT_KEYS - DEPRECATED_FONT_KEYS
        if unknown:
            raise UsageError(f"fonts[{index}] has unknown keys: {sorted(unknown)}")
        target = job.get("target")
        if not isinstance(target, str) or not target or target.startswith("/"):
            raise UsageError(f"fonts[{index}].target must be a ZIP path such as OEBPS/Fonts/st-all.ttf")
        if "action" in job and job["action"] != "preserve":
            raise UsageError(f"fonts[{index}].action is deprecated; only the legacy value 'preserve' is accepted")
        if target in seen:
            raise UsageError(f"fonts[{index}].target {target} is listed twice")
        seen.add(target)
        if "extraText" in job and not isinstance(job["extraText"], str):
            raise UsageError(f"fonts[{index}].extraText must be a string")
    return config


def report_path(out_epub: Path) -> Path:
    return out_epub.with_name(out_epub.stem + ".font-report.json")


def _check_output_paths(epub: Path, out_epub: Path) -> None:
    if out_epub.suffix.lower() != ".epub":
        raise UsageError("--out must end with .epub")
    if out_epub.resolve() == epub.resolve():
        raise UsageError("--out must differ from the input EPUB")
    if out_epub.exists():
        raise UsageError(f"--out {out_epub} already exists; choose a new path")
    report = report_path(out_epub)
    if report.exists():
        raise UsageError(f"report {report} already exists; choose a new output path")


def _preserve_math_job(target: str, item, original_bytes: bytes, original_font) -> dict:
    facts = fontops.font_facts(original_font)
    fontops.check_target_format(target, facts.outline)
    digest = fontops.sha256(original_bytes)
    return {
        "target": target,
        "manifestId": item.item_id,
        "mediaType": item.media_type,
        "action": "preserve",
        "reason": "math-table",
        "sourceFont": {"source": f"epub:{target}", "sha256": digest, "bytes": len(original_bytes),
                       "glyphs": facts.glyph_count, "outline": facts.outline, "axes": facts.axes},
        "variation": {"mode": "preserve", "axes": {}},
        "original": {"sha256": digest, "bytes": len(original_bytes)},
        "output": {"sha256": digest, "bytes": len(original_bytes), "glyphs": facts.glyph_count,
                   "outline": facts.outline, "flavor": facts.flavor, "axes": facts.axes, "tables": facts.tables},
        "requiredCodepoints": None,
        "notInMaster": None,
        "notInMasterCount": None,
        "checks": {"preserved-bytes": {"ok": True, "sha256": digest}},
        "ok": True,
        "warnings": [],
        "_bytes": original_bytes,
    }


def _independent_coverage(harvest: check.Harvest, target: str, source_bytes: bytes, output_bytes: bytes) -> dict:
    source = check.check_font(source_bytes, target, harvest)
    output = check.check_font(output_bytes, target, harvest)
    regressions = check._regressions(source, output)
    return {"ok": not regressions, "regressions": regressions[:200]}


def _process_job(job: dict, book: epubtext.BookText, harvest: check.Harvest, zf: zipfile.ZipFile) -> dict:
    target = job["target"]
    if target not in zf.namelist():
        raise UsageError(f"{target} is not in the EPUB (only existing font entries can be replaced)")
    item = book.item_by_path(target)
    if item is None:
        raise UsageError(f"{target} has no OPF manifest item")
    if target in book.encrypted_paths:
        raise UsageError(f"{target} is listed in META-INF/encryption.xml; stop (encrypted/obfuscated fonts are not handled)")
    original_bytes = zf.read(target)
    original_font = fontops.load_font(original_bytes, f"epub:{target}")
    original_is_math = "MATH" in original_font
    deprecated_preserve = job.get("action") == "preserve"
    if original_is_math:
        result = _preserve_math_job(target, item, original_bytes, original_font)
        result["checks"]["independent-coverage"] = _independent_coverage(
            harvest, target, original_bytes, original_bytes
        )
        result["ok"] = all(check_result["ok"] for check_result in result["checks"].values())
        if deprecated_preserve:
            result["warnings"].append(
                f"{target}: configuration key action is deprecated and ignored; remove it before epub-font {DEPRECATED_ACTION_REMOVAL_VERSION}"
            )
        return result

    spec = fontops.parse_variation(job.get("variation"))
    source_font_bytes = original_bytes
    source_font_label = f"epub:{target}"
    source_font = original_font
    source_facts = fontops.font_facts(source_font)
    if "variation" not in job and source_facts.axes:
        raise fontops.FontJobError(
            f'{target}: variable font; add it to fonts.json with variation.mode '
            '(epub-font subset --config fonts.json or epub run epub.font.subset with '
            'font_config=fonts.json; "instance" is recommended, see README)'
        )
    fontops.check_target_format(target, source_facts.outline)
    limits = fontops.axis_limits(source_facts, spec, source_font_label)

    required = {ord(ch) for ch in book.all_chars() | set(job.get("extraText", ""))}
    location = fontops.target_location(source_facts, spec, limits)
    source_shapes = fontops.glyph_shapes(source_font, source_facts.cmap, sorted(required), location)
    flavor = fontops.FLAVOR_BY_EXT[fontops.target_extension(target)]
    out_bytes, warnings = fontops.make_subset(source_font, required, spec, limits, flavor, target)
    if deprecated_preserve:
        warnings.append(
            f"{target}: configuration key action is deprecated and ignored; remove it before epub-font {DEPRECATED_ACTION_REMOVAL_VERSION}"
        )

    out_font = fontops.load_font(out_bytes, f"{target} (output)")
    out = fontops.font_facts(out_font)
    checks, not_in_master = fontops.verify(source_facts, out, required, spec, limits, target, out_font, source_shapes)
    checks["independent-coverage"] = _independent_coverage(harvest, target, original_bytes, out_bytes)
    if not_in_master:
        warnings.append(f"{target}: {len(not_in_master)} required characters are not in the master font (fallback fonts must cover them)")
    return {
        "target": target,
        "manifestId": item.item_id,
        "mediaType": item.media_type,
        "action": "subset",
        "sourceFont": {"source": source_font_label, "sha256": fontops.sha256(source_font_bytes),
                       "bytes": len(source_font_bytes), "glyphs": source_facts.glyph_count,
                       "outline": source_facts.outline, "axes": source_facts.axes},
        "variation": {"mode": spec.mode or "static", "axes": spec.axes or {}},
        "original": {"sha256": fontops.sha256(original_bytes), "bytes": len(original_bytes)},
        "output": {"sha256": fontops.sha256(out_bytes), "bytes": len(out_bytes), "glyphs": out.glyph_count,
                   "outline": out.outline, "flavor": out.flavor, "axes": out.axes, "tables": out.tables},
        "requiredCodepoints": len(required),
        "notInMaster": [fontops.cp_label(cp) for cp in not_in_master[:200]],
        "notInMasterCount": len(not_in_master),
        "checks": checks,
        "ok": all(check["ok"] for check in checks.values()),
        "warnings": warnings,
        "_bytes": out_bytes,
    }


def write_candidate_epub(src: Path, dst: Path, replacements: dict) -> list:
    """Copy every entry in order; replace only the given entries' bytes. Returns warnings."""
    warnings = []
    partial = dst.with_name(dst.name + ".partial")
    try:
        with zipfile.ZipFile(src) as zin, zipfile.ZipFile(partial, "w") as zout:
            infos = zin.infolist()
            if not infos or infos[0].filename != "mimetype" or infos[0].compress_type != zipfile.ZIP_STORED:
                warnings.append("input EPUB does not start with a stored 'mimetype' entry; copied as-is")
            for info in infos:
                data = replacements.get(info.filename)
                if data is None:
                    data = zin.read(info)
                out_info = zipfile.ZipInfo(info.filename, date_time=info.date_time)
                out_info.compress_type = info.compress_type
                out_info.external_attr = info.external_attr
                out_info.create_system = info.create_system
                zout.writestr(out_info, data)
        os.replace(partial, dst)
    except BaseException:
        partial.unlink(missing_ok=True)
        raise
    return warnings


def run(args) -> int:
    epub = Path(args.epub)
    out_epub = Path(args.out)
    report = report_path(out_epub)
    config_path = Path(args.config) if args.config else None
    if not epub.is_file():
        raise UsageError(f"{epub} is not a regular file")
    _check_output_paths(epub, out_epub)
    config = _load_config(config_path) if config_path else None
    try:
        zf = zipfile.ZipFile(epub)
    except zipfile.BadZipFile as exc:
        raise UsageError(f"{epub} is not a ZIP/EPUB file: {exc}") from exc
    with zf:
        try:
            book = epubtext.read_book_text(zf)
        except epubtext.EpubError as exc:
            raise UsageError(str(exc)) from exc
        try:
            _, independent_harvest = check.harvest_book(zf)
        except check.CheckError as exc:
            raise UsageError(str(exc)) from exc
        manifest_fonts = [
            item for item in book.items
            if item.path.lower().endswith(tuple(fontops.FLAVOR_BY_EXT)) or "font" in item.media_type.lower()
        ]
        if config is None:
            if not manifest_fonts:
                raise UsageError("the EPUB has no manifest fonts")
            jobs = [{"target": item.path} for item in manifest_fonts]
        else:
            jobs = list(config["fonts"])
            configured_targets = {job["target"] for job in jobs}
            automatic_jobs = [
                {"target": item.path} for item in manifest_fonts if item.path not in configured_targets
            ]
            jobs.extend(automatic_jobs)
        try:
            results = [_process_job(job, book, independent_harvest, zf) for job in jobs]
        except fontops.FontJobError as exc:
            raise UsageError(str(exc)) from exc

    all_ok = all(result["ok"] for result in results)
    report_data = {
        "schemaVersion": 2,
        "tool": "epub-font subset",
        "providerVersion": package_version("epub-font"),
        "fontTools": fontTools.version,
        "input": {"path": str(epub), "sha256": fontops.sha256(epub.read_bytes())},
        "config": ({"path": str(config_path), "sha256": fontops.sha256(config_path.read_bytes())}
                   if config_path else None),
        "charset": {
            "total": len(book.all_chars()),
            "bySource": {name: len(chars) for name, chars in sorted(book.chars_by_source.items())},
        },
        "fonts": [{k: v for k, v in result.items() if k != "_bytes"} for result in results],
        "ok": all_ok,
        "output": None,
    }
    out_epub.parent.mkdir(parents=True, exist_ok=True)
    if all_ok:
        warnings = write_candidate_epub(epub, out_epub, {r["target"]: r["_bytes"] for r in results})
        report_data["output"] = {"path": str(out_epub), "sha256": fontops.sha256(out_epub.read_bytes()),
                                 "warnings": warnings}
    report.write_text(
        json.dumps(report_data, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )

    for result in results:
        status = "OK" if result["ok"] else "FAIL"
        failed = [name for name, check in result["checks"].items() if not check["ok"]]
        operation = f"action={result['action']}"
        if result["action"] == "subset":
            operation += f" mode={result['variation']['mode']}"
        else:
            operation += f" reason={result['reason']}"
        print(f"[{status}] {result['target']} {operation} "
              f"{result['original']['bytes']} -> {result['output']['bytes']} bytes, "
              f"glyphs {result['sourceFont']['glyphs']} -> {result['output']['glyphs']}"
              + (f", failed: {', '.join(failed)}" if failed else ""))
        for warning in result["warnings"]:
            print(f"  warning: {warning}")
    print(f"report: {report}")
    print(f"output: {out_epub}" if report_data["output"] else "output: not written (a check failed)")
    return 0 if all_ok else 1


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="epub-font subset", description=__doc__.splitlines()[0])
    parser.add_argument("epub", help="input EPUB (read only)")
    parser.add_argument("--out", required=True, help="new output EPUB path (must not exist)")
    parser.add_argument("--config", help="optional fonts.json (see examples/fonts.*.json)")
    args = parser.parse_args(argv)
    try:
        return run(args)
    except UsageError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
