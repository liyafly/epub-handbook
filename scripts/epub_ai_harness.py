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
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable
from urllib.parse import unquote
from xml.etree import ElementTree as ET


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
MATHML_URI = "http://www.w3.org/1998/Math/MathML"
SVG_URI = "http://www.w3.org/2000/svg"
XML_NS = "http://www.w3.org/XML/1998/namespace"
TEXT_EXTS = {".txt", ".md", ".markdown", ".html", ".xhtml", ".xml"}
IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp", ".svg", ".gif", ".tif", ".tiff"}
FONT_EXTS = {".otf", ".ttf", ".woff", ".woff2"}
FONT_MEDIA_TYPES = {
  "font/otf",
  "font/ttf",
  "font/woff",
  "font/woff2",
  "application/font-sfnt",
  "application/font-woff",
  "application/vnd.ms-opentype",
}
PDF_EXTS = {".pdf"}
LEVEL_ORDER = {"error": 0, "warn": 1, "info": 2}


# ── EpubModel — lightweight in-memory EPUB view for detectors ─────────


@dataclass
class EpubModel:
  xhtml_docs: dict[str, ET.Element] = field(default_factory=dict)
  opf_root: ET.Element | None = None
  opf_path: str = ""
  css_docs: dict[str, str] = field(default_factory=dict)
  book_language: str | None = None


# ── Detector registry ───────────────────────────────────────────────────


@dataclass(frozen=True)
class Detector:
  kind: str
  lane: str  # tag | css | package
  fn: Callable[..., list[dict]]


DETECTORS: list[Detector] = []


def detector(kind: str, lane: str) -> Callable:
  def reg(fn: Callable[..., list[dict]]) -> Callable[..., list[dict]]:
    DETECTORS.append(Detector(kind, lane, fn))
    return fn
  return reg


def collect_actionable_findings(model: EpubModel | None) -> list[dict]:
  if model is None:
    return []
  found: list[dict] = []
  detector_errors: list[dict[str, str]] = []
  for d in DETECTORS:
    try:
      found.extend(d.fn(model))
    except Exception as exc:
      detector_errors.append({"detector": d.kind, "error": str(exc)})
  for err in detector_errors:
    print(f"WARNING: detector {err['detector']} failed: {err['error']}", file=sys.stderr)
  return found


# ── Detectors ───────────────────────────────────────────────────────────


@detector("missing-html-lang", "tag")
def _detect_missing_html_lang(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    if not (root.get("lang") or root.get(f"{{{XML_NS}}}lang")):
      out.append({
        "kind": "missing-html-lang",
        "file": path,
        "locator": {"selector": "html"},
        "params": {"value": model.book_language or "zh-Hans"},
        "lane": "tag",
        "auto_fixable": True,
        "confidence": "high",
        "evidence": "<html> root element missing lang/xml:lang",
      })
  return out


@detector("obfuscated-class", "tag")
def _detect_obfuscated_class(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    for el in root.iter():
      cls = el.get("class") or ""
      obfuscated = [c for c in cls.split() if re.search(r"\bcalibre\d*\b", c)]
      if obfuscated:
        out.append({
          "kind": "obfuscated-class",
          "file": path,
          "locator": {"id": el.get("id") or ""},
          "params": {"mapping": {c: "" for c in obfuscated}},
          "lane": "tag",
          "auto_fixable": False,
          "confidence": "medium",
          "evidence": f"Found obfuscated class '{obfuscated[0]}' — target mapping requires human/AI judgment",
        })
        break
  return out


@detector("empty-paragraph", "tag")
def _detect_empty_paragraph(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    for el in root.iter():
      tag = el.tag.split("}")[-1] if "}" in el.tag else el.tag
      if tag == "p":
        text = "".join(el.itertext()).strip()
        if not text or text == " ":
          out.append({
            "kind": "empty-paragraph",
            "file": path,
            "locator": {"id": el.get("id") or ""},
            "params": {"rule": "empty-paragraph"},
            "lane": "tag",
            "auto_fixable": True,
            "confidence": "high",
            "evidence": "Empty paragraph element (no visible text content)",
          })
  return out


@detector("missing-manifest-properties", "package")
def _detect_missing_manifest_properties(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  if model.opf_root is None:
    return out
  manifest = model.opf_root.findall("opf:manifest/opf:item", OPF_NS)
  opf_dir = posixpath.dirname(model.opf_path)
  manifest_paths: dict[str, ET.Element] = {}
  for item in manifest:
    href = item.attrib.get("href", "")
    if href:
      manifest_paths[norm_join(opf_dir, href)] = item
  for path, root in model.xhtml_docs.items():
    item = manifest_paths.get(path)
    if item is None:
      continue
    props = split_props(item.attrib.get("properties"))
    serialized = ET.tostring(root, encoding="unicode")
    has_math = MATHML_URI in serialized or any(
      "}" in el.tag and "Math/MathML}" in el.tag for el in root.iter()
    )
    has_svg = SVG_URI in serialized or any(
      "}" in el.tag and "2000/svg}" in el.tag for el in root.iter()
    )
    if has_math and "mathml" not in props:
      out.append({
        "kind": "missing-manifest-properties",
        "file": path,
        "locator": {"manifest_id": item.attrib.get("id", "")},
        "params": {"properties": "mathml"},
        "lane": "package",
        "auto_fixable": True,
        "confidence": "high",
        "evidence": "XHTML contains MathML but manifest item lacks properties=\"mathml\"",
      })
    if has_svg and "svg" not in props:
      out.append({
        "kind": "missing-manifest-properties",
        "file": path,
        "locator": {"manifest_id": item.attrib.get("id", "")},
        "params": {"properties": "svg"},
        "lane": "package",
        "auto_fixable": True,
        "confidence": "high",
        "evidence": "XHTML contains SVG but manifest item lacks properties=\"svg\"",
      })
  return out


# ── Model builders ──────────────────────────────────────────────────────


def build_model(zf: zipfile.ZipFile, opf_path: str, opf_root: ET.Element) -> EpubModel:
  opf_dir = posixpath.dirname(opf_path)
  manifest = opf_root.findall("opf:manifest/opf:item", OPF_NS)
  names = set(zf.namelist())
  xhtml_docs: dict[str, ET.Element] = {}
  css_docs: dict[str, str] = {}
  for item in manifest:
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    target = norm_join(opf_dir, href)
    if target not in names:
      continue
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      try:
        root = ET.fromstring(zf.read(target))
        xhtml_docs[target] = root
      except ET.ParseError as exc:
        print(f"WARNING: cannot parse XHTML {target}: {exc}", file=sys.stderr)
    elif media_type == "text/css" or href.endswith(".css"):
      try:
        css_docs[target] = zf.read(target).decode("utf-8", errors="ignore")
      except Exception as exc:
        print(f"WARNING: cannot read CSS {target}: {exc}", file=sys.stderr)
  lang = opf_root.findtext(".//{http://purl.org/dc/elements/1.1/}language")
  return EpubModel(
    xhtml_docs=xhtml_docs,
    opf_root=opf_root,
    opf_path=opf_path,
    css_docs=css_docs,
    book_language=lang,
  )


def _build_model_from_tree(root_dir: Path, opf_path: str, opf_root: ET.Element) -> EpubModel:
  opf_dir = posixpath.dirname(opf_path)
  manifest = opf_root.findall("opf:manifest/opf:item", OPF_NS)
  xhtml_docs: dict[str, ET.Element] = {}
  css_docs: dict[str, str] = {}
  for item in manifest:
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    target = root_dir / norm_join(opf_dir, href)
    if not target.exists():
      continue
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      try:
        root = ET.parse(str(target)).getroot()
        xhtml_docs[norm_join(opf_dir, href)] = root
      except ET.ParseError as exc:
        print(f"WARNING: cannot parse XHTML {target}: {exc}", file=sys.stderr)
    elif media_type == "text/css" or href.endswith(".css"):
      try:
        css_docs[norm_join(opf_dir, href)] = target.read_text(encoding="utf-8", errors="ignore")
      except Exception as exc:
        print(f"WARNING: cannot read CSS {target}: {exc}", file=sys.stderr)
  lang = opf_root.findtext(".//{http://purl.org/dc/elements/1.1/}language")
  return EpubModel(
    xhtml_docs=xhtml_docs,
    opf_root=opf_root,
    opf_path=opf_path,
    css_docs=css_docs,
    book_language=lang,
  )


def split_props(value: str | None) -> set[str]:
  return set((value or "").split())


def norm_join(base: str, href: str) -> str:
  href = href.split("#", 1)[0]
  return posixpath.normpath(posixpath.join(base, unquote(href)))


def is_external_url(value: str) -> bool:
  return bool(re.match(r"^(?:[a-z][a-z0-9+.-]*:|//|#)", value, re.I))


def has_obfuscated_filename(href: str) -> bool:
  basename = posixpath.basename(unquote(href.split("#", 1)[0]))
  return bool(re.search(r"[\x00-\x1f\\:*?\"<>|]", basename))


def css_url_values(css: str) -> list[str]:
  active = re.sub(r"/\*.*?\*/", "", css, flags=re.S)
  urls: list[str] = []
  for match in re.finditer(r"url\(\s*(?P<url>[^)]+?)\s*\)", active, re.I):
    value = match.group("url").strip().strip("\"'")
    if value and not is_external_url(value):
      urls.append(value)
  return urls


def shell_path(path: Path) -> str:
  return shlex.quote(str(path))


def finding(level: str, message: str, path: str | None = None, kind: str | None = None) -> dict[str, str]:
  data = {"level": level, "message": message}
  if path:
    data["path"] = path
  if kind:
    data["kind"] = kind
  return data


class Report:
  def __init__(self, input_path: Path, workflow_mode: str = "build") -> None:
    self.input = str(input_path)
    self.mode = workflow_mode
    self.input_kind = "unknown"
    self.summary: dict[str, object] = {}
    self.findings: list[dict[str, str]] = []
    self.skills: list[str] = []
    self.skill_levels: dict[str, str] = {}
    self.commands: list[str] = []
    self.tools: dict[str, bool] = {}
    self.model: EpubModel | None = None

  def add_skill(self, name: str, level: str = "info") -> None:
    skill = f"${name}"
    if skill not in self.skills:
      self.skills.append(skill)
    current = self.skill_levels.get(skill)
    if current is None or LEVEL_ORDER[level] < LEVEL_ORDER[current]:
      self.skill_levels[skill] = level

  def add_command(self, command: str) -> None:
    if command not in self.commands:
      self.commands.append(command)

  def findings_by_level(self) -> dict[str, int]:
    counts = {"error": 0, "warn": 0, "info": 0}
    for item in self.findings:
      level = item.get("level", "info")
      counts[level] = counts.get(level, 0) + 1
    return counts

  def as_dict(self) -> dict[str, object]:
    return {
      "input": self.input,
      "mode": self.mode,
      "input_kind": self.input_kind,
      "summary": self.summary,
      "findings": self.findings,
      "findings_by_level": self.findings_by_level(),
      "recommended_skills": self.skills,
      "suggested_commands": self.commands,
      "tool_availability": self.tools,
      "actionable_findings": collect_actionable_findings(self.model),
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
  manifest_paths: set[str] = set()
  xhtml_items: list[ET.Element] = []
  css_items: list[ET.Element] = []
  image_items: list[ET.Element] = []
  xhtml_text_chars = 0
  xhtml_image_refs = 0
  obfuscated_filenames = 0

  for item in manifest:
    item_id = item.attrib.get("id")
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    if item_id and href:
      href_by_id[item_id] = href
    if href:
      manifest_paths.add(norm_join(opf_dir, href))
      if has_obfuscated_filename(href):
        obfuscated_filenames += 1
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      media_counts["xhtml"] += 1
      xhtml_items.append(item)
    elif media_type == "text/css" or href.endswith(".css"):
      media_counts["css"] += 1
      css_items.append(item)
    elif media_type.startswith("image/") or Path(href).suffix.lower() in IMAGE_EXTS:
      media_counts["images"] += 1
      image_items.append(item)
    elif media_type in FONT_MEDIA_TYPES or Path(href).suffix.lower() in FONT_EXTS:
      media_counts["fonts"] += 1
    else:
      media_counts["other"] += 1

  report.summary["manifest_items"] = len(manifest)
  report.summary["spine_items"] = len(spine)
  report.summary["media_counts"] = media_counts
  if obfuscated_filenames:
    report.summary["obfuscated_filenames"] = obfuscated_filenames
    report.findings.append(finding(
      "warn",
      "Manifest filenames contain decoded special characters; run structure normalization before richer cleanup",
      kind="filename-obfuscation",
    ))
    report.add_skill("epub-structure-normalizer", "warn")
    if report.input_kind == "existing-epub":
      report.add_command(
        f"python3 scripts/epub_structure_tool.py normalize {shell_path(Path(report.input))} "
        "--output work/after/step-0-normalized.epub --dry-run --report-format json"
      )
  package_version = root.attrib.get("version", "")
  if package_version:
    report.summary["package_version"] = package_version
  if package_version and not package_version.startswith("3"):
    report.findings.append(finding(
      "warn",
      "EPUB 2 package should be migrated to EPUB 3 before richer cleanup/features",
      kind="epub3-migration",
    ))
    report.add_skill("epub-package-nav-auditor", "warn")
    if report.input_kind == "existing-epub":
      report.add_command(
        f"python3 scripts/epub3_migration_harness.py {shell_path(Path(report.input))} "
        "--write-output work/after/step-1-epub3.epub --format json"
      )

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
    report.add_skill("epub-package-nav-auditor", level)

  ncx_items = [
    item for item in manifest
    if item.attrib.get("id") == "ncx" or item.attrib.get("media-type") == "application/x-dtbncx+xml"
  ]
  spine_root = root.find("opf:spine", OPF_NS)
  has_spine_ncx = spine_root is not None and bool(spine_root.attrib.get("toc"))
  if not ncx_items or not has_spine_ncx:
    report.findings.append(finding("warn", "Kindle/legacy delivery should keep toc.ncx and spine toc=\"ncx\""))
    report.add_skill("epub-kindle-compatibility-checker", "warn")
    report.add_skill("epub-package-nav-auditor", "warn")

  cover_items = [item for item in manifest if "cover-image" in split_props(item.attrib.get("properties"))]
  meta_cover = root.find(".//opf:meta[@name='cover']", OPF_NS)
  if not cover_items or meta_cover is None:
    report.findings.append(finding("warn", "Cover should have properties=\"cover-image\" and legacy meta name=\"cover\""))
    report.add_skill("epub-image-layout-optimizer", "warn")
    report.add_skill("epub-kindle-compatibility-checker", "warn")

  for item in manifest:
    href = item.attrib.get("href")
    if not href:
      report.findings.append(finding("error", "Manifest item missing href", item.attrib.get("id")))
      report.add_skill("epub-package-nav-auditor", "error")
      continue
    target = norm_join(opf_dir, href)
    if not exists(target):
      report.findings.append(finding("error", "Manifest href missing", target))
      report.add_skill("epub-package-nav-auditor", "error")

  for itemref in spine:
    idref = itemref.attrib.get("idref")
    if not idref or idref not in href_by_id:
      report.findings.append(finding("error", "Spine idref missing from manifest", idref or "<missing>"))
      report.add_skill("epub-package-nav-auditor", "error")

  for item in css_items:
    href = item.attrib.get("href", "")
    target = norm_join(opf_dir, href)
    if not href or not exists(target):
      continue
    css = read_bytes(target).decode("utf-8", errors="ignore")
    for raw_url in css_url_values(css):
      linked = norm_join(posixpath.dirname(target), raw_url)
      if not exists(linked):
        extension = Path(unquote(raw_url.split("#", 1)[0].split("?", 1)[0])).suffix.lower()
        if extension in FONT_EXTS:
          report.findings.append(finding(
            "warn",
            "CSS font url() target missing; preserve declaration for local() fallback and review manually",
            f"{href} -> {raw_url}",
            kind="missing-css-font-fallback",
          ))
          report.add_skill("epub-css-layering-optimizer", "warn")
          report.add_skill("epub-package-nav-auditor", "warn")
          report.add_skill("epub-typography-optimizer", "warn")
        else:
          report.findings.append(finding("error", "CSS url() target missing", f"{href} -> {raw_url}"))
          report.add_skill("epub-css-layering-optimizer", "error")
          report.add_skill("epub-package-nav-auditor", "error")
      elif linked not in manifest_paths:
        report.findings.append(finding("error", "CSS url() target missing from OPF manifest", f"{href} -> {raw_url}"))
        report.add_skill("epub-package-nav-auditor", "error")

  for item in image_items:
    href = item.attrib.get("href", "")
    ext = Path(href).suffix.lower()
    props = split_props(item.attrib.get("properties"))
    if ext == ".webp":
      report.findings.append(finding("warn", "WebP is not a Kindle main-path image format", href))
      report.add_skill("epub-kindle-compatibility-checker", "warn")
      report.add_skill("epub-image-layout-optimizer", "warn")
    elif ext == ".svg" and "cover-image" in props:
      report.findings.append(finding("warn", "SVG-only cover is risky for Kindle delivery", href))
      report.add_skill("epub-kindle-compatibility-checker", "warn")
      report.add_skill("epub-image-layout-optimizer", "warn")
    elif ext in {".tif", ".tiff", ".gif"}:
      report.findings.append(finding("warn", "Convert this image to JPEG/PNG for EPUB delivery", href))
      report.add_skill("epub-image-layout-optimizer", "warn")

  for item in xhtml_items:
    href = item.attrib.get("href", "")
    target = norm_join(opf_dir, href)
    if not exists(target):
      continue
    text = read_bytes(target).decode("utf-8", errors="ignore")
    text_without_tags = re.sub(r"<[^>]+>", "", text)
    xhtml_text_chars += len(re.sub(r"\s+", "", text_without_tags))
    xhtml_image_refs += len(re.findall(r"<img\b", text, flags=re.I))
    props = split_props(item.attrib.get("properties"))
    if re.search(r'\b(?:xml:)?lang=["\']en(?:[-_][A-Za-z0-9]+)?["\']', text):
      report.add_skill("epub-english-typography-optimizer")
    has_math = "<math" in text or MATHML_URI in text
    if has_math and "mathml" not in props:
      report.findings.append(finding("error", "MathML XHTML item missing properties=\"mathml\"", href))
      report.add_skill("epub-package-nav-auditor", "error")
      report.add_skill("epub-kindle-compatibility-checker", "error")
    has_svg = "<svg" in text or SVG_URI in text
    if has_svg and "svg" not in props:
      report.findings.append(finding("error", "Inline SVG XHTML item missing properties=\"svg\"", href))
      report.add_skill("epub-package-nav-auditor", "error")
    if "epub:type=\"noteref\"" in text or "epub:type='noteref'" in text:
      report.add_skill("epub-popup-footnote-converter")
      if "epub:type=\"footnote\"" not in text and "epub:type='footnote'" not in text:
        report.findings.append(finding("warn", "noteref found without same-file footnote aside", href))
        report.add_skill("epub-popup-footnote-converter", "warn")
    if "duokan-footnote" in text:
      report.add_skill("epub-legacy-footnote-fallback")
    if "writing-mode" in text or "page-vrl" in text or "<ruby" in text:
      report.add_skill("epub-vertical-ruby-optimizer")

  if xhtml_image_refs and xhtml_image_refs >= media_counts["xhtml"] and xhtml_text_chars < max(300, xhtml_image_refs * 120):
    report.findings.append(finding(
      "warn",
      "This EPUB appears to be OCR-derived or scan-heavy; cleanup is unlikely to help until source intake/OCR is revisited",
      kind="ocr-residual",
    ))
    report.add_skill("epub-source-intake", "warn")

  if media_counts["css"]:
    report.add_skill("epub-css-layering-optimizer")
  if media_counts["images"]:
    report.add_skill("epub-image-layout-optimizer")
  if media_counts["xhtml"] and not is_english:
    report.add_skill("epub-typography-optimizer")
  report.add_skill("epub-layout-auditor")
  report.add_skill("epub-package-nav-auditor")


def inspect_epub(path: Path, report: Report) -> None:
  report.input_kind = "existing-epub"
  mode_flag = " --mode cleanup" if report.mode == "cleanup" else ""
  report.add_command(f"python3 scripts/epub_preflight_harness.py {shell_path(path)} --format json")
  report.add_command(f"scripts/epub_ai_harness.py{mode_flag} {shell_path(path)}")
  report.add_command(f"python3 scripts/epub_refinement_harness.py {shell_path(path)} --format json")
  report.add_command(f"scripts/validate-popup-notes.sh --epub {shell_path(path)}")
  if report.mode == "cleanup":
    report.add_command(f"python3 scripts/validate_text_invariance.py {shell_path(path)} {shell_path(path)} --check all")
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
        report.add_skill("epub-package-nav-auditor", "error")
      if "META-INF/container.xml" not in names:
        report.findings.append(finding("error", "EPUB missing META-INF/container.xml"))
        report.add_skill("epub-package-nav-auditor", "error")
        return
      if "META-INF/encryption.xml" in names:
        report.findings.append(finding(
          "error",
          "EPUB has META-INF/encryption.xml; stop unless this is confirmed font obfuscation and explicitly allowed",
          "META-INF/encryption.xml",
          kind="drm",
        ))
      container = parse_xml_bytes(zf.read("META-INF/container.xml"), report, "META-INF/container.xml")
      if container is None:
        return
      rootfile = container.find(".//c:rootfile", CONTAINER_NS)
      opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
      if not opf_path or opf_path not in names:
        report.findings.append(finding("error", "container.xml rootfile does not resolve", opf_path or "<missing>"))
        report.add_skill("epub-package-nav-auditor", "error")
        return
      opf_root = parse_xml_bytes(zf.read(opf_path), report, opf_path)
      if opf_root is None:
        return
      opf_dir = posixpath.dirname(opf_path)
      report.summary["opf"] = opf_path
      inspect_opf(report, opf_root, opf_dir, lambda p: p in names, zf.read)
      report.model = build_model(zf, opf_path, opf_root)
  except zipfile.BadZipFile:
    report.findings.append(finding("error", "Input is not a valid zip/EPUB"))


def inspect_epub_tree(path: Path, report: Report, opf_path: Path) -> None:
  report.input_kind = "epub-source-tree"
  mode_flag = " --mode cleanup" if report.mode == "cleanup" else ""
  report.add_command(f"scripts/epub_ai_harness.py{mode_flag} {shell_path(path)}")
  if path.name == "epub-style-demo":
    report.add_command("sh templates/epub-style-demo/build.sh")
    report.add_command("scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub")
    report.add_command("scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub")
  try:
    opf_root = ET.parse(opf_path).getroot()
  except ET.ParseError as exc:
    report.findings.append(finding("error", f"OPF parse failed: {exc}", str(opf_path)))
    report.add_skill("epub-package-nav-auditor", "error")
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
  report.model = _build_model_from_tree(root_dir, rel_opf, opf_root)


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
  report.input_kind = "source-intake"
  report.add_skill("epub-source-intake")
  mode_flag = " --mode cleanup" if report.mode == "cleanup" else ""
  report.add_command(f"scripts/epub_ai_harness.py{mode_flag} {shell_path(path)}")
  suffix = path.suffix.lower()
  if suffix in PDF_EXTS:
    for tool in ("pdftotext", "mutool", "ocrmypdf", "tesseract"):
      report.tools[tool] = shutil.which(tool) is not None
    if not report.tools.get("pdftotext") and not report.tools.get("mutool"):
      report.findings.append(finding("warn", "No common PDF text extractor found in PATH; extraction is external to this repo"))
    report.findings.append(finding("warn", "PDF extraction requires sample-page proofreading before EPUB layout work"))
    report.add_skill("epub-source-intake", "warn")
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
  report.input_kind = "mixed-directory" if counts["epub"] else "source-intake"
  report.summary["file_counts"] = counts
  mode_flag = " --mode cleanup" if report.mode == "cleanup" else ""
  report.add_command(f"scripts/epub_ai_harness.py{mode_flag} {shell_path(path)}")
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


def apply_workflow_mode(report: Report) -> None:
  if report.mode != "cleanup":
    return
  report.skills = [skill for skill in report.skills if skill != "$epub-source-intake"]
  if report.input_kind not in {"existing-epub", "epub-source-tree"}:
    report.findings.append(finding("warn", "Cleanup mode expects an existing EPUB or EPUB source tree"))
  rest = [skill for skill in report.skills if skill != "$epub-layout-auditor"]
  original_order = {skill: index for index, skill in enumerate(report.skills)}
  rest.sort(key=lambda skill: (
    LEVEL_ORDER.get(report.skill_levels.get(skill, "info"), LEVEL_ORDER["info"]),
    original_order[skill],
  ))
  report.skills = ["$epub-layout-auditor", *rest]


def inspect_path(path: Path, workflow_mode: str = "build") -> Report:
  report = Report(path, workflow_mode)
  if not path.exists():
    report.input_kind = "missing"
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
  apply_workflow_mode(report)
  return report


def render_markdown(report: Report) -> str:
  data = report.as_dict()
  lines = [
    "# EPUB AI Harness Report",
    "",
    f"- Input: `{data['input']}`",
    f"- Mode: `{data['mode']}`",
    f"- Input kind: `{data['input_kind']}`",
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
  parser.add_argument("--mode", choices=("build", "cleanup"), default="build")
  parser.add_argument("path", type=Path, help="EPUB, EPUB source tree, PDF/text source, or mixed source directory")
  parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  report = inspect_path(args.path, args.mode)
  data = report.as_dict()
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(render_markdown(report), end="")
  has_error = any(item["level"] == "error" for item in report.findings)
  return 1 if has_error else 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
