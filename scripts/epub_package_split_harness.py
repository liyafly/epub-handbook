#!/usr/bin/env python3
"""Split one EPUB at explicit TOC indices into a new output directory."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from epub_package.harness import emit, ensure_empty_output_dir, fail
from epub_package.models import PackageToolError
from epub_package.split import split_epub


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("input", type=Path)
  parser.add_argument("--output-dir", required=True, type=Path)
  parser.add_argument("--split-points", required=True)
  parser.add_argument("--format", choices=("json",), default="json")
  args = parser.parse_args(argv)
  try:
    ensure_empty_output_dir(args.output_dir)
    points = [int(value) for value in args.split_points.split(",") if value.strip()]
    emit(split_epub(args.input, args.output_dir, points))
    return 0
  except (PackageToolError, OSError, ValueError) as exc:
    return fail(exc)


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
