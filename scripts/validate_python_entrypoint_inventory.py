#!/usr/bin/env python3
"""Validate the Python-only public CLI and AI Agent entrypoint inventory."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
INVENTORY_PATH = Path("adapters/python/public-entrypoints.v1.json")
KINDS = {"harness", "pipeline", "transformer", "planner", "validator", "utility"}
SURFACES = {"cli", "agent"}
STATUSES = {"registered", "legacy-only"}


def validate_inventory(root: Path = ROOT) -> list[str]:
  root = root.resolve()
  source = root / INVENTORY_PATH
  try:
    payload = json.loads(source.read_text(encoding="utf-8"))
  except (OSError, json.JSONDecodeError) as exc:
    return [f"{INVENTORY_PATH}: cannot read JSON: {exc}"]
  errors: list[str] = []
  if not isinstance(payload, dict) or payload.get("schemaVersion") != "1":
    return [f"{INVENTORY_PATH}: schemaVersion must be 1"]
  if payload.get("provider") != "python":
    errors.append(f"{INVENTORY_PATH}: provider must be python")
  entrypoints = payload.get("entrypoints")
  if not isinstance(entrypoints, list) or not entrypoints:
    return errors + [f"{INVENTORY_PATH}: entrypoints must be a non-empty list"]
  capabilities = {path.stem for path in (root / "contracts" / "capabilities" / "v1").glob("*.json")}
  identifiers: set[str] = set()
  for item in entrypoints:
    if not isinstance(item, dict):
      errors.append(f"{INVENTORY_PATH}: each entrypoint must be an object")
      continue
    identifier = item.get("id")
    path = item.get("path")
    if not isinstance(identifier, str) or not identifier:
      errors.append(f"{INVENTORY_PATH}: entrypoint id must be a string")
    elif identifier in identifiers:
      errors.append(f"{INVENTORY_PATH}: duplicate entrypoint id: {identifier}")
    else:
      identifiers.add(identifier)
    if not isinstance(path, str) or not path.startswith("scripts/") or (root / path).is_file() is False:
      errors.append(f"{INVENTORY_PATH}: missing Python entrypoint: {path}")
    if item.get("kind") not in KINDS:
      errors.append(f"{INVENTORY_PATH}: invalid kind: {item.get('kind')}")
    surfaces = item.get("surfaces")
    if not isinstance(surfaces, list) or not surfaces or not all(surface in SURFACES for surface in surfaces):
      errors.append(f"{INVENTORY_PATH}: surfaces must be non-empty CLI/agent values")
    if item.get("status") not in STATUSES:
      errors.append(f"{INVENTORY_PATH}: invalid status: {item.get('status')}")
    refs = item.get("capabilities")
    if not isinstance(refs, list) or not all(isinstance(reference, str) for reference in refs):
      errors.append(f"{INVENTORY_PATH}: capabilities must be a string list")
    else:
      for reference in refs:
        if reference not in capabilities:
          errors.append(f"{INVENTORY_PATH}: unknown capability: {reference}")
  return sorted(errors)


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("--root", type=Path, default=ROOT)
  args = parser.parse_args(argv)
  errors = validate_inventory(args.root)
  if errors:
    for error in errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("python entrypoint inventory validation ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
