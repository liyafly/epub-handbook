#!/usr/bin/env python3
"""Regression tests for validate_epub_style_demo.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import validate_epub_style_demo as V  # noqa: E402


def test_source_fixture() -> None:
  check = V.Check()
  V.validate_source(check)
  if check.errors:
    raise AssertionError(f"source fixture should validate cleanly: {check.errors}")


def test_epub_manifest_missing_member() -> None:
  with TemporaryDirectory() as raw:
    epub = Path(raw) / "missing-member.epub"
    with zipfile.ZipFile(epub, "w") as zf:
      info = zipfile.ZipInfo("mimetype")
      info.compress_type = zipfile.ZIP_STORED
      zf.writestr(info, b"application/epub+zip")
      zf.writestr(
        "OEBPS/package.opf",
        b'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <manifest><item id="missing" href="Text/missing.xhtml" media-type="application/xhtml+xml"/></manifest>
</package>''',
      )
    check = V.Check()
    V.validate_epub(epub, check)
    if not any("EPUB manifest href missing in zip: Text/missing.xhtml" in err for err in check.errors):
      raise AssertionError(f"missing manifest member was not reported: {check.errors}")



def _package(with_meta: bool):
  meta = '<meta property="ibooks:specified-fonts">true</meta>' if with_meta else ''
  return V.ET.fromstring(
    f'<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata>{meta}</metadata></package>'
  )


def test_body_font_mode_contract_accepts_locked_book() -> None:
  check = V.Check()
  V.validate_body_font_mode_contract(
    _package(with_meta=True),
    "body { margin: 0; line-height: 1.75; }",
    ".body-font-locked, .book-song { font-family: Songti, serif; }",
    {"Text/07.xhtml": '<body class="body-font-locked"><p>x</p></body>'},
    check,
    "test fixture",
  )
  if check.errors:
    raise AssertionError(f"locked book should validate cleanly: {check.errors}")


def test_body_font_mode_contract_rejects_body_font_family() -> None:
  check = V.Check()
  V.validate_body_font_mode_contract(
    _package(with_meta=False),
    "body { margin: 0; font-family: Songti, serif; }",
    ".body-font-locked { font-family: Songti, serif; }",
    {"Text/01.xhtml": "<body><p>x</p></body>"},
    check,
    "test fixture",
  )
  if not any("base.css body block must not set font-family" in err for err in check.errors):
    raise AssertionError(f"body font-family regression was not reported: {check.errors}")


def test_body_font_mode_contract_rejects_meta_mismatch() -> None:
  check = V.Check()
  V.validate_body_font_mode_contract(
    _package(with_meta=False),
    "body { margin: 0; }",
    ".body-font-locked { font-family: Songti, serif; }",
    {"Text/07.xhtml": '<body class="body-font-locked"><p>x</p></body>'},
    check,
    "test fixture",
  )
  if not any("body-font-locked pages and OPF ibooks:specified-fonts meta must match" in err for err in check.errors):
    raise AssertionError(f"locked/meta mismatch was not reported: {check.errors}")

def main() -> int:
  test_source_fixture()
  test_epub_manifest_missing_member()
  test_body_font_mode_contract_accepts_locked_book()
  test_body_font_mode_contract_rejects_body_font_family()
  test_body_font_mode_contract_rejects_meta_mismatch()
  print("validate_epub_style_demo tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
