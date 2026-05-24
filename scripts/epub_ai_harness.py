#!/usr/bin/env python3
"""Route EPUB/source inputs to the right project skills with stdlib-only checks."""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import shlex
import shutil
import sys
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
MATHML_URI = "http://www.w3.org/1998/Math/MathML"
SVG_URI = "http://www.w3.org/2000/svg"
TEXT_EXTS = {".txt", ".md", ".markdown", ".html", ".xhtml", ".xml"}
IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp", ".svg", ".gif", ".tif", ".tiff"}
PDF_EXTS = {".pdf"}


def split_props(value: str | None) -> set[str]:
  return set((value or "").split())


def norm_join(base: str, href: str) -> str:
  href = href.split("#", 1)[0]
  return posixpath.normpath(posixpath.join(base, href))


def shell_path(path: Path) -> str:
  return shlex.quote(str(path))


def finding(level: str, message: str, path: str | None = None) -> dict[str, str]:
  data = {"level": level, "message": message}
  if path:
    data["path"] = path
  return data


class Report:
  def __init__(self, input_path: Path) -> None:
    self.input = str(input_path)
    self.mode = "unknown"
    self.summary: dict[str, object] = {}
    self.findings: list[dict[str, str]] = []
    self.skills: list[str] = []
    self.commands: list[str] = []
    self.tools: dict[str, bool] = {}

  def add_skill(self, name: str) -> None:
    skill = f"${name}"
    if skill not in self.skills:
      self.skills.append(skill)

  def add_command(self, command: str) -> None:
    if command not in self.commands:
      self.commands.append(command)

  def as_dict(self) -> dict[str, object]:
    return {
      "input": self.input,
      "mode": self.mode,
      "summary": self.summary,
      "findings": self.findings,
      "recommended_skills": self.skills,
      "suggested_commands": self.commands,
      "tool_availability": self.tools,
    }


def parse_xml_bytes(data: bytes, report: Report, label: str) -> ET.Element | None:
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    report.findings.append(finding("error", f"XML parse failed: {exc}", label))
    return None


def opf_manifest(root: ET.Element) -> list[ET.Element]:
  return root.findall("opf:manifest/opf:item", OPF_NS)


def inspect_opf(
  report: Report,
  root: ET.Element,
  opf_dir: str,
  exists,
  read_bytes,
) -> None:
  manifest = opf_manifest(root)
  spine = root.findall("opf:spine/opf:itemref", OPF_NS)
  media_counts = {
    "xhtml": 0,
    "css": 0,
    "images": 0,
    "fonts": 0,
    "other": 0,
  }
  href_by_id: dict[str, str] = {}
  xhtml_items: list[ET.Element] = []
  image_items: list[ET.Element] = []

  for item in manifest:
    item_id = item.attrib.get("id")
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    if item_id and href:
      href_by_id[item_id] = href
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      media_counts["xhtml"] += 1
      xhtml_items.append(item)
    elif media_type == "text/css" or href.endswith(".css"):
      media_counts["css"] += 1
    elif media_type.startswith("image/") or Path(href).suffix.lower() in IMAGE_EXTS:
      media_counts["images"] += 1
      image_items.append(item)
    elif "font" in media_type:
      media_counts["fonts"] += 1
    else:
      media_counts["other"] += 1

  report.summary["manifest_items"] = len(manifest)
  report.summary["spine_items"] = len(spine)
  report.summary["media_counts"] = media_counts
  package_version = root.attrib.get("version", "")
  if package_version:
    report.summary["package_version"] = package_version

  language = root.findtext(".//{http://purl.org/dc/elements/1.1/}language")
  if language:
    report.summary["language"] = language
  is_english = bool(language and language.lower().startswith("en"))
  if is_english:
    report.add_skill("epub-english-typography-optimizer")

  nav_items = [item for item in manifest if "nav" in split_props(item.attrib.get("properties"))]
  if len(nav_items) != 1:
    level = "warn" if package_version.startswith("2") else "error"
    report.findings.append(finding(level, "EPUB 3 package should contain exactly one nav item"))
    report.add_skill("epub-package-nav-auditor")

  ncx_items = [
    item for item in manifest
    if item.attrib.get("id") == "ncx" or item.attrib.get("media-type") == "application/x-dtbncx+xml"
  ]
  spine_root = root.find("opf:spine", OPF_NS)
  has_spine_ncx = spine_root is not None and bool(spine_root.attrib.get("toc"))
  if not ncx_items or not has_spine_ncx:
    report.findings.append(finding("warn", "Kindle/legacy delivery should keep toc.ncx and spine toc=\"ncx\""))
    report.add_skill("epub-kindle-compatibility-checker")
    report.add_skill("epub-package-nav-auditor")

  cover_items = [item for item in manifest if "cover-image" in split_props(item.attrib.get("properties"))]
  meta_cover = root.find(".//opf:meta[@name='cover']", OPF_NS)
  if not cover_items or meta_cover is None:
    report.findings.append(finding("warn", "Cover should have properties=\"cover-image\" and legacy meta name=\"cover\""))
    report.add_skill("epub-image-layout-optimizer")
    report.add_skill("epub-kindle-compatibility-checker")

  for item in manifest:
    href = item.attrib.get("href")
    if not href:
      report.findings.append(finding("error", "Manifest item missing href", item.attrib.get("id")))
      report.add_skill("epub-package-nav-auditor")
      continue
    target = norm_join(opf_dir, href)
    if not exists(target):
      report.findings.append(finding("error", "Manifest href missing", target))
      report.add_skill("epub-package-nav-auditor")

  for itemref in spine:
    idref = itemref.attrib.get("idref")
    if not idref or idref not in href_by_id:
      report.findings.append(finding("error", "Spine idref missing from manifest", idref or "<missing>"))
      report.add_skill("epub-package-nav-auditor")

  for item in image_items:
    href = item.attrib.get("href", "")
    ext = Path(href).suffix.lower()
    props = split_props(item.attrib.get("properties"))
    if ext == ".webp":
      report.findings.append(finding("warn", "WebP is not a Kindle main-path image format", href))
      report.add_skill("epub-kindle-compatibility-checker")
      report.add_skill("epub-image-layout-optimizer")
    elif ext == ".svg" and "cover-image" in props:
      report.findings.append(finding("warn", "SVG-only cover is risky for Kindle delivery", href))
      report.add_skill("epub-kindle-compatibility-checker")
      report.add_skill("epub-image-layout-optimizer")
    elif ext in {".tif", ".tiff", ".gif"}:
      report.findings.append(finding("warn", "Convert this image to JPEG/PNG for EPUB delivery", href))
      report.add_skill("epub-image-layout-optimizer")

  for item in xhtml_items:
    href = item.attrib.get("href", "")
    target = norm_join(opf_dir, href)
    if not exists(target):
      continue
    text = read_bytes(target).decode("utf-8", errors="ignore")
    props = split_props(item.attrib.get("properties"))
    if re.search(r'\b(?:xml:)?lang=["\']en(?:[-_][A-Za-z0-9]+)?["\']', text):
      report.add_skill("epub-english-typography-optimizer")
    has_math = "<math" in text or MATHML_URI in text
    if has_math and "mathml" not in props:
      report.findings.append(finding("error", "MathML XHTML item missing properties=\"mathml\"", href))
      report.add_skill("epub-package-nav-auditor")
      report.add_skill("epub-kindle-compatibility-checker")
    has_svg = "<svg" in text or SVG_URI in text
    if has_svg and "svg" not in props:
      report.findings.append(finding("error", "Inline SVG XHTML item missing properties=\"svg\"", href))
      report.add_skill("epub-package-nav-auditor")
    if "epub:type=\"noteref\"" in text or "epub:type='noteref'" in text:
      report.add_skill("epub-popup-footnote-converter")
      if "epub:type=\"footnote\"" not in text and "epub:type='footnote'" not in text:
        report.findings.append(finding("warn", "noteref found without same-file footnote aside", href))
    if "duokan-footnote" in text:
      report.add_skill("epub-legacy-footnote-fallback")
    if "writing-mode" in text or "page-vrl" in text or "<ruby" in text:
      report.add_skill("epub-vertical-ruby-optimizer")

  if media_counts["css"]:
    report.add_skill("epub-css-layering-optimizer")
  if media_counts["images"]:
    report.add_skill("epub-image-layout-optimizer")
  if media_counts["xhtml"] and not is_english:
    report.add_skill("epub-typography-optimizer")
  report.add_skill("epub-layout-auditor")
  report.add_skill("epub-package-nav-auditor")


def inspect_epub(path: Path, report: Report) -> None:
  report.mode = "existing-epub"
  report.add_command(f"scripts/epub_ai_harness.py {shell_path(path)}")
  report.add_command(f"scripts/validate-popup-notes.sh --epub {shell_path(path)}")
  try:
    with zipfile.ZipFile(path) as zf:
      infos = zf.infolist()
      names = set(zf.namelist())
      report.summary["zip_entries"] = len(infos)
      if not infos:
        report.findings.append(finding("error", "EPUB zip is empty"))
        return
      first = infos[0]
      if first.filename != "mimetype" or first.compress_type != zipfile.ZIP_STORED:
        report.findings.append(finding("error", "EPUB mimetype must be first zip entry and stored"))
        report.add_skill("epub-package-nav-auditor")
      if "META-INF/container.xml" not in names:
        report.findings.append(finding("error", "EPUB missing META-INF/container.xml"))
        report.add_skill("epub-package-nav-auditor")
        return
      container = parse_xml_bytes(zf.read("META-INF/container.xml"), report, "META-INF/container.xml")
      if container is None:
        return
      rootfile = container.find(".//c:rootfile", CONTAINER_NS)
      opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
      if not opf_path or opf_path not in names:
        report.findings.append(finding("error", "container.xml rootfile does not resolve", opf_path or "<missing>"))
        report.add_skill("epub-package-nav-auditor")
        return
      opf_root = parse_xml_bytes(zf.read(opf_path), report, opf_path)
      if opf_root is None:
        return
      opf_dir = posixpath.dirname(opf_path)
      report.summary["opf"] = opf_path
      inspect_opf(report, opf_root, opf_dir, lambda p: p in names, zf.read)
  except zipfile.BadZipFile:
    report.findings.append(finding("error", "Input is not a valid zip/EPUB"))


def inspect_epub_tree(path: Path, report: Report, opf_path: Path) -> None:
  report.mode = "epub-source-tree"
  report.add_command(f"scripts/epub_ai_harness.py {shell_path(path)}")
  if path.name == "epub-style-demo":
    report.add_command("sh templates/epub-style-demo/build.sh")
    report.add_command("scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub")
    report.add_command("scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub")
  try:
    opf_root = ET.parse(opf_path).getroot()
  except ET.ParseError as exc:
    report.findings.append(finding("error", f"OPF parse failed: {exc}", str(opf_path)))
    report.add_skill("epub-package-nav-auditor")
    return
  root_dir = path
  rel_opf = opf_path.relative_to(root_dir).as_posix()
  opf_dir = posixpath.dirname(rel_opf)

  def exists(rel: str) -> bool:
    return (root_dir / rel).exists()

  def read(rel: str) -> bytes:
    return (root_dir / rel).read_bytes()

  report.summary["opf"] = rel_opf
  inspect_opf(report, opf_root, opf_dir, exists, read)


def find_opf_in_tree(path: Path) -> Path | None:
  container = path / "META-INF" / "container.xml"
  if container.exists():
    try:
      root = ET.parse(container).getroot()
      rootfile = root.find(".//c:rootfile", CONTAINER_NS)
      opf = rootfile.attrib.get("full-path") if rootfile is not None else None
      if opf and (path / opf).exists():
        return path / opf
    except ET.ParseError:
      return None
  opfs = sorted(path.rglob("*.opf"))
  return opfs[0] if opfs else None


def inspect_source(path: Path, report: Report) -> None:
  report.mode = "source-intake"
  report.add_skill("epub-source-intake")
  report.add_command(f"scripts/epub_ai_harness.py {shell_path(path)}")
  suffix = path.suffix.lower()
  if suffix in PDF_EXTS:
    for tool in ("pdftotext", "mutool", "ocrmypdf", "tesseract"):
      report.tools[tool] = shutil.which(tool) is not None
    if not report.tools.get("pdftotext") and not report.tools.get("mutool"):
      report.findings.append(finding("warn", "No common PDF text extractor found in PATH; extraction is external to this repo"))
    report.findings.append(finding("warn", "PDF extraction requires sample-page proofreading before EPUB layout work"))
  elif suffix in TEXT_EXTS:
    report.findings.append(finding("info", "Text/HTML source should be structured before visual EPUB optimization"))
  elif suffix in IMAGE_EXTS:
    report.findings.append(finding("info", "Image input needs role classification: cover, figure, icon, formula, or source art"))
    report.add_skill("epub-image-layout-optimizer")


def inspect_directory(path: Path, report: Report) -> None:
  opf = find_opf_in_tree(path)
  if opf:
    inspect_epub_tree(path, report, opf)
    return
  files = [p for p in path.rglob("*") if p.is_file()]
  counts = {"pdf": 0, "text": 0, "images": 0, "epub": 0, "other": 0}
  for file in files:
    ext = file.suffix.lower()
    if ext in PDF_EXTS:
      counts["pdf"] += 1
    elif ext in TEXT_EXTS:
      counts["text"] += 1
    elif ext in IMAGE_EXTS:
      counts["images"] += 1
    elif ext == ".epub":
      counts["epub"] += 1
    else:
      counts["other"] += 1
  report.mode = "mixed-directory" if counts["epub"] else "source-intake"
  report.summary["file_counts"] = counts
  report.add_command(f"scripts/epub_ai_harness.py {shell_path(path)}")
  if counts["epub"]:
    report.add_skill("epub-layout-auditor")
  if counts["pdf"] or counts["text"]:
    report.add_skill("epub-source-intake")
  if counts["images"]:
    report.add_skill("epub-image-layout-optimizer")
  if not files:
    report.findings.append(finding("warn", "Directory is empty"))
  else:
    report.findings.append(finding("info", "Directory has no OPF/container.xml; treat as source intake"))


def inspect_path(path: Path) -> Report:
  report = Report(path)
  if not path.exists():
    report.mode = "missing"
    report.findings.append(finding("error", "Input path does not exist"))
    return report
  if path.is_dir():
    inspect_directory(path, report)
  elif path.suffix.lower() == ".epub":
    inspect_epub(path, report)
  else:
    inspect_source(path, report)
  report.add_command("scripts/validate_skills_basic.py")
  if not report.findings:
    report.findings.append(finding("info", "No immediate structural issue detected by harness"))
  return report


def render_markdown(report: Report) -> str:
  data = report.as_dict()
  lines = [
    "# EPUB AI Harness Report",
    "",
    f"- Input: `{data['input']}`",
    f"- Mode: `{data['mode']}`",
  ]
  if data["summary"]:
    lines.extend(["", "## Summary"])
    for key, value in data["summary"].items():
      lines.append(f"- `{key}`: `{value}`")
  lines.extend(["", "## Findings"])
  for item in data["findings"]:
    path = f" `{item['path']}`" if "path" in item else ""
    lines.append(f"- [{item['level']}]{path} {item['message']}")
  lines.extend(["", "## Recommended Skills"])
  for skill in data["recommended_skills"]:
    lines.append(f"- `{skill}`")
  if data["tool_availability"]:
    lines.extend(["", "## Tool Availability"])
    for tool, available in data["tool_availability"].items():
      lines.append(f"- `{tool}`: `{available}`")
  lines.extend(["", "## Suggested Commands"])
  for command in data["suggested_commands"]:
    lines.append(f"- `{command}`")
  return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="EPUB/source routing harness for AI-assisted workflows")
  parser.add_argument("path", type=Path, help="EPUB, EPUB source tree, PDF/text source, or mixed source directory")
  parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  report = inspect_path(args.path)
  data = report.as_dict()
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(render_markdown(report), end="")
  has_error = any(item["level"] == "error" for item in report.findings)
  return 1 if has_error else 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
