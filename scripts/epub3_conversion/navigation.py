"""EPUB NCX-to-nav migration boundary."""

from __future__ import annotations

from xml.etree import ElementTree as ET

from . import core
from .models import ConversionReport


def ensure_nav(files: dict[str, bytes], root: ET.Element, opf_path: str, report: ConversionReport) -> None:
  core.ensure_nav(files, root, opf_path, report)
