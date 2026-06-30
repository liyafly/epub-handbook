#!/usr/bin/env python3
"""Regression tests for the focused EPUB3 apply harness."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory

from test_epub3_oneclick_converter import write_legacy_epub


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub3_migration_apply_harness.py"


def test_apply_harness_reports_digests_and_refuses_overwrite() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    source = root / "legacy.epub"
    output = root / "epub3.epub"
    write_legacy_epub(source)
    process = subprocess.run(
      [sys.executable, str(SCRIPT), str(source), "--output", str(output), "--format", "json"],
      cwd=ROOT,
      text=True,
      capture_output=True,
      check=False,
    )
    assert process.returncode == 0, process.stderr
    report = json.loads(process.stdout)
    assert report["capability"] == "epub.package.migrate.epub3"
    assert report["status"] == "complete"
    assert len(report["before_sha256"]) == 64
    assert len(report["after_sha256"]) == 64
    assert report["conversion"]["package_version_before"] == "2.0"
    collision = subprocess.run(
      [sys.executable, str(SCRIPT), str(source), "--output", str(output), "--format", "json"],
      cwd=ROOT,
      text=True,
      capture_output=True,
      check=False,
    )
    assert collision.returncode == 1
    assert "refusing to overwrite" in collision.stderr


if __name__ == "__main__":
  test_apply_harness_reports_digests_and_refuses_overwrite()
  print("epub3 migration apply harness tests ok")
