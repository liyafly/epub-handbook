"""EPUB3 migration orchestration."""

from __future__ import annotations

import hashlib
import posixpath
from pathlib import Path
from xml.etree import ElementTree as ET

from epub_lib import norm_join, opf_path_from_container, parse_xml, read_epub_files, write_epub

from .models import ConversionReport
from .navigation import ensure_nav
from . import package as package_pass
from . import xhtml as xhtml_pass


def sha256_file(path: Path) -> str:
  digest = hashlib.sha256()
  with path.open("rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
      digest.update(chunk)
  return digest.hexdigest()


def convert_epub(
  input_path: Path,
  output_path: Path,
  popup_notes: bool = True,
  typography: bool = True,
) -> ConversionReport:
  report = ConversionReport(input_sha256=sha256_file(input_path), output=str(output_path))
  files, original_order = read_epub_files(input_path)
  files["mimetype"] = b"application/epub+zip"
  opf_path = opf_path_from_container(files)
  report.opf = opf_path
  root = parse_xml(files[opf_path], opf_path)
  report.package_version_before = root.attrib.get("version")
  opf_dir = posixpath.dirname(opf_path)

  package_pass.normalize_metadata(root, report, body_font_locked=package_pass.has_body_font_locked(files))
  package_pass.normalize_manifest_media(root, report)
  package_pass.ensure_cover_properties(root, report)
  package_pass.ensure_spine_toc(root)
  package_pass.fix_guide_hrefs(root, files, opf_dir, report)
  style_href = package_pass.unique_href(files, opf_dir, "Styles/epub3-enhancements.css")
  style_zip = norm_join(opf_dir, style_href)
  note_href = xhtml_pass.default_note_href(files, root, opf_dir)
  note_zip = norm_join(opf_dir, note_href)
  if typography:
    report.typography_roles = list(xhtml_pass.TYPOGRAPHY_ROLES)
    files[style_zip] = xhtml_pass.enhancement_css()
    package_pass.add_manifest_item(root, report, "epub3-enhancements-css", style_href, "text/css")
  default_note_icon_used = xhtml_pass.update_xhtml_files(
    files,
    root,
    opf_path,
    style_zip,
    note_zip,
    report,
    popup_notes=popup_notes,
    typography=typography,
    default_language=xhtml_pass.xhtml_default_language(root),
  )
  if popup_notes and default_note_icon_used:
    if note_zip not in files:
      files[note_zip] = xhtml_pass.note_png_bytes()
    package_pass.add_manifest_item(root, report, "note-icon", note_href, "image/png")
  ensure_nav(files, root, opf_path, report)
  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, files, original_order)
  return report


__all__ = ["convert_epub", "sha256_file"]
