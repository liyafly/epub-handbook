#!/usr/bin/env python3
"""Regression test for epub3_oneclick_converter.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub3_oneclick_converter import convert_epub  # noqa: E402


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def write_legacy_epub(path: Path) -> None:
  files = {
    "META-INF/container.xml": '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
''',
    "OEBPS/content.opf": '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier id="book-id">urn:uuid:test-oneclick</dc:identifier>
    <dc:title>Oneclick Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:date opf:event="publication">2026-01-01</dc:date>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="cover-page"/>
    <itemref idref="chapter"/>
  </spine>
  <guide>
    <reference type="cover" title="Cover" href="../Text/cover.xhtml"/>
  </guide>
</package>
''',
    "OEBPS/toc.ncx": '''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:test-oneclick"/></head>
  <docTitle><text>Oneclick Fixture</text></docTitle>
  <navMap>
    <navPoint id="navPoint-1" playOrder="1">
      <navLabel><text>第一章</text></navLabel>
      <content src="Text/chapter.xhtml"#c1/>
    </navPoint>
  </navMap>
</ncx>
''',
    "OEBPS/Text/cover.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
  <head><title>Cover</title></head>
  <body><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"></svg></body>
</html>
''',
    "OEBPS/Text/chapter.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
  <head>
    <title>第一章</title>
    <meta http-equiv="Content-Type" content="application/xhtml+xml; charset=utf-8"/>
    <link rel="stylesheet" type="text/css" href="../Styles/main.css"/>
  </head>
  <body>
    <h1 id="c1">第一章</h1>
    <p>正文<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a>继续。</p>
    <hr/>
    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>
  </body>
</html>
''',
    "OEBPS/Styles/main.css": '''body {
  font-family: "cnepub", serif;
  line-height: 1.4;
}
''',
    "OEBPS/Images/cover.jpg": b"jpeg",
  }
  with zipfile.ZipFile(path, "w") as zf:
    for name, data in files.items():
      zf.writestr(name, data.encode("utf-8") if isinstance(data, str) else data)
    zf.writestr("mimetype", b"application/epub+zip")


def main() -> int:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy.epub"
    output = Path(raw) / "converted.epub"
    write_legacy_epub(source)
    report = convert_epub(source, output)

    assert report.plain_notes_converted == 1, report
    assert report.nav_entries == 1, report
    assert report.stylesheet_links_added == 2, report

    with zipfile.ZipFile(output) as zf:
      infos = zf.infolist()
      assert infos[0].filename == "mimetype"
      assert infos[0].compress_type == zipfile.ZIP_STORED
      assert zf.read("mimetype") == b"application/epub+zip"

      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      assert opf.attrib.get("version") == "3.0"
      items = opf.findall("opf:manifest/opf:item", OPF_NS)
      navs = [item for item in items if "nav" in (item.attrib.get("properties") or "").split()]
      assert len(navs) == 1
      assert any(item.attrib.get("href") == "Styles/epub3-enhancements.css" for item in items)
      assert any(item.attrib.get("href") == "Images/note.png" for item in items)
      cover = next(item for item in items if item.attrib.get("id") == "cover-img")
      assert "cover-image" in (cover.attrib.get("properties") or "").split()
      cover_page = next(item for item in items if item.attrib.get("id") == "cover-page")
      assert "svg" in (cover_page.attrib.get("properties") or "").split()
      assert b'href="Text/cover.xhtml"' in zf.read("OEBPS/content.opf")

      assert b'src="Text/chapter.xhtml#c1"' in zf.read("OEBPS/toc.ncx")
      assert "OEBPS/nav.xhtml" in zf.namelist()
      assert b'href="Text/cover.xhtml"' in zf.read("OEBPS/nav.xhtml")
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      assert 'xmlns:epub="http://www.idpf.org/2007/ops"' in chapter
      assert 'href="../Styles/epub3-enhancements.css"' in chapter
      assert 'class="noteref-icon" epub:type="noteref" role="doc-noteref"' in chapter
      assert 'class="footnote-list"' in chapter
      assert 'role="doc-backlink"' in chapter
      assert "注释正文保留。" in chapter

  print("epub3 oneclick converter tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
