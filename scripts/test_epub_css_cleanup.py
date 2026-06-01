#!/usr/bin/env python3
"""Regression test for epub_css_cleanup.py."""

from __future__ import annotations

import re
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub_css_cleanup import clean_epub_css  # noqa: E402


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


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
    <dc:identifier id="book-id">urn:uuid:test-css-cleanup</dc:identifier>
    <dc:title>CSS Cleanup Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="c1" href="Text/chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="Text/chapter2.xhtml" media-type="application/xhtml+xml"/>
    <item id="c3" href="Text/chapter3.xhtml" media-type="application/xhtml+xml"/>
    <item id="toc1" href="Text/toc1.xhtml" media-type="application/xhtml+xml"/>
    <item id="toc2" href="Text/toc2.xhtml" media-type="application/xhtml+xml"/>
    <item id="s2" href="Styles/style0002.css" media-type="text/css"/>
    <item id="s3" href="Styles/style0003.css" media-type="text/css"/>
    <item id="s4" href="Styles/style0004.css" media-type="text/css"/>
    <item id="s5" href="Styles/style0005.css" media-type="text/css"/>
    <item id="s6" href="Styles/style0006.css" media-type="text/css"/>
    <item id="component" href="Styles/component.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
    <itemref idref="c3"/>
    <itemref idref="toc1"/>
    <itemref idref="toc2"/>
  </spine>
</package>
''',
    "OEBPS/Text/chapter1.xhtml": chapter("style0002.css", extra_css="component.css"),
    "OEBPS/Text/chapter2.xhtml": chapter("style0004.css", extra_css="component.css"),
    "OEBPS/Text/chapter3.xhtml": chapter("style0006.css", extra_css="component.css"),
    "OEBPS/Text/toc1.xhtml": chapter("style0003.css", body="<p class=\"toc\">目录甲</p>"),
    "OEBPS/Text/toc2.xhtml": chapter("style0005.css", body="<p class=\"toc\">目录乙</p>"),
    "OEBPS/Styles/style0002.css": legacy_css("#876c4f"),
    "OEBPS/Styles/style0003.css": ".toc { margin-left: 0; }\n",
    "OEBPS/Styles/style0004.css": legacy_css("#876c4f"),
    "OEBPS/Styles/style0005.css": ".toc { margin-left: 0; }\n",
    "OEBPS/Styles/style0006.css": legacy_css("#3fbbd6"),
    "OEBPS/Styles/component.css": ".component { margin: 0 auto; }\n",
  }
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", b"application/epub+zip", compress_type=zipfile.ZIP_STORED)
    for name, data in files.items():
      zf.writestr(name, data.encode("utf-8") if isinstance(data, str) else data)


def chapter(css_name: str, body: str | None = None, extra_css: str | None = None) -> str:
  body = body or '''<h1>标题</h1>
    <p>正文（补充说明）继续。</p>
    <aside epub:type="footnote" role="doc-footnote"><p>脚注（不要二次缩小）</p></aside>'''
  extra_link = f'    <link href="../Styles/{extra_css}" type="text/css" rel="stylesheet"/>\n' if extra_css else ""
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head>
    <title>Fixture</title>
    <link href="../Styles/{css_name}" type="text/css" rel="stylesheet"/>
{extra_link}  </head>
  <body>
    {body}
  </body>
</html>
'''


def legacy_css(color: str) -> str:
  return f'''————————————————标题————————————————
h1 {{
  color: {color};
  font-family: "SimHei";
}}
body {{
  font-family: "cnepub",serif;
}}
.part-text {{
  font-family: "STKaiti"
}}
'''


def visible_text(data: str) -> str:
  return re.sub(r"\s+", "", re.sub(r"<[^>]+>", "", data))


def main() -> int:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "source.epub"
    output = Path(raw) / "cleaned.epub"
    write_epub(source)
    report = clean_epub_css(
      source,
      output,
      merge_scoped_local_css=True,
    )

    assert report.css_files_before == 6, report
    assert report.factored_stylesheets == 3, report
    assert report.duplicate_stylesheets_removed == 1, report
    assert report.scoped_local_stylesheets_merged == 2, report
    assert report.scope_classes_added == 3, report

    with zipfile.ZipFile(output) as zf:
      names = set(zf.namelist())
      assert "OEBPS/Styles/clean-shared-01.css" in names
      assert "OEBPS/Styles/clean-scoped-local.css" in names
      assert "OEBPS/Styles/clean-override-style0006.css" not in names
      assert "OEBPS/Styles/style0003.css" not in names
      assert "OEBPS/Styles/style0005.css" not in names
      assert "OEBPS/Styles/component.css" in names

      shared = zf.read("OEBPS/Styles/clean-shared-01.css").decode("utf-8")
      assert '"Songti SC", "SimSun", "Noto Serif CJK SC", serif' in shared
      assert '"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif' in shared
      assert '"Kaiti SC", "STKaiti", "KaiTi", serif' in shared
      assert "————————————————" not in shared

      scoped = zf.read("OEBPS/Styles/clean-scoped-local.css").decode("utf-8")
      assert "#3fbbd6" in scoped
      assert "body.css-local-01 h1" in scoped
      assert "body.css-local-02 .toc" in scoped

      chapter1 = zf.read("OEBPS/Text/chapter1.xhtml").decode("utf-8")
      chapter3 = zf.read("OEBPS/Text/chapter3.xhtml").decode("utf-8")
      toc1 = zf.read("OEBPS/Text/toc1.xhtml").decode("utf-8")
      assert 'href="../Styles/clean-shared-01.css"' in chapter1
      assert 'href="../Styles/component.css"' in chapter1
      assert 'href="../Styles/clean-scoped-local.css"' in chapter3
      assert 'class="css-local-01"' in chapter3
      assert 'href="../Styles/clean-scoped-local.css"' in toc1
      assert 'class="css-local-02"' in toc1
      assert "正文（补充说明）继续。" in chapter1
      assert visible_text(chapter1) == visible_text(chapter("style0002.css", extra_css="component.css"))

      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      css_hrefs = {
        item.attrib["href"]
        for item in opf.findall("opf:manifest/opf:item", OPF_NS)
        if item.attrib.get("media-type") == "text/css"
      }
      assert "Styles/clean-shared-01.css" in css_hrefs
      assert "Styles/clean-scoped-local.css" in css_hrefs
      assert "Styles/clean-override-style0006.css" not in css_hrefs
      assert "Styles/style0003.css" not in css_hrefs
      assert "Styles/component.css" in css_hrefs
      assert "Styles/style0005.css" not in css_hrefs

    second_output = Path(raw) / "cleaned-again.epub"
    second_report = clean_epub_css(output, second_output, merge_scoped_local_css=True)
    assert second_report.css_files_before == 3, second_report
    assert second_report.css_files_after == 3, second_report
    assert second_report.factored_stylesheets == 0, second_report
    assert second_report.scoped_local_stylesheets_merged == 0, second_report
    with zipfile.ZipFile(second_output) as zf:
      assert "OEBPS/Styles/clean-shared-01-2.css" not in zf.namelist()

  print("epub css cleanup tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
