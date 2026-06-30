#!/usr/bin/env python3
"""Integration tests for the public font coverage adapter."""

from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

from test_support.epub_fixture import write_epub as write_fixture_epub


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_font_coverage_adapter.py"
REFINEMENT = ROOT / "scripts" / "epub_refinement_harness.py"


def make_epub(path: Path) -> None:
  write_fixture_epub(path, {
    "META-INF/container.xml": '<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>',
    "OEBPS/package.opf": '''<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Test</dc:title><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/><item id="css" href="Styles/main.css" media-type="text/css"/></manifest><spine><itemref idref="c1"/></spine></package>''',
    "OEBPS/Text/c1.xhtml": '''<html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN"><head><title>Test</title><link href="../Styles/main.css" rel="stylesheet" type="text/css"/></head><body><p>常用中文测试。</p></body></html>''',
    "OEBPS/Styles/main.css": 'body { font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }',
  })


class FontCoverageAdapterTests(unittest.TestCase):
  def test_adapter_returns_json_and_preserves_input(self) -> None:
    with TemporaryDirectory() as raw:
      epub = Path(raw) / "book.epub"
      make_epub(epub)
      before = hashlib.sha256(epub.read_bytes()).hexdigest()
      result = subprocess.run(
        [sys.executable, str(SCRIPT), str(epub), "--format", "json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
      )
      self.assertEqual(result.returncode, 0, result.stderr)
      report = json.loads(result.stdout)
      self.assertEqual(report["capability"], "epub.font.coverage.analyze")
      self.assertEqual(report["schema_version"], "1.0")
      self.assertIn(report["status"], {"pass", "warn", "fail"})
      self.assertIn("summary", report)
      self.assertEqual(hashlib.sha256(epub.read_bytes()).hexdigest(), before)

  def test_refinement_recommends_font_coverage_analysis(self) -> None:
    with TemporaryDirectory() as raw:
      epub = Path(raw) / "book.epub"
      make_epub(epub)
      result = subprocess.run(
        [sys.executable, str(REFINEMENT), str(epub), "--format", "json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
      )
      self.assertIn(result.returncode, {0, 1}, result.stderr)
      report = json.loads(result.stdout)
      typography = next(item for item in report["recommendations"] if item["id"] == "typography-fonts")
      self.assertIn("$epub-font-coverage-analyzer", typography["skills"])


if __name__ == "__main__":
  unittest.main()
