#!/usr/bin/env python3
"""Consolidate repeated EPUB CSS and add conservative text-role markup."""

from __future__ import annotations

import argparse
import hashlib
import json
import posixpath
import re
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from xml.etree import ElementTree as ET

from epub3_oneclick_converter import (
  OPF_NS,
  OPF_URI,
  ensure_stylesheet_link,
  norm_join,
  opf_path_from_container,
  parse_xml,
  read_epub_files,
  rel_href,
  write_epub,
)


SONG_CHAIN = '"Songti SC", "SimSun", "Noto Serif CJK SC", serif'
HEI_CHAIN = '"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif'
KAI_CHAIN = '"Kaiti SC", "STKaiti", "KaiTi", serif'
PARENTHETICAL_CSS = f'''.type-parenthetical {{
  color: #6f5a50;
  font-family: {KAI_CHAIN};
  font-size: 0.92em;
}}
'''

COMMENT_RE = re.compile(r"/\*.*?\*/", re.S)
RULE_RE = re.compile(r"([^{}]+)\{([^{}]*)\}", re.S)
FONT_FAMILY_RE = re.compile(r"(font-family\s*:\s*)([^;}}]+)", re.I)
LINK_RE = re.compile(r"<link\b[^>]*\bhref=(?P<quote>[\"'])(?P<href>[^\"']+\.css)(?P=quote)[^>]*/?>", re.I)
BODY_RE = re.compile(r"<body\b(?P<attrs>[^>]*)>", re.I)
CLASS_RE = re.compile(r"\bclass=(?P<quote>[\"'])(?P<classes>[^\"']*)(?P=quote)", re.I)
TAG_RE = re.compile(r"<!--.*?-->|<![^>]*>|<[^>]+>", re.S)
OPEN_TAG_RE = re.compile(r"<\s*([A-Za-z][\w:-]*)\b([^>]*)>", re.S)
CLOSE_TAG_RE = re.compile(r"</\s*([A-Za-z][\w:-]*)\s*>", re.S)
PARENTHETICAL_RE = re.compile(r"（[^（）\n]+）")
ORNAMENT_RE = re.compile(r"(?m)^\s*[—-]{3,}.*?标题.*?[—-]{3,}\s*$")
MISSING_SEMICOLON_RE = re.compile(r"(?m)(^\s*[-\w]+\s*:\s*[^;{}\n]+)\n(?=\s*[-\w]+\s*:)")


class CleanupError(Exception):
  """The EPUB CSS cleanup cannot continue safely."""


@dataclass(frozen=True)
class Rule:
  selector: str
  declarations: tuple[tuple[str, str], ...]


@dataclass
class CleanupReport:
  input: str
  output: str
  opf: str = ""
  css_files_before: int = 0
  css_files_after: int = 0
  factored_stylesheets: int = 0
  duplicate_stylesheets_removed: int = 0
  overrides_created: int = 0
  font_declarations_rewritten: int = 0
  parentheticals_marked: int = 0
  xhtml_files_updated: int = 0
  css_manifest_items_removed: int = 0
  css_manifest_items_added: int = 0
  scoped_local_stylesheets_merged: int = 0
  scope_classes_added: int = 0
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return {
      "harness": "epub_css_cleanup",
      "input": self.input,
      "output": self.output,
      "opf": self.opf,
      "css_files_before": self.css_files_before,
      "css_files_after": self.css_files_after,
      "factored_stylesheets": self.factored_stylesheets,
      "duplicate_stylesheets_removed": self.duplicate_stylesheets_removed,
      "overrides_created": self.overrides_created,
      "font_declarations_rewritten": self.font_declarations_rewritten,
      "parentheticals_marked": self.parentheticals_marked,
      "xhtml_files_updated": self.xhtml_files_updated,
      "css_manifest_items_removed": self.css_manifest_items_removed,
      "css_manifest_items_added": self.css_manifest_items_added,
      "scoped_local_stylesheets_merged": self.scoped_local_stylesheets_merged,
      "scope_classes_added": self.scope_classes_added,
      "warnings": self.warnings,
    }


def q(uri: str, name: str) -> str:
  return f"{{{uri}}}{name}"


def normalize_space(value: str) -> str:
  return re.sub(r"\s+", " ", value).strip()


def normalize_css(value: str) -> str:
  return re.sub(r"\s+", "", COMMENT_RE.sub("", value)).lower()


def system_font_family(value: str) -> str | None:
  compact = re.sub(r"\s+", "", value).lower()
  if compact in {'"cnepub",serif', '"simsun"'}:
    return SONG_CHAIN
  if compact == '"simhei"':
    return HEI_CHAIN
  if compact == '"stkaiti"':
    return KAI_CHAIN
  return None


def sanitize_css(value: str) -> tuple[str, int]:
  value = ORNAMENT_RE.sub("", value)
  value = MISSING_SEMICOLON_RE.sub(r"\1;\n", value)
  rewrites = 0

  def replace_font(match: re.Match[str]) -> str:
    nonlocal rewrites
    replacement = system_font_family(match.group(2))
    if replacement is None:
      return match.group(0)
    rewrites += 1
    return f"{match.group(1)}{replacement}"

  value = FONT_FAMILY_RE.sub(replace_font, value)
  return value.strip() + "\n", rewrites


def parse_stylesheet(value: str) -> list[Rule] | None:
  without_comments = COMMENT_RE.sub("", value)
  rules: list[Rule] = []
  for match in RULE_RE.finditer(without_comments):
    selector = normalize_space(match.group(1))
    if not selector or selector.startswith("@"):
      return None
    declarations: list[tuple[str, str]] = []
    for raw in match.group(2).split(";"):
      raw = raw.strip()
      if not raw:
        continue
      if ":" not in raw:
        return None
      name, value = raw.split(":", 1)
      declarations.append((name.strip(), normalize_space(value)))
    rules.append(Rule(selector=selector, declarations=tuple(declarations)))
  return rules or None


def stylesheet_shape(rules: list[Rule]) -> tuple[tuple[str, tuple[str, ...]], ...]:
  return tuple((rule.selector, tuple(name.lower() for name, _ in rule.declarations)) for rule in rules)


def is_cleanup_generated_css(css_path: str) -> bool:
  name = posixpath.basename(css_path)
  return name.startswith(("clean-shared-", "clean-override-", "clean-scoped-local"))


def format_rules(rules: list[Rule]) -> bytes:
  chunks: list[str] = []
  for rule in rules:
    chunks.append(f"{rule.selector} {{")
    chunks.extend(f"  {name}: {value};" for name, value in rule.declarations)
    chunks.append("}")
    chunks.append("")
  return ("\n".join(chunks).rstrip() + "\n").encode("utf-8")


def unique_zip_path(files: dict[str, bytes], base: str) -> str:
  stem, ext = posixpath.splitext(base)
  candidate = base
  index = 2
  while candidate in files:
    candidate = f"{stem}-{index}{ext}"
    index += 1
  return candidate


def sha256_text(value: str) -> str:
  return hashlib.sha256(normalize_css(value).encode("utf-8")).hexdigest()


def css_manifest_items(root: ET.Element, opf_dir: str) -> dict[str, ET.Element]:
  return {
    norm_join(opf_dir, item.attrib["href"]): item
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("media-type") == "text/css" and item.attrib.get("href")
  }


def xhtml_zip_paths(root: ET.Element, opf_dir: str) -> list[str]:
  return [
    norm_join(opf_dir, item.attrib["href"])
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("media-type") == "application/xhtml+xml" and item.attrib.get("href")
  ]


def replacement_links(link: str, xhtml_path: str, css_paths: list[str]) -> str:
  indent_match = re.search(r"(?m)^([ \t]*)[^\n]*" + re.escape(link), link)
  indent = indent_match.group(1) if indent_match else ""
  links: list[str] = []
  for css_path in css_paths:
    href = rel_href(xhtml_path, css_path)
    links.append(re.sub(r"href=([\"'])[^\"']+\1", f'href="{href}"', link, count=1))
  return ("\n" + indent).join(links)


def rewrite_css_links(text: str, xhtml_path: str, mapping: dict[str, list[str]]) -> tuple[str, bool]:
  changed = False

  def replace(match: re.Match[str]) -> str:
    nonlocal changed
    href = match.group("href")
    css_path = posixpath.normpath(posixpath.join(posixpath.dirname(xhtml_path), href))
    css_paths = mapping.get(css_path)
    if not css_paths:
      return match.group(0)
    changed = True
    return replacement_links(match.group(0), xhtml_path, css_paths)

  return LINK_RE.sub(replace, text), changed


def linked_css_paths(text: str, xhtml_path: str) -> list[str]:
  return [
    posixpath.normpath(posixpath.join(posixpath.dirname(xhtml_path), match.group("href")))
    for match in LINK_RE.finditer(text)
  ]


def add_body_class(text: str, class_name: str) -> tuple[str, bool]:
  def replace_body(match: re.Match[str]) -> str:
    attrs = match.group("attrs")
    class_match = CLASS_RE.search(attrs)
    if class_match:
      classes = class_match.group("classes").split()
      if class_name in classes:
        return match.group(0)
      classes.append(class_name)
      updated = " ".join(classes)
      attrs = CLASS_RE.sub(f'class="{updated}"', attrs, count=1)
      return f"<body{attrs}>"
    return f'<body{attrs} class="{class_name}">'

  updated, count = BODY_RE.subn(replace_body, text, count=1)
  return updated, bool(count and updated != text)


def scoped_selector(selector: str, scope_class: str) -> str:
  scoped: list[str] = []
  for part in selector.split(","):
    part = part.strip()
    if re.match(r"^body(?:\b|[.#:[ ])", part, re.I):
      scoped.append(re.sub(r"^body", f"body.{scope_class}", part, count=1, flags=re.I))
    else:
      scoped.append(f"body.{scope_class} {part}")
  return ",\n".join(scoped)


def format_scoped_rules(scope_class: str, css_path: str, rules: list[Rule]) -> list[str]:
  chunks = [f"/* Scoped from {css_path}. */"]
  for rule in rules:
    chunks.append(f"{scoped_selector(rule.selector, scope_class)} {{")
    chunks.extend(f"  {name}: {value};" for name, value in rule.declarations)
    chunks.append("}")
    chunks.append("")
  return chunks


def consolidate_scoped_local_css(
  files: dict[str, bytes],
  xhtml_paths: list[str],
  opf_dir: str,
  removed: set[str],
  generated: dict[str, bytes],
  report: CleanupReport,
) -> None:
  refs: dict[str, set[str]] = defaultdict(set)
  for xhtml_path in xhtml_paths:
    if xhtml_path not in files:
      continue
    text = files[xhtml_path].decode("utf-8", errors="replace")
    for css_path in linked_css_paths(text, xhtml_path):
      refs[css_path].add(xhtml_path)

  excluded_names = {"epub3-enhancements.css", "parentheticals.css", "clean-scoped-local.css"}
  candidates: dict[str, list[Rule]] = {}
  for css_path, pages in refs.items():
    name = posixpath.basename(css_path)
    if not pages or css_path not in files:
      continue
    if name in excluded_names or name.startswith("clean-shared-"):
      continue
    rules = parse_stylesheet(files[css_path].decode("utf-8", errors="replace"))
    if rules:
      candidates[css_path] = rules

  overlapping: set[str] = set()
  paths = sorted(candidates)
  for index, css_path in enumerate(paths):
    for other in paths[index + 1:]:
      if refs[css_path] & refs[other]:
        overlapping.update({css_path, other})
  if overlapping:
    report.warnings.append(
      "skipped overlapping local stylesheets: " + ", ".join(sorted(overlapping))
    )

  merge_paths = [path for path in paths if path not in overlapping]
  if not merge_paths:
    return

  scoped_path = unique_zip_path({**files, **generated}, norm_join(opf_dir, "Styles/clean-scoped-local.css"))
  chunks: list[str] = []
  scope_by_path: dict[str, str] = {}
  for index, css_path in enumerate(merge_paths, start=1):
    scope_class = f"css-local-{index:02d}"
    scope_by_path[css_path] = scope_class
    chunks.extend(format_scoped_rules(scope_class, css_path, candidates[css_path]))

  generated[scoped_path] = ("\n".join(chunks).rstrip() + "\n").encode("utf-8")
  files[scoped_path] = generated[scoped_path]
  mapping = {css_path: [scoped_path] for css_path in merge_paths}
  affected_pages = sorted({page for css_path in merge_paths for page in refs[css_path]})
  for xhtml_path in affected_pages:
    text = files[xhtml_path].decode("utf-8", errors="replace")
    for css_path in merge_paths:
      if xhtml_path in refs[css_path]:
        text, added = add_body_class(text, scope_by_path[css_path])
        report.scope_classes_added += int(added)
    text, _ = rewrite_css_links(text, xhtml_path, mapping)
    files[xhtml_path] = text.encode("utf-8")

  for css_path in merge_paths:
    files.pop(css_path, None)
    generated.pop(css_path, None)
    removed.add(css_path)
  report.scoped_local_stylesheets_merged += len(merge_paths)


def start_tag_is_excluded(tag: str, attrs: str) -> bool:
  if tag in {"head", "aside", "script", "style"}:
    return True
  return tag == "span" and re.search(r'class=[\"\'][^\"\']*\btype-parenthetical\b', attrs, re.I) is not None


def mark_parenthetical_text(text: str) -> tuple[str, int]:
  chunks = TAG_RE.split(text)
  tags = TAG_RE.findall(text)
  stack: list[tuple[str, bool]] = []
  marked = 0

  def replace_text(value: str) -> str:
    nonlocal marked
    if any(excluded for _, excluded in stack):
      return value
    updated, count = PARENTHETICAL_RE.subn(r'<span class="type-parenthetical">\g<0></span>', value)
    marked += count
    return updated

  output = [replace_text(chunks[0])]
  for tag_text, chunk in zip(tags, chunks[1:]):
    close = CLOSE_TAG_RE.fullmatch(tag_text)
    if close:
      tag = close.group(1).lower()
      if stack and stack[-1][0] == tag:
        stack.pop()
      output.append(tag_text)
      output.append(replace_text(chunk))
      continue
    opened = OPEN_TAG_RE.fullmatch(tag_text)
    output.append(tag_text)
    if opened and not tag_text.rstrip().endswith("/>"):
      tag = opened.group(1).lower()
      stack.append((tag, start_tag_is_excluded(tag, opened.group(2))))
    output.append(replace_text(chunk))
  return "".join(output), marked


def add_css_manifest_item(root: ET.Element, opf_dir: str, css_path: str, report: CleanupReport) -> None:
  href = posixpath.relpath(css_path, opf_dir) if opf_dir else css_path
  if any(item.attrib.get("href") == href for item in root.findall("opf:manifest/opf:item", OPF_NS)):
    return
  ids = {item.attrib.get("id") for item in root.findall("opf:manifest/opf:item", OPF_NS)}
  item_id = re.sub(r"[^A-Za-z0-9_.-]+", "-", f"css-{Path(href).stem}").strip("-")
  base = item_id
  index = 2
  while item_id in ids:
    item_id = f"{base}-{index}"
    index += 1
  manifest = root.find("opf:manifest", OPF_NS)
  if manifest is None:
    raise CleanupError("OPF missing manifest")
  ET.SubElement(manifest, q(OPF_URI, "item"), {"id": item_id, "href": href, "media-type": "text/css"})
  report.css_manifest_items_added += 1


def clean_epub_css(
  input_path: Path,
  output_path: Path,
  mark_parentheticals: bool = False,
  merge_scoped_local_css: bool = False,
) -> CleanupReport:
  report = CleanupReport(input=str(input_path), output=str(output_path))
  files, original_order = read_epub_files(input_path)
  opf_path = opf_path_from_container(files)
  report.opf = opf_path
  root = parse_xml(files[opf_path], opf_path)
  opf_dir = posixpath.dirname(opf_path)
  css_items = css_manifest_items(root, opf_dir)
  report.css_files_before = len(css_items)

  css_text: dict[str, str] = {}
  parsed: dict[str, list[Rule]] = {}
  for css_path in sorted(css_items):
    if css_path not in files:
      report.warnings.append(f"CSS manifest item does not resolve: {css_path}")
      continue
    cleaned, rewrites = sanitize_css(files[css_path].decode("utf-8", errors="replace"))
    css_text[css_path] = cleaned
    report.font_declarations_rewritten += rewrites
    rules = parse_stylesheet(cleaned)
    if rules:
      parsed[css_path] = rules

  mapping: dict[str, list[str]] = {}
  removed: set[str] = set()
  generated: dict[str, bytes] = {}
  groups: dict[tuple[tuple[str, tuple[str, ...]], ...], list[str]] = defaultdict(list)
  for css_path, rules in parsed.items():
    if is_cleanup_generated_css(css_path):
      continue
    groups[stylesheet_shape(rules)].append(css_path)

  shared_index = 1
  for paths in sorted(groups.values(), key=lambda value: value[0]):
    if len(paths) < 3 or len({sha256_text(css_text[path]) for path in paths}) < 2:
      continue
    paths = sorted(paths)
    canonical = paths[0]
    css_dir = posixpath.dirname(canonical)
    shared_path = unique_zip_path({**files, **generated}, posixpath.join(css_dir, f"clean-shared-{shared_index:02d}.css"))
    shared_index += 1
    canonical_rules = parsed[canonical]
    generated[shared_path] = format_rules(canonical_rules)
    for css_path in paths:
      changed_rules = [
        rule
        for rule, canonical_rule in zip(parsed[css_path], canonical_rules)
        if rule.declarations != canonical_rule.declarations
      ]
      replacement = [shared_path]
      if changed_rules:
        override_base = posixpath.join(css_dir, f"clean-override-{Path(css_path).stem}.css")
        override_path = unique_zip_path({**files, **generated}, override_base)
        generated[override_path] = format_rules(changed_rules)
        replacement.append(override_path)
        report.overrides_created += 1
      mapping[css_path] = replacement
      removed.add(css_path)
    report.factored_stylesheets += len(paths)

  digest_paths: dict[str, str] = {}
  for css_path in sorted(css_text):
    if css_path in removed:
      continue
    digest = sha256_text(css_text[css_path])
    canonical = digest_paths.get(digest)
    if canonical is None:
      digest_paths[digest] = css_path
      continue
    mapping[css_path] = [canonical]
    removed.add(css_path)
    report.duplicate_stylesheets_removed += 1

  for css_path, value in css_text.items():
    if css_path not in removed:
      files[css_path] = value.encode("utf-8")
  for css_path in removed:
    files.pop(css_path, None)
  files.update(generated)

  parenthetical_css_path = ""
  if mark_parentheticals:
    parenthetical_css_path = unique_zip_path(files, norm_join(opf_dir, "Styles/parentheticals.css"))
    files[parenthetical_css_path] = PARENTHETICAL_CSS.encode("utf-8")

  xhtml_paths = xhtml_zip_paths(root, opf_dir)
  for xhtml_path in xhtml_paths:
    if xhtml_path not in files:
      continue
    text = files[xhtml_path].decode("utf-8", errors="replace")
    text, links_changed = rewrite_css_links(text, xhtml_path, mapping)
    marked = 0
    if mark_parentheticals:
      text, marked = mark_parenthetical_text(text)
      if marked:
        href = rel_href(xhtml_path, parenthetical_css_path)
        text, _ = ensure_stylesheet_link(text, href)
    if links_changed or marked:
      files[xhtml_path] = text.encode("utf-8")
      report.xhtml_files_updated += 1
      report.parentheticals_marked += marked

  if merge_scoped_local_css:
    consolidate_scoped_local_css(files, xhtml_paths, opf_dir, removed, generated, report)

  for item in list(root.findall("opf:manifest/opf:item", OPF_NS)):
    href = item.attrib.get("href")
    if item.attrib.get("media-type") != "text/css" or not href:
      continue
    if norm_join(opf_dir, href) in removed:
      manifest = root.find("opf:manifest", OPF_NS)
      if manifest is not None:
        manifest.remove(item)
        report.css_manifest_items_removed += 1

  for css_path in sorted(generated):
    add_css_manifest_item(root, opf_dir, css_path, report)
  if parenthetical_css_path:
    add_css_manifest_item(root, opf_dir, parenthetical_css_path, report)

  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  report.css_files_after = len([name for name in files if name.lower().endswith(".css")])
  write_epub(output_path, files, original_order)
  return report


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Consolidate repeated EPUB CSS and add conservative text-role markup")
  parser.add_argument("input", type=Path, help="Input EPUB")
  parser.add_argument("--output", type=Path, required=True, help="Output EPUB")
  parser.add_argument("--mark-parentheticals", action="store_true", help="Mark body text in Chinese round parentheses")
  parser.add_argument(
    "--merge-scoped-local-css",
    action="store_true",
    help="Merge disjoint local stylesheets into one body-scoped stylesheet",
  )
  parser.add_argument("--format", choices=("json", "text"), default="text")
  args = parser.parse_args(argv)
  try:
    report = clean_epub_css(
      args.input,
      args.output,
      mark_parentheticals=args.mark_parentheticals,
      merge_scoped_local_css=args.merge_scoped_local_css,
    )
  except CleanupError as exc:
    if args.format == "json":
      print(json.dumps({"harness": "epub_css_cleanup", "error": str(exc)}, ensure_ascii=False, indent=2))
    else:
      print(f"ERROR: {exc}", file=sys.stderr)
    return 1
  if args.format == "json":
    print(json.dumps(report.as_dict(), ensure_ascii=False, indent=2))
  else:
    print(f"Wrote cleaned EPUB: {args.output}")
    print(f"CSS files: {report.css_files_before} -> {report.css_files_after}")
    print(f"Parentheticals marked: {report.parentheticals_marked}")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
