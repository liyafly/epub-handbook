#!/usr/bin/env python3
"""Format EPUB resource layout or deobfuscate resource filenames.

This is a Python standard-library-only implementation. Its behavior is informed
by the `reformat` and filename `decrypt` workflows in cnwxi/epub_tool:
https://github.com/cnwxi/epub_tool

The implementation is intentionally narrower and conservative:
- preserve prose and binary resource bytes;
- rewrite local references after moving resources;
- keep the OPF at its existing path;
- stop on DRM or unsupported encrypted resources;
- allow standard EPUB font obfuscation because font bytes are not decrypted.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import posixpath
import re
import sys
import tempfile
import zipfile
from dataclasses import asdict, dataclass, field
from pathlib import Path
from urllib.parse import quote, unquote, urlsplit, urlunsplit
from xml.etree import ElementTree as ET


CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
OPF_URI = "http://www.idpf.org/2007/opf"
DC_URI = "http://purl.org/dc/elements/1.1/"
DCTERMS_URI = "http://purl.org/dc/terms/"

ET.register_namespace("", OPF_URI)
ET.register_namespace("dc", DC_URI)
ET.register_namespace("dcterms", DCTERMS_URI)

FONT_OBFUSCATION_ALGORITHMS = {
  "http://www.idpf.org/2008/embedding",
  "http://ns.adobe.com/pdf/enc#RC",
}
MARKUP_EXTENSIONS = {".html", ".htm", ".xhtml", ".xml", ".ncx", ".svg", ".smil"}
URI_ATTRIBUTE_RE = re.compile(
  r"(?P<prefix>\b(?:href|src|poster|data|xlink:href|textref)\s*=\s*)"
  r"(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
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
XML_ENCODING_RE = re.compile(br"encoding\s*=\s*[\"']([A-Za-z0-9._-]+)[\"']", flags=re.I)
INVALID_FILENAME_RE = re.compile(r"[\x00-\x1f\\/:*?\"<>|]+")


class StructureToolError(Exception):
  """The EPUB cannot be rewritten conservatively."""


@dataclass
class EncryptionRecord:
  uri: str
  algorithm: str
  archive_path: str


@dataclass
class RewriteReport:
  operation: str
  input: str
  output: str | None = None
  opf: str = ""
  manifest_resources: int = 0
  moved_resources: int = 0
  renamed_resources: int = 0
  rewritten_files: int = 0
  font_obfuscation_resources: int = 0
  removed_stale_encryption_resources: int = 0
  dry_run: bool = False
  mappings: list[dict[str, str]] = field(default_factory=list)
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return asdict(self)


@dataclass
class WorkflowReport:
  operation: str
  input: str
  output: str
  dry_run: bool
  stages: list[dict[str, object]]

  def as_dict(self) -> dict[str, object]:
    return asdict(self)


@dataclass
class ManifestResource:
  item_id: str
  href: str
  media_type: str
  archive_path: str


@dataclass
class Package:
  opf_path: str
  root: ET.Element
  resources: list[ManifestResource]


def local_name(tag: object) -> str:
  if not isinstance(tag, str):
    return ""
  return tag.rsplit("}", 1)[-1]


def validate_archive_path(name: str, label: str) -> str:
  if not name or name.startswith("/"):
    raise StructureToolError(f"{label}: invalid absolute or empty ZIP path: {name!r}")
  normalized = posixpath.normpath(name)
  if normalized == ".." or normalized.startswith("../"):
    raise StructureToolError(f"{label}: ZIP path escapes archive root: {name!r}")
  return normalized


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
          raise StructureToolError(f"duplicate ZIP member: {name}")
        files[name] = zf.read(info.filename)
        order.append(name)
  except zipfile.BadZipFile as exc:
    raise StructureToolError(f"not a readable EPUB ZIP: {path}") from exc
  return files, order


def parse_xml(data: bytes, label: str) -> ET.Element:
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    raise StructureToolError(f"{label}: XML parse failed: {exc}") from exc


def find_child(root: ET.Element, wanted: str) -> ET.Element | None:
  for child in root:
    if local_name(child.tag) == wanted:
      return child
  return None


def resolve_relative_path(base_file: str, uri_path: str) -> str:
  decoded = unquote(uri_path)
  return validate_archive_path(posixpath.join(posixpath.dirname(base_file), decoded), "resource href")


def resolve_root_path(uri_path: str) -> str:
  return validate_archive_path(unquote(uri_path).lstrip("/"), "encryption URI")


def is_external_uri(uri: str) -> bool:
  return bool(urlsplit(uri).scheme) or uri.startswith(("/", "//"))


def read_package(files: dict[str, bytes]) -> Package:
  container_path = "META-INF/container.xml"
  if container_path not in files:
    raise StructureToolError("missing META-INF/container.xml")
  container = parse_xml(files[container_path], container_path)
  rootfile = next((elem for elem in container.iter() if local_name(elem.tag) == "rootfile"), None)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else ""
  if not opf_path:
    raise StructureToolError("container.xml has no rootfile full-path")
  opf_path = validate_archive_path(opf_path, "container.xml rootfile")
  if opf_path not in files:
    raise StructureToolError(f"container.xml rootfile does not resolve: {opf_path}")

  root = parse_xml(files[opf_path], opf_path)
  manifest = find_child(root, "manifest")
  if manifest is None:
    raise StructureToolError(f"{opf_path}: OPF missing manifest")

  resources: list[ManifestResource] = []
  item_ids: set[str] = set()
  for item in manifest:
    if local_name(item.tag) != "item":
      continue
    item_id = item.attrib.get("id", "")
    href = item.attrib.get("href", "")
    media_type = item.attrib.get("media-type", "application/octet-stream")
    if not item_id or not href:
      raise StructureToolError(f"{opf_path}: manifest item missing id or href")
    if item_id in item_ids:
      raise StructureToolError(f"{opf_path}: duplicate manifest id: {item_id}")
    item_ids.add(item_id)
    if is_external_uri(href):
      continue
    archive_path = resolve_relative_path(opf_path, urlsplit(href).path)
    resources.append(ManifestResource(item_id, href, media_type, archive_path))
  return Package(opf_path, root, resources)


def classify_resource(resource: ManifestResource) -> str:
  media_type = resource.media_type.lower()
  ext = posixpath.splitext(resource.archive_path)[1].lower()
  if media_type == "application/xhtml+xml" or ext in {".html", ".htm", ".xhtml"}:
    return "Text"
  if media_type == "text/css" or ext == ".css":
    return "Styles"
  if media_type.startswith("image/") or ext in {".bmp", ".gif", ".jpeg", ".jpg", ".png", ".svg", ".webp"}:
    return "Images"
  if "font" in media_type or ext in {".otf", ".ttf", ".woff", ".woff2"}:
    return "Fonts"
  if media_type.startswith("audio/") or ext in {".m4a", ".mp3", ".ogg"}:
    return "Audio"
  if media_type.startswith("video/") or ext in {".m4v", ".mp4", ".webm"}:
    return "Video"
  if media_type == "application/x-dtbncx+xml" or ext == ".ncx":
    return ""
  return "Misc"


def sanitize_filename_component(value: str, fallback_seed: str) -> str:
  sanitized = INVALID_FILENAME_RE.sub("-", unquote(value)).strip(" .")
  sanitized = re.sub(r"-{2,}", "-", sanitized)
  if sanitized:
    return sanitized
  return f"resource-{hashlib.sha256(fallback_seed.encode('utf-8')).hexdigest()[:12]}"


def deobfuscated_basename(resource: ManifestResource) -> str:
  source_name = posixpath.basename(resource.archive_path)
  _, source_ext = posixpath.splitext(source_name)
  item_name = resource.item_id
  item_stem, item_ext = posixpath.splitext(item_name)
  if item_ext.lower() == source_ext.lower():
    item_name = item_stem

  slim = bool(re.search(r"(?:[~_-]?slim)$", item_name, flags=re.I))
  if slim:
    item_name = re.sub(r"(?:[~_-]?slim)$", "", item_name, flags=re.I)
  elif re.search(r"(?:[~_-]?slim)$", posixpath.splitext(source_name)[0], flags=re.I):
    slim = True

  stem = sanitize_filename_component(item_name, resource.item_id)
  return f"{stem}{'~slim' if slim else ''}{source_ext.lower()}"


def suffix_path(path: str, index: int) -> str:
  stem, ext = posixpath.splitext(path)
  return f"{stem}-{index}{ext}"


def allocate_path(preferred: str, used: set[str]) -> str:
  candidate = validate_archive_path(preferred, "output resource")
  index = 2
  while candidate in used:
    candidate = suffix_path(preferred, index)
    index += 1
  used.add(candidate)
  return candidate


def build_path_map(
  package: Package,
  files: dict[str, bytes],
  operation: str,
  report: RewriteReport,
) -> dict[str, str]:
  source_resources: dict[str, ManifestResource] = {}
  for resource in package.resources:
    if resource.archive_path not in files:
      report.warnings.append(f"manifest href does not resolve: {resource.href}")
      continue
    source_resources.setdefault(resource.archive_path, resource)
  report.manifest_resources = len(source_resources)

  managed = set(source_resources)
  used = set(files) - managed - {"mimetype"}
  opf_dir = posixpath.dirname(package.opf_path)
  path_map: dict[str, str] = {}
  for source, resource in source_resources.items():
    folder = classify_resource(resource)
    basename = (
      deobfuscated_basename(resource)
      if operation == "deobfuscate-filenames"
      else posixpath.basename(source)
    )
    preferred = posixpath.join(opf_dir, folder, basename) if folder else posixpath.join(opf_dir, basename)
    target = allocate_path(preferred, used)
    path_map[source] = target
    if target == source:
      continue
    report.moved_resources += 1
    if posixpath.basename(target) != posixpath.basename(source):
      report.renamed_resources += 1
    report.mappings.append({"from": source, "to": target})
  return path_map


def encryption_path(files: dict[str, bytes]) -> str | None:
  for name in files:
    if name.lower() == "meta-inf/encryption.xml":
      return name
  return None


def inspect_encryption(files: dict[str, bytes]) -> tuple[str | None, list[EncryptionRecord]]:
  path = encryption_path(files)
  if not path:
    return None, []
  root = parse_xml(files[path], path)
  records: list[EncryptionRecord] = []
  for encrypted_data in root.iter():
    if local_name(encrypted_data.tag) != "EncryptedData":
      continue
    algorithm = ""
    for elem in encrypted_data.iter():
      if local_name(elem.tag) == "EncryptionMethod":
        algorithm = elem.attrib.get("Algorithm", "")
        break
    for elem in encrypted_data.iter():
      if local_name(elem.tag) != "CipherReference":
        continue
      uri = elem.attrib.get("URI", "")
      if not uri or is_external_uri(uri):
        raise StructureToolError(f"{path}: unsupported encryption URI: {uri!r}")
      records.append(EncryptionRecord(uri, algorithm, resolve_root_path(urlsplit(uri).path)))
  return path, records


def validate_encryption(
  records: list[EncryptionRecord],
  package: Package,
  files: dict[str, bytes],
  report: RewriteReport,
) -> None:
  if not records:
    return
  resource_by_path = {resource.archive_path: resource for resource in package.resources}
  unsupported: list[EncryptionRecord] = []
  for record in records:
    if record.archive_path not in files:
      report.removed_stale_encryption_resources += 1
      report.warnings.append(f"remove stale encryption reference with missing target: {record.uri}")
      continue
    resource = resource_by_path.get(record.archive_path)
    if not resource or classify_resource(resource) != "Fonts":
      unsupported.append(record)
      continue
    if record.algorithm not in FONT_OBFUSCATION_ALGORITHMS:
      unsupported.append(record)
  if unsupported:
    sample = ", ".join(record.uri for record in unsupported[:3])
    raise StructureToolError(
      "DRM or unsupported encrypted resources detected; this tool only deobfuscates filenames "
      f"and allows standard EPUB font obfuscation. Refusing to rewrite: {sample}"
    )
  report.font_obfuscation_resources = len(records) - report.removed_stale_encryption_resources


def quote_archive_path(value: str) -> str:
  return quote(value, safe="/:@-._~")


def relative_uri(from_archive_path: str, to_archive_path: str) -> str:
  base = posixpath.dirname(from_archive_path)
  relative = posixpath.relpath(to_archive_path, base) if base else to_archive_path
  return quote_archive_path(relative)


def rewrite_uri(
  uri: str,
  old_document: str,
  new_document: str,
  path_map: dict[str, str],
  files: dict[str, bytes],
  report: RewriteReport,
) -> str:
  if not uri or uri.startswith("#") or is_external_uri(uri):
    return uri
  parts = urlsplit(uri)
  if not parts.path:
    return uri
  try:
    old_target = resolve_relative_path(old_document, parts.path)
  except StructureToolError:
    report.warnings.append(f"{old_document}: unsafe local reference left unchanged: {uri}")
    return uri
  if old_target not in files:
    report.warnings.append(f"{old_document}: missing local reference left unchanged: {uri}")
    return uri
  target = path_map.get(old_target, old_target)
  path = relative_uri(new_document, target)
  return urlunsplit(("", "", path, parts.query, parts.fragment))


def decode_text(data: bytes, label: str) -> tuple[str, str]:
  encodings: list[str] = []
  match = XML_ENCODING_RE.search(data[:256])
  if match:
    encodings.append(match.group(1).decode("ascii", errors="ignore"))
  if data.startswith(b"\xef\xbb\xbf"):
    encodings.append("utf-8-sig")
  if data.startswith((b"\xff\xfe", b"\xfe\xff")):
    encodings.append("utf-16")
  encodings.extend(["utf-8", "gb18030"])
  for encoding in dict.fromkeys(encodings):
    try:
      return data.decode(encoding), encoding
    except (LookupError, UnicodeDecodeError):
      continue
  raise StructureToolError(f"{label}: cannot decode text resource as UTF or GB18030")


def rewrite_css_references(
  text: str,
  old_document: str,
  new_document: str,
  path_map: dict[str, str],
  files: dict[str, bytes],
  report: RewriteReport,
) -> str:
  def replace_url(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, files, report)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}{match.group('suffix')}"

  def replace_import(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, files, report)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}"

  text = CSS_URL_RE.sub(replace_url, text)
  return CSS_IMPORT_RE.sub(replace_import, text)


def rewrite_srcset_urls(
  text: str,
  old_document: str,
  new_document: str,
  path_map: dict[str, str],
  files: dict[str, bytes],
  report: RewriteReport,
) -> str:
  """Rewrite comma-separated srcset URL candidates."""
  srcset_re = re.compile(
    r"(?P<prefix>\bsrcset\s*=\s*)(?P<quote>[\"'])(?P<uri>.*?)(?P=quote)",
    flags=re.IGNORECASE | re.DOTALL,
  )

  def replace_srcset(match: re.Match[str]) -> str:
    candidates: list[str] = []
    for candidate in match.group("uri").split(","):
      parts = candidate.strip().split()
      if not parts:
        continue
      url = rewrite_uri(parts[0], old_document, new_document, path_map, files, report)
      descriptor = " ".join(parts[1:])
      candidates.append(f"{url} {descriptor}".strip())
    return f"{match.group('prefix')}{match.group('quote')}{', '.join(candidates)}{match.group('quote')}"

  return srcset_re.sub(replace_srcset, text)


def rewrite_markup_references(
  text: str,
  old_document: str,
  new_document: str,
  path_map: dict[str, str],
  files: dict[str, bytes],
  report: RewriteReport,
) -> str:
  def replace_attr(match: re.Match[str]) -> str:
    uri = rewrite_uri(match.group("uri"), old_document, new_document, path_map, files, report)
    return f"{match.group('prefix')}{match.group('quote')}{uri}{match.group('quote')}"

  text = rewrite_srcset_urls(text, old_document, new_document, path_map, files, report)
  return rewrite_css_references(
    URI_ATTRIBUTE_RE.sub(replace_attr, text),
    old_document,
    new_document,
    path_map,
    files,
    report,
  )


def rewrite_opf(
  package: Package,
  files: dict[str, bytes],
  path_map: dict[str, str],
  report: RewriteReport,
) -> bytes:
  root = copy.deepcopy(package.root)
  manifest = find_child(root, "manifest")
  if manifest is None:
    raise StructureToolError(f"{package.opf_path}: OPF missing manifest")
  for item in manifest:
    if local_name(item.tag) != "item":
      continue
    href = item.attrib.get("href", "")
    if not href or is_external_uri(href):
      continue
    parts = urlsplit(href)
    old_target = resolve_relative_path(package.opf_path, parts.path)
    target = path_map.get(old_target)
    if target:
      item.set("href", urlunsplit(("", "", relative_uri(package.opf_path, target), parts.query, parts.fragment)))

  for elem in root.iter():
    if local_name(elem.tag) == "item":
      continue
    for attr in ("href", "src"):
      uri = elem.attrib.get(attr)
      if uri:
        elem.set(attr, rewrite_uri(uri, package.opf_path, package.opf_path, path_map, files, report))
  return ET.tostring(root, encoding="utf-8", xml_declaration=True)


def rewrite_encryption_xml(
  data: bytes,
  path: str,
  path_map: dict[str, str],
  files: dict[str, bytes],
) -> bytes | None:
  root = parse_xml(data, path)
  parents = {child: parent for parent in root.iter() for child in parent}
  for elem in list(root.iter()):
    if local_name(elem.tag) != "CipherReference":
      continue
    uri = elem.attrib.get("URI", "")
    if not uri:
      continue
    parts = urlsplit(uri)
    old_target = resolve_root_path(parts.path)
    if old_target not in files:
      parent = parents.get(elem)
      if parent is not None:
        parent.remove(elem)
      continue
    target = path_map.get(old_target, old_target)
    elem.set("URI", urlunsplit(("", "", quote_archive_path(target), parts.query, parts.fragment)))
  parents = {child: parent for parent in root.iter() for child in parent}
  for elem in list(root.iter()):
    if local_name(elem.tag) != "EncryptedData":
      continue
    if not any(local_name(child.tag) == "CipherReference" for child in elem.iter()):
      parent = parents.get(elem)
      if parent is not None:
        parent.remove(elem)
  if not any(local_name(elem.tag) == "EncryptedData" for elem in root.iter()):
    return None
  return ET.tostring(root, encoding="utf-8", xml_declaration=True)


def transform_files(
  files: dict[str, bytes],
  package: Package,
  path_map: dict[str, str],
  encryption_xml: str | None,
  report: RewriteReport,
) -> dict[str, bytes]:
  transformed: dict[str, bytes] = {}
  for old_path, data in files.items():
    if old_path == "mimetype":
      continue
    new_path = path_map.get(old_path, old_path)
    updated = data
    if old_path == package.opf_path:
      updated = rewrite_opf(package, files, path_map, report)
    elif encryption_xml and old_path == encryption_xml:
      rewritten_encryption = rewrite_encryption_xml(data, encryption_xml, path_map, files)
      if rewritten_encryption is None:
        continue
      updated = rewritten_encryption
    else:
      ext = posixpath.splitext(old_path)[1].lower()
      if ext == ".css" or ext in MARKUP_EXTENSIONS:
        text, encoding = decode_text(data, old_path)
        if ext == ".css":
          rewritten = rewrite_css_references(text, old_path, new_path, path_map, files, report)
        else:
          rewritten = rewrite_markup_references(text, old_path, new_path, path_map, files, report)
        updated = rewritten.encode(encoding)
    if updated != data:
      report.rewritten_files += 1
    if new_path in transformed:
      raise StructureToolError(f"output path collision: {new_path}")
    transformed[new_path] = updated
  return transformed


def output_order(original_order: list[str], path_map: dict[str, str], transformed: dict[str, bytes]) -> list[str]:
  ordered: list[str] = []
  seen: set[str] = set()
  for old_path in original_order:
    if old_path == "mimetype":
      continue
    new_path = path_map.get(old_path, old_path)
    if new_path in transformed and new_path not in seen:
      ordered.append(new_path)
      seen.add(new_path)
  ordered.extend(name for name in transformed if name not in seen)
  return ordered


def write_epub(path: Path, files: dict[str, bytes], order: list[str], force: bool) -> None:
  if path.exists() and not force:
    raise StructureToolError(f"output already exists; pass --force to replace it: {path}")
  path.parent.mkdir(parents=True, exist_ok=True)
  handle, temp_name = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
  os.close(handle)
  temp_path = Path(temp_name)
  try:
    with zipfile.ZipFile(temp_path, "w") as zf:
      mimetype = zipfile.ZipInfo("mimetype")
      mimetype.compress_type = zipfile.ZIP_STORED
      zf.writestr(mimetype, b"application/epub+zip")
      for name in order:
        info = zipfile.ZipInfo(name)
        info.compress_type = zipfile.ZIP_DEFLATED
        zf.writestr(info, files[name])
    temp_path.replace(path)
  except Exception:
    temp_path.unlink(missing_ok=True)
    raise


def analyze_epub(path: Path, operation: str, output: Path | None = None) -> tuple[
  RewriteReport,
  dict[str, bytes],
  list[str],
  Package,
  dict[str, str],
  str | None,
]:
  files, order = read_archive(path)
  package = read_package(files)
  report = RewriteReport(operation=operation, input=str(path), output=str(output) if output else None, opf=package.opf_path)
  report.manifest_resources = len(package.resources)
  encryption_xml, encryption_records = inspect_encryption(files)
  validate_encryption(encryption_records, package, files, report)
  path_map = build_path_map(package, files, operation, report) if operation != "inspect" else {}
  return report, files, order, package, path_map, encryption_xml


def rewrite_epub(
  input_path: Path,
  output_path: Path,
  operation: str,
  *,
  force: bool = False,
  dry_run: bool = False,
) -> RewriteReport:
  if operation not in {"format", "deobfuscate-filenames"}:
    raise ValueError(f"unsupported rewrite operation: {operation}")
  if input_path.resolve() == output_path.resolve():
    raise StructureToolError("input and output paths must differ")
  report, files, order, package, path_map, encryption_xml = analyze_epub(input_path, operation, output_path)
  report.dry_run = dry_run
  if dry_run:
    return report
  transformed = transform_files(files, package, path_map, encryption_xml, report)
  write_epub(output_path, transformed, output_order(order, path_map, transformed), force)
  return report


def inspect_epub(input_path: Path) -> RewriteReport:
  report, _, _, _, _, _ = analyze_epub(input_path, "inspect")
  return report


def normalize_epub(
  input_path: Path,
  output_path: Path,
  *,
  force: bool = False,
  dry_run: bool = False,
) -> WorkflowReport:
  if input_path.resolve() == output_path.resolve():
    raise StructureToolError("input and output paths must differ")
  with tempfile.TemporaryDirectory(prefix="epub-structure-tool-") as raw:
    formatted = Path(raw) / "formatted.epub"
    format_report = rewrite_epub(input_path, formatted, "format")
    deobfuscate_report = rewrite_epub(
      formatted,
      output_path,
      "deobfuscate-filenames",
      force=force,
      dry_run=dry_run,
    )
  format_report.output = None
  return WorkflowReport(
    operation="normalize",
    input=str(input_path),
    output=str(output_path),
    dry_run=dry_run,
    stages=[format_report.as_dict(), deobfuscate_report.as_dict()],
  )


def render_rewrite_markdown(report: RewriteReport) -> str:
  lines = [
    "# EPUB Structure Tool Report",
    "",
    f"- Operation: `{report.operation}`",
    f"- Input: `{report.input}`",
    f"- OPF: `{report.opf}`",
  ]
  if report.output:
    lines.append(f"- Output: `{report.output}`")
  lines.extend([
    f"- Dry run: `{report.dry_run}`",
    f"- Manifest resources: `{report.manifest_resources}`",
    f"- Moved resources: `{report.moved_resources}`",
    f"- Renamed resources: `{report.renamed_resources}`",
    f"- Rewritten files: `{report.rewritten_files}`",
    f"- Standard font obfuscation resources: `{report.font_obfuscation_resources}`",
    f"- Removed stale encryption resources: `{report.removed_stale_encryption_resources}`",
  ])
  if report.mappings:
    lines.extend(["", "## Mappings"])
    lines.extend(f"- `{item['from']}` -> `{item['to']}`" for item in report.mappings)
  if report.warnings:
    lines.extend(["", "## Warnings"])
    lines.extend(f"- {warning}" for warning in dict.fromkeys(report.warnings))
  return "\n".join(lines) + "\n"


def render_markdown(report: RewriteReport | WorkflowReport) -> str:
  if isinstance(report, RewriteReport):
    return render_rewrite_markdown(report)
  lines = [
    "# EPUB Structure Tool Workflow Report",
    "",
    f"- Operation: `{report.operation}`",
    f"- Input: `{report.input}`",
    f"- Output: `{report.output}`",
    f"- Dry run: `{report.dry_run}`",
  ]
  for index, stage in enumerate(report.stages, start=1):
    lines.extend([
      "",
      f"## Stage {index}: `{stage['operation']}`",
      "",
      f"- Manifest resources: `{stage['manifest_resources']}`",
      f"- Moved resources: `{stage['moved_resources']}`",
      f"- Renamed resources: `{stage['renamed_resources']}`",
      f"- Rewritten files: `{stage['rewritten_files']}`",
      f"- Standard font obfuscation resources: `{stage['font_obfuscation_resources']}`",
      f"- Removed stale encryption resources: `{stage['removed_stale_encryption_resources']}`",
    ])
    mappings = stage.get("mappings") or []
    if mappings:
      lines.append("")
      lines.extend(f"- `{item['from']}` -> `{item['to']}`" for item in mappings)
    warnings = stage.get("warnings") or []
    if warnings:
      lines.extend(["", "### Warnings"])
      lines.extend(f"- {warning}" for warning in dict.fromkeys(warnings))
  return "\n".join(lines) + "\n"


def default_output(input_path: Path, operation: str) -> Path:
  suffix_by_operation = {
    "format": "_formatted.epub",
    "deobfuscate-filenames": "_deobfuscated.epub",
    "normalize": "_normalized.epub",
  }
  suffix = suffix_by_operation[operation]
  return input_path.with_name(f"{input_path.stem}{suffix}")


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(
    description="Format EPUB layout or deobfuscate filenames with Python stdlib only",
  )
  parser.add_argument("operation", choices=("inspect", "format", "deobfuscate-filenames", "normalize"))
  parser.add_argument("input", type=Path)
  parser.add_argument("--output", type=Path, help="Output EPUB path for rewrite operations")
  parser.add_argument("--dry-run", action="store_true", help="Plan a rewrite without writing an EPUB")
  parser.add_argument("--force", action="store_true", help="Replace an existing output file")
  parser.add_argument("--report-format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  try:
    if args.operation == "inspect":
      if args.output or args.dry_run:
        parser.error("inspect does not accept --output or --dry-run")
      report = inspect_epub(args.input)
    else:
      output = args.output or default_output(args.input, args.operation)
      if args.operation == "normalize":
        report = normalize_epub(args.input, output, force=args.force, dry_run=args.dry_run)
      else:
        report = rewrite_epub(args.input, output, args.operation, force=args.force, dry_run=args.dry_run)
  except StructureToolError as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1

  if args.report_format == "json":
    print(json.dumps(report.as_dict(), ensure_ascii=False, indent=2))
  else:
    print(render_markdown(report), end="")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
