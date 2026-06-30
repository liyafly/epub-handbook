#!/usr/bin/env python3
"""Regression tests for active documentation and skill consistency."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))

import validate_docs_consistency as V  # noqa: E402


ROOT = Path(__file__).resolve().parents[1]


class DocsConsistencyTests(unittest.TestCase):
  def test_detects_broken_links_removed_paths_and_forbidden_aliases(self) -> None:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      (root / "docs" / "how-to").mkdir(parents=True)
      (root / "docs" / "how-to" / "bad.md").write_text(
        "[missing](missing.md)\n`docs/plans/old.md`\nfont-family: BookSongBody;\n",
        encoding="utf-8",
      )
      errors = V.validate_repository(root)
      joined = "\n".join(errors)
      self.assertIn("broken relative link", joined)
      self.assertIn("removed docs/plans reference", joined)
      self.assertIn("forbidden active font alias", joined)

  def test_detects_fake_skill_invocation_commands(self) -> None:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      folder = root / "skills" / "epub-test"
      folder.mkdir(parents=True)
      (folder / "SKILL.md").write_text("```sh\n<skill-invocation> book.epub\n```\n", encoding="utf-8")
      self.assertTrue(any("fake <skill-invocation>" in error for error in V.validate_repository(root)))

  def test_html_links_are_checked_but_escaped_code_examples_are_not_resources(self) -> None:
    with TemporaryDirectory() as raw:
      root = Path(raw)
      folder = root / "docs" / "final"
      folder.mkdir(parents=True)
      page = folder / "reference.html"
      page.write_text(
        '<code>&lt;img src="../Images/note.png"/&gt;</code>'
        '<a href="missing.html">missing</a>',
        encoding="utf-8",
      )
      errors = V.validate_repository(root)
      self.assertEqual(len([error for error in errors if "broken HTML reference" in error]), 1)
      (folder / "missing.html").write_text("ok", encoding="utf-8")
      self.assertEqual(V.validate_repository(root), [])

  def test_repository_docs_skills_and_templates_are_consistent(self) -> None:
    self.assertEqual(V.validate_repository(ROOT), [])

  def test_repository_gates_run_consistency_validator(self) -> None:
    hook = (ROOT / "hooks" / "pre-commit.epub-handbook").read_text(encoding="utf-8")
    workflow = (ROOT / ".github" / "workflows" / "build-epub-demo.yml").read_text(encoding="utf-8")
    self.assertIn("python3 scripts/validate_docs_consistency.py", hook)
    self.assertIn("python3 scripts/validate_docs_consistency.py", workflow)


if __name__ == "__main__":
  unittest.main()
