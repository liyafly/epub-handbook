#!/usr/bin/env python3
"""Regression tests for epub_cleanup_pipeline.py."""

from __future__ import annotations

import sys
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub_cleanup_pipeline import PipelineError, run_pipeline  # noqa: E402
from test_epub_cleanup_harnesses import write_epub2  # noqa: E402
from test_epub3_oneclick_converter import write_legacy_epub  # noqa: E402


def main() -> int:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    source = root / "source-epub2.epub"
    write_epub2(source)

    report = run_pipeline(source, root / "audit")
    assert report.status == "complete", report
    assert Path(report.before).exists(), report
    assert Path(report.output).exists(), report
    assert (root / "audit" / "reports" / "pipeline.json").exists(), report
    assert (root / "audit" / "reports" / "preflight-before.json").exists(), report
    assert (root / "audit" / "reports" / "conversion.json").exists(), report
    assert (root / "audit" / "reports" / "preflight-after.json").exists(), report
    assert (root / "audit" / "reports" / "validate-popup-notes.txt").exists(), report
    assert (root / "audit" / "reports" / "validate-redline-subset.txt").exists(), report
    assert (root / "audit" / "reports" / "refinement.json").exists(), report
    assert (root / "audit" / "reports" / "findings.json").exists(), report

    dry_run = run_pipeline(source, root / "normalize-audit", normalize="dry-run")
    assert dry_run.status == "normalize-review-required", dry_run
    assert (root / "normalize-audit" / "reports" / "normalize-dry-run.json").exists(), dry_run
    assert not Path(dry_run.output).exists(), dry_run

    blocked_source = root / "blocked.epub"
    write_legacy_epub(blocked_source)
    try:
      run_pipeline(blocked_source, root / "blocked-audit")
    except PipelineError:
      pass
    else:
      raise AssertionError("pipeline must stop on blocking preflight findings")
    assert (root / "blocked-audit" / "reports" / "pipeline.json").exists()
    assert (root / "blocked-audit" / "reports" / "preflight-before.json").exists()
    assert not (root / "blocked-audit" / "after" / "cleaned.epub").exists()

  print("epub cleanup pipeline tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
