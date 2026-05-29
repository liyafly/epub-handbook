#!/usr/bin/env python3
"""Regression tests for build_demo_epubs.py (structure + determinism)."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build_demo_epubs as bd  # noqa: E402


def main() -> int:
  specs = bd.demo_specs()
  if not specs:
    raise AssertionError("demo_specs() 返回空")

  # TC1：每个 spec 的 files_for 产出基本结构齐全
  for spec in specs:
    files = bd.files_for(spec)
    assert "mimetype" not in files, f"{spec.output_name} files_for 不应包含 mimetype（由 write_epub 单独写入）"
    assert "META-INF/container.xml" in files, f"{spec.output_name} 缺 container.xml"
    opfs = [name for name in files if name.endswith(".opf")]
    assert opfs, f"{spec.output_name} 缺 .opf"
    navs = [name for name in files if name.endswith("nav.xhtml")]
    assert navs, f"{spec.output_name} 缺 nav.xhtml"
    for chapter in spec.chapters:
      hit = [name for name in files if name.endswith(chapter.file_name)]
      assert hit, f"{spec.output_name} 缺章节 {chapter.file_name}"

  # TC2：写出的 EPUB 是合法 zip，且 mimetype 是第一个条目且 stored（不压缩）
  with TemporaryDirectory() as raw:
    out = Path(raw) / "one.epub"
    bd.write_epub(out, bd.files_for(specs[0]))
    with zipfile.ZipFile(out) as zf:
      names = zf.namelist()
      assert names[0] == "mimetype", f"mimetype 必须是首个条目，实际 {names[0]}"
      info = zf.getinfo("mimetype")
      assert info.compress_type == zipfile.ZIP_STORED, "mimetype 必须 stored 不压缩"
      bad = zf.testzip()
      assert bad is None, f"zip 损坏：{bad}"

  # TC3：确定性——同一 spec 写两次，字节完全一致
  with TemporaryDirectory() as raw:
    a = Path(raw) / "a.epub"
    b = Path(raw) / "b.epub"
    bd.write_epub(a, bd.files_for(specs[0]))
    bd.write_epub(b, bd.files_for(specs[0]))
    assert a.read_bytes() == b.read_bytes(), "write_epub 非确定性（FIXED_ZIP_TIME 失效？）"

  # TC4：redline-trap before/after 对存在
  slugs = {spec.slug for spec in specs}
  assert "redline-trap" in slugs, "缺 redline-trap 反例样本"
  variants = {spec.variant for spec in specs if spec.slug == "redline-trap"}
  assert {"before", "after"} <= variants, f"redline-trap 缺 before/after，实际 {variants}"

  print(f"build_demo_epubs tests ok ({len(specs)} specs)")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
