---
name: epub-literary-structure-formatter
description: 格式化 EPUB 文学结构，包括章首、卷首、题记、题献、版权页、对话、诗、信件块和场景分隔。用于小说或散文 EPUB 需要语义 XHTML 与阅读器安全 CSS 润色，但不改写正文时。
---

# EPUB 文学结构格式化

这个 skill 处理 prose presentation 和书籍组成部分结构。它不负责弹注、图片环绕或 A-lite 海报骨架。

## 固定目标

使用语义 XHTML 加 `literary.css` 类：

- 章首使用 `section` / `header` 和少量章首类。
- 前置页使用合适的 `epub:type`。
- 题记、题献、信件、对话和诗都保留真实文本。
- 场景分隔使用文本或简单 CSS，不使用只有图片的装饰。
- 所有文学结构都保持可重排，并能承受阅读器字号缩放。
- 英文小说正文节奏、hyphenation 和 serif 字体链交给 `epub-english-typography-optimizer`；本 skill 只处理章首、题记、诗、信件等结构。

## XHTML 模式

章首：

```html
<section class="chapter-head" epub:type="chapter">
  <header>
    <p class="chapter-kicker">第一章</p>
    <h1>标题</h1>
    <p class="chapter-subtitle">副标题</p>
  </header>
  <blockquote class="epigraph">
    <p>题记文字。</p>
  </blockquote>
</section>
```

前置页：

```html
<section class="frontmatter copyright-page" epub:type="frontmatter copyright-page">
  <h1>版权信息</h1>
  <p>版权文字。</p>
</section>
```

诗：

```html
<div class="poetry" epub:type="z3998:poem">
  <p>第一行<br/>第二行</p>
</div>
```

## 工作流

1. 读取目标 XHTML 和 `literary.css`。
2. 识别书籍结构，不改写正文。
3. 在安全时把纯表现 wrapper 转成语义 section。
4. 使用最小类集合：
   - `chapter-head`、`chapter-kicker`、`chapter-subtitle`、`epigraph`
   - `dialog`、`poetry`、`letter`、`scene-break`
   - `copyright-page`、`dedication`、`epigraph-page`
5. 保持段落、换行和强调与源稿意图一致。
6. 所有新组件规则写进 `literary.css`。
7. 只有结构标题真实变化时才更新 nav 文案。

## 排版规则

- 用 margin 和 text alignment，不用 absolute positioning。
- 章首和前置页谨慎使用 page-break 控制。
- 诗行缩进使用相对单位，不使用固定像素列。
- 信件和 blockquote 要足够窄，便于小屏重排。
- 避免在大字号下挤压正文的装饰边框或花饰。

## 禁止事项

- 不改写对话标点、诗行、译文或正文。
- 用户没有要求 A-lite 时，不把章首转成海报页。
- 不把文学结构规则写进 `base.css`。
- 前置页属于阅读体验时，不从 nav 隐藏。
- 不使用依赖固定页面尺寸的 CSS。

## 验证 fixture

- `Text/11-chapter-opening.xhtml`
- `Text/12-literary-fiction.xhtml`
- `Text/15-frontmatter.xhtml`
- `Text/18-english-fiction.xhtml`

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```
