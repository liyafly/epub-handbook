#!/usr/bin/env python3
"""Regression tests for epub_decision_log.py."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_decision_log.py"


def run_cli(*args: str) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), *args],
    check=False,
    capture_output=True,
    text=True,
  )


def add_args(path: Path, *, finding: str = "lone-image-no-figure") -> list[str]:
  return [
    "add",
    "--file",
    str(path),
    "--scene",
    "image-layout",
    "--finding",
    finding,
    "--candidates",
    "figure.img-left,figure.img-right,figure-fullwidth",
    "--chosen",
    "figure.img-right",
    "--rationale",
    "图注偏长，右浮动更稳",
    "--scope",
    "global",
    "--source",
    "manual-review",
  ]


def read_records(path: Path) -> list[dict[str, object]]:
  return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def test_add_and_increment_ids() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "decisions.jsonl"
    first = run_cli(*add_args(path))
    assert first.returncode == 0, first.stderr
    records = read_records(path)
    assert records[0]["id"] == "dec-0001"
    assert records[0]["scene"] == "image-layout"
    assert records[0]["context"] == {}
    assert records[0]["reusable"] is True

    second = run_cli(*add_args(path, finding="missing-alt"))
    assert second.returncode == 0, second.stderr
    records = read_records(path)
    assert [record["id"] for record in records] == ["dec-0001", "dec-0002"]


def test_missing_required_argument_does_not_modify_file() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "decisions.jsonl"
    ok = run_cli(*add_args(path))
    assert ok.returncode == 0, ok.stderr
    before = path.read_bytes()
    args = add_args(path)
    chosen_index = args.index("--chosen")
    del args[chosen_index:chosen_index + 2]
    result = run_cli(*args)
    assert result.returncode == 2
    assert path.read_bytes() == before


def test_long_rationale_allowed_and_text_context_rejected() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "decisions.jsonl"
    args = add_args(path)
    rationale_index = args.index("--rationale") + 1
    args[rationale_index] = "理由" * 50
    args.extend(["--context", "selector=div.pic > img", "--context", "readers=apple-books,kindle"])
    accepted = run_cli(*args)
    assert accepted.returncode == 0, accepted.stderr
    record = read_records(path)[0]
    assert len(str(record["rationale"])) > 80
    assert record["context"] == {
      "selector": "div.pic > img",
      "readers": ["apple-books", "kindle"],
    }

    before = path.read_bytes()
    rejected = run_cli(*add_args(path), "--context", "text=正文片段")
    assert rejected.returncode == 2
    assert "context" in rejected.stderr
    assert path.read_bytes() == before


def test_list_and_match_filters() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "decisions.jsonl"
    assert run_cli(*add_args(path)).returncode == 0
    second_args = add_args(path, finding="font-fallback")
    scene_index = second_args.index("--scene") + 1
    second_args[scene_index] = "font-chain"
    assert run_cli(*second_args).returncode == 0

    listed = run_cli("list", "--file", str(path), "--scene", "image-layout", "--format", "json")
    assert listed.returncode == 0, listed.stderr
    listed_records = json.loads(listed.stdout)
    assert [record["scene"] for record in listed_records] == ["image-layout"]

    matched = run_cli(
      "match",
      "--file",
      str(path),
      "--scene",
      "image-layout",
      "--finding",
      "lone-image-no-figure",
      "--format",
      "json",
    )
    assert matched.returncode == 0, matched.stderr
    assert len(json.loads(matched.stdout)) == 1

    missing = run_cli(
      "match",
      "--file",
      str(path),
      "--scene",
      "image-layout",
      "--finding",
      "not-recorded",
      "--format",
      "json",
    )
    assert missing.returncode == 0, missing.stderr
    assert json.loads(missing.stdout) == []


def test_corrupt_existing_file_blocks_add() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "decisions.jsonl"
    assert run_cli(*add_args(path)).returncode == 0
    with path.open("a", encoding="utf-8") as handle:
      handle.write("not-json\n")
    before = path.read_bytes()
    result = run_cli(*add_args(path))
    assert result.returncode == 2
    assert "line 2" in result.stderr
    assert path.read_bytes() == before


def main() -> int:
  test_add_and_increment_ids()
  test_missing_required_argument_does_not_modify_file()
  test_long_rationale_allowed_and_text_context_rejected()
  test_list_and_match_filters()
  test_corrupt_existing_file_blocks_add()
  print("epub decision log tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
