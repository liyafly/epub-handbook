"""Conservative CSS font-use attribution for optional per-font subsets.

The parser understands simple type/class/id/universal selector compounds and
the descendant/child combinators. Anything it cannot prove broadens the
affected font to the whole-book charset; it never guesses a narrower scope.
"""

from __future__ import annotations

import posixpath
import zipfile
from dataclasses import dataclass, field
from urllib.parse import unquote, urlsplit

import tinycss2
from lxml import etree
from tinycss2.ast import (
    AtRule,
    Declaration,
    DimensionToken,
    FunctionBlock,
    HashToken,
    IdentToken,
    LiteralToken,
    NumberToken,
    ParseError,
    PercentageToken,
    QualifiedRule,
    StringToken,
    URLToken,
    WhitespaceToken,
)

from . import check, epubtext

_MARKUP_TYPES = {"application/xhtml+xml", "image/svg+xml", "application/x-dtbncx+xml"}
_CSS_WIDE = {"inherit", "initial", "unset", "revert", "revert-layer"}
_FONT_SIZE_KEYWORDS = {
    "xx-small", "x-small", "small", "medium", "large", "x-large", "xx-large", "xxx-large", "larger", "smaller",
}
_FONT_SIZE_UNITS = {
    "cap", "ch", "cm", "em", "ex", "ic", "in", "lh", "mm", "pc", "pt", "px", "q", "rch", "rem", "rex",
    "ric", "rlh", "vb", "vh", "vi", "vmax", "vmin", "vw", "svb", "svh", "svi", "svmax", "svmin", "svw",
    "lvb", "lvh", "lvi", "lvmax", "lvmin", "lvw", "dvb", "dvh", "dvi", "dvmax", "dvmin", "dvw",
}
_SYSTEM_FONT_KEYWORDS = {"caption", "icon", "menu", "message-box", "small-caption", "status-bar"}
_MAX_FALLBACK_REASONS = 64
_MAX_FALLBACK_REASON_LENGTH = 4096


class UnsafeUsageError(Exception):
    """CSS usage mode cannot prove that its character set is complete."""
_SKIP_TAGS = {"script", "style"}


@dataclass
class FontUsage:
    mode: str
    codepoints: set[int] = field(default_factory=set)
    harvest: check.Harvest = field(default_factory=check.Harvest)
    reasons: list[str] = field(default_factory=list)

    def add(self, text: str, where: str) -> None:
        self.codepoints.update(
            ord(char) for char in text
            if ord(char) >= 0x20 and not 0x7F <= ord(char) <= 0x9F and not 0xD800 <= ord(char) <= 0xDFFF
        )
        self.harvest.add(text, where)


@dataclass
class UsagePlan:
    mode: str
    fonts: dict[str, FontUsage]
    fallback_reasons: list[str]


@dataclass(frozen=True)
class _View:
    tag: str
    element_id: str
    classes: frozenset[str]


@dataclass(frozen=True)
class _Compound:
    tag: str | None = None
    element_id: str | None = None
    classes: tuple[str, ...] = ()
    universal: bool = False


@dataclass(frozen=True)
class _Selector:
    # Each combinator relates this compound to the preceding compound.
    parts: tuple[tuple[str, _Compound], ...]

    @property
    def index_key(self) -> tuple[str, str]:
        _combinator, compound = self.parts[-1]
        if compound.element_id:
            return "id", compound.element_id
        if compound.classes:
            return "class", compound.classes[0]
        if compound.tag:
            return "tag", compound.tag.lower()
        return "all", "*"

    def matches(self, chain: tuple[_View, ...]) -> bool:
        if not chain or not self.parts or not _matches_compound(chain[-1], self.parts[-1][1]):
            return False
        element_index = len(chain) - 1
        part_index = len(self.parts) - 1
        while part_index > 0:
            combinator, _compound = self.parts[part_index]
            previous_compound = self.parts[part_index - 1][1]
            if combinator == ">":
                element_index -= 1
                if element_index < 0 or not _matches_compound(chain[element_index], previous_compound):
                    return False
            else:
                element_index -= 1
                while element_index >= 0 and not _matches_compound(chain[element_index], previous_compound):
                    element_index -= 1
                if element_index < 0:
                    return False
            part_index -= 1
        return True


@dataclass(frozen=True)
class _Rule:
    selector: _Selector
    families: tuple[str, ...]
    conditional: bool = False


class _RuleIndex:
    def __init__(self) -> None:
        self._by_key: dict[tuple[str, str], list[_Rule]] = {}

    def add(self, rule: _Rule) -> None:
        self._by_key.setdefault(rule.selector.index_key, []).append(rule)

    def matching(self, view: _View, chain: tuple[_View, ...]) -> tuple[set[str], set[str]]:
        candidates = [*self._by_key.get(("tag", view.tag.lower()), ()), *self._by_key.get(("id", view.element_id), ())]
        for class_name in view.classes:
            candidates.extend(self._by_key.get(("class", class_name), ()))
        candidates.extend(self._by_key.get(("all", "*"), ()))
        definite: set[str] = set()
        conditional: set[str] = set()
        seen: set[int] = set()
        for rule in candidates:
            identity = id(rule)
            if identity not in seen and rule.selector.matches(chain):
                seen.add(identity)
                (conditional if rule.conditional else definite).update(rule.families)
        return definite, conditional


def _matches_compound(view: _View, compound: _Compound) -> bool:
    if compound.tag and view.tag.lower() != compound.tag.lower():
        return False
    if compound.element_id and view.element_id != compound.element_id:
        return False
    return all(class_name in view.classes for class_name in compound.classes)


def _parse_compound(tokens: list) -> _Compound | None:
    tag: str | None = None
    element_id: str | None = None
    classes: list[str] = []
    universal = False
    index = 0
    while index < len(tokens):
        token = tokens[index]
        if isinstance(token, LiteralToken) and token.value == ".":
            index += 1
            if index >= len(tokens) or not isinstance(tokens[index], IdentToken):
                return None
            classes.append(tokens[index].value)
        elif isinstance(token, LiteralToken) and token.value == "#":
            index += 1
            if index >= len(tokens) or not isinstance(tokens[index], IdentToken) or element_id is not None:
                return None
            element_id = tokens[index].value
        elif isinstance(token, HashToken):
            if element_id is not None or not token.is_identifier:
                return None
            element_id = token.value
        elif isinstance(token, LiteralToken) and token.value == "*":
            if tag is not None or universal:
                return None
            universal = True
        elif isinstance(token, IdentToken):
            if tag is not None or universal:
                return None
            tag = token.value
        else:
            return None
        index += 1
    if tag is None and element_id is None and not classes and not universal:
        return None
    return _Compound(tag, element_id, tuple(classes), universal)


def _parse_selector(tokens: list) -> _Selector | None:
    parts: list[tuple[str, _Compound]] = []
    current: list = []
    pending = ""
    index = 0
    while index < len(tokens):
        token = tokens[index]
        if isinstance(token, WhitespaceToken):
            if current:
                compound = _parse_compound(current)
                if compound is None:
                    return None
                parts.append((pending, compound))
                current = []
                pending = " "
            index += 1
            continue
        if isinstance(token, LiteralToken) and token.value == ">":
            if current:
                compound = _parse_compound(current)
                if compound is None:
                    return None
                parts.append((pending, compound))
                current = []
            if not parts or pending == ">":
                return None
            pending = ">"
            index += 1
            continue
        current.append(token)
        index += 1
    if current:
        compound = _parse_compound(current)
        if compound is None:
            return None
        parts.append((pending, compound))
    if not parts or parts[0][0] not in ("",):
        return None
    return _Selector(tuple(parts))


def _selector_groups(tokens: list) -> list[list]:
    groups: list[list] = [[]]
    for token in tokens:
        if isinstance(token, LiteralToken) and token.value == ",":
            groups.append([])
        else:
            groups[-1].append(token)
    return groups


def _families(tokens: list) -> tuple[str, ...] | None:
    """Read a font-family list; return None when a value cannot be proven."""
    significant = [token for token in tokens if not isinstance(token, WhitespaceToken)]
    if len(significant) == 1 and isinstance(significant[0], IdentToken) and significant[0].value.lower() in _CSS_WIDE:
        return ()
    if any(isinstance(token, IdentToken) and token.value.lower() in _CSS_WIDE for token in significant):
        return None
    result: list[str] = []
    current: list[str] = []

    def flush() -> None:
        nonlocal current
        value = "".join(current).strip()
        if value:
            result.append(value)
        current = []

    for token in tokens:
        if isinstance(token, WhitespaceToken):
            if current and current[-1] != " ":
                current.append(" ")
        elif isinstance(token, LiteralToken) and token.value == ",":
            flush()
        elif isinstance(token, StringToken):
            if not token.value:
                return None
            flush()
            result.append(token.value)
        elif isinstance(token, IdentToken):
            if token.value.lower() in _CSS_WIDE:
                return None
            if current and current[-1] != " ":
                current.append(" ")
            current.append(token.value)
        else:
            return None
    flush()
    return tuple(result) if result else None


def _declaration_families(content: list) -> tuple[str, ...] | None:
    found: list[str] = []
    declarations = content if content and isinstance(content[0], (Declaration, ParseError)) else tinycss2.parse_declaration_list(
        content, skip_comments=True, skip_whitespace=True
    )
    for item in declarations:
        if isinstance(item, ParseError):
            return None
        if isinstance(item, Declaration):
            if item.lower_name == "font":
                names = _font_shorthand_families(item.value)
                if names is None:
                    return None
                found.extend(names)
            if item.lower_name == "font-family":
                names = _families(item.value)
                if names is None:
                    return None
                found.extend(names)
    return tuple(found)


def _has_attr_content(declarations: list) -> bool:
    def contains_attr(tokens: list) -> bool:
        for token in tokens:
            if isinstance(token, FunctionBlock):
                if token.lower_name == "attr" or contains_attr(token.arguments):
                    return True
            elif getattr(token, "content", None) and contains_attr(token.content):
                return True
        return False

    return any(
        isinstance(item, Declaration) and item.lower_name == "content" and contains_attr(item.value)
        for item in declarations
    )


def _counter_symbols(rule: AtRule) -> set[str]:
    symbols: set[str] = set()
    for item in tinycss2.parse_declaration_list(rule.content or [], skip_comments=True, skip_whitespace=True):
        if not isinstance(item, Declaration) or item.lower_name not in {
            "symbols", "additive-symbols", "prefix", "suffix",
        }:
            continue
        for token in item.value:
            if isinstance(token, (StringToken, IdentToken)):
                symbols.update(token.value)
    return symbols


def _font_shorthand_families(tokens: list) -> tuple[str, ...] | None:
    significant = [(index, token) for index, token in enumerate(tokens) if not isinstance(token, WhitespaceToken)]
    if len(significant) == 1 and isinstance(significant[0][1], IdentToken):
        keyword = significant[0][1].value.lower()
        if keyword in _CSS_WIDE:
            return ()
        if keyword in _SYSTEM_FONT_KEYWORDS:
            return None
    elif any(
        isinstance(token, IdentToken) and token.value.lower() in (_CSS_WIDE | _SYSTEM_FONT_KEYWORDS)
        for _index, token in significant
    ):
        return None
    if any(isinstance(token, FunctionBlock) for _index, token in significant):
        return None
    size_position = None
    for offset, (index, token) in enumerate(significant):
        if (
            isinstance(token, IdentToken) and token.value.lower() in _FONT_SIZE_KEYWORDS
            or isinstance(token, DimensionToken) and token.lower_unit in _FONT_SIZE_UNITS and token.value >= 0
            or isinstance(token, PercentageToken) and token.value >= 0
            or isinstance(token, NumberToken) and token.value == 0
        ):
            size_position = (offset, index)
            break
    if size_position is None:
        return None
    offset, _size_index = size_position
    family_offset = offset + 1
    if family_offset < len(significant) and isinstance(significant[family_offset][1], LiteralToken) and significant[family_offset][1].value == "/":
        line_height_offset = family_offset + 1
        if line_height_offset >= len(significant):
            return None
        line_height = significant[line_height_offset][1]
        valid_line_height = (
            isinstance(line_height, IdentToken) and line_height.value.lower() == "normal"
            or isinstance(line_height, NumberToken) and line_height.value >= 0
            or isinstance(line_height, DimensionToken)
            and line_height.lower_unit in _FONT_SIZE_UNITS and line_height.value >= 0
            or isinstance(line_height, PercentageToken) and line_height.value >= 0
        )
        if not valid_line_height:
            return None
        family_offset += 2
    if family_offset >= len(significant):
        return None
    family_index = significant[family_offset][0]
    return _families(tokens[family_index:])


def _url_values(tokens: list) -> list[str]:
    urls: list[str] = []
    for token in tokens:
        if isinstance(token, URLToken):
            urls.append(token.value)
        elif isinstance(token, FunctionBlock) and token.lower_name == "url":
            for argument in token.arguments:
                if isinstance(argument, StringToken):
                    urls.append(argument.value)
                    break
                if isinstance(argument, URLToken):
                    urls.append(argument.value)
                    break
    return urls


def _resource_path(base_path: str, href: str) -> str | None:
    parsed = urlsplit(href.strip())
    if parsed.scheme or parsed.netloc or href.startswith("/"):
        return None
    path = unquote(parsed.path)
    if not path:
        return None
    resolved = posixpath.normpath(posixpath.join(posixpath.dirname(base_path), path))
    if resolved in ("..", ".") or resolved.startswith("../"):
        return None
    return resolved


def _add_whole_book(usage: FontUsage, chars: set[str], independent: check.Harvest, where: str) -> None:
    usage.add("".join(sorted(chars, key=ord)), where)
    for base, selector in sorted(independent.sequences):
        usage.add(chr(base) + chr(selector), f"{where}-sequence")


class _Planner:
    def __init__(self, zf: zipfile.ZipFile, book: epubtext.BookText, targets: list[str]) -> None:
        self.zf = zf
        self.book = book
        self.targets = tuple(targets)
        self.target_set = set(targets)
        self.names = set(zf.namelist())
        self.face_targets: dict[str, set[str]] = {}
        self.rules = _RuleIndex()
        self.global_families: set[str] = set()
        self.global_all = False
        self.fallback_reasons: list[str] = []
        self.unsafe_reason: str | None = None
        self.generated_text: set[str] = set()
        self.attribute_text: set[str] = set()
        self.include_all_attributes = False
        self.needs_case_variants = False
        self.needs_fullwidth_variants = False
        self._parsed_sheets: set[str] = set()
        self._active_sheets: set[str] = set()
        self._markup_styles: list[tuple[str, str]] = []
        self._markup_documents: list[tuple[epubtext.ManifestItem, object]] = []
        self._linked_stylesheets: set[str] = set()

    def reason(self, text: str) -> None:
        text = text[:_MAX_FALLBACK_REASON_LENGTH]
        if text not in self.fallback_reasons:
            if len(self.fallback_reasons) < _MAX_FALLBACK_REASONS - 1:
                self.fallback_reasons.append(text)
            elif len(self.fallback_reasons) == _MAX_FALLBACK_REASONS - 1:
                self.fallback_reasons.append("Additional fallback reasons omitted (limit 64)")

    def unsafe(self, text: str) -> None:
        if self.unsafe_reason is None:
            self.unsafe_reason = text
        self.global_all = True
        self.reason(f"{text}; CSS usage mode refused")

    def _record_text_transform(self, declarations: list, where: str) -> None:
        safe_values = {"none", "capitalize", "uppercase", "lowercase", "full-width"}
        for declaration in declarations:
            if not isinstance(declaration, Declaration) or declaration.lower_name != "text-transform":
                continue
            identifiers = {
                token.value.lower() for token in declaration.value if isinstance(token, IdentToken)
            }
            functions = [token for token in declaration.value if isinstance(token, FunctionBlock)]
            if functions or not identifiers or identifiers - safe_values:
                self.unsafe(f"{where}: text-transform cannot be proven to use known glyph variants")
                continue
            self.needs_case_variants |= bool(identifiers & {"capitalize", "uppercase", "lowercase"})
            self.needs_fullwidth_variants |= "full-width" in identifiers

    def _record_generated_style(self, declarations: list, where: str) -> None:
        known_styles = set(check.LIST_MARKERS) | _CSS_WIDE | {"none", "inside", "outside"}
        for declaration in declarations:
            if not isinstance(declaration, Declaration):
                continue
            if declaration.lower_name in {"quotes", *check.EMPHASIS_PROPERTIES}:
                if any(
                    isinstance(token, FunctionBlock) and token.lower_name == "var"
                    for token in declaration.value
                ):
                    self.unsafe(f"{where}: generated quote or emphasis style uses an unresolved CSS variable")
            if declaration.lower_name in {"list-style", "list-style-type"}:
                for token in declaration.value:
                    if isinstance(token, FunctionBlock):
                        if token.lower_name != "url":
                            self.unsafe(f"{where}: functional list-style value is unsupported")
                    elif isinstance(token, IdentToken) and token.value.lower() not in known_styles:
                        self.unsafe(f"{where}: list-style counter type {token.value!r} is unsupported")
            if declaration.lower_name != "content":
                continue

            def inspect(tokens: list) -> None:
                for token in tokens:
                    if not isinstance(token, FunctionBlock):
                        continue
                    if token.lower_name not in {"attr", "counter", "counters", "type", "url"}:
                        self.unsafe(f"{where}: generated content uses an unresolved CSS function")
                    if token.lower_name in {"counter", "counters"}:
                        segments: list[list] = [[]]
                        for argument in token.arguments:
                            if isinstance(argument, LiteralToken) and argument.value == ",":
                                segments.append([])
                            else:
                                segments[-1].append(argument)
                        style_index = 1 if token.lower_name == "counter" else 2
                        if len(segments) > style_index:
                            style = [value for value in segments[style_index] if not isinstance(value, WhitespaceToken)]
                            if (
                                len(style) != 1
                                or not isinstance(style[0], IdentToken)
                                or style[0].value.lower() not in known_styles
                            ):
                                self.unsafe(f"{where}: generated counter style is unsupported")
                    inspect(token.arguments)

            inspect(declaration.value)

    def _parse_font_face(self, rule: AtRule, css_path: str) -> None:
        family: tuple[str, ...] | None = None
        urls: list[str] = []
        for item in tinycss2.parse_declaration_list(rule.content or [], skip_comments=True, skip_whitespace=True):
            if isinstance(item, ParseError):
                self.global_all = True
                self.reason(f"{css_path}: malformed @font-face declaration; using whole-book subsets")
                return
            if not isinstance(item, Declaration):
                continue
            if item.lower_name == "font-family":
                family = _families(item.value)
            elif item.lower_name == "src":
                urls.extend(_url_values(item.value))
        if family is None:
            self.global_all = True
            self.reason(f"{css_path}: @font-face family is not understood; using whole-book subsets")
            return
        if len(family) != 1:
            self.global_all = True
            self.reason(f"{css_path}: @font-face family descriptor is not a single family; using whole-book subsets")
            return
        for href in urls:
            path = _resource_path(css_path, href)
            if path is None:
                continue
            if path not in self.names:
                self.global_all = True
                self.reason(f"{css_path}: @font-face resource {href!r} is missing; using whole-book subsets")
                continue
            if path in self.target_set:
                for name in family:
                    self.face_targets.setdefault(name.casefold(), set()).add(path)

    def _parse_stylesheet(
        self,
        css_path: str,
        text: str | None = None,
        cache_key: str | None = None,
        conditional: bool = False,
    ) -> None:
        if cache_key is None:
            cache_key = f"{css_path}|conditional" if conditional else (css_path if text is None else "")
        if cache_key and cache_key in self._parsed_sheets:
            return
        if cache_key and cache_key in self._active_sheets:
            self.global_all = True
            self.reason(f"{css_path}: cyclic @import; using whole-book subsets")
            return
        if cache_key:
            self._active_sheets.add(cache_key)
        if text is None:
            try:
                text = epubtext.decode_text(self.zf.read(css_path))
            except (KeyError, OSError, UnicodeDecodeError):
                self.unsafe(f"{css_path}: stylesheet cannot be read")
                if cache_key:
                    self._active_sheets.remove(cache_key)
                return
        nodes = tinycss2.parse_stylesheet(text, skip_comments=True, skip_whitespace=True)
        for value in check._strings(text):
            self.generated_text.update(value)
        for node in nodes:
            if isinstance(node, ParseError):
                self.global_all = True
                self.unsafe(f"{css_path}: stylesheet parse error")
            elif isinstance(node, AtRule):
                keyword = node.lower_at_keyword
                if keyword == "font-face":
                    self._parse_font_face(node, css_path)
                elif keyword == "counter-style":
                    self.generated_text.update(_counter_symbols(node))
                elif keyword == "import":
                    urls = _url_values(node.prelude)
                    if not urls:
                        urls = [token.value for token in node.prelude if isinstance(token, StringToken)]
                    imported = _resource_path(css_path, urls[0]) if urls else None
                    if imported is None or imported not in self.names:
                        self.global_all = True
                        self.unsafe(f"{css_path}: @import cannot be resolved locally")
                    else:
                        meaningful = [token for token in node.prelude if not isinstance(token, WhitespaceToken)]
                        self._parse_stylesheet(imported, conditional=conditional or len(meaningful) > 1)
                elif node.content is not None:
                    nested = tinycss2.parse_blocks_contents(node.content, skip_comments=True, skip_whitespace=True)
                    self._parse_rules(nested, css_path)
            elif isinstance(node, QualifiedRule):
                self._add_qualified_rule(node, css_path, conditional=conditional)
        if cache_key:
            self._active_sheets.remove(cache_key)
            self._parsed_sheets.add(cache_key)

    def _parse_rules(self, nodes: list, css_path: str) -> None:
        for node in nodes:
            if isinstance(node, ParseError):
                self.global_all = True
                self.unsafe(f"{css_path}: nested stylesheet parse error")
            elif isinstance(node, AtRule):
                if node.lower_at_keyword == "font-face":
                    self._parse_font_face(node, css_path)
                elif node.lower_at_keyword == "counter-style":
                    self.generated_text.update(_counter_symbols(node))
                elif node.content is not None:
                    nested = tinycss2.parse_blocks_contents(node.content, skip_comments=True, skip_whitespace=True)
                    self._parse_rules(nested, css_path)
            elif isinstance(node, QualifiedRule):
                self._add_qualified_rule(node, css_path, conditional=True)
            elif isinstance(node, Declaration):
                self._record_text_transform([node], css_path)
                self._record_generated_style([node], css_path)
                if node.lower_name in {"font", "font-family"}:
                    self.global_all = True
                    self.reason(f"{css_path}: unscoped font declaration; using whole-book subsets")

    def _add_qualified_rule(self, rule: QualifiedRule, css_path: str, conditional: bool = False) -> None:
        declarations = tinycss2.parse_declaration_list(rule.content or [], skip_comments=True, skip_whitespace=True)
        if any(isinstance(item, ParseError) for item in declarations):
            self.global_all = True
            self.unsafe(f"{css_path}: malformed CSS declaration")
            return
        self._record_text_transform(declarations, css_path)
        self._record_generated_style(declarations, css_path)
        if _has_attr_content(declarations):
            self.include_all_attributes = True
        families = _declaration_families(rule.content or [])
        if families is None:
            self.global_all = True
            self.reason(f"{css_path}: malformed CSS declaration; using whole-book subsets")
            return
        if not families:
            return
        selectors = [_parse_selector(group) for group in _selector_groups(rule.prelude)]
        if not selectors or any(selector is None for selector in selectors):
            self.global_families.update(family.casefold() for family in families)
            self.reason(f"{css_path}: unsupported selector; affected font uses whole-book text")
            return
        for selector in selectors:
            self.rules.add(_Rule(selector, families, conditional))

    def parse_styles(self) -> None:
        for item in self.book.items:
            if item.media_type in _MARKUP_TYPES and item.path in self.names:
                try:
                    root, _warning = check._xml(self.zf.read(item.path), item.path)
                except (KeyError, check.CheckError, OSError, etree.XMLSyntaxError):
                    self.unsafe(f"{item.path}: markup cannot be scanned for CSS")
                    continue
                for instruction in root.getroottree().xpath("//processing-instruction('xml-stylesheet')"):
                    self.unsafe(f"{item.path}: XML stylesheet processing instructions are unsupported")
                self._markup_documents.append((item, root))
                for element in root.iter():
                    if isinstance(element, etree._ProcessingInstruction):
                        if element.target.lower() == "xml-stylesheet":
                            self.unsafe(f"{item.path}: XML stylesheet processing instructions are unsupported")
                        continue
                    if not isinstance(element.tag, str):
                        continue
                    self.attribute_text.update(char for value in element.attrib.values() for char in value)
                    tag = check._local(element.tag).lower()
                    if tag == "script":
                        self.unsafe(f"{item.path}: scripted content may change font use")
                    elif item.media_type == "image/svg+xml" and tag == "use":
                        self.unsafe(f"{item.path}: SVG use elements can render referenced text with inherited styles")
                    elif tag in {"iframe", "object", "embed"}:
                        self.unsafe(f"{item.path}: embedded resources may add unscanned text or styles")
                    elif tag == "style":
                        self._markup_styles.append((item.path, "".join(element.itertext())))
                    elif tag == "link" and "stylesheet" in (element.get("rel") or "").lower().split():
                        href = element.get("href", "")
                        linked = _resource_path(item.path, href)
                        if linked is None or linked not in self.zf.namelist():
                            self.unsafe(f"{item.path}: linked stylesheet cannot be resolved")
                        else:
                            self._linked_stylesheets.add(linked)
                    for name, value in element.attrib.items():
                        local_name = check._local(name).lower()
                        if local_name.startswith("on") and value.strip():
                            self.unsafe(f"{item.path}: executable event attribute may change font use")
                        if local_name in {"href", "src"} and urlsplit(value.strip()).scheme.lower() == "javascript":
                            self.unsafe(f"{item.path}: javascript URL may change font use")
                        if local_name == "style":
                            declarations = tinycss2.parse_declaration_list(value, skip_comments=True, skip_whitespace=True)
                            if any(isinstance(node, ParseError) for node in declarations):
                                self.unsafe(f"{item.path}: malformed inline CSS")
                            if _has_attr_content(declarations):
                                self.include_all_attributes = True
                            self._record_text_transform(declarations, item.path)
                            self._record_generated_style(declarations, item.path)
                            families = _declaration_families(declarations)
                            if families is None:
                                self.global_all = True
                                self.reason(f"{item.path}: inline font shorthand or malformed CSS; using whole-book subsets")
        for path in sorted(self._linked_stylesheets):
            self._parse_stylesheet(path)
        for index, (path, style_text) in enumerate(self._markup_styles):
            if style_text:
                self._parse_stylesheet(path, style_text, cache_key=f"inline:{path}:{index}")

    def _targets_for(self, families: set[str]) -> set[str]:
        targets: set[str] = set()
        for family in families:
            targets.update(self.face_targets.get(family.casefold(), ()))
        return targets if targets else set(self.targets)

    def build(self, independent: check.Harvest) -> UsagePlan:
        self.parse_styles()
        if self.unsafe_reason is not None:
            raise UnsafeUsageError(self.unsafe_reason + "; remove the construct or use usage=book")
        all_chars = self.book.all_chars() | self.generated_text
        if self.include_all_attributes:
            all_chars.update(self.attribute_text)
        if self.needs_case_variants:
            variants = {
                output_char
                for char in all_chars
                for mapping in (char.upper(), char.lower())
                for output_char in mapping
            }
            self.generated_text.update(variants)
            all_chars.update(variants)
        if self.needs_fullwidth_variants:
            variants = {
                chr(ord(char) + 0xFEE0) if 0x21 <= ord(char) <= 0x7E else "　"
                for char in all_chars
                if 0x21 <= ord(char) <= 0x7E or char == " "
            }
            self.generated_text.update(variants)
            all_chars.update(variants)
        usages = {target: FontUsage("css") for target in self.targets}
        all_targets = set(self.targets)
        whole_book_targets = self._targets_for(self.global_families) if self.global_families else set()
        if self.global_all:
            whole_book_targets = all_targets

        global_chars: set[str] = set()
        for source in ("baseline", "css-strings", "case-variants", "full-width-variants"):
            global_chars.update(self.book.chars_by_source.get(source, ()))
        for char, info in independent.chars.items():
            if info.get("first") in {"css", "css-generated"}:
                global_chars.add(char)
        global_chars.update(self.generated_text)
        if self.include_all_attributes:
            global_chars.update(self.attribute_text)
        global_sequences = {
            sequence for sequence, info in independent.sequences.items()
            if info.get("first") in {"css", "css-generated"}
        }
        for target, usage in usages.items():
            for text in global_chars:
                usage.add(text, "css-or-baseline")
            for base, selector in global_sequences:
                usage.add(chr(base) + chr(selector), "css-generated")
            if target in whole_book_targets:
                usage.mode = "book"
                usage.reasons.append("CSS usage could not be safely scoped")
                _add_whole_book(usage, all_chars, independent, "whole-book-fallback")

        for item, root in self._markup_documents:

            def walk(element, ancestors: tuple[_View, ...], inherited: set[str], force_global: bool = False) -> None:
                if not isinstance(element.tag, str):
                    return
                tag = check._local(element.tag).lower()
                attributes = {check._local(key).lower(): value for key, value in element.attrib.items()}
                view = _View(tag, attributes.get("id", ""), frozenset(attributes.get("class", "").split()))
                chain = (*ancestors, view)
                direct_families, conditional_families = self.rules.matching(view, chain)
                inline_value = attributes.get("style")
                inline_families = (
                    _declaration_families(tinycss2.parse_declaration_list(
                        inline_value, skip_comments=True, skip_whitespace=True
                    )) if inline_value else ()
                )
                families: set[str] = set()
                if inline_families is None:
                    direct_families = set()
                    targets = all_targets
                else:
                    if inline_families:
                        direct_families.update(inline_families)
                    presentation = None
                    if item.media_type == "image/svg+xml" and attributes.get("font-family"):
                        presentation = _families(tinycss2.parse_component_value_list(attributes["font-family"]))
                    elif tag == "font" and attributes.get("face"):
                        presentation = _families(tinycss2.parse_component_value_list(attributes["face"]))
                    if presentation is None and (
                        item.media_type == "image/svg+xml" and attributes.get("font-family")
                        or tag == "font" and attributes.get("face")
                    ):
                        self.global_all = True
                        self.reason(f"{item.path}: unsupported presentation font-family; using whole-book subsets")
                        direct_families = set()
                        targets = all_targets
                    else:
                        if presentation:
                            direct_families.update(presentation)
                        families = set(direct_families if direct_families else inherited)
                        families.update(conditional_families)
                        targets = self._targets_for(families)
                if force_global or tag == "nav":
                    targets = all_targets

                def record(text: str | None, selected: set[str] = targets) -> None:
                    if text:
                        for target in selected:
                            usages[target].add(text, item.path)

                if tag not in _SKIP_TAGS:
                    record(element.text)
                    for key, value in attributes.items():
                        if key in epubtext.TEXT_ATTRIBUTES:
                            record(value)
                    for child in element:
                        walk(child, chain, families, force_global or tag == "nav")
                        record(child.tail)

            walk(root, (), set(), item.media_type == "application/x-dtbncx+xml" or item.path.lower().endswith(".ncx"))

        # A missing assignment is evidence that the CSS parser did not understand
        # some required text. Broaden every target rather than producing a hole.
        covered = set().union(*(set(map(chr, usage.codepoints)) for usage in usages.values())) if usages else set()
        missing = all_chars - covered
        if missing:
            self.reason(f"{len(missing)} book characters had no proven font role; using whole-book subsets")
            for usage in usages.values():
                usage.mode = "book"
                usage.reasons.append("unattributed book characters")
                _add_whole_book(usage, all_chars, independent, "whole-book-fallback")

        if self.global_all:
            for usage in usages.values():
                usage.mode = "book"
                _add_whole_book(usage, all_chars, independent, "whole-book-fallback")

        for target in self.targets:
            if not self.face_targets or not any(target in values for values in self.face_targets.values()):
                usages[target].mode = "book"
                usages[target].reasons.append("font target has no resolved @font-face family")
                _add_whole_book(usages[target], all_chars, independent, "whole-book-fallback")
                self.reason(f"{target}: no resolved @font-face family; this font uses whole-book text")

        # Preserve predictable order for the JSON sidecar and findings.
        self.fallback_reasons.sort()
        return UsagePlan("css", usages, list(self.fallback_reasons))


def build_plan(
    mode: str,
    zf: zipfile.ZipFile,
    book: epubtext.BookText,
    independent: check.Harvest,
    targets: list[str],
) -> UsagePlan:
    """Build a book-wide or CSS-role plan for the manifest font targets."""
    if mode == "book":
        return UsagePlan(
            "book",
            {target: FontUsage("book", set(map(ord, book.all_chars())), independent) for target in targets},
            [],
        )
    planner = _Planner(zf, book, targets)
    return planner.build(independent)
