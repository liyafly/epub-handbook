#!/usr/bin/env python3
"""Preflight an EPUB before AI-assisted cleanup.

This is the first machine gate for an existing EPUB. It wraps the broader
`epub_ai_harness` checks into a purpose-built preflight report so humans and AI
agents can stop before touching content when the package itself is broken.
"""

from __future__ import annotations

import argparse
import json
import shutil
import sys
from pathlib import Path

from epub_ai_harness import inspect_path, render_markdown, shell_path


def preflight(path: Path) -> tuple[int, dict[str, object], str]:
  report = inspect_path(path, "cleanup")
  report.tools["epubcheck"] = shutil.which("epubcheck") is not None

  if path.suffix.lower() == ".epub":
    report.add_command(f"python3 scripts/epub_preflight_harness.py {shell_path(path)} --format json")
    if report.tools["epubcheck"]:
      report.add_command(f"epubcheck {shell_path(path)}")
    else:
      report.add_command("# optional: install epubcheck, then run epubcheck <book.epub>")
    report.add_command(f"python3 scripts/epub_refinement_harness.py {shell_path(path)} --format json")

  data = report.as_dict()
  counts = data["findings_by_level"]
  status = "fail" if counts.get("error", 0) else "warn" if counts.get("warn", 0) else "pass"
  data.update({
    "harness": "epub_preflight_harness",
    "preflight_status": status,
    "next_gate": "Fix all error findings before EPUB3 migration, skill cleanup, or diff review.",
  })

  markdown = render_markdown(report)
  markdown = markdown.replace("# EPUB AI Harness Report", "# EPUB Preflight Harness Report", 1)
  markdown += f"\n## Gate\n\n- Status: `{status}`\n- Rule: fix all `error` findings before cleanup.\n"
  return (1 if status == "fail" else 0), data, markdown


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Run the EPUB cleanup preflight gate")
  parser.add_argument("path", type=Path, help="Existing EPUB file")
  parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  code, data, markdown = preflight(args.path)
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(markdown, end="")
  return code


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
