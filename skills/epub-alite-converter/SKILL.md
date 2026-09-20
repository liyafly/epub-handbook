---
name: epub-alite-converter
description: 将既有 EPUB 封面式页面、卷首、章首或海报页转为 A-lite 可重排方案，保留图文构图与资源。不是全书美化预设，不用于普通竖排或固定版式。
---

# EPUB A-lite 转换

## 何时用

明确需要全页封面式视觉时。先读取目标 XHTML/CSS/OPF 与图片/字体，识别背景、叠字、竖列和版权页；完整规则读 [SPEC §2](../../docs/final/SPEC-实现约束.md)，对照 [叠字 fixture](../../templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml) 或 [单图 fixture](../../templates/epub-style-demo/OEBPS/Text/03c-poster-contain.xhtml)。

## 调什么

```sh
epub run epub.alite.convert --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.alite.convert --input "before.epub" --output "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

已知卷数时两次都加 `expect_volumes=N`。审查计划后写出，不覆盖原件。

## 返回怎么读

前缀 `epub.alite.convert.`：`posterPages/copyrightPages` 是改写路径，`posterPagesRefined/copyrightPagesRefined/stylesheetsAdded` 是计数，`warnings` 是需复核项。`alite.no-copyright` 表示未找到相邻版权页，不表示应凭空补一页。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 核对改写范围符合目标，保留文字、图片、嵌入字体和大体视觉顺序，不重新设计构图或新增装饰。
- 使用 body.fullpage + section.fullframe；背景放 poster modifier，不放 shell；按 SPEC 保持 box-sizing、无骨架 padding、正确 overflow 与 writing-mode 前缀。完整 CSS 复用 fixture，不在 skill 再维护一套参数。
- 单图卷封保留 poster-fallback 原图，背景 contain，不能 cover 裁边或拉伸；已有叠字必须仍是真文本。
- 规则进 poster.css，只同步实际资源引用；不转 FXL，不用 absolute/vh/vw/padding-ratio 替代方案。
- 红线后仍检查窄屏、大字号和目标阅读器，特别关注裁切、空白、文字丢失；实测规则变化走 demo skill。
