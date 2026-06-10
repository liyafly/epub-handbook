#!/usr/bin/env python3
"""Suggest image layout candidates without modifying an EPUB."""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_lib import (
  EPUB_NS,
  EpubLibError,
  OPF_NS,
  XHTML_URI,
  local_name,
  norm_join,
  opf_path_from_container,
  parse_xml,
  read_epub_files,
  split_props,
)


STYLE_DECL_RE = re.compile(r"(?P<name>[-\w]+)\s*:\s*(?P<value>[^;]+)")
CSS_RULE_RE = re.compile(r"(?P<selectors>[^{}]+)\{(?P<body>[^{}]*)\}", re.S)
CSS_COMMENT_RE = re.compile(r"/\*.*?\*/", re.S)
CLASS_RE = re.compile(r"\.([A-Za-z_][\w-]*)")
PERCENT_RE = re.compile(r"^\s*(\d+(?:\.\d+)?)%")
ALITE_BODY_CLASSES = {"fullpage", "poster-bg"}
PREPAGINATED_PROPS = {"rendition:layout-pre-paginated"}


CANDIDATES: dict[str, list[dict[str, str]]] = {
  "lone-image-no-figure": [
    {
      "id": "figure.img-left",
      "summary": "左浮动 figure，宽度从 25%–35% 起步",
      "risk": "SPEC §5.6；短段落环绕会塌，见 demo 17-image-layout 反例。",
    },
    {
      "id": "figure.img-right",
      "summary": "右浮动 figure，宽度从 25%–35% 起步",
      "risk": "SPEC §5.6；reader-matrix 17-image-layout 当前仍需大字号复测。",
    },
    {
      "id": "figure-fullwidth",
      "summary": "通栏 figure，可附 figcaption",
      "risk": "SPEC §5.6；不使用 float，仍需目标阅读器人工确认。",
    },
  ],
  "caption-detached": [
    {
      "id": "figure.figcaption",
      "summary": "把图片与短图注并入同一 figure/figcaption",
      "risk": "SPEC §5.6；figure 与可选 figcaption 是通用路径。",
    },
    {
      "id": "keep-separate-caption",
      "summary": "保留独立段落，但显式标记其非图注角色",
      "risk": "未实测，见 reader-matrix 待验证项；需人工确认该短段确为正文。",
    },
  ],
  "float-width-risk": [
    {
      "id": "figure.img-left",
      "summary": "float 与 25%–35% 宽度放到左浮动 figure",
      "risk": "SPEC §5.6；内层 img 使用 width:100%; height:auto。",
    },
    {
      "id": "figure.img-right",
      "summary": "float 与 25%–35% 宽度放到右浮动 figure",
      "risk": "SPEC §5.6；reader-matrix 17-image-layout 仍为 warn。",
    },
    {
      "id": "figure-fullwidth",
      "summary": "取消环绕，改为普通通栏 figure",
      "risk": "SPEC §5.6；正文过短或大字号时更保守。",
    },
  ],
  "missing-alt": [
    {
      "id": "add-alt-text",
      "summary": "人工填写表达图片信息的 alt",
      "risk": "未实测，见 reader-matrix 待验证项；工具不得猜测图片内容。",
    },
    {
      "id": "decorative-empty-alt",
      "summary": "确认纯装饰后使用空 alt",
      "risk": "未实测，见 reader-matrix 待验证项；只有人工确认装饰角色后可选。",
    },
  ],
  "chapter-head-image-candidate": [
    {
      "id": "keep-current",
      "summary": "维持当前普通图片结构",
      "risk": "SPEC §5.11；仍需确认图片不会抢占章节标题语义。",
    },
    {
      "id": "chapter-head-art",
      "summary": "转为 chapter-head-art 图片槽位",
      "risk": "SPEC §5.11；reader-matrix 20-chapter-head-image 当前为 warn。",
    },
  ],
  "fullpage-image-alite-candidate": [
    {
      "id": "alite-contain",
      "summary": "A-lite contain，保留整张图与留白",
      "risk": "SPEC §2；reader-matrix 03c-poster-contain 的 Kindle Previewer 3.104 转换为 0 errors，GUI 仍待复测。",
    },
    {
      "id": "alite-fullbleed",
      "summary": "A-lite fullbleed，允许按视口裁切",
      "risk": "SPEC §2；reader-matrix 03b-poster-fullbleed 标记为未实测。",
    },
    {
      "id": "figure-fullwidth",
      "summary": "保留普通可重排整页 figure",
      "risk": "SPEC §5.6；不启用 A-lite，分页效果需目标阅读器确认。",
    },
  ],
}


def class_tokens(elem: ET.Element) -> set[str]:
  return set(elem.attrib.get("class", "").split())


def declarations(value: str) -> dict[str, str]:
  return {
    match.group("name").lower(): match.group("value").strip().lower().replace("!important", "").strip()
    for match in STYLE_DECL_RE.finditer(value)
  }


def css_class_declarations(files: dict[str, bytes]) -> dict[tuple[str, str], dict[str, str]]:
  rules: dict[tuple[str, str], dict[str, str]] = {}
  for path, data in files.items():
    if not path.lower().endswith(".css"):
      continue
    css = CSS_COMMENT_RE.sub("", data.decode("utf-8", errors="ignore"))
    for match in CSS_RULE_RE.finditer(css):
      props = declarations(match.group("body"))
      for selector in match.group("selectors").split(","):
        selector = selector.strip()
        if " " in selector or ">" in selector or "+" in selector or "~" in selector:
          continue
        classes = CLASS_RE.findall(selector)
        if len(classes) != 1:
          continue
        tag = selector.split(".", 1)[0].strip().lower() or "*"
        rules.setdefault((tag, classes[0]), {}).update(props)
  return rules


def element_style(elem: ET.Element, css_rules: dict[tuple[str, str], dict[str, str]]) -> dict[str, str]:
  tag = local_name(elem.tag).lower()
  style: dict[str, str] = {}
  for class_name in class_tokens(elem):
    style.update(css_rules.get(("*", class_name), {}))
    style.update(css_rules.get((tag, class_name), {}))
  style.update(declarations(elem.attrib.get("style", "")))
  return style


def percentage(value: str | None) -> float | None:
  if value is None:
    return None
  match = PERCENT_RE.match(value)
  return float(match.group(1)) if match else None


def visible_text(elem: ET.Element) -> str:
  return re.sub(r"\s+", "", "".join(elem.itertext()))


def selector_for(elem: ET.Element, parent_map: dict[ET.Element, ET.Element], body: ET.Element) -> str:
  segments: list[str] = []
  current = elem
  while current is not body:
    parent = parent_map.get(current)
    if parent is None:
      break
    tag = local_name(current.tag)
    same = [child for child in list(parent) if local_name(child.tag) == tag]
    index = same.index(current) + 1
    segments.append(f"{tag}:nth-of-type({index})")
    current = parent
  return "body" + "".join(f" > {segment}" for segment in reversed(segments))


def nav_paths(files: dict[str, bytes], root: ET.Element, opf_dir: str) -> tuple[set[str], set[str]]:
  chapters: set[str] = set()
  covers: set[str] = set()
  nav_item = next(
    (
      item
      for item in root.findall("opf:manifest/opf:item", OPF_NS)
      if "nav" in split_props(item.attrib.get("properties"))
    ),
    None,
  )
  if nav_item is None or not nav_item.attrib.get("href"):
    return chapters, covers
  nav_path = norm_join(opf_dir, nav_item.attrib["href"])
  if nav_path not in files:
    return chapters, covers
  nav_root = parse_xml(files[nav_path], nav_path)
  nav_dir = posixpath.dirname(nav_path)
  epub_type = f"{{{EPUB_NS['epub']}}}type"
  for nav in nav_root.iter():
    if local_name(nav.tag) != "nav":
      continue
    types = set(nav.attrib.get(epub_type, "").split())
    for link in nav.iter():
      if local_name(link.tag) != "a" or not link.attrib.get("href"):
        continue
      target = norm_join(nav_dir, link.attrib["href"])
      if "toc" in types:
        chapters.add(target)
      if "landmarks" in types and "cover" in set(link.attrib.get(epub_type, "").split()):
        covers.add(target)
  return chapters, covers


def candidate_list(kind: str) -> list[dict[str, str]]:
  return [dict(candidate) for candidate in CANDIDATES[kind]]


def finding(kind: str, file: str, selector: str, image: str) -> dict[str, object]:
  return {
    "scene": "image-layout",
    "finding": kind,
    "file": file,
    "selector": selector,
    "image": image,
    "candidates": candidate_list(kind),
  }


def analyze_epub(path: Path) -> dict[str, object]:
  files, _ = read_epub_files(path)
  opf_path = opf_path_from_container(files)
  root = parse_xml(files[opf_path], opf_path)
  opf_dir = posixpath.dirname(opf_path)
  items = {
    item.attrib.get("id", ""): item
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
  }
  chapter_paths, cover_paths = nav_paths(files, root, opf_dir)
  css_rules = css_class_declarations(files)
  results: list[dict[str, object]] = []
  warnings: list[str] = []

  for itemref in root.findall("opf:spine/opf:itemref", OPF_NS):
    item = items.get(itemref.attrib.get("idref", ""))
    if item is None or not item.attrib.get("href"):
      continue
    if item.attrib.get("media-type") != "application/xhtml+xml":
      continue
    xhtml_path = norm_join(opf_dir, item.attrib["href"])
    if xhtml_path not in files:
      warnings.append(f"spine XHTML missing: {xhtml_path}")
      continue
    try:
      xhtml_root = parse_xml(files[xhtml_path], xhtml_path)
    except ET.ParseError as exc:
      warnings.append(str(exc))
      continue
    body = next((elem for elem in xhtml_root.iter() if local_name(elem.tag) == "body"), None)
    if body is None:
      warnings.append(f"body missing: {xhtml_path}")
      continue
    parent_map = {child: parent for parent in xhtml_root.iter() for child in list(parent)}
    images = [elem for elem in body.iter() if local_name(elem.tag) == "img"]
    body_children = [child for child in list(body) if isinstance(child.tag, str)]
    body_classes = class_tokens(body)
    fixed_layout = bool(
      (set(split_props(itemref.attrib.get("properties"))) | set(split_props(item.attrib.get("properties"))))
      & PREPAGINATED_PROPS
    )
    alite_page = bool(body_classes & ALITE_BODY_CLASSES)
    cover_page = xhtml_path in cover_paths
    first_child = body_children[0] if body_children else None
    first_images = (
      [elem for elem in first_child.iter() if local_name(elem.tag) == "img"]
      if first_child is not None and local_name(first_child.tag) in {"img", "figure"}
      else []
    )
    fullpage_candidate = (
      len(body_children) == 1
      and local_name(body_children[0].tag) in {"img", "figure"}
      and len(images) == 1
      and len(visible_text(body)) <= 20
    )

    for image_elem in images:
      parent = parent_map.get(image_elem)
      image_src = image_elem.attrib.get("src", "")
      image_path = norm_join(posixpath.dirname(xhtml_path), image_src) if image_src else ""
      image_selector = selector_for(image_elem, parent_map, body)

      if parent is None or local_name(parent.tag) != "figure":
        if not (cover_page or alite_page or fixed_layout):
          results.append(finding("lone-image-no-figure", xhtml_path, image_selector, image_path))

      if parent is not None:
        siblings = [child for child in list(parent) if isinstance(child.tag, str)]
        index = siblings.index(image_elem)
        if index + 1 < len(siblings):
          next_sibling = siblings[index + 1]
          if local_name(next_sibling.tag) == "p":
            sibling_classes = class_tokens(next_sibling)
            if len(visible_text(next_sibling)) <= 30 or sibling_classes & {"caption", "tu-zhu"}:
              results.append(finding("caption-detached", xhtml_path, image_selector, image_path))

      image_style = element_style(image_elem, css_rules)
      image_float = image_style.get("float") in {"left", "right"}
      image_width = percentage(image_style.get("width"))
      figure_elem = parent if parent is not None and local_name(parent.tag) == "figure" else None
      figure_style = element_style(figure_elem, css_rules) if figure_elem is not None else {}
      figure_floated = bool(
        figure_elem is not None
        and (
          figure_style.get("float") in {"left", "right"}
          or class_tokens(figure_elem) & {"img-left", "img-right"}
        )
      )
      figure_width = percentage(figure_style.get("width"))
      direct_percentage = image_width is not None and not (figure_floated and image_width == 100.0)
      figure_width_risk = figure_floated and (figure_width is None or not 25.0 <= figure_width <= 35.0)
      if image_float or direct_percentage or figure_width_risk:
        results.append(finding("float-width-risk", xhtml_path, image_selector, image_path))

      if "alt" not in image_elem.attrib:
        results.append(finding("missing-alt", xhtml_path, image_selector, image_path))

      if xhtml_path in chapter_paths and image_elem in first_images and not alite_page:
        results.append(finding("chapter-head-image-candidate", xhtml_path, image_selector, image_path))

      if fullpage_candidate:
        results.append(finding("fullpage-image-alite-candidate", xhtml_path, image_selector, image_path))

  return {
    "version": "1",
    "epub": str(path),
    "findings": results,
    "warnings": warnings,
  }


def render_markdown(report: dict[str, object]) -> str:
  lines = [
    "# EPUB Image Layout Advisor",
    "",
    f"- EPUB: `{report['epub']}`",
    f"- Findings: `{len(report['findings'])}`",
  ]
  grouped: dict[str, list[dict[str, object]]] = {}
  for item in report["findings"]:
    grouped.setdefault(str(item["file"]), []).append(item)
  for file, items in grouped.items():
    lines.extend(["", f"## `{file}`"])
    for item in items:
      candidate_ids = ",".join(str(candidate["id"]) for candidate in item["candidates"])
      lines.extend([
        "",
        f"### `{item['finding']}`",
        "",
        f"- Selector: `{item['selector']}`",
        f"- Image: `{item['image']}`",
        "",
        "| Candidate | Summary | Risk |",
        "| --- | --- | --- |",
      ])
      for candidate in item["candidates"]:
        lines.append(f"| `{candidate['id']}` | {candidate['summary']} | {candidate['risk']} |")
      lines.extend([
        "",
        "```sh",
        "uv run python scripts/epub_decision_log.py add \\",
        "  --file work/book/reports/decisions.json \\",
        "  --scene image-layout \\",
        f"  --finding {item['finding']} \\",
        f"  --context 'selector={item['selector']}' \\",
        f"  --candidates {candidate_ids} \\",
        "  --chosen CHOOSE_ONE \\",
        '  --rationale "WRITE_REASON" \\',
        "  --scope book \\",
        "  --source manual-review",
        "```",
      ])
  if report.get("warnings"):
    lines.extend(["", "## Warnings"])
    lines.extend(f"- {warning}" for warning in report["warnings"])
  return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
  parser = argparse.ArgumentParser(description="Suggest image layout candidates without modifying an EPUB")
  parser.add_argument("input", type=Path)
  parser.add_argument("--format", choices=("json", "md"), default="json")
  parser.add_argument("--report", type=Path, help="Also write the rendered report to this path")
  args = parser.parse_args(sys.argv[1:] if argv is None else argv)
  try:
    report = analyze_epub(args.input)
  except (OSError, ValueError, EpubLibError, ET.ParseError) as exc:
    parser.error(str(exc))
  output = (
    json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    if args.format == "json"
    else render_markdown(report)
  )
  if args.report:
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(output, encoding="utf-8")
  print(output, end="")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
