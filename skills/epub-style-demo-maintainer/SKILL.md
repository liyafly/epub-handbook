---
name: epub-style-demo-maintainer
description: 维护 epub-style-demo 兼容 fixture、reader matrix、最终规则和验证循环。用于 EPUB 阅读器行为变化、需要新增 demo 覆盖、或需要把实测发现沉淀为最终生产规则时。
---

# EPUB Style Demo 维护

## 何时用

- 修改 `templates/epub-style-demo/`、新增阅读器兼容场景，或把阅读器发现转成最终 EPUB 生产规则时。
- 固定闭环（demo 先行，文档后补）：新增或更新能暴露阅读器行为的最小 fixture → 构建产物 → 用 CLI 能力校验 → 回写 `docs/final/reader-matrix.yaml`（需要人工复测或版本待确认时用 `warn`，不虚构 pass/fail）→ 只有 fixture 和 matrix 记录都存在后才更新 `docs/final/SPEC-实现约束.md`，然后同步最终手册和速查表 → 规则影响自动化行为时同步相关 `skills/*/SKILL.md`（不改 frontmatter 字段名）。
- 权威弹注结构源是 `docs/final/SPEC-实现约束.md` §1；兼容规则不得单方面新增或改名同族 class。

## 调什么

```sh
# 1) 构建 demo（产物在 templates/epub-style-demo/dist/）
sh templates/epub-style-demo/build.sh

# 2) 校验产物
epub run epub.style.demo.maintain --input <dist 产物>.epub --json
epub run epub.notes.popup.normalize --input <dist 产物>.epub --json
```

`epub.style.demo.maintain` Go 实现迁移中：`--input` 指向构建产物 EPUB 时为产物校验，缺省/指向 demo 源树 `templates/epub-style-demo` 时校验源树；当前运行会在 findings 里得到 `warn capability.not-implemented`，且 CLI 用法检查要求提供临时 `--output`（或加全局 `--dry-run`）才能通过。迁移完成前，产物结构判断以 `epub run epub.notes.popup.normalize` 的 findings 与人工复核为准。

本机有 `xmllint` 时可对 `templates/epub-style-demo/OEBPS/package.opf`、`nav.xhtml`、`toc.ncx` 额外运行 `xmllint --noout ...`；没有时记录跳过理由。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`（noteref 数）、`text_files`（XHTML 文件数）、`violations`（结构违反数）；violations > 0 时 `findings` 出现 `error popupnotes`，`detail` 指明具体文件与问题。
- `epub.style.demo.maintain` 迁移期返回：`warn capability.not-implemented`（events 显示 skipped）表示校验逻辑尚未执行——不要把此时的 `status: complete` 当作通过。

## 依据返回怎么判断

- 返回 `error`（含 `popupnotes`、`redline.*`）→ 修 fixture 或产物后重跑；`status == approval-required` → 停下来问人。
- 当前兼容规则（逐条对照 fixture 与 matrix，不得只靠手册推断）：
  - 图片环绕主路径用 `figure.img-left` / `figure.img-right`；float 和百分比 `width` 挂在 `figure` 上，先在 `25%` 到 `35%` 调整，再结合目标阅读器、视口和字号实测。direct `img` float 不是主路径（部分阅读器会把图片渲染得过小）。
  - 不固定图片高度，不把 `aspect-ratio` 当主路径；真实图片用 `height:auto` 保持宽高比，`figure` 需要自然高度承载图注；图文环绕测试需要足够长的周围正文，短段落只是阈值反例。
  - 书内图片以 JPEG/PNG 为生产主路径；WebP 只作现代阅读器实验（demo WebP 在 Kindle conversion logs 触发 W14012/W14015）；SVG 可作增强测试，Kindle 目标构建在渲染不确定时需 JPEG/PNG 栅格 fallback。
  - 波浪下划线必须拆开：先写 `text-decoration: underline;`，再写 `text-decoration-style: wavy;`；Kindle App fallback 为普通 underline。
  - 含 MathML 的 XHTML manifest item 必须带 `properties="mathml"`；MathML 覆盖保持在 KDP Enhanced Typesetting 和 EPUB 3 支持标签范围内。
  - 多看旧版 fallback 用 `ol.footnote-list.duokan-footnote-content`；单个 `li.footnote-item` 只加 `duokan-footnote-item`。
  - 英文书籍规则按类型拆分：小说/散文走 `.english-fiction`；英文正文必须声明 `lang`，用短 serif 链，首段无缩进、后续段缩进，未验证断字不强制 justify；插图默认居中 `figure`；首字优先 `::first-letter`，旧式 span 首字和 float drop cap 只作增强并需大字号复测。
  - 章节头图属于普通可重排章首结构，放 `literary.css`；头图只做装饰，标题必须是真实 `h1`；小章标保守宽度，满栏横幅用 `width:100%` 铺满正文内容栏并由源图比例控制高度。
  - 便签/资料卡主路径用 border、background、padding 和 left-rule；box-shadow、inset、不规则圆角和 outline-offset 只作可丢失增强。
  - 通用 demo 不用 `transform: rotate()` 旋转便签块——Kindle Previewer 3.104（2026-05-23 实测）会触发 KFX 增强排版内部错误；需要斜角感时用不对称边框、圆角和投影模拟。
  - SVG 花边只作 demo 实验项（验证简单内联 SVG 边线可行性），不作推荐边框；长条投影框必须保留真实文本和边框兜底。
