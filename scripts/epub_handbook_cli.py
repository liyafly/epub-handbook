#!/usr/bin/env python3
"""Stable JSON front door for the existing Python CLI / AI-Agent providers.

This command intentionally remains in the Python adapter layer. Apple GUI
targets use the independent `epub-handbook-swift` executable and native Swift
libraries; neither side spawns the other.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import python_provider_adapter
import render_adapter_catalog


ROOT = Path(__file__).resolve().parents[1]


def artifact_uri(value: str) -> str:
  path = Path(value)
  if value.startswith("file://"):
    return value
  if not path.is_absolute():
    raise ValueError("--input must be an absolute path or file URI")
  return path.as_uri()


def emit(payload: object) -> None:
  print(json.dumps(payload, ensure_ascii=False, indent=2))


def catalog() -> int:
  try:
    emit(render_adapter_catalog.build_catalog(ROOT, "cli"))
  except ValueError as exc:
    emit({"schemaVersion": "1", "status": "failed", "error": {"code": "contract", "message": str(exc)}})
    return 1
  return 0


def run(capability: str, input_value: str, result: Path) -> int:
  try:
    request = {
      "schemaVersion": "1",
      "capability": capability,
      "artifact": {"uri": artifact_uri(input_value), "kind": "epub"},
    }
  except ValueError as exc:
    emit({"schemaVersion": "1", "status": "failed", "error": {"code": "input", "message": str(exc)}})
    return 2
  request_path = result.parent / f".{result.name}.request.json"
  result.parent.mkdir(parents=True, exist_ok=True)
  try:
    request_path.write_text(json.dumps(request, ensure_ascii=False) + "\n", encoding="utf-8")
    code = python_provider_adapter.run(request_path, result)
  finally:
    request_path.unlink(missing_ok=True)
  if result.exists():
    emit(json.loads(result.read_text(encoding="utf-8")))
  else:
    emit({
      "schemaVersion": "1",
      "provider": "python",
      "capability": capability,
      "status": "failed",
      "exitCode": code,
      "error": {"code": "provider-request", "message": "Provider request was rejected before a result artifact was written."},
    })
  return code


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  subparsers = parser.add_subparsers(dest="command", required=True)
  catalog_parser = subparsers.add_parser("catalog")
  catalog_parser.add_argument("--format", choices=("json",), default="json")
  run_parser = subparsers.add_parser("run")
  run_parser.add_argument("capability")
  run_parser.add_argument("--input", required=True)
  run_parser.add_argument("--result", required=True, type=Path)
  run_parser.add_argument("--format", choices=("json",), default="json")
  args = parser.parse_args(argv)
  if args.command == "catalog":
    return catalog()
  return run(args.capability, args.input, args.result)


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
