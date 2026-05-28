#!/usr/bin/env python3
"""Smoke-test epub_ai_harness output against the local demo tree."""

from __future__ import annotations

import json
import subprocess
import sys
from tempfile import TemporaryDirectory
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "scripts" / "epub_ai_harness.py"
DEMO = ROOT / "templates" / "epub-style-demo"


def run_harness(path: Path, *extra: str) -> tuple[int, dict[str, object]]:
  result = subprocess.run(
    [sys.executable, str(HARNESS), *extra, str(path), "--format", "json"],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )
  if not result.stdout:
    print(result.stderr, file=sys.stderr)
    return result.returncode, {}
  return result.returncode, json.loads(result.stdout)


def validate_demo_route() -> int:
  returncode, data = run_harness(DEMO)
  if returncode:
    print(json.dumps(data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  expected_skills = {
    "$epub-popup-footnote-converter",
    "$epub-vertical-ruby-optimizer",
    "$epub-css-layering-optimizer",
    "$epub-image-layout-optimizer",
    "$epub-package-nav-auditor",
  }
  skills = set(data.get("recommended_skills", []))
  missing = sorted(expected_skills - skills)
  if missing:
    print(f"ERROR: harness missing expected skills: {', '.join(missing)}", file=sys.stderr)
    return 1

  commands = data.get("suggested_commands", [])
  required_commands = [
    "sh templates/epub-style-demo/build.sh",
    "scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub",
    "scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub",
    "scripts/validate_skills_basic.py",
  ]
  missing_commands = [command for command in required_commands if command not in commands]
  if missing_commands:
    print(f"ERROR: harness missing suggested commands: {missing_commands}", file=sys.stderr)
    return 1

  if data.get("input_kind") != "epub-source-tree":
    print(f"ERROR: expected epub-source-tree input_kind, got {data.get('input_kind')}", file=sys.stderr)
    return 1

  returncode, cleanup_data = run_harness(DEMO, "--mode", "cleanup")
  if returncode:
    print(json.dumps(cleanup_data, ensure_ascii=False, indent=2), file=sys.stderr)
    return returncode
  if cleanup_data.get("mode") != "cleanup":
    print(f"ERROR: expected cleanup mode, got {cleanup_data.get('mode')}", file=sys.stderr)
    return 1
  cleanup_skills = cleanup_data.get("recommended_skills", [])
  if not cleanup_skills or cleanup_skills[0] != "$epub-layout-auditor":
    print(f"ERROR: cleanup mode should start with layout auditor: {cleanup_skills}", file=sys.stderr)
    return 1

  print("epub_ai_harness smoke test ok")
  return 0


def validate_missing_css_url_detection() -> int:
  with TemporaryDirectory() as tmp:
    root = Path(tmp)
    (root / "OEBPS" / "Text").mkdir(parents=True)
    (root / "OEBPS" / "Styles").mkdir(parents=True)
    (root / "OEBPS" / "Styles" / "main.css").write_text(
      '@font-face { font-family: "Missing"; src: url("../Fonts/Missing.otf"); }\n',
      encoding="utf-8",
    )
    (root / "OEBPS" / "Text" / "chapter.xhtml").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
<head><title>Test</title><link rel="stylesheet" type="text/css" href="../Styles/main.css"/></head>
<body><p>Test</p></body></html>
''',
      encoding="utf-8",
    )
    (root / "OEBPS" / "content.opf").write_text(
      '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:test</dc:identifier>
    <dc:title>Test</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="Text/chapter.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="chapter"/>
  </spine>
</package>
''',
      encoding="utf-8",
    )
    returncode, data = run_harness(root)
  if returncode == 0:
    print("ERROR: missing CSS url should make harness fail", file=sys.stderr)
    return 1
  findings = data.get("findings", [])
  if not any(item.get("message") == "CSS url() target missing" for item in findings):
    print(f"ERROR: missing CSS url finding not present: {findings}", file=sys.stderr)
    return 1
  print("epub_ai_harness CSS url smoke test ok")
  return 0


def main() -> int:
  for check in (validate_demo_route, validate_missing_css_url_detection):
    result = check()
    if result:
      return result
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
