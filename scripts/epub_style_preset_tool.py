#!/usr/bin/env python3
"""List, inspect, and apply repository CSS style presets to an EPUB."""

from __future__ import annotations

import argparse
import json
import posixpath
import re
import sys
from pathlib import Path
from typing import Any
from xml.etree import ElementTree as ET

from epub_lib import (
  EpubLibError,
  OPF_NS,
  OPF_URI,
  manifest,
  norm_join,
  opf_path_from_container,
  parse_xml,
  q,
  read_epub_files,
  rel_href,
  spine,
  unique_id,
  write_epub,
)


ROOT = Path(__file__).resolve().parents[1]
PRESETS_ROOT = ROOT / "templates" / "style-presets"
CLASS_ATTR_RE = re.compile(r"\bclass\s*=\s*([\"'])(?P<value>.*?)\1", re.I | re.S)
CSS_COMMENT_RE = re.compile(r"/\*.*?\*/", re.S)
CSS_CLASS_RE = re.compile(r"(?<![\w-])\.([A-Za-z_][\w-]*)")
LINK_RE = re.compile(r"(?P<indent>^[ \t]*)<link\b(?P<attrs>[^>]*)/?>[ \t]*(?:\r?\n)?", re.I | re.M)
HEAD_END_RE = re.compile(r"(?P<indent>^[ \t]*)</head\s*>", re.I | re.M)
COVERAGE_THRESHOLD = 0.3


class PresetError(Exception):
  """A preset or EPUB cannot be processed safely."""


def available_presets() -> list[str]:
  return sorted(
    path.name
    for path in PRESETS_ROOT.iterdir()
    if path.is_dir() and (path / "preset.json").is_file()
  )


def load_preset(name: str) -> tuple[dict[str, Any], Path]:
  preset_dir = PRESETS_ROOT / name
  config_path = preset_dir / "preset.json"
  if not config_path.is_file():
    raise PresetError(f"unknown preset: {name}")
  try:
    config = json.loads(config_path.read_text(encoding="utf-8"))
  except (OSError, json.JSONDecodeError) as exc:
    raise PresetError(f"invalid preset metadata: {config_path}: {exc}") from exc
  if not isinstance(config, dict) or config.get("name") != name or config.get("version") != "1":
    raise PresetError(f"invalid preset metadata: {config_path}")
  layers = config.get("layers")
  if not isinstance(layers, list) or not layers:
    raise PresetError(f"preset has no layers: {name}")
  for layer in layers:
    if not isinstance(layer, str) or Path(layer).name != layer or not layer.endswith(".css"):
      raise PresetError(f"invalid stylesheet layer in preset {name}: {layer!r}")
    css_path = preset_dir / "Styles" / layer
    if not css_path.is_file():
      raise PresetError(f"preset stylesheet is missing: {css_path}")
    line_count = len(css_path.read_text(encoding="utf-8").splitlines())
    if line_count > 500:
      raise PresetError(f"preset stylesheet exceeds the 500-line hard limit: {css_path}")
  return config, preset_dir


def spine_xhtml_paths(root: ET.Element, opf_path: str) -> list[str]:
  opf_dir = posixpath.dirname(opf_path)
  items = {
    item.attrib.get("id", ""): item
    for item in manifest(root).findall(q(OPF_URI, "item"))
  }
  paths: list[str] = []
  for itemref in spine(root).findall(q(OPF_URI, "itemref")):
    item = items.get(itemref.attrib.get("idref", ""))
    if item is None or item.attrib.get("media-type") not in {"application/xhtml+xml", "text/html"}:
      continue
    href = item.attrib.get("href")
    if href:
      paths.append(norm_join(opf_dir, href))
  return paths


def used_classes(files: dict[str, bytes], paths: list[str]) -> set[str]:
  classes: set[str] = set()
  for path in paths:
    data = files.get(path)
    if data is None:
      raise PresetError(f"spine item is missing from EPUB: {path}")
    text = data.decode("utf-8", errors="replace")
    for match in CLASS_ATTR_RE.finditer(text):
      classes.update(token for token in match.group("value").split() if token)
  return classes


def preset_classes(preset_dir: Path, layers: list[str]) -> set[str]:
  classes: set[str] = set()
  for layer in layers:
    text = (preset_dir / "Styles" / layer).read_text(encoding="utf-8")
    classes.update(CSS_CLASS_RE.findall(CSS_COMMENT_RE.sub("", text)))
  return classes


def coverage_report(actual: set[str], styled: set[str]) -> dict[str, object]:
  covered = actual & styled
  ratio = len(covered) / len(actual) if actual else 0.0
  warning = None
  if ratio < COVERAGE_THRESHOLD:
    warning = "该书尚未迁入本仓 class 体系，请先走 cleanup pipeline（oneclick 会注入 typography palette）"
  return {
    "used_classes": sorted(actual),
    "covered_classes": sorted(covered),
    "ratio": round(ratio, 4),
    "threshold": COVERAGE_THRESHOLD,
    "warning": warning,
  }


def stylesheet_actions(
  files: dict[str, bytes],
  opf_path: str,
  preset_dir: Path,
  layers: list[str],
) -> list[dict[str, str]]:
  styles_dir = posixpath.join(posixpath.dirname(opf_path), "Styles")
  actions: list[dict[str, str]] = []
  for layer in layers:
    path = posixpath.join(styles_dir, layer)
    source = preset_dir / "Styles" / layer
    actions.append({
      "path": path,
      "source": str(source.relative_to(ROOT)),
      "action": "replace" if path in files else "add",
    })
  return actions


def is_stylesheet_link(match: re.Match[str]) -> bool:
  attrs = match.group("attrs")
  rel = re.search(r"\brel\s*=\s*([\"'])(.*?)\1", attrs, re.I | re.S)
  href = re.search(r"\bhref\s*=\s*([\"'])(.*?)\1", attrs, re.I | re.S)
  return bool(
    (rel and "stylesheet" in rel.group(2).lower().split())
    or (href and href.group(2).split("#", 1)[0].lower().endswith(".css"))
  )


def rewrite_stylesheet_links(text: str, xhtml_path: str, css_paths: list[str]) -> str:
  def remove(match: re.Match[str]) -> str:
    return "" if is_stylesheet_link(match) else match.group(0)

  without_links = LINK_RE.sub(remove, text)
  head = HEAD_END_RE.search(without_links)
  if head is None:
    raise PresetError(f"XHTML has no </head>: {xhtml_path}")
  indent = head.group("indent") + "  "
  links = "".join(
    f'{indent}<link rel="stylesheet" type="text/css" href="{rel_href(xhtml_path, css_path)}"/>\n'
    for css_path in css_paths
  )
  return without_links[:head.start()] + links + without_links[head.start():]


def ensure_manifest_stylesheets(root: ET.Element, opf_path: str, css_paths: list[str]) -> list[str]:
  opf_dir = posixpath.dirname(opf_path)
  manifest_node = manifest(root)
  existing: dict[str, ET.Element] = {}
  for item in manifest_node.findall(q(OPF_URI, "item")):
    href = item.attrib.get("href")
    if href:
      existing[norm_join(opf_dir, href)] = item
  added: list[str] = []
  for css_path in css_paths:
    href = rel_href(opf_path, css_path)
    item = existing.get(css_path)
    if item is None:
      item = ET.SubElement(
        manifest_node,
        q(OPF_URI, "item"),
        {
          "id": unique_id(root, f"style-{Path(css_path).stem}"),
          "href": href,
          "media-type": "text/css",
        },
      )
      existing[css_path] = item
      added.append(href)
    else:
      item.set("media-type", "text/css")
  return added


def analyze_apply(input_path: Path, preset_name: str) -> tuple[dict[str, Any], dict[str, bytes], list[str]]:
  config, preset_dir = load_preset(preset_name)
  files, order = read_epub_files(input_path)
  opf_path = opf_path_from_container(files)
  root = parse_xml(files[opf_path], opf_path)
  xhtml_paths = spine_xhtml_paths(root, opf_path)
  layers = list(config["layers"])
  coverage = coverage_report(used_classes(files, xhtml_paths), preset_classes(preset_dir, layers))
  stylesheets = stylesheet_actions(files, opf_path, preset_dir, layers)
  return {
    "version": "1",
    "preset": preset_name,
    "input": str(input_path.resolve()),
    "coverage": coverage,
    "stylesheets": stylesheets,
    "xhtml_links": xhtml_paths,
    "layers": layers,
    "notes": config.get("notes", ""),
  }, files, order


def apply_preset(input_path: Path, output_path: Path, preset_name: str, dry_run: bool) -> dict[str, Any]:
  input_path = input_path.resolve()
  output_path = output_path.resolve()
  if not input_path.is_file():
    raise PresetError(f"input EPUB does not exist: {input_path}")
  if input_path == output_path:
    raise PresetError("output must not overwrite the input EPUB")
  if not dry_run and output_path.exists():
    raise PresetError(f"output already exists: {output_path}")

  report, files, order = analyze_apply(input_path, preset_name)
  report["output"] = str(output_path)
  report["dry_run"] = dry_run
  warning = report["coverage"]["warning"]
  if warning:
    print(f"WARNING: {warning}", file=sys.stderr)
  if dry_run:
    return report

  config, preset_dir = load_preset(preset_name)
  layers = list(config["layers"])
  opf_path = opf_path_from_container(files)
  root = parse_xml(files[opf_path], opf_path)
  xhtml_paths = spine_xhtml_paths(root, opf_path)
  styles_dir = posixpath.join(posixpath.dirname(opf_path), "Styles")
  css_paths = [posixpath.join(styles_dir, layer) for layer in layers]

  for layer, css_path in zip(layers, css_paths, strict=True):
    files[css_path] = (preset_dir / "Styles" / layer).read_bytes()
  report["manifest_items_added"] = ensure_manifest_stylesheets(root, opf_path, css_paths)
  files[opf_path] = ET.tostring(root, encoding="utf-8", xml_declaration=True)
  for path in xhtml_paths:
    text = files[path].decode("utf-8")
    files[path] = rewrite_stylesheet_links(text, path, css_paths).encode("utf-8")

  write_epub(output_path, files, order)
  report["written_output"] = str(output_path)
  return report


def main(argv: list[str] | None = None) -> int:
  parser = argparse.ArgumentParser(description="List, inspect, or apply repository EPUB style presets")
  subparsers = parser.add_subparsers(dest="command", required=True)
  subparsers.add_parser("list", help="List available preset names")
  show = subparsers.add_parser("show", help="Show one preset")
  show.add_argument("name")
  apply_parser = subparsers.add_parser("apply", help="Apply one preset to a copied EPUB")
  apply_parser.add_argument("input", type=Path)
  apply_parser.add_argument("--preset", required=True)
  apply_parser.add_argument("--output", type=Path, required=True)
  apply_parser.add_argument("--dry-run", action="store_true")
  args = parser.parse_args(argv)

  try:
    if args.command == "list":
      print("\n".join(available_presets()))
      return 0
    if args.command == "show":
      config, _ = load_preset(args.name)
      print(f"{config['name']}: {config['description']}")
      print("layers: " + ", ".join(config["layers"]))
      print(f"base font chain: {config['base_font_chain']}")
      print(f"notes: {config['notes']}")
      return 0
    report = apply_preset(args.input, args.output, args.preset, args.dry_run)
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0
  except (PresetError, EpubLibError, ET.ParseError, OSError, UnicodeError) as exc:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 2


if __name__ == "__main__":
  raise SystemExit(main())
