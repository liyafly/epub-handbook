#!/usr/bin/env python3
"""Merge two or more EPUBs into a new artifact with a JSON report."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from epub_package.harness import emit, ensure_new_file, fail
from epub_package.merge import merge_epubs
from epub_package.models import PackageToolError


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("inputs", nargs="+", type=Path)
  parser.add_argument("--output", required=True, type=Path)
  parser.add_argument("--title")
  parser.add_argument("--format", choices=("json",), default="json")
  args = parser.parse_args(argv)
  try:
    ensure_new_file(args.output)
    emit(merge_epubs(args.inputs, args.output, args.title))
    return 0
  except (PackageToolError, OSError) as exc:
    return fail(exc)


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
