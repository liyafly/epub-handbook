#!/usr/bin/env python3
"""Regression tests for validate_skills_basic.py."""

from __future__ import annotations

import sys
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import validate_skills_basic as V  # noqa: E402


def write_skill(root: Path, name: str, *, extra_meta: bool = False) -> Path:
  folder = root / "skills" / name
  (folder / "agents").mkdir(parents=True)
  meta_extra = "owner: nobody\n" if extra_meta else ""
  (folder / "SKILL.md").write_text(
    f"""---
name: {name}
description: Test skill description long enough for validation
{meta_extra}---

Use this skill in tests.
""",
    encoding="utf-8",
  )
  (folder / "agents" / "openai.yaml").write_text(
    f"""display_name: Test Skill
short_description: Test skill
default_prompt: Use ${name}
""",
    encoding="utf-8",
  )
  return folder


def test_validate_skill_accepts_minimal_valid_skill() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    folder = write_skill(root, "epub-test-skill")
    errors = V.validate_skill(folder)
    if errors:
      raise AssertionError(f"valid skill should pass: {errors}")


def test_validate_skill_rejects_extra_frontmatter() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    folder = write_skill(root, "epub-test-skill", extra_meta=True)
    errors = V.validate_skill(folder)
    if not any("unsupported frontmatter keys" in error for error in errors):
      raise AssertionError(f"extra frontmatter key was not reported: {errors}")


def test_validate_skill_tables_detects_missing_entries() -> None:
  original_root = V.ROOT
  try:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      skill = write_skill(root, "epub-test-skill")
      (root / "skills" / "README.md").write_text("| Skill | Purpose |\n| --- | --- |\n", encoding="utf-8")
      (root / "docs" / "learn").mkdir(parents=True)
      (root / "docs" / "learn" / "04-skills.md").write_text(
        "| Skill | Purpose |\n| --- | --- |\n",
        encoding="utf-8",
      )
      V.ROOT = root
      errors = V.validate_skill_tables([skill])
      if len(errors) != 2:
        raise AssertionError(f"expected both skill tables to report missing entries: {errors}")
  finally:
    V.ROOT = original_root


def test_validate_skill_tables_accepts_epub3_prefixed_skill_names() -> None:
  original_root = V.ROOT
  try:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      skill = write_skill(root, "epub3-migrator")
      table = "| Skill | Purpose |\n| --- | --- |\n| `epub3-migrator` | migrate |\n"
      (root / "skills" / "README.md").write_text(table, encoding="utf-8")
      (root / "docs" / "learn").mkdir(parents=True)
      (root / "docs" / "learn" / "04-skills.md").write_text(table, encoding="utf-8")
      V.ROOT = root
      errors = V.validate_skill_tables([skill])
      if errors:
        raise AssertionError(f"valid epub3-prefixed skill table row was ignored: {errors}")
  finally:
    V.ROOT = original_root


def test_footnote_class_tokens_reject_unknown_token() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    spec = root / "SPEC.md"
    target = root / "guide.md"
    spec.write_text(
      """# SPEC

## 1) 弹注

`ol.footnote-list > li.footnote-item`
`<ol class="footnote-list duokan-footnote-content">`
`<li class="footnote-item duokan-footnote-item">`

## 2) Other
""",
      encoding="utf-8",
    )
    target.write_text(
      '<ol class="footnote-list duokan-footnote-typo"></ol>\n',
      encoding="utf-8",
    )
    errors = V.validate_footnote_class_tokens(spec, [target])
    if len(errors) != 1:
      raise AssertionError(f"expected one unknown footnote token: {errors}")
    if str(target) not in errors[0] or "duokan-footnote-typo" not in errors[0]:
      raise AssertionError(f"error must identify file and token: {errors}")


def test_repository_footnote_class_tokens_match_spec() -> None:
  spec = ROOT / "docs" / "final" / "SPEC-实现约束.md"
  targets = V.footnote_contract_markdown_paths(ROOT)
  errors = V.validate_footnote_class_tokens(spec, targets)
  if errors:
    raise AssertionError(f"repository footnote tokens should match SPEC §1: {errors}")


def test_footnote_contract_scans_current_how_to_directory() -> None:
  paths = V.footnote_contract_markdown_paths(ROOT)
  if not any(path.parent == ROOT / "docs" / "how-to" for path in paths):
    raise AssertionError(f"footnote contract must scan docs/how-to: {paths}")


def test_repository_exposes_content_and_font_analysis_skills() -> None:
  names = {
    folder.name
    for folder in (ROOT / "skills").iterdir()
    if folder.is_dir() and (folder / "SKILL.md").is_file()
  }
  expected = {"epub-content-analyzer", "epub-font-coverage-analyzer"}
  missing = expected - names
  if missing:
    raise AssertionError(f"missing analysis skills: {sorted(missing)}")


def test_repository_exposes_concrete_package_operator_skill() -> None:
  skill = ROOT / "skills" / "epub-package-operator" / "SKILL.md"
  if not skill.is_file():
    raise AssertionError(f"missing concrete package operator skill: {skill}")
  text = skill.read_text(encoding="utf-8")
  for harness in (
    "epub_package_merge_harness.py",
    "epub_package_split_harness.py",
    "epub_metadata_edit_harness.py",
    "epub_cover_replace_harness.py",
  ):
    if harness not in text:
      raise AssertionError(f"package operator skill missing harness: {harness}")


def test_repository_exposes_epub3_migrator_skill() -> None:
  skill = ROOT / "skills" / "epub3-migrator" / "SKILL.md"
  if not skill.is_file():
    raise AssertionError(f"missing EPUB3 migrator skill: {skill}")
  text = skill.read_text(encoding="utf-8")
  for required in ("epub3_migration_harness.py", "epub3_migration_apply_harness.py", "validate_text_invariance.py"):
    if required not in text:
      raise AssertionError(f"EPUB3 migrator skill missing workflow requirement: {required}")


def main() -> int:
  test_validate_skill_accepts_minimal_valid_skill()
  test_validate_skill_rejects_extra_frontmatter()
  test_validate_skill_tables_detects_missing_entries()
  test_validate_skill_tables_accepts_epub3_prefixed_skill_names()
  test_footnote_class_tokens_reject_unknown_token()
  test_repository_footnote_class_tokens_match_spec()
  test_footnote_contract_scans_current_how_to_directory()
  test_repository_exposes_content_and_font_analysis_skills()
  test_repository_exposes_concrete_package_operator_skill()
  test_repository_exposes_epub3_migrator_skill()
  print("validate_skills_basic tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
