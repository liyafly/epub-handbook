#!/usr/bin/env python3
"""Regression tests for epub_cleanup_pipeline.py."""

from __future__ import annotations

import json
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_cleanup_pipeline as pipeline  # noqa: E402
from epub_cleanup_pipeline import PipelineError, run_pipeline  # noqa: E402
from test_epub_cleanup_harnesses import write_epub2  # noqa: E402
from test_epub3_oneclick_converter import write_legacy_epub  # noqa: E402


def tamper_body_text(path: Path) -> None:
  entries: list[tuple[zipfile.ZipInfo, bytes]] = []
  changed = False
  with zipfile.ZipFile(path) as source:
    for info in source.infolist():
      data = source.read(info.filename)
      if info.filename == "OEBPS/Text/chapter.xhtml":
        altered = data.replace("正文保留不变".encode(), "正文已经变化".encode())
        changed = altered != data
        data = altered
      entries.append((info, data))
  assert changed, "fixture body text was not found"
  with zipfile.ZipFile(path, "w") as target:
    for info, data in entries:
      target.writestr(info, data)


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
    assert not (root / "audit" / "reports" / "preflight-before.json").exists(), report
    assert not (root / "audit" / "reports" / "conversion.json").exists(), report
    assert not (root / "audit" / "reports" / "preflight-after.json").exists(), report
    assert not (root / "audit" / "reports" / "validate-popup-notes.txt").exists(), report
    assert not (root / "audit" / "reports" / "validate-redline-subset.txt").exists(), report
    assert not (root / "audit" / "reports" / "validate-redline-text.txt").exists(), report
    assert not (root / "audit" / "reports" / "refinement.json").exists(), report
    assert not (root / "audit" / "reports" / "findings.json").exists(), report
    compact_data = json.loads((root / "audit" / "reports" / "pipeline.json").read_text(encoding="utf-8"))
    assert compact_data["image_layout_advisor"]["version"] == "1", compact_data
    assert any(step["name"] == "image-layout-advisor" for step in compact_data["steps"]), compact_data

    detailed = run_pipeline(source, root / "detailed-audit", keep_step_reports=True)
    assert detailed.status == "complete", detailed
    assert (root / "detailed-audit" / "reports" / "pipeline.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "preflight-before.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "conversion.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "preflight-after.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "validate-popup-notes.txt").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "validate-redline-subset.txt").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "validate-redline-text.txt").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "refinement.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "findings.json").exists(), detailed
    assert (root / "detailed-audit" / "reports" / "image-layout-advisor.json").exists(), detailed

    without_advisor = run_pipeline(source, root / "no-advisor-audit", image_advisor=False)
    without_data = json.loads(
      (root / "no-advisor-audit" / "reports" / "pipeline.json").read_text(encoding="utf-8")
    )
    assert "image_layout_advisor" not in without_data, without_data
    assert not any(step["name"] == "image-layout-advisor" for step in without_data["steps"]), without_data

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
    assert not (root / "blocked-audit" / "reports" / "preflight-before.json").exists()
    assert not (root / "blocked-audit" / "after" / "cleaned.epub").exists()

    original_run_step = pipeline.run_step

    def tampering_run_step(*args: object, **kwargs: object) -> object:
      result = original_run_step(*args, **kwargs)
      name = args[1]
      command = args[2]
      if name == "convert-epub3":
        assert isinstance(command, list)
        output = Path(command[command.index("--output") + 1])
        tamper_body_text(output)
      return result

    pipeline.run_step = tampering_run_step
    try:
      try:
        run_pipeline(source, root / "tampered-audit")
      except PipelineError:
        pass
      else:
        raise AssertionError("pipeline must stop when conversion changes body text")
    finally:
      pipeline.run_step = original_run_step

    failed = json.loads((root / "tampered-audit" / "reports" / "pipeline.json").read_text(encoding="utf-8"))
    text_gate = [step for step in failed["steps"] if step["name"] == "validate-redline-text"]
    assert len(text_gate) == 1 and text_gate[0]["returncode"] != 0, failed

    def failing_preflight_after(*args: object, **kwargs: object) -> object:
      result = original_run_step(*args, **kwargs)
      if args[1] == "preflight-after":
        return {"preflight_status": "fail"}
      return result

    pipeline.run_step = failing_preflight_after
    try:
      try:
        run_pipeline(source, root / "preflight-after-fail-audit")
      except PipelineError:
        pass
      else:
        raise AssertionError("pipeline must stop when preflight-after reports fail")
    finally:
      pipeline.run_step = original_run_step
    assert not (root / "preflight-after-fail-audit" / "after" / "cleaned.epub").exists()

  print("epub cleanup pipeline tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
