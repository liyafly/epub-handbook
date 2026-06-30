---
name: epub3-migrator
description: Use when 输入是 EPUB2、Kindle/MOBI 回转或其他 legacy EPUB，需要迁移 package/nav、XHTML shell、弹注和基础排版到 EPUB3，同时必须保留正文和原文件时。
---

# EPUB3 迁移

执行“预检 → 计划 → 写出 → 红线 → 产物验证”。迁移只建立 EPUB3 基线，不替代后续文学结构、字体覆盖或阅读器实测。

## 工作流

1. 保留 before 文件并运行：

   ```sh
   python3 scripts/epub_preflight_harness.py book.epub --format json
   ```

   DRM、未知加密、损坏 container/OPF 时停止。

2. 生成只读迁移计划：

   ```sh
   python3 scripts/epub3_migration_harness.py book.epub --format json
   ```

3. 确认计划后写出新文件：

   ```sh
   python3 scripts/epub3_migration_apply_harness.py book.epub \
     --output book-epub3.epub --format json
   ```

   harness 输出 before/after SHA-256 和底层 conversion report，并拒绝覆盖已有输出。

4. 执行正文与包验证：

   ```sh
   python3 scripts/validate_text_invariance.py book.epub book-epub3.epub --check all
   scripts/validate-popup-notes.sh --epub book-epub3.epub
   python3 scripts/epub_lint.py book-epub3.epub
   ```

5. 人工检查 OPF/nav/NCX、封面、spine 和弹注 diff。需要字体、图片或结构精排时，再分派对应专项 skill。

## 迁移内容

- package version、`dcterms:modified`、manifest media/properties；
- 保留 NCX 并生成 EPUB3 `nav.xhtml`；
- XHTML5 shell、语言属性和可维护格式；
- 经识别的 plain/Sigil/Duokan 注释结构；
- 可选基础排版 stylesheet，不嵌入新字体。

## 边界

- 不覆盖输入或已有输出。
- 不解密、不改正文、不自动替换字符。
- 只有已识别的注释结构可以迁移；模糊结构保留并报告。
- `--no-popup-notes` 与 `--no-typography` 只在用户明确要求时使用。
- 迁移报告不是阅读器兼容性实测结论。

## 验证

```sh
python3 scripts/test_epub3_migration_apply_harness.py
python3 scripts/test_epub3_oneclick_converter.py
python3 scripts/validate_skills_basic.py
git diff --check
```
