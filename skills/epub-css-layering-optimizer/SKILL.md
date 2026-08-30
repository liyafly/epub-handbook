---
name: epub-css-layering-optimizer
description: 重构和维护 EPUB CSS 分层。用于 CSS 规则重复、放错文件、层级边界不清，或清洗既有 EPUB 的散乱 CSS、合并互不重叠的局部样式表时；机器清理走能力，新增样式归层按本 skill 契约判断。
---

# EPUB CSS 分层优化

## 何时用

- CSS 需要整理、拆分或修复，但不需要改正文内容时。两条路径：
  - 清洗既有 EPUB 的散乱 CSS（去重、归并、manifest 同步）：机器清理走 `epub.css.layering.optimize`。
  - 新增或手工调整样式时按本 skill 的分层契约选择正确文件：这属于 AI 判断，不由能力替代。
- 分层契约（每条规则按 selector 和页面用途归位）：

  | 文件 | 职责 |
  |---|---|
  | `fonts.css` | `@font-face`、字体工具类、嵌入字体专用 helper |
  | `base.css` | `@page`、`html/body`、标题、段落、列表、表格、代码、基础 `figure/img`、内联语义、默认 Ruby、`.has-ruby` 行距兜底 |
  | `notes.css` | 标准 footnote、popup note、多看 fallback 类 |
  | `effects.css` | 着重号、波浪线 fallback、首字下沉、`.note-box` 边框/阴影视觉增强 |
  | `literary.css` | 章首、章节头图、小章标、满栏横幅、题记、对话、诗、信件、文白对照、场景分隔、前置页 |
  | `media.css` | 图片浮动网格、图文环绕、公式与 math block |
  | `vertical.css` | 非海报整页竖排正文 |
  | `poster.css` | A-lite 骨架、海报背景、全页叠加文字 |

  加载顺序：`fonts.css -> base.css -> notes/effects/literary/media/vertical/poster.css`；XHTML `<link>` 先依赖后覆盖；增删样式文件时同步 OPF manifest。
- 作用域归并规则（清洗既有书时）：多个局部 CSS 的引用页面集合互不重叠时，可改写为 `body.css-local-*` 作用域并归并到一个 `clean-scoped-local.css`；引用集合有交叠时必须跳过并报告，不猜测级联优先级。
- EPUB 安全写法：优先 `em`、`%`，主路径不用 viewport units；不用 Grid/Flexbox 承载关键阅读顺序或可见内容；增强声明必须带基础 fallback（`.wavy` 先 `text-decoration: underline;` 再 `text-decoration-style: wavy;`）；普通正文页避免 `body { width:100%; padding-left:...; padding-right:...; }`；普通 `html`/`body`、`body.fullpage`、标题、图注和引用不写页面级 `color`/`background`/`background-color`，局部组件可保留必要边框阴影装饰；A-lite shell 规则留在 `poster.css`。
- 保持 CSS 可读：selector、花括号和每条 declaration 分行，一致缩进；某层超过 400 行开始评估职责，超过 500 行必须拆分或移入已有正确层。
- 禁止事项：
  - 不把字体声明移入排版布局文件；不把 footnote、media、vertical、poster 规则写进 `base.css`。
  - 不把下游引擎实现细节写进 skill 或 CSS 约定；不因现代浏览器不需要就删掉 EPUB/WebKit 前缀 fallback。
  - 不把无关 CSS 分层改动和正文改写混在一起；只有确认没有目标 XHTML 依赖后才删除重复 selector。
  - 便签、摘录框、资料卡和装饰边框先按 `docs/how-to/note-box-border-styles.md` 判断是否属于 `.note-box` 容器视觉。

## 调什么

```sh
# 先 dry-run 审查计划改动
epub run epub.css.layering.optimize --input <书> --output <新书> --dry-run --json

# 确认后实跑；需要作用域归并时显式开启
epub run epub.css.layering.optimize --input <书> --output <新书> --json merge_scoped_local_css=true
```

写型能力：`--output` 必填且指向新文件。需要旧报告形状明细时加 `legacy_report=true`。

改后校验（能力内置红线已覆盖 text/metadata/spine/anchors/cover；需要 DRM 或字体混淆口径时再跑两文件比对）：

```sh
epub redline --check all <before.epub> <after.epub>
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（缺 `--output`、输出与输入相同等）。
- facts 键前缀 `epub.css.layering.optimize.`：
  - `cssFilesBefore` / `cssFilesAfter`：清理前后样式表数量。
  - `factoredStylesheets`、`duplicateStylesheetsRemoved`、`overridesCreated`：拆分、去重与 override 生成量。
  - `fontDeclarationsRewritten`、`xhtmlFilesUpdated`：字体声明改写量与 XHTML 更新数。
  - `cssManifestItemsRemoved` / `cssManifestItemsAdded`：OPF manifest 同步量。
  - `scopedLocalStylesheetsMerged`、`scopeClassesAdded`：作用域归并量与新增的 `css-local-*` 类。
  - `warnings`、`mergeScopedLocalCss`（开关回显）。
- findings：`warn css_cleanup.warning`（跳过归并、保留的歧义等）；run 内置红线失败时出现 `error redline.<check>`。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（旧清理报告）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- dry-run `status == approval-required`（退出码 2）→ review facts 中各项计数与 `warnings`，确认归并/删除范围合理后实跑。
- `cssFilesAfter > cssFilesBefore` 且无 warning → 通常是拆分/override 生成，核对 manifest 增减是否与 XHTML 引用一致。
- `scopedLocalStylesheetsMerged == 0` 且存在引用集合交叠 → 属预期保护，不强行合并；需要时人工按层拆写。
- `warn css_cleanup.warning` → 逐条复核跳过原因；不允许为凑整洁猜测级联优先级。
- `findings` 出现 `error redline.*` → 停止：输出保留供人工 diff review，先修源再重跑；不允许用宽泛 allow-list 掩盖。
- 手工归层判据（能力不覆盖的新增样式）：通用元素规则留 `base.css`；可复用组件类放进拥有该组件的层；`@font-face` 与字体工具类进 `fonts.css`；A-lite shell 不进普通正文 CSS。
- 红线通过后用 Calibre Editor 或 VS Code 做人工 diff review，再继续后续精排能力。
- 验证 fixture（改 demo 模板时对照层职责）：`Text/01-body.xhtml`（base）、`Text/02-ruby-note.xhtml`（notes+Ruby）、`Text/10-text-effects.xhtml` 与 `Text/19-border-shadow-notes.xhtml`（effects）、`Text/15-frontmatter.xhtml`/`Text/18-english-fiction.xhtml`/`Text/20-chapter-head-image.xhtml`/`Text/21-classical-modern.xhtml`（literary）、`Text/17-image-layout.xhtml`（media）、`Text/14-vertical-body.xhtml`（vertical）、`Text/03-vertical-alite.xhtml` 与 `Text/03c-poster-contain.xhtml`（poster）；demo 构建与验证由 `epub-style-demo-maintainer` 处理。
