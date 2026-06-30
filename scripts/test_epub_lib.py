#!/usr/bin/env python3
"""Regression tests for epub_lib.py."""

from __future__ import annotations

import sys
import warnings
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))

import epub_lib as E  # noqa: E402


def test_namespace_helpers() -> None:
  assert E.local_name("{http://www.w3.org/1999/xhtml}div") == "div"
  assert E.local_name("div") == "div"
  assert E.local_name(None) == ""
  assert E.q(E.OPF_URI, "item") == f"{{{E.OPF_URI}}}item"


def test_path_helpers() -> None:
  assert E.norm_join("OEBPS/Text", "../Images/a.png") == "OEBPS/Images/a.png"
  assert E.rel_href("OEBPS/Text/chapter.xhtml", "OEBPS/Images/a.png") == "../Images/a.png"


def test_safe_archive_and_uri_helpers() -> None:
  assert E.validate_archive_path("OEBPS/Text/../Images/a.png", "member") == "OEBPS/Images/a.png"
  for value in ("", "/absolute", "../escape", "OEBPS/../../escape"):
    try:
      E.validate_archive_path(value, "member")
    except E.EpubLibError as exc:
      assert "ZIP path" in str(exc)
    else:
      raise AssertionError(f"unsafe archive path must fail: {value!r}")
  assert E.is_external_uri("https://example.com/a") is True
  assert E.is_external_uri("//cdn.example.com/a") is True
  assert E.is_external_uri("../Images/a.png") is False
  assert E.resolve_relative_path("OEBPS/Text/ch.xhtml", "../Images/a%20b.png") == "OEBPS/Images/a b.png"
  assert E.quote_archive_path("OEBPS/Images/a b.png") == "OEBPS/Images/a%20b.png"


def test_find_child_and_safe_archive_reader() -> None:
  root = E.parse_xml("<root><first/><wanted/></root>")
  assert E.find_child(root, "wanted") is not None
  assert E.find_child(root, "missing") is None
  with TemporaryDirectory() as raw:
    duplicate = Path(raw) / "duplicate.epub"
    with warnings.catch_warnings():
      warnings.simplefilter("ignore", UserWarning)
      with zipfile.ZipFile(duplicate, "w") as zf:
        zf.writestr("same", b"one")
        zf.writestr("same", b"two")
    try:
      E.read_epub_archive(duplicate)
    except E.EpubLibError as exc:
      assert "duplicate ZIP member" in str(exc)
    else:
      raise AssertionError("duplicate ZIP member must fail")


def test_split_props() -> None:
  assert E.split_props("nav scripted") == ["nav", "scripted"]
  assert E.split_props("") == []
  assert E.split_props(None) == []


def test_parse_xml() -> None:
  root = E.parse_xml("<root><child/></root>", "valid.xml")
  assert root.tag == "root"
  try:
    E.parse_xml("<root>", "broken.xml")
  except ET.ParseError as exc:
    assert "broken.xml" in str(exc)
  else:
    raise AssertionError("invalid XML must raise ET.ParseError")


def test_opf_helpers() -> None:
  root = E.parse_xml(
    f"""<package xmlns="{E.OPF_URI}">
  <metadata/>
  <manifest><item id="chapter" href="chapter.xhtml"/></manifest>
  <spine><itemref idref="chapter"/></spine>
</package>""",
    "content.opf",
  )
  assert E.manifest(root).tag == E.q(E.OPF_URI, "manifest")
  assert E.spine(root).tag == E.q(E.OPF_URI, "spine")
  assert E.unique_id(root, "chapter") == "chapter-2"
  assert E.unique_id(root, "12 title") == "x-12-title"


def test_ensure_stylesheet_link() -> None:
  source = "<html><head><title>Test</title></head><body/></html>"
  updated, changed = E.ensure_stylesheet_link(source, "../Styles/base.css")
  assert changed is True
  assert '<link href="../Styles/base.css" type="text/css" rel="stylesheet"/>' in updated
  assert updated.index("<link ") < updated.index("</head>")
  unchanged, changed_again = E.ensure_stylesheet_link(updated, "../Styles/base.css")
  assert changed_again is False
  assert unchanged == updated


def test_read_write_roundtrip() -> None:
  files = {
    "mimetype": b"application/epub+zip",
    "META-INF/container.xml": b"""<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
""",
    "OEBPS/content.opf": b"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata/>
  <manifest>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
""",
    "OEBPS/Text/chapter.xhtml": b"""<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Test</title></head><body><p>Text</p></body></html>
""",
  }
  order = [
    "META-INF/container.xml",
    "OEBPS/content.opf",
    "OEBPS/Text/chapter.xhtml",
    "mimetype",
  ]
  with TemporaryDirectory() as raw:
    output = Path(raw) / "roundtrip.epub"
    E.write_epub(output, files, order)
    read_files, read_order = E.read_epub_files(output)
    assert read_files == files
    assert read_order[0] == "mimetype"
    assert E.opf_path_from_container(read_files) == "OEBPS/content.opf"
    with zipfile.ZipFile(output) as zf:
      infos = zf.infolist()
      assert infos[0].filename == "mimetype"
      assert infos[0].compress_type == zipfile.ZIP_STORED
      for name, data in files.items():
        assert zf.read(name) == data


def test_write_epub_ignores_ds_store() -> None:
  files = {
    "mimetype": b"application/epub+zip",
    "META-INF/container.xml": b"container",
    ".DS_Store": b"macos-metadata",
    "OEBPS/.DS_Store": b"nested-macos-metadata",
  }
  with TemporaryDirectory() as raw:
    output = Path(raw) / "without-ds-store.epub"
    E.write_epub(output, files, list(files))
    with zipfile.ZipFile(output) as zf:
      assert ".DS_Store" not in zf.namelist()
      assert "OEBPS/.DS_Store" not in zf.namelist()


def main() -> int:
  test_namespace_helpers()
  test_path_helpers()
  test_safe_archive_and_uri_helpers()
  test_find_child_and_safe_archive_reader()
  test_split_props()
  test_parse_xml()
  test_opf_helpers()
  test_ensure_stylesheet_link()
  test_read_write_roundtrip()
  test_write_epub_ignores_ds_store()
  print("epub_lib tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
