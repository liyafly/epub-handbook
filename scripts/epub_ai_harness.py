#!/usr/bin/env python3
"""Backward-compatible CLI façade for EPUB/source analysis and routing."""

from __future__ import annotations

import sys

from epub_ai.core import *  # noqa: F403
from epub_ai.core import main
from epub_ai.detectors import DETECTORS, Detector, collect_actionable_findings, detector
from epub_ai.model import EpubModel, build_model, build_model_from_tree
from epub_ai.report import Report, render_markdown
from epub_ai.routing import inspect_path


if __name__ == "__main__":
  raise SystemExit(main(sys.argv[1:]))
