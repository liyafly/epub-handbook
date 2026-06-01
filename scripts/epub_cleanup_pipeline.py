#!/usr/bin/env python3
"""Run the auditable existing-EPUB cleanup baseline with one command.

The pipeline keeps the conservative boundaries from AGENTS.md:
- preserve an immutable before copy;
- stop on failed preflight;
- make structure normalization opt-in and require an explicit approval flag;
- run the existing EPUB3 converter as the write step;
- collect post-conversion validation, refinement, and AI findings reports;
- leave visual diff review and reader testing to the operator.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"


class PipelineError(Exception):
  """The cleanup pipeline cannot continue safely."""


@dataclass
class Step:
  name: str
  command: list[str]
  returncode: int
  report: str | None = None
  stdout: str = ""
  stderr: str = ""

  def as_dict(self) -> dict[str, object]:
    data: dict[str, object] = {
      "name": self.name,
      "command": self.command,
      "returncode": self.returncode,
    }
    if self.report:
      data["report"] = self.report
    if self.stdout.strip():
      data["stdout"] = self.stdout.strip()
    if self.stderr.strip():
      data["stderr"] = self.stderr.strip()
    return data


@dataclass
class PipelineReport:
  input: str
  work_dir: str
  before: str
  output: str
  normalize: str
  status: str = "running"
  base: str = ""
  reports_dir: str = ""
  keep_step_reports: bool = False
  steps: list[Step] = field(default_factory=list)
  manual_review: list[str] = field(default_factory=lambda: [
    "Review the EPUB diff in Calibre Editor or VS Code.",
    "Inspect typography role assignments; the pipeline adds a palette but does not invent semantic classes.",
    "Import the output into target reading systems and write results back to docs/final/reader-matrix.yaml.",
  ])

  def as_dict(self) -> dict[str, object]:
    return {
      "harness": "epub_cleanup_pipeline",
      "status": self.status,
      "input": self.input,
      "work_dir": self.work_dir,
      "before": self.before,
      "base": self.base,
      "output": self.output,
      "reports_dir": self.reports_dir,
      "keep_step_reports": self.keep_step_reports,
      "normalize": self.normalize,
      "steps": [step.as_dict() for step in self.steps],
      "manual_review": self.manual_review,
    }


def write_json(path: Path, data: dict[str, Any]) -> None:
  path.parent.mkdir(parents=True, exist_ok=True)
  path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def read_json(text: str, label: str) -> dict[str, Any]:
  try:
    value = json.loads(text)
  except json.JSONDecodeError as exc:
    raise PipelineError(f"{label}: expected JSON output: {exc}") from exc
  if not isinstance(value, dict):
    raise PipelineError(f"{label}: expected a JSON object")
  return value


def run_step(
  report: PipelineReport,
  name: str,
  command: list[str],
  report_path: Path | None = None,
  expect_json: bool = False,
  allowed_codes: tuple[int, ...] = (0,),
) -> dict[str, Any] | None:
  result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, check=False)
  step = Step(
    name=name,
    command=command,
    returncode=result.returncode,
    report=str(report_path) if report_path else None,
    stdout=result.stdout,
    stderr=result.stderr,
  )
  report.steps.append(step)
  data: dict[str, Any] | None = None
  if expect_json:
    data = read_json(result.stdout, name)
    if report_path:
      write_json(report_path, data)
  elif report_path:
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(result.stdout + result.stderr, encoding="utf-8")
  if result.returncode not in allowed_codes:
    raise PipelineError(f"{name}: command failed with exit code {result.returncode}")
  return data


def ensure_empty_target(path: Path, label: str) -> None:
  if path.exists():
    raise PipelineError(f"{label} already exists: {path}")


def preserve_before(input_path: Path, before_path: Path) -> None:
  ensure_empty_target(before_path, "before copy")
  before_path.parent.mkdir(parents=True, exist_ok=True)
  shutil.copy2(input_path, before_path)


def optional_report(reports_dir: Path, name: str, keep_step_reports: bool) -> Path | None:
  return reports_dir / name if keep_step_reports else None


def run_pipeline(
  input_path: Path,
  work_dir: Path,
  normalize: str = "skip",
  approve_normalize: bool = False,
  popup_notes: bool = True,
  typography: bool = True,
  keep_step_reports: bool = False,
) -> PipelineReport:
  input_path = input_path.resolve()
  work_dir = work_dir.resolve()
  before_path = work_dir / "before" / "source.epub"
  after_dir = work_dir / "after"
  reports_dir = work_dir / "reports"
  output_path = after_dir / "cleaned.epub"
  pipeline_report_path = reports_dir / "pipeline.json"
  report = PipelineReport(
    input=str(input_path),
    work_dir=str(work_dir),
    before=str(before_path),
    output=str(output_path),
    reports_dir=str(reports_dir),
    normalize=normalize,
    keep_step_reports=keep_step_reports,
  )

  try:
    if not input_path.is_file():
      raise PipelineError(f"input EPUB does not exist: {input_path}")
    if normalize not in {"skip", "dry-run", "apply"}:
      raise PipelineError(f"unsupported normalize mode: {normalize}")
    if normalize == "apply" and not approve_normalize:
      raise PipelineError("--normalize apply requires --approve-normalize after reviewing a dry-run report")

    preserve_before(input_path, before_path)
    after_dir.mkdir(parents=True, exist_ok=True)
    reports_dir.mkdir(parents=True, exist_ok=True)

    preflight_before = run_step(
      report,
      "preflight-before",
      [sys.executable, str(SCRIPTS / "epub_preflight_harness.py"), str(before_path), "--format", "json"],
      optional_report(reports_dir, "preflight-before.json", keep_step_reports),
      expect_json=True,
      allowed_codes=(0, 1),
    )
    if preflight_before and preflight_before.get("preflight_status") == "fail":
      raise PipelineError("preflight-before: blocking findings must be fixed before conversion")

    base_path = before_path
    if normalize in {"dry-run", "apply"}:
      normalized_path = after_dir / "step-0-normalized.epub"
      normalize_command = [
        sys.executable,
        str(SCRIPTS / "epub_structure_tool.py"),
        "normalize",
        str(before_path),
        "--output",
        str(normalized_path),
        "--report-format",
        "json",
      ]
      if normalize == "dry-run":
        normalize_command.append("--dry-run")
      run_step(
        report,
        f"normalize-{normalize}",
        normalize_command,
        reports_dir / f"normalize-{normalize}.json",
        expect_json=True,
      )
      if normalize == "dry-run":
        report.status = "normalize-review-required"
        report.manual_review.insert(
          0,
          "Review reports/normalize-dry-run.json, then rerun in a fresh work directory with --normalize apply --approve-normalize.",
        )
        return report
      base_path = normalized_path
      run_step(
        report,
        "validate-normalized-text",
        [
          sys.executable,
          str(SCRIPTS / "validate_text_invariance.py"),
          str(before_path),
          str(base_path),
          "--check",
          "all",
          "--path-map",
          str(reports_dir / "normalize-apply.json"),
        ],
        optional_report(reports_dir, "validate-normalized-text.txt", keep_step_reports),
      )

    report.base = str(base_path)
    ensure_empty_target(output_path, "cleanup output")
    convert_command = [
      sys.executable,
      str(SCRIPTS / "epub3_oneclick_converter.py"),
      str(base_path),
      "--output",
      str(output_path),
      "--format",
      "json",
    ]
    if not popup_notes:
      convert_command.append("--no-popup-notes")
    if not typography:
      convert_command.append("--no-typography")
    run_step(
      report,
      "convert-epub3",
      convert_command,
      optional_report(reports_dir, "conversion.json", keep_step_reports),
      expect_json=True,
    )

    preflight_after = run_step(
      report,
      "preflight-after",
      [sys.executable, str(SCRIPTS / "epub_preflight_harness.py"), str(output_path), "--format", "json"],
      optional_report(reports_dir, "preflight-after.json", keep_step_reports),
      expect_json=True,
      allowed_codes=(0, 1),
    )
    if preflight_after and preflight_after.get("preflight_status") == "fail":
      raise PipelineError("preflight-after: converter produced blocking findings")

    if popup_notes:
      run_step(
        report,
        "validate-popup-notes",
        ["bash", str(SCRIPTS / "validate-popup-notes.sh"), "--epub", str(output_path)],
        optional_report(reports_dir, "validate-popup-notes.txt", keep_step_reports),
      )

    run_step(
      report,
      "validate-redline-subset",
      [
        sys.executable,
        str(SCRIPTS / "validate_text_invariance.py"),
        str(base_path),
        str(output_path),
        "--check",
        "metadata,drm,anchors",
        "--allow-list",
        "*/nav.xhtml",
      ],
      optional_report(reports_dir, "validate-redline-subset.txt", keep_step_reports),
    )
    run_step(
      report,
      "validate-redline-text",
      [
        sys.executable,
        str(SCRIPTS / "validate_text_invariance.py"),
        str(base_path),
        str(output_path),
        "--check",
        "text",
        "--allow-list",
        "*/nav.xhtml",
        "--allow-list",
        "*/toc.ncx",
      ],
      optional_report(reports_dir, "validate-redline-text.txt", keep_step_reports),
    )
    run_step(
      report,
      "refinement",
      [sys.executable, str(SCRIPTS / "epub_refinement_harness.py"), str(output_path), "--format", "json"],
      optional_report(reports_dir, "refinement.json", keep_step_reports),
      expect_json=True,
    )
    run_step(
      report,
      "ai-findings",
      [sys.executable, str(SCRIPTS / "epub_ai_harness.py"), "--mode", "cleanup", str(output_path), "--format", "json"],
      optional_report(reports_dir, "findings.json", keep_step_reports),
      expect_json=True,
    )
    report.status = "complete"
    return report
  except PipelineError:
    report.status = "failed"
    raise
  finally:
    if reports_dir.exists():
      write_json(pipeline_report_path, report.as_dict())


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Run the auditable existing-EPUB cleanup baseline with one command")
  parser.add_argument("input", type=Path, help="Input EPUB; the pipeline copies it to <work-dir>/before/source.epub")
  parser.add_argument("--work-dir", type=Path, default=Path("work"), help="Fresh audit directory; defaults to ./work")
  parser.add_argument(
    "--normalize",
    choices=("skip", "dry-run", "apply"),
    default="skip",
    help="Optional structure normalization; apply requires a reviewed dry-run and --approve-normalize",
  )
  parser.add_argument("--approve-normalize", action="store_true", help="Confirm that the normalization dry-run was reviewed")
  parser.add_argument("--no-popup-notes", action="store_true", help="Skip plain/duokan footnote normalization")
  parser.add_argument("--no-typography", action="store_true", help="Skip CJK typography role stylesheet injection")
  parser.add_argument(
    "--keep-step-reports",
    action="store_true",
    help="Write detailed per-step reports in addition to the compact reports/pipeline.json summary",
  )
  parser.add_argument("--format", choices=("json", "text"), default="text")
  args = parser.parse_args(argv)

  try:
    report = run_pipeline(
      args.input,
      args.work_dir,
      normalize=args.normalize,
      approve_normalize=args.approve_normalize,
      popup_notes=not args.no_popup_notes,
      typography=not args.no_typography,
      keep_step_reports=args.keep_step_reports,
    )
  except PipelineError as exc:
    data = {"harness": "epub_cleanup_pipeline", "status": "failed", "error": str(exc)}
    if args.format == "json":
      print(json.dumps(data, ensure_ascii=False, indent=2))
    else:
      print(f"ERROR: {exc}", file=sys.stderr)
    return 1

  data = report.as_dict()
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(f"Pipeline status: {report.status}")
    print(f"Before copy: {report.before}")
    if report.base:
      print(f"Conversion base: {report.base}")
    if report.status == "complete":
      print(f"Output EPUB: {report.output}")
    print(f"Audit reports: {report.reports_dir}")
    for item in report.manual_review:
      print(f"- {item}")
  return 0 if report.status == "complete" else 2


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
