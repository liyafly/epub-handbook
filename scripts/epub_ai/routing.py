"""Public input inspection and skill-routing orchestration."""

from __future__ import annotations

from pathlib import Path

from .core import apply_workflow_mode, finding, inspect_directory, inspect_epub, inspect_source
from .report import Report


def inspect_path(path: Path, workflow_mode: str = "build") -> Report:
  """Inspect one input and route it to deterministic commands and skills."""
  report = Report(path, workflow_mode)
  if not path.exists():
    report.input_kind = "missing"
    report.findings.append(finding("error", "Input path does not exist"))
    return report
  if path.is_dir():
    inspect_directory(path, report)
  elif path.suffix.lower() == ".epub":
    inspect_epub(path, report)
  else:
    inspect_source(path, report)
  report.add_command("scripts/validate_skills_basic.py")
  if not report.findings:
    report.findings.append(finding("info", "No immediate structural issue detected by harness"))
  apply_workflow_mode(report)
  return report


__all__ = ["inspect_path"]
