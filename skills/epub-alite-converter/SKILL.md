---
name: epub-alite-converter
description: 将 EPUB 封面式页面、卷首、章首或海报页转换为项目的 A-lite 可重排全页方案，同时保留原有图文叠加构图、文字、图片和资源。
---

# EPUB A-lite 转换

当需要把 EPUB 中已有的封面式页面、卷首、章节标题页或海报页转换成项目最终 A-lite 方案时使用这个 skill。

## 固定目标

严格使用项目 A-lite 方案：

- reflowable EPUB 页面。
- `body.fullpage`。
- `.fullframe`。
- `min-height: 100%`。
- `font-size: 16px`。
- `overflow: hidden`。
- `page-break-before/after/inside`。
- 背景属于 `body.poster-bg` 或其他 `poster-*` modifier，不属于 `body.fullpage`。
- 竖排文字使用 `writing-mode: vertical-rl`。
- 竖排列使用 `float: right`。
- 不转成 FXL。
- 不使用 `vh` / `vw`。
- 不使用 absolute positioning。

## 工作流

1. 读取目标页的源 XHTML、关联 CSS、OPF manifest/spine 和资源。
2. 识别已有构图：
   - 背景图。
   - 叠加文字块。
   - 装饰性叠加图片。
   - 竖排列。
   - 字体名和嵌入字体用法。
3. 保留构图。不要重设设计、重排内容、改写文字、替换图片或新增装饰。
4. 转换页面外壳：

```html
<body class="fullpage poster-bg">
  <section class="fullframe" epub:type="chapter">
    ...
  </section>
</body>
```

5. 把竖排叠加文字转成浮动竖列：

```css
.fullframe .vcol {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  float: right;
  text-indent: 0;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}
```

6. 转换定位：
   - 保持大体视觉顺序。
   - 把大块固定 offset 转成 `%` margin。
   - 把标题字号转成 `%` 或 `em`。
   - 内部基准 `font-size` 保持 `16px`。
7. 保留嵌入字体。锁定题名字体时，书内字体名优先，通常只接 `serif` / `sans-serif` 作为 generic fallback。
8. 保持 CSS 分层：A-lite CSS 必须放入 `Styles/poster.css`，不要写进 `base.css`。
9. OPF manifest 只为实际使用的 assets/CSS/fonts 增减条目；存在 A-lite 时分开声明 `fonts.css` / `base.css` / `poster.css`。保留已有 `nav.xhtml`、`toc.ncx`、`spine toc="ncx"` 和 cover-image metadata。
10. 读取输出 XHTML/CSS，确认所有必须保留的叠加文字和图片没有丢失。

## A-lite CSS 骨架

```css
@page { margin: 0; padding: 0; }

html {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  box-sizing: inherit;
}

body.fullpage {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  font-size: 16px;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;
  box-sizing: border-box;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-page-break-inside: avoid;
  overflow: hidden;
}

body.poster-bg {
  background-image: url("../Images/poster-bg.png");
  background-repeat: no-repeat;
  background-position: left bottom;
  background-size: 80% auto;
}

.fullframe {
  width: 100%;
  height: auto;
  min-height: 90%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
  overflow: visible;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}
```

## 禁止事项

- 不转成 fixed layout。
- 不用 padding-ratio、FXL 或纯整页图片替代项目 A-lite 方案。
- 不因为定位困难就删除叠加文字。
- 不新增营销式装饰或新的视觉概念。
- 不把私有阅读器 CSS 作为主路径。

## 验证 fixture

使用 `templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml` 作为本地 A-lite 输出参考。转换页应满足这些不变量：

- 有海报背景时使用 `<body class="fullpage poster-bg">`。
- `body.fullpage` 承载 shell 规则，`body.poster-bg` 承载背景规则。
- `body.fullpage` 不写页面级 `color`、`background` 或 `background-color`；`body.poster-bg` 只写背景图、位置和尺寸。
- `.fullframe` 包含叠加内容。
- `.fullframe` 保持 `padding:0`；叠加文字用元素 margin 定位，不给页面骨架加 padding。
- 竖排叠加文字使用 `writing-mode: vertical-rl` 和前缀 fallback。
- 不引入 absolute positioning 或 fixed-layout package metadata。
