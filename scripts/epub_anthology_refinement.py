#!/usr/bin/env python3
"""Refine anthology volume posters and adjacent copyright pages."""

from __future__ import annotations

import argparse
import html
import json
import posixpath
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_lib import (
  OPF_NS,
  OPF_URI,
  ensure_stylesheet_link,
  manifest,
  norm_join,
  opf_path_from_container,
  parse_xml,
  q,
  read_epub_files,
  rel_href,
  spine,
  unique_id,
  write_epub,
)


TITLE_RE = re.compile(r"<title\b[^>]*>(?P<title>.*?)</title>", re.I | re.S)
BODY_RE = re.compile(r"(?P<open><body\b[^>]*>)(?P<body>.*?)(?P<close></body\s*>)", re.I | re.S)
IMG_RE = re.compile(r"<img\b(?P<attrs>[^>]*)/?>", re.I | re.S)
SRC_RE = re.compile(r"\bsrc=(?P<quote>[\"'])(?P<src>[^\"']+)(?P=quote)", re.I)
TAG_RE = re.compile(r"<!--.*?-->|<[^>]+>", re.S)
CLASS_RE = re.compile(r"\bclass=(?P<quote>[\"'])(?P<classes>[^\"']*)(?P=quote)", re.I)


class RefinementError(Exception):
  """The anthology refinement cannot continue safely."""


@dataclass
class RefinementReport:
  input: str
  output: str
  opf: str = ""
  poster_pages_refined: int = 0
  copyright_pages_refined: int = 0
  stylesheets_added: int = 0
  poster_pages: list[str] = field(default_factory=list)
  copyright_pages: list[str] = field(default_factory=list)
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return {
      "harness": "epub_anthology_refinement",
      "input": self.input,
      "output": self.output,
      "opf": self.opf,
      "poster_pages_refined": self.poster_pages_refined,
      "copyright_pages_refined": self.copyright_pages_refined,
      "stylesheets_added": self.stylesheets_added,
      "poster_pages": self.poster_pages,
      "copyright_pages": self.copyright_pages,
      "warnings": self.warnings,
    }


def visible_text(value: str) -> str:
  return re.sub(r"\s+", "", html.unescape(TAG_RE.sub("", value)))


def title_text(value: str) -> str:
  match = TITLE_RE.search(value)
  return visible_text(match.group("title")) if match else ""


def class_tokens(attrs: str) -> list[str]:
  match = CLASS_RE.search(attrs)
  return match.group("classes").split() if match else []


def add_class_to_attrs(attrs: str, class_name: str) -> tuple[str, bool]:
  classes = class_tokens(attrs)
  if class_name in classes:
    return attrs, False
  classes.append(class_name)
  if CLASS_RE.search(attrs):
    return CLASS_RE.sub(f'class="{" ".join(classes)}"', attrs, count=1), True
  return f'{attrs} class="{class_name}"', True


def add_class_to_tag(value: str, tag: str, class_name: str, required_class: str | None = None) -> str:
  pattern = re.compile(rf"<{tag}\b(?P<attrs>[^>]*)>", re.I)

  def replace(match: re.Match[str]) -> str:
    attrs = match.group("attrs")
    if required_class and required_class not in class_tokens(attrs):
      return match.group(0)
    attrs, _ = add_class_to_attrs(attrs, class_name)
    return f"<{tag}{attrs}>"

  return pattern.sub(replace, value)


def poster_image_href(value: str) -> str | None:
  if title_text(value) != "封面":
    return None
  body = BODY_RE.search(value)
  if body is None or visible_text(body.group("body")):
    return None
  images = list(IMG_RE.finditer(body.group("body")))
  if len(images) != 1:
    return None
  source = SRC_RE.search(images[0].group("attrs"))
  return source.group("src") if source else None


def is_copyright_page(value: str) -> bool:
  if title_text(value) != "版权信息":
    return False
  body = BODY_RE.search(value)
  if body is None:
    return False
  return bool(re.search(r"<ul\b[^>]*\bclass=[\"'][^\"']*\blist\b", body.group("body"), re.I))


def refine_poster(value: str, volume: int, image_href: str, style_href: str) -> str:
  body = BODY_RE.search(value)
  if body is None:
    raise RefinementError("poster page missing body")
  attrs, _ = add_class_to_attrs(body.group("open")[len("<body"):-1], "fullpage")
  attrs, _ = add_class_to_attrs(attrs, "poster-bg")
  attrs, _ = add_class_to_attrs(attrs, f"poster-bg-volume-{volume:03d}")
  content = (
    f"\n  <section class=\"fullframe\" epub:type=\"chapter\">\n"
    f"    <img class=\"poster-fallback\" alt=\"\" src=\"{image_href}\"/>\n"
    f"  </section>\n"
  )
  value = BODY_RE.sub(f"<body{attrs}>{content}</body>", value, count=1)
  return ensure_stylesheet_link(value, style_href)[0]


def refine_copyright(value: str, style_href: str) -> str:
  body = BODY_RE.search(value)
  if body is None:
    raise RefinementError("copyright page missing body")
  attrs, _ = add_class_to_attrs(body.group("open")[len("<body"):-1], "anthology-copyright-page")
  content = body.group("body")
  content = add_class_to_tag(content, "p", "copyright-heading", required_class="cp")
  content = add_class_to_tag(content, "ul", "copyright-meta", required_class="list")
  content = add_class_to_tag(content, "li", "copyright-meta-item", required_class="i")
  if not re.search(r"\bclass=[\"'][^\"']*\bcopyright-card\b", content, re.I):
    content = f'\n  <section class="copyright-card" epub:type="frontmatter copyright-page">{content}\n  </section>\n'
  value = BODY_RE.sub(f"<body{attrs}>{content}</body>", value, count=1)
  return ensure_stylesheet_link(value, style_href)[0]


def stylesheet(poster_images: list[tuple[int, str]]) -> str:
  lines = [
    "/* Anthology volume poster and copyright refinement layer. */",
    "@page {",
    "  margin: 0;",
    "  padding: 0;",
    "}",
    "",
    "html {",
    "  width: 100%;",
    "  height: 100%;",
    "  min-height: 100%;",
    "}",
    "",
    "body.fullpage {",
    "  width: 100%;",
    "  height: 100%;",
    "  min-height: 100%;",
    "  margin: 0;",
    "  padding: 0;",
    "  -webkit-box-sizing: border-box;",
    "  box-sizing: border-box;",
    "  page-break-before: always;",
    "  page-break-after: always;",
    "  page-break-inside: avoid;",
    "  -webkit-page-break-before: always;",
    "  -webkit-page-break-after: always;",
    "  -webkit-page-break-inside: avoid;",
    "  overflow: hidden;",
    "}",
    "",
    "body.poster-bg {",
    "  background-repeat: no-repeat;",
    "  background-position: center center;",
    "  background-size: contain;",
    "}",
    "",
    ".fullframe {",
    "  width: 100%;",
    "  height: 100%;",
    "  min-height: 100%;",
    "  margin: 0;",
    "  padding: 0;",
    "  -webkit-box-sizing: border-box;",
    "  box-sizing: border-box;",
    "  overflow: visible;",
    "  page-break-inside: avoid;",
    "  -webkit-page-break-inside: avoid;",
    "}",
    "",
    ".poster-fallback {",
    "  display: block;",
    "  width: 100%;",
    "  max-width: 100%;",
    "  height: auto;",
    "  max-height: 100%;",
    "  margin: 0 auto;",
    "  page-break-inside: avoid;",
    "  -webkit-page-break-inside: avoid;",
    "}",
    "",
    "@supports (background-size: contain) {",
    "  body.poster-bg .poster-fallback {",
    "    visibility: hidden;",
    "  }",
    "}",
    "",
    "body.anthology-copyright-page {",
    "  max-width: 36em;",
    "  margin: 0 auto;",
    "  padding: 8% 6%;",
    "  -webkit-box-sizing: border-box;",
    "  box-sizing: border-box;",
    "}",
    "",
    ".anthology-copyright-page .copyright-card {",
    "  margin: 0 auto;",
    "}",
    "",
    ".anthology-copyright-page .copyright-heading {",
    "  margin: 0 0 1.2em;",
    "  text-indent: 0;",
    "  font-size: 1.25em;",
    "  font-weight: bold;",
    "}",
    "",
    ".anthology-copyright-page .copyright-meta {",
    "  margin: 0;",
    "  padding: 0;",
    "  list-style: none;",
    "}",
    "",
    ".anthology-copyright-page .copyright-meta-item {",
    "  margin: 0.38em 0;",
    "  padding: 0;",
    "  line-height: 1.55;",
    "  list-style: none;",
    "  text-indent: 0;",
    "}",
    "",
  ]
  for volume, href in poster_images:
    lines.extend([
      f"body.poster-bg-volume-{volume:03d} {{",
      f'  background-image: url("{href}");',
      "}",
      "",
    ])
  return "\n".join(lines).rstrip() + "\n"


def spine_xhtml_paths(root: ET.Element, opf_dir: str) -> list[str]:
  by_id = {
    item.attrib.get("id"): item
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("id")
  }
  paths: list[str] = []
  for itemref in spine(root).findall("opf:itemref", OPF_NS):
    item = by_id.get(itemref.attrib.get("idref"))
    if item is None or item.attrib.get("media-type") != "application/xhtml+xml":
      continue
    href = item.attrib.get("href")
    if href:
      paths.append(norm_join(opf_dir, href))
  return paths


def add_css_manifest_item(root: ET.Element, opf_dir: str, css_zip_path: str) -> bool:
  href = posixpath.relpath(css_zip_path, opf_dir) if opf_dir else css_zip_path
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    if item.attrib.get("href") == href:
      return False
  ET.SubElement(
    manifest(root),
    q(OPF_URI, "item"),
    {"id": unique_id(root, "anthology-refinement-css"), "href": href, "media-type": "text/css"},
  )
  return True


def refine_anthology(input_path: Path, output_path: Path, expect_volumes: int | None = None) -> RefinementReport:
  report = RefinementReport(input=str(input_path), output=str(output_path))
  files, original_order = read_epub_files(input_path)
  opf_path = opf_path_from_container(files)
  report.opf = opf_path
  root = parse_xml(files[opf_path], opf_path)
  opf_dir = posixpath.dirname(opf_path)
  css_zip_path = norm_join(opf_dir, "Styles/anthology-refinement.css")
  paths = spine_xhtml_paths(root, opf_dir)

  candidates: list[tuple[str, str, str | None]] = []
  for index, path in enumerate(paths):
    if path not in files:
      continue
    text = files[path].decode("utf-8", errors="replace")
    image_href = poster_image_href(text)
    if image_href is None:
      continue
    copyright_path = paths[index + 1] if index + 1 < len(paths) else None
    if copyright_path and copyright_path in files:
      copyright_text = files[copyright_path].decode("utf-8", errors="replace")
      if not is_copyright_page(copyright_text):
        copyright_path = None
    candidates.append((path, image_href, copyright_path))

  if expect_volumes is not None and len(candidates) != expect_volumes:
    raise RefinementError(f"expected {expect_volumes} volume poster pages, found {len(candidates)}")
  if not candidates:
    raise RefinementError("no single-image volume poster pages found")

  poster_images: list[tuple[int, str]] = []
  for volume, (poster_path, image_href, copyright_path) in enumerate(candidates, start=1):
    style_href = rel_href(poster_path, css_zip_path)
    text = files[poster_path].decode("utf-8", errors="replace")
    files[poster_path] = refine_poster(text, volume, image_href, style_href).encode("utf-8")
    report.poster_pages.append(poster_path)
    image_zip_path = norm_join(posixpath.dirname(poster_path), image_href)
    poster_images.append((volume, rel_href(css_zip_path, image_zip_path)))
    if copyright_path:
      copyright_style_href = rel_href(copyright_path, css_zip_path)
      copyright_text = files[copyright_path].decode("utf-8", errors="replace")
      files[copyright_path] = refine_copyright(copyright_text, copyright_style_href).encode("utf-8")
      report.copyright_pages.append(copyright_path)
    else:
      report.warnings.append(f"poster page has no adjacent copyright page: {poster_path}")

  report.poster_pages_refined = len(report.poster_pages)
  report.copyright_pages_refined = len(report.copyright_pages)
  files[css_zip_path] = stylesheet(poster_images).encode("utf-8")
  report.stylesheets_added += int(add_css_manifest_item(root, opf_dir, css_zip_path))
  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, files, original_order)
  return report


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Refine anthology volume posters and adjacent copyright pages")
  parser.add_argument("input", type=Path, help="Input EPUB")
  parser.add_argument("--output", type=Path, required=True, help="Output EPUB")
  parser.add_argument("--expect-volumes", type=int, help="Fail unless this many volume poster pages are found")
  parser.add_argument("--format", choices=("json", "text"), default="text")
  args = parser.parse_args(argv)
  try:
    report = refine_anthology(args.input, args.output, expect_volumes=args.expect_volumes)
  except RefinementError as exc:
    if args.format == "json":
      print(json.dumps({"harness": "epub_anthology_refinement", "error": str(exc)}, ensure_ascii=False, indent=2))
    else:
      print(f"ERROR: {exc}", file=sys.stderr)
    return 1
  if args.format == "json":
    print(json.dumps(report.as_dict(), ensure_ascii=False, indent=2))
  else:
    print(f"Wrote refined EPUB: {args.output}")
    print(f"Poster pages refined: {report.poster_pages_refined}")
    print(f"Copyright pages refined: {report.copyright_pages_refined}")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
