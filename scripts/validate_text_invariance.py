#!/usr/bin/env python3
"""Validate red-line invariants between two EPUB files.

The cleanup pipeline uses this script as its machine gate. It intentionally
uses only the Python standard library so it can run before optional XML
dependencies are installed.
"""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import re
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable
from xml.etree import ElementTree as ET


CONTAINER_NS = {"c": "urn:oasis:names:tc:opendocument:xmlns:container"}
OPF_NS = {"opf": "http://www.idpf.org/2007/opf", "dc": "http://purl.org/dc/elements/1.1/"}
CORE_METADATA = ("title", "creator", "identifier", "language")
BLOCK_TAGS = {"p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "td", "blockquote", "pre", "div"}
IGNORED_TEXT_TAGS = {"rt", "rp", "script", "style"}
CHECKS = ("text", "metadata", "spine", "cover", "drm")


class InputError(Exception):
  """Input is not a processable EPUB."""


@dataclass
class EpubData:
  path: Path
  zf: zipfile.ZipFile
  names: set[str]
  opf_path: str | None = None
  opf_root: ET.Element | None = None

  def close(self) -> None:
    self.zf.close()


def local_name(tag: object) -> str:
  if not isinstance(tag, str):
    return ""
  return tag.rsplit("}", 1)[-1]


def normalize_text(value: str) -> str:
  return re.sub(r"\s+", " ", value.replace("\u00a0", " ")).strip()


def sanitize_xml(data: bytes) -> bytes:
  text = data.decode("utf-8", errors="replace")
  text = re.sub(r"<!DOCTYPE[^>]*>", "", text, flags=re.I | re.S)
  text = text.replace("&nbsp;", "&#160;")
  return text.encode("utf-8")


def parse_xml(data: bytes, label: str) -> ET.Element:
  try:
    return ET.fromstring(sanitize_xml(data))
  except ET.ParseError as exc:
    raise InputError(f"{label}: XML parse failed: {exc}") from exc


def open_epub(path: Path) -> EpubData:
  if not path.exists():
    raise InputError(f"input not found: {path}")
  try:
    zf = zipfile.ZipFile(path)
  except zipfile.BadZipFile as exc:
    raise InputError(f"not a valid zip/EPUB: {path}") from exc
  names = set(zf.namelist())
  data = EpubData(path=path, zf=zf, names=names)
  if "META-INF/container.xml" in names:
    container = parse_xml(zf.read("META-INF/container.xml"), "META-INF/container.xml")
    rootfile = container.find(".//c:rootfile", CONTAINER_NS)
    opf_path = rootfile.attrib.get("full-path") if rootfile is not None else None
    if opf_path and opf_path in names:
      data.opf_path = opf_path
      data.opf_root = parse_xml(zf.read(opf_path), opf_path)
  return data


def has_drm(epub: EpubData) -> bool:
  return "META-INF/encryption.xml" in epub.names


def xhtml_paths(epub: EpubData) -> set[str]:
  return {name for name in epub.names if name.lower().endswith((".xhtml", ".html"))}


def skipped(path: str, patterns: Iterable[str]) -> bool:
  return any(fnmatch.fnmatch(path, pattern) for pattern in patterns)


def element_text_without_ignored(elem: ET.Element) -> str:
  chunks: list[str] = []

  def walk(node: ET.Element) -> None:
    if local_name(node.tag) in IGNORED_TEXT_TAGS:
      if node.tail:
        chunks.append(node.tail)
      return
    if node.text:
      chunks.append(node.text)
    for child in list(node):
      walk(child)
    if node.tail:
      chunks.append(node.tail)

  walk(elem)
  return normalize_text("".join(chunks))


def has_block_descendant(elem: ET.Element) -> bool:
  for child in elem.iter():
    if child is elem:
      continue
    if local_name(child.tag) in BLOCK_TAGS:
      return True
  return False


def extract_text_blocks(content: bytes, label: str) -> list[str]:
  root = parse_xml(content, label)
  blocks: list[str] = []
  for elem in root.iter():
    name = local_name(elem.tag)
    if name not in BLOCK_TAGS:
      continue
    if has_block_descendant(elem):
      continue
    text = element_text_without_ignored(elem)
    if text:
      blocks.append(text)
  return blocks


def block_hashes(blocks: list[str]) -> list[str]:
  return [hashlib.sha256(block.encode("utf-8")).hexdigest() for block in blocks]


def compare_text(before: EpubData, after: EpubData, allow_list: list[str], verbose: bool) -> list[str]:
  problems: list[str] = []
  before_paths = xhtml_paths(before)
  after_paths = xhtml_paths(after)
  for name in sorted(before_paths | after_paths):
    if skipped(name, allow_list):
      continue
    if name not in before_paths:
      problems.append(f"text: added XHTML file: {name}")
      continue
    if name not in after_paths:
      problems.append(f"text: deleted XHTML file: {name}")
      continue
    before_blocks = extract_text_blocks(before.zf.read(name), f"{before.path}:{name}")
    after_blocks = extract_text_blocks(after.zf.read(name), f"{after.path}:{name}")
    before_hashes = block_hashes(before_blocks)
    after_hashes = block_hashes(after_blocks)
    if before_hashes == after_hashes:
      if verbose:
        problems.append(f"verbose: text unchanged: {name} ({len(before_hashes)} blocks)")
      continue
    problems.append(
      f"text: modified {name}: {len(before_hashes)} blocks before, {len(after_hashes)} after"
    )
    for index, (before_hash, after_hash) in enumerate(zip(before_hashes, after_hashes)):
      if before_hash != after_hash:
        problems.append(f"  block {index}: {before_hash[:12]} != {after_hash[:12]}")
        problems.append(f"    before: {before_blocks[index][:160]}")
        problems.append(f"    after:  {after_blocks[index][:160]}")
        break
    if len(before_hashes) != len(after_hashes):
      problems.append("  block count differs")
  return [p for p in problems if not p.startswith("verbose:")] if not verbose else problems


def require_opf(epub: EpubData) -> ET.Element:
  if epub.opf_root is None:
    raise InputError(f"{epub.path}: cannot resolve OPF from META-INF/container.xml")
  return epub.opf_root


def metadata_values(epub: EpubData) -> dict[str, list[str]]:
  root = require_opf(epub)
  values: dict[str, list[str]] = {}
  for field in CORE_METADATA:
    entries = []
    for elem in root.findall(f".//dc:{field}", OPF_NS):
      entries.append(normalize_text("".join(elem.itertext())))
    values[field] = entries
  return values


def compare_metadata(before: EpubData, after: EpubData) -> list[str]:
  b_meta = metadata_values(before)
  a_meta = metadata_values(after)
  problems: list[str] = []
  for field in CORE_METADATA:
    if b_meta[field] != a_meta[field]:
      problems.append(f"metadata: dc:{field} changed: {b_meta[field]!r} -> {a_meta[field]!r}")
  return problems


def spine_idrefs(epub: EpubData) -> list[str]:
  root = require_opf(epub)
  return [item.attrib.get("idref", "") for item in root.findall("opf:spine/opf:itemref", OPF_NS)]


def compare_spine(before: EpubData, after: EpubData) -> list[str]:
  b_spine = spine_idrefs(before)
  a_spine = spine_idrefs(after)
  if b_spine == a_spine:
    return []
  return [f"spine: itemref sequence changed: {b_spine!r} -> {a_spine!r}"]


def opf_dir(epub: EpubData) -> str:
  if not epub.opf_path or "/" not in epub.opf_path:
    return ""
  return epub.opf_path.rsplit("/", 1)[0] + "/"


def cover_path(epub: EpubData) -> str | None:
  root = require_opf(epub)
  base = opf_dir(epub)
  for item in root.findall("opf:manifest/opf:item", OPF_NS):
    props = set((item.attrib.get("properties") or "").split())
    href = item.attrib.get("href")
    if "cover-image" in props and href:
      return base + href
  return None


def sha256_bytes(data: bytes) -> str:
  return hashlib.sha256(data).hexdigest()


def compare_cover(before: EpubData, after: EpubData) -> list[str]:
  b_cover = cover_path(before)
  a_cover = cover_path(after)
  if b_cover != a_cover:
    return [f"cover: cover-image path changed: {b_cover!r} -> {a_cover!r}"]
  if not b_cover:
    return []
  if b_cover not in before.names or b_cover not in after.names:
    return [f"cover: cover-image missing from zip: {b_cover}"]
  b_hash = sha256_bytes(before.zf.read(b_cover))
  a_hash = sha256_bytes(after.zf.read(a_cover))
  if b_hash != a_hash:
    return [f"cover: cover-image bytes changed: {b_hash[:12]} != {a_hash[:12]} ({b_cover})"]
  return []


def parse_checks(value: str, legacy_redlines: str | None) -> set[str]:
  if legacy_redlines:
    value = "all" if legacy_redlines == "all" else "text"
  if value == "all":
    return set(CHECKS)
  parts = {part.strip() for part in value.split(",") if part.strip()}
  invalid = parts - set(CHECKS)
  if invalid:
    raise InputError(f"invalid --check value: {', '.join(sorted(invalid))}")
  return parts


def write_report(lines: list[str], output: Path | None) -> None:
  text = "\n".join(lines) + ("\n" if lines else "")
  if output:
    output.write_text(text, encoding="utf-8")
  elif text:
    print(text, file=sys.stderr, end="")


def validate(args: argparse.Namespace) -> int:
  try:
    checks = parse_checks(args.check, args.redlines)
    before = open_epub(args.before)
    after = open_epub(args.after)
    try:
      if "drm" in checks and (has_drm(before) or has_drm(after)):
        write_report(["DRM detected, refusing to process."], args.output)
        return 2
      problems: list[str] = []
      verbose_lines: list[str] = []
      if "text" in checks:
        text_results = compare_text(before, after, args.allow_list, args.verbose)
        verbose_lines.extend(line for line in text_results if line.startswith("verbose:"))
        problems.extend(line for line in text_results if not line.startswith("verbose:"))
      if "metadata" in checks:
        problems.extend(compare_metadata(before, after))
      if "spine" in checks:
        problems.extend(compare_spine(before, after))
      if "cover" in checks:
        problems.extend(compare_cover(before, after))
      if problems:
        write_report(verbose_lines + problems if args.verbose else problems, args.output)
        return 1
      if args.verbose:
        write_report(verbose_lines + ["All requested red-line checks passed."], args.output)
      return 0
    finally:
      before.close()
      after.close()
  except InputError as exc:
    write_report([f"input error: {exc}"], args.output)
    return 2


def main(argv: list[str]) -> int:
  parser = argparse.ArgumentParser(description="Validate EPUB cleanup red-line invariants")
  parser.add_argument("before", type=Path)
  parser.add_argument("after", type=Path)
  parser.add_argument("--check", default="all", help="text,metadata,spine,cover,drm,all or comma list")
  parser.add_argument("--redlines", choices=("all", "text"), help="legacy alias for --check")
  parser.add_argument("--allow-list", action="append", default=[], help="fnmatch pattern for XHTML paths")
  parser.add_argument("--output", type=Path, help="write report to file instead of stderr")
  parser.add_argument("--verbose", action="store_true")
  return validate(parser.parse_args(argv))


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
