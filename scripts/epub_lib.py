#!/usr/bin/env python3
"""Shared stdlib-only EPUB package helpers.

This module is used by multiple scripts. Changes must run every
``scripts/test_*.py`` regression test.
"""

from __future__ import annotations

import posixpath
import re
import zipfile
from pathlib import Path
from urllib.parse import quote, unquote, urlsplit
from xml.etree import ElementTree as ET


CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
OPF_URI = "http://www.idpf.org/2007/opf"
DC_URI = "http://purl.org/dc/elements/1.1/"
DCTERMS_URI = "http://purl.org/dc/terms/"
NCX_URI = "http://www.daisy.org/z3986/2005/ncx/"
XHTML_URI = "http://www.w3.org/1999/xhtml"
OPS_URI = "http://www.idpf.org/2007/ops"
XML_URI = "http://www.w3.org/XML/1998/namespace"
IBOOKS_PREFIX = "http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/"
RENDITION_PREFIX = "http://www.idpf.org/vocab/rendition/#"

CONTAINER_NS = {"c": CONTAINER_URI}
OPF_NS = {"opf": OPF_URI, "dc": DC_URI}
NCX_NS = {"ncx": NCX_URI}
XHTML_NS = {"xhtml": XHTML_URI}
EPUB_NS = {"epub": OPS_URI}

FIXED_ZIP_TIME = (1980, 1, 1, 0, 0, 0)
HEAD_END_RE = re.compile(r"</head\s*>", re.I)


ET.register_namespace("", OPF_URI)
ET.register_namespace("dc", DC_URI)
ET.register_namespace("dcterms", DCTERMS_URI)
ET.register_namespace("opf", OPF_URI)


class EpubLibError(Exception):
  """The EPUB package cannot be read or written safely."""


def q(uri: str, name: str) -> str:
  return f"{{{uri}}}{name}"


def local_name(tag: object) -> str:
  if not isinstance(tag, str):
    return ""
  return tag.rsplit("}", 1)[-1]


def parse_xml(data: bytes | str, label: str = "<xml>") -> ET.Element:
  if isinstance(data, str):
    data = data.encode("utf-8")
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    raise ET.ParseError(f"{label}: XML parse failed: {exc}") from exc


def norm_join(base: str, href: str) -> str:
  clean = href.split("#", 1)[0]
  return posixpath.normpath(posixpath.join(base, clean))


def validate_archive_path(name: str, label: str) -> str:
  if not name or name.startswith("/"):
    raise EpubLibError(f"{label}: invalid absolute or empty ZIP path: {name!r}")
  normalized = posixpath.normpath(name)
  if normalized == ".." or normalized.startswith("../"):
    raise EpubLibError(f"{label}: ZIP path escapes archive root: {name!r}")
  return normalized


def quote_archive_path(value: str) -> str:
  return quote(value, safe="/:@-._~")


def is_external_uri(uri: str) -> bool:
  return bool(urlsplit(uri).scheme) or uri.startswith(("/", "//"))


def resolve_relative_path(base_file: str, uri_path: str) -> str:
  decoded = unquote(uri_path)
  return validate_archive_path(
    posixpath.join(posixpath.dirname(base_file), decoded),
    "resource href",
  )


def rel_href(from_zip_path: str, to_zip_path: str) -> str:
  base = posixpath.dirname(from_zip_path)
  return posixpath.relpath(to_zip_path, base) if base else to_zip_path


def is_macos_metadata_path(name: str) -> bool:
  return posixpath.basename(name.rstrip("/")) == ".DS_Store"


def split_props(value: str | None) -> list[str]:
  return [part for part in (value or "").split() if part]


def find_child(root: ET.Element, wanted: str) -> ET.Element | None:
  return next((child for child in root if local_name(child.tag) == wanted), None)


def manifest(root: ET.Element) -> ET.Element:
  node = root.find("opf:manifest", OPF_NS)
  if node is None:
    raise EpubLibError("OPF missing manifest")
  return node


def spine(root: ET.Element) -> ET.Element:
  node = root.find("opf:spine", OPF_NS)
  if node is None:
    raise EpubLibError("OPF missing spine")
  return node


def item_id_exists(root: ET.Element, item_id: str) -> bool:
  return any(item.attrib.get("id") == item_id for item in root.findall("opf:manifest/opf:item", OPF_NS))


def unique_id(root: ET.Element, base: str) -> str:
  candidate = re.sub(r"[^A-Za-z0-9_.-]+", "-", base).strip("-") or "item"
  if candidate[0].isdigit():
    candidate = f"x-{candidate}"
  index = 2
  result = candidate
  while item_id_exists(root, result):
    result = f"{candidate}-{index}"
    index += 1
  return result


def ensure_stylesheet_link(text: str, href: str) -> tuple[str, bool]:
  if href in text:
    return text, False
  link = f'  <link href="{href}" type="text/css" rel="stylesheet"/>\n'
  updated, count = HEAD_END_RE.subn(link + "</head>", text, count=1)
  return updated, bool(count)


def read_epub_archive(input_path: Path) -> tuple[dict[str, bytes], list[str]]:
  try:
    with zipfile.ZipFile(input_path) as zf:
      files: dict[str, bytes] = {}
      order: list[str] = []
      for info in zf.infolist():
        if info.is_dir():
          continue
        name = validate_archive_path(info.filename, "archive member")
        if name in files:
          raise EpubLibError(f"duplicate ZIP member: {name}")
        files[name] = zf.read(info.filename)
        order.append(name)
      return files, order
  except zipfile.BadZipFile as exc:
    raise EpubLibError(f"not a valid EPUB zip: {input_path}") from exc


def read_epub_files(input_path: Path) -> tuple[dict[str, bytes], list[str]]:
  files, order = read_epub_archive(input_path)
  clean_order = [name for name in order if not is_macos_metadata_path(name)]
  return {name: files[name] for name in clean_order}, clean_order


def opf_path_from_container(files: dict[str, bytes]) -> str:
  if "META-INF/container.xml" not in files:
    raise EpubLibError("missing META-INF/container.xml")
  container = parse_xml(files["META-INF/container.xml"], "META-INF/container.xml")
  rootfile = container.find(".//c:rootfile", CONTAINER_NS)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
  if not opf_path or opf_path not in files:
    raise EpubLibError(f"container rootfile does not resolve: {opf_path or '<missing>'}")
  return opf_path


def write_epub(output_path: Path, files: dict[str, bytes], original_order: list[str]) -> None:
  output_path.parent.mkdir(parents=True, exist_ok=True)
  ordered: list[str] = ["mimetype"]
  ordered.extend(
    name for name in original_order
    if name != "mimetype" and name in files and not is_macos_metadata_path(name)
  )
  ordered.extend(
    name for name in sorted(files)
    if name not in set(ordered) and not is_macos_metadata_path(name)
  )
  tmp = output_path.with_suffix(output_path.suffix + ".tmp")
  try:
    with zipfile.ZipFile(tmp, "w") as zf:
      for name in ordered:
        data = files[name]
        info = zipfile.ZipInfo(name, FIXED_ZIP_TIME)
        info.compress_type = zipfile.ZIP_STORED if name == "mimetype" else zipfile.ZIP_DEFLATED
        zf.writestr(info, data)
    tmp.replace(output_path)
  except Exception:
    if tmp.exists():
      tmp.unlink()
    raise
