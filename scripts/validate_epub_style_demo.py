#!/usr/bin/env python3
"""Validate the epub-style-demo fixture with only Python stdlib."""

from __future__ import annotations

import argparse
import os
import posixpath
import re
import shutil
import subprocess
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
BASE_CSS = OEBPS / "Styles" / "base.css"
FONTS_CSS = OEBPS / "Styles" / "fonts.css"
NOTES_CSS = OEBPS / "Styles" / "notes.css"
POSTER_CSS = OEBPS / "Styles" / "poster.css"
POSTER_CONTAIN_PAGE = OEBPS / "Text" / "03c-poster-contain.xhtml"
RUBY_NOTE_PAGE = OEBPS / "Text" / "02-ruby-note.xhtml"
FRONTMATTER_PAGE = OEBPS / "Text" / "15-frontmatter.xhtml"
IMAGE_LAYOUT = OEBPS / "Text" / "17-image-layout.xhtml"
ENGLISH_PAGE = OEBPS / "Text" / "18-english-fiction.xhtml"
NOTE_BOXES_PAGE = OEBPS / "Text" / "19-border-shadow-notes.xhtml"
CHAPTER_HEAD_PAGE = OEBPS / "Text" / "20-chapter-head-image.xhtml"
CLASSICAL_MODERN_PAGE = OEBPS / "Text" / "21-classical-modern.xhtml"
MATH_PAGE = OEBPS / "Text" / "16-math.xhtml"

OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
XHTML_NS = {"xhtml": "http://www.w3.org/1999/xhtml"}
NCX_NS = {"ncx": "http://www.daisy.org/z3986/2005/ncx/"}
MATHML_URI = "http://www.w3.org/1998/Math/MathML"
SVG_URI = "http://www.w3.org/2000/svg"


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


def has_namespaced_markup(path: Path, check: Check, uri: str) -> bool:
  root = parse_xml(path, check)
  if root is None:
    return False
  return any(elem.tag.startswith("{" + uri + "}") for elem in root.iter())


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



def has_ibooks_specified_fonts(package_root: ET.Element) -> bool:
  for meta in package_root.findall("opf:metadata/opf:meta", OPF_NS):
    if meta.attrib.get("property") == "ibooks:specified-fonts":
      return (meta.text or "").strip().lower() == "true"
  return False


def has_body_font_locked_markup(xhtml: str) -> bool:
  return re.search(
    r"<body[^>]*\bclass\s*=\s*(['\"])[^'\"]*\bbody-font-locked\b[^'\"]*\1",
    xhtml,
    re.I,
  ) is not None


def has_direct_body_font_family(css: str) -> bool:
  active = strip_css_comments(css)
  for match in re.finditer(r"([^{}]+)\{([^{}]*)\}", active, re.S):
    selectors, body = match.groups()
    if re.search(r"\bfont-family\s*:", body, re.I) is None:
      continue
    if any(
      selector.strip().split(";")[-1].strip().lower() == "body"
      for selector in selectors.split(",")
    ):
      return True
  return False


def validate_body_font_mode_contract(
  package_root: ET.Element,
  base_css: str,
  fonts_css: str,
  xhtml_texts: dict[str, str],
  check: Check,
  context: str,
) -> None:
  active_base_css = strip_css_comments(base_css)
  body_block = selector_block(active_base_css, "body")
  check.require(body_block is not None, f"{context}: base.css must define a body block")
  if body_block is not None:
    check.require(
      re.search(r"\bfont-family\s*:", body_block, re.I) is None,
      f"{context}: base.css body block must not set font-family; put locked-mode role binding in fonts.css",
    )

  active_fonts_css = strip_css_comments(fonts_css)
  has_class_rule = re.search(
    r"\.body-font-locked\b[^{}]*\{[^}]*\bfont-family\s*:",
    active_fonts_css,
    re.S | re.I,
  ) is not None
  has_direct_rule = has_direct_body_font_family(active_fonts_css)
  check.require(
    has_class_rule or has_direct_rule,
    f"{context}: fonts.css must define a direct body or legacy .body-font-locked font-family chain",
  )

  locked_hrefs = sorted(href for href, text in xhtml_texts.items() if has_body_font_locked_markup(text))
  has_locked_mode = bool(locked_hrefs) or has_direct_rule
  has_meta = has_ibooks_specified_fonts(package_root)
  check.require(
    has_locked_mode == has_meta,
    (
      f"{context}: locked body font mode and OPF ibooks:specified-fonts meta must match; "
      f"direct_body={has_direct_rule}, locked_pages={locked_hrefs or 'none'}, meta={has_meta}"
    ),
  )

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
    if not path.exists():
      continue
    props = split_props(item.attrib.get("properties"))
    if has_namespaced_markup(path, check, MATHML_URI):
      check.require("mathml" in props, f"MathML content missing OPF properties=mathml: {href}")
    if has_namespaced_markup(path, check, SVG_URI):
      check.require("svg" in props, f"Inline SVG content missing OPF properties=svg: {href}")

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
  note_item = href_to_item.get("Text/19-border-shadow-notes.xhtml")
  check.require(note_item is not None, "19-border-shadow-notes.xhtml must be in manifest")
  if note_item is not None:
    check.require(
      "svg" in split_props(note_item.attrib.get("properties")),
      "19-border-shadow-notes.xhtml manifest item must declare properties=svg",
    )
  check.require(
    href_to_item.get("Text/20-chapter-head-image.xhtml") is not None,
    "20-chapter-head-image.xhtml must be in manifest",
  )
  check.require(
    href_to_item.get("Text/21-classical-modern.xhtml") is not None,
    "21-classical-modern.xhtml must be in manifest",
  )
  check.require(
    href_to_item.get("Text/03c-poster-contain.xhtml") is not None,
    "03c-poster-contain.xhtml must be in manifest",
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

  base_css = BASE_CSS.read_text(encoding="utf-8")
  fonts_css = FONTS_CSS.read_text(encoding="utf-8")
  active_fonts_css = strip_css_comments(fonts_css)
  check.require(
    "../Fonts/" not in active_fonts_css,
    "fonts.css default @font-face skeleton leaked an active missing font URL",
  )
  xhtml_texts = {
    href: href_path(href).read_text(encoding="utf-8")
    for href in href_to_item
    if href and href.endswith(".xhtml") and href_path(href).exists()
  }
  validate_body_font_mode_contract(
    package_root,
    base_css,
    fonts_css,
    xhtml_texts,
    check,
    "source fixture",
  )

  poster_css = POSTER_CSS.read_text(encoding="utf-8")
  active_poster_css = strip_css_comments(poster_css)
  poster_contain_text = POSTER_CONTAIN_PAGE.read_text(encoding="utf-8")
  for token in [
    "body.poster-bg-contain",
    "background-size: contain",
    ".poster-fallback",
    "max-height: 100%",
    "@supports (background-size: contain)",
    "visibility: hidden",
  ]:
    check.require(token in poster_css, f"poster.css missing single-image contain fallback style: {token}")
  for token in [
    'body class="fullpage poster-bg-contain"',
    'section class="fullframe"',
    'class="poster-fallback"',
  ]:
    check.require(token in poster_contain_text, f"03c-poster-contain.xhtml missing marker: {token}")
  check.require("position: absolute" not in active_poster_css, "poster.css must not use position:absolute")
  check.require(re.search(r"\b[0-9.]+v[hw]\b", active_poster_css) is None, "poster.css must not use vh/vw units")

  note_css = NOTES_CSS.read_text(encoding="utf-8")
  ruby_note_text = RUBY_NOTE_PAGE.read_text(encoding="utf-8")
  check.require(
    'class="note-marker"' in ruby_note_text,
    "02-ruby-note.xhtml must scope image noteref in note-marker",
  )
  for token in [
    "sup.note-marker",
    "line-height: 0",
    "position: relative",
    "top: -0.14em",
    "height: 0.72em",
    "sup.note-marker > .noteref-icon > img",
  ]:
    check.require(token in note_css, f"notes.css missing scoped note-marker baseline rule: {token}")
  check.require("sup img" not in note_css, "notes.css must not use a global sup img rule")

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
    'class="en-noindent en-dropcap-host"',
    'class="en-dropcap"',
    'class="en-illustration"',
    'class="en-large-probe"',
  ]:
    check.require(token in english_text, f"18-english-fiction.xhtml missing English fiction marker: {token}")

  literary_css = (OEBPS / "Styles" / "literary.css").read_text(encoding="utf-8")
  for token in [
    ".en-first-letter::first-letter",
    ".en-dropcap-host",
    ".en-dropcap",
    "Snell Roundhand",
    "float: left",
  ]:
    check.require(token in literary_css, f"literary.css missing English fiction style: {token}")

  frontmatter_text = FRONTMATTER_PAGE.read_text(encoding="utf-8")
  for token in [
    'class="frontmatter copyright-page"',
    'class="copyright-heading"',
    'class="copyright-meta"',
    'class="copyright-meta-item"',
  ]:
    check.require(token in frontmatter_text, f"15-frontmatter.xhtml missing copyright marker: {token}")
  for token in [
    ".copyright-page",
    ".copyright-heading",
    ".copyright-meta",
    ".copyright-meta-item",
    "list-style: none",
  ]:
    check.require(token in literary_css, f"literary.css missing copyright page style: {token}")

  effects_css = (OEBPS / "Styles" / "effects.css").read_text(encoding="utf-8")
  active_effects_css = strip_css_comments(effects_css)
  note_text = NOTE_BOXES_PAGE.read_text(encoding="utf-8")
  # SPEC §5.10 bans rotated note boxes after Kindle Previewer 3.104 KFX failures.
  check.require(
    re.search(r"(?:-webkit-)?transform\s*:\s*[^;{}]*\brotate", active_effects_css) is None,
    "effects.css note fixtures must not use transform: rotate(); see docs/final/SPEC-实现约束.md §5.10",
  )
  for token in [
    ".note-square", ".note-dashed", ".note-double", ".note-left-rule",
    ".note-shadow", ".note-inset", ".note-slant", ".note-corner-ornament",
    ".note-ornate-rule", ".note-ornate-svg", ".note-corner-frame",
    ".note-long-shadow", ".note-irregular", ".note-handcut",
  ]:
    check.require(token in effects_css, f"effects.css missing note box style: {token}")
  for token in [
    'class="note-box note-square"',
    'class="note-box note-shadow"',
    'class="note-box note-slant"',
    'class="note-box note-corner-ornament"',
    'class="note-ornate-svg"',
    '<path class="note-ornate-main"',
    'class="note-box note-long-shadow"',
    'class="note-box note-irregular"',
    'class="note-box note-handcut"',
  ]:
    check.require(token in note_text, f"19-border-shadow-notes.xhtml missing note box sample: {token}")

  chapter_head_text = CHAPTER_HEAD_PAGE.read_text(encoding="utf-8")
  for token in [
    ".chapter-header",
    ".chapter-head-art",
    ".chapter-head-art-roomy",
    ".chapter-head-banner",
    ".decorated-chapter-title",
    ".chapter-head-note",
  ]:
    check.require(token in literary_css, f"literary.css missing chapter head image style: {token}")
  for token in [
    'class="chapter-with-head-image"',
    'class="chapter-header"',
    'class="chapter-head-art"',
    'class="chapter-head-banner"',
    'class="decorated-chapter-title"',
    'class="chapter-head-art chapter-head-art-roomy"',
    "Images/chapter-banner.png",
  ]:
    check.require(token in chapter_head_text, f"20-chapter-head-image.xhtml missing chapter head marker: {token}")

  classical_modern_text = CLASSICAL_MODERN_PAGE.read_text(encoding="utf-8")
  for token in [
    'class="classical-modern"',
    'id="classical-modern-toc"',
    'class="parallel-entry"',
    'class="parallel-entry-title"',
    'class="parallel-source"',
    'parallel-float-pair',
    'parallel-ratio-balanced',
    'parallel-ratio-source-wide',
    'parallel-stack-pair',
    'class="parallel-clear"',
    'class="classical-text font-st"',
    'class="modern-text font-kt"',
    'class="parallel-return"',
    'class="parallel-entry parallel-large-probe"',
  ]:
    check.require(token in classical_modern_text, f"21-classical-modern.xhtml missing marker: {token}")
  for token in [
    ".classical-modern",
    ".classical-modern-local-toc",
    ".parallel-entry",
    ".parallel-entry-title",
    ".parallel-source",
    ".parallel-pair",
    ".parallel-float-pair",
    ".parallel-float-pair.parallel-ratio-balanced",
    ".parallel-float-pair.parallel-ratio-source-wide",
    ".parallel-stack-pair",
    ".parallel-clear",
    ".classical-text",
    ".modern-text",
    ".parallel-return",
    ".parallel-large-probe",
  ]:
    check.require(token in literary_css, f"literary.css missing classical-modern style: {token}")
  classical_block = selector_block(literary_css, ".parallel-float-pair .classical-text") or ""
  modern_block = selector_block(literary_css, ".parallel-float-pair .modern-text") or ""
  pair_block = selector_block(literary_css, ".parallel-pair") or ""
  float_pair_block = selector_block(literary_css, ".parallel-float-pair") or ""
  stack_pair_block = selector_block(literary_css, ".parallel-stack-pair") or ""
  classical_width = percentage_width(literary_css, ".parallel-float-pair .classical-text")
  modern_width = percentage_width(literary_css, ".parallel-float-pair .modern-text")
  balanced_classical_width = percentage_width(
    literary_css,
    ".parallel-float-pair.parallel-ratio-balanced .classical-text",
  )
  balanced_modern_width = percentage_width(
    literary_css,
    ".parallel-float-pair.parallel-ratio-balanced .modern-text",
  )
  source_wide_classical_width = percentage_width(
    literary_css,
    ".parallel-float-pair.parallel-ratio-source-wide .classical-text",
  )
  source_wide_modern_width = percentage_width(
    literary_css,
    ".parallel-float-pair.parallel-ratio-source-wide .modern-text",
  )
  check.require("display: flex" not in pair_block, "parallel-pair must not depend on display:flex")
  check.require("page-break-inside: avoid" not in pair_block, "parallel-pair default must allow long stacked pairs to paginate")
  check.require("page-break-inside: avoid" in float_pair_block, "parallel-float-pair must protect short source/translation pairs from page splits")
  check.require("page-break-inside: auto" in stack_pair_block, "parallel-stack-pair must explicitly allow long stacked pairs to paginate")
  check.require("@media (min-width: 40em)" in literary_css, "parallel float layout must be a wide-screen progressive enhancement")
  check.require("float: left" in classical_block, "classical-text must float left in the wide enhancement")
  check.require("float: right" in modern_block, "modern-text must float right in the wide enhancement")
  check.require("display: flex" not in literary_css, "classical-modern layout must not depend on flex")
  check.require(classical_width is not None, "classical-text must define percentage width in the enhancement")
  check.require(modern_width is not None, "modern-text must define percentage width in the enhancement")
  if classical_width is not None:
    check.require(
      36 <= classical_width <= 40,
      f"classical-text width must stay near the sample 38/58 split, found {classical_width:g}%",
    )
  if modern_width is not None:
    check.require(
      56 <= modern_width <= 60,
      f"modern-text width must stay near the sample 38/58 split, found {modern_width:g}%",
    )
  check.require(
    balanced_classical_width == 48 and balanced_modern_width == 48,
    "parallel-ratio-balanced must define a 48/48 split",
  )
  check.require(
    source_wide_classical_width == 58 and source_wide_modern_width == 38,
    "parallel-ratio-source-wide must define a 58/38 split",
  )


def run_epubcheck(epub_path: Path, check: Check) -> None:
  command: list[str] | None = None
  if shutil.which("epubcheck"):
    command = ["epubcheck", str(epub_path)]
  else:
    jar = os.environ.get("EPUBCHECK_JAR")
    java = shutil.which("java")
    if java:
      probe = subprocess.run([java, "-version"], capture_output=True, check=False)
      if probe.returncode != 0:
        java = None
    if jar and java:
      command = [java, "-jar", jar, str(epub_path)]
    elif not java:
      print("WARN: epubcheck skipped: java is not installed", file=sys.stderr)
      return
    else:
      print(
        "WARN: epubcheck skipped: java is installed but neither epubcheck nor EPUBCHECK_JAR is configured",
        file=sys.stderr,
      )
      return

  result = subprocess.run(command, text=True, capture_output=True, check=False)
  if result.returncode != 0:
    output = "\n".join(part for part in (result.stdout.strip(), result.stderr.strip()) if part)
    check.fail(f"epubcheck failed for {epub_path}: {output[:2000]}")


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
        xhtml_texts: dict[str, str] = {}
        for item in root.findall("opf:manifest/opf:item", OPF_NS):
          href = item.attrib.get("href")
          if href:
            full = posixpath.normpath(posixpath.join("OEBPS", href))
            check.require(full in names, f"EPUB manifest href missing in zip: {href}")
            if href.endswith(".xhtml") and full in names:
              xhtml_texts[href] = zf.read(full).decode("utf-8")
        if "OEBPS/Styles/base.css" in names and "OEBPS/Styles/fonts.css" in names:
          validate_body_font_mode_contract(
            root,
            zf.read("OEBPS/Styles/base.css").decode("utf-8"),
            zf.read("OEBPS/Styles/fonts.css").decode("utf-8"),
            xhtml_texts,
            check,
            "EPUB artifact",
          )
        else:
          check.fail("EPUB artifact missing Styles/base.css or Styles/fonts.css")
    run_epubcheck(epub_path, check)
  except (zipfile.BadZipFile, ET.ParseError, KeyError, UnicodeDecodeError) as exc:
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
