"""
CSS Cascade Resolver — EPUB font-family chain resolution.

Parses CSS stylesheets via *tinycss2*, extracts ``@font-face`` declarations,
resolves font-family chains for text runs by matching CSS rules against
ancestor element chains, and produces deduplicated chain lists with
embedded/generic annotations.

Exposed API
-----------
* ``build_font_face_registry(css_files)`` — extract ``@font-face`` into a dict.
* ``resolve_chains(text_runs, css_files, font_face_registry, font_files)`` —
  resolve font-family chains for all text runs.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from typing import Any

from tinycss2 import parse_stylesheet, parse_declaration_list
from tinycss2.ast import (
    AtRule,
    QualifiedRule,
    Declaration as CSSDecl,
    IdentToken,
    StringToken,
    URLToken,
    HashToken,
    LiteralToken,
    WhitespaceToken,
    FunctionBlock,
)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

GENERIC_FAMILIES: frozenset[str] = frozenset({
    'serif', 'sans-serif', 'monospace', 'cursive', 'fantasy',
    'system-ui', 'ui-serif', 'ui-sans-serif', 'ui-monospace', 'ui-rounded',
    'math', 'emoji', 'fangsong',
})

CSS_WIDE_KEYWORDS: frozenset[str] = frozenset({
    'inherit', 'initial', 'unset', 'revert', 'revert-layer',
})


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

@dataclass
class ChainSegment:
    """One entry in a resolved font-family chain.

    Attributes:
        family: Font family name (e.g. ``'Noto Serif'``, ``'serif'``).
        embedded: Whether this font is declared via ``@font-face``.
        file: Path to the font file within the EPUB if embedded, else None.
        generic: Whether this is a CSS generic font family keyword.
    """
    family: str
    embedded: bool = False
    file: str | None = None
    generic: bool = False


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _decode(bytes_: bytes) -> str:
    """Decode CSS bytes to str, stripping a UTF-8 BOM if present."""
    if bytes_[:3] == b'\xef\xbb\xbf':
        return bytes_[3:].decode('utf-8')
    try:
        return bytes_.decode('utf-8')
    except UnicodeDecodeError:
        return bytes_.decode('utf-8', errors='replace')


def _elem_attr(element: dict, *keys: str) -> Any:
    """Get attribute from element dict trying several key names."""
    for k in keys:
        v = element.get(k)
        if v is not None:
            return v
    return None


def _elem_classes(element: dict) -> list[str]:
    """Get class list from element, handling both str and list."""
    raw = _elem_attr(element, 'class', 'classes', '_class')
    if raw is None:
        return []
    if isinstance(raw, str):
        return raw.split()
    return list(raw)


# ===================================================================
# 1. @font-face registry
# ===================================================================

def _collect_font_face_declarations(content: list) -> dict[str, Any]:
    """Parse declarations inside a ``@font-face`` block.

    Returns a dict with optional keys: *family*, *src*, *font-weight*,
    *font-style*, *font-display*, *unicode-range*.
    """
    decls = parse_declaration_list(content, skip_comments=True)
    result: dict[str, Any] = {}
    for d in decls:
        if not isinstance(d, CSSDecl):
            continue
        name = d.name.lower()
        if name == 'font-family':
            families = _parse_font_family_list(d.value)
            if families:
                result['family'] = families[0]
        elif name == 'src':
            result['src'] = _extract_first_url(d.value)
        elif name in ('font-weight', 'font-style', 'font-display', 'unicode-range'):
            result[name] = ''.join(t.serialize() for t in d.value).strip()
    return result


def _extract_first_url(tokens: list) -> str | None:
    """Extract the first ``url(…)`` from a component-value list.

    Handles both ``url(path)`` (unquoted, a ``URLToken``) and
    ``url('path')`` (quoted, a ``FunctionBlock`` with name ``url``).
    """
    for t in tokens:
        if isinstance(t, URLToken):
            return t.value
        if isinstance(t, FunctionBlock) and t.name == 'url':
            for arg in t.arguments:
                if isinstance(arg, (StringToken, URLToken)):
                    return arg.value
                if isinstance(arg, IdentToken):
                    return arg.value
            return None
    return None


def build_font_face_registry(css_files: list) -> dict:
    """Parse ``@font-face`` rules from all CSS files.

    Args:
        css_files:
            List of dicts, each with ``id``, ``href``, ``content_bytes``.

    Returns:
        Dict mapping ``family_name.lower()`` → ``{src, weight, style}``.

        If multiple ``@font-face`` rules declare the *same* family name
        (e.g. different weights), the value is a *list* of such dicts so
        the caller can inspect all variants.
    """
    registry: dict[str, Any] = {}

    for css_file in css_files:
        raw = css_file.get('content_bytes', b'')
        text = _decode(raw) if isinstance(raw, bytes) else raw

        rules = parse_stylesheet(text, skip_comments=True, skip_whitespace=True)

        for rule in rules:
            if not isinstance(rule, AtRule) or rule.at_keyword.lower() != 'font-face':
                continue

            decls = _collect_font_face_declarations(rule.content)
            family = decls.get('family')
            if not family:
                continue

            key = family.lower()
            entry = {
                'src': decls.get('src', ''),
                'weight': decls.get('font-weight', 'normal'),
                'style': decls.get('font-style', 'normal'),
            }

            if key not in registry:
                registry[key] = entry
            else:
                existing = registry[key]
                if isinstance(existing, list):
                    existing.append(entry)
                else:
                    registry[key] = [existing, entry]

    return registry


# ===================================================================
# 2. Font-family value parsing
# ===================================================================

def _parse_font_family_list(tokens: list) -> list[str]:
    """Parse a ``font-family`` declaration value into an ordered name list.

    Handles comma-separated quoted strings, unquoted identifiers, and
    multi-word unquoted names (invalid per CSS but tolerated in the wild).

    CSS-wide keywords (``inherit``, ``initial``, …) are intentionally
    *not* treated as family names and are filtered out.

    Returns:
        Ordered list of font-family names.  Empty if the value is empty
        or contains only CSS-wide keywords.
    """
    families: list[str] = []
    current: list[str] = []

    def _flush() -> None:
        nonlocal current
        name = ''.join(current).strip()
        if name and name.lower() not in CSS_WIDE_KEYWORDS:
            families.append(name)
        current = []

    for token in tokens:
        if isinstance(token, WhitespaceToken):
            if current:
                current.append(' ')
        elif isinstance(token, LiteralToken) and token.value == ',':
            _flush()
        elif isinstance(token, StringToken):
            _flush()
            val = token.value.strip()
            if val and val.lower() not in CSS_WIDE_KEYWORDS:
                families.append(val)
        elif isinstance(token, IdentToken):
            current.append(token.value)
        # FunctionBlock / other token types → silently skip

    _flush()
    return families


# ===================================================================
# 3. Selector parsing
# ===================================================================

@dataclass
class _CompoundSelector:
    """A compound CSS selector (e.g. ``div.note#main``)."""
    tag: str | None = None
    ids: list[str] = field(default_factory=list)
    classes: list[str] = field(default_factory=list)


@dataclass
class _ParsedSelector:
    """One selector from a comma-separated group.

    ``parts`` is a list of ``(combinator, compound)`` pairs; the first
    combinator is always the empty string.

    Combinators:
        ``''``  — initial compound
        ``' '`` — descendant
        ``'>'`` — child
        ``'+'`` / ``'~'`` — sibling (parsed but not matched; kept for
                            specificity estimation)
    """
    parts: list[tuple[str, _CompoundSelector]] = field(default_factory=list)


# -- specificity ------------------------------------------------------------

def _specificity(s: _ParsedSelector) -> tuple[int, int, int, int]:
    """CSS specificity as ``(inline, id, class, element)``."""
    return (
        0,
        sum(len(c.ids) for _, c in s.parts),
        sum(len(c.classes) for _, c in s.parts),
        sum(1 for _, c in s.parts if c.tag is not None),
    )


# -- combinator detection ---------------------------------------------------

def _is_combinator(tok) -> str | None:
    """Return combinator type or None."""
    if isinstance(tok, WhitespaceToken):
        return ' '
    if isinstance(tok, LiteralToken) and tok.value in ('>', '+', '~'):
        return tok.value
    return None


# -- region-skipping helpers ------------------------------------------------

def _skip_pseudo_region(tokens: list, start: int, n: int) -> int:
    """Skip tokens belonging to a pseudo-class/element (``:hover``, ``:nth-child(…)``)."""
    i = start
    depth = 0
    while i < n:
        t = tokens[i]
        if isinstance(t, FunctionBlock):
            depth += 1
            i += 1
            continue
        if isinstance(t, LiteralToken):
            if t.value == '(':
                depth += 1
                i += 1
                continue
            if t.value == ')':
                depth -= 1
                if depth < 0:
                    return i + 1
                i += 1
                continue
        if depth == 0 and _is_combinator(t):
            return i
        i += 1
    return i


def _skip_to_matching_bracket(tokens: list, start: int, n: int) -> int:
    """Skip tokens until a closing ``]``."""
    i = start
    while i < n:
        if isinstance(tokens[i], LiteralToken) and tokens[i].value == ']':
            return i + 1
        i += 1
    return i


# -- single-selector parser -------------------------------------------------

def _parse_single_selector(tokens: list) -> _ParsedSelector | None:
    """Parse one non-comma-separated selector.

    Supported patterns (others are skipped):
        ``div`` / ``.cls`` / ``#id``
        ``div.cls`` / ``div#id`` / ``div.cls#id``
        ``div p`` / ``div > p``  (descendant / child)
        ``*`` (universal)
        Grouped selectors (``,``) handled at a higher level.

    Pseudo-classes and attribute selectors are silently consumed but do
    **not** participate in matching — this sacrifices precision for
    simplicity but avoids false-positive matches.

    Returns ``None`` when the selector text is structurally unparseable.
    """
    parts: list[tuple[str, _CompoundSelector]] = []
    cur = _CompoundSelector()
    combinator = ''
    after_explicit_combinator = False  # tracks whether the last combinator was >/+/~
    i = 0
    n = len(tokens)

    while i < n:
        tok = tokens[i]

        # -- whitespace / combinator ------------------------------------
        if isinstance(tok, WhitespaceToken):
            if after_explicit_combinator:
                # Whitespace immediately after >, +, ~: just consume, do NOT
                # treat as a descendant combinator separator.
                after_explicit_combinator = False
                i += 1
                continue
            if i + 1 < n:
                nx = _is_combinator(tokens[i + 1])
                if nx in ('>', '+', '~'):
                    _flush(parts, cur, combinator)
                    cur = _CompoundSelector()
                    combinator = nx
                    after_explicit_combinator = True
                    i += 2
                    continue
            _flush(parts, cur, combinator)
            cur = _CompoundSelector()
            combinator = ' '
            i += 1
            continue

        if isinstance(tok, LiteralToken) and tok.value in ('>', '+', '~'):
            _flush(parts, cur, combinator)
            cur = _CompoundSelector()
            combinator = tok.value
            after_explicit_combinator = True
            i += 1
            continue

        # Any non-combinator, non-whitespace token resets the flag.
        after_explicit_combinator = False

        # -- dot → class ------------------------------------------------
        if isinstance(tok, LiteralToken) and tok.value == '.':
            i += 1
            if i < n and isinstance(tokens[i], IdentToken):
                cur.classes.append(tokens[i].value)
                i += 1
                continue
            return None

        # -- hash → id selector -----------------------------------------
        if isinstance(tok, LiteralToken) and tok.value == '#':
            i += 1
            if i < n and isinstance(tokens[i], IdentToken):
                cur.ids.append(tokens[i].value)
                i += 1
                continue
            if i < n and isinstance(tokens[i], HashToken):
                cur.ids.append(tokens[i].value)
                i += 1
                continue
            return None

        # -- pseudo-class / pseudo-element → skip -----------------------
        if isinstance(tok, LiteralToken) and tok.value == ':':
            i += 1
            i = _skip_pseudo_region(tokens, i, n)
            continue

        # -- attribute selector → skip ----------------------------------
        if isinstance(tok, LiteralToken) and tok.value == '[':
            i += 1
            i = _skip_to_matching_bracket(tokens, i, n)
            continue

        # -- universal * -------------------------------------------------
        if isinstance(tok, IdentToken) and tok.value == '*':
            cur.tag = None
            i += 1
            continue
        if isinstance(tok, LiteralToken) and tok.value == '*':
            cur.tag = None
            i += 1
            continue

        # -- plain ident → tag name -------------------------------------
        if isinstance(tok, IdentToken):
            cur.tag = tok.value
            i += 1
            continue

        # -- HashToken (e.g. ``#main`` tokenized as a single token) -----
        if isinstance(tok, HashToken):
            cur.ids.append(tok.value)
            i += 1
            continue

        # -- anything else → skip forward one token
        i += 1

    _flush(parts, cur, combinator)
    return _ParsedSelector(parts=parts) if parts else None


def _flush(
    parts: list[tuple[str, _CompoundSelector]],
    cur: _CompoundSelector,
    combinator: str,
) -> None:
    """Append a non-empty compound selector to *parts*."""
    if cur.tag is not None or cur.ids or cur.classes:
        parts.append((
            combinator,
            _CompoundSelector(tag=cur.tag, ids=list(cur.ids), classes=list(cur.classes)),
        ))


# -- comma-separated selector list -----------------------------------------

def _parse_selector_list(prelude: list) -> tuple[list[_ParsedSelector], list[dict]]:
    """Parse a QualifiedRule prelude into selectors.

    Returns:
        ``(parsed_selectors, unresolved_entries)``.
        Each unresolved entry has ``selector`` and ``reason``; ``source``
        is added later by the caller.
    """
    parsed: list[_ParsedSelector] = []
    unresolved: list[dict] = []

    groups: list[list] = [[]]
    for t in prelude:
        if isinstance(t, LiteralToken) and t.value == ',':
            groups.append([])
        else:
            groups[-1].append(t)

    for group in groups:
        sel = _parse_single_selector(group)
        if sel is not None:
            parsed.append(sel)
        else:
            text = ''.join(t.serialize() for t in group)
            unresolved.append({
                'selector': text.strip(),
                'reason': 'Selector structure could not be parsed',
            })

    return parsed, unresolved


# ===================================================================
# 4. Selector matching vs element chains
# ===================================================================

def _compound_matches(element: dict, compound: _CompoundSelector) -> bool:
    """Check whether *element* matches a compound selector."""
    if compound.tag is not None:
        tag = _elem_attr(element, 'tag', 'name', 'node_name', 'tag_name')
        if tag is None or tag.lower() != compound.tag.lower():
            return False

    if compound.classes:
        eclasses = _elem_classes(element)
        for cls in compound.classes:
            if cls not in eclasses:
                return False

    if compound.ids:
        eid = _elem_attr(element, 'id')
        if eid is None:
            return False
        for iid in compound.ids:
            if eid != iid:
                return False

    return True


def _selector_matches_ancestors(sel: _ParsedSelector, chain: list) -> bool:
    """Check whether *sel* matches any element in the ancestor *chain*.

    The rightmost compound of *sel* must match an element in *chain*.
    Preceding compounds are matched against ancestors with their
    respective combinators (descendant ``' '``, child ``'>'``).
    Returns ``True`` as soon as one position satisfies the full selector.
    """
    if not sel.parts:
        return False

    for idx in range(len(chain) - 1, -1, -1):
        if not _compound_matches(chain[idx], sel.parts[-1][1]):
            continue

        if len(sel.parts) == 1:
            return True

        pi = len(sel.parts) - 2
        ai = idx - 1

        while pi >= 0:
            comb, comp = sel.parts[pi]

            if comb == '>':
                if ai < 0:
                    break
                if not _compound_matches(chain[ai], comp):
                    break
                ai -= 1
                pi -= 1

            elif comb in ('', ' '):
                # '' is the initial compound (no combinator) — for matching
                # purposes it behaves like a descendant combinator:
                # find any ancestor that matches.
                found = False
                while ai >= 0:
                    if _compound_matches(chain[ai], comp):
                        found = True
                        ai -= 1
                        break
                    ai -= 1
                if not found:
                    break
                pi -= 1

            elif comb in ('+', '~'):
                pi -= 1
                continue

            else:
                break

        if pi < 0:
            return True

    return False


# ===================================================================
# 5. Inline-style parsing
# ===================================================================

def _parse_inline_declarations(style: str) -> dict[str, list]:
    """Parse an inline ``style`` attribute into ``{property: [tokens]}``."""
    if not style or not style.strip():
        return {}
    try:
        decls = parse_declaration_list(style, skip_comments=True)
    except Exception:
        return {}
    result: dict[str, list] = {}
    for d in decls:
        if isinstance(d, CSSDecl):
            result[d.name.lower()] = d.value
    return result


# ===================================================================
# 6. Rule collection & per-element font-family resolution
# ===================================================================

def _collect_rules(css_files: list) -> tuple[list[dict], list[dict]]:
    """Parse all qualified rules from CSS files.

    Returns:
        ``(rules, unresolved)``.
        Each rule dict: ``{selectors, declarations, specificity, source}``.
    """
    rules: list[dict] = []
    unresolved: list[dict] = []

    for css_file in css_files:
        raw = css_file.get('content_bytes', b'')
        source = css_file.get('id', css_file.get('href', 'unknown'))
        text = _decode(raw) if isinstance(raw, bytes) else raw

        stylesheet = parse_stylesheet(text, skip_comments=True, skip_whitespace=True)

        for node in stylesheet:
            if not isinstance(node, QualifiedRule):
                continue

            parsed, fails = _parse_selector_list(node.prelude)
            for f in fails:
                f['source'] = source
                unresolved.append(f)

            if not parsed:
                continue

            decls = parse_declaration_list(node.content, skip_comments=True)
            declarations: dict[str, list] = {}
            for d in decls:
                if isinstance(d, CSSDecl):
                    declarations[d.name.lower()] = d.value

            max_spec = max(
                (_specificity(s) for s in parsed),
                default=(0, 0, 0, 0),
            )

            rules.append({
                'selectors': parsed,
                'declarations': declarations,
                'specificity': max_spec,
                'source': source,
            })

    return rules, unresolved


def _resolve_font_family(
    element: dict,
    element_chain: list,
    rules: list[dict],
) -> list[str] | None:
    """Return the font-family list for *element* given matching CSS rules.

    The element's full ancestor path is needed for descendant/child
    combinator matching.  Returns ``None`` when no rule provides a
    ``font-family``.

    When multiple rules match, the highest-specificity rule wins.
    """
    candidates: list[tuple[tuple[int, ...], list[str]]] = []

    for rule in rules:
        for sel in rule['selectors']:
            if _selector_matches_ancestors(sel, element_chain):
                decls = rule['declarations']
                if 'font-family' in decls:
                    families = _parse_font_family_list(decls['font-family'])
                    if families:
                        candidates.append((rule['specificity'], families))
                break  # one matching selector per rule is enough

    if not candidates:
        return None

    candidates.sort(key=lambda x: x[0], reverse=True)
    return candidates[0][1]


def _chain_hash(segments: list[ChainSegment]) -> str:
    """Short hex hash for deduplicating identical chains."""
    parts = '\x00'.join(
        f'{s.family}\x1f{s.embedded}\x1f{s.file or ""}\x1f{s.generic}'
        for s in segments
    )
    return hashlib.sha256(parts.encode()).hexdigest()[:16]


# ===================================================================
# Public API
# ===================================================================

def resolve_chains(
    text_runs: list,
    css_files: list,
    font_face_registry: dict,
    font_files: list,
) -> tuple:
    """Resolve font-family chains for a list of text runs.

    For each run:

    1. Check the inline ``style`` attribute for ``font-family``.
    2. Walk the ancestor chain (closest element to text → root), matching
       CSS rules via :func:`_selector_matches_ancestors`.  The first
       ancestor with a matching ``font-family`` declaration wins.
    3. Fall back to ``'serif'`` when no rule matches any ancestor.
    4. Build a ``[ChainSegment, …]`` list, annotating embedded / generic.
    5. Deduplicate identical chains (same hash → same ``chain_id``).

    Args:
        text_runs:
            List of dicts representing one text node each.  Each dict has:

            - ``ancestor_chain`` (*list*): element dicts from root to the
              text node's parent element.  Each element dict has at least
              a ``'tag'`` key (and optionally ``'id'``, ``'class'``).
            - ``inline_style`` (*str*, optional): inline CSS string.

        css_files:
            List of CSS dicts with ``id``, ``href``, ``content_bytes``.
        font_face_registry:
            Output of :func:`build_font_face_registry`.
        font_files:
            List of font file paths (strings) known to exist in the EPUB.

    Returns:
        Tuple of ``(chains, unresolved, run_chains)``:

        - **chains**: ``{chain_id: [ChainSegment, …]}`` — deduplicated
          font-family chain lists.
        - **unresolved**: list of dicts with ``selector``, ``reason``,
          ``source`` for selectors that could not be parsed.
        - **run_chains**: ``{run_index: chain_id}`` mapping each input
          run to its resolved chain ID.
    """
    rules, unresolved_selectors = _collect_rules(css_files)

    # Flatten registry: lists → first entry
    reg: dict[str, dict] = {}
    for k, v in font_face_registry.items():
        reg[k] = v[0] if isinstance(v, list) else v

    font_set: set[str] = set(font_files or [])

    chains: dict[str, list[ChainSegment]] = {}
    run_chains: dict[int, str] = {}

    for run_idx, run in enumerate(text_runs):
        ancestors: list = run.get('ancestor_chain', [])
        inline: str = run.get('inline_style', '')

        families: list[str] | None = None

        # 1. Inline style (highest priority)
        if inline:
            idecls = _parse_inline_declarations(inline)
            if 'font-family' in idecls:
                families = _parse_font_family_list(idecls['font-family'])

        # 2. Walk ancestor chain (closest to text → root)
        if families is None:
            for idx in range(len(ancestors) - 1, -1, -1):
                elem = ancestors[idx]
                sub_chain = ancestors[: idx + 1]
                result = _resolve_font_family(elem, sub_chain, rules)
                if result is not None:
                    families = result
                    break

        # 3. Default fallback
        if families is None:
            families = ['serif']

        # 4. Build ChainSegment list
        segments: list[ChainSegment] = []
        for fname in families:
            lower = fname.lower()
            generic = lower in GENERIC_FAMILIES
            embedded = lower in reg

            fpath: str | None = None
            if embedded:
                src = reg[lower].get('src', '') or ''
                if src in font_set:
                    fpath = src
                elif src:
                    for ff in font_set:
                        if ff.endswith(src) or src.endswith(ff):
                            fpath = ff
                            break
                if fpath is None and src:
                    fpath = src  # keep URL as hint even if unmatchable

            segments.append(ChainSegment(
                family=fname,
                embedded=embedded,
                file=fpath,
                generic=generic,
            ))

        # 5. Deduplicate
        cid = _chain_hash(segments)
        if cid not in chains:
            chains[cid] = segments
        run_chains[run_idx] = cid

    return chains, unresolved_selectors, run_chains
