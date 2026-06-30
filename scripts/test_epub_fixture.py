#!/usr/bin/env python3
"""Tests for shared EPUB test-fixture construction."""

from __future__ import annotations

import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

from test_support.epub_fixture import EpubFixture


def test_fixture_writes_valid_mimetype_and_deterministic_members() -> None:
  with TemporaryDirectory() as raw:
    output = Path(raw) / "book.epub"
    fixture = EpubFixture()
    fixture.add_text("OEBPS/chapter.xhtml", "<p>正文</p>")
    fixture.add_bytes("OEBPS/image.bin", b"image")
    fixture.write(output)
    with zipfile.ZipFile(output) as zf:
      infos = zf.infolist()
      assert [info.filename for info in infos] == [
        "mimetype",
        "OEBPS/chapter.xhtml",
        "OEBPS/image.bin",
      ]
      assert infos[0].compress_type == zipfile.ZIP_STORED
      assert zf.read("mimetype") == b"application/epub+zip"


def test_fixture_can_build_an_intentionally_invalid_mimetype_entry() -> None:
  with TemporaryDirectory() as raw:
    output = Path(raw) / "broken.epub"
    EpubFixture().write(output, mimetype_compress_type=zipfile.ZIP_DEFLATED)
    with zipfile.ZipFile(output) as zf:
      assert zf.infolist()[0].compress_type == zipfile.ZIP_DEFLATED


if __name__ == "__main__":
  test_fixture_writes_valid_mimetype_and_deterministic_members()
  test_fixture_can_build_an_intentionally_invalid_mimetype_entry()
  print("epub fixture tests ok")
