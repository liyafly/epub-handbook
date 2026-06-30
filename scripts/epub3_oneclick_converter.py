#!/usr/bin/env python3
"""Backward-compatible CLI façade for focused EPUB3 migration passes."""

from __future__ import annotations

import sys

from epub3_conversion.core import *  # noqa: F403
from epub3_conversion.core import main
from epub3_conversion.converter import convert_epub
from epub3_conversion.models import ConversionReport
from epub3_conversion.navigation import ensure_nav
from epub3_conversion.notes import convert_plain_notes
from epub3_conversion.package import normalize_metadata
from epub3_conversion.xhtml import format_xhtml_multiline, normalize_xhtml_shell


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
