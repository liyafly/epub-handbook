---
name: epub-css-layering-optimizer
description: 重构和维护 EPUB CSS 分层。用于 CSS 规则重复、放错文件、层级边界不清，或新增正文、弹注、文字效果、文学结构、图片混排、竖排、A-lite 海报等样式时。
---

# EPUB CSS 分层优化

当 CSS 需要整理、拆分或修复，但不需要改正文内容时使用这个 skill。

## 固定目标

使用项目 CSS 分层契约：

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

加载顺序：

```text
fonts.css -> base.css -> notes/effects/literary/media/vertical/poster.css
```

## 工作流

1. 盘点目标 XHTML 引用、OPF 声明的所有 CSS。
2. 按 selector 和页面用途判断每条规则真正归属。
3. 把规则移动到正确层级：
   - 通用元素规则留在 `base.css`。
   - 可复用组件类放进拥有该组件的层。
   - 便签、摘录框、资料卡和装饰边框先按 `docs/guides/note-box-border-styles.md` 判断是否属于 `.note-box` 容器视觉。
   - A-lite shell 规则不要进入普通正文 CSS。
4. 更新 XHTML `<link>` 顺序，先加载依赖，再加载组件覆盖。
5. 新增或删除样式文件时同步 OPF manifest。
6. 只有确认没有目标 XHTML 依赖后，才删除重复 selector。
7. 保持 CSS 可读。某个层超过 400 行时开始评估职责是否过宽；超过 500 行必须按职责拆分或移入已有正确层。

## EPUB 安全写法

- 优先用 `em`、`%`；只有阅读器支持明确时再用其他单位。主路径不使用 viewport units。
- 不用 CSS Grid/Flexbox 承载关键阅读顺序或可见内容。
- 增强声明必须有基础 fallback：

```css
.wavy {
  text-decoration: underline;
  text-decoration-style: wavy;
}
```

- 普通正文页避免 `body { width: 100%; padding-left: ...; padding-right: ...; }`。
- 普通 `html` / `body`、`body.fullpage`、标题、图注和引用不写页面级 `color`、`background` 或 `background-color`；局部组件可保留必要的边框、阴影和背景装饰。
- A-lite 页把 `body.fullpage` shell 规则留在 `poster.css`。

## 禁止事项

- 不把字体声明移入排版布局文件。
- 不把 footnote、media、vertical、poster 规则写进 `base.css`。
- 不把下游引擎实现细节写进 skill 或 CSS 约定。
- 不因为现代浏览器不需要，就删掉 EPUB/WebKit 前缀 fallback。
- 不把无关 CSS 分层改动和正文改写混在一起。

## 验证 fixture

- `Text/01-body.xhtml`：base 正文规则。
- `Text/02-ruby-note.xhtml`：notes + Ruby 默认。
- `Text/10-text-effects.xhtml`：effects 层。
- `Text/19-border-shadow-notes.xhtml`：effects 层中的 `.note-box` 边框/阴影视觉增强。
- `Text/11-chapter-opening.xhtml`、`Text/12-literary-fiction.xhtml`、`Text/15-frontmatter.xhtml`：literary 层。
- `Text/17-image-layout.xhtml`：media 层。
- `Text/14-vertical-body.xhtml`：vertical 层。
- `Text/03-vertical-alite.xhtml`：poster 层。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
git diff --check
```
