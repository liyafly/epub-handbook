#!/usr/bin/env python3
"""Regression tests for focused package-operation harnesses."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory

from test_epub_package_tool import write_book


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"


def run(script: str, *args: str) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPTS / script), *args, "--format", "json"],
    cwd=ROOT,
    text=True,
    capture_output=True,
    check=False,
  )


def test_focused_package_harnesses_and_non_overwrite() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    first = root / "first.epub"
    second = root / "second.epub"
    write_book(first, "第一册", "one")
    write_book(second, "第二册", "two")

    merged = root / "merged.epub"
    merge = run("epub_package_merge_harness.py", str(first), str(second), "--output", str(merged))
    assert merge.returncode == 0, merge.stderr
    assert json.loads(merge.stdout)["operation"] == "merge"
    collision = run("epub_package_merge_harness.py", str(first), str(second), "--output", str(merged))
    assert collision.returncode == 1
    assert "refusing to overwrite" in collision.stderr

    split_dir = root / "split"
    split = run("epub_package_split_harness.py", str(first), "--output-dir", str(split_dir), "--split-points", "0")
    assert split.returncode == 0, split.stderr
    assert json.loads(split.stdout)["segments_created"] == 1

    metadata_output = root / "metadata.epub"
    metadata = run(
      "epub_metadata_edit_harness.py",
      str(first), "--output", str(metadata_output), "--metadata-json", '{"title":"新书名"}',
    )
    assert metadata.returncode == 0, metadata.stderr
    assert json.loads(metadata.stdout)["fields_updated"] >= 1

    cover = root / "cover.png"
    cover.write_bytes(b"png-cover")
    cover_output = root / "cover.epub"
    replaced = run(
      "epub_cover_replace_harness.py",
      str(first), "--output", str(cover_output), "--cover", str(cover),
    )
    assert replaced.returncode == 0, replaced.stderr
    assert json.loads(replaced.stdout)["operation"] == "replace-cover"


if __name__ == "__main__":
  test_focused_package_harnesses_and_non_overwrite()
  print("epub package harness tests ok")
