#!/usr/bin/env python3
"""Validate project popup footnote invariants with Python stdlib only."""

from __future__ import annotations

import argparse
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OEBPS = ROOT / "templates" / "epub-style-demo" / "OEBPS"

XHTML_NS = "http://www.w3.org/1999/xhtml"
EPUB_NS = "http://www.idpf.org/2007/ops"
OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}


def epub_attr(name: str) -> str:
  return "{" + EPUB_NS + "}" + name


def local_name(tag: str) -> str:
  return tag.rsplit("}", 1)[-1]


def classes(elem: ET.Element) -> set[str]:
  return set(elem.attrib.get("class", "").split())


def typed(elem: ET.Element, token: str) -> bool:
  return token in elem.attrib.get(epub_attr("type"), "").split()


def href_fragment(href: str | None) -> str | None:
  if not href or not href.startswith("#") or len(href) == 1:
    return None
  return href[1:]


def iter_elems(root: ET.Element, name: str) -> list[ET.Element]:
  return [elem for elem in root.iter() if local_name(elem.tag) == name]


@dataclass
class NoteContext:
  path: Path
  root: ET.Element
  ids: dict[str, ET.Element]


class Check:
  def __init__(self) -> None:
    self.errors: list[str] = []

  def require(self, condition: bool, message: str) -> None:
    if not condition:
      self.errors.append(message)

  def fail(self, message: str) -> None:
    self.errors.append(message)


def parse_xml(path: Path, check: Check) -> ET.Element | None:
  try:
    return ET.parse(path).getroot()
  except ET.ParseError as exc:
    check.fail(f"XML parse failed: {path}: {exc}")
    return None


def collect_ids(root: ET.Element, path: Path, check: Check) -> dict[str, ET.Element]:
  ids: dict[str, ET.Element] = {}
  for elem in root.iter():
    elem_id = elem.attrib.get("id")
    if not elem_id:
      continue
    if elem_id in ids:
      check.fail(f"{path}: duplicate id: {elem_id}")
    ids[elem_id] = elem
  return ids


def validate_noteref(ctx: NoteContext, anchor: ET.Element, check: Check) -> str | None:
  prefix = f"{ctx.path}: noteref"
  note_id = anchor.attrib.get("id")
  target_id = href_fragment(anchor.attrib.get("href"))

  check.require(bool(note_id), f"{prefix} missing id")
  check.require(target_id is not None, f"{prefix} href must be same-file fragment")
  check.require(typed(anchor, "noteref"), f"{prefix} must have epub:type=noteref")
  check.require(anchor.attrib.get("role") == "doc-noteref", f"{prefix} must have role=doc-noteref")
  check.require("noteref-icon" in classes(anchor), f"{prefix} must include class=noteref-icon")
  images = iter_elems(anchor, "img")
  check.require(len(images) == 1, f"{prefix} must contain exactly one img icon")
  if images:
    check.require(images[0].attrib.get("alt") is not None, f"{prefix} img icon must have alt")

  if target_id is None:
    return None
  target = ctx.ids.get(target_id)
  check.require(target is not None, f"{prefix} target missing: #{target_id}")
  if target is not None:
    check.require(local_name(target.tag) == "li", f"{prefix} target must be li: #{target_id}")
    check.require("footnote-item" in classes(target), f"{prefix} target li must have class=footnote-item")
  return target_id


def validate_backlink(ctx: NoteContext, backlink: ET.Element, note_ids: set[str], check: Check) -> None:
  prefix = f"{ctx.path}: backlink"
  target_id = href_fragment(backlink.attrib.get("href"))
  check.require(target_id is not None, f"{prefix} href must be same-file fragment")
  check.require(typed(backlink, "backlink"), f"{prefix} must have epub:type=backlink")
  check.require(backlink.attrib.get("role") == "doc-backlink", f"{prefix} must have role=doc-backlink")
  if target_id is not None:
    check.require(target_id in note_ids, f"{prefix} target must be a noteref id: #{target_id}")


def validate_file(path: Path, check: Check) -> bool:
  root = parse_xml(path, check)
  if root is None:
    return False
  ctx = NoteContext(path=path, root=root, ids=collect_ids(root, path, check))

  noterefs = [
    anchor for anchor in iter_elems(root, "a")
    if typed(anchor, "noteref") or anchor.attrib.get("role") == "doc-noteref"
  ]
  footnote_asides = [
    aside for aside in iter_elems(root, "aside")
    if typed(aside, "footnote") or aside.attrib.get("role") == "doc-footnote"
  ]

  if not noterefs and not footnote_asides:
    return False

  check.require(len(footnote_asides) == 1, f"{path}: files with notes must have exactly one grouped footnote aside")
  if footnote_asides:
    aside = footnote_asides[0]
    check.require(typed(aside, "footnote"), f"{path}: footnote aside must have epub:type=footnote")
    check.require(aside.attrib.get("role") == "doc-footnote", f"{path}: footnote aside must have role=doc-footnote")
    lists = [ol for ol in iter_elems(aside, "ol") if "footnote-list" in classes(ol)]
    check.require(len(lists) == 1, f"{path}: footnote aside must contain exactly one ol.footnote-list")
  else:
    lists = []

  target_ids = set()
  note_ids = set()
  for anchor in noterefs:
    if anchor.attrib.get("id"):
      note_ids.add(anchor.attrib["id"])
    target_id = validate_noteref(ctx, anchor, check)
    if target_id:
      target_ids.add(target_id)

  if lists:
    note_list = lists[0]
    footnote_items = [li for li in iter_elems(note_list, "li") if "footnote-item" in classes(li)]
    item_ids = {li.attrib.get("id") for li in footnote_items if li.attrib.get("id")}
    check.require(target_ids <= item_ids, f"{path}: every noteref target must be in ol.footnote-list")

    backlinks = [
      anchor for li in footnote_items for anchor in iter_elems(li, "a")
      if typed(anchor, "backlink") or anchor.attrib.get("role") == "doc-backlink"
    ]
    check.require(len(backlinks) >= len(target_ids), f"{path}: each footnote item should contain a backlink")
    for backlink in backlinks:
      validate_backlink(ctx, backlink, note_ids, check)

    duokan_mode = (
      "duokan-footnote-content" in classes(note_list)
      or any("duokan-footnote" in classes(anchor) for anchor in noterefs)
      or any("duokan-footnote-item" in classes(li) for li in footnote_items)
    )
    if duokan_mode:
      check.require("duokan-footnote-content" in classes(note_list), f"{path}: Duokan fallback requires ol.duokan-footnote-content")
      for anchor in noterefs:
        check.require("duokan-footnote" in classes(anchor), f"{path}: Duokan fallback noteref missing class=duokan-footnote")
      for li in footnote_items:
        li_classes = classes(li)
        check.require("duokan-footnote-item" in li_classes, f"{path}: Duokan fallback li missing class=duokan-footnote-item")
        check.require("duokan-footnote-content" not in li_classes, f"{path}: duokan-footnote-content must not be on li")
  return True


def find_opf(root: Path, oebps: Path) -> Path | None:
  container = root / "META-INF" / "container.xml"
  if container.exists():
    try:
      container_root = ET.parse(container).getroot()
      rootfile = container_root.find(".//c:rootfile", CONTAINER_NS)
      opf = rootfile.attrib.get("full-path") if rootfile is not None else None
      if opf and (root / opf).exists():
        return root / opf
    except ET.ParseError:
      return None
  package = oebps / "package.opf"
  if package.exists():
    return package
  opfs = sorted(oebps.glob("*.opf"))
  return opfs[0] if opfs else None


def validate_manifest(oebps: Path, opf_path: Path | None, check: Check) -> None:
  if opf_path is None:
    check.fail(f"{oebps}: OPF package document not found")
    return
  package = opf_path
  opf_dir = package.parent
  root = parse_xml(package, check)
  if root is None:
    return
  note_items = [
    item for item in root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("href") == "Images/note.png"
  ]
  check.require(note_items, f"{package}: manifest must include Images/note.png")
  check.require((opf_dir / "Images" / "note.png").exists(), f"{opf_dir}: Images/note.png missing on disk")


def validate_oebps(root: Path, oebps: Path, check: Check) -> None:
  opf_path = find_opf(root, oebps)
  found_notes = False
  for path in sorted((oebps / "Text").glob("*.xhtml")):
    found_notes = validate_file(path, check) or found_notes
  if found_notes:
    validate_manifest(oebps, opf_path, check)


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser()
  parser.add_argument("--oebps", type=Path, default=DEFAULT_OEBPS, help="OEBPS directory to validate")
  parser.add_argument("--epub", type=Path, help="Optional EPUB zip to validate instead of --oebps")
  args = parser.parse_args(argv)

  check = Check()
  if args.epub:
    with TemporaryDirectory() as tmp:
      extracted = Path(tmp)
      with zipfile.ZipFile(args.epub) as zf:
        zf.extractall(extracted)
      validate_oebps(extracted, extracted / "OEBPS", check)
  else:
    validate_oebps(args.oebps.parent, args.oebps, check)

  if check.errors:
    for error in check.errors:
      print(f"ERROR: {error}", file=sys.stderr)
    return 1
  print("popup note validation ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
