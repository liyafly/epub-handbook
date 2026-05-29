#!/usr/bin/env python3
"""One-click EPUB 2/legacy cleanup to EPUB 3.

This script is intentionally narrower than a full editor:
- it rewrites package/navigation structure for EPUB 3;
- it fixes common broken NCX fragment quoting from Kindle/MOBI round-trips;
- it normalizes local plain footnotes into the project popup-footnote shape;
- it injects a separate CJK literary typography override stylesheet;
- it repackages the EPUB with the required mimetype ZIP entry.

It does not rewrite prose, embed new fonts, optimize images, or depend on Sigil.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import posixpath
import re
import sys
import zipfile
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable
from xml.etree import ElementTree as ET
from xml.sax.saxutils import escape


ROOT = Path(__file__).resolve().parents[1]
NOTE_ASSET = ROOT / "skills" / "epub-popup-footnote-converter" / "assets" / "note.png"

CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
OPF_URI = "http://www.idpf.org/2007/opf"
DC_URI = "http://purl.org/dc/elements/1.1/"
DCTERMS_URI = "http://purl.org/dc/terms/"
NCX_URI = "http://www.daisy.org/z3986/2005/ncx/"
XHTML_URI = "http://www.w3.org/1999/xhtml"
OPS_URI = "http://www.idpf.org/2007/ops"
IBOOKS_PREFIX = "http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/"
RENDITION_PREFIX = "http://www.idpf.org/vocab/rendition/#"

CONTAINER_NS = {"c": CONTAINER_URI}
OPF_NS = {"opf": OPF_URI, "dc": DC_URI}
NCX_NS = {"ncx": NCX_URI}

FONT_MEDIA_TYPES = {
  "application/x-font-ttf",
  "application/x-font-opentype",
  "application/font-sfnt",
  "font/ttf",
  "font/otf",
}

IMAGE_MEDIA_BY_EXT = {
  ".gif": "image/gif",
  ".jpeg": "image/jpeg",
  ".jpg": "image/jpeg",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".webp": "image/webp",
}

FIXED_ZIP_TIME = (1980, 1, 1, 0, 0, 0)


ET.register_namespace("", OPF_URI)
ET.register_namespace("dc", DC_URI)
ET.register_namespace("dcterms", DCTERMS_URI)
ET.register_namespace("opf", OPF_URI)


class ConversionError(Exception):
  """The input cannot be converted safely by this script."""


@dataclass
class ConversionReport:
  input_sha256: str
  output: str
  opf: str = ""
  package_version_before: str | None = None
  nav_entries: int = 0
  xhtml_files_updated: int = 0
  stylesheet_links_added: int = 0
  plain_notes_converted: int = 0
  duokan_notes_normalized: int = 0
  manifest_items_added: list[str] = field(default_factory=list)
  manifest_items_updated: int = 0
  metadata_updates: list[str] = field(default_factory=list)
  warnings: list[str] = field(default_factory=list)

  def as_dict(self) -> dict[str, object]:
    return {
      "harness": "epub3_oneclick_converter",
      "input_sha256": self.input_sha256,
      "output": self.output,
      "opf": self.opf,
      "package_version_before": self.package_version_before,
      "nav_entries": self.nav_entries,
      "xhtml_files_updated": self.xhtml_files_updated,
      "stylesheet_links_added": self.stylesheet_links_added,
      "plain_notes_converted": self.plain_notes_converted,
      "duokan_notes_normalized": self.duokan_notes_normalized,
      "manifest_items_added": self.manifest_items_added,
      "manifest_items_updated": self.manifest_items_updated,
      "metadata_updates": self.metadata_updates,
      "warnings": self.warnings,
    }


def q(uri: str, name: str) -> str:
  return f"{{{uri}}}{name}"


def local_name(tag: object) -> str:
  if not isinstance(tag, str):
    return ""
  return tag.rsplit("}", 1)[-1]


def sha256_file(path: Path) -> str:
  digest = hashlib.sha256()
  with path.open("rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
      digest.update(chunk)
  return digest.hexdigest()


def parse_xml(data: bytes | str, label: str) -> ET.Element:
  if isinstance(data, str):
    data = data.encode("utf-8")
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    raise ConversionError(f"{label}: XML parse failed: {exc}") from exc


def sanitize_xml_text(data: bytes) -> str:
  text = data.decode("utf-8", errors="replace")
  text = re.sub(r"<!DOCTYPE[^>]*>", "", text, flags=re.I | re.S)
  text = text.replace("&nbsp;", "&#160;")
  return text


def sanitize_ncx_text(data: bytes, report: ConversionReport) -> str:
  text = sanitize_xml_text(data)
  fixed, count = re.subn(
    r'(<content\b[^>]*\bsrc=)(["\'])([^"\']+?)(["\'])(#[^"\'>\s/]+)',
    r"\1\2\3\5\4",
    text,
    flags=re.I,
  )
  if count:
    report.warnings.append(f"fixed malformed NCX content src fragment quoting: {count}")
  return fixed


def norm_join(base: str, href: str) -> str:
  clean = href.split("#", 1)[0]
  return posixpath.normpath(posixpath.join(base, clean))


def href_with_fragment(base: str, href: str) -> str:
  clean, sep, fragment = href.partition("#")
  path = posixpath.normpath(posixpath.join(base, clean)) if clean else ""
  return f"{path}{sep}{fragment}" if sep else path


def rel_href(from_zip_path: str, to_zip_path: str) -> str:
  base = posixpath.dirname(from_zip_path)
  return posixpath.relpath(to_zip_path, base) if base else to_zip_path


def split_props(value: str | None) -> list[str]:
  return [part for part in (value or "").split() if part]


def add_props(elem: ET.Element, *props: str) -> bool:
  current = split_props(elem.attrib.get("properties"))
  changed = False
  for prop in props:
    if prop and prop not in current:
      current.append(prop)
      changed = True
  if changed:
    elem.set("properties", " ".join(current))
  return changed


def remove_props(elem: ET.Element, *props: str) -> bool:
  current = split_props(elem.attrib.get("properties"))
  updated = [prop for prop in current if prop not in props]
  if updated != current:
    if updated:
      elem.set("properties", " ".join(updated))
    elif "properties" in elem.attrib:
      del elem.attrib["properties"]
    return True
  return False


def metadata(root: ET.Element) -> ET.Element:
  node = root.find("opf:metadata", OPF_NS)
  if node is None:
    raise ConversionError("OPF missing metadata")
  return node


def manifest(root: ET.Element) -> ET.Element:
  node = root.find("opf:manifest", OPF_NS)
  if node is None:
    raise ConversionError("OPF missing manifest")
  return node


def spine(root: ET.Element) -> ET.Element:
  node = root.find("opf:spine", OPF_NS)
  if node is None:
    raise ConversionError("OPF missing spine")
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


def href_exists(root: ET.Element, href: str) -> ET.Element | None:
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    if item.attrib.get("href") == href:
      return item
  return None


def unique_href(files: dict[str, bytes], opf_dir: str, href: str) -> str:
  stem, ext = posixpath.splitext(href)
  candidate = href
  index = 2
  while norm_join(opf_dir, candidate) in files:
    candidate = f"{stem}-{index}{ext}"
    index += 1
  return candidate


def add_manifest_item(
  root: ET.Element,
  report: ConversionReport,
  item_id_base: str,
  href: str,
  media_type: str,
  properties: str | None = None,
) -> ET.Element:
  existing = href_exists(root, href)
  if existing is not None:
    if properties and add_props(existing, *properties.split()):
      report.manifest_items_updated += 1
    return existing
  attrs = {
    "id": unique_id(root, item_id_base),
    "href": href,
    "media-type": media_type,
  }
  if properties:
    attrs["properties"] = properties
  item = ET.SubElement(manifest(root), q(OPF_URI, "item"), attrs)
  report.manifest_items_added.append(href)
  return item


def add_package_prefix(root: ET.Element, name: str, uri: str) -> None:
  prefix = root.attrib.get("prefix", "")
  if re.search(rf"(?:^|\s){re.escape(name)}\s*:", prefix):
    return
  addition = f"{name}: {uri}"
  root.set("prefix", f"{prefix.strip()} {addition}".strip())


def text_content(elem: ET.Element | None) -> str:
  if elem is None:
    return ""
  return " ".join("".join(elem.itertext()).split())


def attr_escape(value: str) -> str:
  return escape(value, {'"': "&quot;"})


def normalize_metadata(root: ET.Element, report: ConversionReport) -> None:
  meta = metadata(root)
  root.set("version", "3.0")
  add_package_prefix(root, "rendition", RENDITION_PREFIX)
  add_package_prefix(root, "ibooks", IBOOKS_PREFIX)

  for child in list(meta):
    if child.tag != q(DC_URI, "date"):
      continue
    event = child.attrib.pop(q(OPF_URI, "event"), child.attrib.pop("event", "")).lower()
    if event == "modification":
      meta.remove(child)
      report.metadata_updates.append("removed legacy modification dc:date")
    elif event == "creation":
      created = ET.Element(q(OPF_URI, "meta"), {"property": "dcterms:created"})
      created.text = child.text
      index = list(meta).index(child)
      meta.remove(child)
      meta.insert(index, created)
      report.metadata_updates.append("mapped creation dc:date to dcterms:created")
    elif event in {"publication", "issued"}:
      child.attrib.clear()

  modified = None
  specified_fonts = None
  for child in meta.findall("opf:meta", OPF_NS):
    prop = child.attrib.get("property")
    if prop == "dcterms:modified":
      modified = child
    elif prop == "ibooks:specified-fonts":
      specified_fonts = child

  now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
  if modified is None:
    modified = ET.SubElement(meta, q(OPF_URI, "meta"), {"property": "dcterms:modified"})
    report.metadata_updates.append("added dcterms:modified")
  else:
    report.metadata_updates.append("updated dcterms:modified")
  modified.text = now

  if specified_fonts is None:
    specified_fonts = ET.SubElement(meta, q(OPF_URI, "meta"), {"property": "ibooks:specified-fonts"})
    specified_fonts.text = "true"
    report.metadata_updates.append("added ibooks:specified-fonts")


def manifest_maps(root: ET.Element, opf_dir: str) -> tuple[dict[str, ET.Element], dict[str, ET.Element]]:
  by_id: dict[str, ET.Element] = {}
  by_zip: dict[str, ET.Element] = {}
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    item_id = item.attrib.get("id")
    href = item.attrib.get("href")
    if item_id:
      by_id[item_id] = item
    if href:
      by_zip[norm_join(opf_dir, href)] = item
  return by_id, by_zip


def find_cover_id(root: ET.Element) -> str | None:
  for child in metadata(root).findall("opf:meta", OPF_NS):
    if child.attrib.get("name") == "cover":
      return child.attrib.get("content")
  return None


def ensure_cover_properties(root: ET.Element, report: ConversionReport) -> None:
  cover_id = find_cover_id(root)
  if not cover_id:
    return
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    if item.attrib.get("id") == cover_id:
      if add_props(item, "cover-image"):
        report.manifest_items_updated += 1
      return


def normalize_manifest_media(root: ET.Element, report: ConversionReport) -> None:
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    media_type = item.attrib.get("media-type", "")
    href = item.attrib.get("href", "")
    suffix = Path(href).suffix.lower()
    changed = False
    if media_type in FONT_MEDIA_TYPES or suffix in {".ttf", ".otf"}:
      if media_type != "application/vnd.ms-opentype":
        item.set("media-type", "application/vnd.ms-opentype")
        changed = True
    elif suffix in IMAGE_MEDIA_BY_EXT and media_type != IMAGE_MEDIA_BY_EXT[suffix]:
      item.set("media-type", IMAGE_MEDIA_BY_EXT[suffix])
      changed = True
    if changed:
      report.manifest_items_updated += 1


def itemref_exists(root: ET.Element, item_id: str) -> bool:
  return any(itemref.attrib.get("idref") == item_id for itemref in root.findall("opf:spine/opf:itemref", OPF_NS))


def ensure_nav_in_spine(root: ET.Element, nav_id: str) -> None:
  if itemref_exists(root, nav_id):
    return
  ET.SubElement(spine(root), q(OPF_URI, "itemref"), {"idref": nav_id, "linear": "no"})


def ncx_item(root: ET.Element) -> ET.Element | None:
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    if item.attrib.get("media-type") == "application/x-dtbncx+xml":
      return item
  return None


def ensure_spine_toc(root: ET.Element) -> None:
  ncx = ncx_item(root)
  if ncx is not None and ncx.attrib.get("id"):
    spine(root).set("toc", ncx.attrib["id"])


def fix_guide_hrefs(root: ET.Element, files: dict[str, bytes], opf_dir: str, report: ConversionReport) -> None:
  guide = root.find("opf:guide", OPF_NS)
  if guide is None:
    return
  for ref in guide.findall("opf:reference", OPF_NS):
    href = ref.attrib.get("href", "")
    if not href or norm_join(opf_dir, href) in files:
      continue
    candidate = href
    while candidate.startswith("../"):
      candidate = candidate[3:]
      if norm_join(opf_dir, candidate) in files:
        ref.set("href", candidate)
        report.manifest_items_updated += 1
        report.warnings.append(f"fixed guide href: {href} -> {candidate}")
        break


def parse_nav_points(point: ET.Element, base: str) -> dict[str, object] | None:
  label = text_content(point.find("ncx:navLabel/ncx:text", NCX_NS))
  content = point.find("ncx:content", NCX_NS)
  src = content.attrib.get("src", "") if content is not None else ""
  children = [
    child for child in (parse_nav_points(node, base) for node in point.findall("ncx:navPoint", NCX_NS))
    if child is not None
  ]
  if not src and not children:
    return None
  return {
    "label": label or src or "Untitled",
    "href": href_with_fragment(base, src) if src else "",
    "children": children,
  }


def nav_count(entries: Iterable[dict[str, object]]) -> int:
  total = 0
  for entry in entries:
    total += 1
    total += nav_count(entry.get("children", []))  # type: ignore[arg-type]
  return total


def ncx_entries(files: dict[str, bytes], root: ET.Element, opf_dir: str, report: ConversionReport) -> list[dict[str, object]]:
  item = ncx_item(root)
  if item is None or not item.attrib.get("href"):
    return []
  ncx_href = item.attrib["href"]
  ncx_zip = norm_join(opf_dir, ncx_href)
  if ncx_zip not in files:
    report.warnings.append(f"NCX manifest item does not resolve: {ncx_href}")
    return []
  sanitized = sanitize_ncx_text(files[ncx_zip], report)
  files[ncx_zip] = sanitized.encode("utf-8")
  ncx_root = parse_xml(sanitized, ncx_zip)
  base = posixpath.dirname(ncx_href)
  points = ncx_root.findall("ncx:navMap/ncx:navPoint", NCX_NS)
  return [entry for entry in (parse_nav_points(point, base) for point in points) if entry is not None]


def spine_entries(root: ET.Element) -> list[dict[str, object]]:
  by_id = {
    item.attrib.get("id"): item.attrib.get("href", "")
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
  }
  entries: list[dict[str, object]] = []
  for itemref in root.findall("opf:spine/opf:itemref", OPF_NS):
    href = by_id.get(itemref.attrib.get("idref"), "")
    if href:
      label = posixpath.basename(href).rsplit(".", 1)[0]
      entries.append({"label": label or href, "href": href, "children": []})
  return entries


def package_title(root: ET.Element) -> str:
  return text_content(root.find(".//dc:title", OPF_NS)) or "目录"


def package_language(root: ET.Element) -> str:
  return text_content(root.find(".//dc:language", OPF_NS)) or "und"


def render_nav_items(entries: list[dict[str, object]], indent: str = "        ") -> str:
  lines: list[str] = []
  for entry in entries:
    href = escape(str(entry.get("href", "")), {'"': "&quot;"})
    label = escape(str(entry.get("label", "")))
    children = entry.get("children", [])
    if href:
      lines.append(f'{indent}<li><a href="{href}">{label}</a>')
    else:
      lines.append(f"{indent}<li><span>{label}</span>")
    if children:
      lines.append(f"{indent}  <ol>")
      lines.append(render_nav_items(children, indent + "    "))
      lines.append(f"{indent}  </ol>")
    lines.append(f"{indent}</li>")
  return "\n".join(lines)


GUIDE_TYPE_TO_EPUB = {
  "cover": "cover",
  "toc": "toc",
  "text": "bodymatter",
  "title-page": "titlepage",
  "copyright-page": "copyright-page",
}


def guide_landmarks(root: ET.Element) -> list[tuple[str, str, str]]:
  guide = root.find("opf:guide", OPF_NS)
  if guide is None:
    return []
  landmarks: list[tuple[str, str, str]] = []
  for ref in guide.findall("opf:reference", OPF_NS):
    href = ref.attrib.get("href", "")
    guide_type = ref.attrib.get("type", "")
    epub_type = GUIDE_TYPE_TO_EPUB.get(guide_type)
    if href and epub_type:
      landmarks.append((epub_type, ref.attrib.get("title", guide_type), href))
  return landmarks


def build_nav_xhtml(root: ET.Element, entries: list[dict[str, object]]) -> bytes:
  lang = attr_escape(package_language(root))
  title = escape(package_title(root))
  items = render_nav_items(entries)
  landmarks = guide_landmarks(root)
  landmark_block = ""
  if landmarks:
    landmark_items = "\n".join(
      f'        <li><a epub:type="{attr_escape(epub_type)}" href="{attr_escape(href)}">{escape(label)}</a></li>'
      for epub_type, label, href in landmarks
    )
    landmark_block = f'''
    <nav epub:type="landmarks" hidden="hidden" id="landmarks">
      <h2>Landmarks</h2>
      <ol>
{landmark_items}
      </ol>
    </nav>'''
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="{XHTML_URI}" xmlns:epub="{OPS_URI}" xml:lang="{lang}" lang="{lang}">
  <head>
    <title>{title}目录</title>
  </head>
  <body>
    <nav epub:type="toc" id="toc">
      <h1>{title}</h1>
      <ol>
{items}
      </ol>
    </nav>{landmark_block}
  </body>
</html>
'''.encode("utf-8")


def ensure_nav(files: dict[str, bytes], root: ET.Element, opf_path: str, report: ConversionReport) -> None:
  opf_dir = posixpath.dirname(opf_path)
  navs = [
    item for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if "nav" in split_props(item.attrib.get("properties"))
  ]
  if len(navs) > 1:
    for extra in navs[1:]:
      remove_props(extra, "nav")
      report.manifest_items_updated += 1
    navs = navs[:1]

  if navs:
    nav_item = navs[0]
    nav_id = nav_item.attrib.get("id") or unique_id(root, "nav")
    nav_item.set("id", nav_id)
    ensure_nav_in_spine(root, nav_id)
    return

  entries = ncx_entries(files, root, opf_dir, report) or spine_entries(root)
  if not entries:
    raise ConversionError("cannot build nav.xhtml: no NCX navPoint or spine entries")
  nav_href = unique_href(files, opf_dir, "nav.xhtml")
  nav_zip = norm_join(opf_dir, nav_href)
  files[nav_zip] = build_nav_xhtml(root, entries)
  nav_item = add_manifest_item(root, report, "nav", nav_href, "application/xhtml+xml", "nav")
  ensure_nav_in_spine(root, nav_item.attrib["id"])
  report.nav_entries = nav_count(entries)


def enhancement_css() -> bytes:
  return b'''/* EPUB 3 CJK literary cleanup layer. Keep linked after source stylesheets. */
html,
body {
  margin: 0;
  padding: 0;
}

body {
  font-family: "Songti SC", "SimSun", "Source Han Serif SC", serif;
  line-height: 1.65;
  text-align: justify;
  text-justify: inter-ideograph;
  word-break: normal;
  overflow-wrap: anywhere;
  color: #1f1a17;
}

p {
  margin: 0.35em 0;
  line-height: 1.65;
  text-indent: 2em;
}

h1,
h2,
h3,
h4,
h5,
.cp,
.front,
.back,
.zw-text1,
.chapter-title1,
.fronttitle1,
.backtitle1,
.backtitle2,
.kindle-cn-toc-title,
.kindle-en-toc-title {
  font-family: "Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
  text-indent: 0;
  page-break-after: avoid;
  break-after: avoid;
  page-break-inside: avoid;
  break-inside: avoid;
}

h1,
h1.front,
h1.back,
h1.zw-text1 {
  margin: 1.4em auto 1.2em;
  color: #6f4d35;
  line-height: 1.35;
}

h2,
h3,
h4,
h5 {
  margin: 1.1em 0 0.75em;
  color: #6f4d35;
  line-height: 1.4;
}

.part-text,
.part-textc,
.part-textf,
.block,
.block1,
.block2,
.block3,
.img,
.note,
.footnote,
.fs,
.kt,
.kh {
  font-family: "Kaiti SC", "STKaiti", "KaiTi", serif;
}

.center,
.block2,
.img,
.cover {
  text-align: center;
  text-indent: 0;
}

.right,
.block1 {
  text-align: right;
}

.left {
  text-align: left;
  text-indent: 0;
}

blockquote,
.block,
.block3 {
  margin: 0.8em 0 0.8em 2em;
}

img {
  max-width: 100%;
  height: auto;
}

.cover img,
img.cover,
img.body-image-alone {
  max-width: 100%;
  height: auto;
}

a {
  color: inherit;
}

sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon {
  text-decoration: none;
}

.noteref-icon img {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

aside[epub|type~="footnote"],
aside[role~="doc-footnote"] {
  margin-top: 1.4em;
}

.footnote-line,
hr.xian {
  width: 60%;
  height: 1px;
  margin: 1.5em 0 1em -0.5em;
  border: none;
  border-top: 1px solid #777;
}

.footnote-list {
  margin: 0;
  padding: 0;
  list-style-type: none;
  text-align: left;
}

.footnote-item {
  margin: 0.4em 0;
  padding: 0;
  list-style-type: none;
}

.footnote {
  margin: 0.4em 0;
  text-indent: 0;
  font-size: 0.9em;
  line-height: 1.45;
  text-align: left;
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
'''


def note_png_bytes() -> bytes:
  if NOTE_ASSET.exists():
    return NOTE_ASSET.read_bytes()
  return base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAwAAAAMCAYAAABWdVznAAAAHklEQVR4nGNgGAWjYBSMglEwCkbBKB"
    "gFo2AUDAMABRwAAf1xD6YAAAAASUVORK5CYII="
  )


XML_DECL_RE = re.compile(r"^\s*<\?xml[^>]*\?>", re.I)
DOCTYPE_RE = re.compile(r"<!DOCTYPE[^>]*>", re.I | re.S)
HTML_TAG_RE = re.compile(r"<html\b([^>]*)>", re.I)
HEAD_END_RE = re.compile(r"</head\s*>", re.I)
META_HTTP_RE = re.compile(
  r"<meta\b(?=[^>]*http-equiv=[\"']Content-Type[\"'])(?=[^>]*charset=utf-8)[^>]*/?>",
  re.I,
)


def normalize_xhtml_shell(text: str) -> tuple[str, bool]:
  changed = False
  if DOCTYPE_RE.search(text):
    text = DOCTYPE_RE.sub("", text, count=1)
    changed = True
  declaration = ""
  match = XML_DECL_RE.match(text)
  if match:
    declaration = match.group(0).strip() + "\n"
    text = text[match.end():].lstrip()
  if not text.lstrip().lower().startswith("<!doctype html>"):
    text = declaration + "<!DOCTYPE html>\n" + text.lstrip()
    changed = True
  elif declaration:
    text = declaration + text.lstrip()

  def html_repl(match: re.Match[str]) -> str:
    nonlocal changed
    attrs = match.group(1)
    if "xmlns:epub" not in attrs:
      attrs += f' xmlns:epub="{OPS_URI}"'
      changed = True
    if "lang=" not in attrs and "xml:lang=" in attrs:
      lang_match = re.search(r'xml:lang=(["\'])(.*?)\1', attrs)
      if lang_match:
        attrs += f' lang="{lang_match.group(2)}"'
        changed = True
    return f"<html{attrs}>"

  text = HTML_TAG_RE.sub(html_repl, text, count=1)
  if META_HTTP_RE.search(text):
    text = META_HTTP_RE.sub('<meta charset="utf-8"/>', text, count=1)
    changed = True
  elif "<meta charset=" not in text.lower() and HEAD_END_RE.search(text):
    text = HEAD_END_RE.sub('  <meta charset="utf-8"/>\n</head>', text, count=1)
    changed = True
  if "<big" in text.lower():
    text = re.sub(r"<big\b([^>]*)>", r'<span\1 class="big">', text, flags=re.I)
    text = re.sub(r"</big\s*>", "</span>", text, flags=re.I)
    changed = True
  return text, changed


def ensure_stylesheet_link(text: str, href: str) -> tuple[str, bool]:
  if href in text:
    return text, False
  link = f'  <link href="{href}" type="text/css" rel="stylesheet"/>\n'
  updated, count = HEAD_END_RE.subn(link + "</head>", text, count=1)
  return updated, bool(count)


PLAIN_NOTEREF_RE = re.compile(
  r'<a\s+id="w(?P<num>\d+)"></a>\s*'
  r'<a\s+href="(?P<href>[^"]*#m(?P=num))">\s*<sup>\[(?P=num)\]</sup>\s*</a>',
  re.S,
)
PLAIN_NOTE_RE = re.compile(
  r'\s*<p\s+class="note"\s*>\s*'
  r'<a\s+id="m(?P<num>\d+)"></a>\s*'
  r'<a\s+href="[^"]*#w(?P=num)">\[(?P=num)\]</a>\s*'
  r'(?P<body>.*?)</p>',
  re.S,
)
HR_BEFORE_NOTES_RE = re.compile(r"\s*<hr\b[^>]*/?>\s*$", re.I | re.S)


def convert_plain_notes(text: str, note_href: str) -> tuple[str, int]:
  matches = list(PLAIN_NOTE_RE.finditer(text))
  if not matches:
    return text, 0

  note_ids = {match.group("num") for match in matches}

  def marker_repl(match: re.Match[str]) -> str:
    num = match.group("num")
    if num not in note_ids:
      return match.group(0)
    return (
      f'<sup><a id="w{num}" class="noteref-icon" epub:type="noteref" '
      f'role="doc-noteref" href="#m{num}"><img alt="注" src="{note_href}"/></a></sup>'
    )

  first = matches[0]
  last = matches[-1]
  prefix = text[:first.start()]
  suffix = text[last.end():]
  hr_match = HR_BEFORE_NOTES_RE.search(prefix)
  if hr_match:
    prefix = prefix[:hr_match.start()]

  lines = [
    '  <aside epub:type="footnote" role="doc-footnote">',
    "    <div><hr class=\"footnote-line xian\"/></div>",
    '    <ol class="footnote-list">',
  ]
  for match in matches:
    num = match.group("num")
    body = match.group("body").strip()
    lines.extend([
      f'      <li class="footnote-item" id="m{num}">',
      f'        <p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#w{num}">◎</a>{body}</p>',
      "      </li>",
    ])
  lines.extend(["    </ol>", "  </aside>"])
  rebuilt = prefix + "\n" + "\n".join(lines) + suffix
  rebuilt = PLAIN_NOTEREF_RE.sub(marker_repl, rebuilt)
  return rebuilt, len(matches)


def normalize_duokan_notes(text: str) -> tuple[str, int]:
  if "duokan-footnote" not in text and 'epub:type="footnote"' not in text:
    return text, 0
  count = 0
  updated = text
  updated, n = re.subn(r'<aside\s+epub:type="footnote"(?![^>]*\brole=)', '<aside epub:type="footnote" role="doc-footnote"', updated)
  count += n
  updated, n = re.subn(r'class="duokan-footnote-content"', 'class="footnote-list"', updated)
  count += n
  updated, n = re.subn(r'class="duokan-footnote-item"', 'class="footnote-item"', updated)
  count += n
  updated, n = re.subn(r'class="duokan-footnote"', 'class="noteref-icon"', updated)
  count += n
  updated, n = re.subn(r">⊙</a>", ">◎</a>", updated)
  count += n
  return updated, count


def update_xhtml_files(
  files: dict[str, bytes],
  root: ET.Element,
  opf_path: str,
  style_zip_path: str,
  note_zip_path: str,
  report: ConversionReport,
  popup_notes: bool,
  typography: bool,
) -> None:
  opf_dir = posixpath.dirname(opf_path)
  _, by_zip = manifest_maps(root, opf_dir)
  for zip_path, item in sorted(by_zip.items()):
    if item.attrib.get("media-type") != "application/xhtml+xml" or zip_path not in files:
      continue
    original = files[zip_path]
    text = original.decode("utf-8", errors="replace")
    text, changed = normalize_xhtml_shell(text)
    if typography:
      style_href = rel_href(zip_path, style_zip_path)
      text, linked = ensure_stylesheet_link(text, style_href)
      if linked:
        report.stylesheet_links_added += 1
        changed = True
    if popup_notes:
      note_href = rel_href(zip_path, note_zip_path)
      text, notes = convert_plain_notes(text, note_href)
      if notes:
        report.plain_notes_converted += notes
        changed = True
      text, normalized = normalize_duokan_notes(text)
      if normalized:
        report.duokan_notes_normalized += normalized
        changed = True
    if re.search(r"<(?:svg|svg:svg)\b", text, re.I) and add_props(item, "svg"):
      report.manifest_items_updated += 1
    if re.search(r"<(?:math|m:math)\b", text, re.I) and add_props(item, "mathml"):
      report.manifest_items_updated += 1
    if re.search(r"<script\b", text, re.I) and add_props(item, "scripted"):
      report.manifest_items_updated += 1
    if changed:
      files[zip_path] = text.encode("utf-8")
      report.xhtml_files_updated += 1


def read_epub_files(input_path: Path) -> tuple[dict[str, bytes], list[str]]:
  try:
    with zipfile.ZipFile(input_path) as zf:
      return {name: zf.read(name) for name in zf.namelist()}, zf.namelist()
  except zipfile.BadZipFile as exc:
    raise ConversionError(f"not a valid EPUB zip: {input_path}") from exc


def opf_path_from_container(files: dict[str, bytes]) -> str:
  if "META-INF/container.xml" not in files:
    raise ConversionError("missing META-INF/container.xml")
  container = parse_xml(files["META-INF/container.xml"], "META-INF/container.xml")
  rootfile = container.find(".//c:rootfile", CONTAINER_NS)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
  if not opf_path or opf_path not in files:
    raise ConversionError(f"container rootfile does not resolve: {opf_path or '<missing>'}")
  return opf_path


def write_epub(output_path: Path, files: dict[str, bytes], original_order: list[str]) -> None:
  output_path.parent.mkdir(parents=True, exist_ok=True)
  ordered: list[str] = ["mimetype"]
  ordered.extend(name for name in original_order if name != "mimetype" and name in files)
  ordered.extend(name for name in sorted(files) if name not in set(ordered))
  with zipfile.ZipFile(output_path, "w") as zf:
    for name in ordered:
      data = files[name]
      info = zipfile.ZipInfo(name, FIXED_ZIP_TIME)
      info.compress_type = zipfile.ZIP_STORED if name == "mimetype" else zipfile.ZIP_DEFLATED
      zf.writestr(info, data)


def default_output_path(input_path: Path) -> Path:
  return input_path.with_name(f"{input_path.stem}_epub3_clean.epub")


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

  normalize_metadata(root, report)
  normalize_manifest_media(root, report)
  ensure_cover_properties(root, report)
  ensure_spine_toc(root)
  fix_guide_hrefs(root, files, opf_dir, report)

  style_href = unique_href(files, opf_dir, "Styles/epub3-enhancements.css")
  style_zip = norm_join(opf_dir, style_href)
  note_href = unique_href(files, opf_dir, "Images/note.png")
  note_zip = norm_join(opf_dir, note_href)

  if typography:
    files[style_zip] = enhancement_css()
    add_manifest_item(root, report, "epub3-enhancements-css", style_href, "text/css")
  if popup_notes:
    files[note_zip] = note_png_bytes()
    add_manifest_item(root, report, "note-icon", note_href, "image/png")

  update_xhtml_files(
    files,
    root,
    opf_path,
    style_zip,
    note_zip,
    report,
    popup_notes=popup_notes,
    typography=typography,
  )

  ensure_nav(files, root, opf_path, report)
  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  write_epub(output_path, files, original_order)
  return report


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Convert a legacy EPUB to EPUB 3 with popup notes and CJK typography cleanup")
  parser.add_argument("input", type=Path, help="Input EPUB")
  parser.add_argument("--output", type=Path, help="Output EPUB path; defaults to *_epub3_clean.epub beside input")
  parser.add_argument("--no-popup-notes", action="store_true", help="Skip plain/duokan footnote normalization")
  parser.add_argument("--no-typography", action="store_true", help="Skip CJK typography override stylesheet injection")
  parser.add_argument("--format", choices=("json", "text"), default="text")
  args = parser.parse_args(argv)

  output = args.output or default_output_path(args.input)
  try:
    report = convert_epub(
      args.input,
      output,
      popup_notes=not args.no_popup_notes,
      typography=not args.no_typography,
    )
  except ConversionError as exc:
    data = {"harness": "epub3_oneclick_converter", "error": str(exc)}
    if args.format == "json":
      print(json.dumps(data, ensure_ascii=False, indent=2))
    else:
      print(f"ERROR: {exc}", file=sys.stderr)
    return 1

  data = report.as_dict()
  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(f"Wrote EPUB 3: {output}")
    print(f"Converted plain notes: {report.plain_notes_converted}")
    print(f"Added stylesheet links: {report.stylesheet_links_added}")
    if report.warnings:
      print("Warnings:")
      for warning in report.warnings:
        print(f"- {warning}")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
