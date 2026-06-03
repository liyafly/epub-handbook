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


def main() -> int:
  test_source_fixture()
  test_epub_manifest_missing_member()
  print("validate_epub_style_demo tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
