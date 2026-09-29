"""Tests for the independent completeness check. Run: uv run pytest -q"""

from __future__ import annotations

import io
import json
import zipfile
from pathlib import Path

import pytest
from fontTools.ttLib import TTFont

from epub_font import check, subset
from tests import synth

VS17 = "\U000E0100"
CHAPTER_DTD = f"""<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
<head><title>章</title><style>.x::before {{ content: "\\201C"; }}</style><script>var s = "脚";</script></head>
<body>
<p class="em">中文&nbsp;<q>引</q><img src="a.png" alt="图"/><span title="题">字</span>中{VS17}<!-- 注 -->Ab</p>
<ol class="n"><li>一</li></ol>
</body>
</html>
"""
CHAPTER_NO_DTD = CHAPTER_DTD.replace(
    '<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">\n', ""
).replace("中文&nbsp;", "中文&nbsp;&hellip;")
CSS = """.em { -epub-text-emphasis-style: filled sesame; }
ol.n { list-style-type: cjk-decimal; }
@media screen { .t { text-transform: uppercase; } }
/* "注释里的字" */
"""
PLACEHOLDER = synth.build_glyf_font(variable=False)


def make_epub(chapter: str = CHAPTER_DTD, fonts: dict | None = None, css: str = CSS, **kwargs) -> bytes:
    return synth.build_epub(fonts or {"OEBPS/Fonts/st-all.ttf": PLACEHOLDER}, chapter=chapter, css=css, **kwargs)


def required(epub: bytes) -> check.Harvest:
    with zipfile.ZipFile(io.BytesIO(epub)) as zf:
        return check.harvest_book(zf)[1]


def font_covering(harvest: check.Harvest, drop: str = "", empty: str = "", uvs: bool = True) -> bytes:
    chars = "".join(ch for ch in harvest.chars if ch not in drop)
    return synth.build_font_for(chars, empty=empty, uvs=tuple(harvest.sequences) if uvs else ())


def run_cli(tmp_path: Path, epub: bytes, *args: str) -> tuple[int, dict | None]:
    book = tmp_path / "book.epub"
    book.write_bytes(epub)
    report = tmp_path / "report.json"
    code = check.main([str(book), *args, "--json", str(report)])
    return code, json.loads(report.read_text(encoding="utf-8")) if report.exists() else None


def replace_epub_entry(epub: bytes, path: str, replacement: bytes) -> bytes:
    source = zipfile.ZipFile(io.BytesIO(epub))
    output = io.BytesIO()
    with source, zipfile.ZipFile(output, "w") as target:
        for info in source.infolist():
            data = replacement if info.filename == path else source.read(info)
            target.writestr(info, data)
    return output.getvalue()


def test_collects_everything_the_reader_renders():
    h = required(make_epub())
    for ch in "中文引图题字章一目录导航书名章节Ab “”‘’﹅﹆〇、":
        assert ch in h.chars, ch
    assert "a" in h.chars and "B" in h.chars          # text-transform: uppercase variants
    assert (ord("中"), 0xE0100) in h.sequences
    for ch in "脚注释里":                               # <script>, XML comment, CSS comment
        assert ch not in h.chars, ch


def test_entities_without_doctype_are_decoded():
    h = required(make_epub(CHAPTER_NO_DTD))
    assert " " in h.chars and "…" in h.chars
    assert not h.warnings


def test_font_names_and_charset_are_not_required():
    css = '@charset "utf-8";\nbody { font-family: "思源宋体", serif; }\n.a::before { content: "甲"; }\n'
    chapter = CHAPTER_DTD.replace("<body>", '<body style=\'quotes: "丙" "丁"\'>')
    chars = required(synth.build_epub({"OEBPS/Fonts/st-all.ttf": PLACEHOLDER}, chapter=chapter, css=css)).chars
    assert {"甲", "丙", "丁"} <= set(chars)
    assert not set("思源宋体") & set(chars)
    assert "8" not in chars


def test_complete_font_passes(tmp_path):
    epub = make_epub()
    font = font_covering(required(epub))
    code, report = run_cli(tmp_path, make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font}), "--font", "OEBPS/Fonts/st-all.ttf")
    assert code == 0, report
    assert report["ok"] and report["fonts"][0]["missing"] == []


def test_missing_char_is_reported_with_location(tmp_path):
    font = font_covering(required(make_epub()), drop="“題")
    code, report = run_cli(tmp_path, make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font}), "--font", "OEBPS/Fonts/st-all.ttf")
    assert code == 1
    missing = {m["char"]: m for m in report["fonts"][0]["missing"]}
    assert missing["U+201C “"]["first"] == "css"


def test_against_reports_coverage_lost_from_full_font(tmp_path):
    source = make_epub()
    harvest = required(source)
    full = tmp_path / "full.epub"
    full.write_bytes(make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font_covering(harvest)}))
    candidate = make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font_covering(harvest, drop="“")})

    code, report = run_cli(tmp_path, candidate, "--against", str(full))

    assert code == 1
    assert report["mode"] == "against" and report["against"]["sha256"]
    font = report["fonts"][0]
    assert not font["coverageOk"] and not font["ok"]
    assert font["against"]["regressions"] == [{
        "kind": "missing",
        "char": "U+201C “",
        "count": font["missing"][0]["count"],
        "first": font["missing"][0]["first"],
    }]


def test_against_allows_a_gap_already_present_in_full(tmp_path):
    harvest = required(make_epub())
    missing_font = font_covering(harvest, drop="“")
    full = tmp_path / "full.epub"
    full.write_bytes(make_epub(fonts={"OEBPS/Fonts/st-all.ttf": missing_font}))
    candidate = make_epub(fonts={"OEBPS/Fonts/st-all.ttf": missing_font})

    code, report = run_cli(tmp_path, candidate, "--against", str(full))

    assert code == 0 and report["ok"]
    font = report["fonts"][0]
    assert not font["coverageOk"]
    assert font["missing"] and font["against"]["regressions"] == []


@pytest.mark.parametrize("candidate_font,kind", [
    ("no-ink", "noInk"),
    ("no-uvs", "missingSequences"),
])
def test_against_detects_outline_and_variation_sequence_regressions(tmp_path, candidate_font, kind):
    harvest = required(make_epub())
    full = tmp_path / "full.epub"
    full.write_bytes(make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font_covering(harvest)}))
    if candidate_font == "no-ink":
        degraded = font_covering(harvest, empty="中")
    else:
        degraded = font_covering(harvest, uvs=False)
    candidate = make_epub(fonts={"OEBPS/Fonts/st-all.ttf": degraded})

    code, report = run_cli(tmp_path, candidate, "--against", str(full))

    assert code == 1
    assert any(item["kind"] == kind for item in report["fonts"][0]["against"]["regressions"])


def test_against_rejects_different_font_manifest_paths(tmp_path, capsys):
    full = tmp_path / "full.epub"
    full.write_bytes(make_epub(fonts={"OEBPS/Fonts/original.ttf": PLACEHOLDER}))

    code, report = run_cli(tmp_path, make_epub(), "--against", str(full))

    assert code == 2 and report is None
    assert "font manifest paths differ" in capsys.readouterr().err


def test_against_rejects_external_font_files(tmp_path, capsys):
    full = tmp_path / "full.epub"
    full.write_bytes(make_epub())

    code, report = run_cli(tmp_path, make_epub(), "--against", str(full), "--font-file", "master.ttf")

    assert code == 2 and report is None
    assert "cannot be used with --against" in capsys.readouterr().err


def test_glyph_without_outline_is_reported(tmp_path):
    font = font_covering(required(make_epub()), empty="中")
    code, report = run_cli(tmp_path, make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font}), "--font", "OEBPS/Fonts/st-all.ttf")
    assert code == 1
    assert [m["char"] for m in report["fonts"][0]["noInk"]] == ["U+4E2D 中"]


def test_missing_variation_sequence_is_reported(tmp_path):
    font = font_covering(required(make_epub()), uvs=False)
    code, report = run_cli(tmp_path, make_epub(fonts={"OEBPS/Fonts/st-all.ttf": font}), "--font", "OEBPS/Fonts/st-all.ttf")
    assert code == 1
    assert report["fonts"][0]["missingSequences"][0]["sequence"] == "U+4E2D U+E0100"


def test_chars_file_and_external_font(tmp_path):
    (tmp_path / "rare.txt").write_text("龘中\n", encoding="utf-8")
    (tmp_path / "rare.ttf").write_bytes(synth.build_font_for("中"))
    code, report = run_cli(tmp_path, make_epub(), "--font-file", str(tmp_path / "rare.ttf"),
                           "--chars-file", str(tmp_path / "rare.txt"))
    assert code == 1
    assert report["requiredChars"] == 2
    assert [m["char"] for m in report["fonts"][0]["missing"]] == ["U+9F98 龘"]


@pytest.mark.parametrize("args,kwargs,message", [
    (("--font", "OEBPS/Fonts/none.ttf"), {}, "not a font in the manifest"),
    (("--font", "OEBPS/Fonts/st-all.ttf"), {"encrypted": ("OEBPS/Fonts/st-all.ttf",)}, "obfuscated"),
])
def test_usage_errors(tmp_path, capsys, args, kwargs, message):
    code, _ = run_cli(tmp_path, make_epub(**kwargs), *args)
    assert code == 2 and message in capsys.readouterr().err


@pytest.mark.parametrize(("epub_data", "args", "message"), [
    (b"not a ZIP archive", (), "File is not a zip file"),
    (None, ("--font-file", "missing.ttf"), "missing.ttf"),
    (None, ("--chars-file", "missing.txt"), "missing.txt"),
    ("invalid-utf8-chars-file", ("--chars-file", "bad.txt"), "utf-8"),
    ("malformed-encryption-xml", (), None),
])
def test_input_errors_are_user_facing_exit_two(tmp_path, capsys, epub_data, args, message):
    cli_args = args
    if epub_data == "invalid-utf8-chars-file":
        (tmp_path / "bad.txt").write_bytes(b"\xff\xfe")
        epub = make_epub()
        cli_args = ("--chars-file", str(tmp_path / "bad.txt"))
    elif epub_data == "malformed-encryption-xml":
        epub = replace_epub_entry(
            make_epub(encrypted=("OEBPS/Fonts/st-all.ttf",)),
            "META-INF/encryption.xml",
            b"<encryption",
        )
    else:
        epub = make_epub() if epub_data is None else epub_data
    code, _ = run_cli(tmp_path, epub, *cli_args)
    stderr = capsys.readouterr().err
    assert code == 2 and (message is None or message in stderr)
    assert stderr.startswith("error:")
    assert "Traceback" not in stderr


def test_default_checks_every_manifest_font(tmp_path):
    code, report = run_cli(tmp_path, make_epub())
    assert [font["font"] for font in report["fonts"]] == ["OEBPS/Fonts/st-all.ttf"]


def test_synthetic_epub_contains_manifested_image_resource():
    with zipfile.ZipFile(io.BytesIO(synth.build_epub({}))) as archive:
        opf = archive.read("OEBPS/content.opf").decode("utf-8")
        assert 'href="Images/x.png" media-type="image/png"' in opf
        assert archive.read("OEBPS/Images/x.png").startswith(b"\x89PNG\r\n\x1a\n")


def test_is_independent_of_the_subset_code():
    source = Path(check.__file__).read_text(encoding="utf-8")
    assert "epubtext" not in source.split('"""', 2)[2] and "fontops" not in source.split('"""', 2)[2]


def test_agrees_with_subset_tool(tmp_path):
    """Two independent collectors: every char check finds missing must be in the subset report's notInMaster."""
    book = tmp_path / "book.epub"
    book.write_bytes(make_epub(fonts={"OEBPS/Fonts/st-all.ttf": synth.build_glyf_font(variable=True)}))
    config = tmp_path / "fonts.json"
    config.write_text(json.dumps({"version": 1, "fonts": [
        {"target": "OEBPS/Fonts/st-all.ttf", "variation": {"mode": "instance", "axes": {"wght": 400}}},
    ]}), encoding="utf-8")
    candidate = tmp_path / "candidate.epub"
    assert subset.main([str(book), "--config", str(config), "--out", str(candidate)]) == 0
    subset_report = json.loads(subset.report_path(candidate).read_text(encoding="utf-8"))
    not_in_master = set(subset_report["fonts"][0]["notInMaster"])
    code = check.main([str(tmp_path / "candidate.epub"), "--font", "OEBPS/Fonts/st-all.ttf",
                           "--json", str(tmp_path / "check.json")])
    check_report = json.loads((tmp_path / "check.json").read_text(encoding="utf-8"))
    assert code == 1   # the tiny synthetic master cannot cover the whole book
    missing = {m["char"] for m in check_report["fonts"][0]["missing"]}
    assert missing and missing <= not_in_master
    assert check_report["fonts"][0]["noInk"] == [] and check_report["fonts"][0]["missingSequences"] == []


@pytest.mark.parametrize(("style", "markers"), [
    ("cjk-decimal", "〇一二三四五六七八九、"),
    ("cjk-ideographic", "零一二三四五六七八九十百千万、"),
    ("simp-chinese-informal", "零一二三四五六七八九十百千万、"),
    ("trad-chinese-informal", "零一二三四五六七八九十百千萬、"),
])
def test_generated_cjk_list_markers_survive_font_subsetting(tmp_path, style, markers):
    chapter = CHAPTER_DTD.replace('<ol class="n">', '<ol class="n" start="10000">')
    css = CSS.replace("list-style-type: cjk-decimal;", f"list-style-type: {style};")
    source_without_font = make_epub(chapter=chapter, css=css)
    source_harvest = required(source_without_font)
    assert set(markers) <= set(source_harvest.chars)

    source_font = font_covering(source_harvest)
    source = make_epub(fonts={"OEBPS/Fonts/st-all.ttf": source_font}, chapter=chapter, css=css)
    source_path = tmp_path / "source.epub"
    source_path.write_bytes(source)

    before_code, before_report = run_cli(
        tmp_path, source, "--font", "OEBPS/Fonts/st-all.ttf"
    )
    assert before_code == 0, before_report
    assert before_report["fonts"][0]["missing"] == []

    candidate = tmp_path / "candidate.epub"
    assert subset.main([str(source_path), "--out", str(candidate)]) == 0
    assert source_path.read_bytes() == source
    subset_report = json.loads(subset.report_path(candidate).read_text(encoding="utf-8"))
    assert subset_report["ok"]

    after_code, after_report = run_cli(
        tmp_path, candidate.read_bytes(), "--font", "OEBPS/Fonts/st-all.ttf"
    )
    assert after_code == 0, after_report
    assert after_report["fonts"][0]["missing"] == []

    with zipfile.ZipFile(source_path) as before, zipfile.ZipFile(candidate) as after:
        assert [item.filename for item in before.infolist()] == [item.filename for item in after.infolist()]
        for item in before.infolist():
            if item.filename != "OEBPS/Fonts/st-all.ttf":
                assert before.read(item) == after.read(item), item.filename
        output_font = TTFont(io.BytesIO(after.read("OEBPS/Fonts/st-all.ttf")))
        output_cmap = output_font.getBestCmap() or {}
        assert {ord(ch) for ch in markers} <= set(output_cmap)
