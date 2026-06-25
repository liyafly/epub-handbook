"""Report generator: aggregate classification results into JSON + human-readable summary."""

import json
from dataclasses import is_dataclass, fields


def _get(obj, attr, default=None):
    """Get attribute from dataclass or dict, with fallback."""
    if is_dataclass(obj) and not isinstance(obj, type):
        return getattr(obj, attr, default)
    if isinstance(obj, dict):
        return obj.get(attr, default)
    return default


def _to_dict(obj):
    """Recursively convert dataclass objects to plain dicts for JSON serialization."""
    if is_dataclass(obj) and not isinstance(obj, type):
        result = {}
        for f in fields(obj):
            result[f.name] = _to_dict(getattr(obj, f.name))
        return result
    if isinstance(obj, dict):
        return {k: _to_dict(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [_to_dict(v) for v in obj]
    return obj


def generate_report(
    book: dict,
    fonts: dict,
    chains: dict,
    char_inventory: list,
    unresolved: list,
    candidate_validation: dict | None = None,
) -> dict:
    """Generate the full JSON report.

    Args:
        book: {path, title, opf} from reader
        fonts: {embedded: [FontInfo, ...], referenced_non_embedded: [str, ...]}
        chains: {chain_id: [ChainSegment, ...]}
        char_inventory: list of CoverageResult dicts
        unresolved: list of unresolved CSS items
        candidate_validation: optional candidate font validation result

    Returns:
        Full report dict matching schema v1.0
    """
    # Build summary
    unique_chars = len(char_inventory)
    by_cause: dict[str, int] = {}
    by_profile_risk: dict[str, dict[str, int]] = {}
    pua_flagged = 0
    ivs_flagged = 0

    for ci in char_inventory:
        cause = _get(ci, "cause", "unknown")
        by_cause[cause] = by_cause.get(cause, 0) + 1

        flags = _get(ci, "flags", [])
        if "pua" in flags:
            pua_flagged += 1
        if "ivs" in flags:
            ivs_flagged += 1

        profiles = _get(ci, "profiles", {})
        for profile_name, verdict in profiles.items():
            if profile_name not in by_profile_risk:
                by_profile_risk[profile_name] = {"ok": 0, "risk": 0, "fail": 0}
            by_profile_risk[profile_name][verdict] = (
                by_profile_risk[profile_name].get(verdict, 0) + 1
            )

    report = {
        "schema_version": "1.0",
        "book": book,
        "fonts": fonts,
        "chains": _serialize_chains(chains),
        "char_inventory": _to_dict(char_inventory),
        "summary": {
            "unique_chars": unique_chars,
            "by_cause": by_cause,
            "by_profile_risk": by_profile_risk,
            "unresolved_runs": len(unresolved),
            "pua_flagged": pua_flagged,
            "ivs_flagged": ivs_flagged,
        },
        "unresolved": unresolved,
        "candidate_validation": candidate_validation,
    }

    return report


def _serialize_chains(chains: dict) -> list:
    """Convert chains dict to list format for JSON output."""
    result = []
    for chain_id, segments in chains.items():
        seg_list = []
        for seg in segments:
            d = {"family": seg.family}
            if hasattr(seg, "embedded"):
                d["embedded"] = seg.embedded
            if hasattr(seg, "file") and seg.file:
                d["file"] = seg.file
            if hasattr(seg, "generic") and seg.generic:
                d["generic"] = True
            seg_list.append(d)
        result.append({"id": chain_id, "resolved": seg_list})
    return result


def format_summary(report: dict) -> str:
    """Produce a human-readable summary string from the report."""
    s = report["summary"]
    lines = [
        "=" * 60,
        f"  EPUB Font Coverage Report",
        f"  Book: {report['book'].get('title', 'unknown')}",
        f"  Path: {report['book'].get('path', 'unknown')}",
        "=" * 60,
        "",
        f"  Unique characters rendered: {s['unique_chars']}",
        f"  Embedded fonts: {len(report['fonts'].get('embedded', []))}",
        "",
        "  By cause:",
    ]

    cause_labels = {
        "true-missing": "  True missing (no font has glyph)",
        "fallback-not-reached": "  Fallback not reached (Kindle risk)",
        "subset-cut": "  Subset cut (font subset removed glyph)",
        "unknown": "  Unknown / unresolved",
    }
    for cause, count in sorted(s.get("by_cause", {}).items()):
        label = cause_labels.get(cause, f"  {cause}")
        lines.append(f"{label}: {count}")

    lines.append("")
    lines.append("  By reader profile:")

    for profile, counts in s.get("by_profile_risk", {}).items():
        lines.append(
            f"    {profile}: ok={counts.get('ok', 0)} "
            f"risk={counts.get('risk', 0)} fail={counts.get('fail', 0)}"
        )

    lines.append("")
    lines.append(f"  PUA-flagged characters: {s.get('pua_flagged', 0)}")
    lines.append(f"  IVS-flagged characters: {s.get('ivs_flagged', 0)}")
    lines.append(f"  Unresolved CSS items: {s.get('unresolved_runs', 0)}")

    if s.get("unresolved_runs", 0) > 0:
        lines.append("")
        lines.append(
            "  ⚠ WARNING: Some CSS could not be resolved. "
            "Results may be incomplete. See 'unresolved' in JSON report."
        )

    # Show top failures
    failures = [
        ci
        for ci in report.get("char_inventory", [])
        if any(v == "fail" for v in _get(ci, "profiles", {}).values())
    ]
    if failures:
        lines.append("")
        lines.append(f"  Top failing characters ({min(len(failures), 20)} of {len(failures)}):")
        for ci in failures[:20]:
            char_display = _get(ci, "char", "?")
            cp = _get(ci, "cp", "U+????")
            causes = []
            for chain_id, cov in _get(ci, "coverage", {}).items():
                causes.append(f"{_get(cov, 'cause', '?')}")
            lines.append(f"    {char_display}  {cp}  count={_get(ci, 'count', 0)}  {', '.join(causes)}")

    lines.append("")
    lines.append("=" * 60)

    return "\n".join(lines)


def save_report(report: dict, output_path: str) -> None:
    """Write report JSON to file."""
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)


def print_summary(report: dict) -> None:
    """Print human-readable summary to stdout."""
    print(format_summary(report))
