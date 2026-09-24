"""Offline tests for epub_font. Run: uv run pytest -q"""

from __future__ import annotations

import io
import json
import os
import zipfile
from pathlib import Path

import pytest
from fontTools.ttLib import TTFont

from epub_font import epubtext, fontops, subset
from tests import synth

GLYF_VF = synth.build_glyf_font(variable=True)
GLYF_STATIC = synth.build_glyf_font(variable=False)
CFF2_VF = synth.build_cff2_font()
BOOK_FONTS = {
    "OEBPS/Fonts/st-all.ttf": GLYF_STATIC,          # placeholder bytes, replaced from the VF master
    "OEBPS/Fonts/st-all-semibold.ttf": GLYF_STATIC,
    "OEBPS/Fonts/kt.otf": CFF2_VF,                  # already-embedded CFF2 VF, subset in place
}


def book_chars(epub_bytes: bytes) -> set:
    with zipfile.ZipFile(io.BytesIO(epub_bytes)) as zf:
        return epubtext.read_book_text(zf).all_chars()


def run_job(master: bytes, target: str, variation: dict | None, text: str, shapes_at: dict | None = None):
    font = fontops.load_font(master, "master")
    facts = fontops.font_facts(font)
    spec = fontops.parse_variation(variation)
    limits = fontops.axis_limits(facts, spec, "master")
    required = {ord(c) for c in text}
    location = fontops.target_location(facts, spec, limits) if shapes_at is None else shapes_at
    master_shapes = fontops.glyph_shapes(font, facts.cmap, sorted(required), location)
    flavor = fontops.FLAVOR_BY_EXT[fontops.target_extension(target)]
    data, warnings = fontops.make_subset(font, required, spec, limits, flavor, target)
    out_font = fontops.load_font(data, "out")
    out = fontops.font_facts(out_font)
    checks, missing = fontops.verify(facts, out, required, spec, limits, target, out_font, master_shapes)
    return data, out_font, checks, warnings


# ---------- text collection ----------

def test_collects_text_attributes_css_nav_ncx_and_cdata():
    chars = book_chars(synth.build_epub(BOOK_FONTS))
    for ch in "中文、「」图标题正文导航书名章节第一章目录数据注“【】":
        assert ch in chars, ch
    assert chr(synth.UVS_SELECTOR) in chars


def test_skips_script_and_css_comments_and_urls():
    chars = book_chars(synth.build_epub(BOOK_FONTS))
    for ch in "脚本释里的字不应收集":
        assert ch not in chars, ch


def test_baseline_and_case_variants():
    chars = book_chars(synth.build_epub(BOOK_FONTS))
    assert set(epubtext.BASELINE_TEXT) <= chars
    assert "a" in chars and "A" in chars


def test_css_strings_decodes_escapes():
    assert epubtext.css_strings('p::before{content:"\\201C"} q{quotes:"\\300C" "\\300D"}') == ["“", "「", "」"]
    assert epubtext.css_strings('a{background:url("图.png")}') == []


# ---------- font operations ----------

TEXT = "中\U000E0100、「A 文"   # not 字 (U+5B57): it must be dropped


def test_keep_variable_glyf():
    data, out, checks, warnings = run_job(GLYF_VF, "x.ttf", {"mode": "keep"}, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert "fvar" in out and "gvar" in out
    assert "uni5B57" not in out.getGlyphOrder()
    assert any("still a variable font" in w for w in warnings)


def test_instance_glyf_pins_weight_and_keeps_vert_and_uvs():
    data, out, checks, _ = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 700}}, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert "fvar" not in out and "gvar" not in out
    assert out["OS/2"].usWeightClass == 700
    assert checks["vertical-alternates"]["wanted"] == 2
    assert checks["uvs-sequences"]["wanted"] == 1


def test_instance_changes_outlines():
    _, light, _, _ = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT)
    _, heavy, _, _ = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 900}}, TEXT)
    glyph = light.getBestCmap()[0x4E2D]
    light_x = [p[0] for p in light["glyf"][glyph].coordinates]
    heavy_x = [p[0] for p in heavy["glyf"][glyph].coordinates]
    assert min(heavy_x) == min(light_x) - 40 and max(heavy_x) == max(light_x) + 40


def test_outline_check_catches_wrong_weight():
    # master outlines drawn at wght=900, output pinned at 400: every inked glyph must be flagged
    _, _, checks, _ = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT,
                              shapes_at={"wght": 900})
    assert not checks["outlines"]["ok"]
    assert "U+4E2D 中" in checks["outlines"]["changed"]


def test_outline_check_compares_every_mapped_glyph():
    _, _, checks, _ = run_job(CFF2_VF, "x.otf", {"mode": "instance", "axes": {"wght": 650}}, TEXT)
    assert checks["outlines"]["ok"] and checks["outlines"]["compared"] == 6   # 中 、 「 A space 文


def test_limit_keeps_range():
    _, out, checks, _ = run_job(GLYF_VF, "x.ttf", {"mode": "limit", "axes": {"wght": [400, 700]}}, TEXT)
    assert checks["variation"]["ok"], checks["variation"]
    axis = out["fvar"].axes[0]
    assert (axis.minValue, axis.maxValue) == (400, 700)


def test_cff2_instance_downgrades_to_cff():
    _, out, checks, _ = run_job(CFF2_VF, "x.otf", {"mode": "instance", "axes": {"wght": 600}}, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert "CFF " in out and "CFF2" not in out and "fvar" not in out


def test_cff2_keep_and_woff2_flavor():
    _, out, checks, _ = run_job(CFF2_VF, "x.woff2", None, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert out.flavor == "woff2" and "CFF2" in out and "fvar" in out


def test_uvs_dropped_when_selector_not_in_text():
    _, out, checks, _ = run_job(GLYF_VF, "x.ttf", None, "中、")
    assert checks["uvs-sequences"]["wanted"] == 0
    assert fontops.uvs_pairs(out) == set()


def test_same_input_same_bytes():
    first = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT)[0]
    second = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT)[0]
    assert first == second


@pytest.mark.parametrize("master,variation,message", [
    (GLYF_STATIC, {"mode": "instance", "axes": {"wght": 400}}, "needs a variable font"),
    (GLYF_VF, {"mode": "instance", "axes": {"wdth": 100}}, "unknown axes"),
    (GLYF_VF, {"mode": "instance", "axes": {"wght": 1000}}, "outside"),
    (GLYF_VF, {"mode": "limit", "axes": {"wght": [700, 400]}}, "must satisfy"),
    (GLYF_VF, {"mode": "limit", "axes": {}}, "at least one axis"),
    (GLYF_VF, {"mode": "keep", "axes": {"wght": 400}}, "takes no axes"),
    (GLYF_VF, {"mode": "bogus"}, "must be one of"),
])
def test_bad_variation_config(master, variation, message):
    with pytest.raises(fontops.FontJobError, match=message):
        font = fontops.load_font(master, "master")
        fontops.axis_limits(fontops.font_facts(font), fontops.parse_variation(variation), "master")


def test_format_mismatch_and_collections():
    with pytest.raises(fontops.FontJobError, match="needs TrueType"):
        fontops.check_target_format("x.ttf", "CFF2")
    with pytest.raises(fontops.FontJobError, match="needs CFF"):
        fontops.check_target_format("x.otf", "glyf")
    with pytest.raises(fontops.FontJobError, match="unsupported font extension"):
        fontops.check_target_format("x.eot", "glyf")
    with pytest.raises(fontops.FontJobError, match="collections"):
        fontops.load_font(b"ttcf" + b"\x00" * 20, "x.ttc")


# ---------- command line ----------

def write_inputs(tmp_path: Path, config: dict, epub: bytes | None = None) -> tuple[Path, Path]:
    epub_path = tmp_path / "book.epub"
    epub_path.write_bytes(epub if epub is not None else synth.build_epub(BOOK_FONTS))
    (tmp_path / "masters").mkdir(exist_ok=True)
    (tmp_path / "masters" / "serif-vf.ttf").write_bytes(GLYF_VF)
    (tmp_path / "masters" / "math.ttf").write_bytes(synth.build_math_font())
    config_path = tmp_path / "fonts.json"
    config_path.write_text(json.dumps(config), encoding="utf-8")
    return epub_path, config_path


def run_subset(epub: Path, config: Path | None, out: Path) -> tuple[int, dict | None]:
    args = [str(epub), "--out", str(out)] + (["--config", str(config)] if config else [])
    code = subset.main(args)
    report = subset.report_path(out)
    return code, json.loads(report.read_text(encoding="utf-8")) if report.exists() else None


GOOD_CONFIG = {
    "version": 1,
    "fonts": [
        {"target": "OEBPS/Fonts/st-all.ttf", "master": "masters/serif-vf.ttf",
         "variation": {"mode": "instance", "axes": {"wght": 400}}},
        {"target": "OEBPS/Fonts/st-all-semibold.ttf", "master": "masters/serif-vf.ttf",
         "variation": {"mode": "instance", "axes": {"wght": 600}}},
        {"target": "OEBPS/Fonts/kt.otf", "variation": {"mode": "keep"}, "extraText": "字"},
    ],
}


def test_cli_end_to_end(tmp_path, capsys):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    candidate = tmp_path / "candidate.epub"
    code, report = run_subset(epub, config, candidate)
    assert code == 0, capsys.readouterr()
    assert report["ok"] and report["output"]["warnings"] == []
    assert [f["target"] for f in report["fonts"]] == [f["target"] for f in GOOD_CONFIG["fonts"]]
    kt = report["fonts"][2]
    assert kt["master"]["source"] == "epub:OEBPS/Fonts/kt.otf" and kt["output"]["axes"]

    with zipfile.ZipFile(epub) as before, zipfile.ZipFile(candidate) as after:
        assert [i.filename for i in before.infolist()] == [i.filename for i in after.infolist()]
        assert after.infolist()[0].filename == "mimetype"
        assert after.infolist()[0].compress_type == zipfile.ZIP_STORED
        font_reports = {font["target"]: font for font in report["fonts"]}
        for info in before.infolist():
            if info.filename in BOOK_FONTS:
                output_font = after.read(info.filename)
                assert output_font != before.read(info.filename)
                assert fontops.sha256(output_font) == font_reports[info.filename]["output"]["sha256"]
            else:
                assert after.read(info.filename) == before.read(info.filename), info.filename
        semibold = TTFont(io.BytesIO(after.read("OEBPS/Fonts/st-all-semibold.ttf")))
        kt_font = TTFont(io.BytesIO(after.read("OEBPS/Fonts/kt.otf")))
    assert semibold["OS/2"].usWeightClass == 600
    assert "uni5B57" in kt_font.getGlyphOrder()   # extraText
    assert "uni5B57" not in semibold.getGlyphOrder()


def test_cli_is_deterministic(tmp_path):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    hashes = []
    for name in ("a", "b"):
        out_epub = tmp_path / f"{name}.epub"
        code, _ = run_subset(epub, config, out_epub)
        assert code == 0
        hashes.append(fontops.sha256(out_epub.read_bytes()))
    assert hashes[0] == hashes[1]


@pytest.mark.parametrize("config_patch,epub_kwargs,message", [
    ({"fonts": [{"target": "OEBPS/Fonts/missing.ttf"}]}, {}, "is not in the EPUB"),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "typo": 1}]}, {}, "unknown keys"),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf"}, {"target": "OEBPS/Fonts/st-all.ttf"}]}, {}, "listed twice"),
    ({"version": 2}, {}, "version must be 1"),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "master": "masters/math.ttf"}]}, {}, "MATH"),
    ({"fonts": [{"target": "OEBPS/Fonts/kt.otf", "master": "masters/serif-vf.ttf",
                  "variation": {"mode": "keep"}}]}, {}, "needs CFF"),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf"}]}, {"encrypted": ("OEBPS/Fonts/st-all.ttf",)}, "encryption.xml"),
])
def test_cli_refuses_bad_input(tmp_path, capsys, config_patch, epub_kwargs, message):
    config = {**GOOD_CONFIG, **config_patch}
    epub_bytes = synth.build_epub(BOOK_FONTS, **epub_kwargs)
    epub, config_path = write_inputs(tmp_path, config, epub_bytes)
    candidate = tmp_path / "candidate.epub"
    code, report = run_subset(epub, config_path, candidate)
    assert code == 2 and report is None
    assert message in capsys.readouterr().err
    assert not candidate.exists() and not subset.report_path(candidate).exists()


def test_cli_refuses_existing_outputs(tmp_path, capsys):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    exists = tmp_path / "exists.epub"
    exists.write_bytes(b"x")
    assert run_subset(epub, config, exists)[0] == 2
    assert run_subset(epub, config, epub)[0] == 2
    reserved = tmp_path / "reserved.epub"
    reserved_report = subset.report_path(reserved)
    reserved_report.write_text("{}", encoding="utf-8")
    assert run_subset(epub, config, reserved)[0] == 2
    assert reserved_report.read_text(encoding="utf-8") == "{}"
    assert "already exists" in capsys.readouterr().err


def test_cli_failed_check_writes_no_candidate(tmp_path, monkeypatch, capsys):
    real_options = fontops.subset_options

    def no_features():
        options = real_options()
        options.layout_features = []   # drops vert -> vertical-alternates must fail
        return options

    monkeypatch.setattr(fontops, "subset_options", no_features)
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    candidate = tmp_path / "candidate.epub"
    code, report = run_subset(epub, config, candidate)
    assert code == 1
    assert report and not report["ok"] and report["output"] is None
    assert not report["fonts"][0]["checks"]["vertical-alternates"]["ok"]
    assert not candidate.exists()
    output = capsys.readouterr().out
    assert f"report: {subset.report_path(candidate)}" in output
    assert "output: not written (a check failed)" in output


def test_no_config_subsets_every_static_font(tmp_path):
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({"OEBPS/Fonts/st-all.ttf": GLYF_STATIC}))
    code, report = run_subset(epub, None, tmp_path / "new.epub")
    assert code == 0 and [font["target"] for font in report["fonts"]] == ["OEBPS/Fonts/st-all.ttf"]


def test_variable_font_needs_explicit_mode(tmp_path, capsys):
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({"OEBPS/Fonts/st-all.ttf": GLYF_VF}))
    out = tmp_path / "new.epub"
    code, report = run_subset(epub, None, out)
    assert code == 2 and report is None and "variation.mode" in capsys.readouterr().err
    assert not out.exists() and not subset.report_path(out).exists()


# ---------- optional: a real CJK variable font ----------

REAL_FONT = os.environ.get("EPUB_FONT_REAL_FONT")


@pytest.mark.skipif(not REAL_FONT, reason="set EPUB_FONT_REAL_FONT=/path/to/CJK-VF.ttf|.otf to run")
@pytest.mark.parametrize("variation", [None, {"mode": "instance", "axes": {"wght": 600}}])
def test_real_variable_font(variation):
    master = Path(REAL_FONT).read_bytes()
    target = "real" + (".otf" if master[:4] == b"OTTO" else ".ttf")
    text = "".join(epubtext.BASELINE_TEXT) + "永和九年岁在癸丑暮春之初会于会稽山阴之兰亭修禊事也"
    data, out, checks, _ = run_job(master, target, variation, text)
    assert all(c["ok"] for c in checks.values()), checks
    assert len(data) < len(master) / 10


@pytest.mark.skipif(not REAL_FONT, reason="set EPUB_FONT_REAL_FONT=/path/to/CJK-VF.ttf|.otf to run")
def test_real_variable_font_wrong_weight_is_caught():
    master = Path(REAL_FONT).read_bytes()
    target = "real" + (".otf" if master[:4] == b"OTTO" else ".ttf")
    text = "".join(epubtext.BASELINE_TEXT) + "永和九年岁在癸丑暮春之初会于会稽山阴之兰亭修禊事也"
    _, _, checks, _ = run_job(master, target, {"mode": "instance", "axes": {"wght": 400}}, text,
                              shapes_at={"wght": 420})
    assert not checks["outlines"]["ok"] and checks["outlines"]["areaDrift"] > 0.005
