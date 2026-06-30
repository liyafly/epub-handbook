#!/usr/bin/env python3
"""Regression tests for EPUB/source content role analysis."""

from __future__ import annotations

import json
import subprocess
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))

import epub_content_analysis as A  # noqa: E402
from test_support.epub_fixture import write_epub as write_fixture_epub  # noqa: E402


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_content_analyzer.py"


def xhtml(body: str, body_class: str = "", language: str = "zh-CN") -> str:
  cls = f' class="{body_class}"' if body_class else ""
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="{language}" xml:lang="{language}">
  <head><title>Test</title></head>
  <body{cls}>{body}</body>
</html>'''


def make_epub(path: Path, chapter: str, encrypted: bool = False, extra_chapter: str | None = None) -> None:
  extra_manifest = '<item id="c2" href="Text/c2.xhtml" media-type="application/xhtml+xml"/>' if extra_chapter is not None else ""
  extra_spine = '<itemref idref="c2"/>' if extra_chapter is not None else ""
  files = {
    "META-INF/container.xml": '''<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>''',
    "OEBPS/package.opf": f'''<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/>{extra_manifest}<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/>{extra_spine}</spine></package>''',
    "OEBPS/Text/c1.xhtml": chapter,
    "OEBPS/nav.xhtml": xhtml('<nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">目录标题</a></li></ol></nav>'),
  }
  if encrypted:
    files["META-INF/encryption.xml"] = "<encryption/>"
  if extra_chapter is not None:
    files["OEBPS/Text/c2.xhtml"] = extra_chapter
  write_fixture_epub(path, files)


class ContentAnalysisTests(unittest.TestCase):
  def test_xhtml_structure_wins_over_misleading_text(self) -> None:
    blocks = A.analyze_xhtml(
      "Text/ch01.xhtml",
      xhtml('<h1>正文一样长也仍是标题</h1><figcaption>第一章</figcaption><p>这是普通正文段落，长度足以稳定识别为正文。</p>'),
    )
    self.assertEqual([b["primary_role"] for b in blocks], ["heading", "caption", "body"])
    self.assertEqual(blocks[0]["typography"]["font_role"], "ht")
    self.assertEqual(blocks[1]["locator"], "/html[1]/body[1]/figcaption[1]")
    self.assertEqual(blocks[2]["previous_tag"], "figcaption")

  def test_title_page_heading_and_subtitle_classes_override_generic_heading(self) -> None:
    blocks = A.analyze_xhtml(
      "Text/title.xhtml",
      xhtml(
        '<section class="title-page" epub:type="titlepage">'
        '<h1>书名</h1><h2 class="subtitle">副标题</h2>'
        '</section>'
      ),
    )
    self.assertEqual([block["primary_role"] for block in blocks], ["title", "subtitle"])
    self.assertEqual(blocks[0]["typography"]["text_align"], "center")
    self.assertEqual(blocks[0]["typography"]["line_height"], "1.2")
    self.assertEqual(blocks[1]["typography"]["font_role"], "kt")

  def test_explicit_chinese_roles_and_font_advice(self) -> None:
    blocks = A.analyze_xhtml(
      "Text/roles.xhtml",
      xhtml(
        '<p class="subtitle">副标题</p>'
        '<blockquote>引用内容</blockquote>'
        '<p class="epigraph">题记内容</p>'
        '<div class="poem"><p>床前明月光</p><p>疑是地上霜</p></div>'
        '<section class="letter"><p>亲爱的朋友：</p></section>'
        '<aside epub:type="footnote"><p>注释正文</p></aside>'
        '<pre><code>print(&quot;ok&quot;)</code></pre>'
        '<p class="classical-text">学而时习之，不亦说乎。</p>'
        '<p class="modern-text">学习并经常温习，是令人愉快的。</p>'
        '<hr class="scene-break"/>'
      ),
    )
    roles = {block["primary_role"] for block in blocks}
    self.assertTrue({
      "subtitle", "quotation", "epigraph", "verse", "letter", "note",
      "code", "classical", "modern-translation", "scene-break",
    }.issubset(roles), roles)
    by_role = {block["primary_role"]: block for block in blocks}
    self.assertEqual(by_role["epigraph"]["typography"]["font_role"], "kt")
    self.assertEqual(by_role["code"]["typography"]["font_role"], "mono")
    self.assertEqual(by_role["scene-break"]["typography"]["text_align"], "center")

  def test_dialogue_and_ambiguous_short_chinese_remain_reviewable(self) -> None:
    blocks = A.analyze_xhtml(
      "Text/ambiguous.xhtml",
      xhtml('<p>“你明天还来吗？”她问。</p><p>春风又绿江南岸</p>'),
    )
    self.assertEqual(blocks[0]["primary_role"], "dialogue")
    self.assertTrue(blocks[0]["review_required"])
    self.assertEqual(blocks[1]["primary_role"], "unknown")
    self.assertTrue(blocks[1]["review_required"])
    self.assertIn("body", blocks[1]["candidate_roles"])
    self.assertIn("verse", blocks[1]["candidate_roles"])

  def test_features_cover_mixed_chinese_latin_and_punctuation(self) -> None:
    block = A.analyze_xhtml(
      "Text/mixed.xhtml",
      xhtml('<p>EPUB 3.3 与中文混排：Hello，世界！2026。</p>'),
    )[0]
    features = block["features"]
    self.assertGreater(features["cjk_count"], 0)
    self.assertGreater(features["latin_count"], 0)
    self.assertGreater(features["digit_count"], 0)
    self.assertGreater(features["punctuation_count"], 0)

  def test_unpunctuated_classical_text_is_not_forced_to_body(self) -> None:
    block = A.analyze_xhtml(
      "Text/classical.xhtml",
      xhtml('<p>天地玄黄宇宙洪荒日月盈昃辰宿列张寒来暑往秋收冬藏</p>'),
    )[0]
    self.assertEqual(block["primary_role"], "unknown")
    self.assertIn("classical", block["candidate_roles"])
    self.assertTrue(block["review_required"])

  def test_dash_dialogue_and_br_separated_verse_are_candidates(self) -> None:
    blocks = A.analyze_xhtml(
      "Text/heuristics.xhtml",
      xhtml('<p>——你终于来了？</p><p>床前明月光<br/>疑是地上霜<br/>举头望明月</p>'),
    )
    self.assertEqual(blocks[0]["primary_role"], "dialogue")
    self.assertEqual(blocks[1]["primary_role"], "verse")
    self.assertTrue(blocks[1]["review_required"])
    self.assertEqual(blocks[1]["features"]["line_count"], 3)

  def test_loose_html_and_traditional_language_are_supported(self) -> None:
    blocks = A.analyze_source(
      "chapter.html",
      '<html lang="zh-Hant"><body><h1>第一章<p>這是一段沒有閉合標籤的繁體中文正文內容。',
    )
    self.assertEqual([block["primary_role"] for block in blocks], ["heading", "body"])
    self.assertTrue(all(block["language"] == "zh-Hant" for block in blocks))

  def test_markdown_and_plain_text_inputs(self) -> None:
    markdown = A.analyze_source("chapter.md", "# 第一章\n\n> 引用内容\n\n这是普通正文段落，长度足以稳定识别。")
    self.assertEqual([b["primary_role"] for b in markdown[:2]], ["heading", "quotation"])
    plain = A.analyze_source("chapter.txt", "第一段普通正文，长度足以识别。\n\n第二段普通正文，继续叙述内容。")
    self.assertEqual(len(plain), 2)
    self.assertTrue(all(b["source"] == "chapter.txt" for b in plain))

  def test_markdown_list_and_code_roles(self) -> None:
    blocks = A.analyze_source("notes.md", "- 第一项\n\n```python\nprint('ok')\n```\n")
    self.assertEqual([block["primary_role"] for block in blocks], ["list", "code"])

  def test_default_report_is_private_and_snippets_are_opt_in(self) -> None:
    source = "这是完整私有正文，默认报告不得直接保存这一段文本。"
    private = A.analyze_source_report("private.txt", source)
    encoded = json.dumps(private, ensure_ascii=False)
    self.assertNotIn(source, encoded)
    self.assertIn("text_sha256", private["blocks"][0])
    self.assertNotIn("snippet", private["blocks"][0])
    local = A.analyze_source_report("private.txt", source, include_snippets=True)
    self.assertEqual(local["blocks"][0]["snippet"], source)

  def test_epub_uses_spine_content_and_stops_on_encryption(self) -> None:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      epub = root / "book.epub"
      make_epub(epub, xhtml('<h1>第一章</h1><p>这是正文内容，长度足以识别。</p>'))
      report = A.analyze_path(epub)
      self.assertEqual(report["summary"]["blocks"], 2)
      self.assertNotIn("目录标题", json.dumps(report, ensure_ascii=False))
      encrypted = root / "encrypted.epub"
      make_epub(encrypted, xhtml("<p>正文</p>"), encrypted=True)
      with self.assertRaisesRegex(A.ContentAnalysisError, "encryption"):
        A.analyze_path(encrypted)

  def test_epub_records_bad_xhtml_and_continues_other_spine_documents(self) -> None:
    with TemporaryDirectory() as raw:
      epub = Path(raw) / "partial.epub"
      make_epub(
        epub,
        '<html xmlns="http://www.w3.org/1999/xhtml"><body><p>没有闭合的段落</body></html>',
        extra_chapter=xhtml('<p>这是仍然可以分析的正文段落，长度足以识别。</p>'),
      )
      report = A.analyze_path(epub)
      self.assertEqual(report["status"], "warn")
      self.assertEqual(report["summary"]["blocks"], 1)
      self.assertEqual(report["summary"]["file_errors"], 1)
      self.assertEqual(report["errors"][0]["source"], "OEBPS/Text/c1.xhtml")

  def test_cli_json_markdown_and_read_only(self) -> None:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      source = root / "chapter.txt"
      source.write_text("这是普通正文段落，长度足以稳定识别为正文。", encoding="utf-8")
      before = source.read_bytes()
      result = subprocess.run(
        [sys.executable, str(SCRIPT), str(source), "--format", "json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
      )
      self.assertEqual(result.returncode, 0, result.stderr)
      data = json.loads(result.stdout)
      self.assertEqual(data["capability"], "epub.text.content.analyze")
      self.assertEqual(data["summary"]["blocks"], 1)
      self.assertEqual(source.read_bytes(), before)
      md = subprocess.run(
        [sys.executable, str(SCRIPT), str(source), "--format", "md", "--include-snippets"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
      )
      self.assertEqual(md.returncode, 0, md.stderr)
      self.assertIn("## 结构角色", md.stdout)
      self.assertIn("这是普通正文", md.stdout)

  def test_cli_returns_nonzero_when_no_spine_document_can_be_analyzed(self) -> None:
    with TemporaryDirectory() as raw:
      epub = Path(raw) / "invalid-content.epub"
      make_epub(epub, '<html xmlns="http://www.w3.org/1999/xhtml"><body><p>未闭合</body></html>')
      result = subprocess.run(
        [sys.executable, str(SCRIPT), str(epub), "--format", "json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
      )
      self.assertEqual(result.returncode, 1, result.stderr)
      self.assertEqual(json.loads(result.stdout)["status"], "fail")


if __name__ == "__main__":
  unittest.main()
