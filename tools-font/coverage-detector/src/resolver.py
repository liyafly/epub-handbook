"""Resolve CSS font-family chains for harvested EPUB text runs.

Selector/cascade parsing lives in :mod:`src.css_selectors`; font-family and
``@font-face`` parsing lives in :mod:`src.css_fonts`.  This module preserves
the original public API: ``ChainSegment``, ``build_font_face_registry`` and
``resolve_chains``.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass

from tinycss2 import parse_declaration_list
from tinycss2.ast import Declaration as CSSDecl

from .css_fonts import build_font_face_registry, parse_font_family_list
from .css_selectors import collect_rules, resolve_font_family


GENERIC_FAMILIES: frozenset[str] = frozenset({
    "serif", "sans-serif", "monospace", "cursive", "fantasy",
    "system-ui", "ui-serif", "ui-sans-serif", "ui-monospace", "ui-rounded",
    "math", "emoji", "fangsong",
})


@dataclass
class ChainSegment:
    family: str
    embedded: bool = False
    file: str | None = None
    generic: bool = False
    defaulted: bool = False
    system_ref: bool = False


def _inline_declarations(style: str) -> dict[str, list]:
    if not style or not style.strip():
        return {}
    try:
        declarations = parse_declaration_list(style, skip_comments=True)
    except Exception:
        return {}
    return {
        declaration.name.lower(): declaration.value
        for declaration in declarations
        if isinstance(declaration, CSSDecl)
    }


def _chain_hash(segments: list[ChainSegment]) -> str:
    parts = "\x00".join(
        f'{segment.family}\x1f{segment.embedded}\x1f{segment.file or ""}\x1f{segment.generic}\x1f{segment.defaulted}\x1f{segment.system_ref}'
        for segment in segments
    )
    return hashlib.sha256(parts.encode()).hexdigest()[:16]


def resolve_chains(
    text_runs: list,
    css_files: list,
    font_face_registry: dict,
    font_files: list,
) -> tuple:
    """Return deduplicated chains, unresolved selectors, and run mappings."""
    rules, unresolved_selectors = collect_rules(css_files)
    registry = {
        key: value[0] if isinstance(value, list) else value
        for key, value in font_face_registry.items()
    }
    font_set = set(font_files or [])
    chains: dict[str, list[ChainSegment]] = {}
    run_chains: dict[int, str] = {}

    for run_index, run in enumerate(text_runs):
        ancestors = run.get("ancestor_chain", [])
        inline = run.get("inline_style", "")
        families: list[str] | None = None
        if inline:
            declarations = _inline_declarations(inline)
            if "font-family" in declarations:
                families = parse_font_family_list(declarations["font-family"])
        if families is None:
            for index in range(len(ancestors) - 1, -1, -1):
                result = resolve_font_family(ancestors[:index + 1], rules)
                if result is not None:
                    families = result
                    break
        defaulted = families is None
        if families is None:
            families = ["serif"]

        segments: list[ChainSegment] = []
        for family in families:
            lower = family.lower()
            embedded = False
            font_path: str | None = None
            system_ref = False
            if lower in registry:
                entry = registry[lower]
                source = (entry.get("src", "") or "").strip()
                if source:
                    if source in font_set:
                        embedded = True
                        font_path = source
                    else:
                        for candidate in font_set:
                            if candidate.endswith(source) or source.endswith(candidate) or candidate.endswith("/" + source.split("/")[-1]):
                                embedded = True
                                font_path = candidate
                                break
                if not embedded:
                    if bool(entry.get("system_ref", False)):
                        system_ref = True
                    elif source:
                        font_path = source
            segments.append(ChainSegment(
                family=family,
                embedded=embedded,
                file=font_path,
                generic=lower in GENERIC_FAMILIES,
                defaulted=defaulted,
                system_ref=system_ref,
            ))
        chain_id = _chain_hash(segments)
        chains.setdefault(chain_id, segments)
        run_chains[run_index] = chain_id
    return chains, unresolved_selectors, run_chains
