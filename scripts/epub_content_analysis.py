#!/usr/bin/env python3
"""Read-only structural role and typography analysis for EPUB/source text."""

from __future__ import annotations

import hashlib
import json
import posixpath
import re
import unicodedata
from collections import Counter
from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path
from typing import Iterable
from xml.etree import ElementTree as ET

from epub_lib import OPF_NS, local_name, norm_join, opf_path_from_container, parse_xml, read_epub_files


CAPABILITY = "epub.text.content.analyze"
EPUB_TYPE = "{http://www.idpf.org/2007/ops}type"
XML_LANG = "{http://www.w3.org/XML/1998/namespace}lang"
BLOCK_TAGS = {
  "h1", "h2", "h3", "h4", "h5", "h6", "p", "blockquote", "figcaption",
  "caption", "li", "dt", "dd", "pre", "code", "td", "th", "address", "hr",
}
CONTAINER_TAGS = {"html", "body", "main", "section", "article", "div", "aside", "nav"}
CLASS_SPLIT_RE = re.compile(r"\s+")
CHAPTER_RE = re.compile(r"^(?:第[〇零一二三四五六七八九十百千万两\d]+[章节卷回部篇]|chapter\s+\d+)", re.I)
SCENE_RE = re.compile(r"^(?:[*＊※·•—―\-]\s*){2,}$")
QUOTE_CHARS = set("“”‘’「」『』《》\"'")
SENTENCE_END = set("。！？!?；;：:")


class ContentAnalysisError(Exception):
  """The source cannot be analyzed safely."""


@dataclass(frozen=True)
class TextBlock:
  source: str
  locator: str
  tag: str
  classes: tuple[str, ...]
  ancestor_tags: tuple[str, ...]
  epub_types: tuple[str, ...]
  language: str | None
  text: str
  previous_tag: str | None = None
  next_tag: str | None = None


class _LooseHTMLCollector(HTMLParser):
  """Collect block text from ordinary, not necessarily XML-well-formed HTML."""

  def __init__(self, source: str):
    super().__init__(convert_charrefs=True)
    self.source = source
    self.stack: list[tuple[str, dict[str, str]]] = []
    self.counts: Counter[str] = Counter()
    self.blocks: list[TextBlock] = []
    self.current: dict[str, object] | None = None

  def _context(self) -> tuple[tuple[str, ...], tuple[str, ...], tuple[str, ...], str | None]:
    tags: list[str] = []
    classes: list[str] = []
    epub_types: list[str] = []
    language: str | None = None
    for tag, attrs in self.stack:
      tags.append(tag)
      classes.extend(_classes(attrs.get("class")))
      epub_types.extend((attrs.get("epub:type") or "").split())
      language = attrs.get("lang") or attrs.get("xml:lang") or language
    return tuple(tags), tuple(dict.fromkeys(classes)), tuple(dict.fromkeys(epub_types)), language

  def _finish(self) -> None:
    if self.current is None:
      return
    text = "".join(self.current["parts"]).strip()
    tag = str(self.current["tag"])
    if text or tag == "hr":
      self.blocks.append(TextBlock(
        source=self.source,
        locator=str(self.current["locator"]),
        tag=tag,
        classes=tuple(self.current["classes"]),
        ancestor_tags=tuple(self.current["ancestor_tags"]),
        epub_types=tuple(self.current["epub_types"]),
        language=self.current["language"] if isinstance(self.current["language"], str) else None,
        text=text,
      ))
    self.current = None

  def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
    tag = tag.lower()
    values = {key.lower(): value or "" for key, value in attrs}
    if tag == "br":
      if self.current is not None:
        self.current["parts"].append("\n")
      return
    self.stack.append((tag, values))
    if tag not in BLOCK_TAGS:
      return
    self._finish()
    self.counts[tag] += 1
    tags, classes, epub_types, language = self._context()
    self.current = {
      "tag": tag,
      "locator": f"/html/{tag}[{self.counts[tag]}]",
      "classes": classes,
      "ancestor_tags": tags,
      "epub_types": epub_types,
      "language": language,
      "parts": [],
    }
    if tag == "hr":
      self._finish()

  def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
    self.handle_starttag(tag, attrs)
    self.handle_endtag(tag)

  def handle_data(self, data: str) -> None:
    if self.current is not None:
      self.current["parts"].append(data)

  def handle_endtag(self, tag: str) -> None:
    tag = tag.lower()
    if self.current is not None and self.current["tag"] == tag:
      self._finish()
    for index in range(len(self.stack) - 1, -1, -1):
      if self.stack[index][0] == tag:
        del self.stack[index:]
        break

  def close(self) -> None:
    super().close()
    self._finish()


def _classes(value: str | None) -> tuple[str, ...]:
  return tuple(part for part in CLASS_SPLIT_RE.split((value or "").strip()) if part)


def _node_path(node: ET.Element, parents: dict[ET.Element, ET.Element]) -> str:
  parts: list[str] = []
  current: ET.Element | None = node
  while current is not None:
    tag = local_name(current.tag).lower()
    parent = parents.get(current)
    index = 1
    if parent is not None:
      siblings = [child for child in list(parent) if local_name(child.tag).lower() == tag]
      if current in siblings:
        index = siblings.index(current) + 1
    parts.append(f"{tag}[{index}]")
    current = parent
  return "/" + "/".join(reversed(parts))


def _nearest_language(node: ET.Element, parents: dict[ET.Element, ET.Element]) -> str | None:
  current: ET.Element | None = node
  while current is not None:
    if value := (current.attrib.get("lang") or current.attrib.get(XML_LANG)):
      return value
    current = parents.get(current)
  return None


def _context_tokens(
  node: ET.Element,
  parents: dict[ET.Element, ET.Element],
) -> tuple[tuple[str, ...], tuple[str, ...], tuple[str, ...]]:
  tags: list[str] = []
  classes: list[str] = []
  epub_types: list[str] = []
  chain: list[ET.Element] = []
  current: ET.Element | None = node
  while current is not None:
    chain.append(current)
    current = parents.get(current)
  for element in reversed(chain):
    tags.append(local_name(element.tag).lower())
    classes.extend(_classes(element.attrib.get("class")))
    epub_types.extend((element.attrib.get(EPUB_TYPE) or element.attrib.get("epub:type") or "").split())
  return tuple(tags), tuple(dict.fromkeys(classes)), tuple(dict.fromkeys(epub_types))


def _has_nested_block(node: ET.Element) -> bool:
  return any(local_name(child.tag).lower() in BLOCK_TAGS for child in list(node))


def _element_text(node: ET.Element) -> str:
  parts: list[str] = []

  def walk(element: ET.Element) -> None:
    if element.text:
      parts.append(element.text)
    for child in list(element):
      if local_name(child.tag).lower() == "br":
        parts.append("\n")
      else:
        walk(child)
      if child.tail:
        parts.append(child.tail)

  walk(node)
  return "".join(parts).strip()


def _extract_xhtml_blocks(source: str, content: str) -> list[TextBlock]:
  try:
    root = ET.fromstring(content)
  except ET.ParseError as exc:
    raise ContentAnalysisError(f"{source}: XHTML/XML parse failed: {exc}") from exc
  parents = {child: parent for parent in root.iter() for child in list(parent)}
  raw: list[TextBlock] = []
  for node in root.iter():
    tag = local_name(node.tag).lower()
    if tag not in BLOCK_TAGS:
      continue
    if tag in {"pre", "blockquote"} and _has_nested_block(node):
      continue
    text = "" if tag == "hr" else _element_text(node)
    if not text and tag != "hr":
      continue
    ancestor_tags, classes, epub_types = _context_tokens(node, parents)
    raw.append(TextBlock(
      source=source,
      locator=_node_path(node, parents),
      tag=tag,
      classes=classes,
      ancestor_tags=ancestor_tags,
      epub_types=epub_types,
      language=_nearest_language(node, parents),
      text=text,
    ))
  return _with_neighbors(raw)


def _extract_html_blocks(source: str, content: str) -> list[TextBlock]:
  collector = _LooseHTMLCollector(source)
  try:
    collector.feed(content)
    collector.close()
  except Exception as exc:
    raise ContentAnalysisError(f"{source}: HTML parse failed: {exc}") from exc
  return _with_neighbors(collector.blocks)


def _with_neighbors(blocks: list[TextBlock]) -> list[TextBlock]:
  result: list[TextBlock] = []
  for index, block in enumerate(blocks):
    result.append(TextBlock(
      source=block.source,
      locator=block.locator,
      tag=block.tag,
      classes=block.classes,
      ancestor_tags=block.ancestor_tags,
      epub_types=block.epub_types,
      language=block.language,
      text=block.text,
      previous_tag=blocks[index - 1].tag if index else None,
      next_tag=blocks[index + 1].tag if index + 1 < len(blocks) else None,
    ))
  return result


def _plain_blocks(source: str, content: str) -> list[TextBlock]:
  paragraphs = [part.strip() for part in re.split(r"\n\s*\n", content) if part.strip()]
  return _with_neighbors([
    TextBlock(source, f"/text/p[{index}]", "p", (), ("text", "p"), (), None, text)
    for index, text in enumerate(paragraphs, start=1)
  ])


def _markdown_blocks(source: str, content: str) -> list[TextBlock]:
  blocks: list[TextBlock] = []
  paragraph: list[str] = []
  in_code = False
  code_lines: list[str] = []

  def add(tag: str, text: str, classes: tuple[str, ...] = ()) -> None:
    index = 1 + sum(1 for block in blocks if block.tag == tag)
    blocks.append(TextBlock(source, f"/markdown/{tag}[{index}]", tag, classes, ("markdown", tag), (), None, text.strip()))

  def flush_paragraph() -> None:
    if paragraph:
      add("p", " ".join(paragraph))
      paragraph.clear()

  for line in content.splitlines() + [""]:
    if line.strip().startswith("```"):
      flush_paragraph()
      if in_code:
        add("code", "\n".join(code_lines))
        code_lines.clear()
      in_code = not in_code
      continue
    if in_code:
      code_lines.append(line)
      continue
    if match := re.match(r"^(#{1,6})\s+(.+)$", line):
      flush_paragraph()
      add(f"h{len(match.group(1))}", match.group(2))
    elif line.startswith(">"):
      flush_paragraph()
      add("blockquote", line.lstrip("> "))
    elif re.match(r"^\s*(?:[-+*]|\d+[.)])\s+", line):
      flush_paragraph()
      add("li", re.sub(r"^\s*(?:[-+*]|\d+[.)])\s+", "", line))
    elif not line.strip():
      flush_paragraph()
    else:
      paragraph.append(line.strip())
  return _with_neighbors(blocks)


def _features(text: str) -> dict[str, object]:
  cjk = latin = digits = punctuation = quotes = 0
  for char in text:
    cp = ord(char)
    if 0x3400 <= cp <= 0x9FFF or 0x20000 <= cp <= 0x3134F:
      cjk += 1
    elif char.isascii() and char.isalpha():
      latin += 1
    elif char.isdigit():
      digits += 1
    if unicodedata.category(char).startswith("P"):
      punctuation += 1
    if char in QUOTE_CHARS:
      quotes += 1
  visible = sum(not char.isspace() for char in text)
  return {
    "visible_chars": visible,
    "cjk_count": cjk,
    "latin_count": latin,
    "digit_count": digits,
    "punctuation_count": punctuation,
    "quote_count": quotes,
    "line_count": max(1, len(text.splitlines())),
    "cjk_ratio": round(cjk / visible, 4) if visible else 0.0,
    "latin_ratio": round(latin / visible, 4) if visible else 0.0,
  }


def _has_class(block: TextBlock, *patterns: str) -> bool:
  lowered = {value.lower() for value in block.classes}
  return any(any(pattern in value for value in lowered) for pattern in patterns)


def _role(block: TextBlock, features: dict[str, object]) -> tuple[str, list[str], str, bool, list[str]]:
  tag = block.tag
  text = block.text.strip()
  tags = set(block.ancestor_tags)
  types = {value.lower() for value in block.epub_types}

  explicit: tuple[str, str] | None = None
  if _has_class(block, "subtitle", "sub-title"):
    explicit = ("subtitle", "subtitle class")
  elif tag == "h1" and (_has_class(block, "book-title", "main-title", "title-page") or "titlepage" in types):
    explicit = ("title", "title-page heading structure")
  elif tag in {"h1", "h2", "h3", "h4", "h5", "h6"}:
    explicit = ("heading", "heading element")
  elif tag in {"figcaption", "caption"}:
    explicit = ("caption", f"{tag} element")
  elif tag == "hr" or _has_class(block, "scene-break", "separator"):
    explicit = ("scene-break", "scene-break structure")
  elif tag in {"code", "pre"}:
    explicit = ("code", f"{tag} element")
  elif tag in {"li", "dt", "dd"}:
    explicit = ("list", f"{tag} element")
  elif types.intersection({"footnote", "endnote", "rearnote", "note"}) or _has_class(block, "footnote", "endnote", "duokan-note"):
    explicit = ("note", "note semantic ancestor")
  elif _has_class(block, "epigraph"):
    explicit = ("epigraph", "epigraph class")
  elif _has_class(block, "poem", "verse", "stanza", "poetry"):
    explicit = ("verse", "verse/poem class")
  elif _has_class(block, "letter", "correspondence") or "letter" in types:
    explicit = ("letter", "letter semantic ancestor")
  elif _has_class(block, "classical-text", "classical", "original-text"):
    explicit = ("classical", "classical/original class")
  elif _has_class(block, "modern-text", "translation", "translated-text"):
    explicit = ("modern-translation", "modern/translation class")
  elif tag == "blockquote" or "blockquote" in tags or _has_class(block, "quotation", "quote"):
    explicit = ("quotation", "blockquote/quotation structure")
  elif _has_class(block, "dialogue", "speech"):
    explicit = ("dialogue", "dialogue class")
  if explicit:
    return explicit[0], [explicit[0]], "high", False, [explicit[1]]

  if CHAPTER_RE.match(text):
    return "heading", ["heading", "body"], "medium", True, ["chapter-like opening without heading markup"]
  if text and text[0] in QUOTE_CHARS and int(features["quote_count"]) >= 2:
    return "dialogue", ["dialogue", "quotation", "body"], "medium", True, ["quoted paragraph content"]
  if text.startswith(("——", "—")) and int(features["visible_chars"]) <= 200:
    return "dialogue", ["dialogue", "body"], "medium", True, ["dash-led paragraph resembles dialogue"]
  if SCENE_RE.match(text):
    return "scene-break", ["scene-break"], "medium", True, ["separator-like punctuation-only paragraph"]
  visible = int(features["visible_chars"])
  cjk = int(features["cjk_count"])
  if int(features["line_count"]) >= 2 and all(len(line.strip()) <= 24 for line in text.splitlines() if line.strip()):
    return "verse", ["verse", "body"], "medium", True, ["multiple short lines resemble verse"]
  if 2 <= cjk <= 14 and not any(char in SENTENCE_END for char in text):
    return "unknown", ["unknown", "body", "verse", "subtitle"], "low", True, ["short CJK paragraph is structurally ambiguous"]
  if cjk >= 15 and float(features["cjk_ratio"]) >= 0.8 and int(features["punctuation_count"]) == 0:
    return "unknown", ["unknown", "classical", "body", "verse"], "low", True, ["unpunctuated CJK prose may be classical text"]
  if visible:
    return "body", ["body"], "medium", False, ["ordinary paragraph-like content"]
  return "unknown", ["unknown"], "low", True, ["no visible text"]


TYPOGRAPHY: dict[str, dict[str, object]] = {
  "title": {"font_role": "ht", "line_height": "1.2", "text_indent": "0", "text_align": "center", "spacing": "display", "pagination": "avoid-after"},
  "heading": {"font_role": "ht", "line_height": "1.3", "text_indent": "0", "text_align": "inherit", "spacing": "heading", "pagination": "avoid-after"},
  "subtitle": {"font_role": "kt", "line_height": "1.4", "text_indent": "0", "text_align": "center", "spacing": "compact", "pagination": "avoid-after"},
  "body": {"font_role": "inherit", "line_height": "1.7", "text_indent": "2em", "text_align": "justify", "spacing": "body", "pagination": "auto"},
  "dialogue": {"font_role": "inherit", "line_height": "inherit", "text_indent": "2em", "text_align": "inherit", "spacing": "body", "pagination": "auto"},
  "quotation": {"font_role": "kt", "line_height": "1.7", "text_indent": "0", "text_align": "inherit", "spacing": "extract", "pagination": "auto"},
  "epigraph": {"font_role": "kt", "line_height": "1.6", "text_indent": "0", "text_align": "inherit", "spacing": "epigraph", "pagination": "avoid-inside"},
  "verse": {"font_role": "kt", "line_height": "1.7", "text_indent": "0", "text_align": "left", "spacing": "verse", "pagination": "auto"},
  "letter": {"font_role": "fs", "line_height": "1.7", "text_indent": "0", "text_align": "left", "spacing": "letter", "pagination": "auto"},
  "list": {"font_role": "inherit", "line_height": "1.6", "text_indent": "0", "text_align": "left", "spacing": "list", "pagination": "auto"},
  "caption": {"font_role": "inherit", "line_height": "1.5", "text_indent": "0", "text_align": "center", "spacing": "caption", "pagination": "avoid-before"},
  "note": {"font_role": "inherit", "line_height": "1.5", "text_indent": "0", "text_align": "left", "spacing": "note", "pagination": "auto"},
  "code": {"font_role": "mono", "line_height": "1.45", "text_indent": "0", "text_align": "left", "spacing": "code", "pagination": "auto"},
  "classical": {"font_role": "st", "line_height": "1.8", "text_indent": "2em", "text_align": "justify", "spacing": "classical", "pagination": "auto"},
  "modern-translation": {"font_role": "kt", "line_height": "1.7", "text_indent": "2em", "text_align": "justify", "spacing": "translation", "pagination": "auto"},
  "scene-break": {"font_role": "inherit", "line_height": "normal", "text_indent": "0", "text_align": "center", "spacing": "scene-break", "pagination": "avoid-inside"},
  "unknown": {"font_role": "inherit", "line_height": "preserve", "text_indent": "preserve", "text_align": "preserve", "spacing": "preserve", "pagination": "preserve"},
}


def _public_block(block: TextBlock, include_snippets: bool = False) -> dict[str, object]:
  features = _features(block.text)
  primary, candidates, confidence, review, evidence = _role(block, features)
  result: dict[str, object] = {
    "source": block.source,
    "locator": block.locator,
    "tag": block.tag,
    "classes": list(block.classes),
    "language": block.language,
    "previous_tag": block.previous_tag,
    "next_tag": block.next_tag,
    "text_sha256": hashlib.sha256(block.text.encode("utf-8")).hexdigest(),
    "features": features,
    "primary_role": primary,
    "candidate_roles": candidates,
    "confidence": confidence,
    "review_required": review,
    "evidence": evidence,
    "typography": dict(TYPOGRAPHY[primary]),
  }
  if include_snippets:
    result["snippet"] = block.text[:160]
  return result


def analyze_xhtml(source: str, content: str, include_snippets: bool = False) -> list[dict[str, object]]:
  return [_public_block(block, include_snippets) for block in _extract_xhtml_blocks(source, content)]


def analyze_source(source: str, content: str, include_snippets: bool = False) -> list[dict[str, object]]:
  suffix = Path(source).suffix.lower()
  if suffix in {".xhtml", ".xml"}:
    blocks = _extract_xhtml_blocks(source, content)
  elif suffix in {".html", ".htm"}:
    blocks = _extract_html_blocks(source, content)
  elif suffix in {".md", ".markdown"}:
    blocks = _markdown_blocks(source, content)
  elif suffix == ".txt" or not suffix:
    blocks = _plain_blocks(source, content)
  else:
    raise ContentAnalysisError(f"unsupported source type: {suffix or '<none>'}")
  return [_public_block(block, include_snippets) for block in blocks]


def _report(
  source: str,
  blocks: Iterable[dict[str, object]],
  errors: Iterable[dict[str, str]] = (),
) -> dict[str, object]:
  values = list(blocks)
  file_errors = list(errors)
  roles = Counter(str(block["primary_role"]) for block in values)
  review_count = sum(bool(block["review_required"]) for block in values)
  status = "fail" if file_errors and not values else "warn" if review_count or file_errors else "pass"
  return {
    "schema_version": "1.0",
    "capability": CAPABILITY,
    "input": source,
    "status": status,
    "summary": {
      "blocks": len(values),
      "review_required": review_count,
      "file_errors": len(file_errors),
      "roles": dict(sorted(roles.items())),
    },
    "errors": file_errors,
    "blocks": values,
  }


def analyze_source_report(source: str, content: str, include_snippets: bool = False) -> dict[str, object]:
  return _report(source, analyze_source(source, content, include_snippets))


def _epub_spine_documents(path: Path) -> tuple[list[tuple[str, str]], list[dict[str, str]]]:
  files, _ = read_epub_files(path)
  if "META-INF/encryption.xml" in files:
    raise ContentAnalysisError("encryption marker detected; content analysis stopped")
  try:
    opf_path = opf_path_from_container(files)
    root = parse_xml(files[opf_path], opf_path)
  except (ET.ParseError, KeyError, OSError) as exc:
    raise ContentAnalysisError(f"cannot read EPUB package: {exc}") from exc
  opf_dir = posixpath.dirname(opf_path)
  by_id = {
    item.attrib.get("id", ""): norm_join(opf_dir, item.attrib.get("href", ""))
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("id") and item.attrib.get("href")
  }
  documents: list[tuple[str, str]] = []
  errors: list[dict[str, str]] = []
  for itemref in root.findall("opf:spine/opf:itemref", OPF_NS):
    target = by_id.get(itemref.attrib.get("idref", ""))
    if not target or target not in files:
      continue
    try:
      documents.append((target, files[target].decode("utf-8")))
    except UnicodeDecodeError:
      errors.append({"source": target, "message": "text is not valid UTF-8"})
  if not documents and not errors:
    raise ContentAnalysisError("EPUB spine contains no readable XHTML documents")
  return documents, errors


def analyze_path(path: Path, include_snippets: bool = False) -> dict[str, object]:
  path = path.resolve()
  if not path.is_file():
    raise ContentAnalysisError(f"input does not exist: {path}")
  if path.suffix.lower() == ".epub":
    blocks: list[dict[str, object]] = []
    documents, errors = _epub_spine_documents(path)
    for source, content in documents:
      try:
        blocks.extend(analyze_xhtml(source, content, include_snippets))
      except ContentAnalysisError as exc:
        errors.append({"source": source, "message": str(exc)})
    return _report(str(path), blocks, errors)
  try:
    content = path.read_text(encoding="utf-8")
  except UnicodeDecodeError as exc:
    raise ContentAnalysisError(f"{path}: source is not valid UTF-8") from exc
  return analyze_source_report(str(path), content, include_snippets)


def render_markdown(report: dict[str, object]) -> str:
  summary = report["summary"]
  lines = [
    "# EPUB 文本内容分析",
    "",
    f"- 输入：`{report['input']}`",
    f"- 状态：`{report['status']}`",
    f"- 文本块：`{summary['blocks']}`",
    f"- 待复核：`{summary['review_required']}`",
    "",
    "## 结构角色",
    "",
  ]
  for block in report["blocks"]:
    lines.append(f"### {block['source']} · {block['locator']}")
    lines.append("")
    lines.append(f"- 角色：`{block['primary_role']}`（{block['confidence']}）")
    lines.append(f"- 字体角色：`{block['typography']['font_role']}`")
    lines.append(f"- 行高建议：`{block['typography']['line_height']}`")
    lines.append(f"- 待复核：`{str(block['review_required']).lower()}`")
    if snippet := block.get("snippet"):
      lines.append(f"- 片段：{str(snippet).replace(chr(10), ' ')}")
    lines.append("")
  return "\n".join(lines).rstrip() + "\n"


def dumps_json(report: dict[str, object]) -> str:
  return json.dumps(report, ensure_ascii=False, indent=2) + "\n"
