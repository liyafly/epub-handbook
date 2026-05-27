#!/usr/bin/env python3
"""Build self-authored EPUB pairs for cleanup and diff demos."""

from __future__ import annotations

import json
import re
import struct
import zipfile
import zlib
from dataclasses import dataclass
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT_DIR = ROOT / "samples" / "demo-books" / "dist"
FIXED_ZIP_TIME = (2026, 5, 27, 0, 0, 0)


@dataclass(frozen=True)
class Chapter:
  file_name: str
  title: str
  body: str


@dataclass(frozen=True)
class EpubSpec:
  slug: str
  variant: str
  title: str
  creator: str
  identifier: str
  chapters: tuple[Chapter, ...]
  css: dict[str, str]
  images: dict[str, bytes | str]
  output_name: str


def xml_id(value: str) -> str:
  cleaned = re.sub(r"[^A-Za-z0-9_-]+", "-", value).strip("-")
  if not cleaned or not cleaned[0].isalpha():
    cleaned = f"item-{cleaned or 'id'}"
  return cleaned


def zip_info(name: str, compression: int) -> zipfile.ZipInfo:
  info = zipfile.ZipInfo(name, FIXED_ZIP_TIME)
  info.compress_type = compression
  info.external_attr = 0o644 << 16
  return info


def write_epub(path: Path, files: dict[str, bytes]) -> None:
  path.parent.mkdir(parents=True, exist_ok=True)
  if path.exists():
    path.unlink()
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr(
      zip_info("mimetype", zipfile.ZIP_STORED),
      b"application/epub+zip",
    )
    for name in sorted(files):
      zf.writestr(zip_info(name, zipfile.ZIP_DEFLATED), files[name])


def container_xml() -> str:
  return """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
"""


def nav_xhtml(spec: EpubSpec) -> str:
  items = "\n".join(
    f'        <li><a href="Text/{chapter.file_name}">{chapter.title}</a></li>'
    for chapter in spec.chapters
  )
  return f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN" lang="zh-CN">
  <head>
    <title>{spec.title}目录</title>
  </head>
  <body>
    <nav epub:type="toc" id="toc">
      <h1>{spec.title}</h1>
      <ol>
{items}
      </ol>
    </nav>
    <nav epub:type="landmarks" hidden="">
      <h2>Landmarks</h2>
      <ol>
        <li><a epub:type="bodymatter" href="Text/{spec.chapters[0].file_name}">正文</a></li>
      </ol>
    </nav>
  </body>
</html>
"""


def toc_ncx(spec: EpubSpec) -> str:
  nav_points = []
  for index, chapter in enumerate(spec.chapters, start=1):
    nav_points.append(
      f"""    <navPoint id="navPoint-{index}" playOrder="{index}">
      <navLabel><text>{chapter.title}</text></navLabel>
      <content src="Text/{chapter.file_name}"/>
    </navPoint>"""
    )
  return f"""<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content="{spec.identifier}"/>
    <meta name="dtb:depth" content="1"/>
    <meta name="dtb:totalPageCount" content="0"/>
    <meta name="dtb:maxPageNumber" content="0"/>
  </head>
  <docTitle><text>{spec.title}</text></docTitle>
  <navMap>
{chr(10).join(nav_points)}
  </navMap>
</ncx>
"""


def chapter_xhtml(spec: EpubSpec, chapter: Chapter) -> str:
  css_links = "\n".join(
    f'    <link rel="stylesheet" type="text/css" href="../Styles/{name}"/>'
    for name in spec.css
  )
  return f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN" lang="zh-CN">
  <head>
    <title>{chapter.title}</title>
{css_links}
  </head>
  <body class="{spec.slug} {spec.variant}">
{chapter.body}
  </body>
</html>
"""


def media_type(path: str) -> str:
  if path.endswith(".xhtml"):
    return "application/xhtml+xml"
  if path.endswith(".css"):
    return "text/css"
  if path.endswith(".ncx"):
    return "application/x-dtbncx+xml"
  if path.endswith(".svg"):
    return "image/svg+xml"
  if path.endswith(".png"):
    return "image/png"
  raise ValueError(f"unknown media type for {path}")


def package_opf(spec: EpubSpec) -> str:
  manifest: list[str] = [
    '    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>',
    '    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>',
  ]
  for name in spec.css:
    manifest.append(
      f'    <item id="css-{xml_id(name)}" href="Styles/{name}" media-type="{media_type(name)}"/>'
    )
  for chapter in spec.chapters:
    manifest.append(
      f'    <item id="{xml_id(chapter.file_name)}" href="Text/{chapter.file_name}" media-type="application/xhtml+xml"/>'
    )
  cover_id = ""
  for name in spec.images:
    item_id = f"img-{xml_id(name)}"
    props = ' properties="cover-image"' if name == "cover.png" else ""
    if name == "cover.png":
      cover_id = item_id
    manifest.append(
      f'    <item id="{item_id}" href="Images/{name}" media-type="{media_type(name)}"{props}/>'
    )
  spine = "\n".join(
    f'    <itemref idref="{xml_id(chapter.file_name)}"/>' for chapter in spec.chapters
  )
  return f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id" xml:lang="zh-CN">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">{spec.identifier}</dc:identifier>
    <dc:title>{spec.title}</dc:title>
    <dc:creator>{spec.creator}</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-05-27T00:00:00Z</meta>
    <meta name="cover" content="{cover_id}"/>
  </metadata>
  <manifest>
{chr(10).join(manifest)}
  </manifest>
  <spine toc="ncx">
{spine}
  </spine>
</package>
"""


def files_for(spec: EpubSpec) -> dict[str, bytes]:
  files: dict[str, bytes] = {
    "META-INF/container.xml": container_xml().encode("utf-8"),
    "OEBPS/package.opf": package_opf(spec).encode("utf-8"),
    "OEBPS/nav.xhtml": nav_xhtml(spec).encode("utf-8"),
    "OEBPS/toc.ncx": toc_ncx(spec).encode("utf-8"),
  }
  for name, css in spec.css.items():
    files[f"OEBPS/Styles/{name}"] = css.encode("utf-8")
  for name, svg in spec.images.items():
    files[f"OEBPS/Images/{name}"] = svg if isinstance(svg, bytes) else svg.encode("utf-8")
  for chapter in spec.chapters:
    files[f"OEBPS/Text/{chapter.file_name}"] = chapter_xhtml(spec, chapter).encode("utf-8")
  return files


def png_chunk(kind: bytes, data: bytes) -> bytes:
  return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)


def hex_rgb(value: str) -> tuple[int, int, int]:
  value = value.lstrip("#")
  return (int(value[0:2], 16), int(value[2:4], 16), int(value[4:6], 16))


def mix(a: tuple[int, int, int], b: tuple[int, int, int], ratio: float) -> tuple[int, int, int]:
  return tuple(round(a[index] * (1 - ratio) + b[index] * ratio) for index in range(3))


def png_cover(accent: str) -> bytes:
  width = 600
  height = 800
  accent_rgb = hex_rgb(accent)
  paper = (247, 243, 232)
  ink = (48, 48, 44)
  raw = bytearray()
  for y in range(height):
    raw.append(0)
    for x in range(width):
      color = paper
      border = 52 < x < width - 52 and 62 < y < height - 62
      near_border = border and (x < 64 or x > width - 64 or y < 74 or y > height - 74)
      circle = (x - width // 2) ** 2 + (y - 230) ** 2 < 72 ** 2
      wave = 290 < y < 510 and abs((x - width // 2) * 0.36 + (y - 400)) < 34
      title_bar = 560 < y < 610 and 150 < x < 450
      subtitle_bar = 640 < y < 664 and 210 < x < 390
      if near_border:
        color = accent_rgb
      elif circle:
        color = mix(paper, accent_rgb, 0.25)
      elif wave:
        color = mix(paper, accent_rgb, 0.36)
      elif title_bar or subtitle_bar:
        color = ink
      raw.extend(color)
  payload = zlib.compress(bytes(raw), level=9)
  return (
    b"\x89PNG\r\n\x1a\n"
    + png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
    + png_chunk(b"IDAT", payload)
    + png_chunk(b"IEND", b"")
  )


def note_icon_png(accent: str) -> bytes:
  width = 48
  height = 48
  accent_rgb = hex_rgb(accent)
  paper = (255, 255, 250)
  raw = bytearray()
  for y in range(height):
    raw.append(0)
    for x in range(width):
      color = (0, 0, 0, 0)
      circle = (x - 24) ** 2 + (y - 24) ** 2 <= 21 ** 2
      mark = 20 <= x <= 27 and 12 <= y <= 29
      dot = (x - 24) ** 2 + (y - 36) ** 2 <= 3 ** 2
      if circle:
        color = (*accent_rgb, 255)
      if mark or dot:
        color = (*paper, 255)
      raw.extend(color)
  payload = zlib.compress(bytes(raw), level=9)
  return (
    b"\x89PNG\r\n\x1a\n"
    + png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0))
    + png_chunk(b"IDAT", payload)
    + png_chunk(b"IEND", b"")
  )


def city_map_svg(color: str) -> str:
  return f"""<svg xmlns="http://www.w3.org/2000/svg" width="900" height="520" viewBox="0 0 900 520">
  <rect width="900" height="520" fill="#f5f8fb"/>
  <path d="M90 390 C210 320 250 420 370 330 S580 310 700 260 S820 220 850 160" fill="none" stroke="{color}" stroke-width="26" stroke-linecap="round"/>
  <rect x="150" y="130" width="120" height="180" fill="#d9e2ec"/>
  <rect x="350" y="90" width="90" height="240" fill="#c9d6e2"/>
  <rect x="520" y="150" width="160" height="160" fill="#dce8d5"/>
  <circle cx="735" cy="180" r="46" fill="{color}" opacity="0.55"/>
</svg>
"""


def leaf_svg(color: str) -> str:
  return f"""<svg xmlns="http://www.w3.org/2000/svg" width="900" height="520" viewBox="0 0 900 520">
  <rect width="900" height="520" fill="#fbfaf4"/>
  <path d="M450 450 C250 310 270 120 450 80 C630 120 650 310 450 450Z" fill="{color}" opacity="0.42"/>
  <path d="M450 440 C440 320 440 190 450 90" fill="none" stroke="#47624a" stroke-width="12"/>
  <path d="M450 280 C360 260 320 210 290 150" fill="none" stroke="#47624a" stroke-width="8"/>
  <path d="M450 300 C560 270 610 215 650 150" fill="none" stroke="#47624a" stroke-width="8"/>
</svg>
"""


CITY_CHAPTERS_BEFORE = (
  Chapter(
    "chapter-01.xhtml",
    "第一章 明日城巡游",
    """    <section epub:type="chapter" class="old-section">
      <h1 style="font-size:2em">第一章 明日城巡游</h1>
      <p class="lead">暮色落在明日城的玻璃屋顶上。</p>
      <p>导览员阿青把一本写着 <span xml:lang="en">Field Notes</span> 的小册子递给我，说：请从北门开始。</p>
      <p>城里最旧的地名叫 <ruby>行舟<rt>xíng zhōu</rt></ruby> 巷，巷口有一盏蓝色信号灯<sup><a id="note-1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#footnote-1"><img alt="注" src="../Images/note.png"/></a></sup>。</p>
      <figure class="legacy-image"><img alt="北门地图" src="../Images/city-map.svg"/></figure>
      <p class="caption">图 1：北门钟楼与河面风向标。</p>
      <aside epub:type="footnote" role="doc-footnote" class="notes"><ol class="footnote-list"><li class="footnote-item" id="footnote-1"><p><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#note-1">◎</a>这盏灯只在潮位升高时亮起。</p></li></ol></aside>
    </section>""",
  ),
  Chapter(
    "chapter-02.xhtml",
    "第二章 档案室",
    """    <section epub:type="chapter" class="old-section">
      <h1 style="font-size:2em">第二章 档案室</h1>
      <p>档案室把每一次风向改变都记成一行。</p>
      <table>
        <tr><th>时间</th><th>记录</th></tr>
        <tr><td>07:10</td><td>北门风铃连续响了三次。</td></tr>
        <tr><td>09:40</td><td>旧电梯停在第五层，没有乘客。</td></tr>
      </table>
      <p>值班员留下了一段伪代码，提醒后来的人不要覆盖原稿。</p>
      <pre><code>if text_changed:
    stop_cleanup()</code></pre>
    </section>""",
  ),
)


CITY_CHAPTERS_AFTER = (
  Chapter(
    "chapter-01.xhtml",
    "第一章 明日城巡游",
    """    <section epub:type="chapter" class="chapter">
      <h1>第一章 明日城巡游</h1>
      <p class="lead">暮色落在明日城的玻璃屋顶上。</p>
      <p>导览员阿青把一本写着 <span xml:lang="en">Field Notes</span> 的小册子递给我，说：请从北门开始。</p>
      <p>城里最旧的地名叫 <ruby>行舟<rt>xíng zhōu</rt></ruby> 巷，巷口有一盏蓝色信号灯<sup><a id="note-1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#footnote-1"><img alt="注" src="../Images/note.png"/></a></sup>。</p>
      <figure class="map"><img alt="北门地图" src="../Images/city-map.svg"/></figure>
      <p class="caption">图 1：北门钟楼与河面风向标。</p>
      <aside epub:type="footnote" role="doc-footnote" class="footnotes"><ol class="footnote-list"><li class="footnote-item" id="footnote-1"><p><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#note-1">◎</a>这盏灯只在潮位升高时亮起。</p></li></ol></aside>
    </section>""",
  ),
  Chapter(
    "chapter-02.xhtml",
    "第二章 档案室",
    """    <section epub:type="chapter" class="chapter">
      <h1>第二章 档案室</h1>
      <p>档案室把每一次风向改变都记成一行。</p>
      <table class="records">
        <tr><th>时间</th><th>记录</th></tr>
        <tr><td>07:10</td><td>北门风铃连续响了三次。</td></tr>
        <tr><td>09:40</td><td>旧电梯停在第五层，没有乘客。</td></tr>
      </table>
      <p>值班员留下了一段伪代码，提醒后来的人不要覆盖原稿。</p>
      <pre><code>if text_changed:
    stop_cleanup()</code></pre>
    </section>""",
  ),
)


PAPER_CHAPTERS = (
  Chapter(
    "chapter-01.xhtml",
    "第一章 纸上花园",
    """    <section epub:type="chapter" class="garden">
      <h1>第一章 纸上花园</h1>
      <p>清晨的温室没有人说话。</p>
      <p class="poem">一枚纸叶贴在窗上，光从叶脉之间慢慢经过。<br/>风没有进门，香气已经先到了。</p>
      <p>管理员把这片叶子称作 <ruby>折光<rt>zhé guāng</rt></ruby>，因为它只在阴天显出金边。</p>
      <figure><img alt="纸叶" src="../Images/leaf.svg"/></figure>
      <p class="caption">图 1：温室标本柜里的纸叶。</p>
    </section>""",
  ),
  Chapter(
    "chapter-02.xhtml",
    "第二章 访客名单",
    """    <section epub:type="chapter" class="garden">
      <h1>第二章 访客名单</h1>
      <p>午后来了三位访客：修表匠、邮差和一名不会写日期的学生。</p>
      <blockquote><p>请把最轻的种子留在第三个抽屉里。</p></blockquote>
      <ul>
        <li>修表匠带走一只停止的秒针。</li>
        <li>邮差留下没有地址的信封。</li>
        <li>学生把今天写成明天。</li>
      </ul>
      <p>管理员没有纠正他，只在名单旁边添了一枚小小的星号。</p>
    </section>""",
  ),
)


REDLINE_BEFORE = (
  Chapter(
    "chapter-01.xhtml",
    "第一章 红线测试",
    """    <section epub:type="chapter">
      <h1>第一章 红线测试</h1>
      <p>这本小册子用来演示文本红线。</p>
      <p>清洗可以改变样式，但不能改写这句话。</p>
    </section>""",
  ),
)


REDLINE_AFTER = (
  Chapter(
    "chapter-01.xhtml",
    "第一章 红线测试",
    """    <section epub:type="chapter">
      <h1>第一章 红线测试</h1>
      <p>这本小册子用来演示文本红线。</p>
      <p>清洗可以改变样式，也顺手改写了这句话。</p>
    </section>""",
  ),
)


LEGACY_CSS = """body { margin: 0 7%; line-height: 1.8; font-family: serif; }
h1 { text-align: center; border-bottom: 1px solid #999; }
.lead { font-size: 1.1em; }
.legacy-image img, figure img { width: 100%; height: auto; }
.caption { color: #666; font-size: .9em; text-align: center; }
.noteref-icon img { width: 1em; height: 1em; vertical-align: text-top; }
table { width: 100%; border-collapse: collapse; }
td, th { border: 1px solid #999; padding: .35em; }
pre { background: #eee; padding: .7em; white-space: pre-wrap; }
"""


BASE_CSS = """body { margin: 0 7%; line-height: 1.75; font-family: serif; }
h1 { text-align: center; font-size: 1.65em; margin: 1.8em 0 1.2em; }
p { margin: .8em 0; }
.lead { font-size: 1.08em; }
"""


MEDIA_CSS = """figure { margin: 1.2em auto; text-align: center; }
figure img { max-width: 100%; height: auto; }
.map { width: 78%; }
.caption { color: #555; font-size: .9em; text-align: center; }
.noteref-icon img { width: 1em; height: 1em; vertical-align: text-top; }
"""


NOTES_CSS = """.footnotes { border-top: 1px solid #888; margin-top: 2em; font-size: .92em; }
.footnotes ol { padding-left: 1.4em; }
"""


TABLE_CSS = """table.records { width: 100%; border-collapse: collapse; margin: 1em 0; }
.records th, .records td { border: 1px solid #888; padding: .35em .5em; }
pre { background: #f1f1ec; padding: .7em; white-space: pre-wrap; }
"""


GARDEN_BEFORE_CSS = """body { margin: 0 8%; line-height: 1.9; font-family: serif; }
h1 { text-align: center; }
.poem { margin-left: 2em; }
figure img { width: 100%; height: auto; }
.caption { text-align: center; color: #666; }
blockquote { border-left: .25em solid #bbb; padding-left: 1em; }
"""


GARDEN_AFTER_CSS = """body { margin: 0 8%; line-height: 1.8; font-family: serif; }
h1 { text-align: center; letter-spacing: 0; }
.poem { margin: 1em 0; padding: .8em 1em; border-left: .2em solid #8aa17c; background: #f6f8f1; }
figure { margin: 1.2em auto; text-align: center; }
figure img { max-width: 84%; height: auto; }
.caption { text-align: center; color: #555; font-size: .9em; }
blockquote { border-left: .25em solid #8aa17c; padding-left: 1em; color: #333; }
@media (min-width: 40em) {
  .garden .poem { writing-mode: vertical-rl; max-height: 18em; }
}
"""


REDLINE_CSS = """body { margin: 0 8%; line-height: 1.8; font-family: serif; }
h1 { text-align: center; }
"""


def demo_specs() -> list[EpubSpec]:
  return [
    EpubSpec(
      slug="city-field-notes",
      variant="before",
      title="明日城巡游手记",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-city-field-notes",
      chapters=CITY_CHAPTERS_BEFORE,
      css={"legacy.css": LEGACY_CSS},
      images={
        "cover.png": png_cover("#3f6f8f"),
        "note.png": note_icon_png("#3f6f8f"),
        "city-map.svg": city_map_svg("#3f6f8f"),
      },
      output_name="city-field-notes-before.epub",
    ),
    EpubSpec(
      slug="city-field-notes",
      variant="after",
      title="明日城巡游手记",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-city-field-notes",
      chapters=CITY_CHAPTERS_AFTER,
      css={
        "base.css": BASE_CSS,
        "media.css": MEDIA_CSS,
        "notes.css": NOTES_CSS,
        "tables.css": TABLE_CSS,
      },
      images={
        "cover.png": png_cover("#3f6f8f"),
        "note.png": note_icon_png("#3f6f8f"),
        "city-map.svg": city_map_svg("#629f6d"),
      },
      output_name="city-field-notes-after-clean.epub",
    ),
    EpubSpec(
      slug="paper-garden",
      variant="before",
      title="纸上花园观察录",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-paper-garden",
      chapters=PAPER_CHAPTERS,
      css={"legacy.css": GARDEN_BEFORE_CSS},
      images={
        "cover.png": png_cover("#8aa17c"),
        "leaf.svg": leaf_svg("#8aa17c"),
      },
      output_name="paper-garden-before.epub",
    ),
    EpubSpec(
      slug="paper-garden",
      variant="after",
      title="纸上花园观察录",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-paper-garden",
      chapters=PAPER_CHAPTERS,
      css={"base.css": GARDEN_AFTER_CSS},
      images={
        "cover.png": png_cover("#8aa17c"),
        "leaf.svg": leaf_svg("#b9a862"),
      },
      output_name="paper-garden-after-clean.epub",
    ),
    EpubSpec(
      slug="redline-trap",
      variant="before",
      title="红线变更反例",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-redline-trap",
      chapters=REDLINE_BEFORE,
      css={"base.css": REDLINE_CSS},
      images={"cover.png": png_cover("#9d4c4c")},
      output_name="redline-trap-before.epub",
    ),
    EpubSpec(
      slug="redline-trap",
      variant="after",
      title="红线变更反例",
      creator="epub-handbook demo",
      identifier="urn:uuid:epub-handbook-demo-redline-trap",
      chapters=REDLINE_AFTER,
      css={"base.css": REDLINE_CSS + "\np { text-indent: 2em; }\n"},
      images={"cover.png": png_cover("#9d4c4c")},
      output_name="redline-trap-after-text-changed.epub",
    ),
  ]


def main() -> int:
  OUT_DIR.mkdir(parents=True, exist_ok=True)
  built = []
  for spec in demo_specs():
    output = OUT_DIR / spec.output_name
    write_epub(output, files_for(spec))
    built.append({"slug": spec.slug, "variant": spec.variant, "path": str(output.relative_to(ROOT))})
    print(output.relative_to(ROOT))
  manifest = OUT_DIR / "manifest.json"
  manifest.write_text(json.dumps({"generated": built}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
