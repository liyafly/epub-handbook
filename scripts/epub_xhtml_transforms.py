#!/usr/bin/env python3
"""Deterministic, minimal-diff XHTML string transforms (no DOM reserialize).

Only two jobs live here:
1. sanitize_for_xml: smallest set of replacements so xml.etree can parse dirty XHTML.
2. rename_class_values: anchored attribute-VALUE substitutions (class="..." only).

Anything structural belongs in the DOM layer, never regex.
"""

from __future__ import annotations

import re
import unicodedata
from xml.etree import ElementTree as ET

EPUB_NS = "http://www.idpf.org/2007/ops"
XHTML_NS = "http://www.w3.org/1999/xhtml"
XML_NS = "http://www.w3.org/XML/1998/namespace"

# Only named entities that are invalid in XML; numeric entities are already legal.
_NAMED_ENTITIES = {"&nbsp;": "&#160;"}

# Anchored to class attribute value; supports single/double quotes; never crosses tags
# or enters text content.
_CLASS_ATTR = re.compile(
    r'(?P<pre>\bclass\s*=\s*)(?P<q>["\'])(?P<val>[^"\']*)(?P=q)'
)


class ForbiddenTextChange(Exception):
    """A transform attempted to alter prose text content; refused."""


def _register_ns() -> None:
    """Register EPUB/XHTML namespaces so ET.tostring uses prefixes."""
    ET.register_namespace("", XHTML_NS)
    ET.register_namespace("epub", EPUB_NS)


# ── safe inline replacements ──────────────────────────────────────────────


def sanitize_for_xml(text: str) -> str:
    """Replace XML-invalid named entities with numeric equivalents.

    Only touches a whitelist; never alters character data other than the
    entity ampersand sequence itself.
    """
    for k, v in _NAMED_ENTITIES.items():
        text = text.replace(k, v)
    return text


def rename_class_values(text: str, mapping: dict[str, str]) -> tuple[str, int]:
    """Replace class tokens in class="..." attribute values only.

    Returns (new_text, count_of_elements_whose_class_changed).
    Idempotent: if every token is already the target value, count is 0 and
    the string is unchanged.
    """
    count = 0

    def repl(m: re.Match) -> str:
        nonlocal count
        classes = m.group("val").split()
        new = [mapping.get(c, c) for c in classes]
        if new != classes:
            count += 1
        return f'{m.group("pre")}{m.group("q")}{" ".join(new)}{m.group("q")}'

    return _CLASS_ATTR.sub(repl, text), count


# ── DOM (xml.etree) whitelist operations ──────────────────────────────────


def dom_add_attr(
    xhtml: str, target_id: str, attr: str, value: str
) -> tuple[str, bool]:
    """Add or ensure one attribute on the element whose id == target_id.

    Returns (serialized_xhtml, changed).  changed=False when the attribute
    already has the target value.
    Never edits text nodes.
    """
    _register_ns()
    root = ET.fromstring(xhtml)
    changed = False
    for el in root.iter():
        if el.get("id") == target_id:
            if el.get(attr) != value:
                el.set(attr, value)
                changed = True
            break
    if not changed:
        return xhtml, False
    body = ET.tostring(root, encoding="unicode")
    if xhtml.lstrip().startswith("<?xml"):
        body = '<?xml version="1.0" encoding="utf-8"?>\n' + body
    return body, True


def text_content_equal(a: str, b: str) -> bool:
    """Compare concatenated, NFC-normalized text nodes of two XHTML strings.

    Used as the per-file red-line gate: if this returns False after a
    transform, the transform MUST be rolled back.
    """

    def _texts(s: str) -> str:
        try:
            root = ET.fromstring(s)
        except ET.ParseError:
            root = ET.fromstring(f"<x>{s}</x>")
        return unicodedata.normalize("NFC", "".join(root.itertext()))

    return _texts(a) == _texts(b)


def dom_rewrite_tag(
    xhtml: str, match: dict, new_tag: str
) -> tuple[str, bool]:
    """Rewrite a tag name (e.g. div.quote → blockquote) preserving text.

    ``match`` is a dict with keys "tag" (required) and optionally "class".
    The matched class is removed from the element after the rewrite.

    Raises ForbiddenTextChange if text content is altered.
    """
    _register_ns()
    root = ET.fromstring(xhtml)
    before_text = "".join(root.itertext())
    changed = False
    match_tag = match.get("tag", "")
    match_class = match.get("class")

    for parent in root.iter():
        for i, el in enumerate(list(parent)):
            local = el.tag.split("}")[-1] if "}" in el.tag else el.tag
            if local != match_tag:
                continue
            if match_class:
                el_classes = (el.get("class") or "").split()
                if match_class not in el_classes:
                    continue
            # Found a match — rewrite tag
            ns = "{" + el.tag.split("}")[0].lstrip("{") + "}" if "}" in el.tag else ""
            el.tag = f"{ns}{new_tag}"
            if match_class:
                rest = [c for c in (el.get("class") or "").split() if c != match_class]
                if rest:
                    el.set("class", " ".join(rest))
                elif el.get("class") is not None:
                    del el.attrib["class"]
            changed = True
    if not changed:
        return xhtml, False

    out = ET.tostring(root, encoding="unicode")
    if "".join(ET.fromstring(out).itertext()) != before_text:
        raise ForbiddenTextChange(
            f"tag rewrite changed text content: {match}"
        )
    if xhtml.lstrip().startswith("<?xml"):
        out = '<?xml version="1.0" encoding="utf-8"?>\n' + out
    return out, True


def dom_set_root_attr(xhtml: str, attr: str, value: str) -> tuple[str, bool]:
    """Set *attr* = *value* on the document root element.

    Returns (serialized, changed).  Used when the locator references the
    root ``<html>`` element, which rarely carries an id attribute.
    """
    _register_ns()
    root = ET.fromstring(xhtml)
    if root.get(attr) == value:
        return xhtml, False
    root.set(attr, value)
    body = ET.tostring(root, encoding="unicode")
    if xhtml.lstrip().startswith("<?xml"):
        body = '<?xml version="1.0" encoding="utf-8"?>\n' + body
    return body, True
