#!/usr/bin/env python3
"""Regression tests for epub_package_tool.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub_package_tool import (  # noqa: E402
  merge_epubs,
  read_metadata,
  replace_cover,
  split_epub,
  write_metadata,
)


OPF_NS = {"opf": "http://www.idpf.org/2007/opf", "dc": "http://purl.org/dc/elements/1.1/"}


def zip_info(name: str, compress_type: int = zipfile.ZIP_DEFLATED) -> zipfile.ZipInfo:
  info = zipfile.ZipInfo(name)
  info.compress_type = compress_type
  return info


def write_book(path: Path, title: str, marker: str, *, cover_bytes: bytes = b"cover") -> None:
  files: dict[str, str | bytes] = {
    "META-INF/container.xml": '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
''',
    "OEBPS/content.opf": f'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:{marker}</dc:identifier>
    <dc:title id="main-title">{title}</dc:title>
    <dc:creator>Author {marker}</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:publisher>Publisher {marker}</dc:publisher>
    <dc:description>Description {marker}</dc:description>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="style" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="nav" linear="no"/>
    <itemref idref="chap"/>
  </spine>
</package>
''',
    "OEBPS/nav.xhtml": f'''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>{title}</title></head>
  <body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml#start">{title}</a></li></ol></nav></body>
</html>
''',
    "OEBPS/Text/chapter.xhtml": f'''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>{title}</title><link rel="stylesheet" href="../Styles/main.css"/></head>
  <body>
    <h1 id="start">{title}</h1>
    <p>{marker} 正文保留。<img src="../Images/cover.jpg" alt="cover"/></p>
  </body>
</html>
''',
    "OEBPS/Styles/main.css": "body { background: url('../Images/cover.jpg'); }\n",
    "OEBPS/Images/cover.jpg": cover_bytes,
    ".DS_Store": b"macos-metadata",
  }
  with zipfile.ZipFile(path, "w") as zf:
    for name, data in files.items():
      payload = data.encode("utf-8") if isinstance(data, str) else data
      zf.writestr(zip_info(name), payload)
    zf.writestr(zip_info("mimetype", zipfile.ZIP_DEFLATED), b"wrong-on-purpose")


def assert_valid_mimetype(zf: zipfile.ZipFile) -> None:
  first = zf.infolist()[0]
  assert first.filename == "mimetype"
  assert first.compress_type == zipfile.ZIP_STORED
  assert zf.read("mimetype") == b"application/epub+zip"


def opf_root(zf: zipfile.ZipFile, opf_path: str = "OEBPS/content.opf") -> ET.Element:
  return ET.fromstring(zf.read(opf_path))


def manifest_by_id(root: ET.Element) -> dict[str, ET.Element]:
  return {
    item.attrib["id"]: item
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
  }


def text_for(zf: zipfile.ZipFile, name: str) -> str:
  return zf.read(name).decode("utf-8")


def test_merge_epubs_rewrites_conflicting_resources_and_combines_toc(root: Path) -> None:
  first = root / "first.epub"
  second = root / "second.epub"
  output = root / "merged.epub"
  write_book(first, "第一册", "book-a", cover_bytes=b"cover-a")
  write_book(second, "第二册", "book-b", cover_bytes=b"cover-b")

  report = merge_epubs([first, second], output, title="合集")

  assert report.operation == "merge"
  assert report.inputs == [str(first), str(second)]
  assert report.output == str(output)
  assert report.merged_items == 6
  assert report.renamed_resources == 3
  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    root_el = opf_root(zf)
    assert root_el.attrib["version"] == "3.0"
    manifest = manifest_by_id(root_el)
    assert "nav" in manifest
    assert "ncx" in manifest
    assert manifest["nav"].attrib["href"] == "nav.xhtml"
    assert manifest["ncx"].attrib["href"] == "toc.ncx"
    spine = root_el.find("opf:spine", OPF_NS)
    assert spine is not None
    assert spine.attrib["toc"] == "ncx"
    names = set(zf.namelist())
    assert ".DS_Store" not in names
    assert "OEBPS/toc.ncx" in names
    assert "OEBPS/Text/chapter.xhtml" in names
    assert "OEBPS/Text/vol2_chapter.xhtml" in names
    assert "OEBPS/Images/cover.jpg" in names
    assert "OEBPS/Images/vol2_cover.jpg" in names
    second_chapter = text_for(zf, "OEBPS/Text/vol2_chapter.xhtml")
    assert 'href="../Styles/vol2_main.css"' in second_chapter
    assert 'src="../Images/vol2_cover.jpg"' in second_chapter
    assert "book-b 正文保留。" in second_chapter
    nav = text_for(zf, "OEBPS/nav.xhtml")
    assert '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">' in nav
    assert "html:" not in nav
    assert "ns1:" not in nav
    assert "第一册" in nav
    assert "第二册" in nav
    assert 'href="Text/chapter.xhtml#start"' in nav
    assert 'href="Text/vol2_chapter.xhtml#start"' in nav
    ncx = text_for(zf, "OEBPS/toc.ncx")
    assert 'src="Text/chapter.xhtml#start"' in ncx
    assert 'src="Text/vol2_chapter.xhtml#start"' in ncx


def test_split_epub_builds_independent_segments_with_referenced_resources(root: Path) -> None:
  source = root / "source.epub"
  output_dir = root / "split"
  write_book(source, "拆分书", "split")

  report = split_epub(source, output_dir, split_points=[0])

  assert report.operation == "split"
  assert report.segments_created == 1
  assert report.outputs == [str(output_dir / "source_01.epub")]
  with zipfile.ZipFile(output_dir / "source_01.epub") as zf:
    assert_valid_mimetype(zf)
    names = set(zf.namelist())
    assert "OEBPS/content.opf" in names
    assert "OEBPS/Text/chapter.xhtml" in names
    assert "OEBPS/Styles/main.css" in names
    assert "OEBPS/Images/cover.jpg" in names
    assert "OEBPS/nav.xhtml" in names
    root_el = opf_root(zf)
    manifest = manifest_by_id(root_el)
    assert set(manifest) == {"chap", "style", "cover-image", "nav", "ncx"}
    spine = root_el.find("opf:spine", OPF_NS)
    assert spine is not None
    assert spine.attrib["toc"] == "ncx"
    assert "split 正文保留。" in text_for(zf, "OEBPS/Text/chapter.xhtml")
    nav = text_for(zf, "OEBPS/nav.xhtml")
    assert '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">' in nav
    assert "html:" not in nav
    assert "ns1:" not in nav
    assert 'href="Text/chapter.xhtml#start"' in nav


def test_metadata_read_write_preserves_existing_package_structure(root: Path) -> None:
  source = root / "source.epub"
  output = root / "metadata.epub"
  write_book(source, "原题", "meta")

  before = read_metadata(source)
  assert before.title == "原题"
  assert before.author == "Author meta"
  report = write_metadata(
    source,
    output,
    {
      "title": "新题",
      "subtitle": "副题",
      "author": "新作者",
      "language": "zh-CN",
      "publisher": "新出版社",
      "description": "新简介",
      "rights": "版权声明",
    },
  )

  assert report.operation == "metadata-write"
  assert report.fields_updated == 6
  after = read_metadata(output)
  assert after.title == "新题"
  assert after.subtitle == "副题"
  assert after.author == "新作者"
  assert after.publisher == "新出版社"
  assert after.description == "新简介"
  assert after.rights == "版权声明"
  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    assert ".DS_Store" not in zf.namelist()
    assert "OEBPS/Text/chapter.xhtml" in zf.namelist()
    assert "meta 正文保留。" in text_for(zf, "OEBPS/Text/chapter.xhtml")

  author_only = root / "metadata-author-only.epub"
  report = write_metadata(output, author_only, {"author": "再作者"})
  assert report.fields_updated == 1
  preserved = read_metadata(author_only)
  assert preserved.title == "新题"
  assert preserved.subtitle == "副题"
  assert preserved.author == "再作者"


def test_replace_cover_updates_manifest_metadata_and_removes_old_cover(root: Path) -> None:
  source = root / "source.epub"
  cover = root / "new-cover.png"
  output = root / "cover.epub"
  write_book(source, "封面书", "cover", cover_bytes=b"old-cover")
  cover.write_bytes(b"new-cover")

  report = replace_cover(source, output, cover)

  assert report.operation == "replace-cover"
  assert report.cover_path == "OEBPS/Images/cover.png"
  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    names = set(zf.namelist())
    assert "OEBPS/Images/cover.png" in names
    assert "OEBPS/Images/cover.jpg" not in names
    assert zf.read("OEBPS/Images/cover.png") == b"new-cover"
    chapter = text_for(zf, "OEBPS/Text/chapter.xhtml")
    assert 'src="../Images/cover.png"' in chapter
    assert "../Images/cover.jpg" not in chapter
    css = text_for(zf, "OEBPS/Styles/main.css")
    assert "url('../Images/cover.png')" in css
    assert "cover.jpg" not in css
    root_el = opf_root(zf)
    manifest = manifest_by_id(root_el)
    cover_item = manifest["cover-image"]
    assert cover_item.attrib["href"] == "Images/cover.png"
    assert cover_item.attrib["media-type"] == "image/png"
    assert "cover-image" in cover_item.attrib["properties"].split()
    meta = root_el.find('opf:metadata/opf:meta[@name="cover"]', OPF_NS)
    assert meta is not None
    assert meta.attrib["content"] == "cover-image"


def main() -> int:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    test_merge_epubs_rewrites_conflicting_resources_and_combines_toc(root)
    test_split_epub_builds_independent_segments_with_referenced_resources(root)
    test_metadata_read_write_preserves_existing_package_structure(root)
    test_replace_cover_updates_manifest_metadata_and_removes_old_cover(root)
  print("epub package tool tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
