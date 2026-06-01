#!/usr/bin/env python3
"""Thin wrapper around scripts/validate_text_invariance.py for the loop.

Provides a programmatic Python interface to the body-text red-line gate so
that the cleanup loop can call it per-step without shelling out to the
script directly.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
NAV_ALLOW = ["*/nav.xhtml", "*/toc.ncx"]


def text_invariance_ok(
    before: str | Path,
    after: str | Path,
    allow: list[str] | None = None,
    path_map: str | Path | None = None,
) -> tuple[bool, str]:
    """Run validate_text_invariance.py --check text and return (passed, output).

    Args:
        before: Path to the baseline EPUB.
        after: Path to the transformed EPUB.
        allow: Additional fnmatch globs to exclude from text check.
               Defaults to nav.xhtml and toc.ncx.
        path_map: Optional JSON mapping report for path normalization.

    Returns:
        (True, output) when text invariance holds; (False, output) otherwise.
    """
    cmd = [
        sys.executable,
        str(SCRIPTS / "validate_text_invariance.py"),
        str(before),
        str(after),
        "--check",
        "text",
    ]
    for g in allow if allow is not None else NAV_ALLOW:
        cmd += ["--allow-list", g]
    if path_map:
        cmd += ["--path-map", str(path_map)]
    r = subprocess.run(cmd, cwd=str(ROOT), capture_output=True, text=True)
    return r.returncode == 0, (r.stdout + r.stderr)
