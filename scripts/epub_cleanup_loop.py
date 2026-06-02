#!/usr/bin/env python3
"""Run conservative deterministic package cleanup rounds for an existing EPUB.

The loop stages a safe EPUB 3 baseline first, then applies only allow-listed
package operations. XHTML prose is never rewritten by this module.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import posixpath
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path
from typing import Any
from xml.etree import ElementTree as ET

from epub_ai_harness import inspect_path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
OPF_URI = "http://www.idpf.org/2007/opf"
CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
NS = {"opf": OPF_URI, "c": CONTAINER_URI}
ET.register_namespace("", OPF_URI)
META_PROPERTY = "epub-handbook:cleanup-rounds"
META_PREFIX = "epub-handbook: https://github.com/liyafly/epub-handbook#"

class CleanupError(Exception):
  """Cleanup cannot continue without weakening a safety boundary."""

def _write_json(path: Path, data: dict[str, Any]) -> None:
  path.parent.mkdir(parents=True, exist_ok=True)
  path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

def _run(command: list[str]) -> subprocess.CompletedProcess[str]:
  return subprocess.run(command, cwd=ROOT, capture_output=True, text=True, check=False)

def _read_epub(path: Path) -> tuple[dict[str, bytes], list[str]]:
  try:
    with zipfile.ZipFile(path) as zf:
      return ({name: zf.read(name) for name in zf.namelist() if not name.endswith("/")}, zf.namelist())
  except zipfile.BadZipFile as exc:
    raise CleanupError(f"not a valid EPUB zip: {path}") from exc

def _write_epub(path: Path, files: dict[str, bytes], order: list[str]) -> Path:
  path.parent.mkdir(parents=True, exist_ok=True)
  with zipfile.ZipFile(path, "w") as zf:
    names = [name for name in order if name in files] + sorted(set(files) - set(order))
    if "mimetype" in files:
      zf.writestr("mimetype", files["mimetype"], compress_type=zipfile.ZIP_STORED)
    for name in names:
      if name != "mimetype":
        zf.writestr(name, files[name], compress_type=zipfile.ZIP_DEFLATED)
  return path

def _package(files: dict[str, bytes]) -> tuple[str, ET.Element]:
  try:
    container = ET.fromstring(files["META-INF/container.xml"])
    rootfile = container.find(".//c:rootfile", NS)
    opf_path = rootfile.attrib["full-path"] if rootfile is not None else ""
    return opf_path, ET.fromstring(files[opf_path])
  except (KeyError, ET.ParseError) as exc:
    raise CleanupError("EPUB container or OPF cannot be parsed") from exc

def _fingerprint(path: Path) -> str:
  files, _ = _read_epub(path)
  digest = hashlib.sha256()
  for name in sorted(files):
    digest.update(name.encode()); digest.update(b"\0"); digest.update(files[name])
  return digest.hexdigest()

def _text_gate(before: Path, after: Path, path_map: Path | None = None) -> tuple[bool, str]:
  command = [sys.executable, str(SCRIPTS / "validate_text_invariance.py"), str(before), str(after), "--check", "text", "--allow-list", "*/nav*.xhtml", "--allow-list", "*/toc.ncx"]
  if path_map is not None:
    command.extend(["--path-map", str(path_map)])
  result = _run(command)
  return result.returncode == 0, (result.stdout + result.stderr).strip()

def _epubcheck(path: Path) -> dict[str, Any]:
  executable = shutil.which("epubcheck")
  if not executable:
    return {"status": "skipped", "reason": "epubcheck-not-installed"}
  result = _run([executable, str(path)])
  return {"status": "pass" if result.returncode == 0 else "fail", "returncode": result.returncode, "output": (result.stdout + result.stderr).strip()}

def _audit(path: Path) -> dict[str, Any]:
  return inspect_path(path, "cleanup").as_dict()

def _needs_migration(report: dict[str, Any]) -> bool:
  summary = report.get("summary", {})
  return not str(summary.get("package_version", "")).startswith("3") or any("exactly one nav item" in str(item.get("message")) for item in report.get("findings", []))

def _blocking_preflight(report: dict[str, Any]) -> list[dict[str, Any]]:
  return [item for item in report.get("findings", []) if item.get("level") == "error" and not item.get("auto_fixable")]

def _stage_input(input_epub: Path, work: Path, normalize: str = "skip", approve_normalize: bool = False) -> tuple[Path, list[dict[str, Any]]]:
  before = work / "before" / "source.epub"
  if before.exists():
    raise CleanupError(f"before copy already exists: {before}")
  before.parent.mkdir(parents=True, exist_ok=True)
  shutil.copy2(input_epub, before)
  stages: list[dict[str, Any]] = []
  if normalize not in {"skip", "dry-run", "apply"}:
    raise CleanupError(f"unsupported normalize mode: {normalize}")
  if normalize == "apply" and not approve_normalize:
    raise CleanupError("normalize apply requires --approve-normalize after reviewing a dry-run report")
  preflight_result = _run([sys.executable, str(SCRIPTS / "epub_preflight_harness.py"), str(before), "--format", "json"])
  try:
    preflight = json.loads(preflight_result.stdout)
  except json.JSONDecodeError as exc:
    raise CleanupError("preflight-before did not emit JSON") from exc
  stages.append({"name": "preflight-before", "status": "pass" if not _blocking_preflight(preflight) else "fail", "findings": preflight["findings"]})
  if _blocking_preflight(preflight):
    raise CleanupError("preflight-before: blocking findings must be fixed before cleanup")
  base = before
  if normalize in {"dry-run", "apply"}:
    normalized = work / "after" / "step-0-normalized.epub"
    normalize_report = work / "reports" / f"normalize-{normalize}.json"
    command = [sys.executable, str(SCRIPTS / "epub_structure_tool.py"), "normalize", str(base), "--output", str(normalized), "--report-format", "json"]
    if normalize == "dry-run": command.append("--dry-run")
    result = _run(command); normalize_report.parent.mkdir(parents=True, exist_ok=True); normalize_report.write_text(result.stdout, encoding="utf-8")
    if result.returncode: raise CleanupError(f"normalize {normalize} failed: {(result.stdout + result.stderr).strip()}")
    if normalize == "dry-run": raise CleanupError(f"normalize dry-run complete; review {normalize_report} and rerun with --normalize apply --approve-normalize")
    ok, output = _text_gate(base, normalized, normalize_report)
    stages.append({"name": "normalize-apply", "status": "pass" if ok else "fail", "text_gate": output, "report": str(normalize_report)})
    if not ok: raise CleanupError("normalize apply changed text")
    base = normalized
    normalized_preflight = _audit(base)
    if _blocking_preflight(normalized_preflight): raise CleanupError("preflight-normalized: blocking findings remain after normalization")
  if _needs_migration(_audit(base)):

    migrated = work / "after" / "step-0-epub3.epub"
    migrated.parent.mkdir(parents=True, exist_ok=True)
    result = _run([sys.executable, str(SCRIPTS / "epub3_migration_harness.py"), str(base), "--write-output", str(migrated), "--format", "json"])
    if result.returncode:
      raise CleanupError(f"epub3 migration failed: {(result.stdout + result.stderr).strip()}")
    ok, output = _text_gate(base, migrated)
    stages.append({"name": "epub3-migration", "status": "pass" if ok else "fail", "text_gate": output})
    if not ok:
      raise CleanupError("epub3 migration changed text")
    base = migrated
  else:
    stages.append({"name": "epub3-migration", "status": "skipped", "reason": "already-epub3-with-nav"})
  return base, stages

ALLOWED_OPS = {"add-manifest-properties"}

class RulesPlanner:
  """Turn detector findings into allow-listed deterministic package actions."""
  def plan(self, round_number: int, audit: dict[str, Any]) -> dict[str, Any]:
    actions, suggestions = [], []
    for item in audit.get("findings", []):
      if item.get("kind") == "missing-manifest-properties" and item.get("auto_fixable"):
        action = {"op": "add-manifest-properties", "path": item.get("path")}
        if action not in actions:
          actions.append(action)
      elif item.get("level") in {"error", "warn"}:
        suggestions.append(item)
    return {"round": round_number, "actions": actions, "suggestions": suggestions}

class HandshakePlanner:
  """Read an external host plan from disk while enforcing the same op whitelist."""
  def __init__(self, plan_dir: Path) -> None:
    self.plan_dir = plan_dir

  def plan(self, round_number: int, audit: dict[str, Any]) -> dict[str, Any]:
    self.plan_dir.mkdir(parents=True, exist_ok=True)
    request = self.plan_dir / f"round-{round_number}.plan-request.json"
    response = self.plan_dir / f"round-{round_number}.plan.json"
    _write_json(request, {"round": round_number, "audit": audit, "allowed_ops": sorted(ALLOWED_OPS)})
    if not response.is_file():
      raise CleanupError(f"handshake plan missing: write {response} from {request} and rerun in a fresh work directory")
    try:
      plan = json.loads(response.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
      raise CleanupError(f"handshake plan is not valid JSON: {response}") from exc
    if not isinstance(plan, dict) or not isinstance(plan.get("actions", []), list):
      raise CleanupError(f"handshake plan must be an object with an actions list: {response}")
    for action in plan.get("actions", []):
      if not isinstance(action, dict) or action.get("op") not in ALLOWED_OPS:
        raise CleanupError(f"handshake plan contains unsupported op: {action!r}")
    plan.setdefault("suggestions", [])
    plan["round"] = round_number
    return plan

def apply_action(epub: Path, action: dict[str, Any], output: Path) -> dict[str, Any]:
  if action.get("op") != "add-manifest-properties":
    return {"status": "skipped", "reason": "unsupported-op", "action": action}
  href = str(action.get("path") or "")
  files, order = _read_epub(epub)
  opf_path, root = _package(files)
  opf_dir = posixpath.dirname(opf_path)
  changed: list[str] = []
  for item in root.findall("opf:manifest/opf:item", NS):
    if item.attrib.get("href") != href:
      continue
    target = posixpath.normpath(posixpath.join(opf_dir, href))
    text = files.get(target, b"").decode("utf-8", errors="ignore").lower()
    props = item.attrib.get("properties", "").split()
    for prop, marker in (("svg", "<svg"), ("mathml", "<math")):
      if marker in text and prop not in props:
        props.append(prop); changed.append(prop)
    if changed:
      item.set("properties", " ".join(props))
      files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
      _write_epub(output, files, order)
      return {"status": "applied", "action": action, "added": changed}
  return {"status": "skipped", "reason": "already-clean-or-target-missing", "action": action}

def _finalize(base: Path, output: Path, rounds_run: int) -> Path:
  files, order = _read_epub(base)
  opf_path, root = _package(files)
  metadata = root.find("opf:metadata", NS)
  if metadata is None:
    raise CleanupError("OPF missing metadata")
  marker = next((meta for meta in metadata.findall("opf:meta", NS) if meta.attrib.get("property") == META_PROPERTY), None)
  if marker is None:
    marker = ET.SubElement(metadata, f"{{{OPF_URI}}}meta", {"property": META_PROPERTY})
  marker.text = str(rounds_run)
  prefixes = root.attrib.get("prefix", "").split()
  if "epub-handbook:" not in prefixes:
    root.set("prefix", " ".join(filter(None, [root.attrib.get("prefix", ""), META_PREFIX])))
  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  return _write_epub(output, files, order)

def run_loop(input_epub: Path, work: Path, planner: RulesPlanner | HandshakePlanner | None = None, max_rounds: int = 6, dry_limit: int = 2, normalize: str = "skip", approve_normalize: bool = False) -> dict[str, Any]:
  planner = planner or RulesPlanner(); work = work.resolve(); reports = work / "reports"
  base, stages = _stage_input(input_epub.resolve(), work, normalize=normalize, approve_normalize=approve_normalize)
  reports.mkdir(parents=True, exist_ok=True); _write_json(reports / "staging.json", {"stages": stages})
  seen = {_fingerprint(base)}; dry = 0; round_log: list[dict[str, Any]] = []; stopped_by = "max-rounds"
  for rnd in range(1, max_rounds + 1):
    audit = _audit(base); plan = planner.plan(rnd, audit); _write_json(reports / f"round-{rnd}.plan.json", plan)
    applied, needs_human = [], list(plan.get("suggestions", [])); candidate = base
    for index, action in enumerate(plan.get("actions", []), 1):
      step = work / "after" / f"round-{rnd}-action-{index}.epub"
      result = apply_action(candidate, action, step)
      if result["status"] == "applied": candidate = step; applied.append(result)
      else: needs_human.append(result)
    gate_ok, gate_output = _text_gate(base, candidate)
    check = _epubcheck(candidate)
    if not gate_ok or check["status"] == "fail":
      needs_human.extend(applied); needs_human.append({"reason": "round-gate-failed", "text_gate": gate_output, "epubcheck": check}); applied = []; candidate = base
    base = candidate; fp = _fingerprint(base)
    entry = {"round": rnd, "applied": applied, "needs_human": needs_human, "text_gate": "pass" if gate_ok else "fail", "epubcheck": check}
    round_log.append(entry)
    if fp in seen and applied:
      stopped_by = "fingerprint"; break
    seen.add(fp)
    if applied: dry = 0
    else:
      dry += 1
      if dry >= dry_limit: stopped_by = "dry"; break
  cleaned = _finalize(base, work / "after" / "cleaned.epub", len(round_log))
  report = {"status": "complete", "stopped_by": stopped_by, "rounds_run": len(round_log), "output": str(cleaned), "staging": stages, "round_log": round_log}
  _write_json(reports / "cleanup-loop.json", report); return report

def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Run deterministic package cleanup rounds")
  parser.add_argument("input", type=Path); parser.add_argument("--work-dir", type=Path, default=Path("work/cleanup-loop")); parser.add_argument("--max-rounds", type=int, default=6); parser.add_argument("--dry-limit", type=int, default=2); parser.add_argument("--normalize", choices=("skip", "dry-run", "apply"), default="skip"); parser.add_argument("--approve-normalize", action="store_true"); parser.add_argument("--planner", choices=("rules", "handshake"), default="rules"); parser.add_argument("--plan-dir", type=Path, default=Path("work/cleanup-loop-handshake")); parser.add_argument("--format", choices=("json", "text"), default="text")
  args = parser.parse_args(argv)
  try:
    planner = RulesPlanner() if args.planner == "rules" else HandshakePlanner(args.plan_dir)
    report = run_loop(args.input, args.work_dir, planner=planner, max_rounds=args.max_rounds, dry_limit=args.dry_limit, normalize=args.normalize, approve_normalize=args.approve_normalize)
  except CleanupError as exc:
    print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2) if args.format == "json" else f"ERROR: {exc}", file=sys.stderr); return 1
  print(json.dumps(report, ensure_ascii=False, indent=2) if args.format == "json" else f"Cleanup complete: {report['output']} ({report['rounds_run']} rounds, stopped by {report['stopped_by']})"); return 0
if __name__ == "__main__": raise SystemExit(main(sys.argv[1:]))
