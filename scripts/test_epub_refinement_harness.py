#!/usr/bin/env python3
"""Regression tests for epub_refinement_harness.py."""

from __future__ import annotations

import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_refinement_harness.py"


def write_fixture(path: Path) -> None:
  opf = '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:refinement</dc:identifier><dc:title>Refine</dc:title><dc:language>zh-CN</dc:language></metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="webp" href="Images/risky.webp" media-type="image/webp"/>
    <item id="font" href="Fonts/book.ttf" media-type="font/ttf"/>
  </manifest>
  <spine toc="ncx"><itemref idref="c1"/></spine>
</package>'''
  xhtml = '''<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN"><head><title>One</title></head><body class="page-vrl"><p><ruby>漢<rt>かん</rt></ruby><a epub:type="noteref" href="#n1">注</a></p><aside epub:type="footnote"><ol><li id="n1">note</li></ol></aside></body></html>'''
  css = '''@font-face { font-family: BookFont; src: url('../Fonts/book.ttf'); }
body { font-family: BookFont, Songti SC, serif, sans-serif, fantasy; line-height: 1.7; writing-mode: vertical-rl; }
'''
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", "application/epub+zip")
    zf.writestr("META-INF/container.xml", '<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
    zf.writestr("OEBPS/package.opf", opf)
    zf.writestr("OEBPS/Text/c1.xhtml", xhtml)
    zf.writestr("OEBPS/Styles/main.css", css)
    zf.writestr("OEBPS/Images/risky.webp", b"webp")
    zf.writestr("OEBPS/Fonts/book.ttf", b"font")
    zf.writestr("OEBPS/toc.ncx", '<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap/></ncx>')


def main() -> int:
  with TemporaryDirectory() as raw:
    epub = Path(raw) / "refine.epub"
    write_fixture(epub)
    result = subprocess.run(
      [sys.executable, str(SCRIPT), str(epub), "--format", "json"],
      cwd=ROOT,
      check=False,
      text=True,
      stdout=subprocess.PIPE,
      stderr=subprocess.PIPE,
    )
    data = json.loads(result.stdout)
    facts = data.get("facts", {})
    if facts.get("noteref_count") != 1 or facts.get("ruby_count") != 1 or facts.get("vertical_markers", 0) < 1:
      raise AssertionError(f"refinement facts did not count note/ruby/vertical markers: {facts}")
    if not facts.get("risky_images") or not facts.get("css_font_urls"):
      raise AssertionError(f"refinement facts missed risky image or font URL: {facts}")
    rec_ids = {item.get("id") for item in data.get("recommendations", [])}
    expected = {"preflight", "epub3-migration", "popup-notes", "typography-fonts", "images", "ruby-vertical", "redline-and-diff"}
    missing = expected - rec_ids
    if missing:
      raise AssertionError(f"refinement recommendations missing {sorted(missing)}: {data}")

  print("epub refinement harness tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
