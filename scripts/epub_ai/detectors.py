"""Deterministic actionable-finding registry."""

from __future__ import annotations

import posixpath
import re
import sys
from dataclasses import dataclass
from typing import Callable
from xml.etree import ElementTree as ET

from epub_lib import OPF_NS, norm_join, split_props

from .model import EpubModel


MATHML_URI = "http://www.w3.org/1998/Math/MathML"
SVG_URI = "http://www.w3.org/2000/svg"
XML_NS = "http://www.w3.org/XML/1998/namespace"


@dataclass(frozen=True)
class Detector:
  kind: str
  lane: str
  fn: Callable[..., list[dict]]


DETECTORS: list[Detector] = []


def detector(kind: str, lane: str) -> Callable:
  def register(fn: Callable[..., list[dict]]) -> Callable[..., list[dict]]:
    DETECTORS.append(Detector(kind, lane, fn))
    return fn
  return register


def collect_actionable_findings(model: EpubModel | None) -> list[dict]:
  if model is None:
    return []
  found: list[dict] = []
  errors: list[dict[str, str]] = []
  for registered in DETECTORS:
    try:
      found.extend(registered.fn(model))
    except Exception as exc:
      errors.append({"detector": registered.kind, "error": str(exc)})
  for error in errors:
    print(f"WARNING: detector {error['detector']} failed: {error['error']}", file=sys.stderr)
  return found


@detector("missing-html-lang", "tag")
def _detect_missing_html_lang(model: EpubModel) -> list[dict]:
  return [{
    "kind": "missing-html-lang", "file": path, "locator": {"selector": "html"},
    "params": {"value": model.book_language or "zh-Hans"}, "lane": "tag",
    "auto_fixable": True, "confidence": "high",
    "evidence": "<html> root element missing lang/xml:lang",
  } for path, root in model.xhtml_docs.items() if not (root.get("lang") or root.get(f"{{{XML_NS}}}lang"))]


@detector("obfuscated-class", "tag")
def _detect_obfuscated_class(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    for element in root.iter():
      obfuscated = [value for value in (element.get("class") or "").split() if re.search(r"\bcalibre\d*\b", value)]
      if obfuscated:
        out.append({
          "kind": "obfuscated-class", "file": path, "locator": {"id": element.get("id") or ""},
          "params": {"mapping": {value: "" for value in obfuscated}}, "lane": "tag",
          "auto_fixable": False, "confidence": "medium",
          "evidence": f"Found obfuscated class '{obfuscated[0]}' — target mapping requires human/AI judgment",
        })
        break
  return out


@detector("empty-paragraph", "tag")
def _detect_empty_paragraph(model: EpubModel) -> list[dict]:
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    for element in root.iter():
      tag = element.tag.split("}")[-1] if "}" in element.tag else element.tag
      if tag == "p" and "".join(element.itertext()).strip() in {"", " "}:
        out.append({
          "kind": "empty-paragraph", "file": path, "locator": {"id": element.get("id") or ""},
          "params": {"rule": "empty-paragraph"}, "lane": "tag", "auto_fixable": True,
          "confidence": "high", "evidence": "Empty paragraph element (no visible text content)",
        })
  return out


@detector("missing-manifest-properties", "package")
def _detect_missing_manifest_properties(model: EpubModel) -> list[dict]:
  if model.opf_root is None:
    return []
  opf_dir = posixpath.dirname(model.opf_path)
  manifest_paths = {
    norm_join(opf_dir, item.attrib["href"]): item
    for item in model.opf_root.findall("opf:manifest/opf:item", OPF_NS)
    if item.attrib.get("href")
  }
  out: list[dict] = []
  for path, root in model.xhtml_docs.items():
    item = manifest_paths.get(path)
    if item is None:
      continue
    props = split_props(item.attrib.get("properties"))
    serialized = ET.tostring(root, encoding="unicode")
    has_math = MATHML_URI in serialized or any("}" in element.tag and "Math/MathML}" in element.tag for element in root.iter())
    has_svg = SVG_URI in serialized or any("}" in element.tag and "2000/svg}" in element.tag for element in root.iter())
    for required, present, evidence in (
      ("mathml", has_math, 'XHTML contains MathML but manifest item lacks properties="mathml"'),
      ("svg", has_svg, 'XHTML contains SVG but manifest item lacks properties="svg"'),
    ):
      if present and required not in props:
        out.append({
          "kind": "missing-manifest-properties", "file": path,
          "locator": {"manifest_id": item.attrib.get("id", "")},
          "params": {"properties": required}, "lane": "package", "auto_fixable": True,
          "confidence": "high", "evidence": evidence,
        })
  return out
