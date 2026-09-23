"""Tiny synthetic fonts and EPUBs so the tests run offline.

Every font maps the same characters: space, A, a, 中, 文, 字, 、, 「 plus a UVS
entry 中 + U+E0100 -> uni4E2D.alt, and has a `vert` feature for 、 and 「.
Variable fonts have one axis wght 200..900 (default 400) and a STAT table.
"""

from __future__ import annotations

import io
import zipfile

from fontTools.fontBuilder import FontBuilder
from fontTools.misc.psCharStrings import T2CharString
from fontTools.pens.t2CharStringPen import T2CharStringPen
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.ttLib.tables.TupleVariation import TupleVariation

UVS_SELECTOR = 0xE0100
CMAP = {
    0x20: "space",
    0x41: "A",
    0x61: "a",
    0x4E2D: "uni4E2D",
    0x6587: "uni6587",
    0x5B57: "uni5B57",
    0x3001: "uni3001",
    0x300C: "uni300C",
}
EXTRA_GLYPHS = ["uni4E2D.alt", "uni3001.vert", "uni300C.vert"]
GLYPH_ORDER = [".notdef"] + list(CMAP.values()) + EXTRA_GLYPHS
UVS = [(0x4E2D, UVS_SELECTOR, "uni4E2D.alt")]
FEATURES = """
languagesystem DFLT dflt;
feature vert {
    sub uni3001 by uni3001.vert;
    sub uni300C by uni300C.vert;
} vert;
"""
AXES = [("wght", 200, 400, 900, "Weight")]
INSTANCES = [
    {"location": {"wght": 400}, "stylename": "Regular"},
    {"location": {"wght": 700}, "stylename": "Bold"},
]
STAT_AXES = [
    {"tag": "wght", "name": "Weight", "values": [
        {"value": 200, "name": "ExtraLight"},
        {"value": 400, "name": "Regular", "flags": 0x2},
        {"value": 600, "name": "SemiBold"},
        {"value": 700, "name": "Bold"},
        {"value": 900, "name": "Black"},
    ]},
]


def _box(index: int) -> tuple:
    """A distinct rectangle per glyph (x0, y0, x1, y1); space is empty."""
    return (50 + index * 3, 0, 550 - index * 3, 700 - index * 5)


def _common(fb: FontBuilder, family: str) -> None:
    fb.setupCharacterMap(CMAP, uvs=UVS)
    fb.setupHorizontalMetrics({name: (600, 50) for name in GLYPH_ORDER})
    fb.setupHorizontalHeader(ascent=880, descent=-120)
    fb.setupNameTable({"familyName": family, "styleName": "Regular"})
    fb.setupOS2(sTypoAscender=880, sTypoDescender=-120, usWinAscent=880, usWinDescent=120, usWeightClass=400)
    fb.setupPost()
    fb.addOpenTypeFeatures(FEATURES)


def _add_variation_tables(fb: FontBuilder) -> None:
    fb.setupFvar(AXES, INSTANCES)
    fb.setupStat(STAT_AXES)


def build_glyf_font(variable: bool = True) -> bytes:
    fb = FontBuilder(1000, isTTF=True)
    fb.setupGlyphOrder(GLYPH_ORDER)
    glyphs, variations = {}, {}
    for index, name in enumerate(GLYPH_ORDER):
        pen = TTGlyphPen(None)
        if name != "space":
            x0, y0, x1, y1 = _box(index)
            pen.moveTo((x0, y0))
            pen.lineTo((x0, y1))
            pen.lineTo((x1, y1))
            pen.lineTo((x1, y0))
            pen.closePath()
            # at wght=900 the box gets 40 units wider on each side (+4 phantom points)
            deltas = [(-40, 0), (-40, 0), (40, 0), (40, 0), (0, 0), (0, 0), (0, 0), (0, 0)]
            variations[name] = [TupleVariation({"wght": (0, 1.0, 1.0)}, deltas)]
        glyphs[name] = pen.glyph()
    fb.setupGlyf(glyphs)
    _common(fb, "SynthGlyf")
    if variable:
        _add_variation_tables(fb)
        fb.setupGvar(variations)
    return _save(fb)


def build_cff2_font() -> bytes:
    fb = FontBuilder(1000, isTTF=False)
    fb.setupGlyphOrder(GLYPH_ORDER)
    charstrings = {}
    for index, name in enumerate(GLYPH_ORDER):
        if name == "space":
            pen = T2CharStringPen(None, None, CFF2=True)
            charstrings[name] = pen.getCharString()
            continue
        x0, y0, x1, y1 = _box(index)
        w, h = x1 - x0, y1 - y0
        # rmoveto x0 y0 (x0 blends -40), hlineto w (+80), vlineto h, hlineto -w (-80)
        program = [x0, y0, -40, 0, 2, "blend", "rmoveto",
                   w, 80, 1, "blend", "hlineto",
                   h, "vlineto",
                   -w, -80, 1, "blend", "hlineto"]
        charstrings[name] = T2CharString(program=program)
    _common(fb, "SynthCFF2")
    _add_variation_tables(fb)
    fb.setupCFF2(charstrings, regions=[{"wght": (0, 1.0, 1.0)}])
    return _save(fb)


def build_math_font() -> bytes:
    """Static glyf font with a (dummy) MATH table; the tool must refuse it."""
    from fontTools.ttLib import TTFont
    from fontTools.ttLib.tables.DefaultTable import DefaultTable

    font = TTFont(io.BytesIO(build_glyf_font(variable=False)))
    math = DefaultTable("MATH")
    math.data = b"\x00\x01\x00\x00" + b"\x00" * 6
    font["MATH"] = math
    out = io.BytesIO()
    font.save(out)
    return out.getvalue()


def build_font_for(chars: str, empty: str = "", uvs: tuple = ()) -> bytes:
    """Static glyf font mapping every char in `chars`; chars in `empty` get a glyph without outline.
    uvs: ((base_cp, selector_cp), ...) added to cmap format 14 as default variations."""
    order = [".notdef"] + [f"g{ord(ch):05X}" for ch in chars]
    fb = FontBuilder(1000, isTTF=True)
    fb.setupGlyphOrder(order)
    glyphs = {}
    for index, name in enumerate(order):
        pen = TTGlyphPen(None)
        ch = chr(int(name[1:], 16)) if name != ".notdef" else ""
        if ch not in empty and not ch.isspace():
            x0, y0, x1, y1 = _box(index % 50)
            pen.moveTo((x0, y0))
            pen.lineTo((x0, y1))
            pen.lineTo((x1, y1))
            pen.lineTo((x1, y0))
            pen.closePath()
        glyphs[name] = pen.glyph()
    fb.setupGlyf(glyphs)
    fb.setupCharacterMap({ord(ch): f"g{ord(ch):05X}" for ch in chars},
                         uvs=[(base, sel, None) for base, sel in uvs] or None)
    fb.setupHorizontalMetrics({name: (600, 50) for name in order})
    fb.setupHorizontalHeader(ascent=880, descent=-120)
    fb.setupNameTable({"familyName": "SynthCheck", "styleName": "Regular"})
    fb.setupOS2()
    fb.setupPost()
    return _save(fb)


def _save(fb: FontBuilder) -> bytes:
    fb.font["head"].created = fb.font["head"].modified = 0x7B9B1F80  # fixed timestamp
    out = io.BytesIO()
    fb.save(out)
    return out.getvalue()


CHAPTER = """<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><title>第一章</title><link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
<style>.q::before {{ content: "\\201C"; }}</style>
<script>var hidden = "脚本里的字不应收集";</script></head>
<body>
<h1>中文</h1>
<p>中{uvs}、「A」<img src="../Images/x.png" alt="图"/><span title="标题" style="--x: '注'">正文</span></p>
<p><![CDATA[数据]]></p>
</body>
</html>
"""
NAV = """<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>目录</title></head>
<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">导航</a></li></ol></nav></body>
</html>
"""
NCX = """<?xml version="1.0" encoding="utf-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
<head/><docTitle><text>书名</text></docTitle>
<navMap><navPoint id="p1" playOrder="1"><navLabel><text>章节</text></navLabel><content src="Text/chapter.xhtml"/></navPoint></navMap>
</ncx>
"""
CSS = """@charset "utf-8";
@font-face { font-family: "st-all"; font-weight: 400; src: url("../Fonts/st-all.ttf"); }
@font-face { font-family: "st-all"; font-weight: 600; src: url(../Fonts/st-all-semibold.ttf); }
@font-face { font-family: "kt"; src: url("../Fonts/kt.otf"); }
body { font-family: "st-all", serif; }
h1 { font-weight: 600; }
.note::after { content: "\\3010注\\3011"; }
/* "注释里的字不应收集" */
"""
CONTAINER = """<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
"""


def build_epub(fonts: dict, encrypted: tuple = (), extra_manifest: str = "",
               chapter: str | None = None, css: str | None = None) -> bytes:
    """fonts: {"OEBPS/Fonts/x.ttf": bytes}. Every font gets a manifest item."""
    items = []
    for index, path in enumerate(sorted(fonts)):
        href = path[len("OEBPS/"):]
        media = "font/otf" if path.endswith(".otf") else "font/ttf"
        items.append(f'<item id="font{index}" href="{href}" media-type="{media}"/>')
    opf = f"""<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789abc</dc:identifier>
<dc:title>Synth</dc:title><dc:language>zh-CN</dc:language><meta property="dcterms:modified">2026-01-01T00:00:00Z</meta></metadata>
<manifest>
<item id="nav" href="Text/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="ch1" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
<item id="css" href="Styles/fonts.css" media-type="text/css"/>
<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
{''.join(items)}{extra_manifest}
</manifest>
<spine toc="ncx"><itemref idref="ch1"/></spine>
</package>
"""
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as zf:
        zf.writestr(zipfile.ZipInfo("mimetype", (2026, 1, 1, 0, 0, 0)), "application/epub+zip",
                    compress_type=zipfile.ZIP_STORED)
        entries = {
            "META-INF/container.xml": CONTAINER,
            "OEBPS/content.opf": opf,
            "OEBPS/toc.ncx": NCX,
            "OEBPS/Text/nav.xhtml": NAV,
            "OEBPS/Text/chapter.xhtml": chapter if chapter is not None else CHAPTER.format(uvs=chr(UVS_SELECTOR)),
            "OEBPS/Styles/fonts.css": css if css is not None else CSS,
        }
        if encrypted:
            refs = "".join(
                f'<enc:EncryptedData><enc:CipherData><enc:CipherReference URI="{p}"/></enc:CipherData></enc:EncryptedData>'
                for p in encrypted
            )
            entries["META-INF/encryption.xml"] = (
                '<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" '
                f'xmlns:enc="http://www.w3.org/2001/04/xmlenc#">{refs}</encryption>'
            )
        for name, text in entries.items():
            zf.writestr(zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0)), text.encode("utf-8"),
                        compress_type=zipfile.ZIP_DEFLATED)
        for name, data in sorted(fonts.items()):
            zf.writestr(zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0)), data, compress_type=zipfile.ZIP_DEFLATED)
    return buffer.getvalue()
