#!/usr/bin/env python3
"""Regression tests for validate_popup_notes.py."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "validate_popup_notes.py"

# 1x1 透明 PNG（占位图标，validator 只检查文件存在与 manifest 声明）
PNG_1X1 = bytes.fromhex(
  "89504e470d0a1a0a0000000d4948445200000001000000010806000000"
  "1f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
)

VALID_NOTE_XHTML = '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
<head><title>note</title></head>
<body>
  <p>正文<a id="ref1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#note1"><img src="../Images/note.png" alt="注"/></a>。</p>
  <aside epub:type="footnote" role="doc-footnote">
    <ol class="footnote-list">
      <li id="note1" class="footnote-item">注文。<a epub:type="backlink" role="doc-backlink" href="#ref1">返回</a></li>
    </ol>
  </aside>
</body></html>
'''

PACKAGE_OPF = '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:popup-test</dc:identifier>
    <dc:title>弹注测试</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="Text/notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="note-png" href="Images/note.png" media-type="image/png"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>
'''


def build_oebps(root: Path, note_xhtml: str, *, with_note_png: bool = True) -> Path:
  oebps = root / "OEBPS"
  (oebps / "Text").mkdir(parents=True)
  (oebps / "Images").mkdir(parents=True)
  (oebps / "Text" / "notes.xhtml").write_text(note_xhtml, encoding="utf-8")
  (oebps / "package.opf").write_text(PACKAGE_OPF, encoding="utf-8")
  if with_note_png:
    (oebps / "Images" / "note.png").write_bytes(PNG_1X1)
  return oebps


def run(oebps: Path) -> subprocess.CompletedProcess[str]:
  return subprocess.run(
    [sys.executable, str(SCRIPT), "--oebps", str(oebps)],
    cwd=ROOT,
    check=False,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
  )


def main() -> int:
  # TC1：合法弹注 → 退出码 0
  with TemporaryDirectory() as raw:
    oebps = build_oebps(Path(raw), VALID_NOTE_XHTML)
    result = run(oebps)
    if result.returncode != 0:
      raise AssertionError(f"TC1 合法弹注应通过，实际 rc={result.returncode}\n{result.stderr}")

  # TC2：noteref 缺 role=doc-noteref → 退出码 1
  broken = VALID_NOTE_XHTML.replace(' role="doc-noteref"', '')
  with TemporaryDirectory() as raw:
    oebps = build_oebps(Path(raw), broken)
    result = run(oebps)
    if result.returncode != 1:
      raise AssertionError(f"TC2 缺 role 应失败，实际 rc={result.returncode}\n{result.stdout}")

  # TC3：footnote 目标 li 缺 class=footnote-item → 退出码 1
  broken = VALID_NOTE_XHTML.replace(' class="footnote-item"', '')
  with TemporaryDirectory() as raw:
    oebps = build_oebps(Path(raw), broken)
    result = run(oebps)
    if result.returncode != 1:
      raise AssertionError(f"TC3 li 缺 class 应失败，实际 rc={result.returncode}\n{result.stdout}")

  # TC4：含 note 且 manifest 声明 note.png，但磁盘缺 note.png → 退出码 1
  with TemporaryDirectory() as raw:
    oebps = build_oebps(Path(raw), VALID_NOTE_XHTML, with_note_png=False)
    result = run(oebps)
    if result.returncode != 1:
      raise AssertionError(f"TC4 缺 note.png 应失败，实际 rc={result.returncode}\n{result.stdout}")

  print("validate_popup_notes tests ok (4 cases)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
