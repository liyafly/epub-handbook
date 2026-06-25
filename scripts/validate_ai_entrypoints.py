#!/usr/bin/env python3
"""Validate that model-facing repository instructions have one canonical source."""

from __future__ import annotations

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def read_required(path: Path, errors: list[str]) -> str:
  if not path.exists():
    errors.append(f"missing required file: {path.relative_to(ROOT)}")
    return ""
  return path.read_text(encoding="utf-8")


def require_tokens(label: str, text: str, tokens: tuple[str, ...], errors: list[str]) -> None:
  for token in tokens:
    if token not in text:
      errors.append(f"{label}: missing required reference: {token}")


def main() -> int:
  errors: list[str] = []
  agents = read_required(ROOT / "AGENTS.md", errors)
  claude = read_required(ROOT / "CLAUDE.md", errors)
  readme = read_required(ROOT / "README.md", errors)
  contributing = read_required(ROOT / "CONTRIBUTING.md", errors)
  docs_readme = read_required(ROOT / "docs" / "README.md", errors)
  skills_readme = read_required(ROOT / "skills" / "README.md", errors)
  getting_started_skills = read_required(ROOT / "docs" / "learn" / "04-skills.md", errors)

  require_tokens(
    "AGENTS.md",
    agents,
    (
      "唯一维护源",
      "docs/final/SPEC-实现约束.md",
      "docs/pipeline/cleanup-flow.md",
      "scripts/epub_structure_tool.py",
      "scripts/validate_text_invariance.py",
      "scripts/validate_ai_entrypoints.py",
      "scripts/validate_skills_basic.py",
      "THIRD_PARTY.md",
    ),
    errors,
  )
  require_tokens("CLAUDE.md", claude, ("兼容入口", "[AGENTS.md](AGENTS.md)"), errors)
  require_tokens("README.md", readme, ("[AGENTS.md](AGENTS.md)",), errors)
  require_tokens("CONTRIBUTING.md", contributing, ("[AGENTS.md](AGENTS.md)",), errors)
  require_tokens("docs/README.md", docs_readme, ("`AGENTS.md`",), errors)
  require_tokens("skills/README.md", skills_readme, ("`AGENTS.md`",), errors)
  require_tokens("docs/learn/04-skills.md", getting_started_skills, ("[AGENTS.md](../../AGENTS.md)",), errors)

  if errors:
    for error in errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("AI entrypoint validation ok (AGENTS.md is canonical)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
