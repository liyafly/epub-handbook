#!/usr/bin/env python3
"""Analyze structural text roles and recommend EPUB-safe typography roles."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from epub_content_analysis import ContentAnalysisError, analyze_path, dumps_json, render_markdown


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("input", type=Path)
  parser.add_argument("--format", choices=("json", "md"), default="json")
  parser.add_argument("--output", type=Path)
  parser.add_argument("--include-snippets", action="store_true", help="include local text previews; never commit private reports")
  args = parser.parse_args(argv)
  try:
    report = analyze_path(args.input, include_snippets=args.include_snippets)
  except ContentAnalysisError as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1
  payload = dumps_json(report) if args.format == "json" else render_markdown(report)
  if args.output:
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(payload, encoding="utf-8")
  print(payload, end="")
  return 1 if report["status"] == "fail" else 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
