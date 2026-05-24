---
name: epub-style-demo-maintainer
description: 维护 epub-style-demo 兼容 fixture、reader matrix、最终规则和验证循环。用于 EPUB 阅读器行为变化、需要新增 demo 覆盖、或需要把实测发现沉淀为最终生产规则时。
---

# EPUB Style Demo 维护

当修改 `templates/epub-style-demo/`、新增阅读器兼容场景，或把阅读器发现转成最终 EPUB 生产规则时使用这个 skill。

## 固定工作流

1. 从 `templates/epub-style-demo/` 开始。新增或更新能暴露阅读器行为的最小 fixture。
2. 构建 demo：

```sh
sh templates/epub-style-demo/build.sh
```

3. 用 stdlib-only 脚本验证结构：

```sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

4. 把产物记录到 `docs/final/reader-matrix.yaml`。需要人工复测或版本待确认时使用 `warn`，不要虚构 pass/fail。
5. 只有 fixture 和 matrix 记录都存在后，才更新 `docs/final/SPEC-实现约束.md`，然后更新最终手册和速查表。
6. 如果规则影响自动化行为，同步更新相关 `skills/*/SKILL.md`，但不要改变 frontmatter 字段名。

## 当前兼容规则

- 图片环绕主路径使用 `figure.img-left` / `figure.img-right`。float 和百分比 `width` 挂在 `figure` 上，先从 `25%` 到 `35%` 调整，再结合目标阅读器、视口和字号实测。
- 不固定图片高度，也不把 `aspect-ratio` 作为主路径。真实图片通过 `height:auto` 保持宽高比，`figure` 也需要自然高度以承载图注。
- direct `img` float 不是主路径，因为部分阅读器会把图片渲染得过小。
- 书内图片以 JPEG / PNG 为生产主路径。WebP 只作为现代阅读器实验；demo WebP 在 Kindle conversion logs 中触发 W14012 / W14015。SVG 可作为增强测试，但 Kindle 目标构建需要在渲染不确定时提供 JPEG / PNG 栅格 fallback。
- 图文环绕测试需要足够长的周围正文。短段落只是阈值反例，不足以证明 float 失败。
- 波浪下划线必须拆开：先写 `text-decoration: underline;`，再写 `text-decoration-style: wavy;`。Kindle App fallback 为普通 underline。
- 含 MathML 的 XHTML manifest item 必须带 `properties="mathml"`。
- MathML 覆盖应保持在 KDP Enhanced Typesetting 和 EPUB 3 支持标签范围内，除非新实验有理由扩展。
- 多看旧版 fallback 使用 `ol.footnote-list.duokan-footnote-content`；单个 `li.footnote-item` 只加 `duokan-footnote-item`。
- 英文书籍规则先按类型拆分：小说/散文走 `.english-fiction`，非虚构后续单独 fixture；英文正文必须声明 `lang`，使用短 serif 链，首段无缩进、后续段缩进，未验证断字时不强制 justify。
- 英文小说插图默认居中 `figure`，不把图文环绕作为 fiction 主路径；首字优先用 `::first-letter` 保持正文单词完整，旧式 span 首字和 float drop cap 只作增强并需大字号复测。
- 章节头图属于普通可重排章首结构，放 `literary.css`；头图只做装饰，标题必须是真实 `h1`。小章标保守宽度，满栏横幅用 `width:100%` 铺满正文内容栏并由源图比例控制高度。
- 便签/资料卡主路径使用 border、background、padding 和 left-rule；box-shadow、inset、不规则圆角和 outline-offset 只作为可丢失增强。
- 通用 demo 不使用 `transform: rotate()` 旋转便签块；Kindle Previewer 3.104（2026-05-23 实测）会触发 KFX 增强排版内部错误。需要斜角感时用不对称边框、圆角和投影模拟。
- SVG 花边只作为 demo 实验项，用来验证简单内联 SVG 边线可行性，不作为推荐边框；长条投影框必须保留真实文本和边框兜底。

## 验证期望

提交前至少运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
git diff --check
```

如果本机没有 `xmllint`，Python 验证脚本仍会用标准库解析 XML，并捕获 manifest/link/MathML 错误。
