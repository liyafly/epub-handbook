#!/usr/bin/env python3
"""Regression tests for epub_style_preset_tool.py."""

from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

from test_support.epub_fixture import write_epub as write_fixture_epub


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_style_preset_tool.py"
VALIDATOR = ROOT / "scripts" / "validate_text_invariance.py"
PRESETS = ROOT / "templates" / "style-presets"
OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}
XHTML_NS = {"x": "http://www.w3.org/1999/xhtml"}
LAYERS = ["fonts.css", "base.css", "notes.css", "effects.css", "literary.css", "media.css"]


def run_cli(*args: str) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), *args],
    cwd=ROOT,
    check=False,
    capture_output=True,
    text=True,
  )


def make_epub(path: Path, classes: str) -> None:
  chapter = f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title>Fixture</title>
    <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  </head>
  <body class="{classes}">
    <section>
      <h1>测试章</h1>
      <p>正文保持不变。</p>
    </section>
  </body>
</html>
"""
  files: dict[str, bytes | str] = {
    "mimetype": b"application/epub+zip",
    "META-INF/container.xml": """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
""",
    "OEBPS/content.opf": """<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:test:preset</dc:identifier>
    <dc:title>Preset Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="old-base" href="Styles/base.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
""",
    "OEBPS/Text/chapter.xhtml": chapter,
    "OEBPS/Styles/base.css": "body { line-height: 1.2; }\n",
    "OEBPS/Images/cover.jpg": b"fixture-cover",
  }
  write_fixture_epub(path, files)


def test_list_show_and_missing() -> None:
  listed = run_cli("list")
  assert listed.returncode == 0, listed.stderr
  assert set(listed.stdout.split()) == {"literary-cn", "classical-annotated-cn", "academic-cn"}

  shown = run_cli("show", "literary-cn")
  assert shown.returncode == 0, shown.stderr
  assert "中文文学向" in shown.stdout
  assert "fonts.css" in shown.stdout

  missing = run_cli("show", "not-a-preset")
  assert missing.returncode == 2
  assert "not-a-preset" in missing.stderr


def test_dry_run_coverage_actions_and_read_only() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    source = root / "palette.epub"
    output = root / "output.epub"
    make_epub(source, "font-st chapter-head note-box img-left")
    before = hashlib.sha256(source.read_bytes()).hexdigest()
    result = run_cli(
      "apply",
      str(source),
      "--preset",
      "literary-cn",
      "--output",
      str(output),
      "--dry-run",
    )
    assert result.returncode == 0, result.stderr
    report = json.loads(result.stdout)
    assert report["dry_run"] is True
    assert report["coverage"]["ratio"] >= 0.3, report
    assert "先走 cleanup pipeline" not in result.stderr
    actions = {item["path"]: item["action"] for item in report["stylesheets"]}
    assert actions["OEBPS/Styles/base.css"] == "replace"
    assert actions["OEBPS/Styles/fonts.css"] == "add"
    assert hashlib.sha256(source.read_bytes()).hexdigest() == before
    assert not output.exists()

    random_source = root / "random.epub"
    make_epub(random_source, "calibre99 raw-scene mystery-token")
    random_result = run_cli(
      "apply",
      str(random_source),
      "--preset",
      "literary-cn",
      "--output",
      str(root / "random-output.epub"),
      "--dry-run",
    )
    assert random_result.returncode == 0, random_result.stderr
    random_report = json.loads(random_result.stdout)
    assert random_report["coverage"]["ratio"] < 0.3, random_report
    assert "先走 cleanup pipeline" in random_result.stderr


def test_apply_updates_styles_manifest_links_and_preserves_text() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    source = root / "source.epub"
    output = root / "output.epub"
    make_epub(source, "font-st chapter-head note-box img-left")
    result = run_cli(
      "apply",
      str(source),
      "--preset",
      "literary-cn",
      "--output",
      str(output),
    )
    assert result.returncode == 0, result.stderr
    report = json.loads(result.stdout)
    assert report["written_output"] == str(output.resolve())

    with zipfile.ZipFile(output) as zf:
      infos = zf.infolist()
      assert infos[0].filename == "mimetype"
      assert infos[0].compress_type == zipfile.ZIP_STORED
      for layer in LAYERS:
        assert f"OEBPS/Styles/{layer}" in zf.namelist()
      assert zf.read("OEBPS/Styles/base.css") != b"body { line-height: 1.2; }\n"

      opf = ET.fromstring(zf.read("OEBPS/content.opf"))
      css_hrefs = {
        item.attrib["href"]
        for item in opf.findall("opf:manifest/opf:item", OPF_NS)
        if item.attrib.get("media-type") == "text/css"
      }
      assert {f"Styles/{layer}" for layer in LAYERS}.issubset(css_hrefs)

      chapter = ET.fromstring(zf.read("OEBPS/Text/chapter.xhtml"))
      links = [
        link.attrib["href"]
        for link in chapter.findall("x:head/x:link", XHTML_NS)
        if link.attrib.get("rel") == "stylesheet"
      ]
      assert links == [f"../Styles/{layer}" for layer in LAYERS], links

    gate = subprocess.run(
      [
        sys.executable,
        str(VALIDATOR),
        str(source),
        str(output),
        "--check",
        "all",
      ],
      cwd=ROOT,
      check=False,
      capture_output=True,
      text=True,
    )
    assert gate.returncode == 0, gate.stdout + gate.stderr


def test_preset_css_line_limits() -> None:
  for path in PRESETS.glob("*/Styles/*.css"):
    assert len(path.read_text(encoding="utf-8").splitlines()) <= 400, path


def main() -> int:
  test_list_show_and_missing()
  test_dry_run_coverage_actions_and_read_only()
  test_apply_updates_styles_manifest_links_and_preserves_text()
  test_preset_css_line_limits()
  print("epub style preset tool tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
