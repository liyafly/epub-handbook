#!/usr/bin/env python3
"""Regression tests for epub_image_layout_advisor.py."""

from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))

import epub_image_layout_advisor as A  # noqa: E402


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "epub_image_layout_advisor.py"


def xhtml(body: str, body_class: str = "") -> str:
  class_attr = f' class="{body_class}"' if body_class else ""
  return f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Fixture</title><link rel="stylesheet" href="../Styles/media.css"/></head>
  <body{class_attr}>{body}</body>
</html>
"""


def make_epub(path: Path, pages: dict[str, str], nav_entries: list[str]) -> None:
  manifest = [
    '<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>',
    '<item id="css" href="Styles/media.css" media-type="text/css"/>',
    '<item id="img" href="Images/test.png" media-type="image/png"/>',
  ]
  spine = []
  for index, name in enumerate(pages, start=1):
    manifest.append(f'<item id="p{index}" href="Text/{name}" media-type="application/xhtml+xml"/>')
    spine.append(f'<itemref idref="p{index}"/>')
  nav_items = "".join(f'<li><a href="Text/{name}">{name}</a></li>' for name in nav_entries)
  files: dict[str, bytes | str] = {
    "mimetype": b"application/epub+zip",
    "META-INF/container.xml": """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
""",
    "OEBPS/content.opf": f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata/>
  <manifest>{''.join(manifest)}</manifest>
  <spine>{''.join(spine)}</spine>
</package>
""",
    "OEBPS/nav.xhtml": f"""<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>Nav</title></head>
  <body><nav epub:type="toc"><ol>{nav_items}</ol></nav></body>
</html>
""",
    "OEBPS/Styles/media.css": """.img-left { float: left; width: 30%; }
.img-right { float: right; width: 30%; }
.img-left img, .img-right img { width: 100%; height: auto; }
""",
    "OEBPS/Images/test.png": b"png",
  }
  for name, value in pages.items():
    files[f"OEBPS/Text/{name}"] = value
  with zipfile.ZipFile(path, "w") as zf:
    zf.writestr("mimetype", files.pop("mimetype"), compress_type=zipfile.ZIP_STORED)
    for name, value in files.items():
      zf.writestr(name, value.encode("utf-8") if isinstance(value, str) else value)


def findings(report: dict[str, object], kind: str, filename: str | None = None) -> list[dict[str, object]]:
  values = [
    item
    for item in report["findings"]
    if item["finding"] == kind and (filename is None or item["file"].endswith(filename))
  ]
  return values


def test_lone_image_and_poster_exclusion() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "bare.xhtml": xhtml('<img src="../Images/test.png" alt="test"/><p>正文足够长。</p>'),
        "figure.xhtml": xhtml('<figure><img src="../Images/test.png" alt="test"/></figure><p>正文。</p>'),
        "poster.xhtml": xhtml('<img src="../Images/test.png" alt=""/>', "poster-bg"),
      },
      ["bare.xhtml", "figure.xhtml", "poster.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "lone-image-no-figure", "bare.xhtml")
    assert not findings(report, "lone-image-no-figure", "figure.xhtml")
    assert not findings(report, "lone-image-no-figure", "poster.xhtml")


def test_caption_detection() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "short.xhtml": xhtml('<img src="../Images/test.png" alt="test"/><p>十二字以内图注</p>'),
        "long.xhtml": xhtml(
          '<img src="../Images/test.png" alt="test"/><p>'
          + "这是普通正文段落，不应因为紧跟图片就被当成图注。" * 5
          + "</p>"
        ),
      },
      ["short.xhtml", "long.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "caption-detached", "short.xhtml")
    assert not findings(report, "caption-detached", "long.xhtml")


def test_float_width_risk() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "bad.xhtml": xhtml('<img src="../Images/test.png" alt="test" style="float:left;width:50%"/><p>正文。</p>'),
        "good.xhtml": xhtml(
          '<figure class="img-left" style="width:30%">'
          '<img src="../Images/test.png" alt="test" style="width:100%;height:auto"/>'
          "</figure><p>正文。</p>"
        ),
      },
      ["bad.xhtml", "good.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "float-width-risk", "bad.xhtml")
    assert not findings(report, "float-width-risk", "good.xhtml")


def test_missing_alt() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "missing.xhtml": xhtml('<figure><img src="../Images/test.png"/></figure><p>正文。</p>'),
        "present.xhtml": xhtml('<figure><img src="../Images/test.png" alt=""/></figure><p>正文。</p>'),
      },
      ["missing.xhtml", "present.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "missing-alt", "missing.xhtml")
    assert not findings(report, "missing-alt", "present.xhtml")


def test_chapter_head_candidate_requires_first_content_and_nav_entry() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "chapter.xhtml": xhtml(
          '<figure><img src="../Images/test.png" alt="chapter"/></figure><h1>标题</h1><p>正文。</p>'
        ),
        "not-first.xhtml": xhtml(
          '<h1>标题</h1><figure><img src="../Images/test.png" alt="later"/></figure><p>正文。</p>'
        ),
        "not-nav.xhtml": xhtml(
          '<figure><img src="../Images/test.png" alt="hidden"/></figure><p>正文。</p>'
        ),
      },
      ["chapter.xhtml", "not-first.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "chapter-head-image-candidate", "chapter.xhtml")
    assert not findings(report, "chapter-head-image-candidate", "not-first.xhtml")
    assert not findings(report, "chapter-head-image-candidate", "not-nav.xhtml")


def test_fullpage_alite_candidate() -> None:
  with TemporaryDirectory() as raw:
    path = Path(raw) / "fixture.epub"
    make_epub(
      path,
      {
        "fullpage.xhtml": xhtml('<figure><img src="../Images/test.png" alt="volume"/></figure>'),
        "normal.xhtml": xhtml('<p>普通正文超过二十个字符，不能被识别成整页单图候选。</p><img src="../Images/test.png" alt="inline"/>'),
      },
      ["fullpage.xhtml", "normal.xhtml"],
    )
    report = A.analyze_epub(path)
    assert findings(report, "fullpage-image-alite-candidate", "fullpage.xhtml")
    assert not findings(report, "fullpage-image-alite-candidate", "normal.xhtml")


def test_cli_json_markdown_and_read_only() -> None:
  with TemporaryDirectory() as raw:
    root = Path(raw)
    path = root / "fixture.epub"
    report_path = root / "advisor.json"
    make_epub(
      path,
      {"chapter.xhtml": xhtml('<img src="../Images/test.png"/><p>图注</p>')},
      ["chapter.xhtml"],
    )
    before = hashlib.sha256(path.read_bytes()).hexdigest()
    result = subprocess.run(
      [sys.executable, str(SCRIPT), str(path), "--format", "json", "--report", str(report_path)],
      check=False,
      capture_output=True,
      text=True,
    )
    assert result.returncode == 0, result.stderr
    parsed = json.loads(result.stdout)
    assert parsed["version"] == "1"
    assert report_path.exists()
    assert all(item["scene"] == "image-layout" for item in parsed["findings"])
    assert all(item["candidates"] for item in parsed["findings"])
    assert all(
      "SPEC" in candidate["risk"]
      or "demo" in candidate["risk"]
      or "reader-matrix" in candidate["risk"]
      or "未实测" in candidate["risk"]
      for item in parsed["findings"]
      for candidate in item["candidates"]
    )
    assert hashlib.sha256(path.read_bytes()).hexdigest() == before

    markdown = subprocess.run(
      [sys.executable, str(SCRIPT), str(path), "--format", "md"],
      check=False,
      capture_output=True,
      text=True,
    )
    assert markdown.returncode == 0, markdown.stderr
    assert "epub_decision_log.py add" in markdown.stdout


def main() -> int:
  test_lone_image_and_poster_exclusion()
  test_caption_detection()
  test_float_width_risk()
  test_missing_alt()
  test_chapter_head_candidate_requires_first_content_and_nav_entry()
  test_fullpage_alite_candidate()
  test_cli_json_markdown_and_read_only()
  print("epub image layout advisor tests ok")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
