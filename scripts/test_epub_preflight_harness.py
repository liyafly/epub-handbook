#!/usr/bin/env python3
"""Regression tests for epub_preflight_harness.py."""

from __future__ import annotations

import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

from test_support.epub_fixture import write_epub as write_fixture_epub

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_preflight_harness.py"


def write_epub(path: Path, *, encryption: bool = False, broken_container: bool = False, missing_spine: bool = False) -> None:
  manifest_spine = "" if missing_spine else '<spine><itemref idref="c1"/></spine>'
  opf = f'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:preflight</dc:identifier><dc:title>Preflight</dc:title><dc:language>en</dc:language><meta name="cover" content="cover"/></metadata>
  <manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="Images/cover.png" media-type="image/png" properties="cover-image"/><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/></manifest>
  {manifest_spine.replace('<spine>', '<spine toc="ncx">')}
</package>'''
  files: dict[str, str | bytes] = {
    "OEBPS/package.opf": opf,
    "OEBPS/nav.xhtml": '<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en"><body><nav><ol><li><a href="Text/c1.xhtml">One</a></li></ol></nav></body></html>',
    "OEBPS/Text/c1.xhtml": '<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en"><head><title>One</title></head><body><p>Body.</p></body></html>',
    "OEBPS/toc.ncx": '<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap/></ncx>',
    "OEBPS/Images/cover.png": b"png",
  }
  if not broken_container:
    files["META-INF/container.xml"] = '''<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>'''
  if encryption:
    files["META-INF/encryption.xml"] = "<encryption/>"
  write_fixture_epub(path, files)


def run(path: Path) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), str(path), "--format", "json"],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )


def assert_status(path: Path, expected_status: str, expected_code: int) -> dict[str, object]:
  result = run(path)
  data = json.loads(result.stdout)
  if result.returncode != expected_code or data.get("preflight_status") != expected_status:
    raise AssertionError(f"expected {expected_status}/{expected_code}, got {result.returncode}: {data}\n{result.stderr}")
  if not isinstance(data.get("findings"), list) or not isinstance(data.get("findings_by_level"), dict):
    raise AssertionError(f"preflight JSON missing normalized finding collections: {data}")
  if not all(isinstance(v, int) for v in data["findings_by_level"].values()):
    raise AssertionError(f"finding level counts must be ints: {data['findings_by_level']}")
  return data


def main() -> int:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    valid = root / "valid.epub"
    write_epub(valid)
    assert_status(valid, "pass", 0)

    no_container = root / "no-container.epub"
    write_epub(no_container, broken_container=True)
    assert_status(no_container, "fail", 1)

    encrypted = root / "encrypted.epub"
    write_epub(encrypted, encryption=True)
    assert_status(encrypted, "fail", 1)

    no_spine = root / "no-spine.epub"
    write_epub(no_spine, missing_spine=True)
    assert_status(no_spine, "fail", 1)

  print("epub preflight harness tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
