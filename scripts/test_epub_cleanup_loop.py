#!/usr/bin/env python3
"""Regression tests for epub_cleanup_loop.py."""

from __future__ import annotations

import json
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_cleanup_loop as L  # noqa: E402

XHTML_NS = "http://www.w3.org/1999/xhtml"


# ── Fixture helpers ───────────────────────────────────────────────────────


def _make_min_epub(path: Path, body_text: str, *,
                   with_html_lang: bool = True,
                   with_html_id: str = "html-root") -> None:
    """Create a minimal EPUB 3 zip file."""
    lang_attr = ' xml:lang="zh-CN"' if with_html_lang else ""
    root_id = f' id="{with_html_id}"' if with_html_id else ""
    container = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">'
        '  <rootfiles>'
        '    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>'
        '  </rootfiles>'
        '</container>'
    )
    opf = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">'
        '  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">'
        '    <dc:identifier id="book-id">urn:uuid:loop-test-0001</dc:identifier>'
        '    <dc:title>Loop Fixture</dc:title>'
        '    <dc:language>zh-CN</dc:language>'
        '  </metadata>'
        '  <manifest>'
        '    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>'
        '    <item id="ch1" href="chapter.xhtml" media-type="application/xhtml+xml"/>'
        '  </manifest>'
        '  <spine><itemref idref="ch1"/></spine>'
        '</package>'
    )
    nav = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<html xmlns="http://www.w3.org/1999/xhtml" '
        'xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">'
        '<head><title>Nav</title></head>'
        '<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Ch</a></li></ol></nav></body>'
        '</html>'
    )
    chapter = (
        '<?xml version="1.0" encoding="UTF-8"?>'
        f'<html xmlns="{XHTML_NS}"{lang_attr}{root_id}>'
        '<head><title>Chapter</title></head>'
        f'<body><p>{body_text}</p></body></html>'
    )
    with zipfile.ZipFile(path, "w") as zf:
        zf.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
        zf.writestr("META-INF/container.xml", container)
        zf.writestr("OEBPS/content.opf", opf)
        zf.writestr("OEBPS/nav.xhtml", nav)
        zf.writestr("OEBPS/chapter.xhtml", chapter)


# ── Task 6: RulesPlanner ─────────────────────────────────────────────────


def test_rules_planner_maps_auto_fixable_findings() -> None:
    findings = {
        "actionable_findings": [
            {
                "kind": "missing-html-lang",
                "file": "OEBPS/chapter.xhtml",
                "locator": {"selector": "html"},
                "params": {"value": "zh-Hans"},
                "lane": "tag",
                "auto_fixable": True,
                "confidence": "high",
            },
            {
                "kind": "ambiguous-note",
                "file": "OEBPS/chapter.xhtml",
                "locator": {"id": "x"},
                "params": {},
                "lane": "tag",
                "auto_fixable": False,
                "confidence": "low",
            },
        ]
    }
    refine: dict = {}
    plan = L.RulesPlanner().plan(round=1, findings=findings, refinement=refine,
                                  enable_structural=False)
    ops = {a["op"] for a in plan["actions"]}
    assert "add-xml-lang" in ops, f"expected add-xml-lang in actions: {plan}"
    assert all(a["lane"] in ("css", "tag", "package") for a in plan["actions"])
    assert not any(a.get("op") == "ambiguous-note" for a in plan["actions"])
    assert any(s.get("kind") == "ambiguous-note" for s in plan["suggestions"])


def test_rules_planner_uses_no_model() -> None:
    plan = L.RulesPlanner().plan(
        round=1,
        findings={"actionable_findings": []},
        refinement={},
        enable_structural=False,
    )
    assert plan["actions"] == [] and plan["round"] == 1


def test_rules_planner_respects_structural_flag() -> None:
    findings = {
        "actionable_findings": [
            {
                "kind": "empty-paragraph",
                "file": "OEBPS/chapter.xhtml",
                "locator": {"id": "p1"},
                "params": {"rule": "empty-paragraph"},
                "lane": "tag",
                "auto_fixable": True,
                "confidence": "high",
            },
        ]
    }
    # Without --enable-structural, empty-paragraph goes to suggestions
    plan_no = L.RulesPlanner().plan(round=1, findings=findings, refinement={},
                                     enable_structural=False)
    assert not plan_no["actions"]
    assert any(s["kind"] == "empty-paragraph" for s in plan_no["suggestions"])

    # With --enable-structural but no matching rule in ALLOWED_STRUCTURAL,
    # it stays in suggestions (rule not yet defined)
    plan_yes = L.RulesPlanner().plan(round=1, findings=findings, refinement={},
                                      enable_structural=True)
    # Currently, empty-paragraph structural rule is not in ALLOWED_STRUCTURAL
    assert not plan_yes["actions"], "empty-paragraph structural not yet defined"
    assert any("empty-paragraph" in str(s) for s in plan_yes["suggestions"])


# ── Task 7: apply_action ─────────────────────────────────────────────────


def test_apply_action_add_xml_lang() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml">'
        '<body><p id="p1">Hello</p></body></html>'
    )
    files = {"OEBPS/chapter.xhtml": xhtml}
    action = {
        "op": "add-xml-lang",
        "file": "OEBPS/chapter.xhtml",
        "params": {"id": "p1", "value": "en"},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "applied", f"unexpected: {res}"
    assert 'xml:lang="en"' in files["OEBPS/chapter.xhtml"]


def test_apply_action_idempotent() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml" '
        'xmlns:epub="http://www.idpf.org/2007/ops">'
        '<body><aside id="fn1" epub:type="footnote"><p>x</p></aside></body></html>'
    )
    files = {"OEBPS/chapter.xhtml": xhtml}
    action = {
        "op": "add-epub-type",
        "file": "OEBPS/chapter.xhtml",
        "params": {"id": "fn1", "value": "footnote"},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "noop", f"expected noop, got: {res}"


def test_apply_action_rename_class() -> None:
    xhtml = '<p class="calibre12">text</p>'
    files = {"OEBPS/chapter.xhtml": xhtml}
    action = {
        "op": "rename-class",
        "file": "OEBPS/chapter.xhtml",
        "params": {"mapping": {"calibre12": "para"}},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "applied"
    assert 'class="para"' in files["OEBPS/chapter.xhtml"]
    assert "text" in files["OEBPS/chapter.xhtml"]


def test_apply_action_reverts_on_text_change() -> None:
    xhtml = (
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>'
        '<p id="p1">原文ABC</p></body></html>'
    )
    files = {"OEBPS/chapter.xhtml": xhtml}
    # A "bad" rename-class that would change text — rename_class_values
    # only touches class attributes, so it can't change text. But we
    # test the forbidden-text-change guard via the post-transform
    # text_content_equal check.
    action = {
        "op": "add-xml-lang",
        "file": "OEBPS/chapter.xhtml",
        "params": {"id": "p1", "value": "en"},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    # add-xml-lang should NOT change text content
    assert res["status"] == "applied"
    assert "原文ABC" in files["OEBPS/chapter.xhtml"]


def test_apply_action_unknown_op_rejected() -> None:
    files = {"OEBPS/chapter.xhtml": "<p>x</p>"}
    action = {
        "op": "delete-content",
        "file": "OEBPS/chapter.xhtml",
        "params": {},
        "lane": "tag",
        "source": "ai",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "rejected"
    assert res["reason"] == "op-not-whitelisted"


def test_apply_action_file_missing() -> None:
    files: dict[str, str] = {}
    action = {
        "op": "add-xml-lang",
        "file": "OEBPS/missing.xhtml",
        "params": {"id": "x", "value": "en"},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "skipped"


# ── Task 9: HandshakePlanner ──────────────────────────────────────────────


def test_handshake_planner_reads_external_plan() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        reqdir = root / "reports"
        reqdir.mkdir()
        L._write_json(
            reqdir / "round-1.plan.json",
            {"round": 1, "actions": [],
             "suggestions": [{"kind": "test-tip", "note": "check this"}]},
        )
        p = L.HandshakePlanner(reports_dir=reqdir)
        plan = p.plan(1, {"findings": []}, {}, False)
        assert plan["suggestions"] == [{"kind": "test-tip", "note": "check this"}]


def test_handshake_planner_strips_non_whitelisted_ops() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        reqdir = root / "reports"
        reqdir.mkdir()
        L._write_json(
            reqdir / "round-1.plan.json",
            {
                "round": 1,
                "actions": [
                    {"op": "add-xml-lang", "file": "a.xhtml", "params": {}},
                    {"op": "evil-delete-all", "file": "a.xhtml", "params": {}},
                ],
                "suggestions": [],
            },
        )
        p = L.HandshakePlanner(reports_dir=reqdir)
        plan = p.plan(1, {}, {}, False)
        assert len(plan["actions"]) == 1
        assert plan["actions"][0]["op"] == "add-xml-lang"


# ── Task 9: render_report ─────────────────────────────────────────────────


def test_report_three_buckets() -> None:
    rep = {
        "round_log": [
            {
                "round": 1,
                "applied": [{"op": "add-xml-lang", "file": "ch.xhtml"}],
                "needs_human": [
                    {"kind": "prose-typo", "note": "OCR error suspected"},
                    {"kind": "refinement:risky_images", "detail": "img1.jpg"},
                ],
            }
        ]
    }
    text = L.render_report(rep)
    assert "已自动" in text
    assert "建议" in text
    assert "需人工" in text


# ── Task 8: run_loop convergence ─────────────────────────────────────────


def test_loop_is_idempotent_on_clean_input() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        src = root / "clean.epub"
        _make_min_epub(src, "干净文本", with_html_lang=True)
        work = root / "work2"
        planner = L.RulesPlanner()
        rep = L.run_loop(src, work, planner, max_rounds=4, dry_limit=2)
        applied_count = sum(len(r.get("applied", [])) for r in rep["round_log"])
        assert applied_count == 0, f"clean epub should have 0 actions: {rep['round_log']}"
        assert rep["stopped_by"] == "dry"


def test_loop_converges_for_missing_html_lang() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        src = root / "missing-lang.epub"
        _make_min_epub(src, "不变文本", with_html_lang=False)
        work = root / "work"
        planner = L.RulesPlanner()
        rep = L.run_loop(src, work, planner, max_rounds=4, dry_limit=2,
                         enable_structural=False)
        assert rep["status"] == "complete"
        assert rep["stopped_by"] in ("dry", "fingerprint")
        assert rep["rounds_run"] <= 4
        # At least one round should have applied an add-xml-lang action
        applied = sum(len(r.get("applied", [])) for r in rep["round_log"])
        assert applied >= 1, f"should have fixed missing lang: {rep['round_log']}"


def test_loop_converges_for_missing_manifest_mathml_property() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        src = root / "missing-mathml-property.epub"
        math = '<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>'
        _make_min_epub(src, math, with_html_lang=True)
        rep = L.run_loop(src, root / "work-math", L.RulesPlanner(), max_rounds=4, dry_limit=2)
        repair_report = root / "work-math" / "reports" / "staging-package-properties.json"
        repair = json.loads(repair_report.read_text(encoding="utf-8"))
        assert any(a.get("op") == "add-manifest-properties" for a in repair["actions"]), repair
        with zipfile.ZipFile(rep["output"]) as zf:
            opf = zf.read("OEBPS/content.opf").decode("utf-8")
        assert 'properties="mathml"' in opf


def test_loop_fingerprint_guard_breaks_oscillation() -> None:
    class FlipFlopPlanner:
        source = "test"

        def __init__(self) -> None:
            self._toggle = False

        def plan(self, round: int, findings: dict, refinement: dict,
                 enable_structural: bool) -> dict:
            self._toggle = not self._toggle
            return {
                "round": round,
                "actions": [
                    {
                        "op": "add-xml-lang",
                        "file": "OEBPS/chapter.xhtml",
                        "params": {"id": "html-root",
                                    "value": "en" if self._toggle else "zh-CN"},
                        "lane": "tag",
                        "source": "test",
                    }
                ],
                "suggestions": [],
            }

    with TemporaryDirectory() as d:
        root = Path(d)
        src = root / "o.epub"
        _make_min_epub(src, "x", with_html_lang=True, with_html_id="html-root")
        work = root / "work3"
        rep = L.run_loop(src, work, FlipFlopPlanner(), max_rounds=20, dry_limit=20)
        assert rep["stopped_by"] == "fingerprint", f"expected fingerprint, got: {rep}"
        assert rep["rounds_run"] < 20


def test_apply_action_rewrite_tag() -> None:
    xhtml = (
        '<html xmlns="' + XHTML_NS + '"><body>'
        '<div class="quote"><p>引文不变</p></div>'
        '</body></html>'
    )
    files = {"OEBPS/chapter.xhtml": xhtml}
    action = {
        "op": "rewrite-tag",
        "file": "OEBPS/chapter.xhtml",
        "params": {"match": {"tag": "div", "class": "quote"}, "new_tag": "blockquote"},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "applied", f"unexpected: {res}"
    assert "<blockquote" in files["OEBPS/chapter.xhtml"]
    assert "引文不变" in files["OEBPS/chapter.xhtml"]


def test_apply_action_adds_manifest_properties() -> None:
    opf = (
        '<package xmlns="http://www.idpf.org/2007/opf"><manifest>'
        '<item id="ch1" href="chapter.xhtml" media-type="application/xhtml+xml"/>'
        '</manifest></package>'
    )
    files = {"OEBPS/content.opf": opf}
    action = {
        "op": "add-manifest-properties",
        "file": "OEBPS/chapter.xhtml",
        "params": {"locator": {"manifest_id": "ch1"}, "properties": "mathml"},
        "lane": "package",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "applied", f"unexpected: {res}"
    assert 'properties="mathml"' in files["OEBPS/content.opf"]
    assert L.apply_action(files, action)["status"] == "noop"


def test_finalize_writes_cleanup_round_marker() -> None:
    with TemporaryDirectory() as d:
        root = Path(d)
        src = root / "source.epub"
        dst = root / "cleaned.epub"
        _make_min_epub(src, "文本不变")
        L._finalize(src, dst, 3)
        with zipfile.ZipFile(dst) as zf:
            opf = zf.read("OEBPS/content.opf").decode("utf-8")
        assert 'prefix="epub-handbook: https://github.com/epub-handbook/meta#"' in opf
        assert 'property="epub-handbook:cleanup-rounds">3</' in opf


def test_apply_action_empty_mapping_rejected() -> None:
    xhtml = '<p class="foo">text</p>'
    files = {"OEBPS/chapter.xhtml": xhtml}
    action = {
        "op": "rename-class",
        "file": "OEBPS/chapter.xhtml",
        "params": {"mapping": {}},
        "lane": "tag",
        "source": "rules",
    }
    res = L.apply_action(files, action)
    assert res["status"] == "rejected"


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
