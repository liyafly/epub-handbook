#!/usr/bin/env python3
"""Tests for the Python-only JSON provider adapter used by CLI and agents."""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ADAPTER = ROOT / "scripts" / "python_provider_adapter.py"


class PythonProviderAdapterTests(unittest.TestCase):
  def test_preflight_request_writes_normalized_failed_result(self) -> None:
    with tempfile.TemporaryDirectory() as raw:
      directory = Path(raw)
      request = directory / "request.json"
      result = directory / "result.json"
      request.write_text(json.dumps({
        "schemaVersion": "1",
        "capability": "epub.package.nav.audit",
        "artifact": {"uri": (directory / "missing.epub").as_uri(), "kind": "epub"},
      }), encoding="utf-8")

      process = subprocess.run(
        [sys.executable, str(ADAPTER), "--request", str(request), "--result", str(result)],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
      )

      self.assertEqual(process.returncode, 0, process.stderr)
      payload = json.loads(result.read_text(encoding="utf-8"))
      self.assertEqual(payload["provider"], "python")
      self.assertEqual(payload["capability"], "epub.package.nav.audit")
      self.assertEqual(payload["status"], "failed")
      self.assertEqual(payload["legacyReport"]["harness"], "epub_preflight_harness")
      self.assertEqual(payload["normalizedInspection"]["status"], "fail")
      self.assertEqual(payload["normalizedInspection"]["findings"][0]["severity"], "error")

  def test_unknown_capability_is_rejected_without_running_a_command(self) -> None:
    with tempfile.TemporaryDirectory() as raw:
      directory = Path(raw)
      request = directory / "request.json"
      result = directory / "result.json"
      request.write_text(json.dumps({
        "schemaVersion": "1",
        "capability": "epub.unknown.capability",
        "artifact": {"uri": (directory / "book.epub").as_uri(), "kind": "epub"},
      }), encoding="utf-8")

      process = subprocess.run(
        [sys.executable, str(ADAPTER), "--request", str(request), "--result", str(result)],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
      )

      self.assertEqual(process.returncode, 2)
      self.assertIn("unsupported capability", process.stderr)
      self.assertFalse(result.exists())


if __name__ == "__main__":
  unittest.main()
