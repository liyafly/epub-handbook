#!/usr/bin/env python3
"""Validate active docs, skills, templates, and cross-layer EPUB rules."""

from __future__ import annotations

import argparse
import re
import sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[1]
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
REF_LINK_RE = re.compile(r"^\s*\[[^\]]+\]:\s*(\S+)", re.M)
FORBIDDEN_ALIAS_RE = re.compile(
  r"Book(?:Song|Kai|Hei|FangSong|Title|Signature|Body|Serif|Hand)[A-Za-z0-9_-]*|"
  r"RareSong[A-Za-z0-9_-]*|\.book-[A-Za-z0-9_*-]+"
)


class _HTMLReferenceCollector(HTMLParser):
  def __init__(self) -> None:
    super().__init__(convert_charrefs=True)
    self.references: list[tuple[int, str]] = []

  def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
    line, _ = self.getpos()
    for name, value in attrs:
      if name.lower() in {"href", "src"} and value:
        self.references.append((line, value))


def rel(root: Path, path: Path) -> str:
  return path.relative_to(root).as_posix()


def markdown_files(root: Path) -> list[Path]:
  paths = [*root.glob("*.md"), *root.glob("docs/**/*.md"), *root.glob("skills/*/SKILL.md")]
  return sorted({path for path in paths if path.is_file()})


def _link_target(raw: str) -> str | None:
  value = raw.strip()
  if value.startswith("<") and value.endswith(">"):
    value = value[1:-1]
  if ' "' in value:
    value = value.split(' "', 1)[0]
  if " '" in value:
    value = value.split(" '", 1)[0]
  if not value or value.startswith(("#", "http://", "https://", "mailto:", "data:", "javascript:")):
    return None
  value = unquote(value.split("#", 1)[0].split("?", 1)[0])
  if not value or any(char in value for char in "<>{}*"):
    return None
  return value


def validate_links(root: Path) -> list[str]:
  errors: list[str] = []
  for path in markdown_files(root):
    text = path.read_text(encoding="utf-8", errors="replace")
    for pattern in (LINK_RE, REF_LINK_RE):
      for match in pattern.finditer(text):
        target = _link_target(match.group(1))
        if target is None:
          continue
        resolved = Path(target) if target.startswith("/") else (path.parent / target).resolve()
        if not resolved.exists():
          line = text.count("\n", 0, match.start()) + 1
          errors.append(f"{rel(root, path)}:{line}: broken relative link: {match.group(1)}")
  return errors


def validate_html_links(root: Path) -> list[str]:
  errors: list[str] = []
  for path in sorted(root.glob("docs/**/*.html")):
    parser = _HTMLReferenceCollector()
    parser.feed(path.read_text(encoding="utf-8", errors="replace"))
    parser.close()
    for line, raw in parser.references:
      target = _link_target(raw)
      if target is None:
        continue
      resolved = Path(target) if target.startswith("/") else (path.parent / target).resolve()
      if not resolved.exists():
        errors.append(f"{rel(root, path)}:{line}: broken HTML reference: {raw}")
  return errors


def active_text_files(root: Path) -> list[Path]:
  values: list[Path] = []
  for pattern in (
    "*.md", "docs/**/*.md", "skills/*/SKILL.md", "scripts/*.py",
    "templates/**/*.md", "templates/**/*.css", "templates/**/*.xhtml",
  ):
    values.extend(root.glob(pattern))
  return sorted({
    path for path in values
    if path.is_file()
    and "experiments" not in path.parts
    and "source" not in path.parts
    and not path.name.startswith("test_")
    and path.name != "validate_docs_consistency.py"
  })


def validate_stale_references(root: Path) -> list[str]:
  errors: list[str] = []
  naming_spec = root / "docs" / "final" / "字体别名命名规范.md"
  spec = root / "docs" / "final" / "SPEC-实现约束.md"
  for path in active_text_files(root):
    text = path.read_text(encoding="utf-8", errors="replace")
    if "docs/plans/" in text:
      errors.append(f"{rel(root, path)}: removed docs/plans reference")
    if path not in {naming_spec, spec}:
      for match in FORBIDDEN_ALIAS_RE.finditer(text):
        line = text.count("\n", 0, match.start()) + 1
        errors.append(f"{rel(root, path)}:{line}: forbidden active font alias: {match.group(0)}")
  return errors


def validate_font_rules(root: Path) -> list[str]:
  checks = [
    (
      root / "docs" / "final" / "EPUB 3 终极实践手册.md",
      'body {\n  margin: 0;\n  padding: 0 1em;',
      'font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;',
      "handbook base body must keep free mode without font-family",
    ),
    (
      root / "docs" / "final" / "EPUB 3 HTML CSS 属性速查表.html",
      "Apple Books 正文锁定 / 嵌入字体声明",
      None,
      "HTML cheatsheet still says dedicated embedded fonts require ibooks meta",
    ),
    (
      root / "templates" / "epub-style-demo" / "OEBPS" / "Styles" / "fonts.css",
      "嵌入字体启用后，OPF 需同步加",
      None,
      "demo fonts.css contradicts dedicated-class ibooks meta rule",
    ),
    (
      root / "docs" / "learn" / "README.md",
      "不要在 `body` 上设 line-height",
      None,
      "beginner guide contradicts the unitless body line-height baseline",
    ),
  ]
  errors: list[str] = []
  for path, marker, secondary, message in checks:
    if not path.exists():
      continue
    text = path.read_text(encoding="utf-8", errors="replace")
    if marker in text and (secondary is None or secondary in text[text.index(marker):text.index(marker) + 600]):
      errors.append(f"{rel(root, path)}: {message}")
  spec = root / "docs" / "final" / "SPEC-实现约束.md"
  if spec.exists() and "fonts-css-expansion-plan" in spec.read_text(encoding="utf-8"):
    errors.append(f"{rel(root, spec)}: removed fonts-css-expansion-plan reference")
  return errors


def validate_reader_matrix(root: Path) -> list[str]:
  path = root / "docs" / "final" / "reader-matrix.yaml"
  if not path.exists():
    return []
  text = path.read_text(encoding="utf-8")
  section = text.split("retest_priorities:", 1)[-1].split("expectations:", 1)[0]
  records = re.findall(
    r"- rank:\s*(\d+)\s*\n\s*case:\s*([^\n]+)\s*\n\s*readers:\s*([^\n]+)\s*\n\s*focus:\s*([^\n]+)",
    section,
  )
  seen: dict[tuple[str, str, str], str] = {}
  errors: list[str] = []
  for rank, case, readers, focus in records:
    key = (case.strip(), readers.strip(), focus.strip())
    if key in seen:
      errors.append(f"{rel(root, path)}: duplicate retest priorities ranks {seen[key]} and {rank}")
    else:
      seen[key] = rank
  return errors


def validate_skill_placeholders(root: Path) -> list[str]:
  errors: list[str] = []
  for path in sorted((root / "skills").glob("*/SKILL.md")):
    if "<skill-invocation>" in path.read_text(encoding="utf-8"):
      errors.append(f"{rel(root, path)}: fake <skill-invocation> command must be replaced with an explicit agent/provider protocol")
  return errors


def validate_repository(root: Path = ROOT) -> list[str]:
  root = root.resolve()
  return sorted([
    *validate_links(root),
    *validate_html_links(root),
    *validate_stale_references(root),
    *validate_font_rules(root),
    *validate_reader_matrix(root),
    *validate_skill_placeholders(root),
  ])


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("--root", type=Path, default=ROOT)
  args = parser.parse_args(argv)
  errors = validate_repository(args.root)
  if errors:
    for error in errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("documentation consistency validation ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
