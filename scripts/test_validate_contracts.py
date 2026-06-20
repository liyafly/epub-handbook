#!/usr/bin/env python3
"""Regression tests for the versioned capability-contract catalog."""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import validate_contracts as contracts  # noqa: E402


def write_json(path: Path, payload: object) -> None:
  path.parent.mkdir(parents=True, exist_ok=True)
  path.write_text(json.dumps(payload), encoding="utf-8")


def manifest(*, skill_slug: str = "epub-layout-auditor") -> dict[str, object]:
  return {
    "schemaVersion": "1",
    "id": "epub.layout.audit",
    "version": "1.0.0",
    "kind": "planner",
    "legacySkillSlugs": [skill_slug],
    "inputSchema": "contracts/schemas/v1/inspection-report.schema.json",
    "outputSchema": "contracts/schemas/v1/execution-plan.schema.json",
    "redLines": [],
    "permissions": {"requiresWriteAccess": False, "network": "none"},
    "requires": [],
    "adapters": ["openai", "cli"],
  }


class ValidateContractsTests(unittest.TestCase):
  def test_repository_catalog_covers_every_current_skill_slug(self) -> None:
    errors = contracts.validate_contracts(ROOT)

    self.assertEqual(errors, [])
    catalog_slugs: set[str] = set()
    for path in (ROOT / "contracts" / "capabilities" / "v1").glob("*.json"):
      catalog_slugs.update(json.loads(path.read_text(encoding="utf-8"))["legacySkillSlugs"])
    actual_skills = {
      folder.name
      for folder in (ROOT / "skills").iterdir()
      if folder.is_dir() and (folder / "SKILL.md").is_file()
    }
    self.assertEqual(catalog_slugs, actual_skills)

  def test_accepts_manifest_with_existing_skill_and_schema_paths(self) -> None:
    with tempfile.TemporaryDirectory() as tmp:
      root = Path(tmp)
      (root / "skills" / "epub-layout-auditor").mkdir(parents=True)
      (root / "skills" / "epub-layout-auditor" / "SKILL.md").write_text("---\nname: epub-layout-auditor\ndescription: test\n---\nbody\n", encoding="utf-8")
      write_json(root / "contracts" / "schemas" / "v1" / "inspection-report.schema.json", {"$schema": "https://json-schema.org/draft/2020-12/schema"})
      write_json(root / "contracts" / "schemas" / "v1" / "execution-plan.schema.json", {"$schema": "https://json-schema.org/draft/2020-12/schema"})
      write_json(root / "contracts" / "capabilities" / "v1" / "epub.layout.audit.json", manifest())

      self.assertEqual(contracts.validate_contracts(root), [])

  def test_rejects_manifest_that_references_unknown_skill(self) -> None:
    with tempfile.TemporaryDirectory() as tmp:
      root = Path(tmp)
      write_json(root / "contracts" / "schemas" / "v1" / "inspection-report.schema.json", {"$schema": "https://json-schema.org/draft/2020-12/schema"})
      write_json(root / "contracts" / "schemas" / "v1" / "execution-plan.schema.json", {"$schema": "https://json-schema.org/draft/2020-12/schema"})
      write_json(root / "contracts" / "capabilities" / "v1" / "epub.layout.audit.json", manifest(skill_slug="missing-skill"))

      self.assertEqual(
        contracts.validate_contracts(root),
        ["contracts/capabilities/v1/epub.layout.audit.json: unknown legacy skill slug: missing-skill"],
      )


if __name__ == "__main__":
  unittest.main()
