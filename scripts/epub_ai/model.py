"""Lightweight in-memory EPUB model and model builders."""

from __future__ import annotations

import posixpath
import sys
import zipfile
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import unquote
from xml.etree import ElementTree as ET


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


@dataclass
class EpubModel:
  xhtml_docs: dict[str, ET.Element] = field(default_factory=dict)
  opf_root: ET.Element | None = None
  opf_path: str = ""
  css_docs: dict[str, str] = field(default_factory=dict)
  book_language: str | None = None


def _norm_join(base: str, href: str) -> str:
  href = href.split("#", 1)[0]
  return posixpath.normpath(posixpath.join(base, unquote(href)))


def build_model(zf: zipfile.ZipFile, opf_path: str, opf_root: ET.Element) -> EpubModel:
  """Build the detector model from an opened EPUB archive."""
  opf_dir = posixpath.dirname(opf_path)
  manifest = opf_root.findall("opf:manifest/opf:item", OPF_NS)
  names = set(zf.namelist())
  xhtml_docs: dict[str, ET.Element] = {}
  css_docs: dict[str, str] = {}
  for item in manifest:
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    target = _norm_join(opf_dir, href)
    if target not in names:
      continue
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      try:
        xhtml_docs[target] = ET.fromstring(zf.read(target))
      except ET.ParseError as exc:
        print(f"WARNING: cannot parse XHTML {target}: {exc}", file=sys.stderr)
    elif media_type == "text/css" or href.endswith(".css"):
      try:
        css_docs[target] = zf.read(target).decode("utf-8", errors="ignore")
      except Exception as exc:
        print(f"WARNING: cannot read CSS {target}: {exc}", file=sys.stderr)
  lang = opf_root.findtext(".//{http://purl.org/dc/elements/1.1/}language")
  return EpubModel(xhtml_docs, opf_root, opf_path, css_docs, lang)


def build_model_from_tree(root_dir: Path, opf_path: str, opf_root: ET.Element) -> EpubModel:
  """Build the detector model from an unpacked EPUB source tree."""
  opf_dir = posixpath.dirname(opf_path)
  manifest = opf_root.findall("opf:manifest/opf:item", OPF_NS)
  xhtml_docs: dict[str, ET.Element] = {}
  css_docs: dict[str, str] = {}
  for item in manifest:
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "")
    archive_path = _norm_join(opf_dir, href)
    target = root_dir / archive_path
    if not target.exists():
      continue
    if media_type == "application/xhtml+xml" or href.endswith(".xhtml"):
      try:
        xhtml_docs[archive_path] = ET.parse(str(target)).getroot()
      except ET.ParseError as exc:
        print(f"WARNING: cannot parse XHTML {target}: {exc}", file=sys.stderr)
    elif media_type == "text/css" or href.endswith(".css"):
      try:
        css_docs[archive_path] = target.read_text(encoding="utf-8", errors="ignore")
      except Exception as exc:
        print(f"WARNING: cannot read CSS {target}: {exc}", file=sys.stderr)
  lang = opf_root.findtext(".//{http://purl.org/dc/elements/1.1/}language")
  return EpubModel(xhtml_docs, opf_root, opf_path, css_docs, lang)


__all__ = ["EpubModel", "build_model", "build_model_from_tree"]
