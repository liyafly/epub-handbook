import io
import zipfile
from dataclasses import dataclass

from epub_font.coverage.fixed_roles import check_book, find_fixed_role_missing


@dataclass
class Seg:
    family: str
    embedded: bool = False
    file: str | None = None
    generic: bool = False
    defaulted: bool = False
    system_ref: bool = False


def font(path, chars):
    return {"cmap": {ord(c) for c in chars}, "file_path": path}


INDEX = {"kt.ttf": font("OEBPS/kt.ttf", "信纸"), "en.ttf": font("OEBPS/en.ttf", "abc2026")}
KT = Seg("kt", True, "kt.ttf")
EN = Seg("en", True, "en.ttf")
SERIF = Seg("serif", generic=True)


def missing(text, *chain):
    return find_fixed_role_missing([{"text": text, "file": "a.xhtml"}], {"c": list(chain)}, {0: "c"}, INDEX)


def test_reports_character_missing_from_role_font():
    result = missing("信缺缺", KT, SERIF)
    assert [(e["char"], e["codepoint"], e["family"], e["count"]) for e in result] == [("缺", "U+7F3A", "kt", 2)]
    assert result[0]["font"] == "OEBPS/kt.ttf" and result[0]["first_location"] == "a.xhtml"


def test_checks_ascii_and_punctuation():
    assert [e["char"] for e in missing("信 9!", KT, SERIF)] == ["!", "9"]


def test_later_embedded_font_counts():
    assert missing("abc信", EN, KT, SERIF) == []
    assert [e["char"] for e in missing("abc缺", EN, KT, SERIF)] == ["缺"]


def test_chain_starting_with_system_font_is_exempt():
    assert missing("缺", Seg("Georgia"), EN, SERIF) == []


def test_free_body_is_exempt():
    assert missing("缺", Seg("serif", generic=True, defaulted=True)) == []


def test_ignores_spaces_format_characters_and_variation_selectors():
    assert missing("信 　 ​‍︀", KT, SERIF) == []


def book_zip(files):
    data = io.BytesIO()
    with zipfile.ZipFile(data, "w") as zf:
        for name, content in files.items():
            zf.writestr(name, content)
    data.seek(0)
    return zipfile.ZipFile(data)


PAGE = '<html><head>{}</head><body><p class="letter">{}</p></body></html>'
ROLE_CSS = b'.letter { font-family: "kt", serif; }'


def run_check(files, runs):
    zf = book_zip(files)
    book = {
        "font_files": [{"resolved_path": "OEBPS/kt.ttf"}],
        "xhtml_docs": [{"href": name, "resolved_path": "OEBPS/" + name} for name in {r["file"] for r in runs}],
    }
    css = [{"resolved_path": n, "content_bytes": c} for n, c in files.items() if n.endswith(".css")]
    faces = {"kt": {"src": "OEBPS/kt.ttf", "system_ref": False}}
    index = {"OEBPS/kt.ttf": font("OEBPS/kt.ttf", "信")}
    return check_book(zf, book, runs, css, faces, index)


def run_of(name):
    ancestors = [{"tag": "html"}, {"tag": "body"}, {"tag": "p", "classes": ["letter"], "id": None, "attrs": {}}]
    return {"text": "信缺", "file": name, "ancestor_chain": ancestors, "inline_style": ""}


def test_page_that_loads_the_role_css_fails():
    link = '<link rel="stylesheet" href="role.css"/>'
    files = {"OEBPS/a.xhtml": PAGE.format(link, "信缺"), "OEBPS/role.css": ROLE_CSS}
    assert [e["char"] for e in run_check(files, [run_of("a.xhtml")])] == ["缺"]


def test_page_that_does_not_load_the_role_css_passes():
    files = {"OEBPS/a.xhtml": PAGE.format("", "信缺"), "OEBPS/role.css": ROLE_CSS}
    assert run_check(files, [run_of("a.xhtml")]) == []


def test_imported_stylesheet_counts():
    link = '<link rel="stylesheet" href="main.css"/>'
    files = {
        "OEBPS/a.xhtml": PAGE.format(link, "信缺"),
        "OEBPS/main.css": b'@import url("role.css");',
        "OEBPS/role.css": ROLE_CSS,
    }
    assert [e["char"] for e in run_check(files, [run_of("a.xhtml")])] == ["缺"]


def test_inline_style_element_counts():
    style = '<style>.letter { font-family: "kt", serif; }</style>'
    files = {"OEBPS/a.xhtml": PAGE.format(style, "信缺")}
    assert [e["char"] for e in run_check(files, [run_of("a.xhtml")])] == ["缺"]
