"""EPUB split operation."""

from __future__ import annotations

import copy
import posixpath
from dataclasses import asdict
from pathlib import Path
from urllib.parse import urlsplit
from xml.etree import ElementTree as ET

from epub_lib import is_external_uri, q, split_props

from .core import (
  CSS_IMPORT_RE,
  CSS_URL_RE,
  MARKUP_EXTENSIONS,
  OPF_URI,
  OPF_NS,
  URI_ATTRIBUTE_RE,
  item_by_id,
  item_by_path,
  metadata_node,
  package_title,
  relative_uri,
  remove_prop,
  resolve_relative_path,
  unique_id,
)
from .models import OperationReport, Package, PackageToolError, TocEntry
from .navigation import build_nav, build_ncx, parse_toc
from .package_io import build_container, ensure_no_encryption, read_archive, read_package, write_epub


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


def build_split_package(source_package: Package, selected_paths: list[str], resource_paths: set[str], opf_path: str, nav_path: str, ncx_path: str) -> bytes:
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
  target_spine_indices = [spine_paths.index(target.href.split("#", 1)[0]) if target.href.split("#", 1)[0] in spine_paths else -1 for target in targets]
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
  output_dir.mkdir(parents=True, exist_ok=True)
  for segment_index, (start, end) in enumerate(ranges, start=1):
    selected = spine_paths[start:end]
    selected_set = set(selected)
    resources = collect_referenced_resources(files, package, selected_set)
    nav_path = posixpath.join(posixpath.dirname(package.opf_path), "nav.xhtml")
    ncx_path = posixpath.join(posixpath.dirname(package.opf_path), "toc.ncx")
    segment_toc = [entry for entry in targets if not entry.href or entry.href.split("#", 1)[0] in selected_set]
    path_identity = {path: path for path in files}
    segment_files: dict[str, bytes] = {
      "META-INF/container.xml": build_container(package.opf_path),
      package.opf_path: build_split_package(package, selected, resources, package.opf_path, nav_path, ncx_path),
      nav_path: build_nav(package_title(package), [(package_title(package), segment_toc)], nav_path, path_identity),
      ncx_path: build_ncx(package_title(package), [(package_title(package), segment_toc)], ncx_path, path_identity),
    }
    for path in selected_set | resources:
      if path in files:
        segment_files[path] = files[path]
    output_path = output_dir / f"{input_path.stem}_{segment_index:02d}.epub"
    write_epub(output_path, segment_files, ["mimetype", "META-INF/container.xml", package.opf_path, nav_path, ncx_path])
    report.outputs.append(str(output_path))
    report.segments_created += 1
  return report


__all__ = ["list_split_targets", "split_epub"]
