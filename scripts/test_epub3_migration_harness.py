#!/usr/bin/env python3
"""Regression tests for epub3_migration_harness.py."""

from __future__ import annotations

import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub3_migration_harness.py"
OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def write_epub(
  path: Path,
  *,
  nav_count: int = 1,
  version: str = "3.0",
  modified: bool = True,
  nav_in_spine: bool = False,
) -> None:
  nav_items = "\n".join(
    f'    <item id="nav{i}" href="nav{i}.xhtml" media-type="application/xhtml+xml" properties="nav"/>'
    for i in range(1, nav_count + 1)
  )
  modified_meta = '    <meta property="dcterms:modified">2026-06-03T00:00:00Z</meta>\n' if modified else ""
  spine_nav = '<itemref idref="nav1"/>' if nav_in_spine else ""
  opf = f'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="{version}" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:migration-test</dc:identifier>
    <dc:title>Migration Test</dc:title>
    <dc:language>en</dc:language>
{modified_meta}  </metadata>
  <manifest>
{nav_items}
    <item id="chap1" href="Text/chap1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  </manifest>
  <spine toc="ncx">{spine_nav}<itemref idref="chap1"/></spine>
</package>
'''
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", "application/epub+zip")
    zf.writestr("META-INF/container.xml", '''<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
''')
    zf.writestr("OEBPS/package.opf", opf)
    for i in range(1, nav_count + 1):
      zf.writestr(f"OEBPS/nav{i}.xhtml", '<html xmlns="http://www.w3.org/1999/xhtml"><body><nav><ol/></nav></body></html>')
    zf.writestr("OEBPS/Text/chap1.xhtml", '<html xmlns="http://www.w3.org/1999/xhtml"><head><title>One</title></head><body><h1>One</h1><p>Body.</p></body></html>')
    zf.writestr("OEBPS/toc.ncx", '''<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap><navPoint id="n1"><navLabel><text>One</text></navLabel><content src="Text/chap1.xhtml"/></navPoint></navMap>
</ncx>
''')


def run(*args: str) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), *args],
    cwd=ROOT,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    check=False,
  )


def nav_manifest_count(epub: Path) -> int:
  with zipfile.ZipFile(epub) as zf:
    root = ET.fromstring(zf.read("OEBPS/package.opf"))
  return sum(
    1
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if "nav" in item.attrib.get("properties", "").split()
  )


def zip_names(epub: Path) -> set[str]:
  with zipfile.ZipFile(epub) as zf:
    return set(zf.namelist())


def read_zip_text(epub: Path, name: str) -> str:
  with zipfile.ZipFile(epub) as zf:
    return zf.read(name).decode("utf-8")


def missing_spine_idrefs(epub: Path) -> set[str]:
  with zipfile.ZipFile(epub) as zf:
    root = ET.fromstring(zf.read("OEBPS/package.opf"))
  manifest_ids = {
    item.attrib.get("id")
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
  }
  return {
    itemref.attrib.get("idref", "")
    for itemref in root.findall("opf:spine/opf:itemref", OPF_NS)
    if itemref.attrib.get("idref") not in manifest_ids
  }


def main() -> int:
  with TemporaryDirectory() as raw:
    tmp = Path(raw)
    source = tmp / "multi-nav.epub"
    output = tmp / "out.epub"
    write_epub(source, nav_count=3)
    result = run(str(source), "--write-output", str(output), "--format", "json")
    if result.returncode:
      raise AssertionError(f"multi-nav migration failed: {result.stderr}\n{result.stdout}")
    nav_text = read_zip_text(output, "OEBPS/nav.xhtml")
    if not nav_text.startswith('<?xml version="1.0" encoding="utf-8"?>\n<!DOCTYPE html>\n\n<html'):
      raise AssertionError("generated nav.xhtml should use the editor-stable XHTML header")
    if nav_manifest_count(output) != 1:
      raise AssertionError("migrated EPUB must contain exactly one nav manifest item")
    stale_nav_files = {f"OEBPS/nav{i}.xhtml" for i in range(1, 4)} & zip_names(output)
    if stale_nav_files:
      raise AssertionError(f"migrated EPUB must not retain removed nav files: {sorted(stale_nav_files)}")

  with TemporaryDirectory() as raw:
    tmp = Path(raw)
    source = tmp / "multi-nav-spine.epub"
    output = tmp / "out.epub"
    write_epub(source, nav_count=2, nav_in_spine=True)
    result = run(str(source), "--write-output", str(output), "--format", "json")
    if result.returncode:
      raise AssertionError(f"multi-nav spine migration failed: {result.stderr}\n{result.stdout}")
    missing = missing_spine_idrefs(output)
    if missing:
      raise AssertionError(f"migrated EPUB must not leave stale spine idrefs: {sorted(missing)}")

  with TemporaryDirectory() as raw:
    source = Path(raw) / "plan.epub"
    write_epub(source, nav_count=0, version="2.0", modified=False)
    result = run(str(source), "--format", "json")
    data = json.loads(result.stdout)
    if result.returncode or "actions" not in data or "warnings" not in data:
      raise AssertionError(f"plan JSON missing expected keys: {result.stderr}\n{result.stdout}")

  with TemporaryDirectory() as raw:
    source = Path(raw) / "ready.epub"
    write_epub(source, nav_count=1)
    result = run(str(source), "--format", "json")
    data = json.loads(result.stdout)
    if result.returncode or not data.get("already_epub3") or data.get("actions"):
      raise AssertionError(f"EPUB3-ready package should be idempotent: {result.stdout}")

  print("epub3 migration harness tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
