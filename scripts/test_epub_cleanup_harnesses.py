#!/usr/bin/env python3
"""Smoke-test cleanup/refinement harnesses on a synthetic EPUB 2 book."""

from __future__ import annotations

import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def write_epub2(path: Path) -> None:
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
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:test-epub2</dc:identifier>
    <dc:title>Harness EPUB2</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="font" href="Fonts/BookSong.otf" media-type="font/otf"/>
    <item id="legacy-gif" href="Images/legacy.gif" media-type="image/gif"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="chapter"/>
  </spine>
</package>
''',
    "OEBPS/toc.ncx": '''<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:test-epub2"/></head>
  <docTitle><text>Harness EPUB2</text></docTitle>
  <navMap>
    <navPoint id="navPoint-1" playOrder="1">
      <navLabel><text>第一章</text></navLabel>
      <content src="Text/chapter.xhtml"/>
    </navPoint>
  </navMap>
</ncx>
''',
    "OEBPS/Text/chapter.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
  <head>
    <title>第一章</title>
    <link rel="stylesheet" type="text/css" href="../Styles/main.css"/>
  </head>
  <body>
    <p id="p1">正文保留不变<a href="#fn1">[1]</a>，并有<ruby>字<rt>zi</rt></ruby>。</p>
    <p id="fn1">注释正文保留。</p>
    <p><img src="../Images/legacy.gif" alt="旧格式图"/></p>
  </body>
</html>
''',
    "OEBPS/Styles/main.css": '''@font-face {
  font-family: "BookSong";
  src: url("../Fonts/BookSong.otf");
}
body {
  font-family: "BookSong", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
  line-height: 1.4;
}
''',
    "OEBPS/Fonts/BookSong.otf": b"fake-font",
    "OEBPS/Images/legacy.gif": b"GIF89a",
  }
  with zipfile.ZipFile(path, "w") as zf:
    info = zipfile.ZipInfo("mimetype")
    info.compress_type = zipfile.ZIP_STORED
    zf.writestr(info, b"application/epub+zip")
    for name, data in files.items():
      zf.writestr(name, data.encode("utf-8") if isinstance(data, str) else data)


def run_json(script: str, *args: str, expect_success: bool = True) -> tuple[int, dict[str, object]]:
  result = subprocess.run(
    [sys.executable, str(SCRIPTS / script), *args, "--format", "json"],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )
  if expect_success and result.returncode:
    print(result.stdout, file=sys.stderr)
    print(result.stderr, file=sys.stderr)
    raise AssertionError(f"{script} failed with {result.returncode}")
  return result.returncode, json.loads(result.stdout)


def main() -> int:
  with TemporaryDirectory() as tmp:
    source = Path(tmp) / "source-epub2.epub"
    upgraded = Path(tmp) / "upgraded.epub"
    write_epub2(source)

    code, preflight = run_json("epub_preflight_harness.py", str(source))
    if code != 0 or preflight.get("preflight_status") != "warn":
      print(json.dumps(preflight, ensure_ascii=False, indent=2), file=sys.stderr)
      return 1
    findings = preflight.get("findings", [])
    if not any(item.get("kind") == "epub3-migration" for item in findings):
      print("ERROR: preflight did not recommend EPUB3 migration", file=sys.stderr)
      return 1

    _, refine = run_json("epub_refinement_harness.py", str(source))
    rec_ids = {item.get("id") for item in refine.get("recommendations", [])}
    expected = {"preflight", "epub3-migration", "popup-notes", "typography-fonts", "images", "redline-and-diff"}
    missing = expected - rec_ids
    if missing:
      print(f"ERROR: refinement missing recommendations: {sorted(missing)}", file=sys.stderr)
      print(json.dumps(refine, ensure_ascii=False, indent=2), file=sys.stderr)
      return 1

    _, migrated = run_json("epub3_migration_harness.py", str(source), "--write-output", str(upgraded))
    if migrated.get("written_output") != str(upgraded) or not upgraded.exists():
      print(json.dumps(migrated, ensure_ascii=False, indent=2), file=sys.stderr)
      return 1
    with zipfile.ZipFile(upgraded) as zf:
      root = ET.fromstring(zf.read("OEBPS/content.opf"))
      navs = [
        item for item in root.findall("opf:manifest/opf:item", OPF_NS)
        if "nav" in (item.attrib.get("properties") or "").split()
      ]
      if root.attrib.get("version") != "3.0" or len(navs) != 1 or "OEBPS/nav.xhtml" not in zf.namelist():
        print("ERROR: migrated EPUB is not EPUB3/nav complete", file=sys.stderr)
        return 1

    redline = subprocess.run(
      [
        sys.executable,
        str(SCRIPTS / "validate_text_invariance.py"),
        str(source),
        str(upgraded),
        "--check",
        "text,metadata,spine,anchors",
        "--allow-list",
        "OEBPS/nav.xhtml",
      ],
      cwd=ROOT,
      check=False,
      text=True,
      stdout=subprocess.PIPE,
      stderr=subprocess.PIPE,
    )
    if redline.returncode:
      print(redline.stdout, file=sys.stderr)
      print(redline.stderr, file=sys.stderr)
      return redline.returncode

  print("epub cleanup harness tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
