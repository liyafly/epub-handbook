"""XHTML shell, formatting, stylesheet, and document update passes."""

from __future__ import annotations

from . import core
from .models import ConversionReport


TYPOGRAPHY_ROLES = tuple(core.TYPOGRAPHY_ROLES)


def normalize_xhtml_shell(text: str, default_language: str | None = None) -> tuple[str, bool]:
  return core.normalize_xhtml_shell(text, default_language)


def format_xhtml_multiline(text: str) -> tuple[str, bool]:
  return core.format_xhtml_multiline(text)


def enhancement_css() -> bytes:
  return core.enhancement_css()


def note_png_bytes() -> bytes:
  return core.note_png_bytes()


def xhtml_default_language(root) -> str | None:
  return core.xhtml_default_language(root)


def default_note_href(files: dict[str, bytes], root, opf_dir: str) -> str:
  return core.default_note_href(files, root, opf_dir)


def update_xhtml_files(
  files: dict[str, bytes],
  root,
  opf_path: str,
  style_zip: str,
  note_zip: str,
  report: ConversionReport,
  popup_notes: bool,
  typography: bool,
  default_language: str | None = None,
) -> bool:
  return core.update_xhtml_files(
    files,
    root,
    opf_path,
    style_zip,
    note_zip,
    report,
    popup_notes,
    typography,
    default_language,
  )


__all__ = [
  "TYPOGRAPHY_ROLES",
  "default_note_href",
  "enhancement_css",
  "format_xhtml_multiline",
  "normalize_xhtml_shell",
  "note_png_bytes",
  "update_xhtml_files",
  "xhtml_default_language",
]
