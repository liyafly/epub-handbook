# 清洗记录：明日城巡游手记

> 类型：自造 demo
> 生成命令：`bash samples/demo-books/build.sh`

## 目标

演示一本版式较丰富的小型 EPUB 从旧式单 CSS 清理到分层 CSS 的过程。文本、核心元数据、spine 和封面必须不变。

## 覆盖点

- 中英混排。
- Ruby。
- 标准 `epub:type="footnote"` 弹注。
- 表格与代码块。
- 非封面图片资源改动。
- CSS 从 `legacy.css` 拆成 `base.css` / `media.css` / `notes.css` / `tables.css`。

## 验证

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/city-field-notes-before.epub \
  samples/demo-books/dist/city-field-notes-after-clean.epub \
  --check all
```

预期：退出码 0。

## Diff 概览

- 结构：manifest/CSS 资源变化，spine 不变。
- 文本：一致。
- 样式：从单文件整理为分层文件。
- 资源：章节插图颜色变化；封面不变。
- 元数据：核心字段不变。
