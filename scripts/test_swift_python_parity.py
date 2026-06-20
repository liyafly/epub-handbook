#!/usr/bin/env python3
"""Cross-provider baseline for the native Swift redline and popup CLI paths.

The Python side remains the CLI/Agent compatibility provider. This test does
not make Swift invoke it: it independently runs both surfaces on generated,
text-free fixtures and compares their normalized pass/fail decisions.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SWIFT_ROOT = ROOT / "swift"
PY_REDLINE = ROOT / "scripts" / "validate_text_invariance.py"
PY_POPUP = ROOT / "scripts" / "validate-popup-notes.sh"
PY_LINT = ROOT / "scripts" / "epub_lint.py"


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

  print("Swift/Python parity baseline ok (redlines + native popup artifact)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
