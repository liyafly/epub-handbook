#!/usr/bin/env python3
"""Regression tests for epub_text_gate.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_text_gate as G  # noqa: E402

XHTML_NS = "http://www.w3.org/1999/xhtml"


def _make_min_epub(path: Path, body_text: str) -> Path:
    """Create a minimal EPUB 3 file with the given body text."""
    container = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">'
        "  <rootfiles>"
        '    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>'
        "  </rootfiles>"
        "</container>"
    )
    opf = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">'
        '  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">'
        "    <dc:identifier id=\"book-id\">urn:uuid:test-gate-00000001</dc:identifier>"
        "    <dc:title>Gate Fixture</dc:title>"
        "    <dc:language>zh-CN</dc:language>"
        "  </metadata>"
        "  <manifest>"
        '    <item id="toc" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>'
        '    <item id="ch1" href="chapter.xhtml" media-type="application/xhtml+xml"/>'
        "  </manifest>"
        '  <spine><itemref idref="ch1"/></spine>'
        "</package>"
    )
    nav = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<html xmlns="http://www.w3.org/1999/xhtml" '
        'xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">'
        "<head><title>Nav</title></head>"
        '<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body>'
        "</html>"
    )
    chapter = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        f'<html xmlns="{XHTML_NS}" xml:lang="zh-CN">'
        "<head><title>Chapter</title></head>"
        f"<body><p>{body_text}</p></body></html>"
    )
    with zipfile.ZipFile(path, "w") as zf:
        zf.writestr("META-INF/container.xml", container)
        zf.writestr("OEBPS/content.opf", opf)
        zf.writestr("OEBPS/nav.xhtml", nav)
        zf.writestr("OEBPS/chapter.xhtml", chapter)
    return path


def test_epub_text_gate_passes_for_identical() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        a = _make_min_epub(root / "a.epub", "不变文本")
        b = root / "b.epub"
        # Copy identical content
        import shutil
        shutil.copy2(a, b)
        ok, report = G.text_invariance_ok(a, b, allow=["*/nav.xhtml"])
        assert ok is True, f"gate should pass, got: {report[:200]}"


def test_epub_text_gate_fails_for_tampered() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        a = _make_min_epub(root / "a.epub", "原始文本")
        b = _make_min_epub(root / "b.epub", "被改文本")
        ok, report = G.text_invariance_ok(a, b)
        assert ok is False, "gate should fail when body text differs"
        assert "text" in report.lower()


def test_text_gate_respects_allow_list() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        # Same body text but different nav — should pass with allow-list
        a = _make_min_epub(root / "a.epub", "不变文本")
        b = _make_min_epub(root / "b.epub", "不变文本")
        ok, _ = G.text_invariance_ok(a, b, allow=["*/nav.xhtml", "*/toc.ncx"])
        assert ok is True, "gate should pass when body text is identical"


def test_text_gate_default_allow_list() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        a = _make_min_epub(root / "a.epub", "不变文本")
        b = _make_min_epub(root / "b.epub", "不变文本")
        ok, _ = G.text_invariance_ok(a, b)
        assert ok is True


def main() -> int:
    failures = 0
    for name, fn in list(globals().items()):
        if name.startswith("test_") and callable(fn):
            try:
                fn()
            except Exception:
                print(f"FAIL {name}")
                import traceback

                traceback.print_exc()
                failures += 1
    if failures:
        print(f"\n{failures} test(s) failed")
        return 1
    print("all tests passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
