"""Character standard-zone classification (标准字区).

Two orthogonal, pure-codepoint axes (no font needed):

1. block_tier(cp)         — which Unicode CJK tier the codepoint sits in.
2. standard_zone(cp, ...) — membership tier in the national standard char
   sets GB2312 / GBK, generated at runtime from Python's stdlib codecs
   (no data file). An optional user-supplied StandardCharset can overlay.
"""

from __future__ import annotations

import functools
import os.path
from dataclasses import dataclass

# Ordered, non-overlapping CJK ranges. First match wins.
_RANGES: list[tuple[str, int, int]] = [
    ("cjk-basic",      0x4E00, 0x9FFF),
    ("cjk-ext-a",      0x3400, 0x4DBF),
    ("cjk-compat",     0xF900, 0xFAFF),
    ("cjk-ext-b-plus", 0x20000, 0x2A6DF),  # Ext B
    ("cjk-ext-b-plus", 0x2A700, 0x2EE5F),  # Ext C–F
    ("cjk-compat",     0x2F800, 0x2FA1F),  # Compat Ideographs Supplement
    ("cjk-ext-b-plus", 0x30000, 0x323AF),  # Ext G–H
]

_PUA: list[tuple[int, int]] = [(0xE000, 0xF8FF), (0xF0000, 0xFFFFD), (0x100000, 0x10FFFD)]
_VS: list[tuple[int, int]] = [(0xFE00, 0xFE0F), (0xE0100, 0xE01EF)]


def block_tier(cp: int) -> str:
    """Classify a codepoint into a CJK tier (or pua/vs/non-cjk)."""
    for lo, hi in _VS:
        if lo <= cp <= hi:
            return "vs"
    for lo, hi in _PUA:
        if lo <= cp <= hi:
            return "pua"
    for name, lo, hi in _RANGES:
        if lo <= cp <= hi:
            return name
    return "non-cjk"


def is_cjk_tier(tier: str) -> bool:
    """True for the hanzi tiers (cjk-basic / ext-a / ext-b-plus / compat)."""
    return tier.startswith("cjk-")


# CJK ideograph ranges to probe when enumerating a codec's coverage.
_SCAN_RANGES: list[tuple[int, int]] = [
    (0x3400, 0x4DBF),   # Ext A
    (0x4E00, 0x9FFF),   # Basic
    (0xF900, 0xFAFF),   # Compat Ideographs
]


@functools.lru_cache(maxsize=1)
def build_standard_charsets() -> dict:
    """Enumerate GB2312 / GBK hanzi code-point sets from stdlib codecs.

    No data file: a codepoint is "in GB2312" iff it encodes via the gb2312
    codec; GBK is a superset. Cached (the scan is a few ms, run once).
    """
    gb2312: set[int] = set()
    gbk: set[int] = set()
    for lo, hi in _SCAN_RANGES:
        for cp in range(lo, hi + 1):
            ch = chr(cp)
            try:
                ch.encode("gb2312")
                gb2312.add(cp)
            except UnicodeEncodeError:
                pass
            try:
                ch.encode("gbk")
                gbk.add(cp)
            except UnicodeEncodeError:
                pass
    return {"gb2312": frozenset(gb2312), "gbk": frozenset(gbk)}


@dataclass
class StandardCharset:
    """Optional user-supplied standard char set (e.g. 通用规范汉字表 8105)."""

    name: str
    codepoints: frozenset

    def contains(self, cp: int) -> bool:
        return cp in self.codepoints


def load_standard_charset(path: str, name: str | None = None) -> StandardCharset:
    """Load an OPTIONAL custom char list from a UTF-8 text file.

    Every non-whitespace character contributes its codepoint; lines whose
    first non-blank char is '#' are comments. Lets a user paste a raw list
    (e.g. the 8105 通用规范汉字表) with no markup, to override/extend GBK.
    """
    cps: set[int] = set()
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            if line.lstrip().startswith("#"):
                continue
            for ch in line:
                if not ch.isspace():
                    cps.add(ord(ch))
    if name is None:
        name = os.path.splitext(os.path.basename(path))[0]
    return StandardCharset(name=name, codepoints=frozenset(cps))


def standard_zone(cp: int, charsets: dict, extra: "StandardCharset | None" = None) -> str | None:
    """Tightest standard zone for a CJK codepoint.

    'gb2312' | 'gbk' | 'out-of-gbk' for hanzi; None for non-hanzi.
    A custom *extra* set promotes a char to at least 'gbk'.
    """
    if not is_cjk_tier(block_tier(cp)):
        return None
    if cp in charsets["gb2312"]:
        return "gb2312"
    if cp in charsets["gbk"] or (extra is not None and extra.contains(cp)):
        return "gbk"
    return "out-of-gbk"


def classify_tier(cp: int, charsets: dict, extra: "StandardCharset | None" = None) -> dict:
    """Return {block_tier, std_zone, is_rare} for a codepoint."""
    tier = block_tier(cp)
    zone = standard_zone(cp, charsets, extra)
    is_rare = (zone == "out-of-gbk") or tier == "pua"
    return {"block_tier": tier, "std_zone": zone, "is_rare": is_rare}
