"""Offline tests for epub_font. Run: uv run pytest -q"""

from __future__ import annotations

import io
import json
import os
import zipfile
from pathlib import Path

import pytest
from fontTools.ttLib import TTFont

from epub_font import check, epubtext, fontops, subset
from tests import synth

GLYF_VF = synth.build_glyf_font(variable=True)
GLYF_STATIC = synth.build_glyf_font(variable=False)
CFF2_VF = synth.build_cff2_font()
BOOK_FONTS = {
    "OEBPS/Fonts/st-all.ttf": GLYF_VF,
    "OEBPS/Fonts/st-all-semibold.ttf": GLYF_VF,
    "OEBPS/Fonts/kt.otf": CFF2_VF,                  # already-embedded CFF2 VF, subset in place
}


def book_chars(epub_bytes: bytes) -> set:
    with zipfile.ZipFile(io.BytesIO(epub_bytes)) as zf:
        return epubtext.read_book_text(zf).all_chars()


def run_job(master: bytes, target: str, variation: dict | None, text: str, shapes_at: dict | None = None):
    font = fontops.load_font(master, "source font")
    facts = fontops.font_facts(font)
    spec = fontops.parse_variation(variation)
    limits = fontops.axis_limits(facts, spec, "source font")
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


def test_cff2_instance_downgrades_to_cff():
    _, out, checks, _ = run_job(CFF2_VF, "x.otf", {"mode": "instance", "axes": {"wght": 600}}, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert "CFF " in out and "CFF2" not in out and "fvar" not in out


def test_cff2_instance_and_woff2_flavor():
    _, out, checks, _ = run_job(CFF2_VF, "x.woff2", {"mode": "instance"}, TEXT)
    assert all(c["ok"] for c in checks.values()), checks
    assert out.flavor == "woff2" and "CFF " in out and "CFF2" not in out and "fvar" not in out


def test_uvs_dropped_when_selector_not_in_text():
    _, out, checks, _ = run_job(GLYF_STATIC, "x.ttf", None, "中、")
    assert checks["uvs-sequences"]["wanted"] == 0
    assert fontops.uvs_pairs(out) == set()


def test_same_input_same_bytes():
    first = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT)[0]
    second = run_job(GLYF_VF, "x.ttf", {"mode": "instance", "axes": {"wght": 400}}, TEXT)[0]
    assert first == second


@pytest.mark.parametrize("font_data,variation,message", [
    (GLYF_STATIC, {"mode": "instance", "axes": {"wght": 400}}, "needs a variable font"),
    (GLYF_VF, {"mode": "instance", "axes": {"wdth": 100}}, "unknown axes"),
    (GLYF_VF, {"mode": "instance", "axes": {"wght": 1000}}, "outside"),
    (GLYF_VF, {"mode": "bogus"}, "must be 'instance'"),
])
def test_bad_variation_config(font_data, variation, message):
    with pytest.raises(fontops.FontJobError, match=message):
        font = fontops.load_font(font_data, "source font")
        fontops.axis_limits(fontops.font_facts(font), fontops.parse_variation(variation), "source font")


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
    config_path = tmp_path / "fonts.json"
    config_path.write_text(json.dumps(config), encoding="utf-8")
    return epub_path, config_path


def run_subset(epub: Path, config: Path | None, out: Path) -> tuple[int, dict | None]:
    args = [str(epub), "--out", str(out)] + (["--config", str(config)] if config else [])
    code = subset.main(args)
    report = subset.report_path(out)
    return code, json.loads(report.read_text(encoding="utf-8")) if report.exists() else None


def independent_coverage_pair(
    source_empty: str = "",
    source_has_uvs: bool = True,
    output_empty: str = "",
    output_has_uvs: bool = True,
):
    epub = synth.build_epub({"OEBPS/Fonts/st-all.ttf": GLYF_STATIC})
    with zipfile.ZipFile(io.BytesIO(epub)) as zf:
        _, harvest = check.harvest_book(zf)
    chars = "".join(harvest.chars)
    sequences = tuple(harvest.sequences)
    source = synth.build_font_for(chars, empty=source_empty, uvs=sequences if source_has_uvs else ())
    output = synth.build_font_for(chars, empty=output_empty, uvs=sequences if output_has_uvs else ())
    return epub, harvest, source, output


def check_independent_coverage(epub: bytes, source: bytes, output: bytes) -> dict:
    with zipfile.ZipFile(io.BytesIO(epub)) as zf:
        _, harvest = check.harvest_book(zf)
        return subset._independent_coverage(harvest, "OEBPS/Fonts/st-all.ttf", source, output)


GOOD_CONFIG = {
    "version": 1,
    "fonts": [
        {"target": "OEBPS/Fonts/st-all.ttf", "variation": {"mode": "instance", "axes": {"wght": 400}}},
        {"target": "OEBPS/Fonts/st-all-semibold.ttf", "variation": {"mode": "instance", "axes": {"wght": 600}}},
        {"target": "OEBPS/Fonts/kt.otf", "variation": {"mode": "instance"}, "extraText": "字"},
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
    assert kt["sourceFont"]["source"] == "epub:OEBPS/Fonts/kt.otf" and kt["output"]["axes"] == []

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
    assert ord("字") in kt_font.getBestCmap()   # extraText
    assert ord("字") not in semibold.getBestCmap()


def test_cli_harvests_independent_book_coverage_once_for_all_fonts(tmp_path, capsys, monkeypatch):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    calls = 0
    original = check.harvest_book

    def count_harvest(zf):
        nonlocal calls
        calls += 1
        return original(zf)

    monkeypatch.setattr(check, "harvest_book", count_harvest)
    code, report = run_subset(epub, config, tmp_path / "candidate.epub")

    assert code == 0, capsys.readouterr()
    assert report["ok"]
    assert calls == 1


def test_report_schema_version_is_3(tmp_path):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    code, report = run_subset(epub, config, tmp_path / "candidate.epub")
    assert code == 0, report
    assert report["schemaVersion"] == 3
    assert report["usage"] == {"mode": "book", "algorithm": "book-v1", "fallbackReasons": []}


def test_cli_is_deterministic(tmp_path):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    hashes = []
    for name in ("a", "b"):
        out_epub = tmp_path / f"{name}.epub"
        code, _ = run_subset(epub, config, out_epub)
        assert code == 0
        hashes.append(fontops.sha256(out_epub.read_bytes()))
    assert hashes[0] == hashes[1]


def test_subset_fails_when_collector_misses_char(tmp_path, monkeypatch):
    epub, config = write_inputs(tmp_path, GOOD_CONFIG)
    output = tmp_path / "candidate.epub"
    original_all_chars = epubtext.BookText.all_chars

    def omit_documented_character(book):
        return original_all_chars(book) - {"文"}

    monkeypatch.setattr(epubtext.BookText, "all_chars", omit_documented_character)

    code, report = run_subset(epub, config, output)

    assert code == 1
    assert report is not None and not report["ok"]
    regular = report["fonts"][0]
    independent = regular["checks"]["independent-coverage"]
    assert not independent["ok"]
    assert any(issue.get("char") == "U+6587 文" for issue in independent["regressions"])
    assert not output.exists()


def test_independent_coverage_flags_no_ink_regression():
    epub, harvest, source, output = independent_coverage_pair(output_empty="中")

    assert check.check_font(source, "source", harvest)["noInk"] == []
    result = check_independent_coverage(epub, source, output)

    assert not result["ok"]
    assert any(item["kind"] == "noInk" and item["char"] == "U+4E2D 中" for item in result["regressions"])


def test_independent_coverage_flags_lost_variation_sequence():
    epub, harvest, source, output = independent_coverage_pair(output_has_uvs=False)

    assert check.check_font(source, "source", harvest)["missingSequences"] == []
    result = check_independent_coverage(epub, source, output)

    assert not result["ok"]
    assert any(
        item["kind"] == "missingSequences" and item["sequence"] == "U+4E2D U+E0100"
        for item in result["regressions"]
    )


def test_independent_coverage_allows_gap_present_in_source():
    epub, harvest, source, output = independent_coverage_pair(
        source_empty="中", source_has_uvs=False, output_empty="中", output_has_uvs=False
    )

    source_report = check.check_font(source, "source", harvest)
    assert source_report["noInk"]
    assert source_report["missingSequences"]
    result = check_independent_coverage(epub, source, output)

    assert result == {"ok": True, "regressions": []}


def test_cli_rejects_external_master_config_field(tmp_path, capsys):
    config = {
        "version": 1,
        "fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "master": "outside.ttf"}],
    }
    epub, config_path = write_inputs(tmp_path, config)
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 2 and report is None
    assert "unknown keys" in capsys.readouterr().err
    assert not output.exists() and not subset.report_path(output).exists()


def test_deprecated_action_has_removal_version(tmp_path, capsys):
    config = {"version": 1, "fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "action": "preserve"}]}
    regular = synth.build_epub({"OEBPS/Fonts/st-all.ttf": GLYF_STATIC})
    epub, config_path = write_inputs(tmp_path, config, regular)
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 0, report
    assert report["fonts"][0]["action"] == "subset"
    assert report["fonts"][0]["original"]["sha256"] != report["fonts"][0]["output"]["sha256"]
    output_text = capsys.readouterr().out
    assert "deprecated" in output_text and "3.0.0" in output_text
    with zipfile.ZipFile(output) as candidate:
        assert candidate.read("OEBPS/Fonts/st-all.ttf") != GLYF_STATIC


@pytest.mark.parametrize("variation", [
    {"mode": "keep"},
    {"mode": "limit", "axes": {"wght": [400, 700]}},
])
def test_removed_variable_font_modes_are_rejected(variation):
    with pytest.raises(fontops.FontJobError, match="must be 'instance'"):
        fontops.parse_variation(variation)


@pytest.mark.parametrize("config_patch,epub_kwargs,message,config_bytes", [
    ({"fonts": [{"target": "OEBPS/Fonts/missing.ttf"}]}, {}, "is not in the EPUB", None),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "typo": 1}]}, {}, "unknown keys", None),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "action": "subset"}]}, {}, "only the legacy value 'preserve'", None),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf", "action": []}]}, {}, "only the legacy value 'preserve'", None),
    ({"usage": []}, {}, "config.usage must be 'book' or 'css'", None),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf"}, {"target": "OEBPS/Fonts/st-all.ttf"}]}, {}, "listed twice", None),
    ({"version": 2}, {}, "version must be 1", None),
    ({"fonts": [{"target": "OEBPS/Fonts/st-all.ttf"}]}, {"encrypted": ("OEBPS/Fonts/st-all.ttf",)}, "encryption.xml", None),
    ({}, {}, "not valid UTF-8", b"\xff\xfe{}"),
])
def test_cli_refuses_bad_input(tmp_path, capsys, config_patch, epub_kwargs, message, config_bytes):
    config = {**GOOD_CONFIG, **config_patch}
    epub_bytes = synth.build_epub(BOOK_FONTS, **epub_kwargs)
    epub, config_path = write_inputs(tmp_path, config, epub_bytes)
    if config_bytes is not None:
        config_path.write_bytes(config_bytes)
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


def test_config_only_overrides_listed_fonts(tmp_path):
    targets = ("OEBPS/Fonts/regular.ttf", "OEBPS/Fonts/alternate.ttf")
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({target: GLYF_STATIC for target in targets}))
    config = {"version": 1, "fonts": [{"target": targets[0]}]}
    config_path = tmp_path / "fonts.json"
    config_path.write_text(json.dumps(config), encoding="utf-8")
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 0, report
    results = {font["target"]: font for font in report["fonts"]}
    assert set(results) == set(targets)
    assert all(result["output"]["sha256"] != result["original"]["sha256"] for result in results.values())
    with zipfile.ZipFile(output) as candidate:
        assert all(candidate.read(target) != GLYF_STATIC for target in targets)


def test_config_automatically_preserves_unlisted_math_font(tmp_path):
    regular_target = "OEBPS/Fonts/regular.ttf"
    math_target = "OEBPS/Fonts/math.ttf"
    math_font = synth.build_math_font()
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({regular_target: GLYF_STATIC, math_target: math_font}))
    config = {"version": 1, "fonts": [{"target": regular_target}]}
    config_path = tmp_path / "fonts.json"
    config_path.write_text(json.dumps(config), encoding="utf-8")
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 0, report
    results = {font["target"]: font for font in report["fonts"]}
    assert set(results) == {regular_target, math_target}
    assert results[math_target]["action"] == "preserve"
    assert results[math_target]["reason"] == "math-table"
    assert results[math_target]["output"]["sha256"] == results[math_target]["original"]["sha256"]
    with zipfile.ZipFile(output) as candidate:
        assert candidate.read(math_target) == math_font


def test_config_automatically_rejects_unlisted_variable_font(tmp_path, capsys):
    regular_target = "OEBPS/Fonts/regular.ttf"
    variable_target = "OEBPS/Fonts/variable.ttf"
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({regular_target: GLYF_STATIC, variable_target: GLYF_VF}))
    config = {"version": 1, "fonts": [{"target": regular_target}]}
    config_path = tmp_path / "fonts.json"
    config_path.write_text(json.dumps(config), encoding="utf-8")
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 2 and report is None
    assert "variation.mode" in capsys.readouterr().err
    assert not output.exists() and not subset.report_path(output).exists()


def test_no_config_preserves_math_font_and_reports_hashes(tmp_path):
    math_font = synth.build_math_font()
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({"OEBPS/Fonts/math.ttf": math_font}))
    source_sha = fontops.sha256(epub.read_bytes())
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, None, output)

    assert code == 0, report
    assert report["ok"]
    result, = report["fonts"]
    assert result["target"] == "OEBPS/Fonts/math.ttf"
    assert result["action"] == "preserve" and result["reason"] == "math-table"
    assert result["original"]["sha256"] == result["output"]["sha256"]
    assert result["original"]["sha256"] == fontops.sha256(math_font)
    assert fontops.sha256(epub.read_bytes()) == source_sha
    with zipfile.ZipFile(epub) as before, zipfile.ZipFile(output) as after:
        assert after.read("OEBPS/Fonts/math.ttf") == math_font
        for info in before.infolist():
            if info.filename != "OEBPS/Fonts/math.ttf":
                assert after.read(info) == before.read(info), info.filename


def test_no_config_preserves_math_and_subsets_regular_fonts(tmp_path):
    math_font = synth.build_math_font()
    regular_font = GLYF_STATIC
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({
        "OEBPS/Fonts/math.ttf": math_font,
        "OEBPS/Fonts/st-all.ttf": regular_font,
    }))
    source_sha = fontops.sha256(epub.read_bytes())
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, None, output)

    assert code == 0, report
    results = {result["target"]: result for result in report["fonts"]}
    assert results["OEBPS/Fonts/math.ttf"]["action"] == "preserve"
    assert results["OEBPS/Fonts/math.ttf"]["reason"] == "math-table"
    assert results["OEBPS/Fonts/math.ttf"]["original"]["sha256"] == results["OEBPS/Fonts/math.ttf"]["output"]["sha256"]
    assert results["OEBPS/Fonts/st-all.ttf"]["action"] == "subset"
    assert results["OEBPS/Fonts/st-all.ttf"]["original"]["sha256"] != results["OEBPS/Fonts/st-all.ttf"]["output"]["sha256"]
    assert fontops.sha256(epub.read_bytes()) == source_sha
    with zipfile.ZipFile(output) as after:
        assert after.read("OEBPS/Fonts/math.ttf") == math_font
        assert after.read("OEBPS/Fonts/st-all.ttf") != regular_font


def test_explicit_preserve_action_accepts_math_font(tmp_path):
    math_font = synth.build_math_font()
    config = {"version": 1, "fonts": [{"target": "OEBPS/Fonts/math.ttf", "action": "preserve"}]}
    epub, config_path = write_inputs(tmp_path, config, synth.build_epub({"OEBPS/Fonts/math.ttf": math_font}))
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 0, report
    assert report["fonts"][0]["action"] == "preserve"
    assert report["fonts"][0]["reason"] == "math-table"
    assert any("deprecated" in warning for warning in report["fonts"][0]["warnings"])
    with zipfile.ZipFile(output) as after:
        assert after.read("OEBPS/Fonts/math.ttf") == math_font


@pytest.mark.parametrize("deprecated_action", [None, "preserve"])
def test_configured_math_font_is_automatically_preserved(tmp_path, deprecated_action):
    math_font = synth.build_math_font()
    job = {"target": "OEBPS/Fonts/math.ttf"}
    if deprecated_action is not None:
        job["action"] = deprecated_action
    config = {"version": 1, "fonts": [job]}
    epub, config_path = write_inputs(tmp_path, config, synth.build_epub({"OEBPS/Fonts/math.ttf": math_font}))
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, config_path, output)

    assert code == 0, report
    assert report["fonts"][0]["action"] == "preserve"
    assert report["fonts"][0]["output"]["sha256"] == report["fonts"][0]["original"]["sha256"]
    with zipfile.ZipFile(output) as candidate:
        assert candidate.read("OEBPS/Fonts/math.ttf") == math_font


def test_auto_mode_does_not_preserve_encrypted_math_font(tmp_path, capsys):
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({"OEBPS/Fonts/math.ttf": synth.build_math_font()},
                                      encrypted=("OEBPS/Fonts/math.ttf",)))
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, None, output)

    assert code == 2 and report is None
    assert "encryption.xml" in capsys.readouterr().err
    assert not output.exists() and not subset.report_path(output).exists()


@pytest.mark.parametrize("target,font_bytes,message", [
    ("OEBPS/Fonts/math.ttf", b"not a font", "cannot parse font"),
    ("OEBPS/Fonts/math.eot", synth.build_math_font(), "unsupported font extension"),
])
def test_auto_math_preserve_still_rejects_bad_or_unsupported_fonts(tmp_path, capsys, target, font_bytes, message):
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({target: font_bytes}))
    output = tmp_path / "new.epub"

    code, report = run_subset(epub, None, output)

    assert code == 2 and report is None
    assert message in capsys.readouterr().err
    assert not output.exists() and not subset.report_path(output).exists()


def test_variable_font_needs_explicit_mode(tmp_path, capsys):
    epub = tmp_path / "book.epub"
    epub.write_bytes(synth.build_epub({"OEBPS/Fonts/st-all.ttf": GLYF_VF}))
    out = tmp_path / "new.epub"
    code, report = run_subset(epub, None, out)
    error = capsys.readouterr().err
    assert code == 2 and report is None
    assert "fonts.json" in error and "variation.mode" in error and "font_config=fonts.json" in error
    assert not out.exists() and not subset.report_path(out).exists()


# ---------- optional: a real CJK variable font ----------

REAL_FONT = os.environ.get("EPUB_FONT_REAL_FONT")


@pytest.mark.skipif(not REAL_FONT, reason="set EPUB_FONT_REAL_FONT=/path/to/CJK-VF.ttf|.otf to run")
def test_real_variable_font():
    master = Path(REAL_FONT).read_bytes()
    target = "real" + (".otf" if master[:4] == b"OTTO" else ".ttf")
    text = "".join(epubtext.BASELINE_TEXT) + "永和九年岁在癸丑暮春之初会于会稽山阴之兰亭修禊事也"
    data, out, checks, _ = run_job(master, target, {"mode": "instance", "axes": {"wght": 600}}, text)
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
