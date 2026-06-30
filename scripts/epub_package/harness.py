"""Shared CLI safeguards for focused package harnesses."""

from __future__ import annotations

import json
import sys
from pathlib import Path

from .models import PackageToolError


def ensure_new_file(path: Path) -> None:
  if path.exists():
    raise PackageToolError(f"refusing to overwrite existing output: {path}")


def ensure_empty_output_dir(path: Path) -> None:
  if path.exists() and any(path.iterdir()):
    raise PackageToolError(f"refusing to write into non-empty output directory: {path}")


def emit(value: object) -> None:
  if hasattr(value, "as_dict"):
    value = value.as_dict()
  print(json.dumps(value, ensure_ascii=False, indent=2))


def fail(exc: Exception) -> int:
  print(f"ERROR: {exc}", file=sys.stderr)
  return 1
