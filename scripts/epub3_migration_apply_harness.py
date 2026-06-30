#!/usr/bin/env python3
"""Apply EPUB3 migration to a new artifact and emit an auditable JSON report."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from xml.etree import ElementTree as ET

from epub3_conversion import core
from epub3_conversion.converter import convert_epub, sha256_file


CAPABILITY = "epub.package.migrate.epub3"


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("input", type=Path)
  parser.add_argument("--output", required=True, type=Path)
  parser.add_argument("--no-popup-notes", action="store_true")
  parser.add_argument("--no-typography", action="store_true")
  parser.add_argument("--format", choices=("json",), default="json")
  args = parser.parse_args(argv)
  if args.output.exists():
    print(f"ERROR: refusing to overwrite existing output: {args.output}", file=sys.stderr)
    return 1
  try:
    conversion = convert_epub(
      args.input,
      args.output,
      popup_notes=not args.no_popup_notes,
      typography=not args.no_typography,
    )
    report = {
      "schema_version": "1.0",
      "capability": CAPABILITY,
      "status": "complete",
      "input": str(args.input.resolve()),
      "output": str(args.output.resolve()),
      "before_sha256": conversion.input_sha256,
      "after_sha256": sha256_file(args.output),
      "conversion": conversion.as_dict(),
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0
  except (core.ConversionError, ET.ParseError, OSError) as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
