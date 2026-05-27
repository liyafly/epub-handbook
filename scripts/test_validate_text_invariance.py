#!/usr/bin/env python3
"""Regression tests for validate_text_invariance.py."""

from __future__ import annotations

import shutil
import subprocess
import sys
import time
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "validate_text_invariance.py"


def package_xml(title: str = "测试书", identifier: str = "urn:uuid:test", subject: str = "demo", spine: list[str] | None = None) -> str:
  spine = spine or ["chap1", "chap2"]
  itemrefs = "\n".join(f'    <itemref idref="{item}"/>' for item in spine)
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">{identifier}</dc:identifier>
    <dc:title>{title}</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:subject>{subject}</dc:subject>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap1" href="Text/chap1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chap2" href="Text/chap2.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover" href="Images/cover.png" media-type="image/png" properties="cover-image"/>
  </manifest>
  <spine>
{itemrefs}
  </spine>
</package>
'''


def xhtml(body: str) -> str:
  return f'''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><title>t</title></head><body>{body}</body></html>
'''


def source_tree(root: Path) -> None:
  (root / "META-INF").mkdir(parents=True)
  (root / "OEBPS" / "Text").mkdir(parents=True)
  (root / "OEBPS" / "Styles").mkdir(parents=True)
  (root / "OEBPS" / "Images").mkdir(parents=True)
  (root / "mimetype").write_text("application/epub+zip", encoding="ascii")
  (root / "META-INF" / "container.xml").write_text(
    '''<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
''',
    encoding="utf-8",
  )
  (root / "OEBPS" / "package.opf").write_text(package_xml(), encoding="utf-8")
  (root / "OEBPS" / "nav.xhtml").write_text(xhtml("<nav epub:type=\"toc\"><ol><li>目录</li></ol></nav>"), encoding="utf-8")
  (root / "OEBPS" / "Text" / "chap1.xhtml").write_text(xhtml("<p>第一段文字。</p><p>第二段文字。</p>"), encoding="utf-8")
  (root / "OEBPS" / "Text" / "chap2.xhtml").write_text(xhtml("<p>第三段文字。</p>"), encoding="utf-8")
  (root / "OEBPS" / "Styles" / "main.css").write_text("body { margin: 0; }\n", encoding="utf-8")
  (root / "OEBPS" / "Images" / "cover.png").write_bytes(b"PNG-cover-v1")


def build_epub(src: Path, out: Path) -> None:
  with zipfile.ZipFile(out, "w") as zf:
    zf.write(src / "mimetype", "mimetype", compress_type=zipfile.ZIP_STORED)
    for path in sorted(src.rglob("*")):
      if path.is_file() and path.name != "mimetype":
        zf.write(path, path.relative_to(src).as_posix(), compress_type=zipfile.ZIP_DEFLATED)


def make_pair(tmp: Path) -> tuple[Path, Path, Path, Path]:
  before_src = tmp / "before-src"
  after_src = tmp / "after-src"
  source_tree(before_src)
  shutil.copytree(before_src, after_src)
  before = tmp / "before.epub"
  after = tmp / "after.epub"
  return before_src, after_src, before, after


def run(before: Path, after: Path, *args: str) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), str(before), str(after), *args],
    cwd=ROOT,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    check=False,
  )


def case(name: str, mutator, expected: int, *args: str, must_contain: str | None = None) -> None:
  with TemporaryDirectory() as raw:
    tmp = Path(raw)
    before_src, after_src, before, after = make_pair(tmp)
    mutator(before_src, after_src, tmp)
    build_epub(before_src, before)
    build_epub(after_src, after)
    result = run(before, after, *args)
    if result.returncode != expected:
      raise AssertionError(f"{name}: expected {expected}, got {result.returncode}\n{result.stderr}")
    if must_contain and must_contain not in result.stderr:
      raise AssertionError(f"{name}: missing {must_contain!r}\n{result.stderr}")


def noop(_b: Path, _a: Path, _t: Path) -> None:
  pass


def main() -> int:
  tests = [
    ("TC1 self", noop, 0, []),
    ("TC2 text change", lambda _b, a, _t: (a / "OEBPS/Text/chap1.xhtml").write_text(xhtml("<p>第一段文字改。</p><p>第二段文字。</p>"), encoding="utf-8"), 1, [], "text: modified"),
    ("TC3 delete chapter", lambda _b, a, _t: (a / "OEBPS/Text/chap2.xhtml").unlink(), 1, [], "deleted XHTML"),
    ("TC4 empty paragraph", lambda _b, a, _t: (a / "OEBPS/Text/chap1.xhtml").write_text(xhtml("<p>第一段文字。</p><p> </p><p>第二段文字。</p>"), encoding="utf-8"), 0, []),
    ("TC5 p to div", lambda _b, a, _t: (a / "OEBPS/Text/chap1.xhtml").write_text(xhtml("<div>第一段文字。</div><div>第二段文字。</div>"), encoding="utf-8"), 0, []),
    ("TC6 css change", lambda _b, a, _t: (a / "OEBPS/Styles/main.css").write_text("body { margin: 1em; }\n", encoding="utf-8"), 0, []),
    ("TC8 drm", lambda _b, a, _t: (a / "META-INF/encryption.xml").write_text("<encryption/>", encoding="utf-8"), 2, [], "DRM detected"),
    ("TC10 title", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(title="新标题"), encoding="utf-8"), 1, [], "metadata"),
    ("TC11 spine reorder", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(spine=["chap2", "chap1"]), encoding="utf-8"), 1, [], "spine"),
    ("TC12 title explicit", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(title="新标题"), encoding="utf-8"), 1, ["--check", "metadata"], "dc:title"),
    ("TC13 identifier", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(identifier="urn:uuid:changed"), encoding="utf-8"), 1, [], "identifier"),
    ("TC14 spine swap", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(spine=["chap2", "chap1"]), encoding="utf-8"), 1, ["--check", "spine"], "spine"),
    ("TC15 spine delete", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(spine=["chap1"]), encoding="utf-8"), 1, [], "spine"),
    ("TC16 cover", lambda _b, a, _t: (a / "OEBPS/Images/cover.png").write_bytes(b"PNG-cover-v2"), 1, [], "cover"),
    ("TC17 subject", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(subject="changed"), encoding="utf-8"), 0, []),
    ("TC18 drm explicit", lambda _b, a, _t: (a / "META-INF/encryption.xml").write_text("<encryption/>", encoding="utf-8"), 2, ["--check", "drm"], "DRM detected"),
    ("TC19 check text only", lambda _b, a, _t: (a / "OEBPS/package.opf").write_text(package_xml(title="新标题"), encoding="utf-8"), 0, ["--check", "text"]),
    ("TC20 verbose pass", noop, 0, ["--verbose"], "All requested red-line checks passed."),
  ]

  for name, mutator, expected, args, *rest in tests:
    case(name, mutator, expected, *args, must_contain=rest[0] if rest else None)

  with TemporaryDirectory() as raw:
    tmp = Path(raw)
    nonzip = tmp / "not.epub"
    nonzip.write_text("not a zip", encoding="utf-8")
    result = run(nonzip, nonzip)
    if result.returncode != 2:
      raise AssertionError("TC7 non zip failed")

  with TemporaryDirectory() as raw:
    tmp = Path(raw)
    before_src, after_src, before, after = make_pair(tmp)
    for index in range(3, 23):
      (before_src / "OEBPS/Text" / f"chap{index}.xhtml").write_text(xhtml(f"<p>段落 {index}</p>"), encoding="utf-8")
      (after_src / "OEBPS/Text" / f"chap{index}.xhtml").write_text(xhtml(f"<p>段落 {index}</p>"), encoding="utf-8")
    build_epub(before_src, before)
    build_epub(after_src, after)
    start = time.monotonic()
    result = run(before, after)
    elapsed = time.monotonic() - start
    if result.returncode or elapsed > 5:
      raise AssertionError(f"TC9 22 XHTML failed: rc={result.returncode}, elapsed={elapsed:.2f}\n{result.stderr}")

  print("validate_text_invariance tests ok (20 cases)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
