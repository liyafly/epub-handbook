#!/usr/bin/env python3
"""Run allow-listed Python CLI providers from a file-based JSON request.

This adapter belongs exclusively to the Python CLI / AI Agent surface. Swift
packages and GUI targets do not import, invoke, or depend on it.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / "adapters" / "python" / "provider-catalog.v1.json"


class ProviderRequestError(Exception):
  """The request cannot be routed to an allow-listed provider."""


def load_catalog() -> dict[str, dict[str, Any]]:
  try:
    payload = json.loads(CATALOG.read_text(encoding="utf-8"))
  except (OSError, json.JSONDecodeError) as exc:
    raise ProviderRequestError(f"cannot read provider catalog: {exc}") from exc
  if not isinstance(payload, dict) or payload.get("schemaVersion") != "1":
    raise ProviderRequestError("provider catalog schemaVersion must be 1")
  capabilities = payload.get("capabilities")
  if not isinstance(capabilities, list):
    raise ProviderRequestError("provider catalog capabilities must be a list")
  records: dict[str, dict[str, Any]] = {}
  for item in capabilities:
    if not isinstance(item, dict) or not isinstance(item.get("id"), str):
      raise ProviderRequestError("provider catalog contains an invalid capability")
    records[item["id"]] = item
  return records


def artifact_path(request: dict[str, Any]) -> Path:
  artifact = request.get("artifact")
  if not isinstance(artifact, dict) or not isinstance(artifact.get("uri"), str):
    raise ProviderRequestError("request artifact.uri must be a string")
  uri = artifact["uri"]
  split = urlsplit(uri)
  if split.scheme not in {"", "file"}:
    raise ProviderRequestError("request artifact.uri must be a local file URI")
  path = Path(unquote(split.path if split.scheme else uri))
  if not path.is_absolute():
    raise ProviderRequestError("request artifact.uri must be an absolute path")
  return path


def render_arguments(template: list[Any], path: Path) -> list[str]:
  if not all(isinstance(item, str) for item in template):
    raise ProviderRequestError("provider catalog arguments must be strings")
  return [item.replace("{artifact.path}", str(path)) for item in template]


def normalized_status(return_code: int, legacy_report: Any) -> str:
  if return_code != 0:
    return "failed"
  if isinstance(legacy_report, dict) and legacy_report.get("preflight_status") == "fail":
    return "failed"
  return "complete"


def normalize_inspection(request: dict[str, Any], legacy_report: Any) -> dict[str, Any] | None:
  """Project a compatible Python preflight JSON report into the shared shape."""
  if not isinstance(legacy_report, dict) or "findings" not in legacy_report:
    return None
  level_to_severity = {"info": "info", "warn": "warn", "error": "error", "fatal": "fatal"}
  findings: list[dict[str, Any]] = []
  raw_findings = legacy_report.get("findings")
  if isinstance(raw_findings, list):
    for index, finding in enumerate(raw_findings):
      if not isinstance(finding, dict):
        continue
      level = finding.get("level")
      message = finding.get("message")
      if not isinstance(level, str) or not isinstance(message, str):
        continue
      severity = level_to_severity.get(level, "warn")
      findings.append({
        "code": f"python.{legacy_report.get('harness', 'provider')}.{index}",
        "severity": severity,
        "message": message,
      })
  reported_status = legacy_report.get("preflight_status")
  if reported_status not in {"pass", "warn", "fail"}:
    reported_status = "fail" if any(item["severity"] in {"error", "fatal"} for item in findings) else "pass"
  return {
    "schemaVersion": "1",
    "artifact": request["artifact"],
    "status": reported_status,
    "findings": findings,
  }


def run(request_path: Path, result_path: Path) -> int:
  try:
    request = json.loads(request_path.read_text(encoding="utf-8"))
  except (OSError, json.JSONDecodeError) as exc:
    print(f"ERROR: cannot read request JSON: {exc}", file=sys.stderr)
    return 2
  if not isinstance(request, dict) or request.get("schemaVersion") != "1":
    print("ERROR: request schemaVersion must be 1", file=sys.stderr)
    return 2
  capability = request.get("capability")
  if not isinstance(capability, str):
    print("ERROR: request capability must be a string", file=sys.stderr)
    return 2
  try:
    record = load_catalog().get(capability)
    if record is None:
      raise ProviderRequestError(f"unsupported capability: {capability}")
    path = artifact_path(request)
    entrypoint = record.get("entrypoint")
    if not isinstance(entrypoint, str):
      raise ProviderRequestError(f"provider catalog entrypoint missing for {capability}")
    script = (ROOT / entrypoint).resolve()
    if ROOT not in script.parents or not script.is_file():
      raise ProviderRequestError(f"provider entrypoint is not an allowed repository file: {entrypoint}")
    args = render_arguments(record.get("arguments", []), path)
  except ProviderRequestError as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 2

  process = subprocess.run(
    [sys.executable, str(script), *args],
    cwd=ROOT,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    check=False,
  )
  try:
    legacy_report = json.loads(process.stdout)
  except json.JSONDecodeError:
    legacy_report = {"rawStdout": process.stdout}
  response = {
    "schemaVersion": "1",
    "provider": "python",
    "capability": capability,
    "status": normalized_status(process.returncode, legacy_report),
    "exitCode": process.returncode,
    "input": request["artifact"],
    "legacyReport": legacy_report,
    "stderr": process.stderr,
  }
  inspection = normalize_inspection(request, legacy_report)
  if inspection is not None:
    response["normalizedInspection"] = inspection
  result_path.parent.mkdir(parents=True, exist_ok=True)
  result_path.write_text(json.dumps(response, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
  return 0


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("--request", type=Path, required=True)
  parser.add_argument("--result", type=Path, required=True)
  args = parser.parse_args(argv)
  return run(args.request, args.result)


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
