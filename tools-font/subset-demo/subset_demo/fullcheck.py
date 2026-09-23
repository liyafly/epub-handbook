"""Check that fonts contain EVERY character the book (or a given list) needs.

    uv run python -m subset_demo.fullcheck BOOK.epub --font OEBPS/Fonts/st-all.ttf [--font ...]
    uv run python -m subset_demo.fullcheck BOOK.epub --all-fonts
    uv run python -m subset_demo.fullcheck BOOK.epub --font-file /path/master.ttf --chars-file rare.txt
    ... [--json REPORT.json]

Deliberately independent of epubtext.py / fontops.py (nothing is imported from them):
text is collected with lxml instead of html.parser, fonts are read straight from cmap,
so a collection bug in the subset tool cannot hide itself. Unlike coverage-detector's
inventory, nothing is skipped: ASCII, U+2000-U+2E7F punctuation (“”‘’——…) and CSS
generated characters are all required.

Per font it reports:
  missing           required characters with no cmap entry (or mapped to .notdef)
  noInk             characters mapped to a glyph without any outline (except spaces/format chars)
  missingSequences  IVS/SVS sequences used in the text but absent from cmap format 14
  optionalMissing   format characters (ZWSP, ZWJ, soft hyphen, BOM ...) readers handle without a glyph
Exit codes: 0 = every font complete; 1 = something missing; 2 = bad input.
"""

from __future__ import annotations

import argparse
import hashlib
import html.entities
import io
import json
import posixpath
import re
import sys
import unicodedata
import zipfile
from pathlib import Path
from urllib.parse import unquote

import tinycss2
from fontTools.pens.boundsPen import BoundsPen
from fontTools.ttLib import TTFont
from lxml import etree

FONT_EXTENSIONS = (".ttf", ".otf", ".woff", ".woff2")
MARKUP_TYPES = {"application/xhtml+xml", "image/svg+xml", "application/x-dtbncx+xml"}
TEXT_ATTRS = {"alt", "title", "aria-label"}
XML_ENTITIES = {"amp", "lt", "gt", "quot", "apos"}
ENTITY_REF = re.compile(r"&([A-Za-z][A-Za-z0-9]*);")

# Characters CSS / the reader generates (SPEC §4 item 3), keyed by what triggers them.
EMPHASIS_MARKS = {
    "dot": "•◦", "circle": "●○", "double-circle": "◉◎",
    "triangle": "▲△", "sesame": "﹅﹆",
}
LIST_MARKERS = {
    "disc": "•", "circle": "◦", "square": "▪",
    "decimal": "0123456789.", "decimal-leading-zero": "0123456789.",
    "cjk-decimal": "〇一二三四五六七八九、",
    "cjk-ideographic": "零一二三四五六七八九十百千万、",
    "simp-chinese-informal": "零一二三四五六七八九十百千万、",
    "trad-chinese-informal": "零一二三四五六七八九十百千萬、",
    "lower-alpha": "abcdefghijklmnopqrstuvwxyz.", "upper-alpha": "ABCDEFGHIJKLMNOPQRSTUVWXYZ.",
    "lower-roman": "ivxlcdm.", "upper-roman": "IVXLCDM.",
}


class CheckError(Exception):
    pass


def _local(tag) -> str:
    return tag.rsplit("}", 1)[-1] if isinstance(tag, str) else ""


def _xml(data: bytes, name: str) -> tuple:
    """Parse XML; retry with HTML named entities turned into numeric references."""
    try:
        return etree.fromstring(data, etree.XMLParser(resolve_entities=False, no_network=True)), None
    except etree.XMLSyntaxError:
        pass
    text = data.decode("utf-8-sig", errors="replace")

    def numeric(match):
        entity = match.group(1)
        if entity in XML_ENTITIES:
            return match.group(0)
        value = html.entities.html5.get(entity + ";")
        return "".join(f"&#x{ord(ch):X};" for ch in value) if value else match.group(0)

    text = re.sub(r"^\s*<\?xml[^>]*\?>", "", ENTITY_REF.sub(numeric, text))
    try:
        return etree.fromstring(text.encode("utf-8"), etree.XMLParser(resolve_entities=False, no_network=True)), None
    except etree.XMLSyntaxError:
        root = etree.fromstring(text.encode("utf-8"), etree.XMLParser(recover=True, no_network=True))
        return root, f"{name}: not well-formed XML, parsed in recover mode (text may be incomplete)"


class Harvest:
    def __init__(self) -> None:
        self.chars: dict[str, dict] = {}   # char -> {"count": n, "first": "file"}
        self.sequences: dict[tuple, dict] = {}
        self.css: list[str] = []      # stylesheets and <style> blocks
        self.inline: list[str] = []   # style="" attributes
        self.has_q = False
        self.warnings: list[str] = []

    def add(self, text: str, where: str) -> None:
        previous = ""
        for ch in text:
            cp = ord(ch)
            if cp < 0x20 or 0x7F <= cp <= 0x9F:
                previous = ""
                continue
            if _is_selector(cp):
                if previous:
                    entry = self.sequences.setdefault((ord(previous), cp), {"count": 0, "first": where})
                    entry["count"] += 1
                continue
            entry = self.chars.setdefault(ch, {"count": 0, "first": where})
            entry["count"] += 1
            previous = ch

    def walk(self, element, where: str) -> None:
        if element.tag is etree.Entity:  # &nbsp; kept as a node when a DOCTYPE is present
            value = html.entities.html5.get(element.name + ";")
            if value:
                self.add(value, where)
            else:
                self.warnings.append(f"{where}: unknown entity &{element.name};")
            if element.tail:
                self.add(element.tail, where)
            return
        tag = _local(element.tag)
        if isinstance(element.tag, str):
            for name, value in element.attrib.items():
                local = _local(name).lower()
                if local in TEXT_ATTRS:
                    self.add(value, where)
                elif local == "style":
                    self.inline.append(value)
            if tag == "q":
                self.has_q = True
            if tag == "style":
                self.css.append("".join(element.itertext()))
            elif tag != "script":
                if element.text:
                    self.add(element.text, where)
                for child in element:
                    self.walk(child, where)
        if element.tail:
            self.add(element.tail, where)


def _is_selector(cp: int) -> bool:
    return 0xFE00 <= cp <= 0xFE0F or 0xE0100 <= cp <= 0xE01EF or 0x180B <= cp <= 0x180F


def _strings(css_text: str) -> list:
    out = []

    def walk(nodes):
        for node in nodes or ():
            kind = getattr(node, "type", None)
            if kind == "string":
                out.append(node.value)
            elif kind == "function" and node.lower_name != "url":
                walk(node.arguments)
            elif kind in ("() block", "[] block", "{} block"):
                walk(node.content)

    walk(tinycss2.parse_component_value_list(css_text, skip_comments=True))
    return out


EMPHASIS_PROPERTIES = {"text-emphasis", "text-emphasis-style", "-webkit-text-emphasis",
                       "-webkit-text-emphasis-style", "-epub-text-emphasis", "-epub-text-emphasis-style"}
HYPHENS_PROPERTIES = {"hyphens", "-webkit-hyphens", "-epub-hyphens"}


def _declarations(nodes):
    """Every declaration, including those nested in @media / @supports / qualified rules."""
    for node in nodes:
        kind = getattr(node, "type", None)
        if kind == "declaration":
            yield node
        elif kind in ("qualified-rule", "at-rule") and node.content is not None:
            yield from _declarations(tinycss2.parse_blocks_contents(node.content, skip_comments=True, skip_whitespace=True))


def _generated(sheets: list, inline: list, has_q: bool, harvested: set) -> str:
    """Characters produced by CSS keywords rather than by strings in the text (SPEC §4 item 3)."""
    declarations = []
    for text in sheets:
        declarations += _declarations(tinycss2.parse_stylesheet(text, skip_comments=True, skip_whitespace=True))
    for text in inline:
        declarations += _declarations(tinycss2.parse_blocks_contents(text, skip_comments=True, skip_whitespace=True))
    out = []
    for decl in declarations:
        idents = {token.lower_value for token in decl.value if token.type == "ident"}
        if decl.lower_name in EMPHASIS_PROPERTIES:
            shapes = [shape for shape in EMPHASIS_MARKS if shape in idents]
            if not shapes and idents & {"filled", "open"}:
                shapes = ["circle", "sesame"]  # horizontal / vertical defaults
            out.extend(EMPHASIS_MARKS[shape] for shape in shapes)
        elif decl.lower_name in ("list-style", "list-style-type"):
            out.extend(chars for keyword, chars in LIST_MARKERS.items() if keyword in idents)
        elif decl.lower_name in HYPHENS_PROPERTIES and "auto" in idents:
            out.append("-")
        elif decl.lower_name == "text-transform":
            if idents & {"uppercase", "lowercase", "capitalize"}:
                for ch in harvested:
                    out.extend(v for v in (ch.upper(), ch.lower()) if len(v) == 1)
            if "full-width" in idents:
                out.extend(chr(ord(ch) + 0xFEE0) for ch in harvested if 0x21 <= ord(ch) <= 0x7E)
    if has_q:
        out.append("\u201c\u201d\u2018\u2019")
    return "".join(out)


def harvest_book(zf: zipfile.ZipFile) -> tuple:
    try:
        container = etree.fromstring(zf.read("META-INF/container.xml"))
    except (KeyError, etree.XMLSyntaxError) as exc:
        raise CheckError(f"cannot read META-INF/container.xml: {exc}") from exc
    opf_path = next((el.get("full-path") for el in container.iter() if _local(el.tag) == "rootfile"), None)
    if not opf_path:
        raise CheckError("container.xml has no rootfile")
    opf, warning = _xml(zf.read(opf_path), opf_path)
    base = posixpath.dirname(opf_path)
    items = []
    for el in opf.iter():
        if _local(el.tag) == "item" and el.get("href"):
            path = posixpath.normpath(posixpath.join(base, unquote(el.get("href").split("#")[0])))
            items.append((path, (el.get("media-type") or "").lower()))
    names = set(zf.namelist())
    harvest = Harvest()
    if warning:
        harvest.warnings.append(warning)
    for path, media in items:
        if path not in names:
            continue
        if media in MARKUP_TYPES:
            root, warning = _xml(zf.read(path), path)
            if warning:
                harvest.warnings.append(warning)
            harvest.walk(root, path)
        elif media == "text/css":
            harvest.css.append(zf.read(path).decode("utf-8-sig", errors="replace"))
    for text in harvest.css + harvest.inline:
        for value in _strings(text):
            harvest.add(value, "css")
    generated = _generated(harvest.css, harvest.inline, harvest.has_q, set(harvest.chars))
    harvest.add("".join(ch for ch in dict.fromkeys(generated) if ch not in harvest.chars), "css-generated")
    return items, harvest


def _encrypted(zf: zipfile.ZipFile) -> set:
    try:
        root = etree.fromstring(zf.read("META-INF/encryption.xml"))
    except KeyError:
        return set()
    return {unquote(el.get("URI", "")) for el in root.iter() if _local(el.tag) == "CipherReference"}


def _label(cp: int) -> str:
    return f"U+{cp:04X} {chr(cp)}"


def check_font(data: bytes, name: str, harvest: Harvest) -> dict:
    if data[:4] == b"ttcf":
        raise CheckError(f"{name}: font collections are not supported")
    try:
        font = TTFont(io.BytesIO(data), lazy=True)
        cmap = font.getBestCmap() or {}
    except Exception as exc:
        raise CheckError(f"{name}: cannot parse font ({exc}); obfuscated or damaged?") from exc
    glyphset = font.getGlyphSet()
    uvs = set()
    for table in font["cmap"].tables:
        if table.format == 14:
            for selector, entries in table.uvsDict.items():
                uvs.update((base, selector) for base, _glyph in entries)
    ink: dict[str, bool] = {}
    missing, no_ink, optional = [], [], []
    for ch, info in sorted(harvest.chars.items(), key=lambda kv: ord(kv[0])):
        cp = ord(ch)
        category = unicodedata.category(ch)
        glyph = cmap.get(cp)
        record = {"char": _label(cp), "count": info["count"], "first": info["first"]}
        if glyph is None or glyph == ".notdef":
            (optional if category == "Cf" else missing).append(record)
            continue
        if category in ("Zs", "Zl", "Zp", "Cf"):
            continue
        if glyph not in ink:
            pen = BoundsPen(glyphset)
            glyphset[glyph].draw(pen)
            ink[glyph] = pen.bounds is not None
        if not ink[glyph]:
            no_ink.append(record)
    sequences = [
        {"sequence": f"U+{base:04X} U+{sel:04X}", "count": info["count"], "first": info["first"]}
        for (base, sel), info in sorted(harvest.sequences.items()) if (base, sel) not in uvs
    ]
    return {
        "font": name,
        "sha256": hashlib.sha256(data).hexdigest(),
        "glyphs": len(font.getGlyphOrder()),
        "variable": "fvar" in font,
        "required": len(harvest.chars),
        "requiredSequences": len(harvest.sequences),
        "missing": missing,
        "noInk": no_ink,
        "missingSequences": sequences,
        "optionalMissing": optional,
        "ok": not (missing or no_ink or sequences),
    }


def _chars_from_file(path: Path) -> Harvest:
    harvest = Harvest()
    harvest.add(path.read_text(encoding="utf-8").replace("\n", "").replace("\r", "").replace("\t", ""), path.name)
    return harvest


def run(args) -> int:
    epub = Path(args.epub)
    if not epub.is_file():
        raise CheckError(f"{epub} is not a file")
    if not (args.font or args.all_fonts or args.font_file):
        raise CheckError("choose the fonts to check: --font PATH_IN_EPUB (repeatable), --all-fonts, or --font-file FILE")
    with zipfile.ZipFile(epub) as zf:
        items, harvest = harvest_book(zf)
        if args.chars_file:
            harvest = _chars_from_file(Path(args.chars_file))
        manifest_fonts = [p for p, media in items if p.lower().endswith(FONT_EXTENSIONS) or "font" in media]
        targets = list(dict.fromkeys(manifest_fonts if args.all_fonts else (args.font or [])))
        encrypted = _encrypted(zf)
        results = []
        for target in targets:
            if target not in manifest_fonts or target not in zf.namelist():
                raise CheckError(f"{target} is not a font in the manifest; manifest fonts: {manifest_fonts}")
            if target in encrypted:
                raise CheckError(f"{target} is obfuscated (META-INF/encryption.xml); check the clear master with --font-file")
            results.append(check_font(zf.read(target), target, harvest))
    for path in args.font_file or []:
        results.append(check_font(Path(path).read_bytes(), str(path), harvest))
    if not results:
        raise CheckError("no fonts to check (the book has no manifest fonts)")

    report = {
        "tool": "subset_demo.fullcheck",
        "input": {"path": str(epub), "sha256": hashlib.sha256(epub.read_bytes()).hexdigest()},
        "scope": f"chars-file:{args.chars_file}" if args.chars_file else "book",
        "requiredChars": len(harvest.chars),
        "requiredSequences": len(harvest.sequences),
        "warnings": harvest.warnings,
        "fonts": results,
        "ok": all(r["ok"] for r in results),
    }
    if args.json:
        Path(args.json).write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"scope={report['scope']} required chars={report['requiredChars']} sequences={report['requiredSequences']}")
    for warning in harvest.warnings:
        print(f"  warning: {warning}")
    for r in results:
        print(f"[{'OK' if r['ok'] else 'INCOMPLETE'}] {r['font']} glyphs={r['glyphs']} "
              f"missing={len(r['missing'])} noInk={len(r['noInk'])} missingSequences={len(r['missingSequences'])} "
              f"optionalMissing={len(r['optionalMissing'])}")
        for key in ("missing", "noInk", "missingSequences"):
            for item in r[key][:20]:
                what = item.get("char") or item.get("sequence")
                print(f"  {key}: {what} x{item['count']} (first in {item['first']})")
            if len(r[key]) > 20:
                print(f"  {key}: ... {len(r[key]) - 20} more (see --json)")
    return 0 if report["ok"] else 1


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="python -m subset_demo.fullcheck", description=__doc__.splitlines()[0])
    parser.add_argument("epub")
    parser.add_argument("--font", action="append", help="font path inside the EPUB (repeatable)")
    parser.add_argument("--all-fonts", action="store_true", help="check every manifest font against the same set")
    parser.add_argument("--font-file", action="append", help="font file outside the EPUB (repeatable)")
    parser.add_argument("--chars-file", help="UTF-8 file listing the required characters instead of the whole book")
    parser.add_argument("--json", help="write the full report here")
    args = parser.parse_args(argv)
    try:
        return run(args)
    except CheckError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
