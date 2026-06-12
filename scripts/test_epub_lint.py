#!/usr/bin/env python3
"""Regression tests for epub_lint.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import epub_lint as L  # noqa: E402

CONTAINER = """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
"""

PAGE = """<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
  <head><title>t</title><link rel="stylesheet" type="text/css" href="../Styles/base.css"/></head>
  {body}
</html>
"""


def make_epub(
  path: Path,
  css: str = "body { margin: 0; }",
  body: str = "<body><p>正文。</p></body>",
  extra_meta: str = "",
  prefix: str = ' prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks-vocabulary-1.0/"',
  chapter_properties: str = "",
  extra_items: str = "",
) -> None:
  opf = f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bid"{prefix}>
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bid">urn:uuid:lint-fixture</dc:identifier>
    <dc:title>Lint Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-06-12T00:00:00Z</meta>
{extra_meta}  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"{chapter_properties}/>
    <item id="css" href="Styles/base.css" media-type="text/css"/>
{extra_items}  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
"""
  nav = PAGE.format(body='<body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">一</a></li></ol></nav></body>')
  files = {
    "META-INF/container.xml": CONTAINER,
    "OEBPS/package.opf": opf,
    "OEBPS/nav.xhtml": nav,
    "OEBPS/Text/chapter.xhtml": PAGE.format(body=body),
    "OEBPS/Styles/base.css": css,
  }
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", b"application/epub+zip")
    for name, data in files.items():
      zf.writestr(name, data)


def rules_of(findings: list[L.Finding]) -> set[str]:
  return {f.rule for f in findings}


def case(tmp: Path, name: str, **kwargs) -> set[str]:
  path = tmp / f"{name}.epub"
  make_epub(path, **kwargs)
  return rules_of(L.lint_epub(path))


def main() -> int:
  with TemporaryDirectory() as raw:
    tmp = Path(raw)

    clean = case(tmp, "clean")
    assert clean == set(), f"clean book must have no findings: {clean}"

    r = case(tmp, "f01", css='p { font-family: "A", "B", "C", "D", "E", serif; }')
    assert "L-F01" in r, r

    r = case(tmp, "f02", css='p { font-family: "SimSun", "宋体", serif; }')
    assert "L-F02" in r, r

    r = case(tmp, "f03", css='@font-face { font-family: "Ghost"; src: url("../Fonts/g.ttf"); }')
    assert "L-F03" in r, r

    r = case(tmp, "f04", css='@font-face { font-family: "Embed"; src: url("../Fonts/e.ttf"); }\nbody { font-family: "Embed", serif; }')
    assert "L-F04" in r, r
    assert "L-F03" not in r, r

    r = case(tmp, "f05-locked-no-meta", body='<body class="body-font-locked"><p>x</p></body>')
    assert "L-F05" in r, r

    r = case(tmp, "f05-meta-no-lock", extra_meta='    <meta property="ibooks:specified-fonts">true</meta>\n')
    assert "L-F05" in r, r

    r = case(
      tmp, "f05-meta-with-font-item",
      extra_meta='    <meta property="ibooks:specified-fonts">true</meta>\n',
      extra_items='    <item id="f1" href="Fonts/e.ttf" media-type="font/ttf"/>\n',
    )
    assert "L-F05" not in r, r

    r = case(tmp, "o01", extra_meta='    <meta property="ibooks:version">1.0</meta>\n', prefix="")
    assert "L-O01" in r, r

    r = case(tmp, "n01", body='<body><p><a epub:type="noteref" href="#fn-missing">注</a></p></body>')
    assert "L-N01" in r, r

    n01_ok = case(
      tmp, "n01-ok",
      body=(
        '<body><p><a epub:type="noteref" href="#fn1">注</a></p>'
        '<aside epub:type="footnote" id="fn1"><p>注文。</p></aside></body>'
      ),
    )
    assert "L-N01" not in n01_ok, n01_ok

    n01_grouped_ok = case(
      tmp, "n01-grouped-ok",
      body=(
        '<body><p><a epub:type="noteref" href="#fn2">注</a></p>'
        '<aside epub:type="footnote"><ol><li id="fn2"><p>注文。</p></li></ol></aside></body>'
      ),
    )
    assert "L-N01" not in n01_grouped_ok, n01_grouped_ok

    r = case(tmp, "m01", body='<body><p><math xmlns="http://www.w3.org/1998/Math/MathML"><mn>1</mn></math></p></body>')
    assert "L-M01" in r, r

    m01_ok = case(
      tmp, "m01-ok",
      body='<body><p><math xmlns="http://www.w3.org/1998/Math/MathML"><mn>1</mn></math></p></body>',
      chapter_properties=' properties="mathml"',
    )
    assert "L-M01" not in m01_ok, m01_ok

    r = case(tmp, "t01", css='.wavy { text-decoration-style: wavy; }')
    assert "L-T01" in r, r

    t01_ok = case(tmp, "t01-ok", css='.wavy { text-decoration: underline; text-decoration-style: wavy; }')
    assert "L-T01" not in t01_ok, t01_ok

    r = case(tmp, "a01", css='.poster { width: 50vw; }')
    assert "L-A01" in r, r

    r = case(tmp, "x01", body="<body><p>broken</body>")
    assert "L-X01" in r, r

  print("epub lint tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
