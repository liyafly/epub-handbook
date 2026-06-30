#!/usr/bin/env python3
"""Write selected EPUB metadata fields to a new artifact."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from epub_package.harness import emit, ensure_new_file, fail
from epub_package.metadata import write_metadata
from epub_package.models import PackageToolError


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("input", type=Path)
  parser.add_argument("--output", required=True, type=Path)
  parser.add_argument("--metadata-json", required=True)
  parser.add_argument("--format", choices=("json",), default="json")
  args = parser.parse_args(argv)
  try:
    ensure_new_file(args.output)
    metadata = json.loads(args.metadata_json)
    if not isinstance(metadata, dict) or not all(isinstance(key, str) and isinstance(value, str) for key, value in metadata.items()):
      raise PackageToolError("metadata JSON must be an object of string fields")
    emit(write_metadata(args.input, args.output, metadata))
    return 0
  except (PackageToolError, OSError, json.JSONDecodeError) as exc:
    return fail(exc)


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
