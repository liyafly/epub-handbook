import os.path
import re

VIEWER = os.path.normpath(os.path.join(
    os.path.dirname(__file__), "..", "..", "font-coverage-viewer.html"
))


def _script(html: str) -> str:
    m = re.findall(r"<script>(.*?)</script>", html, re.S)
    assert m, "no <script> block found"
    return m[-1]


def test_loadFontFile_declaration_present():
    html = open(VIEWER, encoding="utf-8").read()
    assert "async function loadFontFile(blob,fname){" in html


def test_no_orphan_toplevel_await():
    js = _script(open(VIEWER, encoding="utf-8").read())
    head = js.find("async function loadFontFile(blob,fname){")
    aw0 = js.find("const buf=await blob.arrayBuffer()")
    assert head != -1 and aw0 != -1 and head < aw0
