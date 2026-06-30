#!/usr/bin/env python3
"""Regression tests for the Python CLI / agent entrypoint inventory."""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import validate_python_entrypoint_inventory as inventory  # noqa: E402


class PythonEntrypointInventoryTests(unittest.TestCase):
  def test_repository_inventory_lists_public_preflight_and_cleanup_entrypoints(self) -> None:
    errors = inventory.validate_inventory(ROOT)

    self.assertEqual(errors, [])
    payload = json.loads((ROOT / "adapters" / "python" / "public-entrypoints.v1.json").read_text(encoding="utf-8"))
    paths = {item["path"] for item in payload["entrypoints"]}
    by_id = {item["id"]: item for item in payload["entrypoints"]}
    self.assertIn("scripts/epub_preflight_harness.py", paths)
    self.assertIn("scripts/epub_cleanup_pipeline.py", paths)
    self.assertIn("scripts/validate_text_invariance.py", paths)
    self.assertIn("scripts/epub_handbook_cli.py", paths)
    self.assertIn("scripts/epub_content_analyzer.py", paths)
    self.assertIn("scripts/epub_font_coverage_adapter.py", paths)
    self.assertTrue({
      "scripts/epub_package_merge_harness.py",
      "scripts/epub_package_split_harness.py",
      "scripts/epub_metadata_edit_harness.py",
      "scripts/epub_cover_replace_harness.py",
      "scripts/epub3_migration_apply_harness.py",
    }.issubset(paths))
    self.assertEqual(by_id["python.content-analysis"]["kind"], "detector")
    self.assertEqual(by_id["python.font-coverage"]["kind"], "detector")

  def test_rejects_unknown_capability_reference(self) -> None:
    with tempfile.TemporaryDirectory() as raw:
      root = Path(raw)
      (root / "scripts").mkdir()
      (root / "scripts" / "tool.py").write_text("", encoding="utf-8")
      (root / "contracts" / "capabilities" / "v1").mkdir(parents=True)
      (root / "contracts" / "capabilities" / "v1" / "epub.layout.audit.json").write_text("{}", encoding="utf-8")
      (root / "adapters" / "python").mkdir(parents=True)
      (root / "adapters" / "python" / "public-entrypoints.v1.json").write_text(json.dumps({
        "schemaVersion": "1",
        "provider": "python",
        "entrypoints": [{
          "id": "test",
          "path": "scripts/tool.py",
          "kind": "validator",
          "surfaces": ["cli"],
          "capabilities": ["epub.unknown.capability"],
          "status": "registered",
        }],
      }), encoding="utf-8")

      self.assertEqual(
        inventory.validate_inventory(root),
        ["adapters/python/public-entrypoints.v1.json: unknown capability: epub.unknown.capability"],
      )


if __name__ == "__main__":
  unittest.main()
