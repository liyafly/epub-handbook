#!/usr/bin/env python3
"""Validate versioned capability manifests without third-party dependencies."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CAPABILITY_ID_RE = re.compile(r"^epub(?:\.[a-z0-9][a-z0-9-]*){2,}$")
SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+$")
KINDS = {"detector", "planner", "transformer", "validator"}
RED_LINES = {"text", "metadata", "spine", "anchors", "cover", "drm"}
NETWORK_POLICIES = {"none", "readonly", "full"}
ADAPTERS = {"openai", "claude", "mcp", "cli", "gui"}
REQUIRED_FIELDS = {
  "schemaVersion",
  "id",
  "version",
  "kind",
  "legacySkillSlugs",
  "inputSchema",
  "outputSchema",
  "redLines",
  "permissions",
  "requires",
  "adapters",
}


def rel(root: Path, path: Path) -> str:
  return path.relative_to(root).as_posix()


def load_json(path: Path, root: Path, errors: list[str]) -> object | None:
  try:
    return json.loads(path.read_text(encoding="utf-8"))
  except json.JSONDecodeError as exc:
    errors.append(f"{rel(root, path)}: invalid JSON: {exc.msg}")
  except OSError as exc:
    errors.append(f"{rel(root, path)}: cannot read: {exc}")
  return None


def string_list(value: object) -> list[str] | None:
  if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
    return None
  return value


def validate_schema_reference(value: object, field: str, root: Path, source: Path, errors: list[str]) -> None:
  if not isinstance(value, str) or not value.startswith("contracts/schemas/v1/"):
    errors.append(f"{rel(root, source)}: {field} must reference contracts/schemas/v1/")
    return
  target = root / value
  if not target.is_file():
    errors.append(f"{rel(root, source)}: {field} target is missing: {value}")


def validate_manifest(
  payload: object,
  source: Path,
  root: Path,
  skill_slugs: set[str],
  errors: list[str],
) -> str | None:
  path = rel(root, source)
  if not isinstance(payload, dict):
    errors.append(f"{path}: manifest must be a JSON object")
    return None

  missing = sorted(REQUIRED_FIELDS - set(payload))
  if missing:
    errors.append(f"{path}: missing required fields: {', '.join(missing)}")
    return None
  extra = sorted(set(payload) - REQUIRED_FIELDS)
  if extra:
    errors.append(f"{path}: unsupported fields: {', '.join(extra)}")

  if payload["schemaVersion"] != "1":
    errors.append(f"{path}: schemaVersion must be 1")
  identifier = payload["id"]
  if not isinstance(identifier, str) or not CAPABILITY_ID_RE.fullmatch(identifier):
    errors.append(f"{path}: invalid capability id")
    identifier = None
  elif source.name != f"{identifier}.json":
    errors.append(f"{path}: filename must be {identifier}.json")
  if not isinstance(payload["version"], str) or not SEMVER_RE.fullmatch(payload["version"]):
    errors.append(f"{path}: version must be semantic version x.y.z")
  if payload["kind"] not in KINDS:
    errors.append(f"{path}: kind must be one of {', '.join(sorted(KINDS))}")

  skill_values = string_list(payload["legacySkillSlugs"])
  if skill_values is None or not skill_values:
    errors.append(f"{path}: legacySkillSlugs must be a non-empty string array")
  else:
    for slug in skill_values:
      if slug not in skill_slugs:
        errors.append(f"{path}: unknown legacy skill slug: {slug}")

  validate_schema_reference(payload["inputSchema"], "inputSchema", root, source, errors)
  validate_schema_reference(payload["outputSchema"], "outputSchema", root, source, errors)

  red_lines = string_list(payload["redLines"])
  if red_lines is None:
    errors.append(f"{path}: redLines must be a string array")
  elif unknown := sorted(set(red_lines) - RED_LINES):
    errors.append(f"{path}: unknown redLines: {', '.join(unknown)}")

  permissions = payload["permissions"]
  if not isinstance(permissions, dict):
    errors.append(f"{path}: permissions must be an object")
  else:
    if not isinstance(permissions.get("requiresWriteAccess"), bool):
      errors.append(f"{path}: permissions.requiresWriteAccess must be boolean")
    if permissions.get("network") not in NETWORK_POLICIES:
      errors.append(f"{path}: permissions.network must be one of {', '.join(sorted(NETWORK_POLICIES))}")

  requires = string_list(payload["requires"])
  if requires is None:
    errors.append(f"{path}: requires must be a string array")
  adapters = string_list(payload["adapters"])
  if adapters is None or not adapters:
    errors.append(f"{path}: adapters must be a non-empty string array")
  elif unknown := sorted(set(adapters) - ADAPTERS):
    errors.append(f"{path}: unknown adapters: {', '.join(unknown)}")
  return identifier


def validate_contracts(root: Path = ROOT) -> list[str]:
  root = root.resolve()
  errors: list[str] = []
  skill_root = root / "skills"
  skill_slugs = {
    folder.name
    for folder in skill_root.iterdir()
    if folder.is_dir() and (folder / "SKILL.md").is_file()
  } if skill_root.is_dir() else set()
  capability_root = root / "contracts" / "capabilities" / "v1"
  manifest_paths = sorted(capability_root.glob("*.json")) if capability_root.is_dir() else []
  if not manifest_paths:
    errors.append("contracts/capabilities/v1: no capability manifests found")
    return errors

  identifiers: dict[str, Path] = {}
  requires_by_source: list[tuple[Path, list[str]]] = []
  for path in manifest_paths:
    payload = load_json(path, root, errors)
    if payload is None:
      continue
    identifier = validate_manifest(payload, path, root, skill_slugs, errors)
    if isinstance(identifier, str):
      if identifier in identifiers:
        errors.append(f"{rel(root, path)}: duplicate capability id: {identifier}")
      identifiers[identifier] = path
    if isinstance(payload, dict) and (requires := string_list(payload.get("requires"))) is not None:
      requires_by_source.append((path, requires))

  for path, requires in requires_by_source:
    for requirement in requires:
      if requirement not in identifiers:
        errors.append(f"{rel(root, path)}: unknown required capability: {requirement}")
  return sorted(errors)


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Validate versioned capability contracts")
  parser.add_argument("--root", type=Path, default=ROOT, help="repository root")
  args = parser.parse_args(argv)
  errors = validate_contracts(args.root)
  if errors:
    for error in errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("contract validation ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
