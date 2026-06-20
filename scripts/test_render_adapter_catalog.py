#!/usr/bin/env python3
"""Tests for agent / product projections of the neutral capability catalog."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import render_adapter_catalog as adapters  # noqa: E402


class RenderAdapterCatalogTests(unittest.TestCase):
  def test_cli_projection_describes_neutral_capability_command(self) -> None:
    catalog = adapters.build_catalog(ROOT, "cli")
    popup = next(item for item in catalog["capabilities"] if item["id"] == "epub.notes.popup.normalize")

    self.assertEqual(popup["command"], "epub-handbook run epub.notes.popup.normalize --input <uri>")
    self.assertEqual(popup["requestSchema"], "contracts/schemas/v1/artifact-reference.schema.json")
    self.assertNotIn("scripts/", popup["command"])

  def test_mcp_projection_uses_stable_tool_name(self) -> None:
    catalog = adapters.build_catalog(ROOT, "mcp")
    popup = next(item for item in catalog["capabilities"] if item["id"] == "epub.notes.popup.normalize")

    self.assertEqual(popup["tool"], "epub_notes_popup_normalize")
    self.assertEqual(popup["responseSchema"], "contracts/schemas/v1/run-report.schema.json")


if __name__ == "__main__":
  unittest.main()
