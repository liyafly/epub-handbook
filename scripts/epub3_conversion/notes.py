"""Plain, Sigil, and Duokan note migration boundary."""

from __future__ import annotations

from . import core


def convert_plain_notes(text: str, note_href: str) -> tuple[str, int, int]:
  return core.convert_plain_notes(text, note_href)


def convert_sigil_legacy_notes(text: str, note_href: str) -> tuple[str, int, int]:
  return core.convert_sigil_legacy_notes(text, note_href)


def normalize_duokan_notes(text: str) -> tuple[str, int]:
  return core.normalize_duokan_notes(text)
