#!/usr/bin/env python3
"""Recommend an AI-assisted refinement plan for one existing EPUB.

This harness does not rewrite the book. It turns package facts into a staged
cleanup brief: preflight, EPUB3 migration, popup notes, typography/fonts,
images, diff review, and skill order.
"""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import shutil
import sys
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_ai_harness import inspect_path, shell_path


CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
OPF_NS = {"opf": "http://www.idpf.org/2007/opf", "dc": "http://purl.org/dc/elements/1.1/"}
IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp", ".svg", ".gif", ".tif", ".tiff"}
FONT_EXTS = {".otf", ".ttf", ".woff", ".woff2"}
RISKY_IMAGE_EXTS = {
  ".webp": "convert to JPEG/PNG for Kindle/main-path EPUB delivery",
  ".tif": "convert to JPEG/PNG before EPUB delivery",
  ".tiff": "convert to JPEG/PNG before EPUB delivery",
  ".gif": "replace animated/legacy GIF with a still PNG/JPEG unless animation is explicitly required",
}


def parse_xml(data: bytes, label: str) -> ET.Element | None:
  try:
    return ET.fromstring(data)
  except ET.ParseError:
    return None


def norm_join(base: str, href: str) -> str:
  return posixpath.normpath(posixpath.join(base, href.split("#", 1)[0]))


def split_props(value: str | None) -> set[str]:
  return set((value or "").split())


def css_url_values(css: str) -> list[str]:
  active = re.sub(r"/\*.*?\*/", "", css, flags=re.S)
  values: list[str] = []
  for match in re.finditer(r"url\(\s*(?P<url>[^)]+?)\s*\)", active, re.I):
    value = match.group("url").strip().strip("\"'")
    if value and not re.match(r"^(?:[a-z][a-z0-9+.-]*:|//|#)", value, re.I):
      values.append(value)
  return values


def css_blocks(css: str, selector: str) -> list[str]:
  return [m.group("body") for m in re.finditer(rf"{re.escape(selector)}\s*\{{(?P<body>[^}}]+)\}}", css, re.S)]


def font_family_values(css: str) -> list[str]:
  return [m.group("value").strip() for m in re.finditer(r"font-family\s*:\s*(?P<value>[^;]+);", css, re.I)]


def split_font_chain(value: str) -> list[str]:
  return [part.strip().strip("\"'") for part in value.split(",") if part.strip()]


def font_face_families(css: str) -> set[str]:
  families: set[str] = set()
  for block in re.findall(r"@font-face\s*\{(?P<body>[^}]+)\}", css, re.I | re.S):
    for value in font_family_values(block):
      chain = split_font_chain(value)
      if chain:
        families.add(chain[0])
  return families


def resolve_package(zf: zipfile.ZipFile) -> tuple[str | None, ET.Element | None]:
  if "META-INF/container.xml" not in zf.namelist():
    return None, None
  container = parse_xml(zf.read("META-INF/container.xml"), "META-INF/container.xml")
  if container is None:
    return None, None
  rootfile = container.find(".//c:rootfile", CONTAINER_NS)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
  if not opf_path or opf_path not in zf.namelist():
    return opf_path, None
  return opf_path, parse_xml(zf.read(opf_path), opf_path)


def text_content(elem: ET.Element | None) -> str:
  if elem is None:
    return ""
  return " ".join("".join(elem.itertext()).split())


def tool_availability() -> dict[str, bool]:
  return {
    "magick": shutil.which("magick") is not None,
    "oxipng": shutil.which("oxipng") is not None,
    "pngquant": shutil.which("pngquant") is not None,
    "jpegoptim": shutil.which("jpegoptim") is not None,
    "svgo": shutil.which("svgo") is not None,
    "epubcheck": shutil.which("epubcheck") is not None,
  }


def scan_epub(path: Path) -> dict[str, object]:
  facts: dict[str, object] = {
    "package_version": None,
    "opf": None,
    "language": None,
    "nav_items": 0,
    "has_ncx": False,
    "has_drm_marker": False,
    "xhtml_count": 0,
    "css_count": 0,
    "image_counts": {},
    "font_files": [],
    "font_face_families": [],
    "body_font_chains": [],
    "body_line_height_rules": 0,
    "noteref_count": 0,
    "footnote_body_count": 0,
    "legacy_note_candidates": 0,
    "duokan_note_markers": 0,
    "ruby_count": 0,
    "vertical_markers": 0,
    "risky_images": [],
    "css_font_urls": [],
  }
  with zipfile.ZipFile(path) as zf:
    names = set(zf.namelist())
    facts["has_drm_marker"] = "META-INF/encryption.xml" in names
    opf_path, root = resolve_package(zf)
    facts["opf"] = opf_path
    if root is None or opf_path is None:
      return facts
    opf_dir = posixpath.dirname(opf_path)
    facts["package_version"] = root.attrib.get("version")
    facts["language"] = text_content(root.find(".//dc:language", OPF_NS)) or None
    navs = [
      item for item in root.findall("opf:manifest/opf:item", OPF_NS)
      if "nav" in split_props(item.attrib.get("properties"))
    ]
    facts["nav_items"] = len(navs)
    facts["has_ncx"] = any(
      item.attrib.get("media-type") == "application/x-dtbncx+xml" or item.attrib.get("id") == "ncx"
      for item in root.findall("opf:manifest/opf:item", OPF_NS)
    )

    image_counts: dict[str, int] = {}
    font_files: list[str] = []
    css_files: list[str] = []
    xhtml_files: list[str] = []
    risky_images: list[dict[str, str]] = []
    for item in root.findall("opf:manifest/opf:item", OPF_NS):
      href = item.attrib.get("href", "")
      media_type = item.attrib.get("media-type", "")
      zip_path = norm_join(opf_dir, href) if href else ""
      ext = Path(href).suffix.lower()
      if media_type == "application/xhtml+xml" or ext in {".xhtml", ".html"}:
        xhtml_files.append(zip_path)
      elif media_type == "text/css" or ext == ".css":
        css_files.append(zip_path)
      elif media_type.startswith("image/") or ext in IMAGE_EXTS:
        image_counts[ext or media_type] = image_counts.get(ext or media_type, 0) + 1
        props = split_props(item.attrib.get("properties"))
        if ext in RISKY_IMAGE_EXTS:
          risky_images.append({"path": zip_path, "reason": RISKY_IMAGE_EXTS[ext]})
        if ext == ".svg" and "cover-image" in props:
          risky_images.append({"path": zip_path, "reason": "SVG-only cover is risky for Kindle delivery"})
      elif ext in FONT_EXTS or media_type.startswith("font/") or "font" in media_type:
        font_files.append(zip_path)

    all_css = ""
    css_font_urls: list[str] = []
    for css_path in css_files:
      if css_path not in names:
        continue
      css = zf.read(css_path).decode("utf-8", errors="ignore")
      all_css += "\n" + css
      for value in css_url_values(css):
        if Path(value.split("#", 1)[0]).suffix.lower() in FONT_EXTS:
          css_font_urls.append(f"{css_path} -> {value}")
    body_font_chains = []
    body_line_height_rules = 0
    for block in css_blocks(all_css, "body"):
      body_line_height_rules += len(re.findall(r"line-height\s*:", block, re.I))
      for value in font_family_values(block):
        body_font_chains.append(split_font_chain(value))

    noteref_count = 0
    footnote_body_count = 0
    legacy_note_candidates = 0
    duokan_note_markers = 0
    ruby_count = 0
    vertical_markers = 0
    for xhtml_path in xhtml_files:
      if xhtml_path not in names:
        continue
      text = zf.read(xhtml_path).decode("utf-8", errors="ignore")
      noteref_count += len(re.findall(r'epub:type=["\']noteref["\']', text))
      footnote_body_count += len(re.findall(r'epub:type=["\']footnote["\']', text))
      legacy_note_candidates += len(re.findall(r'<a\b[^>]+href=["\']#[^"\']+["\'][^>]*>\s*(?:\[\d+\]|\d+|注|[*†‡])\s*</a>', text, re.I))
      duokan_note_markers += text.count("duokan-")
      ruby_count += len(re.findall(r"<ruby\b", text, re.I))
      vertical_markers += len(re.findall(r"writing-mode|page-vrl|vertical-rl", text, re.I))

    facts.update({
      "xhtml_count": len(xhtml_files),
      "css_count": len(css_files),
      "image_counts": image_counts,
      "font_files": font_files,
      "font_face_families": sorted(font_face_families(all_css)),
      "body_font_chains": body_font_chains,
      "body_line_height_rules": body_line_height_rules,
      "noteref_count": noteref_count,
      "footnote_body_count": footnote_body_count,
      "legacy_note_candidates": legacy_note_candidates,
      "duokan_note_markers": duokan_note_markers,
      "ruby_count": ruby_count,
      "vertical_markers": vertical_markers,
      "risky_images": risky_images,
      "css_font_urls": css_font_urls,
    })
  return facts


def phase(level: str, phase_id: str, title: str, why: str, commands: list[str], skills: list[str]) -> dict[str, object]:
  return {
    "level": level,
    "id": phase_id,
    "title": title,
    "why": why,
    "commands": commands,
    "skills": skills,
  }


def build_recommendations(path: Path, facts: dict[str, object], ai_data: dict[str, object]) -> list[dict[str, object]]:
  quoted = shell_path(path)
  recommendations: list[dict[str, object]] = []
  recommendations.append(phase(
    "required",
    "preflight",
    "Run format/package preflight before any rewrite",
    "Broken ZIP, container, OPF, manifest, spine, XML, CSS url(), or DRM markers must stop cleanup.",
    [f"python3 scripts/epub_preflight_harness.py {quoted} --format json > work/preflight.json"],
    ["$epub-package-nav-auditor"],
  ))
  if facts.get("package_version") != "3.0" or facts.get("nav_items") != 1:
    recommendations.append(phase(
      "recommended",
      "epub3-migration",
      "Migrate package to EPUB 3 before richer cleanup",
      "EPUB 3 nav, metadata, and manifest semantics are the stable base for popup notes, richer CSS, and reader-matrix testing.",
      [f"python3 scripts/epub3_migration_harness.py {quoted} --write-output work/after/step-1-epub3.epub --format json"],
      ["$epub-package-nav-auditor", "$epub-kindle-compatibility-checker"],
    ))
  if facts.get("noteref_count") or facts.get("legacy_note_candidates") or facts.get("duokan_note_markers"):
    recommendations.append(phase(
      "recommended",
      "popup-notes",
      "Normalize notes to the project popup footnote structure",
      "Detected noteref or legacy note markers. Convert to same-file grouped footnotes, then optionally add legacy fallback.",
      [
        "Dry-run $epub-popup-footnote-converter and review note body preservation.",
        f"bash scripts/validate-popup-notes.sh --epub {quoted}",
      ],
      ["$epub-popup-footnote-converter", "$epub-legacy-footnote-fallback"],
    ))
  body_chains = facts.get("body_font_chains") or []
  embedded_families = set(facts.get("font_face_families") or [])
  has_long_body_chain = any(len(chain) > 4 for chain in body_chains)
  embeds_body_font = any(any(name in embedded_families for name in chain) for chain in body_chains)
  if body_chains or facts.get("font_files") or facts.get("css_font_urls"):
    level = "recommended" if has_long_body_chain or embeds_body_font or facts.get("font_files") else "optional"
    reason = "Review system-first chains, title/rare-character embedded fonts, and OPF font manifest."
    if embeds_body_font:
      reason += " Embedded font appears in a body chain; this usually needs AI judgment."
    recommendations.append(phase(
      level,
      "typography-fonts",
      "Generate a CJK typography and font strategy",
      reason,
      [
        "Dry-run $epub-typography-optimizer; keep default body fonts system-first unless the book has a documented rare-character exception.",
        "Use explicit classes such as .title-special or .rare for embedded fonts.",
      ],
      ["$epub-typography-optimizer", "$epub-css-layering-optimizer"],
    ))
  risky_images = facts.get("risky_images") or []
  if risky_images or facts.get("image_counts"):
    commands = [
      "Use ImageMagick `magick` for format conversion when WebP/TIFF/GIF/SVG must become JPEG/PNG.",
      "Use `oxipng` or `pngquant` for PNG optimization and `jpegoptim` for JPEG optimization after reviewing visual quality.",
      "After external conversion, rerun package/nav audit and diff review.",
    ]
    recommendations.append(phase(
      "recommended" if risky_images else "optional",
      "images",
      "Review image delivery formats and figure layout",
      "Images affect Kindle compatibility, file size, cover detection, and reflow behavior.",
      commands,
      ["$epub-image-layout-optimizer", "$epub-kindle-compatibility-checker"],
    ))
  if facts.get("ruby_count") or facts.get("vertical_markers"):
    recommendations.append(phase(
      "optional",
      "ruby-vertical",
      "Review Ruby and vertical-writing behavior",
      "Ruby and vertical markers need reader testing rather than pure CSS assumptions.",
      ["Dry-run $epub-vertical-ruby-optimizer, then test the generated EPUB in target readers."],
      ["$epub-vertical-ruby-optimizer"],
    ))
  recommendations.append(phase(
    "required",
    "redline-and-diff",
    "Gate every written step with redline validation and diff review",
    "AI can suggest structure and styling, but text, metadata, spine, anchors, and cover are red lines.",
    [
      f"python3 scripts/validate_text_invariance.py work/before/source.epub work/after/step-N.epub --check all",
      "Use Calibre Editor or VS Code as documented in README #epub-diff-review.",
    ],
    ["$epub-layout-auditor"],
  ))
  ai_skills = ai_data.get("recommended_skills") or []
  if ai_skills:
    recommendations.append(phase(
      "info",
      "ai-skill-order",
      "Use AI skills as dry-run planners before write steps",
      "The skill list is a candidate order; the operator still decides based on findings and red-line risk.",
      [f"Candidate skills: {', '.join(ai_skills)}"],
      list(ai_skills),
    ))
  return recommendations


def analyze(path: Path) -> tuple[int, dict[str, object]]:
  ai_report = inspect_path(path, "cleanup")
  ai_data = ai_report.as_dict()
  try:
    facts = scan_epub(path)
  except zipfile.BadZipFile:
    facts = {"error": "Input is not a valid zip/EPUB"}
  data: dict[str, object] = {
    "harness": "epub_refinement_harness",
    "input": str(path),
    "facts": facts,
    "tool_availability": tool_availability(),
    "preflight": {
      "findings_by_level": ai_data.get("findings_by_level", {}),
      "findings": ai_data.get("findings", []),
    },
    "recommendations": build_recommendations(path, facts, ai_data),
  }
  errors = ai_data.get("findings_by_level", {}).get("error", 0)
  return (1 if errors else 0), data


def render_markdown(data: dict[str, object]) -> str:
  facts = data["facts"]
  lines = [
    "# EPUB Refinement Harness Report",
    "",
    f"- Input: `{data['input']}`",
    f"- Package version: `{facts.get('package_version')}`",
    f"- OPF: `{facts.get('opf')}`",
    f"- Language: `{facts.get('language')}`",
    "",
    "## Facts",
    f"- XHTML files: `{facts.get('xhtml_count')}`",
    f"- CSS files: `{facts.get('css_count')}`",
    f"- Images: `{facts.get('image_counts')}`",
    f"- Font files: `{facts.get('font_files')}`",
    f"- Font-face families: `{facts.get('font_face_families')}`",
    f"- Body font chains: `{facts.get('body_font_chains')}`",
    f"- Noterefs: `{facts.get('noteref_count')}`, legacy note candidates: `{facts.get('legacy_note_candidates')}`",
    "",
    "## Recommendations",
  ]
  for item in data["recommendations"]:
    lines.append(f"- [{item['level']}] `{item['id']}` {item['title']}: {item['why']}")
    for command in item["commands"]:
      lines.append(f"  - {command}")
  lines.extend(["", "## Tool Availability"])
  for tool, available in data["tool_availability"].items():
    lines.append(f"- `{tool}`: `{available}`")
  return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Recommend an EPUB refinement plan")
  parser.add_argument("path", type=Path, help="Existing EPUB file")
  parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  code, data = analyze(args.path)
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(render_markdown(data), end="")
  return code


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
