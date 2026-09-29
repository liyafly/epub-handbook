"""
Coverage Classifier — classify every unique character by font coverage.

For each (character, resolved chain) pair, determines the coverage
*position* (which font in the chain provides it, if any) and a *cause*
when ideal coverage is missing.

Exposed API
-----------
* ``classify(chars, font_index, chains, run_chains)`` — produce a list of
  ``CoverageResult``-like dicts sorted by codepoint.
"""

from __future__ import annotations

from dataclasses import dataclass, field

# ---------------------------------------------------------------------------
# Constants — Unicode ranges
# ---------------------------------------------------------------------------

PUA_RANGES: list[tuple[int, int]] = [
    (0xE000, 0xF8FF),       # BMP Private Use Area
    (0xF0000, 0xFFFFF),     # Supplementary PUA-A
    (0x100000, 0x10FFFF),   # Supplementary PUA-B
]

VS_RANGES: list[tuple[int, int]] = [
    (0xFE00, 0xFE0F),       # Standardized Variation Selectors
    (0xE0100, 0xE01EF),     # Variation Selectors Supplement
]


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

@dataclass
class CoverageResult:
    """Coverage information for one unique character.

    Attributes:
        char: The character as a Python string.
        cp: Codepoint string in ``U+XXXX`` or ``U+XXXXXX`` format.
        count: Total number of occurrences across all text runs.
        chains: List of chain_ids this character appears in.
        coverage:
            ``{chain_id: {position, covered_by, cause}}``.
        flags: List of flag strings (``"pua"``, ``"ivs"``).
        occurrences:
            Representative occurrence locations
            ``[{file, node_path, offset, context}, …]``.
    """
    char: str
    cp: str
    count: int
    chains: list = field(default_factory=list)
    coverage: dict = field(default_factory=dict)
    flags: list = field(default_factory=list)
    occurrences: list = field(default_factory=list)


# ---------------------------------------------------------------------------
# Flag helpers
# ---------------------------------------------------------------------------

def _is_pua(codepoint: int) -> bool:
    """Return True if *codepoint* falls in a Private Use Area."""
    for lo, hi in PUA_RANGES:
        if lo <= codepoint <= hi:
            return True
    return False


def _is_vs(codepoint: int) -> bool:
    """Return True if *codepoint* is a Variation Selector."""
    for lo, hi in VS_RANGES:
        if lo <= codepoint <= hi:
            return True
    return False


def _cp_string(cp: int) -> str:
    """Format codepoint as ``U+XXXX`` (BMP) or ``U+XXXXX`` / ``U+XXXXXX`` (supplementary)."""
    if cp < 0x10000:
        return f'U+{cp:04X}'
    return f'U+{cp:X}'


# ---------------------------------------------------------------------------
# Coverage analysis for a single (codepoint, chain) pair
# ---------------------------------------------------------------------------

def _analyze_chain_coverage(
    codepoint: int,
    chain: list,
    font_index: dict,
) -> dict:
    """Determine coverage status for *codepoint* within *chain*.

    Args:
        codepoint: Integer Unicode codepoint.
        chain: List of ``ChainSegment`` objects (from resolver).
        font_index:
            ``{font_file_path: {cmap: set[int], subset: bool}}``
            mapping font files to their Unicode coverage and subset flag.

    Returns:
        ``{'position': str, 'covered_by': str | None, 'cause': str | None}``
    """
    embedded_segments = [s for s in chain if s.embedded]
    non_embedded_segments = [s for s in chain if not s.embedded]

    position: str = 'none'
    covered_by: str | None = None
    cause: str | None = None
    subset_missing: bool = False

    # Walk embedded fonts in order
    for seg in embedded_segments:
        file_path = seg.file
        if not file_path:
            continue

        font_data = font_index.get(file_path, {})
        cmap: set[int] = font_data.get('cmap', set())
        is_subset: bool = font_data.get('subset', False)

        if codepoint in cmap:
            if covered_by is None:
                position = 'first-embedded'
            else:
                position = 'later-embedded'
            covered_by = seg.family
            break
        else:
            if is_subset:
                subset_missing = True

    if covered_by is not None:
        # Found in at least one embedded font
        if position == 'later-embedded':
            # First embedded font did NOT have it; a later one does.
            # Cause: the first font failed to cover it — either because it
            # is a subset that cut it, or it genuinely doesn't have it.
            cause = 'subset-cut' if subset_missing else 'fallback-not-reached'

    else:
        # No embedded font has this codepoint
        if non_embedded_segments:
            position = 'only-non-embedded'
            # cause stays None — non-embedded fonts might cover it
        else:
            position = 'none'
            cause = 'subset-cut' if subset_missing else 'true-missing'

    return {
        'position': position,
        'covered_by': covered_by,
        'cause': cause,
    }


# ---------------------------------------------------------------------------
# Context helper
# ---------------------------------------------------------------------------

def _get_context(text: str, offset: int, radius: int = 20) -> str:
    """Extract a short context snippet around *offset* in *text*."""
    start = max(0, offset - radius)
    end = min(len(text), offset + radius + 1)
    ctx = text[start:end]
    if start > 0:
        ctx = '…' + ctx
    if end < len(text):
        ctx = ctx + '…'
    return ctx


# ===================================================================
# Public API
# ===================================================================

def classify(
    chars: list,
    font_index: dict,
    chains: dict,
    run_chains: dict,
) -> list:
    """Classify every unique character by font coverage.

    For each unique character across all text runs, determines which
    font-family chains the character appears in, and for each chain
    evaluates the coverage *position* and *cause*.

    Args:
        chars:
            Character inventory — a list of dicts, one per unique
            character.  Each dict has:

            - ``char`` (*str*): the character itself.
            - ``count`` (*int*): total occurrences.
            - ``runs`` (*list[int]*): indices of the text runs that
              contain this character.
            - ``occurrences`` (*list[dict]*): representative occurrence
              locations, each with ``file``, ``node_path``, ``offset``,
              ``context``.

        font_index:
            ``{font_file_path: {cmap: set[int], subset: bool}}``
            mapping each embedded font file to its cmap coverage and
            subset flag.  Use ``None`` for fonts not found.
        chains:
            ``{chain_id: [ChainSegment, …]}`` — output from
            :func:`resolver.resolve_chains`.
        run_chains:
            ``{run_index: chain_id}`` — also from
            :func:`resolver.resolve_chains`.

    Returns:
        List of :class:`CoverageResult`-like dicts sorted by codepoint.
    """
    results: list[CoverageResult] = []

    for entry in chars:
        ch: str = entry['char']
        cp_int: int = ord(ch)
        cp_str: str = _cp_string(cp_int)
        count: int = entry.get('count', 0)
        occurrences: list = entry.get('occurrences', [])

        # Determine which chain_ids this character appears in
        run_indices: list[int] = entry.get('runs', [])
        chain_ids: set[str] = set()
        for ri in run_indices:
            cid = run_chains.get(ri)
            if cid is not None:
                chain_ids.add(cid)

        # Per-chain coverage analysis
        coverage: dict[str, dict] = {}
        for cid in sorted(chain_ids):
            chain = chains.get(cid)
            if not chain:
                continue
            coverage[cid] = _analyze_chain_coverage(cp_int, chain, font_index)

        # Flags
        flags: list[str] = []
        if _is_pua(cp_int):
            flags.append('pua')
        if _is_vs(cp_int):
            flags.append('ivs')

        results.append(CoverageResult(
            char=ch,
            cp=cp_str,
            count=count,
            chains=sorted(chain_ids),
            coverage=coverage,
            flags=flags,
            occurrences=occurrences,
        ))

    # Sort by codepoint
    results.sort(key=lambda r: int(r.cp.split('+')[1], 16))
    return results
