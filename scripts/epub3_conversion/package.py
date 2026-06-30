"""EPUB package metadata, manifest, cover, guide, and spine passes."""

from __future__ import annotations

from xml.etree import ElementTree as ET

from . import core
from .models import ConversionReport


def normalize_metadata(root: ET.Element, report: ConversionReport, body_font_locked: bool = False) -> None:
  core.normalize_metadata(root, report, body_font_locked)


def has_body_font_locked(files: dict[str, bytes]) -> bool:
  return core.has_body_font_locked(files)


def normalize_manifest_media(root: ET.Element, report: ConversionReport) -> None:
  core.normalize_manifest_media(root, report)


def ensure_cover_properties(root: ET.Element, report: ConversionReport) -> None:
  core.ensure_cover_properties(root, report)


def ensure_spine_toc(root: ET.Element) -> None:
  core.ensure_spine_toc(root)


def fix_guide_hrefs(
  root: ET.Element,
  files: dict[str, bytes],
  opf_dir: str,
  report: ConversionReport,
) -> None:
  core.fix_guide_hrefs(root, files, opf_dir, report)


def unique_href(files: dict[str, bytes], opf_dir: str, href: str) -> str:
  return core.unique_href(files, opf_dir, href)


def add_manifest_item(
  root: ET.Element,
  report: ConversionReport,
  wanted_id: str,
  href: str,
  media_type: str,
  properties: str | None = None,
) -> ET.Element:
  return core.add_manifest_item(root, report, wanted_id, href, media_type, properties)


__all__ = [
  "add_manifest_item",
  "ensure_cover_properties",
  "ensure_spine_toc",
  "fix_guide_hrefs",
  "has_body_font_locked",
  "normalize_manifest_media",
  "normalize_metadata",
  "unique_href",
]
