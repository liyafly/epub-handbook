---
name: epub-package-operator
description: Use when 需要合并或拆分 EPUB、修改书名作者等元数据、替换封面，且必须保留原文件并输出新 EPUB 与可审计 JSON 报告时。
---

# EPUB 包操作

只执行用户明确选择的一个写操作。先审计输入，再写出新产物；不要把包结构审计和写操作混成自动修复。

## 选择入口

| 目标 | Harness |
| --- | --- |
| 合并两本或多本 EPUB | `scripts/epub_package_merge_harness.py` |
| 按 TOC 索引拆分 | `scripts/epub_package_split_harness.py` |
| 修改元数据 | `scripts/epub_metadata_edit_harness.py` |
| 替换封面 | `scripts/epub_cover_replace_harness.py` |
| 只检查 OPF/nav/NCX | 改用 `$epub-package-nav-auditor` |

## 工作流

1. 保留输入文件并运行 preflight；真实加密资源、损坏 ZIP 或无效 OPF 时停止。
2. 对拆分操作先运行旧兼容入口查看索引：

   ```sh
   python3 scripts/epub_package_tool.py split-targets book.epub
   ```

3. 运行一个具体 harness，并始终指定新输出：

   ```sh
   python3 scripts/epub_package_merge_harness.py a.epub b.epub \
     --output merged.epub --format json

   python3 scripts/epub_package_split_harness.py book.epub \
     --output-dir split-out --split-points 0,8 --format json

   python3 scripts/epub_metadata_edit_harness.py book.epub \
     --output metadata.epub --metadata-json '{"title":"新书名"}' --format json

   python3 scripts/epub_cover_replace_harness.py book.epub \
     --output covered.epub --cover cover.png --format json
   ```

4. 运行 `scripts/epub_lint.py`、相关 OPF/nav XML 校验和 `validate_text_invariance.py`。合并或拆分会改变 package/spine，人工确认报告中的输入、输出、段数和重命名资源。

## 边界

- harness 拒绝覆盖已存在的输出；不要删除这一保护。
- 合并至少需要两个输入；拆分点必须来自当前书的 `split-targets`。
- 元数据 JSON 只接受字符串字段。
- 替换封面时同步 OPF cover metadata、`cover-image` properties 和本地引用。
- 不在包操作中重写正文、改字体、转换图片或绕过 DRM。

## 验证

```sh
python3 scripts/test_epub_package_harnesses.py
python3 scripts/test_epub_package_tool.py
python3 scripts/validate_skills_basic.py
git diff --check
```
