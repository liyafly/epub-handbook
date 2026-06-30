#!/usr/bin/env python3
"""Regression tests for epub_anthology_refinement.py."""

from __future__ import annotations

import re
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub_anthology_refinement import RefinementError, refine_anthology  # noqa: E402
from test_support.epub_fixture import write_epub as write_fixture_epub  # noqa: E402


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def xhtml(title: str, body: str) -> str:
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head>
    <title>{title}</title>
    <link href="../Styles/base.css" type="text/css" rel="stylesheet"/>
  </head>
  <body>
    {body}
  </body>
</html>
'''


def copyright_page(title: str) -> str:
  return xhtml(
    "版权信息",
    f'''<p class="cp">版权信息</p>
    <ul class="list">
      <li class="i">书名：{title}</li>
      <li class="i">作者：测试作者</li>
      <li class="i">主页：<a href="https://example.com">示例</a></li>
    </ul>''',
  )


def write_epub(path: Path) -> None:
  files = {
    "META-INF/container.xml": '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
''',
    "OEBPS/content.opf": '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:test-anthology</dc:identifier>
    <dc:title>Anthology Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="poster1" href="Text/poster1.xhtml" media-type="application/xhtml+xml"/>
    <item id="copyright1" href="Text/copyright1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="poster2" href="Text/poster2.xhtml" media-type="application/xhtml+xml"/>
    <item id="copyright2" href="Text/copyright2.xhtml" media-type="application/xhtml+xml"/>
    <item id="base" href="Styles/base.css" media-type="text/css"/>
    <item id="image1" href="Images/poster1.jpg" media-type="image/jpeg"/>
    <item id="image2" href="Images/poster2.jpg" media-type="image/jpeg"/>
  </manifest>
  <spine>
    <itemref idref="poster1"/>
    <itemref idref="copyright1"/>
    <itemref idref="chapter"/>
    <itemref idref="poster2"/>
    <itemref idref="copyright2"/>
  </spine>
</package>
''',
    "OEBPS/Text/poster1.xhtml": xhtml("封面", '<p class="center"><img alt="" src="../Images/poster1.jpg"/></p>'),
    "OEBPS/Text/copyright1.xhtml": copyright_page("第一卷"),
    "OEBPS/Text/chapter.xhtml": xhtml("正文", "<p>正文保持不变。</p>"),
    "OEBPS/Text/poster2.xhtml": xhtml("封面", '<p class="center"><img alt="" src="../Images/poster2.jpg"/></p>'),
    "OEBPS/Text/copyright2.xhtml": copyright_page("第二卷"),
    "OEBPS/Styles/base.css": "body { line-height: 1.6; }\n",
    "OEBPS/Images/poster1.jpg": b"jpeg1",
    "OEBPS/Images/poster2.jpg": b"jpeg2",
  }
  write_fixture_epub(path, files)


def visible_text(value: str) -> str:
  return re.sub(r"\s+", "", re.sub(r"<[^>]+>", "", value))


def main() -> int:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    source = root / "source.epub"
    output = root / "refined.epub"
    second_output = root / "refined-again.epub"
    write_epub(source)

    report = refine_anthology(source, output, expect_volumes=2)
    assert report.poster_pages_refined == 2, report
    assert report.copyright_pages_refined == 2, report
    assert report.stylesheets_added == 1, report
    assert report.warnings == [], report

    with zipfile.ZipFile(source) as before, zipfile.ZipFile(output) as after:
      poster = after.read("OEBPS/Text/poster1.xhtml").decode("utf-8")
      copyright_text = after.read("OEBPS/Text/copyright1.xhtml").decode("utf-8")
      chapter = after.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      css = after.read("OEBPS/Styles/anthology-refinement.css").decode("utf-8")
      assert 'class="fullpage poster-bg poster-bg-volume-001"' in poster
      assert '<section class="fullframe" epub:type="chapter">' in poster
      assert 'class="poster-fallback"' in poster
      assert 'href="../Styles/anthology-refinement.css"' in poster
      assert 'class="anthology-copyright-page"' in copyright_text
      assert 'class="copyright-card" epub:type="frontmatter copyright-page"' in copyright_text
      assert 'class="list copyright-meta"' in copyright_text
      assert 'class="i copyright-meta-item"' in copyright_text
      assert 'href="../Styles/anthology-refinement.css"' in copyright_text
      assert chapter == before.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      assert visible_text(copyright_text) == visible_text(before.read("OEBPS/Text/copyright1.xhtml").decode("utf-8"))
      assert "background-size: contain;" in css
      assert "@supports (background-size: contain)" in css
      assert "page-break-inside: avoid;" in css
      assert 'background-image: url("../Images/poster1.jpg");' in css
      assert 'background-image: url("../Images/poster2.jpg");' in css
      assert "background-size: cover" not in css
      assert "position: absolute" not in css
      assert "vh" not in css
      assert "vw" not in css

      opf = ET.fromstring(after.read("OEBPS/content.opf"))
      hrefs = [
        item.attrib.get("href")
        for item in opf.findall("opf:manifest/opf:item", OPF_NS)
        if item.attrib.get("media-type") == "text/css"
      ]
      assert hrefs.count("Styles/anthology-refinement.css") == 1, hrefs

    second_report = refine_anthology(output, second_output, expect_volumes=2)
    assert second_report.stylesheets_added == 0, second_report
    with zipfile.ZipFile(second_output) as zf:
      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      hrefs = [
        item.attrib.get("href")
        for item in opf.findall("opf:manifest/opf:item", OPF_NS)
        if item.attrib.get("media-type") == "text/css"
      ]
      assert hrefs.count("Styles/anthology-refinement.css") == 1, hrefs

    try:
      refine_anthology(source, root / "wrong-count.epub", expect_volumes=3)
    except RefinementError:
      pass
    else:
      raise AssertionError("expected volume count mismatch must stop refinement")

  print("epub anthology refinement tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
