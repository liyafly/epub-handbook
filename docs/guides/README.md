# Guides

这个目录放仓库维护说明，不作为 `epub-pro` 技术架构的一部分。

## 当前文档

- `skills-and-templates.md`：skills 维护方式、当前手册补充建议、模板目录约定。
- `css-layering-plan.md`：把 `Styles/` 拆为 fonts/base/notes/effects/literary/media/vertical/poster 共 8 文件（或最小 4 文件版）的分层指南，含每页 link 矩阵、OPF manifest 写法、SPEC §7 改写清单。
- `fonts-css-expansion-plan.md`：`fonts.css` 重写为「系统字体优先 + 嵌入字体仅特定场景 + 注释骨架」的模板，含跨平台系统字体清单、`ibooks:specified-fonts` 通用预防默认策略，并给出 SPEC §8 与终极手册 §四的改写清单。
- `duokan-footnote-fallback-fix.md`：多看弹注 fallback 结构修正说明（`duokan-footnote-content` 应挂 `<ol>` 而非 `<li>`），含 SPEC/手册/skill/demo 改动清单与多看实测步骤。
- `demo-scene-expansion-plan.md`：demo 模板 10–17 共 8 个新场景（文字效果、章首、文学体、多看富文本、竖排正文、版权页、数学公式、图文九宫格）的 XHTML / 分层 CSS / nav / opf / ncx / SCENE_MATRIX / reader-matrix 改动清单。
- `english-fiction-layout.md`：英文正文排版与优化流程，覆盖 prose 类书籍的字体、段落、章首、首字、强调、插图和阅读器复测。
- `chapter-head-image.md`：普通可重排章节开头的小型头图、满栏横幅头图、kicker、真实标题和单书 fallback 规则。
- `anthology-navigation.md`：短篇全集、作品合集和分卷文集的总目录、局部目录、回本卷目录链接与 `(•)` 可访问性规则。
- `classical-modern-layout.md`：文白对照、原文/译文对照和双文本条目的 section 结构、字体类组合、局部目录与验证清单。
- `note-box-border-styles.md`：可重排便签、摘录框、阴影、不规则边缘和 SVG 花边实验的安全边界。
- `epub-cleanup-flow.md`：已有 EPUB 清洗流水线，覆盖健康检查、红线 gate、diff review、批量、回滚和 notes 模板。
- `epub-cleanup-patterns.md`：典型脏 EPUB 模式与推荐 skill 顺序。
- `asset-optimization.md`：图片与字体优化指南，覆盖 WebP 转换、无损压缩、字体子集化和封面红线。
- `epub-diff-tool.md`：纯静态 EPUB Diff Web App 的使用说明。

> 早期 plan 文档保留推导过程；已落地的规则以 `docs/final/`、`templates/epub-style-demo/`、`skills/` 和本目录的专题指南为准。

## 落地顺序（四份文档协作时的推荐顺序）

执行模型按以下顺序应用，可减少回头返工：

1. **`css-layering-plan.md`**：先把 `Styles/` 的 8 个文件骨架建好（即便是空文件也行），并在 OPF manifest 加齐 CSS item。原因：所有后续视觉类都要往这些文件里填，先建好骨架避免被迫一边写一边迁移。
2. **`fonts-css-expansion-plan.md`**：整体替换 `fonts.css`；OPF 的 `<meta property="ibooks:specified-fonts">true</meta>` 保留为通用预防默认（不删）；更新 `base.css` 的 `body / h1–h6 / code` 字体链到纯系统字体。
3. **`demo-scene-expansion-plan.md`**：写入 10–17 共 8 个新 XHTML，把视觉类填入对应 CSS 层（按本文 §4.2–§4.6 的分配），同步 nav / opf / ncx / SCENE_MATRIX / reader-matrix（cases / readers 列），跑构建产出 dist。
4. **`duokan-footnote-fallback-fix.md`**：把 05 / 06 demo 改成新 fallback 结构，跑构建后**先在多看实测**，通过后再回写 SPEC §1 / 终极手册 §7.3 / skill `epub-legacy-footnote-fallback`。
5. **SPEC / 手册 / skill 收尾**：在 1–4 全部跑通后，按 `css-layering-plan.md §5` 改 SPEC §7、按 `fonts-css-expansion-plan.md §6` 改 SPEC §8 与终极手册 §四 / §一总览 / §十二、按 `duokan-footnote-fallback-fix.md §4.1–4.3` 改 SPEC §1 / 终极手册 §7.3 / skill。

> 步骤 1–3 之间不要打乱顺序：先有骨架（1）再有字体策略（2）再填内容（3），否则视觉类会被迫先落到 `base.css`，事后再切。步骤 4 必须等多看实测；步骤 5 是文档收尾，先源码后规范，符合"demo 先行、文档后补"。

## 边界

- `docs/final/` 仍是对外稳定手册与约束层；SPEC §1 / §7 / §8 与终极手册的改动都通过本目录的 plan 文档驱动。
- `docs/source/` 和 `docs/experiments/` 仍保留推导、补充与实验过程。
- `templates/` 放可运行、可打包、可对比的 EPUB 样式样本；本目录的 plan 文档落地后，回写产物（XHTML、CSS、构建脚本输出）都进 `templates/`。
- `docs/final/epub-pro 技术架构 v1.md` 是架构正本，当前保持现状。
