"""Font-family chain health assessment (字体链体检).

Pure analysis over already-resolved chains + the font cmap index. Flags
chain-*writing* problems independent of any single character.
"""

from __future__ import annotations

_FAIL_ISSUES = frozenset({
    "broken-font-face", "full-font-not-at-head",
})


def _is_full_font(font_data: dict) -> bool:
    return bool(font_data) and not font_data.get("subset", False)


def _assess_one(segments: list, font_index: dict) -> dict:
    issues: list[str] = []

    embedded = [s for s in segments if getattr(s, "embedded", False)]
    non_generic = [s for s in segments if not getattr(s, "generic", False)]

    if any(getattr(s, "defaulted", False) for s in segments):
        issues.append("no-font-family-declared")

    # A family with a package-path hint that never resolved to an embedded
    # file = genuinely broken @font-face. System-font references (res://,
    # file://, local()) are legitimate "borrow the reader's font" writing and
    # carry no file hint (system_ref=True), so they must NOT be flagged.
    for s in segments:
        if (getattr(s, "file", None)
                and not getattr(s, "embedded", False)
                and not getattr(s, "system_ref", False)):
            issues.append("broken-font-face")
            break

    head = segments[0] if segments else None
    head_embedded = bool(head) and getattr(head, "embedded", False)
    head_is_full_font = (
        head_embedded and _is_full_font(font_index.get(getattr(head, "file", None), {}))
    )

    any_full_embedded = any(
        _is_full_font(font_index.get(getattr(s, "file", None), {})) for s in embedded
    )
    if any_full_embedded and not head_is_full_font:
        issues.append("full-font-not-at-head")

    has_generic = any(getattr(s, "generic", False) for s in segments)

    # subset-only is only a problem when there's NO generic safety net — a
    # subset + generic tail (e.g. a rare-char subset → serif) is legitimate.
    subset_only = embedded and all(
        font_index.get(getattr(s, "file", None), {}).get("subset", False) for s in embedded
    )
    if subset_only and not has_generic:
        issues.append("subset-only")

    if not non_generic:
        issues.append("generic-only")
    elif not has_generic:
        # has named families but no generic safety net — the strongest
        # Kindle-failure discriminator observed on real books.
        issues.append("no-generic-fallback")

    issues = sorted(set(issues))
    if any(i in _FAIL_ISSUES for i in issues):
        grade = "fail"
    elif issues:
        grade = "warn"
    else:
        grade = "ok"

    return {
        "issues": issues,
        "head_embedded": head_embedded,
        "head_is_full_font": head_is_full_font,
        "grade": grade,
    }


def assess_chains(chains: dict, font_index: dict, run_chains: dict) -> dict:
    """Assess every resolved chain. Returns {chain_id: health_dict}."""
    run_counts: dict[str, int] = {}
    for cid in (run_chains or {}).values():
        run_counts[cid] = run_counts.get(cid, 0) + 1

    out: dict = {}
    for cid, segments in chains.items():
        health = _assess_one(segments, font_index)
        health["run_count"] = run_counts.get(cid, 0)
        out[cid] = health
    return out
