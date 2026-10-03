from __future__ import annotations

import io
import json
import zipfile

import pytest

from epub_font import check, epubtext, font_usage, subset
from tests import synth

BODY = "OEBPS/Fonts/body.ttf"
TITLE = "OEBPS/Fonts/title.ttf"
CHAPTER = """<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>第一章</title><link rel="stylesheet" href="../Styles/fonts.css"/></head>
<body><h1>乙标题</h1><p>甲正文内容</p></body>
</html>
"""
CSS = """@font-face { font-family: BodyFace; src: url('../Fonts/body.ttf'); }
@font-face { font-family: TitleFace; src: url('../Fonts/title.ttf'); }
body { font-family: BodyFace; }
h1 { font-family: TitleFace; }
"""


def build_plan(chapter: str = CHAPTER, css: str = CSS, mode: str = "css"):
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False), TITLE: synth.build_glyf_font(variable=False)},
        chapter=chapter,
        css=css,
    )
    zf = zipfile.ZipFile(io.BytesIO(epub))
    book = epubtext.read_book_text(zf)
    _items, independent = check.harvest_book(zf)
    plan = font_usage.build_plan(mode, zf, book, independent, [BODY, TITLE])
    return zf, book, independent, plan


def with_imported_stylesheet(epub: bytes, main_css: str, imported_css: str) -> bytes:
    source_entries = []
    with zipfile.ZipFile(io.BytesIO(epub)) as source:
        for info in source.infolist():
            data = source.read(info.filename)
            if info.filename == "OEBPS/Styles/fonts.css":
                data = main_css.encode("utf-8")
            elif info.filename == "OEBPS/content.opf":
                opf = data.decode("utf-8").replace(
                    "</manifest>",
                    '<item id="extra-css" href="Styles/extra.css" media-type="text/css"/></manifest>',
                )
                data = opf.encode("utf-8")
            source_entries.append((info, data))
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as destination:
        for info, data in source_entries:
            destination.writestr(info, data)
        destination.writestr("OEBPS/Styles/extra.css", imported_css)
    return output.getvalue()


def test_css_plan_assigns_text_to_declared_font_roles_and_inherits():
    zf, book, _independent, plan = build_plan()
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]

    assert plan.mode == "css"
    assert body.mode == title.mode == "css"
    assert ord("甲") in body.codepoints and ord("甲") not in title.codepoints
    assert ord("乙") in title.codepoints and ord("乙") not in body.codepoints
    assert set().union(*(set(map(chr, usage.codepoints)) for usage in plan.fonts.values())) == book.all_chars()
    assert len(title.codepoints) < len(book.all_chars())


def test_inline_style_font_family_assigns_element_text():
    chapter = CHAPTER.replace("<h1>乙标题</h1>", '<h1 style="font-family: TitleFace">乙标题</h1>')
    css = CSS.replace("h1 { font-family: TitleFace; }", "")
    zf, _book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]
    assert body.mode == title.mode == "css"
    assert ord("乙") in title.codepoints and ord("乙") not in body.codepoints


def test_child_selector_with_compound_type_id_and_class_assigns_role():
    chapter = CHAPTER.replace("<h1>乙标题</h1>", '<h1 id="chapter-title" class="title">乙标题</h1>')
    css = CSS.replace("h1 { font-family: TitleFace; }", "body > h1#chapter-title.title { font-family: TitleFace; }")
    zf, _book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]
    assert title.mode == body.mode == "css"
    assert ord("乙") in title.codepoints and ord("乙") not in body.codepoints


def test_descendant_selector_assigns_role_to_matching_element():
    chapter = CHAPTER.replace("<h1>乙标题</h1>", '<h1 id="chapter-title" class="title">乙标题</h1>')
    css = CSS.replace("h1 { font-family: TitleFace; }", "html h1#chapter-title.title { font-family: TitleFace; }")
    zf, _book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]
    assert title.mode == body.mode == "css"
    assert ord("乙") in title.codepoints and ord("乙") not in body.codepoints


def test_conditional_import_keeps_inherited_font_as_a_possible_role():
    root_css = '@import "extra.css" screen;\n' + CSS.replace("h1 { font-family: TitleFace; }", "")
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False), TITLE: synth.build_glyf_font(variable=False)},
        chapter=CHAPTER,
        css=CSS,
    )
    epub = with_imported_stylesheet(epub, root_css, "h1 { font-family: TitleFace; }")
    zf = zipfile.ZipFile(io.BytesIO(epub))
    book = epubtext.read_book_text(zf)
    _items, independent = check.harvest_book(zf)
    plan = font_usage.build_plan("css", zf, book, independent, [BODY, TITLE])
    with zf:
        body = plan.fonts[BODY]

    assert ord("乙") in body.codepoints


def test_imported_fullwidth_transform_variants_are_retained():
    chapter = CHAPTER.replace("乙标题", "A标题")
    root_css = '@import "extra.css";\n' + CSS.replace("h1 { font-family: TitleFace; }", "")
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False), TITLE: synth.build_glyf_font(variable=False)},
        chapter=chapter,
        css=CSS,
    )
    epub = with_imported_stylesheet(
        epub, root_css, "h1 { font-family: TitleFace; text-transform: full-width; }"
    )
    zf = zipfile.ZipFile(io.BytesIO(epub))
    book = epubtext.read_book_text(zf)
    _items, independent = check.harvest_book(zf)
    plan = font_usage.build_plan("css", zf, book, independent, [BODY, TITLE])
    with zf:
        title = plan.fonts[TITLE]

    assert ord("Ａ") in title.codepoints


def test_multicodepoint_case_transform_results_are_retained():
    chapter = CHAPTER.replace("乙标题", "ǰ标题")
    css = CSS + "h1 { text-transform: uppercase; }\n"
    zf, _book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        title = plan.fonts[TITLE]
    assert ord("J") in title.codepoints
    assert ord("\u030c") in title.codepoints


def test_xml_stylesheet_processing_instruction_refuses_css_mode():
    chapter = CHAPTER.replace(
        "<html xmlns=", '<?xml-stylesheet type="text/css" href="../Styles/fonts.css"?>\n<html xmlns='
    )
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False), TITLE: synth.build_glyf_font(variable=False)},
        chapter=chapter,
        css=CSS,
    )
    zf = zipfile.ZipFile(io.BytesIO(epub))
    book = epubtext.read_book_text(zf)
    _items, independent = check.harvest_book(zf)
    with zf, pytest.raises(font_usage.UnsafeUsageError, match="XML stylesheet processing instructions"):
        font_usage.build_plan("css", zf, book, independent, [BODY, TITLE])


def test_unsupported_selector_broadens_only_the_affected_font():
    chapter = CHAPTER.replace("乙标题", "乙中\U000E0100标题")
    css = CSS + "h1:nth-child(1) { font-family: TitleFace; }\n"
    zf, book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        title = plan.fonts[TITLE]
        body = plan.fonts[BODY]

    assert title.mode == "book"
    assert title.codepoints == set(map(ord, book.all_chars()))
    assert body.mode == "css"
    assert any("unsupported selector" in reason for reason in plan.fallback_reasons)
    assert (ord("中"), 0xE0100) in title.harvest.sequences


def test_supported_font_shorthand_keeps_its_font_family_scope():
    css = CSS + "p { font: normal 14px BodyFace; }\n"
    zf, book, _independent, plan = build_plan(css=css)
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]
    assert body.mode == title.mode == "css"
    assert ord("甲") in body.codepoints and ord("甲") not in title.codepoints
    assert set().union(*(set(map(chr, usage.codepoints)) for usage in plan.fonts.values())) == book.all_chars()


def test_inherit_font_family_uses_the_parent_role():
    css = CSS + "p { font-family: inherit; }\n"
    zf, _book, _independent, plan = build_plan(css=css)
    with zf:
        body = plan.fonts[BODY]
        title = plan.fonts[TITLE]
    assert body.mode == title.mode == "css"
    assert ord("甲") in body.codepoints and ord("甲") not in title.codepoints


def test_unknown_font_family_value_falls_back_to_whole_book():
    css = CSS + "body { font-family: var(--body-font); }\n"
    zf, book, _independent, plan = build_plan(css=css)
    with zf:
        assert all(usage.mode == "book" for usage in plan.fonts.values())
        assert all(usage.codepoints == set(map(ord, book.all_chars())) for usage in plan.fonts.values())


@pytest.mark.parametrize("invalid_css,expected_modes", [
    ("p { font-family: inherit, TitleFace; }", ("book", "book")),
    ("p { font: 20deg BodyFace; }", ("book", "book")),
    ("@font-face { font-family: BodyFace, TitleFace; src: url('../Fonts/body.ttf'); }", ("book", "book")),
    ("*p { font-family: TitleFace; }", ("css", "book")),
])
def test_invalid_or_unprovable_css_broadens_before_subset(invalid_css, expected_modes):
    zf, book, _independent, plan = build_plan(css=CSS + invalid_css)
    with zf:
        assert tuple(plan.fonts[target].mode for target in (BODY, TITLE)) == expected_modes
        for target, usage in plan.fonts.items():
            if usage.mode == "book":
                assert usage.codepoints == set(map(ord, book.all_chars()))


def test_css_content_attr_keeps_all_possible_attribute_characters():
    chapter = CHAPTER.replace("<p>甲正文内容</p>", '<p data-code="丙">甲正文内容</p>')
    css = CSS + 'p::before { content: attr(data-code); font-family: BodyFace; }\n'
    zf, _book, _independent, plan = build_plan(chapter=chapter, css=css)
    with zf:
        assert all(ord("丙") in usage.codepoints for usage in plan.fonts.values())


def test_custom_counter_symbols_are_added_to_every_possible_font():
    css = CSS + '@counter-style custom-marker { system: cyclic; symbols: "¤" "§"; suffix: "!"; }\n'
    zf, _book, _independent, plan = build_plan(css=css)
    with zf:
        assert all({ord("¤"), ord("§"), ord("!")} <= usage.codepoints for usage in plan.fonts.values())


@pytest.mark.parametrize("generated_css", [
    "ol { list-style-type: hiragana; }",
    'p::before { content: counter(chapter, hiragana); }',
    'h1::before { content: var(--generated-label); }',
    "q { quotes: var(--quotes); }",
    "p { text-emphasis-style: var(--mark); }",
])
def test_untracked_generated_glyphs_refuse_css_mode(generated_css):
    zf, book, independent, _plan = build_plan(css=CSS + generated_css, mode="book")
    with zf, pytest.raises(font_usage.UnsafeUsageError):
        font_usage.build_plan("css", zf, book, independent, [BODY, TITLE])


def test_list_style_image_url_does_not_block_css_mode():
    zf, _book, _independent, plan = build_plan(css=CSS + "ol { list-style: url('../Images/bullet.png') inside; }")
    with zf:
        assert all(usage.mode == "css" for usage in plan.fonts.values())


def test_fallback_reason_report_is_bounded_to_provider_contract():
    zf, book, _independent, _plan = build_plan()
    with zf:
        planner = font_usage._Planner(zf, book, [BODY])
        planner.reason("x" * 5000)
        for index in range(100):
            planner.reason(f"fallback-{index}")

    assert len(planner.fallback_reasons) == 64
    assert all(0 < len(reason) <= 4096 for reason in planner.fallback_reasons)
    assert planner.fallback_reasons[-1] == "Additional fallback reasons omitted (limit 64)"


def test_css_mode_refuses_scripted_content_without_writing_output(tmp_path, capsys):
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False)},
        chapter=CHAPTER.replace("</body>", '<script>document.body.textContent="丁";</script></body>'),
        css="""@font-face { font-family: BodyFace; src: url('../Fonts/body.ttf'); }
body { font-family: BodyFace; }
""",
    )
    input_path = tmp_path / "scripted-source.epub"
    config_path = tmp_path / "scripted-fonts.json"
    output_path = tmp_path / "scripted-subset.epub"
    input_path.write_bytes(epub)
    config_path.write_text(json.dumps({"version": 1, "usage": "css", "fonts": [{"target": BODY}]}))

    code = subset.main([str(input_path), "--out", str(output_path), "--config", str(config_path)])

    assert code == 2
    assert "scripted content" in capsys.readouterr().err
    assert not output_path.exists() and not subset.report_path(output_path).exists()


def test_css_mode_refuses_executable_event_attributes():
    chapter = CHAPTER.replace("<body>", '<body onload="document.body.textContent=\'丁\'">')
    zf, book, independent, _plan = build_plan(chapter=chapter, mode="book")
    with zf, pytest.raises(font_usage.UnsafeUsageError, match="event attribute"):
        font_usage.build_plan("css", zf, book, independent, [BODY, TITLE])


def test_css_subset_is_smaller_and_independently_checked(tmp_path, capsys):
    epub = synth.build_epub(
        {BODY: synth.build_glyf_font(variable=False), TITLE: synth.build_glyf_font(variable=False)},
        chapter=CHAPTER,
        css=CSS,
    )
    input_path = tmp_path / "source.epub"
    input_path.write_bytes(epub)
    reports = {}
    for mode in ("book", "css"):
        config = tmp_path / f"{mode}.json"
        config.write_text(json.dumps({"version": 1, "usage": mode, "fonts": [{"target": BODY}, {"target": TITLE}]}))
        output = tmp_path / f"{mode}.epub"
        assert subset.main([str(input_path), "--out", str(output), "--config", str(config)]) == 0, capsys.readouterr()
        reports[mode] = json.loads(subset.report_path(output).read_text())

    assert reports["css"]["schemaVersion"] == 3
    assert reports["css"]["usage"]["mode"] == "css"
    css_fonts = {font["target"]: font for font in reports["css"]["fonts"]}
    book_fonts = {font["target"]: font for font in reports["book"]["fonts"]}
    assert all(font["checks"]["independent-coverage"]["ok"] for font in css_fonts.values())
    assert css_fonts[TITLE]["usage"]["mode"] == "css"
    assert css_fonts[TITLE]["output"]["bytes"] < book_fonts[TITLE]["output"]["bytes"]


def test_css_usage_keeps_variable_font_instance_checks(tmp_path, capsys):
    variable_css = """@font-face { font-family: BodyFace; src: url('../Fonts/body.ttf'); }
body { font-family: BodyFace; }
"""
    epub = synth.build_epub({BODY: synth.build_glyf_font(variable=True)}, chapter=CHAPTER, css=variable_css)
    input_path = tmp_path / "variable-source.epub"
    config_path = tmp_path / "variable-fonts.json"
    output_path = tmp_path / "variable-subset.epub"
    input_path.write_bytes(epub)
    config_path.write_text(json.dumps({
        "version": 1,
        "usage": "css",
        "fonts": [{"target": BODY, "variation": {"mode": "instance", "axes": {"wght": 400}}}],
    }))

    code = subset.main([str(input_path), "--out", str(output_path), "--config", str(config_path)])
    report = json.loads(subset.report_path(output_path).read_text())

    assert code == 0, capsys.readouterr()
    font = report["fonts"][0]
    assert font["usage"]["mode"] == "css"
    assert font["variation"]["mode"] == "instance"
    assert font["output"]["axes"] == []
    assert all(result["ok"] for result in font["checks"].values())
