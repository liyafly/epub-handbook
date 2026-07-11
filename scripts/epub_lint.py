#!/usr/bin/env python3
"""Generic SPEC-rule linter for any EPUB (stdlib only).

Rules map directly to clauses in docs/final/SPEC-实现约束.md.
Complements scripts/validate_epub_style_demo.py (demo-fixture-only checks).
"""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from xml.etree import ElementTree as ET

OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
EPUB_TYPE = "{http://www.idpf.org/2007/ops}type"
MATH_TAG = "{http://www.w3.org/1998/Math/MathML}math"

# 同平台别名组（SPEC §8）；`*` 结尾按前缀匹配。
ALIAS_GROUPS: list[list[str]] = [
  ["songti sc", "stsongti-*"],
  ["simsun", "nsimsun", "宋体"],
  ["microsoft yahei", "微软雅黑"],
  ["kaiti", "楷体"],
  ["fangsong", "仿宋"],
  ["noto serif cjk sc", "source han serif sc", "思源宋体"],
  ["noto sans cjk sc", "source han sans sc", "思源黑体"],
]

CLASS_REQUIRED_ELEMENT_RE = re.compile(r"^(p|code|pre|blockquote|li|div|span)$")
BODY_FONT_LOCKED_RE = re.compile(
  r"<body[^>]*\bclass\s*=\s*(['\"])[^'\"]*\bbody-font-locked\b[^'\"]*\1", re.I
)
VH_VW_RE = re.compile(r"\b\d+(?:\.\d+)?(?:vh|vw)\b")
EPUB_NAMESPACE_USE_RE = re.compile(r"(?<![\w-])epub\|")
EPUB_NAMESPACE_URI_RE = re.escape("http://www.idpf.org/2007/ops")
CSS_STRING_TOKEN_RE = r'''(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')'''
CSS_URL_TOKEN_RE = rf"(?i:url)\(\s*(?:{CSS_STRING_TOKEN_RE}|[^)\s]+)\s*\)"
EPUB_NAMESPACE_DECL_RE = re.compile(
  rf'''(?i:@namespace)\s+epub\s+(?:
  "{EPUB_NAMESPACE_URI_RE}"|'{EPUB_NAMESPACE_URI_RE}'|
  (?i:url)\(\s*(?:"{EPUB_NAMESPACE_URI_RE}"|'{EPUB_NAMESPACE_URI_RE}'|{EPUB_NAMESPACE_URI_RE})\s*\)
  )\s*;''',
  re.X,
)
EPUB_NAMESPACE_ANY_DECL_RE = re.compile(
  rf"(?i:@namespace)\s+epub\s+(?:{CSS_STRING_TOKEN_RE}|{CSS_URL_TOKEN_RE})\s*;"
)
CSS_STRING_TOKEN_PATTERN = re.compile(CSS_STRING_TOKEN_RE)
CSS_CHARSET_RE = re.compile(r"@charset\s+(?:\"[^\"]*\"|'[^']*')\s*;", re.I)
CSS_IMPORT_RE = re.compile(r"@import\b[^;]*;", re.I)
CSS_NAMESPACE_RULE_RE = re.compile(r"@namespace\b[^;]*;", re.I)


@dataclass
class Finding:
  rule: str
  severity: str  # error | warn
  location: str
  detail: str


@dataclass
class Book:
  opf_path: str
  opf_root: ET.Element
  prefix_attr: str
  manifest: list[dict]
  css_texts: dict[str, str]      # href -> comment-stripped css
  xhtml_texts: dict[str, str]    # href -> raw text
  xhtml_roots: dict[str, ET.Element | None]


def strip_css_comments(css: str) -> str:
  return re.sub(r"/\*.*?\*/", "", css, flags=re.S)


def iter_css_rules(css: str):
  """Yield (selector, body). @media 内层规则也会被捕获；@media 头部行被忽略。"""
  for match in re.finditer(r"([^{}]+)\{([^{}]*)\}", css):
    selector = match.group(1).strip()
    selector = selector.split("{")[-1].strip()
    if selector.startswith("@media") or selector.startswith("@supports"):
      continue
    yield selector, match.group(2)


def has_valid_epub_namespace(css: str) -> bool:
  """Return whether a valid epub namespace occurs in CSS's leading at-rule prelude."""
  pos = 1 if css.startswith("\ufeff") else 0

  def skip_space(offset: int) -> int:
    match = re.match(r"\s*", css[offset:])
    return offset + (match.end() if match else 0)

  pos = skip_space(pos)
  charset = CSS_CHARSET_RE.match(css, pos)
  if charset:
    pos = skip_space(charset.end())

  while True:
    imported = CSS_IMPORT_RE.match(css, pos)
    if not imported:
      break
    pos = skip_space(imported.end())

  found = False
  while True:
    namespace = CSS_NAMESPACE_RULE_RE.match(css, pos)
    if not namespace:
      break
    statement = namespace.group(0)
    if EPUB_NAMESPACE_ANY_DECL_RE.fullmatch(statement):
      found = EPUB_NAMESPACE_DECL_RE.fullmatch(statement) is not None
    pos = skip_space(namespace.end())
  return found


def uses_epub_namespace_selector(css: str) -> bool:
  return any(
    EPUB_NAMESPACE_USE_RE.search(
      CSS_STRING_TOKEN_PATTERN.sub("", selector.rsplit(";", 1)[-1])
    )
    for selector, _body in iter_css_rules(css)
  )


def split_font_chain(value: str) -> list[str]:
  parts = re.findall(r'"[^"]*"|\'[^\']*\'|[^,]+', value)
  return [p.strip().strip("\"'").strip() for p in parts if p.strip().strip("\"'").strip()]


def font_family_decls(rule_body: str) -> list[str]:
  return [
    m.group(1).strip().rstrip(";").strip()
    for m in re.finditer(r"font-family\s*:\s*([^;}]+)", rule_body, re.I)
  ]


def has_direct_body_font_family(css_texts: dict[str, str]) -> bool:
  """Return whether an active rule binds font-family to the whole-book body role."""
  for css in css_texts.values():
    body_no_fontface = re.sub(r"@font-face\s*\{[^{}]*\}", "", css, flags=re.S | re.I)
    for selector, rule_body in iter_css_rules(body_no_fontface):
      if not font_family_decls(rule_body):
        continue
      if any(
        simple.strip().split(";")[-1].strip().lower() == "body"
        for simple in selector.split(",")
      ):
        return True
  return False


def load_book(path: Path) -> tuple[Book | None, list[Finding]]:
  findings: list[Finding] = []
  with zipfile.ZipFile(path) as zf:
    names = set(zf.namelist())
    container = ET.fromstring(zf.read("META-INF/container.xml"))
    rootfile = container.find("c:rootfiles/c:rootfile", CONTAINER_NS)
    opf_path = rootfile.attrib["full-path"] if rootfile is not None else None
    if not opf_path or opf_path not in names:
      candidates = [n for n in names if n.endswith(".opf")]
      if not candidates:
        return None, [Finding("L-FATAL", "error", "META-INF/container.xml", "无法定位 OPF")]
      opf_path = candidates[0]
    opf_root = ET.fromstring(zf.read(opf_path))
    opf_dir = posixpath.dirname(opf_path)

    manifest: list[dict] = []
    css_texts: dict[str, str] = {}
    xhtml_texts: dict[str, str] = {}
    xhtml_roots: dict[str, ET.Element | None] = {}
    for item in opf_root.findall("opf:manifest/opf:item", OPF_NS):
      href = item.attrib.get("href", "")
      zip_path = posixpath.normpath(posixpath.join(opf_dir, href)) if href else ""
      entry = {
        "id": item.attrib.get("id", ""),
        "href": href,
        "media_type": item.attrib.get("media-type", ""),
        "properties": (item.attrib.get("properties") or "").split(),
        "zip_path": zip_path,
      }
      manifest.append(entry)
      if zip_path not in names:
        continue
      raw = zf.read(zip_path)
      if entry["media_type"] == "text/css":
        css_texts[href] = strip_css_comments(raw.decode("utf-8", errors="replace"))
      elif entry["media_type"] == "application/xhtml+xml":
        text = raw.decode("utf-8", errors="replace")
        xhtml_texts[href] = text
        try:
          xhtml_roots[href] = ET.fromstring(raw)
        except ET.ParseError as exc:
          xhtml_roots[href] = None
          findings.append(
            Finding("L-X01", "warn", href, f"XHTML 解析失败，结构类规则跳过：{exc}")
          )

  book = Book(
    opf_path=opf_path,
    opf_root=opf_root,
    prefix_attr=opf_root.attrib.get("prefix", ""),
    manifest=manifest,
    css_texts=css_texts,
    xhtml_texts=xhtml_texts,
    xhtml_roots=xhtml_roots,
  )
  return book, findings


def fontface_families(css: str) -> dict[str, bool]:
  """family(lower) -> has_src，仅统计 @font-face 块。"""
  result: dict[str, bool] = {}
  for match in re.finditer(r"@font-face\s*\{([^{}]*)\}", css, re.S | re.I):
    body = match.group(1)
    fams = font_family_decls(body)
    has_src = re.search(r"\bsrc\s*:", body, re.I) is not None
    for fam in fams:
      for name in split_font_chain(fam):
        result[name.lower()] = result.get(name.lower(), False) or has_src
  return result


def alias_matches(group: list[str], family: str) -> bool:
  fam = family.lower()
  for pattern in group:
    if pattern.endswith("*"):
      if fam.startswith(pattern[:-1]):
        return True
    elif fam == pattern:
      return True
  return False


def check_css_rules(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  embedded = {
    fam
    for css in book.css_texts.values()
    for fam, has_src in fontface_families(css).items()
    if has_src
  }
  used_families: set[str] = set()

  for href, css in book.css_texts.items():
    if uses_epub_namespace_selector(css) and not has_valid_epub_namespace(css):
      findings.append(
        Finding(
          "L-C01",
          "error",
          href,
          (
            "CSS 使用 `epub|` 命名空间选择器但缺 "
            '`@namespace epub "http://www.idpf.org/2007/ops";`（SPEC §8）'
          ),
        )
      )
    body_no_fontface = re.sub(r"@font-face\s*\{[^{}]*\}", "", css, flags=re.S | re.I)
    for selector, rule_body in iter_css_rules(body_no_fontface):
      for decl in font_family_decls(rule_body):
        chain = split_font_chain(decl)
        used_families.update(f.lower() for f in chain)
        loc = f"{href} `{selector}`"
        if "inherit" in (f.lower() for f in chain):
          continue
        if len(chain) > 5:
          findings.append(
            Finding("L-F01", "error", loc, f"font-family 链 {len(chain)} 段（>5）：{decl}")
          )
        elif len(chain) > 4:
          findings.append(
            Finding(
              "L-F01",
              "warn",
              loc,
              f"font-family 链 {len(chain)} 段（SPEC §8 默认 ≤4）：{decl}",
            )
          )
        for group in ALIAS_GROUPS:
          hits = sorted({f for f in chain if alias_matches(group, f)})
          if len(hits) > 1:
            findings.append(Finding("L-F02", "error", loc, f"同平台别名堆叠：{hits}"))
        chain_embedded = [f for f in chain if f.lower() in embedded]
        if chain_embedded:
          for simple in selector.split(","):
            if CLASS_REQUIRED_ELEMENT_RE.fullmatch(simple.strip()):
              findings.append(
                Finding(
                  "L-F04",
                  "error",
                  loc,
                  f"嵌入字体 {chain_embedded} 出现在多用途裸元素 `{simple.strip()}`，须改用角色类（SPEC §8）",
                )
              )
      if re.search(r"text-decoration-style\s*:", rule_body, re.I) and not re.search(
        r"text-decoration(-line)?\s*:", rule_body, re.I
      ):
        findings.append(
          Finding(
            "L-T01",
            "error",
            f"{href} `{selector}`",
            "写了 text-decoration-style 但缺基础 text-decoration（SPEC §5.7）",
          )
        )
    if VH_VW_RE.search(css):
      findings.append(Finding("L-A01", "warn", href, "CSS 使用了 vh/vw 单位（SPEC §2 A-lite 禁用）"))

  for fam in sorted(embedded):
    if fam not in used_families:
      findings.append(
        Finding(
          "L-F03",
          "error",
          "fonts css",
          f"@font-face `{fam}` 有 src 但无任何规则引用（SPEC §8：删除或保持注释）",
        )
      )
  return findings


def check_font_lock_pairing(
  book: Book,
  *,
  allow_free_body_ibooks_meta: bool = False,
) -> list[Finding]:
  class_locked = sorted(h for h, t in book.xhtml_texts.items() if BODY_FONT_LOCKED_RE.search(t))
  direct_locked = has_direct_body_font_family(book.css_texts)
  has_meta = any(
    m.attrib.get("property") == "ibooks:specified-fonts"
    and (m.text or "").strip().lower() == "true"
    for m in book.opf_root.findall("opf:metadata/opf:meta", OPF_NS)
  )
  findings: list[Finding] = []
  if (class_locked or direct_locked) and not has_meta:
    findings.append(
      Finding(
        "L-F05",
        "error",
        book.opf_path,
        (
          f"存在正文字体锁定（直接 body 规则={direct_locked}，"
          f"body-font-locked 页={class_locked or 'none'}）但 OPF 缺 "
          "ibooks:specified-fonts=true（SPEC §8）"
        ),
      )
    )
  if (
    has_meta
    and not class_locked
    and not direct_locked
    and not allow_free_body_ibooks_meta
  ):
    findings.append(
      Finding(
        "L-F05",
        "error",
        book.opf_path,
        (
          "OPF 有 ibooks:specified-fonts=true 但既无直接 body 字体规则，也无锁定页"
          "（SPEC §8：局部嵌入字体不构成正文字体锁定；书级历史例外须显式豁免）"
        ),
      )
    )
  return findings


def check_opf(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  has_ibooks_meta = any(
    (m.attrib.get("property") or "").startswith("ibooks:")
    for m in book.opf_root.findall("opf:metadata/opf:meta", OPF_NS)
  )
  if has_ibooks_meta and "ibooks:" not in book.prefix_attr:
    findings.append(
      Finding(
        "L-O01",
        "error",
        book.opf_path,
        "存在 ibooks:* property 但 package 未声明 ibooks prefix（SPEC §3）",
      )
    )
  return findings


def check_documents(book: Book) -> list[Finding]:
  findings: list[Finding] = []
  props_by_href = {e["href"]: e["properties"] for e in book.manifest}
  for href, root in book.xhtml_roots.items():
    if root is None:
      continue
    ids: dict[str, ET.Element] = {}
    footnote_targets: set[str] = set()
    noterefs: list[ET.Element] = []
    has_math = False
    for el in root.iter():
      el_id = el.attrib.get("id")
      if el_id:
        ids[el_id] = el
      types = (el.attrib.get(EPUB_TYPE) or "").split()
      if "footnote" in types and el.tag.rsplit("}", 1)[-1] == "aside":
        footnote_targets.update(
          descendant.attrib["id"]
          for descendant in el.iter()
          if descendant.attrib.get("id")
        )
      if "noteref" in types:
        noterefs.append(el)
      if el.tag == MATH_TAG:
        has_math = True
    for ref in noterefs:
      target = ref.attrib.get("href", "")
      if not target.startswith("#"):
        findings.append(
          Finding(
            "L-N01",
            "error",
            href,
            f"noteref href `{target}` 不是同文件锚点（SPEC §1：本文件 aside 聚合）",
          )
        )
        continue
      frag = target[1:]
      if frag not in ids:
        findings.append(Finding("L-N01", "error", href, f"noteref 目标 #{frag} 不存在"))
      elif frag not in footnote_targets:
        findings.append(
          Finding(
            "L-N01",
            "error",
            href,
            f"noteref 目标 #{frag} 不是 aside[epub:type~=footnote]（SPEC §1）",
          )
        )
    if has_math and "mathml" not in props_by_href.get(href, []):
      findings.append(
        Finding(
          "L-M01",
          "error",
          href,
          '含 MathML 但 manifest item 缺 properties="mathml"（SPEC §5.8）',
        )
      )
  return findings


def lint_epub(
  path: Path,
  *,
  allow_free_body_ibooks_meta: bool = False,
) -> list[Finding]:
  book, findings = load_book(path)
  if book is None:
    return findings
  findings.extend(check_css_rules(book))
  findings.extend(
    check_font_lock_pairing(
      book,
      allow_free_body_ibooks_meta=allow_free_body_ibooks_meta,
    )
  )
  findings.extend(check_opf(book))
  findings.extend(check_documents(book))
  return findings


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(
    description="Lint any EPUB against docs/final/SPEC-实现约束.md mechanical rules"
  )
  parser.add_argument("epub", type=Path)
  parser.add_argument("--json", action="store_true", help="输出 JSON")
  parser.add_argument("--strict", action="store_true", help="warn 也按失败处理")
  parser.add_argument(
    "--allow-free-body-ibooks-meta",
    action="store_true",
    help=(
      "显式豁免自由正文保留 ibooks:specified-fonts=true 的书级历史例外；"
      "必须在书级报告记录理由"
    ),
  )
  args = parser.parse_args(argv)

  findings = lint_epub(
    args.epub,
    allow_free_body_ibooks_meta=args.allow_free_body_ibooks_meta,
  )
  errors = [f for f in findings if f.severity == "error"]
  warns = [f for f in findings if f.severity == "warn"]
  if args.json:
    print(json.dumps([f.__dict__ for f in findings], ensure_ascii=False, indent=2))
  else:
    for f in findings:
      print(f"[{f.severity}] {f.rule} {f.location}: {f.detail}")
    print(f"epub-lint: {len(errors)} error(s), {len(warns)} warning(s) in {args.epub}")
  if errors or (args.strict and warns):
    return 1
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
