#!/usr/bin/env python3
"""Conservative EPUB package operations.

This script borrows the useful shape of epub-gadget's merge/split and
metadata/cover tools, but keeps the implementation standard-library-only and
auditable for this repository:
- never edits the only source file;
- rewrites local references when package resources move;
- regenerates package/nav files instead of preserving stale navigation;
- emits structured reports for downstream validation.
"""

from __future__ import annotations

import argparse
import copy
import json
import posixpath
import re
import sys
import zipfile
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import quote, unquote, urlsplit, urlunsplit
from xml.etree import ElementTree as ET
from xml.sax.saxutils import escape, quoteattr


CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
OPF_URI = "http://www.idpf.org/2007/opf"
DC_URI = "http://purl.org/dc/elements/1.1/"
DCTERMS_URI = "http://purl.org/dc/terms/"
XHTML_URI = "http://www.w3.org/1999/xhtml"
OPS_URI = "http://www.idpf.org/2007/ops"
NCX_URI = "http://www.daisy.org/z3986/2005/ncx/"

OPF_NS = {"opf": OPF_URI, "dc": DC_URI}

ET.register_namespace("", OPF_URI)
ET.register_namespace("dc", DC_URI)
ET.register_namespace("dcterms", DCTERMS_URI)

URI_ATTRIBUTE_RE = re.compile(
  r"(?P<prefix>\b(?:href|src|poster|data|xlink:href|textref)\s*=\s*)"
  r"(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
  flags=re.IGNORECASE | re.DOTALL,
)
SRCSET_RE = re.compile(
  r"(?P<prefix>\bsrcset\s*=\s*)(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
  flags=re.IGNORECASE | re.DOTALL,
)
CSS_URL_RE = re.compile(
  r"(?P<prefix>\burl\(\s*)(?P<quote>[\"']?)(?P<uri>.*?)(?P=quote)(?P<suffix>\s*\))",
  flags=re.IGNORECASE | re.DOTALL,
)
CSS_IMPORT_RE = re.compile(
  r"(?P<prefix>@import\s+)(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
  flags=re.IGNORECASE | re.DOTALL,
)
SVG_BLOCK_RE = re.compile(
  r"(?P<open><svg\b[^>]*>)(?P<body>.*?</svg\s*>)",
  flags=re.IGNORECASE | re.DOTALL,
)
SVG_IMAGE_RE = re.compile(r"<image\b[^>]*>", flags=re.IGNORECASE | re.DOTALL)
MARKUP_EXTENSIONS = {".html", ".htm", ".xhtml", ".xml", ".ncx", ".svg", ".smil"}
IMAGE_MEDIA_BY_EXT = {
  ".gif": "image/gif",
  ".jpeg": "image/jpeg",
  ".jpg": "image/jpeg",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".webp": "image/webp",
}


class PackageToolError(Exception):
  """The EPUB cannot be changed conservatively."""


@dataclass
class ManifestItem:
  item_id: str
  href: str
  media_type: str
  properties: str
  archive_path: str


@dataclass
class SpineItem:
  idref: str
  linear: str = ""
  properties: str = ""


@dataclass
class Package:
  opf_path: str
  root: ET.Element
  manifest_items: list[ManifestItem]
  spine_items: list[SpineItem]
  toc_id: str = ""


@dataclass
class MetadataInfo:
  title: str = ""
  subtitle: str = ""
  author: str = ""
  language: str = "zh-CN"
  publisher: str = ""
  description: str = ""
  identifier: str = ""
  rights: str = ""
  cover_href: str = ""

  def as_dict(self) -> dict[str, str]:
    return asdict(self)


@dataclass
class OperationReport:
  operation: str
  input: str | None = None
  inputs: list[str] = field(default_factory=list)
  output: str | None = None
  outputs: list[str] = field(default_factory=list)
  opf: str = ""
  merged_items: int = 0
  renamed_resources: int = 0
  segments_created: int = 0
  fields_updated: int = 0
  cover_path: str = ""
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return asdict(self)


@dataclass
class TocEntry:
  title: str
  href: str
  level: int = 1


def q(uri: str, name: str) -> str:
  return f"{{{uri}}}{name}"


def local_name(tag: object) -> str:
  if not isinstance(tag, str):
    return ""
  return tag.rsplit("}", 1)[-1]


def split_props(value: str | None) -> list[str]:
  return [part for part in (value or "").split() if part]


def prop_text(value: list[str]) -> str:
  return " ".join(dict.fromkeys(value))


def add_prop(value: str, prop: str) -> str:
  props = split_props(value)
  if prop not in props:
    props.append(prop)
  return prop_text(props)


def remove_prop(value: str, prop: str) -> str:
  return prop_text([item for item in split_props(value) if item != prop])


def validate_archive_path(name: str, label: str) -> str:
  if not name or name.startswith("/"):
    raise PackageToolError(f"{label}: invalid absolute or empty ZIP path: {name!r}")
  normalized = posixpath.normpath(name)
  if normalized == ".." or normalized.startswith("../"):
    raise PackageToolError(f"{label}: ZIP path escapes archive root: {name!r}")
  return normalized


def quote_archive_path(value: str) -> str:
  return quote(value, safe="/:@-._~")


def is_external_uri(uri: str) -> bool:
  return bool(urlsplit(uri).scheme) or uri.startswith(("/", "//"))


def resolve_relative_path(base_file: str, uri_path: str) -> str:
  decoded = unquote(uri_path)
  return validate_archive_path(posixpath.join(posixpath.dirname(base_file), decoded), "resource href")


def relative_uri(from_archive_path: str, to_archive_path: str) -> str:
  base = posixpath.dirname(from_archive_path)
  relative = posixpath.relpath(to_archive_path, base) if base else to_archive_path
  return quote_archive_path(relative)


def href_with_fragment(base_file: str, href: str) -> str:
  path, sep, fragment = href.partition("#")
  if not path:
    return f"#{fragment}" if sep else ""
  resolved = resolve_relative_path(base_file, urlsplit(path).path)
  return f"{resolved}#{fragment}" if sep else resolved


def parse_xml(data: bytes | str, label: str) -> ET.Element:
  if isinstance(data, str):
    data = data.encode("utf-8")
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    raise PackageToolError(f"{label}: XML parse failed: {exc}") from exc


def find_child(root: ET.Element, wanted: str) -> ET.Element | None:
  for child in root:
    if local_name(child.tag) == wanted:
      return child
  return None


def read_archive(path: Path) -> tuple[dict[str, bytes], list[str]]:
  try:
    with zipfile.ZipFile(path) as zf:
      files: dict[str, bytes] = {}
      order: list[str] = []
      for info in zf.infolist():
        if info.is_dir():
          continue
        name = validate_archive_path(info.filename, "archive member")
        if name in files:
          raise PackageToolError(f"duplicate ZIP member: {name}")
        files[name] = zf.read(info.filename)
        order.append(name)
  except zipfile.BadZipFile as exc:
    raise PackageToolError(f"not a readable EPUB ZIP: {path}") from exc
  return files, order


def write_epub(output_path: Path, files: dict[str, bytes], order: list[str] | None = None) -> None:
  output_path.parent.mkdir(parents=True, exist_ok=True)
  written: set[str] = set()
  with zipfile.ZipFile(output_path, "w", zipfile.ZIP_DEFLATED) as zf:
    zf.writestr("mimetype", b"application/epub+zip", compress_type=zipfile.ZIP_STORED)
    written.add("mimetype")
    for name in order or []:
      if name in files and name not in written and name != "mimetype" and not is_macos_metadata_path(name):
        zf.writestr(name, files[name])
        written.add(name)
    for name in sorted(files):
      if name not in written and name != "mimetype" and not is_macos_metadata_path(name):
        zf.writestr(name, files[name])
        written.add(name)


def is_macos_metadata_path(name: str) -> bool:
  return posixpath.basename(name.rstrip("/")) == ".DS_Store"


def ensure_no_encryption(files: dict[str, bytes], action: str) -> None:
  if any(name.lower() == "meta-inf/encryption.xml" for name in files):
    raise PackageToolError(f"{action}: encrypted EPUB resources detected; refusing package rewrite")


def read_package(files: dict[str, bytes]) -> Package:
  container_path = "META-INF/container.xml"
  if container_path not in files:
    raise PackageToolError("missing META-INF/container.xml")
  container = parse_xml(files[container_path], container_path)
  rootfile = next((elem for elem in container.iter() if local_name(elem.tag) == "rootfile"), None)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else ""
  if not opf_path:
    raise PackageToolError("container.xml has no rootfile full-path")
  opf_path = validate_archive_path(opf_path, "container.xml rootfile")
  if opf_path not in files:
    raise PackageToolError(f"container.xml rootfile does not resolve: {opf_path}")

  root = parse_xml(files[opf_path], opf_path)
  manifest = find_child(root, "manifest")
  if manifest is None:
    raise PackageToolError(f"{opf_path}: OPF missing manifest")
  spine = find_child(root, "spine")
  if spine is None:
    raise PackageToolError(f"{opf_path}: OPF missing spine")

  manifest_items: list[ManifestItem] = []
  for item in manifest:
    if local_name(item.tag) != "item":
      continue
    item_id = item.attrib.get("id", "")
    href = item.attrib.get("href", "")
    if not item_id or not href:
      raise PackageToolError(f"{opf_path}: manifest item missing id or href")
    media_type = item.attrib.get("media-type", "application/octet-stream")
    properties = item.attrib.get("properties", "")
    if is_external_uri(href):
      continue
    archive_path = resolve_relative_path(opf_path, urlsplit(href).path)
    manifest_items.append(ManifestItem(item_id, href, media_type, properties, archive_path))

  spine_items: list[SpineItem] = []
  for itemref in spine:
    if local_name(itemref.tag) != "itemref":
      continue
    idref = itemref.attrib.get("idref", "")
    if idref:
      spine_items.append(SpineItem(idref, itemref.attrib.get("linear", ""), itemref.attrib.get("properties", "")))

  return Package(opf_path, root, manifest_items, spine_items, spine.attrib.get("toc", ""))


def package_title(package: Package) -> str:
  node = package.root.find("opf:metadata/dc:title", OPF_NS)
  return "".join(node.itertext()).strip() if node is not None else "Untitled"


def metadata_node(root: ET.Element) -> ET.Element:
  node = root.find("opf:metadata", OPF_NS)
  if node is None:
    raise PackageToolError("OPF missing metadata")
  return node


def manifest_node(root: ET.Element) -> ET.Element:
  node = root.find("opf:manifest", OPF_NS)
  if node is None:
    raise PackageToolError("OPF missing manifest")
  return node


def spine_node(root: ET.Element) -> ET.Element:
  node = root.find("opf:spine", OPF_NS)
  if node is None:
    raise PackageToolError("OPF missing spine")
  return node


def item_by_id(package: Package) -> dict[str, ManifestItem]:
  return {item.item_id: item for item in package.manifest_items}


def item_by_path(package: Package) -> dict[str, ManifestItem]:
  return {item.archive_path: item for item in package.manifest_items}


def unique_id(base: str, used: set[str]) -> str:
  candidate = re.sub(r"[^A-Za-z0-9_.-]+", "-", base).strip("-") or "item"
  if candidate[0].isdigit():
    candidate = f"x-{candidate}"
  result = candidate
  index = 2
  while result in used:
    result = f"{candidate}-{index}"
    index += 1
  used.add(result)
  return result


def prefixed_archive_path(path: str, prefix: str) -> str:
  folder = posixpath.dirname(path)
  basename = posixpath.basename(path)
  return posixpath.join(folder, f"{prefix}{basename}") if folder else f"{prefix}{basename}"


def allocate_archive_path(preferred: str, used: set[str], prefix: str) -> tuple[str, bool]:
  candidate = preferred
  renamed = False
  if candidate in used:
    candidate = prefixed_archive_path(preferred, prefix)
    renamed = True
  stem, ext = posixpath.splitext(candidate)
  index = 2
  while candidate in used:
    candidate = f"{stem}-{index}{ext}"
    renamed = True
    index += 1
  used.add(candidate)
  return candidate, renamed


def rewrite_uri(uri: str, old_document: str, new_document: str, path_map: dict[str, str], known_files: set[str]) -> str:
  if not uri or uri.startswith("#") or is_external_uri(uri):
    return uri
  parts = urlsplit(uri)
  if not parts.path:
    return uri
  try:
    old_target = resolve_relative_path(old_document, parts.path)
  except PackageToolError:
    return uri
  if old_target not in known_files:
    return uri
  target = path_map.get(old_target, old_target)
  path = relative_uri(new_document, target)
  return urlunsplit(("", "", path, parts.query, parts.fragment))


def split_srcset_candidates(value: str) -> list[str]:
  candidates: list[str] = []
  start = 0
  in_url = True
  for index, char in enumerate(value):
    if char.isspace() and value[start:index].strip():
      in_url = False
    elif char == ",":
      current = value[start:index].strip()
      current_url = current.split(None, 1)[0] if current else ""
      if in_url and current_url.lower().startswith("data:"):
        continue
      candidates.append(value[start:index])
      start = index + 1
      in_url = True
  candidates.append(value[start:])
  return candidates


def rewrite_srcset(text: str, old_document: str, new_document: str, path_map: dict[str, str], known_files: set[str]) -> str:
  def replace(match: re.Match[str]) -> str:
    candidates: list[str] = []
    for candidate in split_srcset_candidates(match.group("uri")):
      parts = candidate.strip().split()
      if not parts:
        continue
      url = rewrite_uri(parts[0], old_document, new_document, path_map, known_files)
      descriptor = " ".join(parts[1:])
      candidates.append(f"{url} {descriptor}".strip())
    return f"{match.group('prefix')}{match.group('quote')}{', '.join(candidates)}{match.group('quote')}"

  return SRCSET_RE.sub(replace, text)


def rewrite_text_references(text: str, old_document: str, new_document: str, path_map: dict[str, str], known_files: set[str]) -> str:
  def replace_attr(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, known_files)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}"

  def replace_url(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, known_files)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}{match.group('suffix')}"

  def replace_import(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, known_files)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}"

  text = rewrite_srcset(text, old_document, new_document, path_map, known_files)
  text = URI_ATTRIBUTE_RE.sub(replace_attr, text)
  text = CSS_URL_RE.sub(replace_url, text)
  return CSS_IMPORT_RE.sub(replace_import, text)


def transform_resource(data: bytes, old_path: str, new_path: str, path_map: dict[str, str], known_files: set[str]) -> bytes:
  ext = posixpath.splitext(old_path)[1].lower()
  if ext != ".css" and ext not in MARKUP_EXTENSIONS:
    return data
  try:
    text = data.decode("utf-8")
  except UnicodeDecodeError:
    return data
  return rewrite_text_references(text, old_path, new_path, path_map, known_files).encode("utf-8")


def cover_raster_dimensions(data: bytes) -> tuple[int, int] | None:
  """Return PNG/JPEG pixel dimensions without adding an image dependency."""
  if data.startswith(b"\x89PNG\r\n\x1a\n") and len(data) >= 24 and data[12:16] == b"IHDR":
    return int.from_bytes(data[16:20], "big"), int.from_bytes(data[20:24], "big")
  if not data.startswith(b"\xff\xd8"):
    return None
  index = 2
  while index + 9 <= len(data):
    if data[index] != 0xFF:
      index += 1
      continue
    marker = data[index + 1]
    if marker in {0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF}:
      return int.from_bytes(data[index + 7:index + 9], "big"), int.from_bytes(data[index + 5:index + 7], "big")
    if marker in {0xD8, 0xD9}:
      index += 2
      continue
    length = int.from_bytes(data[index + 2:index + 4], "big")
    if length < 2:
      return None
    index += 2 + length
  return None


def set_xml_tag_attribute(tag: str, name: str, value: str) -> str:
  pattern = re.compile(rf"(?P<prefix>\s{name}\s*=\s*)(?P<quote>[\"']).*?(?P=quote)", re.IGNORECASE | re.DOTALL)
  if pattern.search(tag):
    return pattern.sub(lambda match: f'{match.group("prefix")}{match.group("quote")}{value}{match.group("quote")}', tag, count=1)
  closing = "/>" if tag.rstrip().endswith("/>") else ">"
  return f'{tag[:-len(closing)]} {name}="{value}"{tag[-len(closing):]}'


def uri_targets_archive(uri: str, document_path: str, target_path: str) -> bool:
  if not uri or is_external_uri(uri):
    return False
  path = urlsplit(uri).path
  if not path:
    return False
  try:
    return resolve_relative_path(document_path, path) == target_path
  except PackageToolError:
    return False


def resize_svg_cover_pages(data: bytes, document_path: str, cover_path: str, dimensions: tuple[int, int] | None) -> bytes:
  """Keep inline SVG cover wrappers aligned to the replacement raster size."""
  if dimensions is None or posixpath.splitext(document_path)[1].lower() not in MARKUP_EXTENSIONS:
    return data
  try:
    text = data.decode("utf-8")
  except UnicodeDecodeError:
    return data
  width, height = dimensions

  def replace_svg(match: re.Match[str]) -> str:
    has_cover_image = False

    def replace_image(image_match: re.Match[str]) -> str:
      nonlocal has_cover_image
      tag = image_match.group(0)
      for uri_match in URI_ATTRIBUTE_RE.finditer(tag):
        if uri_targets_archive(uri_match.group("uri"), document_path, cover_path):
          has_cover_image = True
          return set_xml_tag_attribute(set_xml_tag_attribute(tag, "width", str(width)), "height", str(height))
      return tag

    body = SVG_IMAGE_RE.sub(replace_image, match.group("body"))
    if not has_cover_image:
      return match.group(0)
    opening = set_xml_tag_attribute(match.group("open"), "viewBox", f"0 0 {width} {height}")
    return opening + body

  return SVG_BLOCK_RE.sub(replace_svg, text).encode("utf-8")


def parse_toc_nav(files: dict[str, bytes], nav_path: str) -> list[TocEntry]:
  if nav_path not in files:
    return []
  root = parse_xml(files[nav_path], nav_path)
  entries: list[TocEntry] = []

  def is_toc_nav(elem: ET.Element) -> bool:
    epub_type = elem.attrib.get(q(OPS_URI, "type"), "") or elem.attrib.get("epub:type", "")
    return local_name(elem.tag) == "nav" and "toc" in epub_type.split()

  def find_toc(elem: ET.Element) -> ET.Element | None:
    if is_toc_nav(elem):
      return elem
    for child in elem:
      found = find_toc(child)
      if found is not None:
        return found
    return None

  def walk_list(elem: ET.Element, level: int) -> None:
    for child in elem:
      if local_name(child.tag) != "li":
        continue
      for grandchild in child:
        tag = local_name(grandchild.tag)
        if tag == "a":
          title = " ".join("".join(grandchild.itertext()).split())
          href = grandchild.attrib.get("href", "")
          entries.append(TocEntry(title, href_with_fragment(nav_path, href) if href else "", level))
        elif tag == "span":
          title = " ".join("".join(grandchild.itertext()).split())
          entries.append(TocEntry(title, "", level))
        elif tag == "ol":
          walk_list(grandchild, level + 1)

  toc = find_toc(root)
  if toc is not None:
    for child in toc:
      if local_name(child.tag) == "ol":
        walk_list(child, 1)
  return entries


def parse_toc_ncx(files: dict[str, bytes], ncx_path: str) -> list[TocEntry]:
  if ncx_path not in files:
    return []
  root = parse_xml(files[ncx_path], ncx_path)
  entries: list[TocEntry] = []

  def walk(elem: ET.Element, level: int) -> None:
    for child in elem:
      if local_name(child.tag) != "navPoint":
        continue
      title = ""
      href = ""
      for grandchild in child:
        if local_name(grandchild.tag) == "navLabel":
          title = " ".join("".join(grandchild.itertext()).split())
        elif local_name(grandchild.tag) == "content":
          src = grandchild.attrib.get("src", "")
          href = href_with_fragment(ncx_path, src) if src else ""
      entries.append(TocEntry(title, href, level))
      walk(child, level + 1)

  for child in root:
    if local_name(child.tag) == "navMap":
      walk(child, 1)
  return entries


def spine_toc_entries(package: Package) -> list[TocEntry]:
  by_id = item_by_id(package)
  entries: list[TocEntry] = []
  for spine_item in package.spine_items:
    item = by_id.get(spine_item.idref)
    if not item or "nav" in split_props(item.properties):
      continue
    if item.media_type == "application/xhtml+xml" or item.archive_path.lower().endswith((".xhtml", ".html")):
      entries.append(TocEntry(posixpath.basename(item.href), item.archive_path, 1))
  return entries


def parse_toc(files: dict[str, bytes], package: Package) -> list[TocEntry]:
  for item in package.manifest_items:
    if "nav" in split_props(item.properties):
      entries = parse_toc_nav(files, item.archive_path)
      if entries:
        return entries
  if package.toc_id:
    toc_item = item_by_id(package).get(package.toc_id)
    if toc_item:
      entries = parse_toc_ncx(files, toc_item.archive_path)
      if entries:
        return entries
  return spine_toc_entries(package)


def build_container(opf_path: str) -> bytes:
  root = ET.Element(q(CONTAINER_URI, "container"), {"version": "1.0"})
  rootfiles = ET.SubElement(root, q(CONTAINER_URI, "rootfiles"))
  ET.SubElement(
    rootfiles,
    q(CONTAINER_URI, "rootfile"),
    {"full-path": opf_path, "media-type": "application/oebps-package+xml"},
  )
  return ET.tostring(root, encoding="utf-8", xml_declaration=True)


def build_nav(title: str, entries_by_group: list[tuple[str, list[TocEntry]]], nav_path: str, path_map: dict[str, str]) -> bytes:
  lines = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">',
    "<head>",
    f"  <title>{escape(title)}</title>",
    "</head>",
    "<body>",
    '<nav epub:type="toc" id="toc">',
    f"  <h1>{escape(title)}</h1>",
    "  <ol>",
  ]

  def append_entry(entry: TocEntry, indent: str) -> None:
    if entry.href:
      href, sep, fragment = entry.href.partition("#")
      target = path_map.get(href, href)
      rendered_href = relative_uri(nav_path, target) + (f"#{fragment}" if sep else "")
      lines.append(f"{indent}<li><a href={quoteattr(rendered_href)}>{escape(entry.title or posixpath.basename(target))}</a></li>")
    else:
      lines.append(f"{indent}<li><span>{escape(entry.title)}</span></li>")

  for group_title, entries in entries_by_group:
    if len(entries_by_group) > 1:
      lines.append("    <li>")
      lines.append(f"      <span>{escape(group_title)}</span>")
      lines.append("      <ol>")
      for entry in entries:
        append_entry(entry, "        ")
      lines.append("      </ol>")
      lines.append("    </li>")
    else:
      for entry in entries:
        append_entry(entry, "    ")
  lines.extend(["  </ol>", "</nav>", "</body>", "</html>"])
  return ("\n".join(lines) + "\n").encode("utf-8")


def build_ncx(title: str, entries_by_group: list[tuple[str, list[TocEntry]]], ncx_path: str, path_map: dict[str, str]) -> bytes:
  lines = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">',
    '<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">',
    "  <head>",
    '    <meta name="dtb:uid" content="epub-package-tool"/>',
    '    <meta name="dtb:depth" content="1"/>',
    '    <meta name="dtb:totalPageCount" content="0"/>',
    '    <meta name="dtb:maxPageNumber" content="0"/>',
    "  </head>",
    f"  <docTitle><text>{escape(title)}</text></docTitle>",
    "  <navMap>",
  ]
  play_order = 1
  for _group_title, entries in entries_by_group:
    for entry in entries:
      if not entry.href:
        continue
      href, sep, fragment = entry.href.partition("#")
      target = path_map.get(href, href)
      rendered_href = relative_uri(ncx_path, target) + (f"#{fragment}" if sep else "")
      lines.append(f'    <navPoint id="navPoint-{play_order}" playOrder="{play_order}">')
      lines.append(f"      <navLabel><text>{escape(entry.title or posixpath.basename(target))}</text></navLabel>")
      lines.append(f"      <content src={quoteattr(rendered_href)}/>")
      lines.append("    </navPoint>")
      play_order += 1
  lines.extend(["  </navMap>", "</ncx>"])
  return ("\n".join(lines) + "\n").encode("utf-8")


def build_opf(
  title: str,
  opf_path: str,
  metadata_source: ET.Element | None,
  manifest_items: list[tuple[str, str, str, str]],
  spine_items: list[SpineItem],
) -> bytes:
  package = ET.Element(
    q(OPF_URI, "package"),
    {
      "version": "3.0",
      "unique-identifier": "book-id",
      "prefix": "dcterms: http://purl.org/dc/terms/",
    },
  )
  metadata = ET.SubElement(package, q(OPF_URI, "metadata"))
  identifier = ET.SubElement(metadata, q(DC_URI, "identifier"), {"id": "book-id"})
  identifier.text = "urn:uuid:epub-package-tool"
  title_elem = ET.SubElement(metadata, q(DC_URI, "title"))
  title_elem.text = title
  language = ET.SubElement(metadata, q(DC_URI, "language"))
  language.text = "zh-CN"
  if metadata_source is not None:
    for tag in ("creator", "publisher", "description", "rights"):
      source = metadata_source.find(f"dc:{tag}", OPF_NS)
      if source is not None and "".join(source.itertext()).strip():
        elem = ET.SubElement(metadata, q(DC_URI, tag))
        elem.text = "".join(source.itertext()).strip()
  modified = ET.SubElement(metadata, q(OPF_URI, "meta"), {"property": "dcterms:modified"})
  modified.text = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
  cover_id = next((item_id for item_id, _href, _media_type, props in manifest_items if "cover-image" in split_props(props)), "")
  if cover_id:
    ET.SubElement(metadata, q(OPF_URI, "meta"), {"name": "cover", "content": cover_id})

  manifest = ET.SubElement(package, q(OPF_URI, "manifest"))
  ET.SubElement(manifest, q(OPF_URI, "item"), {"id": "nav", "href": "nav.xhtml", "media-type": "application/xhtml+xml", "properties": "nav"})
  ET.SubElement(manifest, q(OPF_URI, "item"), {"id": "ncx", "href": "toc.ncx", "media-type": "application/x-dtbncx+xml"})
  for item_id, href, media_type, properties in manifest_items:
    attrs = {"id": item_id, "href": href, "media-type": media_type}
    if properties:
      attrs["properties"] = properties
    ET.SubElement(manifest, q(OPF_URI, "item"), attrs)

  spine = ET.SubElement(package, q(OPF_URI, "spine"), {"toc": "ncx"})
  ET.SubElement(spine, q(OPF_URI, "itemref"), {"idref": "nav", "linear": "no"})
  for item in spine_items:
    attrs = {"idref": item.idref}
    if item.linear:
      attrs["linear"] = item.linear
    if item.properties:
      attrs["properties"] = item.properties
    ET.SubElement(spine, q(OPF_URI, "itemref"), attrs)

  return ET.tostring(package, encoding="utf-8", xml_declaration=True)


def merge_epubs(input_paths: list[Path], output_path: Path, title: str | None = None) -> OperationReport:
  if len(input_paths) < 2:
    raise PackageToolError("merge requires at least two input EPUB files")
  report = OperationReport("merge", inputs=[str(path) for path in input_paths], output=str(output_path), opf="OEBPS/content.opf")
  used_paths = {"META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/toc.ncx"}
  used_ids = {"nav", "ncx"}
  output_files: dict[str, bytes] = {
    "META-INF/container.xml": build_container("OEBPS/content.opf"),
  }
  merged_manifest_items: list[tuple[str, str, str, str]] = []
  merged_spine_items: list[SpineItem] = []
  nav_groups: list[tuple[str, list[TocEntry]]] = []
  first_metadata: ET.Element | None = None
  merged_title = title

  for volume_index, input_path in enumerate(input_paths, start=1):
    files, _order = read_archive(input_path)
    ensure_no_encryption(files, "merge")
    package = read_package(files)
    if first_metadata is None:
      first_metadata = copy.deepcopy(metadata_node(package.root))
    if merged_title is None:
      merged_title = package_title(package)
    path_map: dict[str, str] = {}
    id_map: dict[str, str] = {}
    prefix = f"vol{volume_index}_"
    by_id = item_by_id(package)

    for item in package.manifest_items:
      if item.archive_path not in files:
        report.warnings.append(f"{input_path}: manifest href does not resolve: {item.href}")
        continue
      if "nav" in split_props(item.properties) or item.media_type == "application/x-dtbncx+xml":
        continue
      final_path, renamed = allocate_archive_path(item.archive_path, used_paths, prefix)
      path_map[item.archive_path] = final_path
      report.renamed_resources += int(renamed)
      base_id = item.item_id if item.item_id not in used_ids else f"vol{volume_index}_{item.item_id}"
      new_id = unique_id(base_id, used_ids)
      id_map[item.item_id] = new_id
      props = remove_prop(item.properties, "nav")
      merged_manifest_items.append((new_id, relative_uri("OEBPS/content.opf", final_path), item.media_type, props))
      report.merged_items += 1

    known_files = set(files)
    for item in package.manifest_items:
      final_path = path_map.get(item.archive_path)
      if not final_path:
        continue
      output_files[final_path] = transform_resource(files[item.archive_path], item.archive_path, final_path, path_map, known_files)

    for spine_item in package.spine_items:
      source_item = by_id.get(spine_item.idref)
      if not source_item or source_item.item_id not in id_map:
        continue
      if "nav" in split_props(source_item.properties):
        continue
      merged_spine_items.append(SpineItem(id_map[source_item.item_id], spine_item.linear, spine_item.properties))

    entries: list[TocEntry] = []
    for entry in parse_toc(files, package):
      if not entry.href:
        entries.append(entry)
        continue
      href, sep, fragment = entry.href.partition("#")
      if href in path_map:
        entries.append(TocEntry(entry.title, path_map[href] + (f"#{fragment}" if sep else ""), entry.level))
    if not entries:
      for spine_item in package.spine_items:
        source_item = by_id.get(spine_item.idref)
        if source_item and source_item.archive_path in path_map:
          entries.append(TocEntry(posixpath.basename(source_item.href), path_map[source_item.archive_path], 1))
    nav_groups.append((package_title(package), entries))

  output_files["OEBPS/content.opf"] = build_opf(
    merged_title or "Merged EPUB",
    "OEBPS/content.opf",
    first_metadata,
    merged_manifest_items,
    merged_spine_items,
  )
  nav_path_map = {path: path for path in output_files}
  output_files["OEBPS/nav.xhtml"] = build_nav(merged_title or "Merged EPUB", nav_groups, "OEBPS/nav.xhtml", nav_path_map)
  output_files["OEBPS/toc.ncx"] = build_ncx(merged_title or "Merged EPUB", nav_groups, "OEBPS/toc.ncx", nav_path_map)
  write_epub(output_path, output_files, ["mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/toc.ncx"])
  return report


def content_spine_paths(package: Package) -> list[str]:
  by_id = item_by_id(package)
  paths: list[str] = []
  for spine_item in package.spine_items:
    item = by_id.get(spine_item.idref)
    if not item or "nav" in split_props(item.properties):
      continue
    if item.media_type == "application/xhtml+xml" or item.archive_path.lower().endswith((".xhtml", ".html")):
      paths.append(item.archive_path)
  return paths


def list_split_targets(input_path: Path) -> list[dict[str, object]]:
  files, _order = read_archive(input_path)
  package = read_package(files)
  return [asdict(entry) for entry in parse_toc(files, package)]


def collect_referenced_resources(files: dict[str, bytes], package: Package, content_paths: set[str]) -> set[str]:
  known = item_by_path(package)
  referenced: set[str] = set()
  queue = list(content_paths)
  scanned: set[str] = set()
  while queue:
    current = queue.pop()
    if current in scanned or current not in files:
      continue
    scanned.add(current)
    ext = posixpath.splitext(current)[1].lower()
    if ext != ".css" and ext not in MARKUP_EXTENSIONS:
      continue
    try:
      text = files[current].decode("utf-8")
    except UnicodeDecodeError:
      continue

    raw_refs = [match.group("uri") for match in URI_ATTRIBUTE_RE.finditer(text)]
    raw_refs.extend(match.group("uri") for match in CSS_URL_RE.finditer(text))
    raw_refs.extend(match.group("uri") for match in CSS_IMPORT_RE.finditer(text))
    for raw in raw_refs:
      if not raw or raw.startswith("#") or is_external_uri(raw):
        continue
      try:
        target = resolve_relative_path(current, urlsplit(raw).path)
      except PackageToolError:
        continue
      if target in known and target not in content_paths and target not in referenced:
        referenced.add(target)
        queue.append(target)
  return referenced


def build_split_package(
  source_package: Package,
  selected_paths: list[str],
  resource_paths: set[str],
  toc_entries: list[TocEntry],
  opf_path: str,
  nav_path: str,
  ncx_path: str,
) -> bytes:
  source_root = source_package.root
  package = ET.Element(q(OPF_URI, "package"), {"version": source_root.attrib.get("version", "3.0"), "unique-identifier": source_root.attrib.get("unique-identifier", "book-id")})
  package.append(copy.deepcopy(metadata_node(source_root)))
  manifest = ET.SubElement(package, q(OPF_URI, "manifest"))
  by_path = item_by_path(source_package)
  by_id = item_by_id(source_package)
  added_ids: set[str] = set()
  for path in list(selected_paths) + sorted(resource_paths):
    item = by_path.get(path)
    if not item or item.item_id in added_ids:
      continue
    props = remove_prop(item.properties, "nav")
    attrs = {"id": item.item_id, "href": relative_uri(opf_path, path), "media-type": item.media_type}
    if props:
      attrs["properties"] = props
    ET.SubElement(manifest, q(OPF_URI, "item"), attrs)
    added_ids.add(item.item_id)
  nav_id = unique_id("nav", added_ids)
  ET.SubElement(manifest, q(OPF_URI, "item"), {"id": nav_id, "href": relative_uri(opf_path, nav_path), "media-type": "application/xhtml+xml", "properties": "nav"})
  ncx_id = unique_id("ncx", added_ids)
  ET.SubElement(manifest, q(OPF_URI, "item"), {"id": ncx_id, "href": relative_uri(opf_path, ncx_path), "media-type": "application/x-dtbncx+xml"})

  spine = ET.SubElement(package, q(OPF_URI, "spine"), {"toc": ncx_id})
  ET.SubElement(spine, q(OPF_URI, "itemref"), {"idref": nav_id, "linear": "no"})
  selected_set = set(selected_paths)
  for spine_item in source_package.spine_items:
    item = by_id.get(spine_item.idref)
    if item and item.archive_path in selected_set:
      attrs = {"idref": spine_item.idref}
      if spine_item.linear:
        attrs["linear"] = spine_item.linear
      if spine_item.properties:
        attrs["properties"] = spine_item.properties
      ET.SubElement(spine, q(OPF_URI, "itemref"), attrs)
  return ET.tostring(package, encoding="utf-8", xml_declaration=True)


def split_epub(input_path: Path, output_dir: Path, split_points: list[int]) -> OperationReport:
  files, _order = read_archive(input_path)
  ensure_no_encryption(files, "split")
  package = read_package(files)
  targets = parse_toc(files, package)
  spine_paths = content_spine_paths(package)
  if not targets:
    targets = [TocEntry(posixpath.basename(path), path, 1) for path in spine_paths]
  if not targets or not spine_paths:
    raise PackageToolError("split: EPUB has no splittable spine content")
  if not split_points:
    raise PackageToolError("split: at least one split point is required")

  sorted_points = sorted(set(split_points))
  for point in sorted_points:
    if point < 0 or point >= len(targets):
      raise PackageToolError(f"split point out of range: {point}")

  target_spine_indices: list[int] = []
  for target in targets:
    href = target.href.split("#", 1)[0]
    target_spine_indices.append(spine_paths.index(href) if href in spine_paths else -1)

  ranges: list[tuple[int, int]] = []
  for index, point in enumerate(sorted_points):
    start = target_spine_indices[point]
    if start < 0:
      start = next((target_spine_indices[i] for i in range(point, len(targets)) if target_spine_indices[i] >= 0), 0)
    if index + 1 < len(sorted_points):
      next_point = sorted_points[index + 1]
      end = target_spine_indices[next_point]
      if end < 0:
        end = next((target_spine_indices[i] for i in range(next_point, len(targets)) if target_spine_indices[i] >= 0), len(spine_paths))
    else:
      end = len(spine_paths)
    if start < end:
      ranges.append((start, end))

  report = OperationReport("split", input=str(input_path), opf=package.opf_path)
  basename = input_path.stem
  output_dir.mkdir(parents=True, exist_ok=True)
  for segment_index, (start, end) in enumerate(ranges, start=1):
    selected = spine_paths[start:end]
    selected_set = set(selected)
    resources = collect_referenced_resources(files, package, selected_set)
    nav_path = posixpath.join(posixpath.dirname(package.opf_path), "nav.xhtml")
    ncx_path = posixpath.join(posixpath.dirname(package.opf_path), "toc.ncx")
    segment_toc = [
      entry
      for entry in targets
      if not entry.href or entry.href.split("#", 1)[0] in selected_set
    ]
    path_identity = {path: path for path in files}
    segment_files: dict[str, bytes] = {
      "META-INF/container.xml": build_container(package.opf_path),
      package.opf_path: build_split_package(package, selected, resources, segment_toc, package.opf_path, nav_path, ncx_path),
      nav_path: build_nav(package_title(package), [(package_title(package), segment_toc)], nav_path, path_identity),
      ncx_path: build_ncx(package_title(package), [(package_title(package), segment_toc)], ncx_path, path_identity),
    }
    for path in selected_set | resources:
      if path in files:
        segment_files[path] = files[path]
    output_path = output_dir / f"{basename}_{segment_index:02d}.epub"
    write_epub(output_path, segment_files, ["mimetype", "META-INF/container.xml", package.opf_path, nav_path, ncx_path])
    report.outputs.append(str(output_path))
    report.segments_created += 1
  return report


def read_metadata(input_path: Path) -> MetadataInfo:
  files, _order = read_archive(input_path)
  package = read_package(files)
  root = package.root
  info = MetadataInfo()
  meta = metadata_node(root)

  titles = meta.findall("dc:title", OPF_NS)
  for title_elem in titles:
    text = "".join(title_elem.itertext()).strip()
    title_id = title_elem.attrib.get("id", "")
    title_type = ""
    if title_id:
      refine = meta.find(f'opf:meta[@refines="#{title_id}"][@property="title-type"]', OPF_NS)
      title_type = "".join(refine.itertext()).strip() if refine is not None else ""
    if title_type == "subtitle":
      info.subtitle = text
    elif title_type == "main" or not info.title:
      info.title = text

  def text(tag: str) -> str:
    elem = meta.find(f"dc:{tag}", OPF_NS)
    return "".join(elem.itertext()).strip() if elem is not None else ""

  info.author = text("creator")
  info.language = text("language") or "zh-CN"
  info.publisher = text("publisher")
  info.description = text("description")
  info.identifier = text("identifier")
  info.rights = text("rights")

  cover_id = ""
  cover_meta = meta.find('opf:meta[@name="cover"]', OPF_NS)
  if cover_meta is not None:
    cover_id = cover_meta.attrib.get("content", "")
  for item in package.manifest_items:
    if item.item_id == cover_id or "cover-image" in split_props(item.properties):
      info.cover_href = item.href
      break
  return info


def set_dc_text(meta: ET.Element, tag: str, value: str) -> bool:
  elem = meta.find(f"dc:{tag}", OPF_NS)
  if elem is None:
    elem = ET.SubElement(meta, q(DC_URI, tag))
  before = elem.text or ""
  elem.text = value
  return before != value


def remove_title_type_meta(meta: ET.Element, title_id: str) -> None:
  for elem in list(meta.findall(f'opf:meta[@refines="#{title_id}"][@property="title-type"]', OPF_NS)):
    meta.remove(elem)


def set_titles(meta: ET.Element, title: str | None, subtitle: str | None, update_subtitle: bool) -> int:
  changed = 0
  titles = meta.findall("dc:title", OPF_NS)
  main = titles[0] if titles else ET.SubElement(meta, q(DC_URI, "title"))
  if not main.attrib.get("id"):
    main.set("id", "main-title")
  if title is not None and (main.text or "") != title:
    main.text = title
    changed += 1
  remove_title_type_meta(meta, main.attrib["id"])
  main_meta = ET.SubElement(meta, q(OPF_URI, "meta"), {"refines": f"#{main.attrib['id']}", "property": "title-type"})
  main_meta.text = "main"

  if update_subtitle:
    for title_elem in titles[1:]:
      title_id = title_elem.attrib.get("id", "")
      if title_id:
        refine = meta.find(f'opf:meta[@refines="#{title_id}"][@property="title-type"]', OPF_NS)
        if refine is not None and "".join(refine.itertext()).strip() == "subtitle":
          meta.remove(title_elem)
          meta.remove(refine)

    if subtitle:
      subtitle_elem = ET.SubElement(meta, q(DC_URI, "title"), {"id": "subtitle"})
      subtitle_elem.text = subtitle
      subtitle_meta = ET.SubElement(meta, q(OPF_URI, "meta"), {"refines": "#subtitle", "property": "title-type"})
      subtitle_meta.text = "subtitle"
      changed += 1
  return changed


def write_metadata(input_path: Path, output_path: Path, metadata: dict[str, str]) -> OperationReport:
  files, order = read_archive(input_path)
  ensure_no_encryption(files, "metadata-write")
  package = read_package(files)
  root = copy.deepcopy(package.root)
  meta = metadata_node(root)
  report = OperationReport("metadata-write", input=str(input_path), output=str(output_path), opf=package.opf_path)
  if "title" in metadata or "subtitle" in metadata:
    report.fields_updated += set_titles(
      meta,
      metadata.get("title"),
      metadata.get("subtitle"),
      update_subtitle="subtitle" in metadata,
    )
  field_map = {
    "author": "creator",
    "language": "language",
    "publisher": "publisher",
    "description": "description",
    "identifier": "identifier",
    "rights": "rights",
  }
  for key, tag in field_map.items():
    if key in metadata:
      report.fields_updated += int(set_dc_text(meta, tag, metadata[key]))
  updated = dict(files)
  updated[package.opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, updated, order)
  return report


def media_type_for_cover(path: Path) -> str:
  ext = path.suffix.lower()
  if ext == ".jpeg":
    return "image/jpeg"
  return IMAGE_MEDIA_BY_EXT.get(ext, "image/jpeg")


def cover_item_id(root: ET.Element) -> str:
  meta = metadata_node(root)
  cover_meta = meta.find('opf:meta[@name="cover"]', OPF_NS)
  if cover_meta is not None and cover_meta.attrib.get("content"):
    return cover_meta.attrib["content"]
  for item in manifest_node(root).findall("opf:item", OPF_NS):
    if "cover-image" in split_props(item.attrib.get("properties")):
      return item.attrib.get("id", "cover-image")
  return "cover-image"


def replace_cover(input_path: Path, output_path: Path, cover_path: Path) -> OperationReport:
  files, order = read_archive(input_path)
  ensure_no_encryption(files, "replace-cover")
  if not cover_path.exists():
    raise PackageToolError(f"cover image not found: {cover_path}")
  package = read_package(files)
  root = copy.deepcopy(package.root)
  manifest = manifest_node(root)
  meta = metadata_node(root)
  cover_id = cover_item_id(root)
  opf_dir = posixpath.dirname(package.opf_path)
  ext = cover_path.suffix.lower() or ".jpg"
  if ext == ".jpeg":
    ext = ".jpg"
  new_rel_href = f"Images/cover{ext}"
  new_archive_path = validate_archive_path(posixpath.join(opf_dir, new_rel_href), "cover output")
  media_type = media_type_for_cover(cover_path)
  cover_data = cover_path.read_bytes()
  cover_dimensions = cover_raster_dimensions(cover_data)
  updated = dict(files)

  old_cover_paths: set[str] = set()
  cover_item = None
  for item in list(manifest.findall("opf:item", OPF_NS)):
    item_id = item.attrib.get("id", "")
    href = item.attrib.get("href", "")
    props = split_props(item.attrib.get("properties"))
    if item_id == cover_id:
      cover_item = item
      if href and not is_external_uri(href):
        old_cover_paths.add(resolve_relative_path(package.opf_path, urlsplit(href).path))
    elif "cover-image" in props or ("cover" in href.lower() and href.lower().endswith((".jpg", ".jpeg", ".png", ".webp", ".gif"))):
      if href and not is_external_uri(href):
        old_cover_paths.add(resolve_relative_path(package.opf_path, urlsplit(href).path))
      manifest.remove(item)

  if cover_item is None:
    cover_item = ET.SubElement(manifest, q(OPF_URI, "item"), {"id": cover_id})
  cover_item.set("href", new_rel_href)
  cover_item.set("media-type", media_type)
  cover_item.set("properties", add_prop(cover_item.attrib.get("properties", ""), "cover-image"))

  for old_path in old_cover_paths:
    if old_path != new_archive_path:
      updated.pop(old_path, None)
  updated[new_archive_path] = cover_data
  if old_cover_paths:
    path_map = {old_path: new_archive_path for old_path in old_cover_paths}
    known_files = set(files)
    for name, data in list(updated.items()):
      if name == package.opf_path or name == new_archive_path:
        continue
      transformed = transform_resource(data, name, name, path_map, known_files)
      updated[name] = resize_svg_cover_pages(transformed, name, new_archive_path, cover_dimensions)

  for elem in list(meta.findall('opf:meta[@name="cover"]', OPF_NS)):
    meta.remove(elem)
  ET.SubElement(meta, q(OPF_URI, "meta"), {"name": "cover", "content": cover_id})
  updated[package.opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, updated, order)
  return OperationReport("replace-cover", input=str(input_path), output=str(output_path), opf=package.opf_path, cover_path=new_archive_path)


def print_json(value: object) -> None:
  if hasattr(value, "as_dict"):
    value = value.as_dict()
  print(json.dumps(value, ensure_ascii=False, indent=2))


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Conservative EPUB package merge/split/metadata/cover operations")
  sub = parser.add_subparsers(dest="command", required=True)

  merge_parser = sub.add_parser("merge", help="Merge multiple EPUB files")
  merge_parser.add_argument("inputs", nargs="+", type=Path)
  merge_parser.add_argument("--output", required=True, type=Path)
  merge_parser.add_argument("--title")

  split_targets_parser = sub.add_parser("split-targets", help="List split targets")
  split_targets_parser.add_argument("input", type=Path)

  split_parser = sub.add_parser("split", help="Split one EPUB at TOC target indices")
  split_parser.add_argument("input", type=Path)
  split_parser.add_argument("--output-dir", required=True, type=Path)
  split_parser.add_argument("--split-points", required=True, help="Comma-separated target indices")

  read_meta_parser = sub.add_parser("metadata-read", help="Read EPUB metadata")
  read_meta_parser.add_argument("input", type=Path)

  write_meta_parser = sub.add_parser("metadata-write", help="Write EPUB metadata")
  write_meta_parser.add_argument("input", type=Path)
  write_meta_parser.add_argument("--output", required=True, type=Path)
  write_meta_parser.add_argument("--metadata-json", required=True, help="JSON object with title/author/etc.")

  cover_parser = sub.add_parser("replace-cover", help="Replace EPUB cover image")
  cover_parser.add_argument("input", type=Path)
  cover_parser.add_argument("--output", required=True, type=Path)
  cover_parser.add_argument("--cover", required=True, type=Path)

  args = parser.parse_args(argv)
  try:
    if args.command == "merge":
      print_json(merge_epubs(args.inputs, args.output, title=args.title))
    elif args.command == "split-targets":
      print_json(list_split_targets(args.input))
    elif args.command == "split":
      points = [int(item) for item in args.split_points.split(",") if item.strip()]
      print_json(split_epub(args.input, args.output_dir, points))
    elif args.command == "metadata-read":
      print_json(read_metadata(args.input))
    elif args.command == "metadata-write":
      print_json(write_metadata(args.input, args.output, json.loads(args.metadata_json)))
    elif args.command == "replace-cover":
      print_json(replace_cover(args.input, args.output, args.cover))
    return 0
  except (PackageToolError, ValueError, json.JSONDecodeError) as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
