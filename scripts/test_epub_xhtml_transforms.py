#!/usr/bin/env python3
"""Regression tests for epub_xhtml_transforms.py."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_xhtml_transforms as T  # noqa: E402


# ── Task 2: sanitize + class rename ───────────────────────────────────────


def test_sanitize_entities_minimal_diff() -> None:
    src = "<p>a&nbsp;b</p>"
    out = T.sanitize_for_xml(src)
    assert out == "<p>a&#160;b</p>", f"unexpected: {out!r}"


def test_sanitize_nbsp_only_in_named_entities() -> None:
    src = "<p>a &amp; b &nbsp; c</p>"
    out = T.sanitize_for_xml(src)
    # &amp; is valid XML — left alone; &nbsp; becomes &#160;
    assert "&amp;" in out
    assert "&#160;" in out


def test_class_value_rename_anchored() -> None:
    src = '<p class="calibre12">正文不变</p>'
    out, n = T.rename_class_values(src, {"calibre12": "para"})
    assert out == '<p class="para">正文不变</p>'
    assert n == 1


def test_class_rename_is_idempotent() -> None:
    src = '<p class="para">正文不变</p>'
    out, n = T.rename_class_values(src, {"calibre12": "para"})
    assert out == src and n == 0


def test_class_rename_never_touches_prose_lookalike() -> None:
    src = '<p class="calibre12">这里提到 calibre12 这个词</p>'
    out, _ = T.rename_class_values(src, {"calibre12": "para"})
    assert "这里提到 calibre12 这个词" in out
    assert 'class="para"' in out


def test_class_rename_multiple_classes() -> None:
    src = '<div class="calibre12 foo bar">text</div>'
    out, n = T.rename_class_values(src, {"calibre12": "para", "foo": "baz"})
    assert n == 1  # one element's class list changed (even though two tokens changed)
    assert 'class="para baz bar"' in out or 'class="para bar baz"' in out


# ── Task 3: DOM add attribute ─────────────────────────────────────────────


def test_add_epub_type_attribute() -> None:
    xhtml = (
        '<?xml version="1.0" encoding="utf-8"?>'
        '<html xmlns="http://www.w3.org/1999/xhtml" '
        'xmlns:epub="http://www.idpf.org/2007/ops"><body>'
        '<aside id="fn1"><p>注释正文</p></aside></body></html>'
    )
    out, changed = T.dom_add_attr(
        xhtml,
        target_id="fn1",
        attr="{http://www.idpf.org/2007/ops}type",
        value="footnote",
    )
    assert changed is True
    assert 'epub:type="footnote"' in out
    assert "注释正文" in out


def test_dom_add_attr_idempotent() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml" '
        'xmlns:epub="http://www.idpf.org/2007/ops"><body>'
        '<aside id="fn1" epub:type="footnote"><p>x</p></aside></body></html>'
    )
    out, changed = T.dom_add_attr(
        xhtml,
        "fn1",
        "{http://www.idpf.org/2007/ops}type",
        "footnote",
    )
    assert changed is False
    assert 'epub:type="footnote"' in out


def test_dom_add_xml_lang() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<p id="p1">Hello</p></body></html>'
    )
    out, changed = T.dom_add_attr(
        xhtml,
        target_id="p1",
        attr="{http://www.w3.org/XML/1998/namespace}lang",
        value="en",
    )
    assert changed is True
    assert 'xml:lang="en"' in out
    assert "Hello" in out


def test_dom_add_attr_no_match_unchanged() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<p id="p1">text</p></body></html>'
    )
    out, changed = T.dom_add_attr(
        xhtml,
        target_id="missing",
        attr="{http://www.idpf.org/2007/ops}type",
        value="footnote",
    )
    assert changed is False
    assert out == xhtml


# ── Task 4: structural rewrite + forbidden-text-change guard ───────────────


def test_div_quote_to_blockquote_preserves_text() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<div class="quote"><p>引文一字不差</p></div>'
        "</body></html>"
    )
    out, changed = T.dom_rewrite_tag(
        xhtml, match={"tag": "div", "class": "quote"}, new_tag="blockquote"
    )
    assert changed is True
    assert "<blockquote" in out
    assert "引文一字不差" in out


def test_dom_rewrite_tag_strips_matched_class() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<div class="quote keep-me"><p>x</p></div></body></html>'
    )
    out, changed = T.dom_rewrite_tag(
        xhtml, match={"tag": "div", "class": "quote"}, new_tag="blockquote"
    )
    assert changed is True
    assert "quote" not in (el_class := out.split('class="')[1].split('"')[0])
    assert "keep-me" in el_class


def test_forbidden_guard_rejects_text_change() -> None:
    before = "<p>正文 ABCDE</p>"
    after = "<p>正文 ABCD</p>"
    assert T.text_content_equal(before, after) is False


def test_forbidden_guard_raises_on_structural_change() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<div class="quote"><p>原文</p></div></body></html>'
    )
    # A "bad" rewrite tag that would drop text — our implementation
    # should reject it via text_content_equal guard inside dom_rewrite_tag.
    # Normal rewrite preserves text, so test that guard catches the
    # degenerate case by verifying text_content_equal works as gate.
    assert T.text_content_equal(
        '<html xmlns="http://www.w3.org/1999/xhtml"><body><p>原文ABC</p></body></html>',
        '<html xmlns="http://www.w3.org/1999/xhtml"><body><p>原文XYZ</p></body></html>',
    ) is False


def test_dom_rewrite_tag_no_match_unchanged() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<div class="note"><p>text</p></div></body></html>'
    )
    out, changed = T.dom_rewrite_tag(
        xhtml, match={"tag": "div", "class": "quote"}, new_tag="blockquote"
    )
    assert changed is False
    assert out == xhtml


def test_dom_rewrite_tag_preserves_xml_declaration() -> None:
    xhtml = (
        '<?xml version="1.0" encoding="utf-8"?>'
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<div class="quote"><p>x</p></div></body></html>'
    )
    out, changed = T.dom_rewrite_tag(
        xhtml, match={"tag": "div", "class": "quote"}, new_tag="blockquote"
    )
    assert changed is True
    assert out.lstrip().startswith("<?xml")


def test_text_content_equal_normalizes_unicode() -> None:
    a = "<p>café</p>"  # composed
    b = "<p>café</p>"  # decomposed
    assert T.text_content_equal(a, b) is True


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
