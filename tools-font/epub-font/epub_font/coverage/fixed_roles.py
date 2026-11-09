"""Find characters that a fixed font role cannot show.

A run whose font chain starts with an embedded font has a font chosen for it:
a locked body, a letter, a poem, a title. Every character in it should be in an
embedded font of that chain, or a reader's fallback font shows it. That fallback
may look fine, so the build only warns and the result is checked in real readers. A later embedded font counts, as in "en", "st", serif, where Latin and
Chinese each have their own font. Chains that start with a system font, such as
Georgia, "en", serif, are exempt: those readers are meant to use their own fonts.
See SPEC section 8.

Unlike the character inventory, this looks at ASCII and general punctuation too.
A date in a letter falling back to another font is exactly what it must catch.
"""

from __future__ import annotations

import posixpath
import re
import unicodedata
from urllib.parse import unquote

from lxml import etree

from .resolver import resolve_chains

_CSS_IMPORT = re.compile(rb"""@import\s+(?:url\(\s*)?["']?([^"')\s;]+)""", re.IGNORECASE)


def _needs_glyph(ch: str) -> bool:
    if ch.isspace():
        return False
    # Control and format characters, and combining marks such as variation
    # selectors, are not expected to have their own visible glyph.
    return unicodedata.category(ch) not in {"Cc", "Cf", "Mn", "Me"}


def find_fixed_role_missing(runs: list, chains: dict, run_chains: dict, font_index: dict) -> list[dict]:
    """Return one entry per (character, font), sorted by font then code point."""
    found: dict[tuple[str, str], dict] = {}
    for index, run in enumerate(runs):
        chain = chains.get(run_chains.get(index))
        if not chain or not chain[0].embedded:
            continue
        fonts = []
        for segment in chain:
            if segment.embedded and segment.file:
                base = posixpath.basename(segment.file)
                font = font_index.get(segment.file) or font_index.get(base) or font_index.get(base.lower())
                if font is not None:
                    fonts.append(font)
        if not fonts:
            continue
        for ch in run.get("text", ""):
            if not _needs_glyph(ch) or any(ord(ch) in font["cmap"] for font in fonts):
                continue
            key = (fonts[0]["file_path"], ch)
            entry = found.get(key)
            if entry is None:
                found[key] = {
                    "char": ch,
                    "codepoint": f"U+{ord(ch):04X}",
                    "family": chain[0].family,
                    "font": fonts[0]["file_path"],
                    "count": 1,
                    "first_location": run.get("file", ""),
                }
            else:
                entry["count"] += 1
    return [found[key] for key in sorted(found, key=lambda k: (k[0], ord(k[1])))]


def _resolve_href(base: str, href: str) -> str:
    return posixpath.normpath(posixpath.join(posixpath.dirname(base), unquote(href.split("#", 1)[0])))


def _page_stylesheets(content: bytes, doc_path: str, css_by_path: dict) -> tuple[list, tuple]:
    """Return the stylesheets a page really loads, with a key for grouping pages."""
    tree = etree.fromstring(content, etree.HTMLParser(recover=True))
    sheets: list = []
    key: list = []
    seen: set = set()

    def add_file(path: str) -> None:
        sheet = css_by_path.get(path)
        if sheet is None or path in seen:
            return
        seen.add(path)
        for match in _CSS_IMPORT.finditer(sheet.get("content_bytes", b"")):
            add_file(_resolve_href(path, match.group(1).decode("utf-8", "ignore")))
        sheets.append(sheet)
        key.append(path)

    if tree is None:
        return sheets, tuple(key)
    for node in tree.iter("link", "style"):
        if node.tag == "style":
            text = "".join(node.itertext()).encode("utf-8")
            sheets.append({"resolved_path": doc_path, "content_bytes": text})
            key.append(text)
        elif "stylesheet" in (node.get("rel") or "").lower() and node.get("href"):
            add_file(_resolve_href(doc_path, node.get("href")))
    return sheets, tuple(key)


def check_book(zf, book: dict, runs: list, css_files: list, font_faces: dict, font_index: dict) -> list[dict]:
    """Check each page against the stylesheets it loads.

    The coverage report applies every stylesheet to every page, which is fine
    for an overview but would make a build fail over a page that never loads the
    role's CSS. Pages that load the same stylesheets are resolved together.
    """
    css_by_path = {sheet["resolved_path"]: sheet for sheet in css_files}
    font_files = [font["resolved_path"] for font in book["font_files"]]
    runs_by_doc: dict[str, list] = {}
    for run in runs:
        runs_by_doc.setdefault(run.get("file", ""), []).append(run)

    groups: dict[tuple, tuple[list, list]] = {}
    for doc in book["xhtml_docs"]:
        doc_runs = runs_by_doc.get(doc["href"])
        if not doc_runs:
            continue
        doc_path = doc.get("resolved_path", doc["href"])
        sheets, key = _page_stylesheets(zf.read(doc_path), doc_path, css_by_path)
        groups.setdefault(key, (sheets, []))[1].extend(doc_runs)

    merged: dict[tuple[str, str], dict] = {}
    for sheets, group_runs in groups.values():
        chains, _, run_chains = resolve_chains(group_runs, sheets, font_faces, font_files)
        for entry in find_fixed_role_missing(group_runs, chains, run_chains, font_index):
            key = (entry["font"], entry["char"])
            if key in merged:
                merged[key]["count"] += entry["count"]
            else:
                merged[key] = entry
    return [merged[key] for key in sorted(merged, key=lambda k: (k[0], ord(k[1])))]
