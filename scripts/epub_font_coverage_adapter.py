#!/usr/bin/env python3
"""Public read-only adapter for tools-font/coverage-detector."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
TOOL_ROOT = ROOT / "tools-font" / "coverage-detector"
CAPABILITY = "epub.font.coverage.analyze"


class FontCoverageAdapterError(Exception):
  """The independent font detector could not produce a valid report."""


def status_for(report: dict[str, object], profile: str) -> str:
  summary = report.get("summary")
  if not isinstance(summary, dict):
    return "fail"
  profiles = summary.get("by_profile_risk")
  counts = profiles.get(profile, {}) if isinstance(profiles, dict) else {}
  if isinstance(counts, dict) and int(counts.get("fail", 0) or 0) > 0:
    return "fail"
  if (
    isinstance(counts, dict) and int(counts.get("risk", 0) or 0) > 0
    or int(summary.get("unresolved_runs", 0) or 0) > 0
  ):
    return "warn"
  return "pass"


def analyze(path: Path, profile: str = "kindle-pessimistic") -> dict[str, object]:
  path = path.resolve()
  if not path.is_file():
    raise FontCoverageAdapterError(f"input EPUB does not exist: {path}")
  if shutil.which("uv") is None:
    raise FontCoverageAdapterError("uv is required for tools-font/coverage-detector")
  process = subprocess.run(
    ["uv", "run", "python", "-m", "src.cli", str(path), "--profile", profile, "--json", "--quiet"],
    cwd=TOOL_ROOT,
    capture_output=True,
    text=True,
    check=False,
  )
  try:
    report = json.loads(process.stdout)
  except json.JSONDecodeError as exc:
    detail = process.stderr.strip() or process.stdout.strip() or f"exit code {process.returncode}"
    raise FontCoverageAdapterError(f"coverage detector did not return JSON: {detail}") from exc
  if not isinstance(report, dict) or report.get("schema_version") != "1.0":
    raise FontCoverageAdapterError("coverage detector returned an unsupported report schema")
  report["capability"] = CAPABILITY
  report["status"] = status_for(report, profile)
  report["profile"] = profile
  report["detector_exit_code"] = process.returncode
  if process.stderr.strip():
    report["detector_stderr"] = process.stderr.strip()
  return report


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("input", type=Path)
  parser.add_argument("--profile", default="kindle-pessimistic", choices=("ideal-browser", "kindle-pessimistic"))
  parser.add_argument("--format", choices=("json",), default="json")
  parser.add_argument("--output", type=Path)
  args = parser.parse_args(argv)
  try:
    report = analyze(args.input, args.profile)
  except FontCoverageAdapterError as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 2
  payload = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
  if args.output:
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(payload, encoding="utf-8")
  print(payload, end="")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
