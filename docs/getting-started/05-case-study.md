# 自造 EPUB 清洗演示案例

本页汇总 Stage 4 的自造 before / after EPUB。`samples/demo-books/dist/` 下的生成产物可入 Git，方便直接演示；每个样本目录提供 `notes.md` 复现流程。

## 先生成样本

```sh
bash samples/demo-books/build.sh
```

输出目录：`samples/demo-books/dist/`。

## 案例 1：明日城巡游手记

样本记录：[samples/demo-books/city-field-notes/notes.md](../../samples/demo-books/city-field-notes/notes.md)。

流程摘要：

1. `before` 使用旧式单 CSS、内联标题样式和基础资源。
2. `after` 拆成 `base.css` / `media.css` / `notes.css` / `tables.css`。
3. 正文、核心元数据、spine 和封面不变。
4. 用 `validate_text_invariance.py --check all` 做红线 gate。
5. 按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 或 VS Code 对比结构、文本、样式、资源、元数据五层。

## 案例 2：纸上花园观察录

样本记录：[samples/demo-books/paper-garden/notes.md](../../samples/demo-books/paper-garden/notes.md)。

本案例重点看诗段、Ruby、blockquote、列表和宽屏竖排增强。after 只调整 CSS 和非封面插图资源，文本红线应通过。

## 反例：红线变更反例

样本记录：[samples/demo-books/redline-trap/notes.md](../../samples/demo-books/redline-trap/notes.md)。

这对文件故意改写正文，用来验证红线 gate 会失败。它不是合法清洗结果，只用于教学和 diff 文本层演示。

## 学到了什么

- 现成 EPUB 的清洗要先守红线，再谈样式整理。
- AI 最常用入口是 `epub-layout-auditor`，真正写盘前先 dry-run。
- 外部 diff 工具（Calibre / VS Code）负责让人看见结构、文本、样式、资源、元数据五层变化；它不替代 reader-matrix 实测。
- 自造 demo 可以覆盖脚注、表格、代码、Ruby、竖排增强和红线反例，不受第三方来源质量限制。

## 下一步

- 拿你自己的 epub，跟着 [cleanup-flow.md](../pipeline/cleanup-flow.md) 跑一遍。
- 按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 看自己的清洗结果。
