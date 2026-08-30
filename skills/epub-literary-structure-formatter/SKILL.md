---
name: epub-literary-structure-formatter
description: 格式化 EPUB 文学结构，包括章首、卷首、题记、题献、版权页、对话、诗、信件块、文白对照条目和场景分隔。用于小说、散文或古籍 EPUB 需要语义 XHTML 与阅读器安全 CSS 润色，但不改写正文时。
---

# EPUB 文学结构格式化

## 何时用

- 小说、散文或古籍 EPUB 的 prose presentation 与书籍组成部分结构：章首、章节头图、前置页、题记、题献、对话、诗、信件、文白对照、场景分隔。
- 职责边界：不负责弹注（`epub-popup-footnote-converter`）、图片环绕（`epub-image-layout-optimizer`）、A-lite 海报骨架（`epub-alite-converter`）、英文正文节奏与断字（`epub-english-typography-optimizer`）、OPF/nav（`epub-package-nav-auditor`）。
- 规则来源：章首头图见 [docs/how-to/chapter-head-image.md](../../docs/how-to/chapter-head-image.md)，大合集/分卷/局部目录见 [docs/how-to/anthology-navigation.md](../../docs/how-to/anthology-navigation.md)，文白对照见 [docs/how-to/classical-modern-layout.md](../../docs/how-to/classical-modern-layout.md)。

## 调什么

本 skill 是 AI 分析与手工精排类 skill：读目标 XHTML 与 `literary.css`，识别书籍结构后做最小语义化改写。改书后必须跑校验组合：

```sh
epub run epub.notes.popup.normalize --input <产物> --json    # 涉及弹注时
epub run epub.style.demo.maintain --input <demo 产物> --json # 涉及 demo 模板时（能力迁移中：findings 出现 warn capability.not-implemented 表示校验逻辑尚未执行）
epub redline --check all <before.epub> <after.epub>          # 每次改书后
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`、`text_files`、`violations`；violations 对应 `error popupnotes` findings。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过。

## 依据返回怎么判断

- 固定目标：语义 XHTML 加 `literary.css` 类。章首用 `section`/`header` 和少量章首类；普通章节头图用 `figure.chapter-head-art` 小章标或 `figure.chapter-head-banner` 满栏横幅 + 真实 `h1`；前置页用合适 `epub:type`；题记、题献、信件、对话、诗都保留真实文本；场景分隔用文本或简单 CSS，不用只有图片的装饰；所有文学结构保持可重排并承受字号缩放。
- 文白对照：条目级 `section`，每组直接放原文段落和白话段落；默认上下，短组只在宽屏 `min-width` 增强中用双侧 `float`。默认文白 38/58；原译接近可加 `.parallel-ratio-balanced` 用 48/48；原文较长可加 `.parallel-ratio-source-wide` 用 58/38；单书自定义比例两栏总和建议不超过 96%。长组和 Kindle 窄屏用 `.parallel-stack-pair` 保持上下。不用 table、flex、grid 或固定版式。
- 最小类集合：`chapter-head`、`chapter-head-art`、`chapter-head-banner`、`chapter-kicker`、`chapter-subtitle`、`epigraph`、`dialog`、`poetry`、`letter`、`scene-break`、`classical-modern`、`parallel-entry`、`parallel-pair`、`parallel-float-pair`、`parallel-ratio-balanced`、`parallel-ratio-source-wide`、`parallel-stack-pair`、`parallel-clear`、`classical-text`、`modern-text`、`copyright-page`、`dedication`、`epigraph-page`。
- 排版规则：用 margin 和 text alignment，不用 absolute positioning；章首和前置页谨慎使用 page-break；诗行缩进用相对单位；信件和 blockquote 足够窄；避免大字号下挤压正文的装饰边框或花饰。
- XHTML 纪律：写回保持两空格缩进、多行块级结构和 XML-valid；正文、诗行及其他 mixed-content 的文字不得因美化被拆分或改写；保持段落、换行和强调与源稿意图一致；只有结构标题真实变化时才更新 nav 文案；所有新组件规则写进 `literary.css`。
- 禁止事项：不改写对话标点、诗行、译文或正文；用户没要求 A-lite 时不把章首转成海报页；不把文学结构规则写进 `base.css`；前置页属于阅读体验时不从 nav 隐藏；不使用依赖固定页面尺寸的 CSS。
- `findings` 出现 `error`（含 `popupnotes`、`redline.*`）→ 回滚或修复后重跑；红线通过后做人工 diff review。demo 相关新规则先落 fixture（`Text/15-frontmatter.xhtml`、`Text/18-english-fiction.xhtml`、`Text/20-chapter-head-image.xhtml`、`Text/21-classical-modern.xhtml`），实测后回写 `docs/final/reader-matrix.yaml`。
- `status == approval-required` → 停下来问人。
