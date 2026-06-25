"""
Reader profile translator for EPUB font coverage detection.

Maps a *coverage position* (where in the font-family chain a character
is covered) to a *verdict* (ok / risk / fail) according to a named
reader profile.  Profiles are data-driven dictionaries, making it
trivial to add new ones.

Coverage positions
------------------
The coverage detector classifies every rendered character into one of
four positions:

``first-embedded``
    The character is covered by the **first** embedded font in the
    ``font-family`` chain.  This is the safest position.

``later-embedded``
    The character is covered by a **later** (second, third, …) embedded
    font in the chain.  Most browsers handle this fine, but some
    readers (e.g. Kindle) may stop looking after the first embedded
    font and fall back to an incorrect reader-default font.

``only-non-embedded``
    The character is **not** covered by any embedded font and relies
    entirely on a system / reader-default font.  Acceptable on some
    platforms, risky on others.

``none``
    The character is covered by **none** of the fonts in the chain
    (no embedded font covers it and no system fallback is available).
    This is always a failure situation.

Usage::

    >>> from profiles import translate, get_verdicts, PROFILES, PROFILE_DESCRIPTIONS
    >>> translate("later-embedded", "ideal-browser")
    'ok'
    >>> translate("later-embedded", "kindle-pessimistic")
    'fail'
    >>> get_verdicts({"h1-chain": "later-embedded", "body-chain": "first-embedded"}, "ideal-browser")
    {'h1-chain': 'ok', 'body-chain': 'ok'}
"""

from __future__ import annotations

from typing import Dict

# ---------------------------------------------------------------------------
# Profile data
# ---------------------------------------------------------------------------

PROFILES: Dict[str, Dict[str, str]] = {
    "ideal-browser": {
        "first-embedded": "ok",
        "later-embedded": "ok",
        "only-non-embedded": "risk",
        "none": "fail",
    },
    "kindle-pessimistic": {
        "first-embedded": "ok",
        "later-embedded": "fail",
        "only-non-embedded": "risk",
        "none": "fail",
    },
}

PROFILE_DESCRIPTIONS: Dict[str, str] = {
    "ideal-browser": (
        "Standard per-glyph fallback (WebKit/Blink based readers). "
        "Each embedded font in the chain is consulted in order, and if "
        "none matches, the system fallback font is used. This is the "
        "most permissive profile."
    ),
    "kindle-pessimistic": (
        "Kindle conservative: first embedded font = safe; later embedded "
        "fonts may not be reached (Kindle's per-glyph fallback is unreliable); "
        "system fonts = risk (common CJK usually OK, rare/ext-B chars may be missing) "
    ),
}

_VALID_POSITIONS = frozenset({"first-embedded", "later-embedded", "only-non-embedded", "none"})
_VALID_VERDICTS = frozenset({"ok", "risk", "fail"})


def _validate_profile(profile_name: str) -> Dict[str, str]:
    """Look up a profile by name and validate its structure.

    Raises:
        ValueError: If the profile name is unknown or the profile dict is
            malformed.
    """
    profile = PROFILES.get(profile_name)
    if profile is None:
        known = ", ".join(sorted(PROFILES))
        raise ValueError(
            f"Unknown profile: {profile_name!r}. "
            f"Available profiles: {known}"
        )
    return profile


def _validate_position(position: str) -> None:
    """Raise ValueError if *position* is not a recognised coverage position."""
    if position not in _VALID_POSITIONS:
        valid = ", ".join(sorted(_VALID_POSITIONS))
        raise ValueError(
            f"Unknown coverage position: {position!r}. "
            f"Valid positions: {valid}"
        )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def translate(position: str, profile_name: str) -> str:
    """Return the verdict (``ok`` | ``risk`` | ``fail``) for a coverage
    position under a given reader profile.

    Args:
        position: One of ``"first-embedded"``, ``"later-embedded"``,
            ``"only-non-embedded"``, ``"none"``.
        profile_name: Name of a reader profile defined in :data:`PROFILES`.

    Returns:
        One of ``"ok"``, ``"risk"``, ``"fail"``.

    Raises:
        ValueError: If *position* or *profile_name* is unknown.

    Example::

        >>> translate("first-embedded", "ideal-browser")
        'ok'
        >>> translate("only-non-embedded", "kindle-pessimistic")
        'fail'
    """
    _validate_position(position)
    profile = _validate_profile(profile_name)
    return profile[position]


def get_verdicts(positions: Dict[str, str], profile_name: str) -> Dict[str, str]:
    """Translate multiple coverage positions into verdicts for a single profile.

    Args:
        positions: A dict mapping chain identifiers (or any string key) to
            coverage positions, e.g.::

                {"h1-chain": "later-embedded", "body-chain": "first-embedded"}

        profile_name: Name of a reader profile.

    Returns:
        A dict with the same keys as *positions*, each mapped to its
        verdict (``"ok"`` | ``"risk"`` | ``"fail"``).

    Raises:
        ValueError: If any position or *profile_name* is unknown.

    Example::

        >>> positions = {"title": "first-embedded", "body": "none"}
        >>> get_verdicts(positions, "ideal-browser")
        {'title': 'ok', 'body': 'fail'}
    """
    profile = _validate_profile(profile_name)
    results: Dict[str, str] = {}
    for chain_id, position in positions.items():
        _validate_position(position)
        results[chain_id] = profile[position]
    return results


def get_profile_names() -> list:
    """Return sorted list of all available profile names."""
    return sorted(PROFILES.keys())
