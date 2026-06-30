"""EPUB metadata read/write operations."""

from __future__ import annotations

import copy
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_lib import q, split_props

from .core import (
  DC_URI,
  OPF_NS,
  OPF_URI,
  metadata_node,
)
from .models import MetadataInfo, OperationReport
from .package_io import ensure_no_encryption, read_archive, read_package, write_epub


def read_metadata(input_path: Path) -> MetadataInfo:
  files, _order = read_archive(input_path)
  package = read_package(files)
  info = MetadataInfo()
  meta = metadata_node(package.root)
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
  cover_meta = meta.find('opf:meta[@name="cover"]', OPF_NS)
  cover_id = cover_meta.attrib.get("content", "") if cover_meta is not None else ""
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
    report.fields_updated += set_titles(meta, metadata.get("title"), metadata.get("subtitle"), update_subtitle="subtitle" in metadata)
  for key, tag in {
    "author": "creator", "language": "language", "publisher": "publisher",
    "description": "description", "identifier": "identifier", "rights": "rights",
  }.items():
    if key in metadata:
      report.fields_updated += int(set_dc_text(meta, tag, metadata[key]))
  updated = dict(files)
  updated[package.opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, updated, order)
  return report


__all__ = ["read_metadata", "write_metadata"]
