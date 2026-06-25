"""
Font cmap index module.

Reads embedded font files from an EPUB zip container via fontTools, extracts
cmap coverage information (codepoints, IVS records, glyph count, subset
heuristic), and returns a family-indexed dictionary of FontInfo dataclasses.

Usage::

    from font_index import build_font_index, get_font_index

    with zipfile.ZipFile("book.epub") as zf:
        font_files = ["OEBPS/fonts/NotoSerif-Regular.ttf", ...]
        index = build_font_index(zf, font_files)

Cached variant (recommended when fonts are processed multiple times)::

    index = get_font_index(zf, font_files, cache={})
"""

from __future__ import annotations

import io
import logging
from dataclasses import dataclass, field
from typing import Dict, List, Optional
from zipfile import ZipFile

from fontTools.ttLib import TTFont
from fontTools.ttLib.tables._c_m_a_p import CmapSubtable

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Data containers
# ---------------------------------------------------------------------------


@dataclass
class FontInfo:
    """Information about a single embedded font, keyed by family name.

    Attributes:
        family: Normalised font family name (lowercase) as found in the
            ``name`` table (name ID 1). Falls back to the filename stem if
            the name table is missing or empty.
        file_path: Path of the font file *inside* the EPUB zip container.
        codepoints: Set of integer Unicode codepoints covered by the font's
            best cmap (format 4 or 12 subtable).
        ivs_records: List of (base_codepoint, selector_codepoint) tuples
            extracted from cmap format 14, if present.
        glyph_count: Number of glyphs in the font (``maxp.numGlyphs``).
        is_subset: ``True`` if the font is heuristically determined to be a
            subset (small glyph count + no GSUB table).
    """

    family: str
    file_path: str
    codepoints: set = field(default_factory=set)
    ivs_records: list = field(default_factory=list)
    glyph_count: int = 0
    is_subset: bool = False


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

_SUBSET_HARD_LIMIT = 1000
_SUBSET_SOFT_LIMIT = 5000


def _extract_family_name(font: TTFont, file_path: str) -> str:
    """Read the font family name from the ``name`` table (name ID 1).

    Falls back to the filename stem (sans extension) when the name table is
    missing or does not contain name ID 1.
    """
    if font["name"] and font["name"].getName(1, 3, 1, 0x0409):
        name_entry = font["name"].getName(1, 3, 1, 0x0409)
        return name_entry.toUnicode().strip().lower()
    if font["name"] and font["name"].getName(1, 3, 1):
        name_entry = font["name"].getName(1, 3, 1)
        return name_entry.toUnicode().strip().lower()
    # Platform-agnostic fallback: take any name ID 1 record
    for rec in font["name"].names:
        if rec.nameID == 1:
            try:
                return rec.toUnicode().strip().lower()
            except Exception:
                continue
    # Last resort: use the filename stem
    import os.path

    stem, _ = os.path.splitext(os.path.basename(file_path))
    return stem.lower()


def _get_best_cmap(font: TTFont) -> dict:
    """Return the best cmap subtable as a {codepoint: glyphName} dict.

    Delegates to ``font.getBestCmap()`` which prefers, in order:
    format 4 (BMP) and format 12 (full Unicode) subtables.
    """
    return font.getBestCmap()


def _get_cmap_format14(font: TTFont) -> list:
    """Extract variation-sequence (IVS) records from cmap format 14.

    Returns a list of ``(base_codepoint: int, selector_codepoint: int)``
    tuples. Returns an empty list when no format 14 subtable exists.
    """
    ivs_records: list = []
    cmap_table = font.get("cmap")
    if cmap_table is None:
        return ivs_records

    for subtable in cmap_table.tables:
        if subtable.format == 14:
            ivs_records.extend(_extract_format14(subtable))

    return ivs_records


def _extract_format14(subtable: CmapSubtable) -> list:
    """Extract (base, selector) pairs from a cmap format 14 subtable.

    The format-14 subtable maps variation-sequence selectors (UVSes) to
    default-UVS tables (non-standard mappings) or to the default Unicode
    mapping. We only emit records for which the variation-sequence base
    character is *not* the default (i.e. the font provides a non-default
    glyph for that combination).

    Implementation note: ``subtable.uvsDict`` is a dict of
    ``{int: {int: int}}`` where the outer key is the selector, and the
    inner dict maps base characters to either their mapped glyph ID or
    ``None`` (meaning "use default"). We skip ``None`` values since the
    font does not provide a custom glyph for those entries.
    """
    records: list = []
    try:
        # fontTools stores format-14 data in .uvsDict
        for selector, base_map in subtable.uvsDict.items():
            for base, glyph_id in base_map.items():
                if glyph_id is not None:
                    records.append((base, selector))
    except AttributeError:
        # Older fontTools or unexpected attribute layout
        logger.debug("Could not parse format-14 subtable")
    return records


def _detect_subset(font: TTFont, glyph_count: int) -> bool:
    """Heuristic subset detection.

    - Glyph count < ``_SUBSET_HARD_LIMIT`` (1000)  →  definitely a subset.
    - Glyph count < ``_SUBSET_SOFT_LIMIT`` (5000) **and** no GSUB table
      present  →  likely a subset.
    - Otherwise  →  not a subset.
    """
    if glyph_count < _SUBSET_HARD_LIMIT:
        return True
    if glyph_count < _SUBSET_SOFT_LIMIT and font.get("GSUB") is None:
        return True
    return False


# ---------------------------------------------------------------------------
# Font-reading core
# ---------------------------------------------------------------------------


def _read_single_font(zf: ZipFile, file_path: str) -> Optional[FontInfo]:
    """Read one font from the zip and return a populated ``FontInfo``.

    Returns ``None`` if the path does not exist in the archive or the font
    cannot be parsed.
    """
    if file_path not in zf.namelist():
        logger.warning("Font file not found in archive: %s", file_path)
        return None

    try:
        data = zf.read(file_path)
    except KeyError:
        logger.warning("Font file not readable in archive: %s", file_path)
        return None

    try:
        font = TTFont(io.BytesIO(data))
    except Exception as exc:
        logger.warning("Failed to parse font '%s': %s", file_path, exc)
        return None

    family = _extract_family_name(font, file_path)
    cmap = _get_best_cmap(font)
    codepoints = set(cmap.keys())
    ivs = _get_cmap_format14(font)

    glyph_count = 0
    if font.get("maxp"):
        glyph_count = font["maxp"].numGlyphs

    is_subset = _detect_subset(font, glyph_count)

    font.close()

    return FontInfo(
        family=family,
        file_path=file_path,
        codepoints=codepoints,
        ivs_records=ivs,
        glyph_count=glyph_count,
        is_subset=is_subset,
    )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def build_font_index(zf: ZipFile, font_files: list) -> Dict[str, FontInfo]:
    """Build a family-indexed font coverage index from a list of font paths.

    For each path in *font_files* (which must exist inside the opened
    *zf* archive), the font is opened with fontTools, its cmap is read,
    and a :class:`FontInfo` record is produced.

    The returned dict is keyed by **lowercased** family name. If multiple
    font files resolve to the same family name, only the *last* one wins
    (callers should deduplicate or order deliberately).

    Args:
        zf: An already-open ``zipfile.ZipFile`` pointing to an EPUB.
        font_files: List of font paths inside the archive
            (e.g. ``["OEBPS/fonts/NotoSerif-Regular.ttf"]``).

    Returns:
        ``{family_lower: FontInfo, ...}`` mapping.

    Example::

        with zipfile.ZipFile("book.epub") as zf:
            index = build_font_index(zf, ["OEBPS/fonts/MyFont.otf"])
            info = index.get("myfont")
            if info:
                print(f"{hex(ord('A'))} covered? {ord('A') in info.codepoints}")
    """
    index: Dict[str, FontInfo] = {}
    for fpath in font_files:
        info = _read_single_font(zf, fpath)
        if info is not None:
            index[info.family] = info
    return index


def get_font_index(
    zf: ZipFile,
    font_files: list,
    cache: Optional[Dict[str, FontInfo]] = None,
) -> Dict[str, FontInfo]:
    """Cached version of :func:`build_font_index`.

    When *cache* is provided (as a mutable dict), already-indexed font
    files are skipped on subsequent calls. The cache dict itself is
    modified in place and can be reused across calls.

    Args:
        zf: An opened ``zipfile.ZipFile``.
        font_files: List of font paths inside the archive.
        cache: Optional mutable dict for caching. If ``None``, behaves
            identically to :func:`build_font_index`.

    Returns:
        ``{family_lower: FontInfo, ...}`` mapping (same dict as *cache*
        when one was supplied).
    """
    if cache is None:
        return build_font_index(zf, font_files)

    # Deduplicate: only process font files not already cached
    cached_paths = {info.file_path for info in cache.values()}
    to_process = [f for f in font_files if f not in cached_paths]

    if not to_process:
        return cache

    for fpath in to_process:
        info = _read_single_font(zf, fpath)
        if info is not None:
            cache[info.family] = info

    return cache

