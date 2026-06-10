#!/usr/bin/env python3
"""Record and query reusable EPUB typesetting decisions."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from datetime import date
from pathlib import Path


ID_RE = re.compile(r"^dec-\d{4}$")
SOURCES = {"manual-review", "handshake", "refinement"}
SCENES = {
  "image-layout",
  "popup-note",
  "font-chain",
  "chapter-head",
  "poster",
  "vertical",
  "css-layering",
  "other",
}
SCOPES = {"book", "global"}
CONTEXT_KEYS = {
  "selector",
  "classes",
  "structure",
  "reader",
  "readers",
  "reader_version",
  "artifact",
}
REQUIRED_FIELDS = {
  "id",
  "date",
  "source",
  "book",
  "scene",
  "finding",
  "context",
  "candidates",
  "chosen",
  "rationale",
  "scope",
  "reusable",
}


class DecisionLogError(ValueError):
  """The decision log or requested entry is invalid."""


def validate_context(context: object) -> dict[str, object]:
  if not isinstance(context, dict):
    raise DecisionLogError("context must be an object")
  unknown = sorted(set(context) - CONTEXT_KEYS)
  if unknown:
    raise DecisionLogError(f"context keys are not allowed: {', '.join(unknown)}")
  for key, value in context.items():
    if isinstance(value, str):
      continue
    if key in {"classes", "readers"} and isinstance(value, list) and all(isinstance(item, str) for item in value):
      continue
    raise DecisionLogError(f"context value must be structural text or a string list: {key}")
  return context


def validate_record(record: object) -> dict[str, object]:
  if not isinstance(record, dict):
    raise DecisionLogError("record must be a JSON object")
  missing = sorted(REQUIRED_FIELDS - set(record))
  extra = sorted(set(record) - REQUIRED_FIELDS)
  if missing:
    raise DecisionLogError(f"missing fields: {', '.join(missing)}")
  if extra:
    raise DecisionLogError(f"unsupported fields: {', '.join(extra)}")
  record_id = record["id"]
  if not isinstance(record_id, str) or not ID_RE.fullmatch(record_id):
    raise DecisionLogError("id must match dec-0000")
  try:
    date.fromisoformat(str(record["date"]))
  except ValueError as exc:
    raise DecisionLogError("date must use YYYY-MM-DD") from exc
  if record["source"] not in SOURCES:
    raise DecisionLogError(f"unsupported source: {record['source']}")
  if record["scene"] not in SCENES:
    raise DecisionLogError(f"unsupported scene: {record['scene']}")
  if record["scope"] not in SCOPES:
    raise DecisionLogError(f"unsupported scope: {record['scope']}")
  for field in ("book", "finding", "chosen", "rationale"):
    if not isinstance(record[field], str):
      raise DecisionLogError(f"{field} must be a string")
  if not record["finding"].strip() or not record["chosen"].strip() or not record["rationale"].strip():
    raise DecisionLogError("finding, chosen, and rationale must not be empty")
  candidates = record["candidates"]
  if not isinstance(candidates, list) or not candidates or not all(isinstance(item, str) and item for item in candidates):
    raise DecisionLogError("candidates must be a non-empty string list")
  if record["chosen"] not in candidates:
    raise DecisionLogError("chosen must be one of candidates")
  validate_context(record["context"])
  if not isinstance(record["reusable"], bool):
    raise DecisionLogError("reusable must be boolean")
  if record["reusable"] != (record["scope"] == "global"):
    raise DecisionLogError("reusable must be true exactly when scope is global")
  return record


def load_records(path: Path) -> list[dict[str, object]]:
  if not path.exists():
    return []
  records: list[dict[str, object]] = []
  ids: set[str] = set()
  for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
    if not raw.strip():
      continue
    try:
      parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
      raise DecisionLogError(f"{path}: line {line_number}: invalid JSON: {exc.msg}") from exc
    try:
      record = validate_record(parsed)
    except DecisionLogError as exc:
      raise DecisionLogError(f"{path}: line {line_number}: {exc}") from exc
    record_id = str(record["id"])
    if record_id in ids:
      raise DecisionLogError(f"{path}: line {line_number}: duplicate id: {record_id}")
    ids.add(record_id)
    records.append(record)
  return records


def parse_context(values: list[str]) -> dict[str, object]:
  context: dict[str, object] = {}
  for raw in values:
    key, separator, value = raw.partition("=")
    key = key.strip()
    if not separator or not key or not value.strip():
      raise DecisionLogError("context must use KEY=VALUE")
    if key in context:
      raise DecisionLogError(f"duplicate context key: {key}")
    if key not in CONTEXT_KEYS:
      raise DecisionLogError(f"context key is not allowed: {key}")
    if key in {"classes", "readers"}:
      items = [item.strip() for item in value.split(",") if item.strip()]
      if not items:
        raise DecisionLogError(f"context list must not be empty: {key}")
      context[key] = items
    else:
      context[key] = value.strip()
  return validate_context(context)


def next_id(records: list[dict[str, object]]) -> str:
  highest = max((int(str(record["id"]).split("-", 1)[1]) for record in records), default=0)
  return f"dec-{highest + 1:04d}"


def write_records(path: Path, records: list[dict[str, object]]) -> None:
  path.parent.mkdir(parents=True, exist_ok=True)
  tmp = path.with_name(f".{path.name}.{os.getpid()}.tmp")
  try:
    with tmp.open("w", encoding="utf-8", newline="\n") as handle:
      for record in records:
        handle.write(json.dumps(record, ensure_ascii=False, separators=(",", ":")) + "\n")
      handle.flush()
      os.fsync(handle.fileno())
    os.replace(tmp, path)
  finally:
    if tmp.exists():
      tmp.unlink()


def render(records: list[dict[str, object]], output_format: str) -> str:
  if output_format == "json":
    return json.dumps(records, ensure_ascii=False, indent=2)
  lines = [
    "| id | date | scene | finding | chosen | scope | rationale |",
    "| --- | --- | --- | --- | --- | --- | --- |",
  ]
  for record in records:
    values = [
      str(record[key]).replace("|", "\\|").replace("\n", " ")
      for key in ("id", "date", "scene", "finding", "chosen", "scope", "rationale")
    ]
    lines.append("| " + " | ".join(values) + " |")
  return "\n".join(lines)


def build_parser() -> argparse.ArgumentParser:
  parser = argparse.ArgumentParser(description="Record and query EPUB typesetting decisions")
  subparsers = parser.add_subparsers(dest="command", required=True)

  add = subparsers.add_parser("add", help="Append one validated decision")
  add.add_argument("--file", type=Path, required=True)
  add.add_argument("--date", dest="decision_date", default=date.today().isoformat())
  add.add_argument("--source", choices=sorted(SOURCES), required=True)
  add.add_argument("--book", default="")
  add.add_argument("--scene", choices=sorted(SCENES), required=True)
  add.add_argument("--finding", required=True)
  add.add_argument("--context", action="append", default=[], metavar="KEY=VALUE")
  add.add_argument("--candidates", required=True, help="Comma-separated candidate ids")
  add.add_argument("--chosen", required=True)
  add.add_argument("--rationale", required=True)
  add.add_argument("--scope", choices=sorted(SCOPES), required=True)

  list_parser = subparsers.add_parser("list", help="List decisions with optional filters")
  list_parser.add_argument("--file", type=Path, required=True)
  list_parser.add_argument("--scene", choices=sorted(SCENES))
  list_parser.add_argument("--finding")
  list_parser.add_argument("--scope", choices=sorted(SCOPES))
  list_parser.add_argument("--format", choices=("json", "md"), default="json")

  match = subparsers.add_parser("match", help="Match decisions by scene and finding")
  match.add_argument("--file", type=Path, required=True)
  match.add_argument("--scene", choices=sorted(SCENES), required=True)
  match.add_argument("--finding", required=True)
  match.add_argument("--format", choices=("json", "md"), default="json")
  return parser


def main(argv: list[str] | None = None) -> int:
  parser = build_parser()
  args = parser.parse_args(sys.argv[1:] if argv is None else argv)
  try:
    records = load_records(args.file)
    if args.command == "add":
      candidates = [candidate.strip() for candidate in args.candidates.split(",") if candidate.strip()]
      record = validate_record({
        "id": next_id(records),
        "date": args.decision_date,
        "source": args.source,
        "book": args.book,
        "scene": args.scene,
        "finding": args.finding,
        "context": parse_context(args.context),
        "candidates": candidates,
        "chosen": args.chosen,
        "rationale": args.rationale,
        "scope": args.scope,
        "reusable": args.scope == "global",
      })
      write_records(args.file, [*records, record])
      print(json.dumps(record, ensure_ascii=False, indent=2))
      return 0

    filtered = [
      record
      for record in records
      if (not getattr(args, "scene", None) or record["scene"] == args.scene)
      and (not getattr(args, "finding", None) or record["finding"] == args.finding)
      and (not getattr(args, "scope", None) or record["scope"] == args.scope)
    ]
    print(render(filtered, args.format))
    return 0
  except DecisionLogError as exc:
    parser.error(str(exc))
  return 2


if __name__ == "__main__":
  raise SystemExit(main())
