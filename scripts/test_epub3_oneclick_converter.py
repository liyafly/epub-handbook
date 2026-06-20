#!/usr/bin/env python3
"""Regression test for epub3_oneclick_converter.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub3_oneclick_converter as C  # noqa: E402


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def write_legacy_epub(
  path: Path,
  body_class: str = "",
  extra_metadata: str = "",
  chapter_note_markup: str | None = None,
  extra_manifest_items: str = "",
  extra_files: dict[str, bytes | str] | None = None,
  missing_html_language: bool = False,
  minified_chapter: bool = False,
) -> None:
  note_markup = chapter_note_markup or (
    '<p>正文<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a>继续。</p>\n'
    '    <hr/>\n'
    '    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>'
  )
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
{extra_metadata}  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg"/>
{extra_manifest_items}  </manifest>
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
    {note_markup}
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
  if extra_metadata:
    files["OEBPS/content.opf"] = files["OEBPS/content.opf"].format(
      extra_metadata=extra_metadata,
      extra_manifest_items=extra_manifest_items,
    )
  else:
    files["OEBPS/content.opf"] = files["OEBPS/content.opf"].format(
      extra_metadata="",
      extra_manifest_items=extra_manifest_items,
    )
  files["OEBPS/Text/chapter.xhtml"] = files["OEBPS/Text/chapter.xhtml"].format(
    note_markup=note_markup,
  )
  if minified_chapter:
    files["OEBPS/Text/chapter.xhtml"] = (
      '<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html>'
      '<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">'
      '<head><title>第一章</title></head><body><h1 id="c1">第一章</h1>'
      f'{note_markup}</body></html>'
    )
  files.update(extra_files or {})

  if body_class:
    chapter = files["OEBPS/Text/chapter.xhtml"]
    assert "<body>" in chapter
    files["OEBPS/Text/chapter.xhtml"] = chapter.replace("<body>", f'<body class="{body_class}">', 1)

  if missing_html_language:
    for xhtml_path in ("OEBPS/Text/cover.xhtml", "OEBPS/Text/chapter.xhtml"):
      files[xhtml_path] = files[xhtml_path].replace(' xml:lang="zh-CN"', "")

  with zipfile.ZipFile(path, "w") as zf:
    for name, data in files.items():
      zf.writestr(name, data.encode("utf-8") if isinstance(data, str) else data)
    zf.writestr("mimetype", b"application/epub+zip")


def main() -> int:
  inline_only_paragraph_formatting_case()
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy.epub"
    output = Path(raw) / "converted.epub"
    write_legacy_epub(source, minified_chapter=True)
    report = C.convert_epub(source, output)

    assert report.plain_notes_converted == 1, report
    assert report.nav_entries == 1, report
    assert report.stylesheet_links_added == 2, report
    assert report.typography_roles == [
      "type-body",
      "type-title",
      "type-subtitle",
      "type-quote",
      "type-note",
      "type-emphasis",
      "type-meta",
    ], report

    with zipfile.ZipFile(output) as zf:
      infos = zf.infolist()
      assert infos[0].filename == "mimetype"
      assert infos[0].compress_type == zipfile.ZIP_STORED
      assert zf.read("mimetype") == b"application/epub+zip"

      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      assert opf.attrib.get("version") == "3.0"
      metas = opf.findall("opf:metadata/opf:meta", OPF_NS)
      assert not any(
        m.attrib.get("property") == "ibooks:specified-fonts" for m in metas
      ), "free-mode book must not get ibooks:specified-fonts"
      assert "ibooks:" not in opf.attrib.get("prefix", ""), "free-mode book must not get ibooks prefix"
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
      assert ".type-quote" in zf.read("OEBPS/Styles/epub3-enhancements.css").decode("utf-8")
      assert '<sup class="note-marker">' in chapter
      assert 'class="noteref-icon" epub:type="noteref" role="doc-noteref"' in chapter
      assert 'class="footnote-list"' in chapter
      assert 'role="doc-backlink"' in chapter
      assert "注释正文保留。" in chapter
      assert "\n  <head>" in chapter
      assert "\n  <body>" in chapter

  locked_mode_case()
  ibooks_prefix_case()
  custom_image_noteref_case()
  sigil_legacy_notes_case()
  missing_html_language_case()
  non_note_sup_case()
  sigil_partial_section_case()
  missing_html_language_case()
  missing_package_language_case()
  print("epub3 oneclick converter tests ok")
  return 0


def inline_only_paragraph_formatting_case() -> None:
  source = (
    '<?xml version="1.0"?><!DOCTYPE html>'
    '<html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head>'
    '<body><p><span>甲</span><span>乙</span></p></body></html>'
  )
  formatted, changed = C.format_xhtml_multiline(source)
  assert changed
  assert "<p><span>甲</span><span>乙</span></p>" in formatted, formatted


def ibooks_prefix_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-ibooks-version.epub"
    output = Path(raw) / "converted-ibooks-version.epub"
    write_legacy_epub(source, extra_metadata='    <meta property="ibooks:version">1.0</meta>\n')
    report = C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      metas = opf.findall("opf:metadata/opf:meta", OPF_NS)
      assert not any(
        m.attrib.get("property") == "ibooks:specified-fonts" for m in metas
      ), "free-mode book with other ibooks meta must not get ibooks:specified-fonts"
      assert "ibooks:" in opf.attrib.get("prefix", ""), "other ibooks properties must retain ibooks prefix"
    assert "kept existing ibooks:specified-fonts" not in report.metadata_updates, report


def locked_mode_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-locked.epub"
    output = Path(raw) / "converted-locked.epub"
    write_legacy_epub(source, body_class="body-font-locked")
    report = C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      metas = opf.findall("opf:metadata/opf:meta", OPF_NS)
      locked = [m for m in metas if m.attrib.get("property") == "ibooks:specified-fonts"]
      assert len(locked) == 1 and locked[0].text == "true", "locked book must get ibooks:specified-fonts=true"
      assert "ibooks:" in opf.attrib.get("prefix", ""), "locked book must get ibooks prefix"
    assert "added ibooks:specified-fonts (body-font-locked detected)" in report.metadata_updates, report


def custom_image_noteref_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-image-note.epub"
    output = Path(raw) / "converted-image-note.epub"
    write_legacy_epub(
      source,
      chapter_note_markup=(
        '<p>正文<a id="w1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#m1">'
        '<img alt="注" src="../Images/custom-note.png"/></a>继续。</p>\n'
        '    <hr/>\n'
        '    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>'
      ),
      extra_manifest_items='    <item id="custom-note" href="Images/custom-note.png" media-type="image/png"/>\n',
      extra_files={"OEBPS/Images/custom-note.png": b"png"},
    )

    report = C.convert_epub(source, output)
    assert report.plain_notes_converted == 1, report
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      items = opf.findall("opf:manifest/opf:item", OPF_NS)
      assert 'src="../Images/custom-note.png"' in chapter
      assert 'src="../Images/note.png"' not in chapter
      assert "OEBPS/Images/custom-note.png" in zf.namelist()
      assert "OEBPS/Images/note.png" not in zf.namelist()
      assert not any(item.attrib.get("href") == "Images/note.png" for item in items)


def sigil_legacy_notes_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "sigil-legacy-notes.epub"
    output = Path(raw) / "converted-sigil-legacy-notes.epub"
    write_legacy_epub(
      source,
      chapter_note_markup=(
        '<p>正文<sup><a id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a></sup>'
        '继续<sup><a id="noteref_2" href="#footnote_2" epub:type="noteref">[2]</a></sup>。</p>\n'
        '    <section class="fnote" epub:type="footnotes">\n'
        '      <aside id="footnote_1" epub:type="footnote"><p>'
        '<a href="#noteref_1" epub:type="noteref">[1]</a> 第一条注释正文保留。</p></aside>\n'
        '      <aside id="footnote_2" epub:type="footnote"><p>'
        '<a href="#noteref_2" epub:type="noteref">[2]</a> 第二条注释正文保留。</p></aside>\n'
        '    </section>'
      ),
    )

    report = C.convert_epub(source, output)
    assert report.plain_notes_converted == 2, report
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      assert chapter.count('<aside epub:type="footnote" role="doc-footnote">') == 1
      assert chapter.count('class="footnote-item"') == 2
      assert chapter.count('<sup class="note-marker">') == 2
      assert '<sup><a id="noteref_1"' not in chapter
      assert 'id="noteref_1" class="noteref-icon"' in chapter
      assert 'id="noteref_2" class="noteref-icon"' in chapter
      assert 'id="footnote_1"' in chapter
      assert 'id="footnote_2"' in chapter
      assert 'href="#noteref_1">◎</a>第一条注释正文保留。' in chapter
      assert 'href="#noteref_2">◎</a>第二条注释正文保留。' in chapter


def sigil_partial_section_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "sigil-partial-notes.epub"
    output = Path(raw) / "converted-sigil-partial-notes.epub"
    write_legacy_epub(
      source,
      chapter_note_markup=(
        '<p>正文<sup><a id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a></sup>继续。</p>\n'
        '    <section epub:type="footnotes">\n'
        '      <aside id="footnote_1" epub:type="footnote"><p>'
        '<a href="#noteref_1" epub:type="noteref">[1]</a> 注释正文保留。</p></aside>\n'
        '      <p>不能自动识别的残余内容。</p>\n'
        '    </section>'
      ),
    )

    report = C.convert_epub(source, output)
    assert report.plain_notes_converted == 0, report
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      assert '<section epub:type="footnotes">' in chapter
      assert 'id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a>' in chapter
      assert "不能自动识别的残余内容。" in chapter


def missing_html_language_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-missing-language.epub"
    output = Path(raw) / "converted-missing-language.epub"
    write_legacy_epub(source, missing_html_language=True)

    C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      cover = zf.read("OEBPS/Text/cover.xhtml").decode("utf-8")
      for page in (chapter, cover):
        assert 'lang="zh-CN"' in page
        assert 'xml:lang="zh-CN"' in page


def non_note_sup_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-non-note-sup.epub"
    output = Path(raw) / "converted-non-note-sup.epub"
    write_legacy_epub(
      source,
      chapter_note_markup=(
        '<p>水的式子是 H<sup>2</sup>O。<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a></p>\n'
        '    <hr/>\n'
        '    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>'
      ),
    )
    C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      assert 'H<sup>2</sup>O' in chapter
      assert 'H<sup class="note-marker">2</sup>O' not in chapter
def missing_package_language_case() -> None:
  with TemporaryDirectory() as raw:
    source = Path(raw) / "legacy-missing-package-language.epub"
    output = Path(raw) / "converted-missing-package-language.epub"
    write_legacy_epub(source, missing_html_language=True)
    stripped = Path(raw) / "legacy-without-package-language.epub"
    with zipfile.ZipFile(source) as source_zip, zipfile.ZipFile(stripped, "w") as stripped_zip:
      for info in source_zip.infolist():
        data = source_zip.read(info.filename)
        if info.filename == "OEBPS/content.opf":
          data = data.replace(b"    <dc:language>zh-CN</dc:language>\n", b"")
        stripped_zip.writestr(info, data)
    stripped.replace(source)

    C.convert_epub(source, output)
    with zipfile.ZipFile(output) as zf:
      chapter = zf.read("OEBPS/Text/chapter.xhtml").decode("utf-8")
      nav = zf.read("OEBPS/nav.xhtml").decode("utf-8")
      assert 'lang="zh-CN"' not in chapter
      assert 'xml:lang="zh-CN"' not in chapter
      assert 'lang="und"' in nav


if __name__ == "__main__":
  raise SystemExit(main())
