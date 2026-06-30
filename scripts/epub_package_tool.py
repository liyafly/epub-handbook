#!/usr/bin/env python3
"""Backward-compatible CLI façade for focused EPUB package operations."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from epub_package.core import *  # noqa: F403
from epub_package.cover import replace_cover
from epub_package.merge import merge_epubs
from epub_package.metadata import read_metadata, write_metadata
from epub_package.split import list_split_targets, split_epub


def print_json(value: object) -> None:
  if hasattr(value, "as_dict"):
    value = value.as_dict()
  print(json.dumps(value, ensure_ascii=False, indent=2))


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Conservative EPUB package merge/split/metadata/cover operations")
  sub = parser.add_subparsers(dest="command", required=True)
  merge_parser = sub.add_parser("merge", help="Merge multiple EPUB files")
  merge_parser.add_argument("inputs", nargs="+", type=Path)
  merge_parser.add_argument("--output", required=True, type=Path)
  merge_parser.add_argument("--title")
  split_targets_parser = sub.add_parser("split-targets", help="List split targets")
  split_targets_parser.add_argument("input", type=Path)
  split_parser = sub.add_parser("split", help="Split one EPUB at TOC target indices")
  split_parser.add_argument("input", type=Path)
  split_parser.add_argument("--output-dir", required=True, type=Path)
  split_parser.add_argument("--split-points", required=True, help="Comma-separated target indices")
  read_meta_parser = sub.add_parser("metadata-read", help="Read EPUB metadata")
  read_meta_parser.add_argument("input", type=Path)
  write_meta_parser = sub.add_parser("metadata-write", help="Write EPUB metadata")
  write_meta_parser.add_argument("input", type=Path)
  write_meta_parser.add_argument("--output", required=True, type=Path)
  write_meta_parser.add_argument("--metadata-json", required=True, help="JSON object with title/author/etc.")
  cover_parser = sub.add_parser("replace-cover", help="Replace EPUB cover image")
  cover_parser.add_argument("input", type=Path)
  cover_parser.add_argument("--output", required=True, type=Path)
  cover_parser.add_argument("--cover", required=True, type=Path)
  args = parser.parse_args(argv)
  try:
    if args.command == "merge":
      print_json(merge_epubs(args.inputs, args.output, title=args.title))
    elif args.command == "split-targets":
      print_json(list_split_targets(args.input))
    elif args.command == "split":
      print_json(split_epub(args.input, args.output_dir, [int(item) for item in args.split_points.split(",") if item.strip()]))
    elif args.command == "metadata-read":
      print_json(read_metadata(args.input))
    elif args.command == "metadata-write":
      print_json(write_metadata(args.input, args.output, json.loads(args.metadata_json)))
    elif args.command == "replace-cover":
      print_json(replace_cover(args.input, args.output, args.cover))
    return 0
  except (PackageToolError, ValueError, json.JSONDecodeError) as exc:  # noqa: F405
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
