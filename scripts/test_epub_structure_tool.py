#!/usr/bin/env python3
"""Regression tests for epub_structure_tool.py."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from xml.etree import ElementTree as ET

sys.path.insert(0, str(Path(__file__).resolve().parent))
from epub_structure_tool import StructureToolError, normalize_epub, rewrite_epub  # noqa: E402


OPF_NS = {"opf": "http://www.idpf.org/2007/opf"}


def zip_info(name: str, compress_type: int) -> zipfile.ZipInfo:
  info = zipfile.ZipInfo(name)
  info.compress_type = compress_type
  return info


def encryption_xml(uri: str, algorithm: str) -> str:
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"
    xmlns:enc="http://www.w3.org/2001/04/xmlenc#">
  <enc:EncryptedData>
    <enc:EncryptionMethod Algorithm="{algorithm}"/>
    <enc:CipherData>
      <enc:CipherReference URI="{uri}"/>
    </enc:CipherData>
  </enc:EncryptedData>
</encryption>
'''


def write_fixture(path: Path, encrypted: str | None = None) -> None:
  files: dict[str, str | bytes] = {
    "META-INF/container.xml": '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
''',
    "OPS/package.opf": '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:structure-tool-test</dc:identifier>
    <dc:title>Structure Tool Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="toc" href="legacy/book.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="nav" href="legacy/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter-one.xhtml" href="legacy/%3Fmix.xhtml" media-type="application/xhtml+xml"/>
    <item id="appendix" href="legacy/appendix.xhtml" media-type="application/xhtml+xml"/>
    <item id="main-css" href="legacy/theme.css" media-type="text/css"/>
    <item id="cover-image" href="assets/%2Acover.JPG" media-type="image/jpeg"/>
    <item id="font-main" href="assets/font.ttf" media-type="font/ttf"/>
  </manifest>
  <spine toc="toc">
    <itemref idref="chapter-one.xhtml"/>
    <itemref idref="appendix"/>
  </spine>
  <guide>
    <reference type="text" title="Start" href="legacy/%3Fmix.xhtml#start"/>
  </guide>
</package>
''',
    "OPS/legacy/book.ncx": '''<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="n1"><navLabel><text>第一章</text></navLabel><content src="%3Fmix.xhtml#start"/></navPoint>
  </navMap>
</ncx>
''',
    "OPS/legacy/nav.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>目录</title></head>
  <body><nav><ol><li><a href="%3Fmix.xhtml#start">第一章</a></li></ol></nav></body>
</html>
''',
    "OPS/legacy/?mix.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title>第一章</title>
    <link rel="stylesheet" href="theme.css"/>
  </head>
  <body style="background-image: url('../assets/%2Acover.JPG')">
    <h1 id="start">第一章</h1>
    <p>正文保留。<a href="appendix.xhtml#end">附录</a></p>
    <img src="../assets/%2Acover.JPG" alt="cover"/>
  </body>
</html>
''',
    "OPS/legacy/appendix.xhtml": '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>附录</title></head><body><p id="end">附录正文。</p></body></html>
''',
    "OPS/legacy/theme.css": "body { background-image: url('../assets/%2Acover.JPG'); }\n",
    "OPS/assets/*cover.JPG": b"jpeg-bytes",
    "OPS/assets/font.ttf": b"font-bytes",
    "OPS/extras/unlisted.bin": b"unlisted-bytes",
  }
  if encrypted == "font":
    files["META-INF/encryption.xml"] = encryption_xml(
      "OPS/assets/font.ttf",
      "http://www.idpf.org/2008/embedding",
    )
  elif encrypted == "text":
    files["META-INF/encryption.xml"] = encryption_xml(
      "OPS/legacy/%3Fmix.xhtml",
      "http://www.w3.org/2001/04/xmlenc#aes128-cbc",
    )
  elif encrypted == "stale":
    files["META-INF/encryption.xml"] = encryption_xml(
      "OPS/Styles/dkagent.css",
      "http://www.w3.org/2001/04/xmlenc#aes128-ctr",
    )

  with zipfile.ZipFile(path, "w") as zf:
    for name, data in files.items():
      payload = data.encode("utf-8") if isinstance(data, str) else data
      zf.writestr(zip_info(name, zipfile.ZIP_DEFLATED), payload)
    zf.writestr(zip_info("mimetype", zipfile.ZIP_DEFLATED), b"wrong-on-purpose")


def assert_valid_mimetype(zf: zipfile.ZipFile) -> None:
  first = zf.infolist()[0]
  assert first.filename == "mimetype"
  assert first.compress_type == zipfile.ZIP_STORED
  assert zf.read("mimetype") == b"application/epub+zip"


def opf_items(zf: zipfile.ZipFile) -> dict[str, ET.Element]:
  root = ET.fromstring(zf.read("OPS/package.opf"))
  return {
    item.attrib["id"]: item
    for item in root.findall("opf:manifest/opf:item", OPF_NS)
  }


def test_format(root: Path) -> None:
  source = root / "source.epub"
  output = root / "formatted.epub"
  write_fixture(source)
  report = rewrite_epub(source, output, "format")
  assert report.moved_resources == 7, report
  assert report.renamed_resources == 0, report

  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    names = set(zf.namelist())
    assert "OPS/Text/?mix.xhtml" in names
    assert "OPS/Text/nav.xhtml" in names
    assert "OPS/Styles/theme.css" in names
    assert "OPS/Images/*cover.JPG" in names
    assert "OPS/Fonts/font.ttf" in names
    assert "OPS/book.ncx" in names
    assert zf.read("OPS/extras/unlisted.bin") == b"unlisted-bytes"

    items = opf_items(zf)
    assert items["chapter-one.xhtml"].attrib["href"] == "Text/%3Fmix.xhtml"
    assert items["cover-image"].attrib["href"] == "Images/%2Acover.JPG"
    opf = zf.read("OPS/package.opf").decode("utf-8")
    assert 'href="Text/%3Fmix.xhtml#start"' in opf

    chapter = zf.read("OPS/Text/?mix.xhtml").decode("utf-8")
    assert 'href="../Styles/theme.css"' in chapter
    assert 'href="appendix.xhtml#end"' in chapter
    assert 'src="../Images/%2Acover.JPG"' in chapter
    assert "正文保留。" in chapter
    assert "../Images/%2Acover.JPG" in zf.read("OPS/Styles/theme.css").decode("utf-8")
    assert 'src="Text/%3Fmix.xhtml#start"' in zf.read("OPS/book.ncx").decode("utf-8")


def test_deobfuscate_filenames(root: Path) -> None:
  source = root / "font-obfuscated.epub"
  output = root / "deobfuscated.epub"
  write_fixture(source, encrypted="font")
  report = rewrite_epub(source, output, "deobfuscate-filenames")
  assert report.font_obfuscation_resources == 1, report
  assert report.renamed_resources == 5, report

  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    names = set(zf.namelist())
    assert "OPS/Text/chapter-one.xhtml" in names
    assert "OPS/Text/appendix.xhtml" in names
    assert "OPS/Styles/main-css.css" in names
    assert "OPS/Images/cover-image.jpg" in names
    assert "OPS/Fonts/font-main.ttf" in names
    assert "OPS/toc.ncx" in names

    items = opf_items(zf)
    assert items["chapter-one.xhtml"].attrib["href"] == "Text/chapter-one.xhtml"
    assert items["cover-image"].attrib["href"] == "Images/cover-image.jpg"
    chapter = zf.read("OPS/Text/chapter-one.xhtml").decode("utf-8")
    assert 'href="../Styles/main-css.css"' in chapter
    assert 'src="../Images/cover-image.jpg"' in chapter
    assert "正文保留。" in chapter
    encryption = zf.read("META-INF/encryption.xml").decode("utf-8")
    assert 'URI="OPS/Fonts/font-main.ttf"' in encryption


def test_refuse_drm(root: Path) -> None:
  source = root / "drm.epub"
  output = root / "should-not-exist.epub"
  write_fixture(source, encrypted="text")
  try:
    rewrite_epub(source, output, "deobfuscate-filenames")
  except StructureToolError as exc:
    assert "DRM or unsupported encrypted resources detected" in str(exc)
  else:
    raise AssertionError("expected encrypted XHTML to be refused")
  assert not output.exists()


def test_normalize_workflow(root: Path) -> None:
  source = root / "workflow.epub"
  output = root / "workflow-normalized.epub"
  write_fixture(source)
  report = normalize_epub(source, output)
  assert [stage["operation"] for stage in report.stages] == ["format", "deobfuscate-filenames"], report
  with zipfile.ZipFile(output) as zf:
    assert_valid_mimetype(zf)
    assert "OPS/Text/chapter-one.xhtml" in zf.namelist()
    assert "OPS/Styles/main-css.css" in zf.namelist()
    chapter = zf.read("OPS/Text/chapter-one.xhtml").decode("utf-8")
    assert "正文保留。" in chapter
    assert 'src="../Images/cover-image.jpg"' in chapter


def test_remove_stale_encryption_reference(root: Path) -> None:
  source = root / "stale-encryption.epub"
  output = root / "stale-encryption-normalized.epub"
  write_fixture(source, encrypted="stale")
  report = normalize_epub(source, output)
  assert report.stages[0]["removed_stale_encryption_resources"] == 1, report
  assert any("remove stale encryption reference" in warning for warning in report.stages[0]["warnings"]), report
  with zipfile.ZipFile(output) as zf:
    assert "META-INF/encryption.xml" not in zf.namelist()


def main() -> int:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    test_format(root)
    test_deobfuscate_filenames(root)
    test_refuse_drm(root)
    test_normalize_workflow(root)
    test_remove_stale_encryption_reference(root)
  print("epub structure tool tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
