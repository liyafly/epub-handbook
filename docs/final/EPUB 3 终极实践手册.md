# EPUB 3 终极实践手册

> 版本：2026-05-18  
> 定位：把现有主手册、补充篇、`wang-chapterpage-demo-v2.epub` 实测结果收敛为一套最终推荐方案。  
> 原则：只写推荐路径，围绕 A-lite、标准弹注、标准 CSS 和授权嵌入字体。

---

## 一、最终方案总览

| 模块 | 最终方案 |
|---|---|
| 正文 | EPUB 3.3 可重排，正文主字体可内嵌授权宋体类字体，系统字体作为 fallback |
| 整页海报 / 卷首 / 章节扉页 | A-lite：可重排整页、无 FXL、无 `vh/vw`、无绝对定位 |
| 标题 / 题签 / 特殊排版 | 内嵌授权字体，只用书内字体名 + 通用族兜底 |
| Apple Books 字体 | 嵌入字体 + OPF `ibooks:specified-fonts=true` + 测试“原版字体” |
| Kindle 字体 | 嵌入 `.ttf` / `.otf`，主字体放 `body`，测试 Publisher Font 开关 |
| 弹出注释 | 图片图标触发，单个 `aside epub:type="footnote"` 内用 `ol/li` 聚合本文件注释，`◎` 返回；需兼容多看旧版时在同一结构上叠加 `duokan-*` fallback |
| 波浪线 | `text-decoration: underline` 兜底 + `text-decoration-style: wavy` 渐进增强；Kindle 退化为普通下划线 |
| 着重号 | 标准 `text-emphasis: filled dot` |
| Ruby 注音 | 标准 `ruby + rt`，段落加行距兜底 |

---

## 二、推荐目录结构

```text
book.epub
├── mimetype
├── META-INF/
│   └── container.xml
└── OEBPS/
    ├── content.opf
    ├── toc.ncx
    ├── Text/
    │   ├── nav.xhtml
    │   ├── cover.xhtml
    │   ├── chapter-poster.xhtml
    │   └── ch01.xhtml
    ├── Styles/
    │   ├── fonts.css
    │   ├── base.css
    │   └── poster.css
    ├── Images/
    │   ├── cover.jpg
    │   ├── note.png
    │   └── poster-bg.png
    └── Fonts/
        ├── BookBodySong.ttf
        ├── BookTitleKai.ttf
        └── RareSongSubset.ttf
```

`fonts.css` 管字体，`base.css` 管正文组件，`poster.css` 管 A-lite 海报页。

---

## 三、OPF 模板

```xml
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf"
         version="3.0"
         unique-identifier="bookid"
         xml:lang="zh-CN"
         prefix="rendition: http://www.idpf.org/vocab/rendition/#
                 ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx</dc:identifier>
    <dc:title>书名</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-05-18T00:00:00Z</meta>
    <meta property="rendition:layout">reflowable</meta>
    <meta property="rendition:orientation">auto</meta>
    <meta property="rendition:spread">auto</meta>
    <meta property="ibooks:specified-fonts">true</meta>
    <meta name="cover" content="cover-img"/>
  </metadata>

  <manifest>
    <item id="nav" href="Text/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>

    <item id="css-fonts" href="Styles/fonts.css" media-type="text/css"/>
    <item id="css-base" href="Styles/base.css" media-type="text/css"/>
    <item id="css-poster" href="Styles/poster.css" media-type="text/css"/>

    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="poster-01" href="Text/chapter-poster.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch01" href="Text/ch01.xhtml" media-type="application/xhtml+xml"/>

    <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="note-icon" href="Images/note.png" media-type="image/png"/>
    <item id="poster-bg" href="Images/poster-bg.png" media-type="image/png"/>

    <item id="font-body-song" href="Fonts/BookBodySong.ttf" media-type="font/ttf"/>
    <item id="font-title-kai" href="Fonts/BookTitleKai.ttf" media-type="font/ttf"/>
    <item id="font-rare-song" href="Fonts/RareSongSubset.ttf" media-type="font/ttf"/>
  </manifest>

  <spine toc="ncx" page-progression-direction="ltr">
    <itemref idref="cover-page"/>
    <itemref idref="nav"/>
    <itemref idref="poster-01"/>
    <itemref idref="ch01"/>
  </spine>
</package>
```

规则：

- 全书默认 `reflowable`。
- A-lite 海报页仍是普通 spine item。
- 字体文件、注释图标、背景图都进入 `manifest`。
- 无论是否嵌入字体，Apple Books 路径都保留 `ibooks:specified-fonts=true`，避免用户偏好字体覆盖 CSS 字体链。

---

## 四、字体方案

### 4.1 `fonts.css`

```css
@charset "utf-8";

@font-face {
  font-family: "BookBodySong";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/BookBodySong.ttf") format("truetype");
}

@font-face {
  font-family: "BookTitleKai";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/BookTitleKai.ttf") format("truetype");
}

@font-face {
  font-family: "RareSong";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/RareSongSubset.ttf") format("truetype");
}
```

### 4.2 正文字体

```css
body {
  font-family: "BookBodySong", "Songti SC", "Source Han Serif SC", serif;
}
```

> 反例：上面的长链别名堆叠（如 `STSongti-*` / `NSimSun` / `宋体`）违反 SPEC §8，仅用于说明 anti-pattern。

正文采用书内字体优先，系统字体链兜底。iOS / Apple Books 对系统中文字体名命中不稳定，默认体验依靠 `BookBodySong`。

### 4.3 特殊标题字体

```css
.poster-title,
.inscription,
.title-kai {
  font-family: "BookTitleKai", serif;
}
```

特殊标题、题签、卷首页只写书内字体名 + 通用族兜底，避免系统字体提前替换设计字形。

### 4.4 生僻字

```css
.rare {
  font-family: "RareSong", "BookBodySong", serif;
}
```

生僻字字体只放子集。

---

## 五、正文基础样式

```css
@charset "utf-8";

html {
  font-size: 100%;
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  -webkit-box-sizing: inherit;
  box-sizing: inherit;
}

body {
  margin: 0;
  padding: 0 1em;
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
  font-size: 1em;
  line-height: 1.7;
  text-align: justify;
  font-family: "BookBodySong", "Songti SC", "Source Han Serif SC", serif;
}

p {
  margin: 0.6em 0;
  text-indent: 2em;
}

h1, h2, h3, h4, h5, h6 {
  line-height: 1.35;
  page-break-after: avoid;
}

h1 + p,
h2 + p,
h3 + p,
blockquote p,
figure + p,
hr + p {
  text-indent: 0;
}

img {
  max-width: 100%;
  height: auto;
}

code, pre, kbd, samp {
  font-family: "SF Mono", "Consolas", "Source Code Pro", monospace;
}
```

---

## 五点五、图片环绕兼容路径

图文环绕使用 `<figure>` 浮动作为主路径。Kindle App 实测 figure 也能环绕；关键是 figure 有明确固定宽度，并且前后正文足够长，让阅读器有足够行数展示绕排。direct `img` 直挂 float 不作为主路径，避免部分阅读器图片显示过小。

```html
<figure class="img-left">
  <img src="../Images/poster.png" alt="左浮动"/>
  <figcaption>图片说明。</figcaption>
</figure>
<p>
  正文从图片右侧环绕，段落要足够长才能观察绕排。
</p>
```

```css
figure.img-left {
  float: left;
  width: 240px;
  max-width: 45%;
  margin: 0 1em 0.6em 0;
  text-align: center;
}

figure.img-right {
  float: right;
  width: 240px;
  max-width: 45%;
  margin: 0 0 0.6em 1em;
  text-align: center;
}

figure.img-left img,
figure.img-right img {
  width: 100%;
  height: auto;
}
```

`width` 用固定 px；不要用 `em` 做 Kindle 主路径。`max-width` 只是给 Readest / Apple Books / Thorium 等非 Kindle 引擎的窄屏兜底，不能依赖 Kindle 一定执行。短段落无法证明环绕失败，实际测试要让正文至少有数行能贴住浮动图片。

---

## 六、A-lite 整页海报方案

### 6.1 XHTML

```xml
<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
<head>
  <title>分卷页</title>
  <link href="../Styles/fonts.css" rel="stylesheet" type="text/css"/>
  <link href="../Styles/poster.css" rel="stylesheet" type="text/css"/>
</head>
<body class="fullpage poster-bg">
  <section class="fullframe" epub:type="chapter">
    <h1 class="poster-title">汪曾祺全集</h1>
    <p class="poster-subtitle">①小说卷</p>
  </section>
</body>
</html>
```

### 6.2 CSS

```css
@charset "utf-8";

@page {
  margin: 0;
  padding: 0;
}

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
  background-color: #eceae7;
  background-image: url("../Images/poster-bg.png");
  background-repeat: no-repeat;
  background-position: left bottom;
  background-size: 80% auto;
}

> 约束：`body.fullpage` 仅负责页面骨架，不直接挂背景图；背景通过 `body.poster-bg` 等 modifier 类提供。

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

.poster-title,
.poster-subtitle {
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

.poster-title {
  clear: right;
  margin: 2% 4% 0 0;
  padding: 0;
  font-family: "BookTitleKai", serif;
  font-weight: normal;
  font-size: 260%;
  line-height: 1.12;
  letter-spacing: 0;
  color: #080400;
}

.poster-subtitle {
  clear: none;
  margin: 15% 4% 0 0;
  padding: 0;
  font-family: "BookTitleKai", serif;
  font-weight: normal;
  font-size: 160%;
  line-height: 1.25;
  letter-spacing: 0;
  color: #8f978a;
}
```

---

## 六点五、CSS 文件分层

- `fonts.css`：仅放 `@font-face` 与字体工具类（如 `.rare`）。
- `base.css`：正文组件（段落、列表、表格、注释、ruby）。
- `poster.css`：A-lite 海报页（`body.fullpage`、`body.poster-bg`、`.fullframe`、`.poster-title`、`.poster-subtitle`、`.vcol`）。

海报页建议链接 `fonts.css + poster.css`（可按需再链 `base.css`）；正文页链接 `fonts.css + base.css`。

## 七、弹出注释方案

### 7.1 XHTML

注释触发采用图片图标，图标放 `Images/note.png`。返回符号采用 `◎`。

同一个 XHTML 文件内只放一个注释容器：`aside epub:type="footnote"`。多条注释放在容器内的 `ol.footnote-list`，每条注释用 `li.footnote-item` 承载，正文 `noteref` 直接指向对应 `li` 的 `id`。这样保留 EPUB 3 标准弹注识别点，也保留 demo 的多注释聚合结构。

```xml
<p>
  正文内容
  <sup>
    <a id="note-1"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  第二处正文内容
  <sup>
    <a id="note-2"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-2">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>

  <ol class="footnote-list">
    <li class="footnote-item" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        第一条注释内容。
      </p>
    </li>

    <li class="footnote-item" id="footnote-2">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-2">◎</a>
        第二条注释内容。
      </p>
    </li>
  </ol>
</aside>
```

### 7.2 CSS

```css
sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon {
  text-decoration: none;
}

.noteref-icon img {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

.footnote-line {
  width: 60%;
  height: 1px;
  margin: 1.5em 0 1em -0.5em;
  border: none;
  border-top: 1px solid #777;
}

.footnote-list {
  margin: 0;
  padding: 0;
  list-style-type: none;
  text-align: left;
}

.footnote-item {
  margin: 0.4em 0;
  padding: 0;
  list-style-type: none;
}

.footnote {
  margin: 0.4em 0;
  text-indent: 0;
  font-size: 0.9em;
  line-height: 1.35;
  text-align: left;
  font-family: "BookTitleKai", "BookBodySong", serif;
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
```

这个结构同时保留标准弹注识别点和 demo 的视觉逻辑：正文点图片，同文件的 `aside` 统一承载本章注释，注释内用 `◎` 返回。不要使用多看私有类名或私有 CSS 作为主路径；如从旧多看结构转换，可以把原有 `ol/li` 视觉分组迁移成这里的中性类名。

### 7.3 叠加多看 fallback

只有目标 EPUB 明确需要多看旧版兼容时，才在标准结构上叠加多看类名；不要创建第二份注释容器。

```html
<p>
  需要注释的正文
  <sup>
    <a id="note-legacy-1"
       class="noteref-icon duokan-footnote"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-legacy-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  继续正文。
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line"/></div>
  <ol class="footnote-list duokan-footnote-content">
    <li class="footnote-item duokan-footnote-item" id="footnote-legacy-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-legacy-1">◎</a>
        注释正文仍只保留在同一个章末列表内。
      </p>
    </li>
  </ol>
</aside>
```

---

## 八、文字效果

### 8.1 着重号

```css
.emp {
  font-style: normal;
  text-emphasis: filled dot;
  -webkit-text-emphasis-style: filled dot;
  -epub-text-emphasis-style: filled dot;
  text-emphasis-position: under;
  -webkit-text-emphasis-position: under;
  -epub-text-emphasis-position: under;
}
```

```html
<span class="emp">着重内容</span>
```

### 8.2 波浪线

```css
.wavy {
  text-decoration: underline;
  text-decoration-style: wavy;
  -webkit-text-decoration-style: wavy;
  text-decoration-color: #c03030;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.12em;
}
```

```html
<span class="wavy">波浪线内容</span>
```

Kindle App 只显示基础 underline，不显示 wavy；这是预期降级。不要写 `text-decoration: underline wavy`，旧引擎可能把整条 declaration 丢弃，导致连下划线也没有。

### 8.3 Ruby 注音

```html
<p class="has-ruby">
  <ruby>字<rt>zì</rt></ruby>
</p>
```

```css
ruby {
  ruby-align: center;
}

rt {
  font-size: 0.5em;
  line-height: 1;
  font-family: "BookBodySong", sans-serif;
}

p.has-ruby {
  line-height: 1.9;
}
```

### 8.4 引用

```html
<p>他说：<q>文字要经得起换设备。</q></p>

<blockquote>
  <p>一段较长的引用。</p>
</blockquote>
```

```css
q,
blockquote {
  font-family: "BookTitleKai", "BookBodySong", serif;
}

blockquote {
  margin: 1em 0.5em;
  padding: 0.6em 1em;
  border-left: 3px solid #999;
}

blockquote p {
  text-indent: 0;
}
```

---

## 九、图片与封面

```html
<figure>
  <img src="../Images/example.jpg" alt="图像内容说明"/>
  <figcaption>图 1：说明文字</figcaption>
</figure>
```

```css
figure {
  margin: 1em 0;
  text-align: center;
}

figure img {
  max-width: 100%;
  height: auto;
}

figcaption {
  margin-top: 0.5em;
  font-size: 0.9em;
  text-indent: 0;
  text-align: center;
}
```

格式采用：

- 封面和照片：JPEG。
- 透明图、注释图标、贴图、截图：PNG。
- 简单矢量：SVG。

---

## 十、竖排

全书横排、部分页面竖排时，只在那几个页面局部写竖排类。OPF 的 `spine` 仍保持横排：

```xml
<spine toc="ncx" page-progression-direction="ltr">
```

局部竖排页面的 XHTML：

```xml
<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
<head>
  <title>竖排题词页</title>
  <link href="../Styles/fonts.css" rel="stylesheet" type="text/css"/>
  <link href="../Styles/base.css" rel="stylesheet" type="text/css"/>
</head>
<body class="page-vrl">
  <section class="vrl-section" epub:type="chapter">
    <h1 class="vrl-title">题词</h1>
    <p>一段竖排文字。</p>
    <p>第二段竖排文字。</p>
  </section>
</body>
</html>
```

```css
body.page-vrl {
  margin: 0;
  padding: 1em;
}

.vrl-section {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  line-height: 1.8;
  height: 100%;
}

.vrl-section p {
  text-indent: 2em;
  margin: 0 0 0 1em;
}

.vrl-title {
  font-family: "BookTitleKai", serif;
  font-weight: normal;
  line-height: 1.2;
  margin: 0 0 0 1.5em;
}
```

整本竖排书：

```xml
<spine toc="ncx" page-progression-direction="rtl">
```

局部章节扉页竖排时，保留整本 `page-progression-direction="ltr"`。

---

## 十点五、MathML

Kindle Enhanced Typesetting 支持 MathML。含 MathML 的 XHTML 必须在 OPF manifest 上声明 `properties="mathml"`。

```xml
<item id="math"
      href="Text/16-math.xhtml"
      media-type="application/xhtml+xml"
      properties="mathml"/>
```

demo 覆盖常用组合：`mfrac`、`msqrt`、`mroot`、`msub`、`msup`、`msubsup`、`mover`、`munder`、`munderover`、`menclose`、`mfenced`、`mtable`、`mlabeledtr`、`maligngroup`、`malignmark`、`semantics`、`annotation`、`mmultiscripts`、`ms`、`mspace`、`mstyle`、`mpadded`、`mphantom`。

不支持 MathML 的目标阅读器需要文本公式或图片公式 fallback；不要把复杂公式只保存在不可读的截图里。

---

## 十一、制作流程

1. 准备文本、封面、海报背景、注释图标、授权字体。
2. 写 `content.opf`，声明 reflowable、字体、图片、CSS、`ibooks:specified-fonts=true`。
3. 写 `fonts.css`，正文内嵌字体、标题字体、生僻字字体分开。
4. 写 `base.css`，正文、图片、注释、ruby、文字效果。
5. 写 `poster.css`，A-lite 海报页。
6. 写正文 XHTML 和海报 XHTML。
7. EPUBCheck 校验。
8. Apple Books 删除旧书后重新导入测试。
9. Kindle Previewer 转换并测试 Publisher Font 开关。
10. Thorium、Calibre、KOReader 抽测正文、注释、海报、字体、夜间模式。

---

## 十二、最终检查清单

### OPF

- [ ] `nav.xhtml` 有 `properties="nav"`。
- [ ] 需 Kindle/旧工具链兼容时，manifest 含 `toc.ncx` 且 `spine toc="ncx"`。
- [ ] 封面图使用 JPEG/PNG，并同时声明 `properties="cover-image"` 与 `<meta name="cover">`。
- [ ] 所有 XHTML / CSS / 图片 / 字体都进入 `manifest`。
- [ ] `rendition:layout` 是 `reflowable`。
- [ ] 有嵌入字体时写 `ibooks:specified-fonts=true`。
- [ ] `spine` 顺序正确。

### 字体

- [ ] 正文主字体为书内授权字体。
- [ ] 正文系统字体 fallback 链完整。
- [ ] 标题/题签使用独立书内字体。
- [ ] 生僻字使用子集字体。
- [ ] Kindle Previewer 测试 Publisher Font 开关。

### A-lite

- [ ] `body.fullpage` 有 `min-height:100%`。
- [ ] 有 `page-break-before/after/inside`。
- [ ] `body.fullpage` 有 `overflow:hidden`，`.fullframe` 保持 `overflow:visible`。
- [ ] 内部基准字号为 `16px`。
- [ ] 竖排使用 `writing-mode: vertical-rl`。
- [ ] 多列使用 `float:right`。

### 弹注

- [ ] 正文引用是图片图标。
- [ ] `<a>` 有 `epub:type="noteref"` 和 `role="doc-noteref"`。
- [ ] 每个含注释的 XHTML 有一个 `<aside epub:type="footnote" role="doc-footnote">` 注释容器。
- [ ] 多条注释放在同一个容器内的 `ol.footnote-list > li.footnote-item`。
- [ ] noteref 的 `href` 指向对应 `li.footnote-item` 的 `id`。
- [ ] 注释返回符号是 `◎`。
- [ ] noteref、注释 `li` 和外层 aside 在同一 XHTML 文件。

### 标准效果

- [ ] 波浪线使用 `text-decoration-style: wavy`。
- [ ] 着重号使用 `text-emphasis`。
- [ ] Ruby 使用 `ruby + rt`。
- [ ] 图片使用 `figure + img + figcaption`。

---

## 十三、参考

- Apple Books Asset Guide: [Fonts Overview](https://help.apple.com/itc/booksassetguide/en.lproj/itc74d42b31e.html)
- Apple Books Asset Guide: [Defining Book Layout Metadata](https://help.apple.com/itc/booksassetguide/en.lproj/itc2cf4d26eb.html)
- Amazon: [Kindle Publishing Guidelines](https://kindlegen.s3.amazonaws.com/AmazonKindlePublishingGuidelines.pdf?rw_useCurrentProtocol=1)
- W3C: [EPUB 3.3](https://www.w3.org/TR/epub-33/)
- MDN: [text-decoration-style](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/text-decoration-style)
- 本项目：`docs/experiments/EPUB 3 章节扉页与竖排实战 · 补充 05.md`


### 自检补充（A-lite / 弹注 / 字体）

- [ ] 根 `html` 含 `width/height/min-height:100%`。
- [ ] `body.fullpage` 不携带 `background-*`；背景通过 `poster-bg` 等 modifier 提供。
- [ ] `body.fullpage` 含 `-webkit-text-size-adjust:100%; text-size-adjust:100%`。
- [ ] `.fullframe` 骨架 `padding:0; overflow:visible`，留白靠内部元素 `margin`。
- [ ] 需多看旧版兼容时，noteref 锚带 `duokan-footnote` 且内含 `<img>`。
- [ ] 注释列表 `<ol>` 同时挂 `footnote-list duokan-footnote-content`。
- [ ] 每条 `li.footnote-item` 只额外挂 `duokan-footnote-item`，不重复挂 `duokan-footnote-content`。
- [ ] 任一 `font-family` 链 ≤ 4 段，嵌入字体放链首。
