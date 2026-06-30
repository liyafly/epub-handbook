"""EPUB merge operation."""

from __future__ import annotations

import copy
import posixpath
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_lib import split_props

from .core import (
  allocate_archive_path,
  build_opf,
  item_by_id,
  metadata_node,
  package_title,
  relative_uri,
  remove_prop,
  unique_id,
)
from .models import OperationReport, PackageToolError, SpineItem, TocEntry
from .navigation import build_nav, build_ncx, parse_toc
from .package_io import build_container, ensure_no_encryption, read_archive, read_package, write_epub
from .references import transform_resource


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
      if final_path:
        output_files[final_path] = transform_resource(files[item.archive_path], item.archive_path, final_path, path_map, known_files)

    for spine_item in package.spine_items:
      source_item = by_id.get(spine_item.idref)
      if source_item and source_item.item_id in id_map and "nav" not in split_props(source_item.properties):
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

  output_files["OEBPS/content.opf"] = build_opf(merged_title or "Merged EPUB", "OEBPS/content.opf", first_metadata, merged_manifest_items, merged_spine_items)
  nav_path_map = {path: path for path in output_files}
  output_files["OEBPS/nav.xhtml"] = build_nav(merged_title or "Merged EPUB", nav_groups, "OEBPS/nav.xhtml", nav_path_map)
  output_files["OEBPS/toc.ncx"] = build_ncx(merged_title or "Merged EPUB", nav_groups, "OEBPS/toc.ncx", nav_path_map)
  write_epub(output_path, output_files, ["mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/toc.ncx"])
  return report


__all__ = ["merge_epubs"]
