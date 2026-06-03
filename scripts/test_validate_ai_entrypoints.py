#!/usr/bin/env python3
"""Regression tests for validate_ai_entrypoints.py."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "validate_ai_entrypoints.py"
sys.path.insert(0, str(ROOT / "scripts"))

import validate_ai_entrypoints as V  # noqa: E402


def test_current_repo_entrypoints() -> None:
  result = subprocess.run(
    [sys.executable, str(SCRIPT)],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )
  if result.returncode:
    raise AssertionError(f"current repo entrypoints should validate:\n{result.stdout}\n{result.stderr}")


def test_read_required_reports_missing_file() -> None:
  errors: list[str] = []
  text = V.read_required(ROOT / "__missing_ai_entrypoint__.md", errors)
  if text != "" or not errors or "missing required file" not in errors[0]:
    raise AssertionError(f"missing file was not reported: text={text!r}, errors={errors}")


def test_require_tokens_reports_missing_token() -> None:
  errors: list[str] = []
  V.require_tokens("sample", "present", ("present", "missing"), errors)
  if errors != ["sample: missing required reference: missing"]:
    raise AssertionError(f"missing token was not reported exactly: {errors}")


def main() -> int:
  test_current_repo_entrypoints()
  test_read_required_reports_missing_file()
  test_require_tokens_reports_missing_token()
  print("validate_ai_entrypoints tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
