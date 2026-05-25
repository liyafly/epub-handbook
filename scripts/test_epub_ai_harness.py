#!/usr/bin/env python3
"""Smoke-test epub_ai_harness output against the local demo tree."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "scripts" / "epub_ai_harness.py"
DEMO = ROOT / "templates" / "epub-style-demo"


def main() -> int:
  result = subprocess.run(
    [sys.executable, str(HARNESS), str(DEMO), "--format", "json"],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )
  if result.returncode:
    print(result.stderr or result.stdout, file=sys.stderr)
    return result.returncode

  data = json.loads(result.stdout)
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

  if data.get("mode") != "epub-source-tree":
    print(f"ERROR: expected epub-source-tree mode, got {data.get('mode')}", file=sys.stderr)
    return 1

  print("epub_ai_harness smoke test ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
