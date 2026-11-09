#!/usr/bin/env python3
"""Build the font-role probe EPUBs from locally supplied fonts; no font bytes in git."""

import argparse
import hashlib
import html
import io
import json
from pathlib import Path
import uuid
import zipfile

from fontTools import subset
from fontTools.ttLib import TTFont


ROOT = Path(__file__).resolve().parent
RECIPE = ROOT.parent / "recipes" / "letter-paper"
MISSING = "缺"  # Deliberately removed from the letter font to expose its fallback.

# role -> (CSS family, CSS style, marker code point, marker glyph taken from the font)
ROLES = {
    "st": ("st", "normal", 0xE100, "正"),
    "letter": ("tszt-letter", "normal", 0xE101, "信"),
    "verse": ("tszt-verse", "normal", 0xE102, "诗"),
    "title": ("tszt-title", "normal", 0xE103, "题"),
    "note": ("tszt-note", "normal", 0xE104, "注"),
    "en": ("en", "normal", 0xE110, "R"),
    "en-italic": ("en", "italic", 0xE111, "I"),
    "en-plain": ("en-plain", "normal", 0xE112, "P"),
    "mono": ("mono", "normal", 0xE113, "M"),
}
LATIN_ROLES = {"en", "en-italic", "en-plain", "mono"}

SYSTEM_BODY = '"Songti SC", "SimSun", "Noto Serif CJK SC", serif'
VARIANTS = {
    "a-free": ("A 正文自由", None, False),
    "b-free-meta": ("B 正文自由加 meta", None, True),
    "c-locked": ("C 正文锁定嵌入字体", '"st", serif', True),
    "d-system": ("D 正文系统字体链", SYSTEM_BODY, True),
    "letter": ("书信成品", None, False),
}


def mark(role: str) -> str:
    return chr(ROLES[role][2])


PARA = "最近工作还顺利吗？我在学校一切都好，请您放心。上周的考试我进步了不少，老师还表扬了我。只是最近天气转凉，您在家要注意保暖，别太操劳。等放假了我就回家看您。"


def letter(*body: str, marked: bool = True) -> str:
    def para(text: str, cls: str = "") -> str:
        attr = f' class="{cls}"' if cls else ""
        return f'<p{attr}><span class="letter-rule">{text}</span></p>'

    gap = '<div class="letter-gap"></div>'
    marker = mark("letter") + " " if marked else ""
    paragraphs = "".join(para(text) for text in body)
    return f"""<div class="letter-paper">
{para(marker + "亲爱的妈妈：", "letter-salutation")}
{para("您好！")}
{paragraphs}
{gap}{para("祝您", "letter-closing")}
{para("身体健康，万事如意！", "letter-wish")}
{gap}<div class="letter-signoff">{para("您的儿子：小明", "letter-signature")}{para("2026年11月9日", "letter-date")}</div>
{gap}</div>"""


PAGES = {
    "card": ("怎么看", f"""<h1>怎么看</h1>
<p>每个角色行开头有一个标记字。标记字只存在于该角色的嵌入字体里，所以：</p>
<ul>
<li>看到标记字：这个嵌入字体生效了。</li>
<li>看到方块、问号或空白：嵌入字体没有生效。</li>
</ul>
<p>先用阅读器默认设置看一遍，再把阅读器字体换成别的字体，翻回来看第二遍。标记字消失的角色，就是被阅读器换字覆盖了。</p>
<p>正文标记字只有 C 包应该显示；A、B、D 包没有嵌入正文字体，显示方块才是对的。</p>"""),
    "roles": ("角色字体", f"""<h1>{mark("title")} T1 标题角色</h1>
<p>{mark("st")} B1 正文。山川草木，风雨晦明，皆可入书。这一段跟随正文字体设置，换字时应该变化。English words stay here too.</p>
<p class="role-letter">{mark("letter")} L1 书信角色。见字如面，纸短情长，换正文字体时不应该变。</p>
<p class="role-verse">{mark("verse")} V1 诗歌角色。春水初生，春林初盛，换正文字体时不应该变。</p>
<p class="role-note">{mark("note")} N1 注释角色。此处为注释文字，换正文字体时不应该变。</p>
<p class="role-letter">{mark("letter")} S1 只有常规字重时的<strong>粗体观察</strong>，记录是否变成机器加粗。</p>"""),
    "letter": ("书信版式", f"""<h1>书信版式</h1>
{letter(*PARA.replace("只是", "\n只是").split("\n"))}
<h2>跨页长信</h2>
{letter(PARA, PARA * 3, PARA * 2)}
<h2>长信，段落超过一页</h2>
{letter(PARA, PARA * 12)}"""),
    "missing": ("缺字观察", f"""<h1>缺字观察</h1>
<p>下一行括号里的字已从书信字体里删除。记录它显示成什么：系统字体、方块还是空白。</p>
<p class="role-letter">{mark("letter")} M1 书信字体没有的字：（{MISSING}）前后文字仍是书信字体。</p>"""),
    "english": ("英文字体", f"""<h1 xml:lang="en" lang="en">English fonts</h1>
<p class="e-missing" xml:lang="en" lang="en">{mark("en")} E1 No such system font, so the embedded font should appear.</p>
<p class="e-system" xml:lang="en" lang="en">{mark("en")} E2 Georgia first, then embedded. The text should look like Georgia if the reader has it.</p>
<p class="e-apple" xml:lang="en" lang="en">{mark("en")} E3 Iowan Old Style, Palatino, then embedded.</p>
<p class="e-missing" xml:lang="en" lang="en"><i>{mark("en-italic")} E4 Embedded font with a true italic face: a, e, f and g change shape.</i></p>
<p class="e-plain" xml:lang="en" lang="en"><i>{mark("en-plain")} E5 Embedded font without an italic face: letters only lean.</i></p>
<p class="e-system" xml:lang="en" lang="en"><i>E6 System font italic through Georgia, if the reader has it.</i></p>
<p class="e-code" xml:lang="en" lang="en">{mark("mono")} E7 code: SF Mono, Consolas, then embedded; 0Oo 1lI</p>
<p class="e-missing"><i>E8 English 夹中文的斜体，记录中文是否被斜切</i></p>"""),
}


SHORT = PARA.replace("只是", "\n只是").split("\n")
LETTER_PAGES = {
    "letter": ("书信", f"""<h1>短信</h1>
{letter(*SHORT, marked=False)}
<h1>多段信，跨页</h1>
{letter(PARA, PARA * 2, PARA * 3, PARA * 2, PARA, marked=False)}
<h1>长信，段落超过一页</h1>
{letter(PARA, PARA * 12, PARA, marked=False)}"""),
}


FONTS_CSS = {
    "letter": ".letter-paper, .letter-paper p, .role-letter { font-family: \"tszt-letter\", serif; }",
    "verse": ".role-verse { font-family: \"tszt-verse\", serif; }",
    "title": "h1 { font-family: \"tszt-title\", sans-serif; }",
    "note": ".role-note { font-family: \"tszt-note\", serif; }",
    "en": ".e-missing { font-family: \"No Such Probe Font\", \"en\", serif; }\n"
          ".e-system { font-family: Georgia, \"en\", serif; }\n"
          ".e-apple { font-family: \"Iowan Old Style\", Palatino, \"en\", serif; }",
    "en-plain": ".e-plain { font-family: \"en-plain\", serif; }",
    "mono": ".e-code { font-family: \"SF Mono\", Consolas, \"mono\", monospace; }",
}
BASE_CSS = """.role-letter, .role-verse, .role-note { margin: 1em 0; text-indent: 0; }
.role-verse { text-align: center; }
"""


def page_xhtml(title: str, body: str) -> bytes:
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN" lang="zh-CN">
<head>
  <title>{html.escape(title)}</title>
  <link rel="stylesheet" type="text/css" href="fonts.css"/>
  <link rel="stylesheet" type="text/css" href="base.css"/>
  <link rel="stylesheet" type="text/css" href="recipe.css"/>
  <link rel="stylesheet" type="text/css" href="reader.css"/>
</head>
<body>
{body}
</body>
</html>
""".encode()


def characters() -> str:
    """Every character the probe can render, for the CJK subsets."""
    pages = [*PAGES.values(), *LETTER_PAGES.values()]
    text = "".join(title + body for title, body in pages) + "".join(v[0] for v in VARIANTS.values())
    return "".join(sorted({c for c in text if not 0xE000 <= ord(c) <= 0xF8FF}))


def build_font(source: Path, role: str) -> tuple[bytes, str]:
    """Subset one font and map the role's private-use marker to its marker glyph."""
    code, glyph_char = ROLES[role][2], ROLES[role][3]
    if role in LATIN_ROLES:
        text = "".join(chr(c) for c in range(0x20, 0x7F)) + "’“”—"
    else:
        text = characters().replace(MISSING, "") if role == "letter" else characters()
    font = TTFont(source, recalcTimestamp=False, fontNumber=0)
    options = subset.Options()
    options.notdef_outline = True
    options.name_IDs = [1, 2]
    options.hinting = False
    subsetter = subset.Subsetter(options)
    subsetter.populate(text=text + glyph_char)
    subsetter.subset(font)
    glyph = font.getBestCmap()[ord(glyph_char)]
    for table in font["cmap"].tables:
        if table.isUnicode():
            table.cmap[code] = glyph
    if role == "letter" and ord(MISSING) in font.getBestCmap():
        raise SystemExit("letter subset must not contain the missing-glyph test character")
    output = io.BytesIO()
    font.save(output)
    return output.getvalue(), "otf" if font.sfntVersion == "OTTO" else "ttf"


def fonts_css(roles: list[str], names: dict[str, str], body: str | None) -> str:
    lines = ['@charset "utf-8";']
    for role in roles:
        family, style, *_ = ROLES[role]
        lines.append(f'@font-face {{ font-family: "{family}"; font-style: {style}; font-weight: 400; src: url("Fonts/{names[role]}"); }}')
    if body:
        lines.append(f"body {{ font-family: {body}; }}")
    lines += [css for role, css in FONTS_CSS.items() if role in roles]
    return "\n".join(lines) + "\n"


def package(variant: str, fonts: dict[str, tuple[bytes, str]], duokan: bool = False) -> dict:
    """duokan=True fills reader.css with the Duokan overlay; otherwise it stays empty."""
    label, body_chain, meta = VARIANTS[variant]
    pages = LETTER_PAGES if variant == "letter" else PAGES
    roles = ["letter"] if variant == "letter" else [role for role in ROLES if role != "st" or body_chain == '"st", serif']
    names = {role: f"{role}.{fonts[role][1]}" for role in roles}
    identity = uuid.uuid5(uuid.NAMESPACE_URL, "https://example.invalid/font-role-probe/" + variant + ("-duokan" if duokan else ""))
    title = html.escape(f"字体角色探针 {label}" + (" 多看" if duokan else ""))

    spine = "".join(f'<itemref idref="{name}"/>' for name in pages)
    points = "".join(
        f'<navPoint id="{name}" playOrder="{i}"><navLabel><text>{html.escape(item[0])}</text></navLabel><content src="{name}.xhtml"/></navPoint>'
        for i, (name, item) in enumerate(pages.items(), 1)
    )
    links = "".join(f'<li><a href="{name}.xhtml">{html.escape(item[0])}</a></li>' for name, item in pages.items())
    manifest = "".join(f'<item id="{name}" href="{name}.xhtml" media-type="application/xhtml+xml"/>' for name in pages)
    manifest += "".join(
        f'<item id="font-{role}" href="Fonts/{names[role]}" media-type="font/{fonts[role][1]}"/>' for role in roles
    )
    prefix = ' prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/"' if meta else ""
    meta_line = '<meta property="ibooks:specified-fonts">true</meta>' if meta else ""

    files = {
        "META-INF/container.xml": b'<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>',
        "OEBPS/nav.xhtml": f'<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN"><head><title>目录</title></head><body><nav epub:type="toc" id="toc"><h1>目录</h1><ol>{links}</ol></nav></body></html>'.encode(),
        "OEBPS/toc.ncx": f'<?xml version="1.0" encoding="UTF-8"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="urn:uuid:{identity}"/></head><docTitle><text>{title}</text></docTitle><navMap>{points}</navMap></ncx>'.encode(),
        "OEBPS/fonts.css": fonts_css(roles, names, body_chain).encode(),
        "OEBPS/base.css": BASE_CSS.encode(),
        "OEBPS/recipe.css": (RECIPE / "recipe.css").read_bytes(),
        "OEBPS/reader.css": (RECIPE / "reader-duokan.css").read_bytes() if duokan else b"/* Generic: reader overlay empty. */\n",
        "OEBPS/package.opf": f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid"{prefix}>
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="bookid">urn:uuid:{identity}</dc:identifier><dc:title>{title}</dc:title><dc:language>zh-CN</dc:language><meta property="dcterms:modified">2026-11-09T00:00:00Z</meta>{meta_line}</metadata>
<manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="fonts" href="fonts.css" media-type="text/css"/><item id="base" href="base.css" media-type="text/css"/><item id="recipe" href="recipe.css" media-type="text/css"/><item id="reader" href="reader.css" media-type="text/css"/>{manifest}</manifest>
<spine toc="ncx">{spine}</spine>
</package>
""".encode(),
    }
    for name, (page_title, body) in pages.items():
        files[f"OEBPS/{name}.xhtml"] = page_xhtml(page_title, body)
    for role in roles:
        files[f"OEBPS/Fonts/{names[role]}"] = fonts[role][0]

    output = ROOT / "dist" / f"font-role-{variant}{'-duokan' if duokan else ''}.epub"
    with zipfile.ZipFile(output, "w") as archive:
        for path, data in [("mimetype", b"application/epub+zip"), *sorted(files.items())]:
            info = zipfile.ZipInfo(path, (2026, 11, 9, 0, 0, 0))
            info.compress_type = zipfile.ZIP_STORED if path == "mimetype" else zipfile.ZIP_DEFLATED
            archive.writestr(info, data)
    return {"artifact": output.name, "sha256": hashlib.sha256(output.read_bytes()).hexdigest(), "readerStatus": "not-verified"}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fonts", required=True, type=Path, help="JSON mapping each role to a local font file; see fonts.example.json")
    config_path = parser.parse_args().fonts
    config = json.loads(config_path.read_text(encoding="utf-8"))
    missing = sorted(set(ROLES) - set(config))
    if missing:
        raise SystemExit(f"{config_path}: missing roles {missing}")
    fonts = {role: build_font((config_path.parent / config[role]).resolve(), role) for role in ROLES}
    (ROOT / "dist").mkdir(exist_ok=True)
    inventory = [package(variant, fonts) for variant in VARIANTS]
    inventory.append(package("letter", fonts, duokan=True))
    (ROOT / "dist" / "inventory.json").write_text(json.dumps(inventory, indent=2) + "\n", encoding="utf-8")
    for item in inventory:
        print(item["sha256"], item["artifact"])


if __name__ == "__main__":
    main()
