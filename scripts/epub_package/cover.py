"""EPUB cover replacement operation."""

from __future__ import annotations

import copy
import posixpath
from pathlib import Path
from urllib.parse import urlsplit
from xml.etree import ElementTree as ET

from epub_lib import is_external_uri, q, split_props

from .core import (
  IMAGE_MEDIA_BY_EXT,
  OPF_NS,
  OPF_URI,
  add_prop,
  cover_raster_dimensions,
  manifest_node,
  metadata_node,
  resize_svg_cover_pages,
  resolve_relative_path,
  validate_archive_path,
)
from .models import OperationReport, PackageToolError
from .package_io import ensure_no_encryption, read_archive, read_package, write_epub
from .references import transform_resource


def media_type_for_cover(path: Path) -> str:
  return "image/jpeg" if path.suffix.lower() == ".jpeg" else IMAGE_MEDIA_BY_EXT.get(path.suffix.lower(), "image/jpeg")


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
  cover_item.set("media-type", media_type_for_cover(cover_path))
  cover_item.set("properties", add_prop(cover_item.attrib.get("properties", ""), "cover-image"))
  for old_path in old_cover_paths:
    if old_path != new_archive_path:
      updated.pop(old_path, None)
  updated[new_archive_path] = cover_data
  if old_cover_paths:
    path_map = {old_path: new_archive_path for old_path in old_cover_paths}
    known_files = set(files)
    for name, data in list(updated.items()):
      if name not in {package.opf_path, new_archive_path}:
        transformed = transform_resource(data, name, name, path_map, known_files)
        updated[name] = resize_svg_cover_pages(transformed, name, new_archive_path, cover_dimensions)
  for elem in list(meta.findall('opf:meta[@name="cover"]', OPF_NS)):
    meta.remove(elem)
  ET.SubElement(meta, q(OPF_URI, "meta"), {"name": "cover", "content": cover_id})
  updated[package.opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, updated, order)
  return OperationReport("replace-cover", input=str(input_path), output=str(output_path), opf=package.opf_path, cover_path=new_archive_path)


__all__ = ["replace_cover"]
