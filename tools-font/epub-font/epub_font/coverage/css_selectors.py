"""Pragmatic CSS selector parsing, matching, and cascade resolution."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from tinycss2 import parse_declaration_list, parse_stylesheet
from tinycss2.ast import Declaration as CSSDecl, FunctionBlock, HashToken, IdentToken, LiteralToken, QualifiedRule, WhitespaceToken

from .css_fonts import decode_css, parse_font_family_list


def _element_attr(element: dict, *keys: str) -> Any:
    for key in keys:
        value = element.get(key)
        if value is not None:
            return value
    return None


def _element_classes(element: dict) -> list[str]:
    raw = _element_attr(element, "class", "classes", "_class")
    if raw is None:
        return []
    return raw.split() if isinstance(raw, str) else list(raw)


@dataclass
class CompoundSelector:
    tag: str | None = None
    ids: list[str] = field(default_factory=list)
    classes: list[str] = field(default_factory=list)
    universal: bool = False


@dataclass
class ParsedSelector:
    parts: list[tuple[str, CompoundSelector]] = field(default_factory=list)


def _specificity(selector: ParsedSelector) -> tuple[int, int, int, int]:
    return (
        0,
        sum(len(compound.ids) for _, compound in selector.parts),
        sum(len(compound.classes) for _, compound in selector.parts),
        sum(1 for _, compound in selector.parts if compound.tag is not None),
    )


def _combinator(token) -> str | None:
    if isinstance(token, WhitespaceToken):
        return " "
    if isinstance(token, LiteralToken) and token.value in (">", "+", "~"):
        return token.value
    return None


def _skip_pseudo(tokens: list, start: int, count: int) -> int:
    index = start
    depth = 0
    while index < count:
        token = tokens[index]
        if isinstance(token, FunctionBlock):
            depth += 1
            index += 1
            continue
        if isinstance(token, LiteralToken):
            if token.value == "(":
                depth += 1
                index += 1
                continue
            if token.value == ")":
                depth -= 1
                if depth < 0:
                    return index + 1
                index += 1
                continue
        if depth == 0 and _combinator(token):
            return index
        index += 1
    return index


def _skip_attribute(tokens: list, start: int, count: int) -> int:
    index = start
    while index < count:
        if isinstance(tokens[index], LiteralToken) and tokens[index].value == "]":
            return index + 1
        index += 1
    return index


def _flush(parts: list[tuple[str, CompoundSelector]], current: CompoundSelector, combinator: str) -> None:
    if current.tag is not None or current.ids or current.classes or current.universal:
        parts.append((combinator, CompoundSelector(
            current.tag,
            list(current.ids),
            list(current.classes),
            current.universal,
        )))


def _parse_selector(tokens: list) -> ParsedSelector | None:
    parts: list[tuple[str, CompoundSelector]] = []
    current = CompoundSelector()
    combinator = ""
    after_explicit = False
    index = 0
    count = len(tokens)
    while index < count:
        token = tokens[index]
        if isinstance(token, WhitespaceToken):
            if after_explicit:
                after_explicit = False
                index += 1
                continue
            if index + 1 < count and _combinator(tokens[index + 1]) in (">", "+", "~"):
                _flush(parts, current, combinator)
                current = CompoundSelector()
                combinator = _combinator(tokens[index + 1]) or " "
                after_explicit = True
                index += 2
                continue
            _flush(parts, current, combinator)
            current = CompoundSelector()
            combinator = " "
            index += 1
            continue
        if isinstance(token, LiteralToken) and token.value in (">", "+", "~"):
            _flush(parts, current, combinator)
            current = CompoundSelector()
            combinator = token.value
            after_explicit = True
            index += 1
            continue
        after_explicit = False
        if isinstance(token, LiteralToken) and token.value == ".":
            index += 1
            if index < count and isinstance(tokens[index], IdentToken):
                current.classes.append(tokens[index].value)
                index += 1
                continue
            return None
        if isinstance(token, LiteralToken) and token.value == "#":
            index += 1
            if index < count and isinstance(tokens[index], (IdentToken, HashToken)):
                current.ids.append(tokens[index].value)
                index += 1
                continue
            return None
        if isinstance(token, LiteralToken) and token.value == ":":
            index = _skip_pseudo(tokens, index + 1, count)
            continue
        if isinstance(token, LiteralToken) and token.value == "[":
            index = _skip_attribute(tokens, index + 1, count)
            continue
        if (isinstance(token, IdentToken) and token.value == "*") or (isinstance(token, LiteralToken) and token.value == "*"):
            current.tag = None
            current.universal = True
            index += 1
            continue
        if isinstance(token, IdentToken):
            current.tag = token.value
            index += 1
            continue
        if isinstance(token, HashToken):
            current.ids.append(token.value)
            index += 1
            continue
        index += 1
    _flush(parts, current, combinator)
    return ParsedSelector(parts) if parts else None


def _parse_selector_list(prelude: list) -> tuple[list[ParsedSelector], list[dict]]:
    groups: list[list] = [[]]
    for token in prelude:
        if isinstance(token, LiteralToken) and token.value == ",":
            groups.append([])
        else:
            groups[-1].append(token)
    parsed: list[ParsedSelector] = []
    unresolved: list[dict] = []
    for group in groups:
        selector = _parse_selector(group)
        if selector is None:
            unresolved.append({
                "selector": "".join(token.serialize() for token in group).strip(),
                "reason": "Selector structure could not be parsed",
            })
        else:
            parsed.append(selector)
    return parsed, unresolved


def _compound_matches(element: dict, compound: CompoundSelector) -> bool:
    if compound.tag is not None:
        tag = _element_attr(element, "tag", "name", "node_name", "tag_name")
        if tag is None or tag.lower() != compound.tag.lower():
            return False
    classes = _element_classes(element)
    if any(value not in classes for value in compound.classes):
        return False
    if compound.ids:
        element_id = _element_attr(element, "id")
        if element_id is None or any(element_id != value for value in compound.ids):
            return False
    return True


def _applies(selector: ParsedSelector, chain: list) -> bool:
    if not selector.parts:
        return False
    last = len(chain) - 1
    if last < 0 or not _compound_matches(chain[last], selector.parts[-1][1]):
        return False
    if len(selector.parts) == 1:
        return True
    part_index = len(selector.parts) - 2
    ancestor_index = last - 1
    while part_index >= 0:
        combinator, compound = selector.parts[part_index]
        if combinator == ">":
            if ancestor_index < 0 or not _compound_matches(chain[ancestor_index], compound):
                return False
            ancestor_index -= 1
            part_index -= 1
        elif combinator in ("", " "):
            found = False
            while ancestor_index >= 0:
                if _compound_matches(chain[ancestor_index], compound):
                    found = True
                    ancestor_index -= 1
                    break
                ancestor_index -= 1
            if not found:
                return False
            part_index -= 1
        elif combinator in ("+", "~"):
            part_index -= 1
        else:
            return False
    return True


def collect_rules(css_files: list) -> tuple[list[dict], list[dict]]:
    """Parse qualified rules and return them with unresolved selectors."""
    rules: list[dict] = []
    unresolved: list[dict] = []
    for css_file in css_files:
        raw = css_file.get("content_bytes", b"")
        source = css_file.get("id", css_file.get("href", "unknown"))
        text = decode_css(raw) if isinstance(raw, bytes) else raw
        for node in parse_stylesheet(text, skip_comments=True, skip_whitespace=True):
            if not isinstance(node, QualifiedRule):
                continue
            parsed, failures = _parse_selector_list(node.prelude)
            for failure in failures:
                failure["source"] = source
                unresolved.append(failure)
            if not parsed:
                continue
            declarations: dict[str, list] = {}
            for declaration in parse_declaration_list(node.content, skip_comments=True):
                if isinstance(declaration, CSSDecl):
                    declarations[declaration.name.lower()] = declaration.value
            rules.append({
                "selectors": parsed,
                "declarations": declarations,
                "specificity": max((_specificity(selector) for selector in parsed), default=(0, 0, 0, 0)),
                "source": source,
            })
    return rules, unresolved


def resolve_font_family(element_chain: list, rules: list[dict]) -> list[str] | None:
    """Resolve the highest-specificity font-family set on the chain's last element."""
    candidates: list[tuple[tuple[int, ...], list[str]]] = []
    for rule in rules:
        for selector in rule["selectors"]:
            if _applies(selector, element_chain):
                declarations = rule["declarations"]
                if "font-family" in declarations:
                    families = parse_font_family_list(declarations["font-family"])
                    if families:
                        candidates.append((rule["specificity"], families))
                break
    if not candidates:
        return None
    candidates.sort(key=lambda item: item[0], reverse=True)
    return candidates[0][1]
