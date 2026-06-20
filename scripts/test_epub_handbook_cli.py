#!/usr/bin/env python3
"""Regression coverage for the Python-only unified JSON CLI."""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "scripts" / "epub_handbook_cli.py"


class EPUBHandbookCLITests(unittest.TestCase):
  def test_catalog_exposes_manifest_projected_cli_surface(self) -> None:
    process = subprocess.run(
      [sys.executable, str(CLI), "catalog", "--format", "json"],
      cwd=ROOT,
      text=True,
      capture_output=True,
      check=False,
    )

    self.assertEqual(process.returncode, 0, process.stderr)
    payload = json.loads(process.stdout)
    self.assertEqual(payload["adapter"], "cli")
    self.assertIn("epub.notes.popup.normalize", {item["id"] for item in payload["capabilities"]})

  def test_unknown_provider_capability_preserves_file_based_result_contract(self) -> None:
    with tempfile.TemporaryDirectory() as raw:
      directory = Path(raw)
      result = directory / "result.json"
      process = subprocess.run(
        [
          sys.executable, str(CLI), "run", "epub.unknown.capability",
          "--input", str(directory / "book.epub"),
          "--result", str(result),
          "--format", "json",
        ],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
      )

      self.assertEqual(process.returncode, 2)
      self.assertFalse(result.exists())
      payload = json.loads(process.stdout)
      self.assertEqual(payload["status"], "failed")
      self.assertEqual(payload["exitCode"], 2)
      self.assertIn("unsupported capability", process.stderr)

  def test_registered_provider_returns_the_same_json_to_stdout_and_result_file(self) -> None:
    with tempfile.TemporaryDirectory() as raw:
      directory = Path(raw)
      result = directory / "result.json"
      process = subprocess.run(
        [
          sys.executable, str(CLI), "run", "epub.package.nav.audit",
          "--input", str(directory / "missing.epub"),
          "--result", str(result),
          "--format", "json",
        ],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
      )

      self.assertEqual(process.returncode, 0, process.stderr)
      self.assertTrue(result.exists())
      self.assertEqual(json.loads(process.stdout), json.loads(result.read_text(encoding="utf-8")))


if __name__ == "__main__":
  unittest.main()
