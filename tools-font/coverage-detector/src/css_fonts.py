"""CSS font-family value and @font-face parsing."""

from __future__ import annotations

from typing import Any

from tinycss2 import parse_declaration_list, parse_stylesheet
from tinycss2.ast import AtRule, Declaration as CSSDecl, FunctionBlock, IdentToken, LiteralToken, StringToken, URLToken, WhitespaceToken


CSS_WIDE_KEYWORDS: frozenset[str] = frozenset({
    "inherit", "initial", "unset", "revert", "revert-layer",
})


def decode_css(value: bytes) -> str:
    """Decode CSS bytes, stripping a UTF-8 BOM when present."""
    if value[:3] == b"\xef\xbb\xbf":
        return value[3:].decode("utf-8")
    try:
        return value.decode("utf-8")
    except UnicodeDecodeError:
        return value.decode("utf-8", errors="replace")


def parse_font_family_list(tokens: list) -> list[str]:
    """Parse an ordered comma-separated font-family value."""
    families: list[str] = []
    current: list[str] = []

    def flush() -> None:
        nonlocal current
        name = "".join(current).strip()
        if name and name.lower() not in CSS_WIDE_KEYWORDS:
            families.append(name)
        current = []

    for token in tokens:
        if isinstance(token, WhitespaceToken):
            if current:
                current.append(" ")
        elif isinstance(token, LiteralToken) and token.value == ",":
            flush()
        elif isinstance(token, StringToken):
            flush()
            value = token.value.strip()
            if value and value.lower() not in CSS_WIDE_KEYWORDS:
                families.append(value)
        elif isinstance(token, IdentToken):
            current.append(token.value)
    flush()
    return families


def _extract_first_url(tokens: list) -> str | None:
    for token in tokens:
        if isinstance(token, URLToken):
            return token.value
        if isinstance(token, FunctionBlock) and token.name == "url":
            for argument in token.arguments:
                if isinstance(argument, (StringToken, URLToken, IdentToken)):
                    return argument.value
            return None
    return None


def _font_face_declarations(content: list) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for declaration in parse_declaration_list(content, skip_comments=True):
        if not isinstance(declaration, CSSDecl):
            continue
        name = declaration.name.lower()
        if name == "font-family":
            families = parse_font_family_list(declaration.value)
            if families:
                result["family"] = families[0]
        elif name == "src":
            result["src"] = _extract_first_url(declaration.value)
            source = "".join(token.serialize() for token in declaration.value).lower()
            result["system_ref"] = "res://" in source or "file://" in source or "local(" in source
        elif name in {"font-weight", "font-style", "font-display", "unicode-range"}:
            result[name] = "".join(token.serialize() for token in declaration.value).strip()
    return result


def build_font_face_registry(css_files: list) -> dict:
    """Return family-name to @font-face source/weight/style records."""
    registry: dict[str, Any] = {}
    for css_file in css_files:
        raw = css_file.get("content_bytes", b"")
        text = decode_css(raw) if isinstance(raw, bytes) else raw
        for rule in parse_stylesheet(text, skip_comments=True, skip_whitespace=True):
            if not isinstance(rule, AtRule) or rule.at_keyword.lower() != "font-face":
                continue
            declarations = _font_face_declarations(rule.content)
            family = declarations.get("family")
            if not family:
                continue
            entry = {
                "src": declarations.get("src", ""),
                "weight": declarations.get("font-weight", "normal"),
                "style": declarations.get("font-style", "normal"),
                "system_ref": declarations.get("system_ref", False),
            }
            key = family.lower()
            if key not in registry:
                registry[key] = entry
            elif isinstance(registry[key], list):
                registry[key].append(entry)
            else:
                registry[key] = [registry[key], entry]
    return registry
