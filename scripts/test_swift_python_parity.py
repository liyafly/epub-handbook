#!/usr/bin/env python3
"""Cross-provider baseline for the native Swift redline and popup CLI paths.

The Python side remains the CLI/Agent compatibility provider. This test does
not make Swift invoke it: it independently runs both surfaces on generated,
text-free fixtures and compares their normalized pass/fail decisions.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
SWIFT_ROOT = ROOT / "swift"
PY_REDLINE = ROOT / "scripts" / "validate_text_invariance.py"
PY_POPUP = ROOT / "scripts" / "validate-popup-notes.sh"
PY_LINT = ROOT / "scripts" / "epub_lint.py"
PY_CSS = ROOT / "scripts" / "epub_css_cleanup.py"
OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def add(zf: zipfile.ZipFile, name: str, data: bytes, *, stored: bool = False) -> None:
  zf.writestr(name, data, compress_type=zipfile.ZIP_STORED if stored else zipfile.ZIP_DEFLATED)


def make_epub(path: Path, *, title: str = "Book", encryption: str | None = None, legacy_popup: bool = False) -> None:
  if legacy_popup:
    chapter = '''<html xmlns="http://www.w3.org/1999/xhtml"><body>
<p>正文<a id="note-one" role="doc-noteref" href="#footnote-one"><img src="../Images/note.png" alt="注"/></a></p>
<aside role="doc-footnote"><p id="footnote-one">注释正文。</p></aside>
</body></html>'''
  else:
    chapter = '<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="chapter">正文。</p></body></html>'
  with zipfile.ZipFile(path, "w") as zf:
    add(zf, "mimetype", b"application/epub+zip", stored=True)
    add(zf, "META-INF/container.xml", b'<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
    add(zf, "OEBPS/package.opf", f'''<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>{title}</dc:title><dc:creator>A</dc:creator><dc:identifier>I</dc:identifier><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="icon" href="Images/note.png" media-type="image/png"/><item id="cover" href="Images/cover.png" media-type="image/png" properties="cover-image"/></manifest><spine><itemref idref="chapter"/></spine></package>'''.encode())
    add(zf, "OEBPS/Text/chapter.xhtml", chapter.encode())
    add(zf, "OEBPS/Images/note.png", b"\x89PNG")
    add(zf, "OEBPS/Images/cover.png", b"cover")
    if encryption is not None:
      add(zf, "META-INF/encryption.xml", encryption.encode())


def make_css_epub(path: Path) -> None:
  """Write the same factoring fixture to both independent providers."""
  with zipfile.ZipFile(path, "w") as zf:
    add(zf, "mimetype", b"application/epub+zip", stored=True)
    add(zf, "META-INF/container.xml", b'<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
    add(zf, "OEBPS/package.opf", b'''<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>CSS fixture</dc:title><dc:creator>A</dc:creator><dc:identifier id="book-id">I</dc:identifier><dc:language>en</dc:language></metadata><manifest><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="three" href="Text/three.xhtml" media-type="application/xhtml+xml"/><item id="cover" href="Images/cover.png" media-type="image/png" properties="cover-image"/><item id="style-one" href="Styles/style0002.css" media-type="text/css"/><item id="style-two" href="Styles/style0003.css" media-type="text/css"/><item id="style-three" href="Styles/style0004.css" media-type="text/css"/></manifest><spine><itemref idref="one"/><itemref idref="two"/><itemref idref="three"/></spine></package>''')
    add(zf, "OEBPS/Images/cover.png", b"cover")
    for chapter, stylesheet in (("one", "style0002.css"), ("two", "style0003.css"), ("three", "style0004.css")):
      add(zf, f"OEBPS/Text/{chapter}.xhtml", f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><title>{chapter}</title><link href="../Styles/{stylesheet}" rel="stylesheet" type="text/css"/></head><body><h1 id="{chapter}">{chapter.title()}</h1><p>Body {chapter}.</p></body></html>'''.encode())
    for path, color in (("style0002.css", "#876c4f"), ("style0003.css", "#3fbbd6"), ("style0004.css", "#70543b")):
      add(zf, f"OEBPS/Styles/{path}", f'''————————————————标题————————————————
h1 {{
  color: {color};
  font-family: "SimHei";
}}
body {{
  font-family: "cnepub", serif;
}}
.part-text {{
  font-family: "STKaiti"
}}
'''.encode())


def make_css_scope_epub(path: Path) -> None:
  with zipfile.ZipFile(path, "w") as zf:
    add(zf, "mimetype", b"application/epub+zip", stored=True)
    add(zf, "META-INF/container.xml", b'<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
    add(zf, "OEBPS/package.opf", b'''<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>CSS scope fixture</dc:title><dc:creator>A</dc:creator><dc:identifier id="book-id">I</dc:identifier><dc:language>en</dc:language></metadata><manifest><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="three" href="Text/three.xhtml" media-type="application/xhtml+xml"/><item id="cover" href="Images/cover.png" media-type="image/png" properties="cover-image"/><item id="local-one" href="Styles/one.css" media-type="text/css"/><item id="local-two" href="Styles/two.css" media-type="text/css"/><item id="global" href="Styles/global.css" media-type="text/css"/></manifest><spine><itemref idref="one"/><itemref idref="two"/><itemref idref="three"/></spine></package>''')
    add(zf, "OEBPS/Images/cover.png", b"cover")
    for chapter, local in (("one", "one.css"), ("two", "two.css"), ("three", None)):
      local_link = f'<link href="../Styles/{local}" rel="stylesheet" type="text/css"/>' if local else ""
      add(zf, f"OEBPS/Text/{chapter}.xhtml", f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><title>{chapter}</title>{local_link}<link href="../Styles/global.css" rel="stylesheet" type="text/css"/></head><body><h1 id="{chapter}">{chapter.title()}</h1><p>Body {chapter}.</p></body></html>'''.encode())
    add(zf, "OEBPS/Styles/one.css", b"h1 { color: red; }")
    add(zf, "OEBPS/Styles/two.css", b".toc { margin: 0; }")
    add(zf, "OEBPS/Styles/global.css", b"body { line-height: 1.5; }")


def swift_binary() -> Path:
  configured = os.environ.get("EPUB_HANDBOOK_SWIFT_BIN")
  if configured:
    binary = Path(configured)
    if binary.is_file():
      return binary
    raise AssertionError(f"EPUB_HANDBOOK_SWIFT_BIN does not exist: {binary}")
  if shutil.which("swift") is None:
    print("swift unavailable; parity test skipped")
    raise SystemExit(0)
  build = subprocess.run(
    ["swift", "build", "--product", "epub-handbook-swift"],
    cwd=SWIFT_ROOT,
    text=True,
    capture_output=True,
    check=False,
  )
  if build.returncode:
    raise AssertionError(f"Swift CLI build failed:\n{build.stdout}\n{build.stderr}")
  bin_path = subprocess.run(
    ["swift", "build", "--show-bin-path"],
    cwd=SWIFT_ROOT,
    text=True,
    capture_output=True,
    check=True,
  ).stdout.strip()
  binary = Path(bin_path) / "epub-handbook-swift"
  if not binary.is_file():
    raise AssertionError(f"Swift CLI binary missing: {binary}")
  return binary


def run(command: list[str]) -> subprocess.CompletedProcess[str]:
  return subprocess.run(command, cwd=ROOT, text=True, capture_output=True, check=False)


def cli_json(binary: Path, *arguments: str) -> tuple[int, dict[str, object]]:
  process = run([str(binary), *arguments])
  try:
    payload = json.loads(process.stdout)
  except json.JSONDecodeError as exc:
    raise AssertionError(f"Swift CLI did not return JSON:\n{process.stdout}\n{process.stderr}") from exc
  return process.returncode, payload


def python_redline(before: Path, after: Path) -> subprocess.CompletedProcess[str]:
  return run([sys.executable, str(PY_REDLINE), str(before), str(after), "--check", "all"])


def python_css(input_path: Path, output_path: Path, *, merge_scoped_local_css: bool = False) -> tuple[int, dict[str, object]]:
  command = [sys.executable, str(PY_CSS), str(input_path), "--output", str(output_path), "--format", "json"]
  if merge_scoped_local_css:
    command.append("--merge-scoped-local-css")
  process = run(command)
  try:
    payload = json.loads(process.stdout)
  except json.JSONDecodeError as exc:
    raise AssertionError(f"Python CSS cleanup did not return JSON:\n{process.stdout}\n{process.stderr}") from exc
  return process.returncode, payload


def css_facts(path: Path) -> tuple[set[str], dict[str, tuple[str, ...]], dict[str, str]]:
  with zipfile.ZipFile(path) as zf:
    names = set(zf.namelist())
    root = ET.fromstring(zf.read("OEBPS/package.opf"))
    manifest_css = {
      item.attrib["href"]
      for item in root.findall("opf:manifest/opf:item", OPF_NS)
      if item.attrib.get("media-type") == "text/css"
    }
    links: dict[str, tuple[str, ...]] = {}
    for chapter in ("one", "two", "three"):
      text = zf.read(f"OEBPS/Text/{chapter}.xhtml").decode("utf-8")
      hrefs = tuple(re.findall(r'<link\b[^>]*\bhref=["\']([^"\']+\.css)["\']', text, flags=re.I))
      links[chapter] = hrefs
    css = {
      name.removeprefix("OEBPS/Styles/"): zf.read(name).decode("utf-8")
      for name in names
      if name.startswith("OEBPS/Styles/") and name.endswith(".css")
    }
  return manifest_css, links, css


def assert_css_cleanup_parity(binary: Path, root: Path) -> None:
  source = root / "css-before.epub"
  python_output = root / "css-python.epub"
  swift_output = root / "css-swift.epub"
  make_css_epub(source)
  python_code, python_report = python_css(source, python_output)
  swift_code, swift_report = cli_json(
    binary,
    "run", "epub.css.layering.optimize",
    "--input", str(source),
    "--output", str(swift_output),
    "--workspace", str(root / "css-swift-workspace"),
    "--format", "json",
  )
  require(python_code == 0, f"Python CSS cleanup failed: {python_report}")
  require(python_report.get("factored_stylesheets") == 3, f"Python factor fixture drifted: {python_report}")
  require(python_report.get("overrides_created") == 2, f"Python override fixture drifted: {python_report}")
  require(swift_code == 0 and swift_report.get("status") == "complete", f"Swift CSS cleanup failed: {swift_report}")

  python_manifest, python_links, python_css_files = css_facts(python_output)
  swift_manifest, swift_links, swift_css_files = css_facts(swift_output)
  require(python_manifest == swift_manifest, f"CSS manifest parity mismatch: {python_manifest} != {swift_manifest}")
  require(python_links == swift_links, f"XHTML stylesheet link parity mismatch: {python_links} != {swift_links}")
  require(set(python_css_files) == set(swift_css_files), f"Generated CSS paths differ: {python_css_files.keys()} != {swift_css_files.keys()}")
  for name in python_css_files:
    normalize = lambda value: "".join(value.split()).lower()
    require(normalize(python_css_files[name]) == normalize(swift_css_files[name]), f"Generated CSS differs: {name}")

  for artifact in (python_output, swift_output):
    lint = run([sys.executable, str(PY_LINT), str(artifact)])
    require(lint.returncode == 0, f"EPUB lint rejected CSS artifact {artifact}:\n{lint.stdout}\n{lint.stderr}")
    redline = python_redline(source, artifact)
    require(redline.returncode == 0, f"Python redline rejected CSS artifact {artifact}:\n{redline.stderr}")


def assert_css_scope_parity(binary: Path, root: Path) -> None:
  source = root / "css-scope-before.epub"
  python_output = root / "css-scope-python.epub"
  swift_output = root / "css-scope-swift.epub"
  make_css_scope_epub(source)
  python_code, python_report = python_css(source, python_output, merge_scoped_local_css=True)
  swift_code, swift_report = cli_json(
    binary,
    "normalize-css",
    "--input", str(source),
    "--output", str(swift_output),
    "--workspace", str(root / "css-scope-swift-workspace"),
    "--merge-scoped-local-css",
    "--format", "json",
  )
  require(python_code == 0 and python_report.get("scoped_local_stylesheets_merged") == 2, f"Python CSS scope cleanup failed: {python_report}")
  require(swift_code == 0 and swift_report.get("status") == "complete", f"Swift CSS scope cleanup failed: {swift_report}")
  python_manifest, python_links, python_css_files = css_facts(python_output)
  swift_manifest, swift_links, swift_css_files = css_facts(swift_output)
  require(python_manifest == swift_manifest, f"Scoped CSS manifest parity mismatch: {python_manifest} != {swift_manifest}")
  require(python_links == swift_links, f"Scoped XHTML link parity mismatch: {python_links} != {swift_links}")
  require(set(python_css_files) == set(swift_css_files), f"Scoped generated CSS paths differ: {python_css_files.keys()} != {swift_css_files.keys()}")
  for name in python_css_files:
    normalize = lambda value: "".join(value.split()).lower()
    require(normalize(python_css_files[name]) == normalize(swift_css_files[name]), f"Scoped generated CSS differs: {name}")
  with zipfile.ZipFile(python_output) as python_zip, zipfile.ZipFile(swift_output) as swift_zip:
    for chapter, scope in (("one", "css-local-01"), ("two", "css-local-02")):
      python_chapter = python_zip.read(f"OEBPS/Text/{chapter}.xhtml").decode("utf-8")
      swift_chapter = swift_zip.read(f"OEBPS/Text/{chapter}.xhtml").decode("utf-8")
      require(scope in python_chapter and scope in swift_chapter, f"Missing scoped body class {scope}")
  for artifact in (python_output, swift_output):
    lint = run([sys.executable, str(PY_LINT), str(artifact)])
    require(lint.returncode == 0, f"EPUB lint rejected scoped CSS artifact {artifact}:\n{lint.stdout}\n{lint.stderr}")
    redline = python_redline(source, artifact)
    require(redline.returncode == 0, f"Python redline rejected scoped CSS artifact {artifact}:\n{redline.stderr}")


def require(condition: bool, message: str) -> None:
  if not condition:
    raise AssertionError(message)


def main() -> int:
  binary = swift_binary()
  with tempfile.TemporaryDirectory(prefix="epub-handbook-swift-parity-") as raw:
    root = Path(raw)
    before = root / "before.epub"
    after = root / "after.epub"
    make_epub(before)
    make_epub(after)

    python_pass = python_redline(before, after)
    swift_code, swift_pass = cli_json(binary, "validate-redlines", "--before", str(before), "--after", str(after), "--format", "json")
    require(python_pass.returncode == 0, python_pass.stderr)
    require(swift_code == 0 and swift_pass.get("status") == "complete", f"Swift baseline mismatch: {swift_pass}")

    changed = root / "metadata-changed.epub"
    make_epub(changed, title="Changed")
    python_changed = python_redline(before, changed)
    swift_code, swift_changed = cli_json(binary, "validate-redlines", "--before", str(before), "--after", str(changed), "--format", "json")
    package = swift_changed.get("package", {})
    issue_kinds = {issue.get("kind") for issue in package.get("issues", [])} if isinstance(package, dict) else set()
    require(python_changed.returncode == 1, python_changed.stderr)
    require(swift_code == 1 and swift_changed.get("status") == "failed" and "metadata-changed" in issue_kinds, f"Swift metadata mismatch: {swift_changed}")

    drm = root / "drm.epub"
    make_epub(drm, encryption="<encryption/>")
    python_drm = python_redline(before, drm)
    swift_code, swift_drm = cli_json(binary, "validate-redlines", "--before", str(before), "--after", str(drm), "--format", "json")
    package = swift_drm.get("package", {})
    issue_kinds = {issue.get("kind") for issue in package.get("issues", [])} if isinstance(package, dict) else set()
    require(python_drm.returncode == 2, python_drm.stderr)
    require(swift_code == 1 and swift_drm.get("status") == "failed" and "drm-detected" in issue_kinds, f"Swift DRM mismatch: {swift_drm}")

    popup_before = root / "popup-before.epub"
    popup_after = root / "popup-after.epub"
    workspace = root / "popup-workspace"
    make_epub(popup_before, legacy_popup=True)
    swift_code, popup_run = cli_json(
      binary,
      "run", "epub.notes.popup.normalize",
      "--input", str(popup_before),
      "--output", str(popup_after),
      "--workspace", str(workspace),
      "--format", "json",
    )
    require(swift_code == 0 and popup_run.get("status") == "complete", f"Swift popup run failed: {popup_run}")
    popup_check = run(["bash", str(PY_POPUP), "--epub", str(popup_after)])
    require(popup_check.returncode == 0, f"Python popup validator failed:\n{popup_check.stdout}\n{popup_check.stderr}")
    lint = run([sys.executable, str(PY_LINT), str(popup_after)])
    require(lint.returncode == 0, f"EPUB lint rejected Swift popup output:\n{lint.stdout}\n{lint.stderr}")
    popup_redline = python_redline(popup_before, popup_after)
    require(popup_redline.returncode == 0, f"Python redline rejected Swift popup output:\n{popup_redline.stderr}")
    swift_code, popup_redline_swift = cli_json(binary, "validate-redlines", "--before", str(popup_before), "--after", str(popup_after), "--format", "json")
    require(swift_code == 0 and popup_redline_swift.get("status") == "complete", f"Swift popup redline failed: {popup_redline_swift}")

    assert_css_cleanup_parity(binary, root)
    assert_css_scope_parity(binary, root)

  print("Swift/Python parity baseline ok (redlines + popup + native CSS cleanup and scope artifacts)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
