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
    self.assertIn("scripts/epub_preflight_harness.py", paths)
    self.assertIn("scripts/epub_cleanup_pipeline.py", paths)
    self.assertIn("scripts/validate_text_invariance.py", paths)

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
