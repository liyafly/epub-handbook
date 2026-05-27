#!/usr/bin/env python3
"""Stdlib-only validation for local skill folders.

agents/openai.yaml intentionally supports only flat string keys; do not introduce
lists, nested maps, or block scalars without replacing parse_simple_yaml_strings.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILLS = ROOT / "skills"
NAME_RE = re.compile(r"^[a-z0-9-]{1,63}$")
CONTRACTS: dict[str, list[tuple[str, str]]] = {
  "epub-css-layering-optimizer": [
    ("skills/epub-css-layering-optimizer/SKILL.md", "docs/guides/note-box-border-styles.md"),
    ("docs/guides/note-box-border-styles.md", "便签、边框与阴影文本框"),
    ("templates/epub-style-demo/OEBPS/Styles/effects.css", ".note-box"),
    ("templates/epub-style-demo/OEBPS/Text/19-border-shadow-notes.xhtml", "note-box note-shadow"),
  ],
  "epub-english-typography-optimizer": [
    ("docs/guides/english-fiction-layout.md", "templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml"),
    ("docs/guides/anthology-navigation.md", "局部目录"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".english-fiction"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".en-noindent"),
  ],
  "epub-literary-structure-formatter": [
    ("skills/epub-literary-structure-formatter/SKILL.md", "docs/guides/chapter-head-image.md"),
    ("skills/epub-literary-structure-formatter/SKILL.md", "docs/guides/anthology-navigation.md"),
    ("skills/epub-literary-structure-formatter/SKILL.md", "docs/guides/classical-modern-layout.md"),
    ("docs/guides/anthology-navigation.md", "回本卷目录"),
    ("docs/guides/chapter-head-image.md", "chapter-head-art"),
    ("docs/guides/chapter-head-image.md", "chapter-head-banner"),
    ("docs/guides/classical-modern-layout.md", "文白对照"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".chapter-head"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".chapter-head-art"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".chapter-head-banner"),
    ("templates/epub-style-demo/OEBPS/Styles/literary.css", ".classical-modern"),
    ("templates/epub-style-demo/OEBPS/Text/21-classical-modern.xhtml", "classical-text book-song"),
  ],
  "epub-package-nav-auditor": [
    ("skills/epub-package-nav-auditor/SKILL.md", "docs/guides/anthology-navigation.md"),
    ("docs/guides/anthology-navigation.md", "局部目录"),
    ("templates/epub-style-demo/OEBPS/package.opf", 'properties="svg"'),
  ],
  "epub-typography-optimizer": [
    ("skills/epub-typography-optimizer/SKILL.md", "C1-body"),
    ("skills/epub-typography-optimizer/SKILL.md", "默认 `font-family` 链最多 4 段"),
    ("docs/plans/fonts-css-expansion-plan.md", "模式 C1-body"),
    ("templates/epub-style-demo/OEBPS/Styles/fonts.css", ".book-song"),
  ],
}


def parse_frontmatter(path: Path) -> dict[str, str]:
  text = path.read_text(encoding="utf-8")
  lines = text.splitlines()
  if not lines or lines[0] != "---":
    raise ValueError("missing opening frontmatter marker")
  try:
    end = lines.index("---", 1)
  except ValueError as exc:
    raise ValueError("missing closing frontmatter marker") from exc
  data: dict[str, str] = {}
  for raw in lines[1:end]:
    if not raw.strip():
      continue
    if ":" not in raw:
      raise ValueError(f"invalid frontmatter line: {raw}")
    key, value = raw.split(":", 1)
    data[key.strip()] = value.strip().strip('"')
  body = "\n".join(lines[end + 1:]).strip()
  if not body:
    raise ValueError("empty skill body")
  return data


def parse_simple_yaml_strings(path: Path) -> dict[str, str]:
  values: dict[str, str] = {}
  for raw in path.read_text(encoding="utf-8").splitlines():
    stripped = raw.strip()
    if not stripped or stripped == "interface:" or stripped.startswith("#"):
      continue
    if ":" not in stripped:
      continue
    key, value = stripped.split(":", 1)
    values[key.strip()] = value.strip().strip('"')
  return values


def validate_skill(folder: Path) -> list[str]:
  errors: list[str] = []
  skill_path = folder / "SKILL.md"
  try:
    meta = parse_frontmatter(skill_path)
  except ValueError as exc:
    return [f"{skill_path}: {exc}"]

  name = meta.get("name", "")
  desc = meta.get("description", "")
  if name != folder.name:
    errors.append(f"{skill_path}: name must match folder name")
  if not NAME_RE.match(name):
    errors.append(f"{skill_path}: invalid skill name: {name}")
  if not desc or "TODO" in desc or len(desc) < 20:
    errors.append(f"{skill_path}: description is missing or too short")
  allowed = {"name", "description"}
  extra = set(meta) - allowed
  if extra:
    errors.append(f"{skill_path}: unsupported frontmatter keys: {sorted(extra)}")

  agent_path = folder / "agents" / "openai.yaml"
  if not agent_path.exists():
    errors.append(f"{folder}: missing agents/openai.yaml")
  else:
    values = parse_simple_yaml_strings(agent_path)
    for key in ("display_name", "short_description", "default_prompt"):
      if not values.get(key):
        errors.append(f"{agent_path}: missing {key}")
    prompt = values.get("default_prompt", "")
    if f"${name}" not in prompt:
      errors.append(f"{agent_path}: default_prompt must mention ${name}")

  for rel_path, token in CONTRACTS.get(name, []):
    target = ROOT / rel_path
    if not target.exists():
      errors.append(f"{skill_path}: contract target missing: {rel_path}")
      continue
    if token not in target.read_text(encoding="utf-8"):
      errors.append(f"{skill_path}: contract token missing in {rel_path}: {token}")
  return errors


def main() -> int:
  errors: list[str] = []
  folders = [p for p in sorted(SKILLS.iterdir()) if (p / "SKILL.md").exists()]
  for folder in folders:
    errors.extend(validate_skill(folder))
  if errors:
    for error in errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print(f"skills basic validation ok ({len(folders)} skills)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
