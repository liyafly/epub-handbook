"""Tests for the independent completeness check. Run: uv run pytest -q"""

from __future__ import annotations

import io
import json
import zipfile
from pathlib import Path

import pytest

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


def make_epub(chapter: str = CHAPTER_DTD, fonts: dict | None = None, **kwargs) -> bytes:
    return synth.build_epub(fonts or {"OEBPS/Fonts/st-all.ttf": PLACEHOLDER}, chapter=chapter, css=CSS, **kwargs)


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


def test_default_checks_every_manifest_font(tmp_path):
    code, report = run_cli(tmp_path, make_epub())
    assert [font["font"] for font in report["fonts"]] == ["OEBPS/Fonts/st-all.ttf"]


def test_is_independent_of_the_subset_code():
    source = Path(check.__file__).read_text(encoding="utf-8")
    assert "epubtext" not in source.split('"""', 2)[2] and "fontops" not in source.split('"""', 2)[2]


def test_agrees_with_subset_tool(tmp_path):
    """Two independent collectors: every char check finds missing must be in the subset report's notInMaster."""
    book = tmp_path / "book.epub"
    book.write_bytes(make_epub())
    (tmp_path / "master.ttf").write_bytes(synth.build_glyf_font(variable=True))
    config = tmp_path / "fonts.json"
    config.write_text(json.dumps({"version": 1, "fonts": [
        {"target": "OEBPS/Fonts/st-all.ttf", "master": "master.ttf", "variation": {"mode": "instance", "axes": {"wght": 400}}},
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
