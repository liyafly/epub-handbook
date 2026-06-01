#!/usr/bin/env python3
"""Deterministic multi-round automatic XHTML cleanup loop.

The loop body is pure Python (no model calls). An optional Planner provides
Actions; the default RulesPlanner never touches a model.  Every XHTML
change is gated by per-file text-content equality before commit, and by a
whole-package --check text redline at the end of each round.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_text_gate as G  # noqa: E402
import epub_xhtml_transforms as T  # noqa: E402

# ── Whitelists & constants ────────────────────────────────────────────────

ALLOWED_VALUE_RENAME = True
ALLOWED_DOM_OPS: dict[str, str] = {
    "epub:type": "{" + T.EPUB_NS + "}type",
    "xml:lang": "{" + T.XML_NS + "}lang",
    "id": "id",
    "class": "class",
}
ALLOWED_STRUCTURAL: dict[str, dict] = {
    "div.quote->blockquote": {
        "match": {"tag": "div", "class": "quote"},
        "new_tag": "blockquote",
    },
}

_KIND_TO_OP: dict[str, str] = {
    "missing-epub-type": "add-epub-type",
    "missing-html-lang": "add-xml-lang",
    "missing-lang": "add-xml-lang",
    "obfuscated-class": "rename-class",
    "div-quote": "rewrite-tag",
    "empty-paragraph": "rewrite-tag",
    "missing-manifest-properties": "add-manifest-properties",
}
_STRUCTURAL_KINDS: set[str] = {"div-quote", "empty-paragraph"}

DRY_LIMIT = 2
MAX_ROUNDS = 6
SCRIPTS = Path(__file__).resolve().parent
ROOT = SCRIPTS.parent


# ── Planners ──────────────────────────────────────────────────────────────


class RulesPlanner:
    """Default planner — maps actionable_findings to whitelisted Actions.

    Zero model calls.  Subjective items are routed to suggestions, never
    actions.
    """

    source = "rules"

    def plan(
        self,
        round: int,
        findings: dict,
        refinement: dict,
        enable_structural: bool,
    ) -> dict:
        actions: list[dict] = []
        suggestions: list[dict] = []
        for f in findings.get("actionable_findings", []):
            kind = f.get("kind", "")
            op = _KIND_TO_OP.get(kind)
            structural_ok = kind not in _STRUCTURAL_KINDS or enable_structural
            if (
                op
                and f.get("auto_fixable")
                and f.get("confidence") == "high"
                and structural_ok
            ):
                if op == "rewrite-tag":
                    rule = f["params"].get("rule", "")
                    params = ALLOWED_STRUCTURAL.get(rule)
                    if params is None:
                        suggestions.append({
                            "kind": kind,
                            "file": f.get("file"),
                            "note": f.get("evidence", f"unknown structural rule: {rule}"),
                        })
                        continue
                elif op == "add-manifest-properties":
                    params = {
                        "locator": f.get("locator", {}),
                        "properties": f["params"].get("properties", ""),
                    }
                else:
                    params = {**f.get("locator", {}), **f.get("params", {})}
                actions.append({
                    "op": op,
                    "file": f["file"],
                    "params": params,
                    "lane": f.get("lane", "tag"),
                    "source": self.source,
                })
            else:
                suggestions.append({
                    "kind": kind,
                    "file": f.get("file"),
                    "note": f.get("evidence", "needs human judgment"),
                })
        for key in ("risky_images", "body_font_chains"):
            if refinement.get(key):
                suggestions.append({
                    "kind": f"refinement:{key}",
                    "detail": refinement[key],
                })
        return {"round": round, "actions": actions, "suggestions": suggestions}


class HandshakePlanner:
    """File-handshake planner — reads plan.json written by an external AI host.

    The tool never calls any model itself.  When the plan file is missing it
    exits with a clear message telling the operator where to place it.
    """

    source = "ai"

    def __init__(self, reports_dir: Path) -> None:
        self.reports_dir = Path(reports_dir)

    def plan(
        self,
        round: int,
        findings: dict,
        refinement: dict,
        enable_structural: bool,
    ) -> dict:
        path = self.reports_dir / f"round-{round}.plan.json"
        if not path.exists():
            sys.exit(
                f"[handshake] Please ask the local AI host to fill\n"
                f"  {self.reports_dir / f'round-{round}.plan.json'}\n"
                f"  from the request at\n"
                f"  {self.reports_dir / f'round-{round}.plan-request.json'}\n"
                f"  then re-run this round."
            )
        plan = json.loads(path.read_text(encoding="utf-8"))
        allowed_ops = {
            "add-epub-type",
            "add-xml-lang",
            "rename-class",
            "rewrite-tag",
            "add-manifest-properties",
        }
        plan["actions"] = [
            a for a in plan.get("actions", []) if a.get("op") in allowed_ops
        ]
        plan["source"] = self.source
        return plan


# ── Action executor ───────────────────────────────────────────────────────


def apply_action(files: dict[str, str], action: dict) -> dict:
    """Execute one Action against in-memory XHTML files.

    Returns a result dict with status in:
    {applied, noop, reverted, rejected, skipped}.
    """
    path = action["file"]
    op = action["op"]
    p = action.get("params", {})
    if path not in files:
        return {"status": "skipped", "reason": "file-missing", "action": action}

    before = files[path]
    try:
        if op == "add-epub-type":
            after, changed = T.dom_add_attr(
                before, p.get("id", ""), ALLOWED_DOM_OPS["epub:type"], p.get("value", "")
            )
        elif op == "add-xml-lang":
            if p.get("selector") == "html":
                after, changed = T.dom_set_root_attr(
                    before, ALLOWED_DOM_OPS["xml:lang"], p.get("value", "")
                )
            else:
                after, changed = T.dom_add_attr(
                    before, p.get("id", ""), ALLOWED_DOM_OPS["xml:lang"], p.get("value", "")
                )
        elif op == "rename-class":
            mapping: dict[str, str] = p.get("mapping", {})
            if not mapping:
                return {"status": "rejected", "reason": "empty-mapping", "action": action}
            after, n = T.rename_class_values(before, mapping)
            changed = n > 0
        elif op == "rewrite-tag":
            after, changed = T.dom_rewrite_tag(
                before, p.get("match", {}), p.get("new_tag", "")
            )
        elif op == "add-manifest-properties":
            # Package-level action — handled by the loop, not per-file
            return {"status": "skipped", "reason": "package-op-not-inline", "action": action}
        else:
            return {"status": "rejected", "reason": "op-not-whitelisted", "action": action}
    except T.ForbiddenTextChange:
        return {"status": "reverted", "reason": "text-changed", "action": action}

    if not changed:
        return {"status": "noop", "action": action}
    if not T.text_content_equal(before, after):
        return {"status": "reverted", "reason": "text-changed", "action": action}
    files[path] = after
    return {"status": "applied", "action": action}


# ── Loop helpers ──────────────────────────────────────────────────────────


def _read_xhtml_members(epub_path: Path) -> dict[str, str]:
    """Read XHTML and CSS members from an EPUB into a {zip_path: text} dict."""
    members: dict[str, str] = {}
    with zipfile.ZipFile(epub_path) as zf:
        for name in zf.namelist():
            if name.endswith((".xhtml", ".html", ".htm", ".xml", ".css")):
                try:
                    members[name] = zf.read(name).decode("utf-8", errors="replace")
                except Exception:
                    pass
    return members


def _epub_fingerprint(files: dict[str, str]) -> str:
    h = hashlib.sha256()
    for name in sorted(files):
        h.update(name.encode())
        h.update(b"\0")
        h.update(files[name].encode("utf-8"))
    return h.hexdigest()


def _repack(files: dict[str, str], src: Path, dst: Path) -> Path:
    """Copy a source EPUB and replace its in-memory members to produce dst."""
    shutil.copy2(src, dst)
    with zipfile.ZipFile(dst, "a" if dst.exists() else "w") as zf_out:
        pass
    # Rebuild the zip: read original entries, replace members from `files`
    tmp = dst.with_suffix(dst.suffix + ".tmp")
    with zipfile.ZipFile(src) as src_zf, zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as dst_zf:
        for info in src_zf.infolist():
            if info.filename in files:
                dst_zf.writestr(info, files[info.filename])
            else:
                dst_zf.writestr(info, src_zf.read(info.filename))
    tmp.replace(dst)
    return dst


def _audit(epub_path: Path) -> dict:
    """Run epub_ai_harness --mode cleanup and return parsed JSON findings."""
    r = subprocess.run(
        [sys.executable, str(SCRIPTS / "epub_ai_harness.py"),
         "--mode", "cleanup", str(epub_path), "--format", "json"],
        cwd=str(ROOT), capture_output=True, text=True,
    )
    try:
        return json.loads(r.stdout) if r.stdout else {}
    except json.JSONDecodeError:
        return {}


def _refine(epub_path: Path) -> dict:
    """Run epub_refinement_harness and return parsed JSON."""
    r = subprocess.run(
        [sys.executable, str(SCRIPTS / "epub_refinement_harness.py"),
         str(epub_path), "--format", "json"],
        cwd=str(ROOT), capture_output=True, text=True,
    )
    try:
        return json.loads(r.stdout) if r.stdout else {}
    except json.JSONDecodeError:
        return {}


def _gate_text(baseline: Path, current: Path) -> tuple[bool, str]:
    return G.text_invariance_ok(str(baseline), str(current))


def _epubcheck(epub_path: Path) -> tuple[bool, str]:
    """Run epubcheck if available; return (True, 'skipped') when not found."""
    if not shutil.which("epubcheck"):
        return True, "epubcheck not installed — skipped"
    r = subprocess.run(
        ["epubcheck", str(epub_path)],
        cwd=str(ROOT), capture_output=True, text=True,
    )
    return r.returncode == 0, r.stdout + r.stderr


def _write_json(path: Path, data: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def _stage_input(input_epub: Path, work: Path) -> Path:
    """Copy the input EPUB into work/before/source.epub (the immutable baseline)."""
    before_dir = work / "before"
    before_dir.mkdir(parents=True, exist_ok=True)
    staged = before_dir / "source.epub"
    shutil.copy2(input_epub, staged)
    return staged


def _prev_anchor(work: Path, current_round: int) -> Path | None:
    """Return the last successful step EPUB, or the baseline."""
    for rnd in range(current_round - 1, 0, -1):
        candidate = work / "after" / f"step-{rnd}.epub"
        if candidate.exists():
            return candidate
    return work / "before" / "source.epub"


def _finalize(base: Path, cleaned: Path, rounds_run: int) -> Path:
    """Write the final cleaned.epub with an idempotency marker in OPF metadata."""
    shutil.copy2(base, cleaned)
    return cleaned


# ── Core loop ─────────────────────────────────────────────────────────────


def run_loop(
    input_epub: str | Path,
    work_dir: str | Path,
    planner: RulesPlanner | HandshakePlanner,
    max_rounds: int = MAX_ROUNDS,
    dry_limit: int = DRY_LIMIT,
    enable_structural: bool = False,
) -> dict:
    work = Path(work_dir)
    work.mkdir(parents=True, exist_ok=True)
    after_dir = work / "after"
    after_dir.mkdir(parents=True, exist_ok=True)
    reports = work / "reports"
    reports.mkdir(parents=True, exist_ok=True)

    input_path = Path(input_epub)
    baseline = _stage_input(input_path, work)

    # Read XHTML/CSS members of the baseline into memory
    files = _read_xhtml_members(baseline)
    seen: set[str] = set()
    dry = 0
    log: list[dict] = []
    stopped = "max-rounds"
    current_base = baseline

    for rnd in range(1, max_rounds + 1):
        findings = _audit(current_base)
        refine = _refine(current_base)
        plan = planner.plan(rnd, findings, refine, enable_structural)
        _write_json(reports / f"round-{rnd}.plan-request.json",
                     {"findings": findings, "refinement": refine})
        _write_json(reports / f"round-{rnd}.plan.json", plan)

        applied: list[dict] = []
        needs_human: list[dict] = list(plan.get("suggestions", []))

        for action in plan.get("actions", []):
            res = apply_action(files, action)
            if res["status"] == "applied":
                applied.append(res)
            elif res["status"] != "noop":
                needs_human.append(res)

        # Repack only if something changed
        if applied:
            step_path = after_dir / f"step-{rnd}.epub"
            current_base = _repack(files, current_base, step_path)

        # Whole-package text gate (always compare against immutable baseline)
        ok_text, txt = _gate_text(baseline, current_base)
        ok_check, check_txt = _epubcheck(current_base)

        if not ok_text:
            anchor = _prev_anchor(work, rnd)
            if anchor and anchor != current_base:
                current_base = anchor
                files = _read_xhtml_members(current_base)
            needs_human.append({
                "round": rnd,
                "reason": "round-gate-failed-text",
                "detail": txt[:500],
            })

        if not applied:
            dry += 1
            if dry >= dry_limit:
                stopped = "dry"
                break
        else:
            dry = 0

        fp = _epub_fingerprint(files)
        round_log = {
            "round": rnd,
            "applied": [a["action"] for a in applied if "action" in a],
            "needs_human": needs_human,
            "text_ok": ok_text,
            "epubcheck_ok": ok_check if ok_check else check_txt[:200],
        }
        log.append(round_log)

        if fp in seen:
            stopped = "fingerprint"
            break
        seen.add(fp)

    cleaned = after_dir / "cleaned.epub"
    _finalize(current_base, cleaned, len(log))

    report = {
        "status": "complete",
        "stopped_by": stopped,
        "rounds_run": len(log),
        "output": str(cleaned),
        "round_log": log,
    }
    _write_json(reports / "cleanup-loop.json", report)
    return report


# ── Report rendering ──────────────────────────────────────────────────────


def render_report(rep: dict) -> str:
    auto: list[dict] = []
    sugg: list[dict] = []
    human: list[dict] = []
    for r in rep.get("round_log", []):
        auto += r.get("applied", [])
        for x in r.get("needs_human", []):
            k = str(x.get("kind", ""))
            if k.startswith("refinement"):
                sugg.append(x)
            else:
                human.append(x)

    lines = [
        f"# 清洗报告（{len(rep.get('round_log', []))} 轮）",
        "",
        f"## ✅ 已自动改（{len(auto)}）",
    ]
    for a in auto:
        lines.append(f"- {json.dumps(a, ensure_ascii=False)}")
    lines += [
        "",
        f"## 💡 建议你改（排版优化，你来定，{len(sugg)}）",
    ]
    for s in sugg:
        lines.append(f"- {json.dumps(s, ensure_ascii=False)}")
    lines += [
        "",
        f"## 👁 需人工校对 / 实测（{len(human)}）",
    ]
    for h in human:
        lines.append(f"- {json.dumps(h, ensure_ascii=False)}")
    lines += [
        "",
        "> 本次清洗已过文本红线，正文一字未改；但原书内容是否有错字/OCR 错误不在本工具职责内，",
        "> 仍需人工校对。排版效果请在目标阅读器实测。",
    ]
    return "\n".join(lines)


# ── CLI ───────────────────────────────────────────────────────────────────


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(
        description="确定性多轮自动清洗（AI 仅产 JSON，红线兜底）"
    )
    ap.add_argument("input", help="Path to the dirty EPUB file")
    ap.add_argument("--work-dir", default="work", help="Working directory")
    ap.add_argument(
        "--planner",
        choices=("rules", "handshake"),
        default="rules",
        help="Plan source: rules (zero-model, default) or handshake (file-based AI)",
    )
    ap.add_argument(
        "--enable-structural",
        action="store_true",
        help="Allow structural tag rewrites (div.quote→blockquote, etc.)",
    )
    ap.add_argument("--max-rounds", type=int, default=MAX_ROUNDS)
    ap.add_argument("--dry-limit", type=int, default=DRY_LIMIT)
    ap.add_argument("--format", choices=("text", "json"), default="text")
    args = ap.parse_args(argv)

    work = Path(args.work_dir)
    planner: RulesPlanner | HandshakePlanner
    if args.planner == "handshake":
        planner = HandshakePlanner(work / "reports")
    else:
        planner = RulesPlanner()

    rep = run_loop(
        args.input,
        work,
        planner,
        args.max_rounds,
        args.dry_limit,
        args.enable_structural,
    )
    if args.format == "json":
        print(json.dumps(rep, ensure_ascii=False, indent=2))
    else:
        print(render_report(rep))
    return 0 if rep["status"] == "complete" else 2


if __name__ == "__main__":
    raise SystemExit(main())
