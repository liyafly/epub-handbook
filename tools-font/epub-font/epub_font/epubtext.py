"""Read an EPUB and collect every character an embedded font may have to render.

Scope is the whole book (handbook §4.6 step 2-3): every XHTML/SVG/NCX item in
the manifest (nav included), text nodes, alt/title/aria-label attributes, and
every string literal in CSS (files, <style> blocks, style="" attributes), which
covers `content:` and `quotes:` generated text. A fixed baseline adds
characters that readers or CSS may generate without them appearing in the
source (list markers, emphasis marks, CJK numerals, common punctuation).
"""

from __future__ import annotations

import posixpath
import re
import zipfile
from dataclasses import dataclass, field
from html.parser import HTMLParser
from urllib.parse import unquote
from xml.etree import ElementTree as ET

import tinycss2

CONTAINER_PATH = "META-INF/container.xml"
ENCRYPTION_PATH = "META-INF/encryption.xml"
MARKUP_MEDIA_TYPES = {
    "application/xhtml+xml",
    "image/svg+xml",
    "application/x-dtbncx+xml",
}
CSS_MEDIA_TYPE = "text/css"
TEXT_ATTRIBUTES = {"alt", "title", "aria-label"}

# Characters a reader or CSS can render without them appearing in the text:
# ASCII, spaces, dashes/quotes, CJK punctuation, text-emphasis marks, list
# markers, CJK numerals (list-style cjk-decimal / trad-chinese-informal) and
# the default back-link symbol ◎ (SPEC §4).
BASELINE_TEXT = (
    "".join(chr(cp) for cp in range(0x20, 0x7F))
    + " ­·‐‑–—‘’“”…　"
    + "、。，．：；！？（）《》〈〉「」『』【】〔〕〖〗～·—…"
    + "﹅﹆•◦●○◉◎▲△"
    + "▪▫■□"
    + "〇一二三四五六七八九十百千万"
)

_XML_DECL_ENCODING = re.compile(rb"""^\s*<\?xml[^>]*encoding\s*=\s*["']([A-Za-z0-9._-]+)["']""")


class EpubError(Exception):
    """The EPUB cannot be read well enough to collect its text."""


@dataclass
class ManifestItem:
    item_id: str
    path: str  # archive path, percent-decoded, relative to the ZIP root
    media_type: str


@dataclass
class BookText:
    opf_path: str
    items: list[ManifestItem]
    encrypted_paths: set[str]
    chars_by_source: dict[str, set[str]] = field(default_factory=dict)

    def all_chars(self) -> set[str]:
        out: set[str] = set()
        for chars in self.chars_by_source.values():
            out |= chars
        return out

    def item_by_path(self, path: str) -> ManifestItem | None:
        for item in self.items:
            if item.path == path:
                return item
        return None


def decode_text(data: bytes) -> str:
    """Decode XHTML/CSS bytes: BOM or UTF-8 first, then the XML declaration."""
    try:
        return data.decode("utf-8-sig")
    except UnicodeDecodeError:
        pass
    if data.startswith((b"\xff\xfe", b"\xfe\xff")):
        return data.decode("utf-16")
    match = _XML_DECL_ENCODING.match(data)
    if match:
        try:
            return data.decode(match.group(1).decode("ascii"))
        except (LookupError, UnicodeDecodeError):
            pass
    return data.decode("utf-8", errors="replace")


def _local(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def _find_opf(zf: zipfile.ZipFile) -> str:
    try:
        root = ET.fromstring(zf.read(CONTAINER_PATH))
    except KeyError as exc:
        raise EpubError(f"missing {CONTAINER_PATH}") from exc
    except ET.ParseError as exc:
        raise EpubError(f"{CONTAINER_PATH} is not well-formed XML: {exc}") from exc
    for el in root.iter():
        if _local(el.tag) == "rootfile" and el.get("full-path"):
            return el.get("full-path")
    raise EpubError(f"{CONTAINER_PATH} has no rootfile full-path")


def _resolve(base_dir: str, href: str) -> str | None:
    href = unquote(href.split("#", 1)[0])
    if not href or re.match(r"^[A-Za-z][A-Za-z0-9+.-]*:", href):
        return None  # empty or absolute URL (http:, data:, ...)
    path = posixpath.normpath(posixpath.join(base_dir, href))
    if path.startswith("../") or path == "..":
        return None
    return path


def _read_manifest(zf: zipfile.ZipFile, opf_path: str) -> list[ManifestItem]:
    try:
        root = ET.fromstring(zf.read(opf_path))
    except KeyError as exc:
        raise EpubError(f"OPF {opf_path} is not in the archive") from exc
    except ET.ParseError as exc:
        raise EpubError(f"OPF {opf_path} is not well-formed XML: {exc}") from exc
    base_dir = posixpath.dirname(opf_path)
    items = []
    for el in root.iter():
        if _local(el.tag) != "item":
            continue
        path = _resolve(base_dir, el.get("href", ""))
        if path is None:
            continue
        items.append(ManifestItem(el.get("id", ""), path, (el.get("media-type") or "").strip().lower()))
    return items


def _encrypted_paths(zf: zipfile.ZipFile) -> set[str]:
    try:
        data = zf.read(ENCRYPTION_PATH)
    except KeyError:
        return set()
    try:
        root = ET.fromstring(data)
    except ET.ParseError as exc:
        raise EpubError(f"{ENCRYPTION_PATH} is not well-formed XML: {exc}") from exc
    paths = set()
    for el in root.iter():
        if _local(el.tag) == "CipherReference" and el.get("URI"):
            path = _resolve("", el.get("URI"))
            if path:
                paths.add(path)
    return paths


class _MarkupCollector(HTMLParser):
    """Tolerant collector for XHTML / SVG / NCX text (html.parser never raises on bad markup)."""

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.text: list[str] = []
        self.attr_text: list[str] = []
        self.css: list[str] = []
        self._script_depth = 0
        self._style_depth = 0

    def handle_starttag(self, tag, attrs):
        self._collect_attrs(attrs)
        if tag == "script":
            self._script_depth += 1
        elif tag == "style":
            self._style_depth += 1

    def handle_startendtag(self, tag, attrs):
        self._collect_attrs(attrs)

    def handle_endtag(self, tag):
        if tag == "script" and self._script_depth:
            self._script_depth -= 1
        elif tag == "style" and self._style_depth:
            self._style_depth -= 1

    def handle_data(self, data):
        if self._script_depth:
            return
        if self._style_depth:
            self.css.append(data)
        else:
            self.text.append(data)

    def unknown_decl(self, data):
        if data.upper().startswith("CDATA["):
            self.handle_data(data[len("CDATA["):])

    def _collect_attrs(self, attrs):
        for name, value in attrs:
            if value is None:
                continue
            local = name.lower().rsplit(":", 1)[-1]
            if local in TEXT_ATTRIBUTES:
                self.attr_text.append(value)
            elif local == "style":
                self.css.append(value)


def css_strings(css_text: str) -> list[str]:
    """Return every CSS string literal (escapes decoded), except url("...") arguments."""
    out: list[str] = []

    def walk(nodes):
        for node in nodes or ():
            kind = getattr(node, "type", None)
            if kind == "string":
                out.append(node.value)
            elif kind == "function":
                if node.lower_name != "url":
                    walk(node.arguments)
            elif kind in ("() block", "[] block", "{} block"):
                walk(node.content)

    walk(tinycss2.parse_component_value_list(css_text, skip_comments=True))
    return out


def _renderable(chars) -> set[str]:
    out = set()
    for ch in chars:
        cp = ord(ch)
        if cp < 0x20 or 0x7F <= cp <= 0x9F or 0xD800 <= cp <= 0xDFFF:
            continue
        out.add(ch)
    return out


def _case_variants(chars: set[str]) -> set[str]:
    """text-transform can change case, so keep both cases of every harvested letter."""
    out = set()
    for ch in chars:
        for variant in (ch.upper(), ch.lower()):
            if len(variant) == 1 and variant != ch:
                out.add(variant)
    return out


def _fullwidth_variants(chars: set[str]) -> set[str]:
    """text-transform: full-width maps ASCII to U+FF01..U+FF5E and space to U+3000."""
    out = set()
    for ch in chars:
        cp = ord(ch)
        if 0x21 <= cp <= 0x7E:
            out.add(chr(cp + 0xFEE0))
        elif cp == 0x20:
            out.add("　")
    return out


def read_book_text(zf: zipfile.ZipFile) -> BookText:
    opf_path = _find_opf(zf)
    items = _read_manifest(zf, opf_path)
    names = set(zf.namelist())
    markup_text: list[str] = []
    attr_text: list[str] = []
    css_texts: list[str] = []
    for item in items:
        if item.path not in names:
            continue
        if item.media_type in MARKUP_MEDIA_TYPES:
            collector = _MarkupCollector()
            collector.feed(decode_text(zf.read(item.path)))
            collector.close()
            markup_text.extend(collector.text)
            attr_text.extend(collector.attr_text)
            css_texts.extend(collector.css)
        elif item.media_type == CSS_MEDIA_TYPE:
            css_texts.append(decode_text(zf.read(item.path)))
    css_all = "\n".join(css_texts)
    sources = {
        "markup-text": _renderable("".join(markup_text)),
        "attributes": _renderable("".join(attr_text)),
        "css-strings": _renderable("".join(css_strings(css_all))),
        "baseline": set(BASELINE_TEXT),
    }
    harvested = sources["markup-text"] | sources["attributes"] | sources["css-strings"]
    sources["case-variants"] = _case_variants(harvested) - harvested
    if "full-width" in css_all.lower():
        sources["full-width-variants"] = _fullwidth_variants(harvested | sources["baseline"])
    return BookText(opf_path, items, _encrypted_paths(zf), sources)
