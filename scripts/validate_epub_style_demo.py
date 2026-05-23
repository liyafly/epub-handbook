#!/usr/bin/env python3
"""Validate the epub-style-demo fixture with only Python stdlib."""

from __future__ import annotations

import argparse
import posixpath
import re
import sys
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
DEMO = ROOT / "templates" / "epub-style-demo"
OEBPS = DEMO / "OEBPS"
PACKAGE = OEBPS / "package.opf"
NAV = OEBPS / "nav.xhtml"
NCX = OEBPS / "toc.ncx"
MEDIA_CSS = OEBPS / "Styles" / "media.css"
FONTS_CSS = OEBPS / "Styles" / "fonts.css"
IMAGE_LAYOUT = OEBPS / "Text" / "17-image-layout.xhtml"
ENGLISH_PAGE = OEBPS / "Text" / "18-english-fiction.xhtml"
NOTE_BOXES_PAGE = OEBPS / "Text" / "19-border-shadow-notes.xhtml"
MATH_PAGE = OEBPS / "Text" / "16-math.xhtml"

OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
XHTML_NS = {"xhtml": "http://www.w3.org/1999/xhtml"}
NCX_NS = {"ncx": "http://www.daisy.org/z3986/2005/ncx/"}
MATHML_URI = "http://www.w3.org/1998/Math/MathML"


class Check:
  def __init__(self) -> None:
    self.errors: list[str] = []

  def require(self, condition: bool, message: str) -> None:
    if not condition:
      self.errors.append(message)

  def fail(self, message: str) -> None:
    self.errors.append(message)


def parse_xml(path: Path, check: Check) -> ET.Element | None:
  try:
    return ET.parse(path).getroot()
  except ET.ParseError as exc:
    check.fail(f"XML parse failed: {path}: {exc}")
    return None


def split_props(value: str | None) -> set[str]:
  return set((value or "").split())


def manifest_map(package_root: ET.Element) -> dict[str, ET.Element]:
  return {
    item.attrib["id"]: item
    for item in package_root.findall("opf:manifest/opf:item", OPF_NS)
    if "id" in item.attrib
  }


def href_path(href: str) -> Path:
  href = href.split("#", 1)[0]
  return OEBPS / href


def has_mathml(path: Path, check: Check) -> bool:
  root = parse_xml(path, check)
  if root is None:
    return False
  return any(elem.tag.startswith("{" + MATHML_URI + "}") for elem in root.iter())


def selector_block(css: str, selector: str) -> str | None:
  match = re.search(rf"{re.escape(selector)}\s*\{{(?P<body>[^}}]+)\}}", css, re.S)
  return match.group("body") if match else None


def percentage_width(css: str, selector: str) -> float | None:
  block = selector_block(css, selector)
  if block is None:
    return None
  match = re.search(r"width\s*:\s*(?P<value>[0-9]+(?:\.[0-9]+)?)%\s*;", block)
  return float(match.group("value")) if match else None


def strip_css_comments(css: str) -> str:
  return re.sub(r"/\*.*?\*/", "", css, flags=re.S)


def validate_source(check: Check) -> None:
  package_root = parse_xml(PACKAGE, check)
  if package_root is None:
    return

  manifest = manifest_map(package_root)
  href_to_item = {item.attrib.get("href"): item for item in manifest.values()}

  nav_items = [
    item for item in manifest.values()
    if "nav" in split_props(item.attrib.get("properties"))
  ]
  check.require(len(nav_items) == 1, "OPF manifest must contain exactly one nav item")
  check.require("ncx" in manifest, "OPF manifest must contain toc.ncx item id=ncx")

  for item in manifest.values():
    href = item.attrib.get("href")
    if not href:
      check.fail(f"Manifest item {item.attrib.get('id', '<missing id>')} has no href")
      continue
    check.require(href_path(href).exists(), f"Manifest href missing on disk: {href}")

  for itemref in package_root.findall("opf:spine/opf:itemref", OPF_NS):
    idref = itemref.attrib.get("idref")
    check.require(idref in manifest, f"Spine idref missing from manifest: {idref}")

  for href, item in href_to_item.items():
    if not href or not href.endswith(".xhtml"):
      continue
    path = href_path(href)
    if path.exists() and has_mathml(path, check):
      props = split_props(item.attrib.get("properties"))
      check.require("mathml" in props, f"MathML content missing OPF properties=mathml: {href}")

  check.require(
    href_to_item.get("Text/16-math.xhtml") is not None,
    "16-math.xhtml must be in manifest",
  )
  math_item = href_to_item.get("Text/16-math.xhtml")
  if math_item is not None:
    check.require(
      "mathml" in split_props(math_item.attrib.get("properties")),
      "16-math.xhtml manifest item must declare properties=mathml",
    )

  nav_root = parse_xml(NAV, check)
  if nav_root is not None:
    for link in nav_root.findall(".//xhtml:a", XHTML_NS):
      href = link.attrib.get("href")
      if href and not href.startswith("#"):
        check.require(href_path(href).exists(), f"nav.xhtml link missing: {href}")

  ncx_root = parse_xml(NCX, check)
  if ncx_root is not None:
    for content in ncx_root.findall(".//ncx:content", NCX_NS):
      src = content.attrib.get("src")
      if src:
        check.require(href_path(src).exists(), f"toc.ncx content missing: {src}")

  fonts_css = FONTS_CSS.read_text(encoding="utf-8")
  active_fonts_css = strip_css_comments(fonts_css)
  check.require(
    "../Fonts/" not in active_fonts_css,
    "fonts.css default @font-face skeleton leaked an active missing font URL",
  )

  media_css = MEDIA_CSS.read_text(encoding="utf-8")
  image_layout = IMAGE_LAYOUT.read_text(encoding="utf-8")
  check.require("kindle-img" not in media_css, "media.css must not define direct img kindle-* float classes")
  check.require("kindle-img" not in image_layout, "17-image-layout must not use direct img kindle-* float classes")
  for selector in (".img-left", ".img-right"):
    width = percentage_width(media_css, selector)
    check.require(width is not None, f"{selector} must define percentage width")
    if width is not None:
      check.require(
        25 <= width <= 35,
        f"{selector} width must stay in the 25%-35% range, found {width:g}%",
      )
  check.require("aspect-ratio" not in media_css, "media.css must not depend on aspect-ratio for image wrapping")
  check.require("class=\"img-left\"" in image_layout, "17-image-layout must include figure.img-left")
  check.require("class=\"img-right\"" in image_layout, "17-image-layout must include figure.img-right")
  check.require("短段反例" in image_layout, "17-image-layout must include a short-text threshold counterexample")
  check.require("大字号 figure 回归" in image_layout, "17-image-layout must include large-font figure regression")

  math_text = MATH_PAGE.read_text(encoding="utf-8")
  for token in [
    "<mfrac", "<msqrt", "<mroot", "<msub", "<msup", "<msubsup",
    "<mover", "<munder", "<munderover", "<menclose", "<mfenced",
    "<mtable", "<mlabeledtr", "<maligngroup", "<malignmark",
    "<semantics", "<annotation", "<mmultiscripts", "<ms>",
  ]:
    check.require(token in math_text, f"16-math.xhtml missing MathML sample: {token}")

  english_text = ENGLISH_PAGE.read_text(encoding="utf-8")
  for token in [
    'xml:lang="en"',
    'body class="english-fiction"',
    'class="english-chapter-title"',
    'class="en-noindent"',
    'class="en-noindent en-first-letter"',
    'class="en-illustration"',
    'class="en-large-probe"',
  ]:
    check.require(token in english_text, f"18-english-fiction.xhtml missing English fiction marker: {token}")

  effects_css = (OEBPS / "Styles" / "effects.css").read_text(encoding="utf-8")
  note_text = NOTE_BOXES_PAGE.read_text(encoding="utf-8")
  check.require(
    "transform:" not in effects_css and "-webkit-transform:" not in effects_css,
    "effects.css note fixtures must not use transform; Kindle Previewer 3 KFX conversion crashes on rotated note boxes",
  )
  for token in [
    ".note-square", ".note-dashed", ".note-double", ".note-left-rule",
    ".note-shadow", ".note-inset", ".note-slant", ".note-irregular",
  ]:
    check.require(token in effects_css, f"effects.css missing note box style: {token}")
  for token in [
    'class="note-box note-square"',
    'class="note-box note-shadow"',
    'class="note-box note-slant"',
    'class="note-box note-irregular"',
  ]:
    check.require(token in note_text, f"19-border-shadow-notes.xhtml missing note box sample: {token}")


def validate_epub(epub_path: Path, check: Check) -> None:
  if not epub_path.exists():
    check.fail(f"EPUB does not exist: {epub_path}")
    return
  try:
    with zipfile.ZipFile(epub_path) as zf:
      infos = zf.infolist()
      check.require(bool(infos), "EPUB zip is empty")
      if infos:
        first = infos[0]
        check.require(first.filename == "mimetype", "EPUB mimetype must be first zip entry")
        check.require(first.compress_type == zipfile.ZIP_STORED, "EPUB mimetype must be stored")
        check.require(
          zf.read("mimetype") == b"application/epub+zip",
          "EPUB mimetype content is invalid",
        )

      names = set(zf.namelist())
      package_name = "OEBPS/package.opf"
      check.require(package_name in names, "EPUB missing OEBPS/package.opf")
      if package_name in names:
        root = ET.fromstring(zf.read(package_name))
        for item in root.findall("opf:manifest/opf:item", OPF_NS):
          href = item.attrib.get("href")
          if href:
            full = posixpath.normpath(posixpath.join("OEBPS", href))
            check.require(full in names, f"EPUB manifest href missing in zip: {href}")
  except (zipfile.BadZipFile, ET.ParseError, KeyError) as exc:
    check.fail(f"EPUB validation failed: {epub_path}: {exc}")


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser()
  parser.add_argument("--epub", type=Path, help="Optional built EPUB to validate")
  args = parser.parse_args(argv)

  check = Check()
  validate_source(check)
  if args.epub:
    validate_epub(args.epub, check)

  if check.errors:
    for error in check.errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("epub-style-demo validation ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
