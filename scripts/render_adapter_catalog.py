#!/usr/bin/env python3
"""Render agent and product adapter catalogs from neutral capability manifests."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

import validate_contracts


ROOT = Path(__file__).resolve().parents[1]
ADAPTERS = {"openai", "claude", "mcp", "cli", "gui"}


def tool_name(capability_id: str) -> str:
  return capability_id.replace(".", "_").replace("-", "_")


def project(manifest: dict[str, Any], adapter: str) -> dict[str, Any]:
  descriptor: dict[str, Any] = {
    "id": manifest["id"],
    "version": manifest["version"],
    "kind": manifest["kind"],
    "legacySkillSlugs": manifest["legacySkillSlugs"],
    "requestSchema": manifest["inputSchema"],
    "responseSchema": manifest["outputSchema"],
    "redLines": manifest["redLines"],
    "requires": manifest["requires"],
  }
  if adapter in {"openai", "mcp"}:
    descriptor["tool"] = tool_name(manifest["id"])
  elif adapter == "claude":
    descriptor["skillTokens"] = [f"${slug}" for slug in manifest["legacySkillSlugs"]]
  elif adapter == "cli":
    descriptor["command"] = f"epub-handbook run {manifest['id']} --input <uri>"
  elif adapter == "gui":
    descriptor["featureID"] = manifest["id"]
  return descriptor


def build_catalog(root: Path = ROOT, adapter: str = "cli") -> dict[str, Any]:
  if adapter not in ADAPTERS:
    raise ValueError(f"unknown adapter: {adapter}")
  errors = validate_contracts.validate_contracts(root)
  if errors:
    raise ValueError("invalid contracts: " + "; ".join(errors))

  manifests: list[dict[str, Any]] = []
  for path in sorted((root / "contracts" / "capabilities" / "v1").glob("*.json")):
    manifest = json.loads(path.read_text(encoding="utf-8"))
    if adapter in manifest["adapters"]:
      manifests.append(project(manifest, adapter))
  return {"schemaVersion": "1", "adapter": adapter, "capabilities": manifests}


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("--adapter", choices=sorted(ADAPTERS), required=True)
  parser.add_argument("--root", type=Path, default=ROOT)
  parser.add_argument("--output", type=Path, help="write JSON instead of stdout")
  args = parser.parse_args(argv)
  try:
    payload = json.dumps(build_catalog(args.root.resolve(), args.adapter), ensure_ascii=False, indent=2) + "\n"
  except ValueError as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1
  if args.output:
    args.output.write_text(payload, encoding="utf-8")
  else:
    print(payload, end="")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
