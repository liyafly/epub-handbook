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
from dataclasses import asdict
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit
from xml.etree import ElementTree as ET
from xml.sax.saxutils import escape, quoteattr

from epub_lib import (
  EpubLibError,
  find_child,
  is_external_uri,
  is_macos_metadata_path,
  local_name,
  q,
  quote_archive_path,
  read_epub_archive,
  resolve_relative_path as _resolve_relative_path,
  split_props,
  validate_archive_path as _validate_archive_path,
)
from .models import (
  ManifestItem,
  MetadataInfo,
  OperationReport,
  Package,
  PackageToolError,
  SpineItem,
  TocEntry,
)


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
  try:
    return _validate_archive_path(name, label)
  except EpubLibError as exc:
    raise PackageToolError(str(exc)) from exc


def resolve_relative_path(base_file: str, uri_path: str) -> str:
  try:
    return _resolve_relative_path(base_file, uri_path)
  except EpubLibError as exc:
    raise PackageToolError(str(exc)) from exc


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


def read_archive(path: Path) -> tuple[dict[str, bytes], list[str]]:
  try:
    return read_epub_archive(path)
  except EpubLibError as exc:
    raise PackageToolError(str(exc)) from exc


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
