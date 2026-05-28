#!/usr/bin/env python3
"""Plan or write a minimal EPUB 3 package migration.

The migration is intentionally conservative:
- update OPF `version` to 3.0;
- add `meta property="dcterms:modified"` when missing;
- add an EPUB 3 `nav.xhtml` item when the package has no nav;
- keep `toc.ncx` and `spine toc="ncx"` for legacy/Kindle compatibility;
- never rewrite XHTML body content.
"""

from __future__ import annotations

import argparse
import copy
import json
import posixpath
import shlex
import sys
import zipfile
from datetime import datetime, timezone
from pathlib import Path
from xml.etree import ElementTree as ET
from xml.sax.saxutils import escape


CONTAINER_URI = "urn:oasis:names:tc:opendocument:xmlns:container"
OPF_URI = "http://www.idpf.org/2007/opf"
DC_URI = "http://purl.org/dc/elements/1.1/"
DCTERMS_URI = "http://purl.org/dc/terms/"
NCX_URI = "http://www.daisy.org/z3986/2005/ncx/"
XHTML_URI = "http://www.w3.org/1999/xhtml"
OPS_URI = "http://www.idpf.org/2007/ops"
XML_URI = "http://www.w3.org/XML/1998/namespace"

NS = {
  "c": CONTAINER_URI,
  "opf": OPF_URI,
  "dc": DC_URI,
  "dcterms": DCTERMS_URI,
  "ncx": NCX_URI,
}


ET.register_namespace("", OPF_URI)
ET.register_namespace("dc", DC_URI)
ET.register_namespace("dcterms", DCTERMS_URI)
ET.register_namespace("epub", OPS_URI)
ET.register_namespace("xml", XML_URI)


class Epub3Error(Exception):
  """Input cannot be migrated safely by this harness."""


def tag(uri: str, name: str) -> str:
  return f"{{{uri}}}{name}"


def split_props(value: str | None) -> set[str]:
  return set((value or "").split())


def norm_join(base: str, href: str) -> str:
  return posixpath.normpath(posixpath.join(base, href.split("#", 1)[0]))


def parse_xml(data: bytes, label: str) -> ET.Element:
  try:
    return ET.fromstring(data)
  except ET.ParseError as exc:
    raise Epub3Error(f"{label}: XML parse failed: {exc}") from exc


def read_package(zf: zipfile.ZipFile) -> tuple[str, ET.Element]:
  names = set(zf.namelist())
  if "META-INF/container.xml" not in names:
    raise Epub3Error("missing META-INF/container.xml")
  container = parse_xml(zf.read("META-INF/container.xml"), "META-INF/container.xml")
  rootfile = container.find(".//c:rootfile", NS)
  opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
  if not opf_path or opf_path not in names:
    raise Epub3Error(f"container.xml rootfile does not resolve: {opf_path or '<missing>'}")
  return opf_path, parse_xml(zf.read(opf_path), opf_path)


def manifest(root: ET.Element) -> ET.Element:
  node = root.find("opf:manifest", NS)
  if node is None:
    raise Epub3Error("OPF missing manifest")
  return node


def metadata(root: ET.Element) -> ET.Element:
  node = root.find("opf:metadata", NS)
  if node is None:
    raise Epub3Error("OPF missing metadata")
  return node


def item_id_exists(root: ET.Element, item_id: str) -> bool:
  return any(item.attrib.get("id") == item_id for item in root.findall("opf:manifest/opf:item", NS))


def unique_item_id(root: ET.Element, base: str) -> str:
  candidate = base
  index = 2
  while item_id_exists(root, candidate):
    candidate = f"{base}-{index}"
    index += 1
  return candidate


def choose_nav_href(opf_dir: str, names: set[str]) -> str:
  href = "nav.xhtml"
  index = 2
  while norm_join(opf_dir, href) in names:
    href = f"nav-{index}.xhtml"
    index += 1
  return href


def nav_items(root: ET.Element) -> list[ET.Element]:
  return [
    item for item in root.findall("opf:manifest/opf:item", NS)
    if "nav" in split_props(item.attrib.get("properties"))
  ]


def ncx_item(root: ET.Element) -> ET.Element | None:
  for item in root.findall("opf:manifest/opf:item", NS):
    if item.attrib.get("media-type") == "application/x-dtbncx+xml" or item.attrib.get("id") == "ncx":
      return item
  return None


def text_content(elem: ET.Element | None) -> str:
  if elem is None:
    return ""
  return " ".join("".join(elem.itertext()).split())


def attr_escape(value: str) -> str:
  return escape(value, {'"': "&quot;"})


def shell_path(path: Path | str) -> str:
  return shlex.quote(str(path))


def language(root: ET.Element) -> str:
  value = text_content(root.find(".//dc:language", NS))
  return value or "und"


def spine_entries(root: ET.Element) -> list[tuple[str, str]]:
  href_by_id = {
    item.attrib.get("id"): item.attrib.get("href", "")
    for item in root.findall("opf:manifest/opf:item", NS)
  }
  entries: list[tuple[str, str]] = []
  for itemref in root.findall("opf:spine/opf:itemref", NS):
    href = href_by_id.get(itemref.attrib.get("idref"), "")
    if href:
      entries.append((href, href.rsplit("/", 1)[-1].rsplit(".", 1)[0] or href))
  return entries


def ncx_entries(zf: zipfile.ZipFile, root: ET.Element, opf_dir: str) -> list[tuple[str, str]]:
  item = ncx_item(root)
  href = item.attrib.get("href") if item is not None else None
  if not href:
    return []
  ncx_path = norm_join(opf_dir, href)
  if ncx_path not in zf.namelist():
    return []
  ncx_root = parse_xml(zf.read(ncx_path), ncx_path)
  base = posixpath.dirname(href)
  entries: list[tuple[str, str]] = []
  for nav_point in ncx_root.findall(".//ncx:navPoint", NS):
    label = text_content(nav_point.find("ncx:navLabel/ncx:text", NS))
    content = nav_point.find("ncx:content", NS)
    src = content.attrib.get("src") if content is not None else ""
    if src:
      entries.append((norm_join(base, src), label or src))
  return entries


def build_nav_xhtml(title: str, lang: str, entries: list[tuple[str, str]]) -> bytes:
  safe_title = escape(f"{title}目录" if title else "目录")
  safe_heading = escape(title or "目录")
  safe_lang = attr_escape(lang)
  items = "\n".join(
    f'        <li><a href="{attr_escape(href)}">{escape(label or href)}</a></li>'
    for href, label in entries
  )
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="{XHTML_URI}" xmlns:epub="{OPS_URI}" xml:lang="{safe_lang}" lang="{safe_lang}">
  <head>
    <title>{safe_title}</title>
  </head>
  <body>
    <nav epub:type="toc" id="toc">
      <h1>{safe_heading}</h1>
      <ol>
{items}
      </ol>
    </nav>
  </body>
</html>
'''.encode("utf-8")


def package_title(root: ET.Element) -> str:
  return text_content(root.find(".//dc:title", NS))


def has_modified_meta(root: ET.Element) -> bool:
  return any(
    elem.attrib.get("property") == "dcterms:modified"
    for elem in root.findall("opf:metadata/opf:meta", NS)
  )


def add_modified_meta(root: ET.Element) -> None:
  value = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
  meta = ET.SubElement(metadata(root), tag(OPF_URI, "meta"), {"property": "dcterms:modified"})
  meta.text = value


def plan_epub3(path: Path) -> dict[str, object]:
  with zipfile.ZipFile(path) as zf:
    names = set(zf.namelist())
    opf_path, root = read_package(zf)
    opf_dir = posixpath.dirname(opf_path)
    version = root.attrib.get("version", "")
    navs = nav_items(root)
    ncx = ncx_item(root)
    actions: list[dict[str, str]] = []
    warnings: list[str] = []
    if version != "3.0":
      actions.append({"kind": "opf", "message": f"Set package version from {version or '<missing>'} to 3.0"})
    if not has_modified_meta(root):
      actions.append({"kind": "metadata", "message": "Add meta property=\"dcterms:modified\""})
    if len(navs) != 1:
      actions.append({"kind": "nav", "message": "Generate nav.xhtml and add manifest item with properties=\"nav\""})
    if ncx is None:
      warnings.append("No toc.ncx found; this harness will not synthesize legacy NCX.")
    if "META-INF/encryption.xml" in names:
      warnings.append("DRM/encryption.xml detected; stop unless the file is known to be font obfuscation only.")
    suggested = [
      f"python3 scripts/epub3_migration_harness.py {shell_path(path)} --write-output work/after/step-1-epub3.epub --format json",
      f"python3 scripts/validate_text_invariance.py {shell_path(path)} work/after/step-1-epub3.epub --check text,metadata,spine,cover,anchors --allow-list '*/nav*.xhtml'",
      "Use Calibre / VS Code diff review for OPF/nav structural changes.",
    ] if actions else [
      "No EPUB3 migration output needed; continue with scripts/epub_refinement_harness.py.",
    ]
    return {
      "harness": "epub3_migration_harness",
      "input": str(path),
      "opf": opf_path,
      "package_version": version or None,
      "already_epub3": version == "3.0" and len(navs) == 1 and has_modified_meta(root),
      "nav_items": len(navs),
      "has_ncx": ncx is not None,
      "actions": actions,
      "warnings": warnings,
      "suggested_next_commands": suggested,
    }


def migrate_epub3(input_path: Path, output_path: Path) -> dict[str, object]:
  plan = plan_epub3(input_path)
  with zipfile.ZipFile(input_path) as zin:
    names = set(zin.namelist())
    opf_path, root = read_package(zin)
    opf_dir = posixpath.dirname(opf_path)
    new_root = copy.deepcopy(root)
    new_root.set("version", "3.0")
    generated_nav_href: str | None = None
    generated_nav_bytes: bytes | None = None

    if not has_modified_meta(new_root):
      add_modified_meta(new_root)

    if len(nav_items(new_root)) != 1:
      generated_nav_href = choose_nav_href(opf_dir, names)
      entries = ncx_entries(zin, new_root, opf_dir) or spine_entries(new_root)
      if not entries:
        raise Epub3Error("cannot generate nav.xhtml: no NCX navPoint or spine entries")
      generated_nav_bytes = build_nav_xhtml(package_title(new_root), language(new_root), entries)
      ET.SubElement(manifest(new_root), tag(OPF_URI, "item"), {
        "id": unique_item_id(new_root, "nav"),
        "href": generated_nav_href,
        "media-type": "application/xhtml+xml",
        "properties": "nav",
      })

    output_path.parent.mkdir(parents=True, exist_ok=True)
    opf_bytes = ET.tostring(new_root, encoding="utf-8", xml_declaration=True)
    nav_zip_path = norm_join(opf_dir, generated_nav_href) if generated_nav_href else None

    with zipfile.ZipFile(output_path, "w") as zout:
      mimetype = zin.read("mimetype") if "mimetype" in names else b"application/epub+zip"
      info = zipfile.ZipInfo("mimetype")
      info.compress_type = zipfile.ZIP_STORED
      zout.writestr(info, mimetype)
      for old_info in zin.infolist():
        if old_info.filename in {"mimetype", opf_path, nav_zip_path}:
          continue
        new_info = zipfile.ZipInfo(old_info.filename, old_info.date_time)
        new_info.compress_type = old_info.compress_type
        new_info.external_attr = old_info.external_attr
        zout.writestr(new_info, zin.read(old_info.filename))
      opf_info = zipfile.ZipInfo(opf_path)
      opf_info.compress_type = zipfile.ZIP_DEFLATED
      zout.writestr(opf_info, opf_bytes)
      if nav_zip_path and generated_nav_bytes:
        nav_info = zipfile.ZipInfo(nav_zip_path)
        nav_info.compress_type = zipfile.ZIP_DEFLATED
        zout.writestr(nav_info, generated_nav_bytes)

  plan["written_output"] = str(output_path)
  if generated_nav_href:
    plan["generated_nav_href"] = generated_nav_href
  return plan


def render_markdown(data: dict[str, object]) -> str:
  lines = [
    "# EPUB3 Migration Harness Report",
    "",
    f"- Input: `{data['input']}`",
    f"- OPF: `{data.get('opf')}`",
    f"- Package version: `{data.get('package_version')}`",
    f"- Already EPUB3-ready: `{data.get('already_epub3')}`",
    "",
    "## Actions",
  ]
  actions = data.get("actions") or []
  if actions:
    for item in actions:
      lines.append(f"- `{item['kind']}`: {item['message']}")
  else:
    lines.append("- No package migration action needed.")
  warnings = data.get("warnings") or []
  if warnings:
    lines.extend(["", "## Warnings"])
    for warning in warnings:
      lines.append(f"- {warning}")
  if data.get("written_output"):
    lines.extend(["", "## Output", f"- Written: `{data['written_output']}`"])
  lines.extend(["", "## Suggested Next Commands"])
  for command in data.get("suggested_next_commands", []):
    lines.append(f"- `{command}`")
  return "\n".join(lines) + "\n"


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Plan or write a conservative EPUB 3 migration")
  parser.add_argument("input", type=Path)
  parser.add_argument("--write-output", type=Path, help="Write migrated EPUB to this path")
  parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
  args = parser.parse_args(argv)

  try:
    data = migrate_epub3(args.input, args.write_output) if args.write_output else plan_epub3(args.input)
  except (Epub3Error, zipfile.BadZipFile) as exc:
    data = {
      "harness": "epub3_migration_harness",
      "input": str(args.input),
      "error": str(exc),
      "actions": [],
      "warnings": [],
    }
    code = 1
  else:
    code = 0

  if args.format == "json":
    print(json.dumps(data, ensure_ascii=False, indent=2))
  else:
    print(render_markdown(data), end="")
  return code


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
