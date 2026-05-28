# EPUB 3 制作完全参考手册
 
> 用于自制兼容性最佳的 EPUB 3 电子书。覆盖 XHTML 元素、CSS 属性、避坑清单、文件结构、工具链与验证流程。
> 适用于:小说、随笔、技术书、图文书等可重排内容。固定版式 (FXL) 不在此手册范围。
 
---
 
## 目录
 
1. [核心原则](#一核心原则)
2. [文件结构](#二文件结构)
3. [XHTML 元素速查表](#三xhtml-元素速查表)
4. [文本语义标签详解](#四文本语义标签详解)
5. [CSS 属性速查表](#五css-属性速查表)
6. [基础样式表模板](#六基础样式表模板)
7. [常见组件 HTML 写法](#七常见组件-html-写法)
8. [中文排版专项](#八中文排版专项)
9. [避坑清单](#九避坑清单)
10. [工具链推荐](#十工具链推荐)
11. [验证与测试清单](#十一验证与测试清单)
 
---
 
## 一、核心原则
 
1. **用 EPUB 3 标准,但只用"保守子集"**:不依赖 JS、不依赖固定版式、不依赖音视频做必需内容
2. **用 XHTML 严格语法**:所有标签闭合、所有属性加引号
3. **用语义标签**:让阅读器和无障碍工具理解结构
4. **用基础 CSS**:避免 Flexbox / Grid / position:absolute,改用 inline-block / float / columns 等老牌特性
5. **让阅读器接管响应式**:不写媒体查询,用相对单位和弹性布局
6. **同时保留 NCX (toc.ncx) 和 EPUB 3 nav**:兼容老阅读器
7. **完成后必过 EPUBCheck**,且至少在 4 个阅读器实测
 
---
 
## 二、文件结构
 
标准 EPUB 3 目录结构:
 
```
mybook.epub  (本质是 zip 压缩包)
├── mimetype                    (必须第一个文件,不压缩,内容固定)
├── META-INF/
│   └── container.xml           (指向 OPF 文件位置)
└── OEBPS/  (或 EPUB/)
    ├── content.opf             (元数据 + 文件清单 + 阅读顺序)
    ├── toc.ncx                 (EPUB 2 目录,兼容老阅读器)
    ├── nav.xhtml               (EPUB 3 目录)
    ├── styles/
    │   └── main.css
    ├── fonts/
    │   └── *.otf / *.ttf
    ├── images/
    │   └── *.jpg / *.png / *.svg
    └── text/
        ├── cover.xhtml
        ├── titlepage.xhtml
        ├── copyright.xhtml
        ├── ch01.xhtml
        ├── ch02.xhtml
        └── ...
```
 
**XHTML 文件标准头部**:
 
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" 
      xmlns:epub="http://www.idpf.org/2007/ops"
      lang="zh-CN" xml:lang="zh-CN">
<head>
  <meta charset="UTF-8"/>
  <title>章节标题</title>
  <link rel="stylesheet" type="text/css" href="../styles/main.css"/>
</head>
<body epub:type="bodymatter chapter">
  <!-- 内容 -->
</body>
</html>
```
 
---
 
## 三、XHTML 元素速查表
 
### 结构元素
 
| 用途 | 标签 | 备注 |
|---|---|---|
| 文档根 | `<html xmlns lang xml:lang>` | 必须带命名空间 |
| 章节容器 | `<section epub:type="chapter">` | 配 epub:type 标语义 |
| 文章块 | `<article>` | 独立内容单元 |
| 旁注/侧栏 | `<aside>` | 配合 epub:type 区分用途 |
| 导航 | `<nav epub:type="toc">` | 用于 nav.xhtml |
| 章节标题 | `<h1>` ~ `<h6>` | 严格按层级,不跳级 |
| 段落 | `<p>` | 不要用 div 代替 |
| 横线分隔 | `<hr/>` | 自闭合 |
| 通用块 | `<div>` | 仅当无更合适语义标签时用 |
| 通用行内 | `<span>` | 仅当无更合适语义标签时用 |
 
### 文本语义元素
 
| 用途 | 标签 | 备注 |
|---|---|---|
| 语气强调 | `<em>` | 屏幕朗读会加重 |
| 重要性强调 | `<strong>` | 警告、关键信息 |
| 区分性斜体 | `<i>` | 书名、外语、学名、术语,无强调含义 |
| 区分性加粗 | `<b>` | 关键词高亮、产品名首现,无重要性 |
| 作品标题 | `<cite>` | 比 `<i>` 更精确 |
| 术语定义 | `<dfn>` | 术语首次出现并定义时 |
| 高亮标记 | `<mark>` | 类似荧光笔 |
| 缩写 | `<abbr title="">` | 配 title 显示全称 |
| 删除线 | `<s>` | 不再适用的内容 |
| 下划线 | `<u>` | 慎用,易与链接混淆 |
| 附属小字 | `<small>` | 版权、注解、免责声明 |
| 注音 | `<ruby><rt>` | 拼音、振假名 |
| 短引文 | `<q>` | 行内,自动加引号 |
| 长引文 | `<blockquote>` | 块级 |
| 行内代码 | `<code>` | 等宽字体 |
| 代码块 | `<pre><code>` | 保留空白换行 |
| 键盘按键 | `<kbd>` | 如 Ctrl+C |
| 变量名 | `<var>` | 数学/编程变量 |
| 程序输出 | `<samp>` | 计算机输出 |
 
### 媒体与图形
 
| 用途 | 标签 | 备注 |
|---|---|---|
| 图片 | `<img alt/>` | alt 必填,自闭合 |
| 图+说明 | `<figure>` + `<figcaption>` | 配套用 |
| 矢量图 | `<svg>` | 直接嵌入或外链 |
| 数学公式 | `<math xmlns altimg>` | MathML,备 SVG 后备 |
 
### 列表与表格
 
| 用途 | 标签 | 备注 |
|---|---|---|
| 有序列表 | `<ol><li>` | |
| 无序列表 | `<ul><li>` | |
| 定义列表 | `<dl><dt><dd>` | 术语表、API 参数 |
| 表格 | `<table><thead><tbody><tr><th><td>` | 必须分 thead / tbody |
| 表格说明 | `<caption>` | 表格内首个子元素 |
 
### 链接与脚注
 
| 用途 | 标签 | 备注 |
|---|---|---|
| 链接 | `<a href="ch.xhtml#id">` | 内部跳转用相对路径 |
| 锚点 | `id="xxx"` | 加在任意元素,不用 name |
| 脚注引用 | `<a epub:type="noteref" href="#fn1">` | |
| 脚注内容 | `<aside epub:type="footnote" id="fn1">` | |
 
### EPUB 3 专用语义 (epub:type)
 
| 用途 | 写法 | 备注 |
|---|---|---|
| 封面 | `<section epub:type="cover">` | |
| 题献页 | `<section epub:type="dedication">` | |
| 版权页 | `<section epub:type="copyright-page">` | |
| 序言 | `<section epub:type="preface">` | |
| 前言 | `<section epub:type="foreword">` | |
| 引言 | `<section epub:type="introduction">` | |
| 正文章节 | `<section epub:type="chapter">` | |
| 附录 | `<section epub:type="appendix">` | |
| 术语表 | `<section epub:type="glossary">` | |
| 索引 | `<section epub:type="index">` | |
| 参考书目 | `<section epub:type="bibliography">` | |
| 目录 | `<nav epub:type="toc">` | |
| 页码列表 | `<nav epub:type="page-list">` | |
| 地标列表 | `<nav epub:type="landmarks">` | |
 
---
 
## 四、文本语义标签详解
 
`<em>` `<strong>` `<i>` `<b>` 这四个标签在 HTML5 中**全部合法且语义不同**,不是新旧替换关系。
 
### 强调类(影响朗读语气)
 
- **`<em>`**:句子中需要重读、强调语气的部分
  - 例:你**真的**这么想吗?
- **`<strong>`**:内容本身的重要性,关键警示
  - 例:**警告**:操作不可逆
 
### 区分类(只是视觉区分,不影响语气)
 
- **`<i>`**:书名、外语词、学名、术语、内心独白
  - 例:`<i>The Great Gatsby</i>`、`<i>Homo sapiens</i>`、`<i>déjà vu</i>`
- **`<b>`**:关键词、产品名首现、需要醒目但不"重要"的内容
 
### 选择原则
 
| 想表达 | 用什么 |
|---|---|
| "这里要加重读音" | `<em>` |
| "这件事非常重要" | `<strong>` |
| "这是个书名/外语词/术语" | `<i>` 或更精确的 `<cite>` `<dfn>` |
| "想加粗显示但不是强调" | `<b>` |
 
**为什么这区分重要**:屏幕朗读器对 `<em>` `<strong>` 会改变语调,对 `<i>` `<b>` 不会。如果用错(比如把书名标成 `<em>`),视障读者每次听到书名都会被加重读音,体验变差。
 
---
 
## 五、CSS 属性速查表
 
### 字体与文字
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 字号 | `font-size: 1em / 1.25em / 1.5em` | 用 em,不用 px |
| 字体 | `font-family: "中文字体", "西文字体", serif` | 长回退链 |
| 行距 | `line-height: 1.6 ~ 1.8` | 中文 1.7 较舒适 |
| 字重 | `font-weight: normal / bold` | 不用数字值 |
| 字形 | `font-style: italic / normal` | 中文谨慎用 italic |
| 首行缩进 | `text-indent: 2em` | 中文段落标准 |
| 对齐 | `text-align: justify / center / left` | |
| 字间距 | `letter-spacing: 0.05em` | 谨慎用 |
| 词间距 | `word-spacing` | 西文调整 |
| 西文断词 | `hyphens: auto` | 加 -webkit- 前缀 |
| 中文着重号 | `text-emphasis: dot` | 加 -webkit- 前缀 |
| 文字阴影 | `text-shadow` | 慎用,墨水屏不显示 |
| 文字变换 | `text-transform: uppercase / lowercase` | |
| 文字方向 | `direction: ltr / rtl` | 阿拉伯文等用 |
| 行内对齐 | `vertical-align: super / sub / middle` | 上下标 |
 
### 间距与边框
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 外边距 | `margin: 1em 0` | 段间距 |
| 内边距 | `padding: 0.5em 1em` | |
| 边框 | `border: 1px solid #888` | |
| 单边边框 | `border-left / right / top / bottom` | 引文常用左边框 |
| 圆角 | `border-radius: 3px` | 谨慎,部分阅读器忽略 |
| 居中块 | `margin: 0 auto` + `text-align: center` | 替代 flex 居中 |
 
### 布局(基础替代方案)
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 块级显示 | `display: block` | |
| 行内块 | `display: inline-block` | 替代 flex 并排 |
| 浮动 | `float: left / right` | 文字环绕图片 |
| 清除浮动 | `clear: both` | |
| 多列布局 | `column-count: 2; column-gap: 1.5em` | 索引、术语表 |
| 隐藏 | `display: none` | |
| 块宽度 | `width: 48%` / `max-width: 100%` | 用百分比 |
| 块高度 | `height: auto` | 让内容决定 |
 
### 颜色与背景
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 文字颜色 | `color` | 慎用,留给阅读器 |
| 背景色 | `background: #f5f5f5` | 浅色,避免深色 |
| 透明度 | `opacity` | 谨慎用 |
 
### 图片相关
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 图片自适应 | `max-width: 100%; height: auto` | 必加 |
| 块级图片 | `display: block` | 配合 margin 居中 |
| 图片居中 | `margin: 0 auto` | block 配合用 |
 
### 表格相关
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 折叠边框 | `border-collapse: collapse` | |
| 表格满宽 | `width: 100%` | |
| 表格布局 | `table-layout: auto / fixed` | |
| 单元格对齐 | `vertical-align: top / middle` | |
| 横向滚动 | `overflow-x: auto` | 包在外层 div 上 |
 
### 代码块
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 等宽字体 | `font-family: "Consolas", monospace` | |
| 保留空白 | `white-space: pre` | |
| 强制换行 | `word-wrap: break-word` | 如果不想横向滚动 |
| 超出滚动 | `overflow-x: auto` | 推荐用法 |
 
### 分页控制
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 章节强制分页 | `page-break-before: always` | 章首 |
| 避免内部分页 | `page-break-inside: avoid` | 图、表、代码 |
| 标题不孤立 | `page-break-after: avoid` | 标题后跟内容 |
 
### 伪元素与伪类
 
| 用途 | 推荐属性 | 备注 |
|---|---|---|
| 前后插入内容 | `::before` / `::after { content: }` | 加图标、引号 |
| 首字下沉 | `::first-letter` | 文章开头装饰 |
| 首行 | `::first-line` | |
| 隔行变色 | `tr:nth-child(even)` | 表格 |
| 第一个子元素 | `:first-child` | |
| 链接状态 | `:link` `:visited` | |
| 语言匹配 | `:lang(zh)` | 多语言区分样式 |
 
---
 
## 六、基础样式表模板
 
```css
@charset "UTF-8";
 
/* === 全局 === */
html { font-size: 100%; }
 
body {
  font-family: "Source Han Serif SC", "Noto Serif CJK SC", 
               "Songti SC", "SimSun", serif;
  font-size: 1em;
  line-height: 1.7;
  margin: 0;
  padding: 0 1em;
  text-align: justify;
  -webkit-hyphens: auto;
  hyphens: auto;
}
 
/* === 标题层级 === */
h1, h2, h3, h4, h5, h6 {
  font-family: "Source Han Sans SC", "Noto Sans CJK SC", 
               "PingFang SC", sans-serif;
  font-weight: bold;
  line-height: 1.3;
  page-break-after: avoid;
  -webkit-hyphens: none;
  hyphens: none;
}
 
h1 {
  font-size: 2em;
  text-align: center;
  margin: 2em 0 1.5em;
  page-break-before: always;
}
 
h2 {
  font-size: 1.5em;
  margin: 2em 0 0.8em;
  border-bottom: 1px solid #888;
  padding-bottom: 0.3em;
}
 
h3 { font-size: 1.25em; margin: 1.5em 0 0.6em; }
h4 { font-size: 1.1em;  margin: 1.2em 0 0.4em; }
 
/* === 段落 === */
p {
  margin: 0;
  text-indent: 2em;
}
 
/* 章首段不缩进 */
h1 + p, h2 + p, h3 + p, 
hr + p, blockquote + p {
  text-indent: 0;
}
 
/* === 文本语义标签 === */
em       { font-style: italic; }
strong   { font-weight: bold; }
i        { font-style: italic; }
b        { font-weight: bold; }
cite     { font-style: italic; }
dfn      { font-style: italic; font-weight: bold; }
mark     { background: #fff3a0; padding: 0 0.1em; }
abbr     { border-bottom: 1px dotted; cursor: help; }
small    { font-size: 0.85em; }
 
/* 中文特殊处理:斜体改用着重号 */
:lang(zh) em,
:lang(zh) i,
:lang(zh) cite { 
  font-style: normal; 
  text-emphasis: dot;
  -webkit-text-emphasis: dot;
}
 
/* === 引文 === */
blockquote {
  margin: 1em 2em;
  padding: 0.5em 1em;
  border-left: 3px solid #888;
  font-style: italic;
}
 
blockquote p { text-indent: 0; }
 
/* === 图片 === */
img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em auto;
}
 
figure {
  margin: 1.5em 0;
  text-align: center;
  page-break-inside: avoid;
}
 
figcaption {
  font-size: 0.9em;
  font-style: italic;
  margin-top: 0.5em;
  text-indent: 0;
}
 
/* 两图并排 */
.figure-pair { 
  text-align: center; 
  margin: 1.5em 0; 
}
.figure-pair figure { 
  display: inline-block; 
  width: 48%; 
  vertical-align: top; 
  margin: 0 0.5%;
}
 
/* === 代码 === */
code {
  font-family: "SF Mono", "Consolas", "Source Code Pro", 
               "DejaVu Sans Mono", monospace;
  font-size: 0.9em;
  background: #f0f0f0;
  padding: 0.1em 0.3em;
  border-radius: 3px;
}
 
pre {
  font-family: "SF Mono", "Consolas", "Source Code Pro", monospace;
  font-size: 0.85em;
  line-height: 1.4;
  background: #f5f5f5;
  padding: 1em;
  margin: 1em 0;
  overflow-x: auto;
  border-left: 3px solid #666;
  page-break-inside: avoid;
  white-space: pre;
}
 
pre code {
  background: none;
  padding: 0;
  font-size: 1em;
}
 
/* === 表格 === */
.table-wrap {
  overflow-x: auto;
  margin: 1.5em 0;
}
 
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.95em;
  page-break-inside: avoid;
}
 
th, td {
  border: 1px solid #888;
  padding: 0.4em 0.6em;
  text-align: left;
  vertical-align: top;
}
 
th {
  background: #eee;
  font-weight: bold;
}
 
caption {
  font-size: 0.9em;
  font-style: italic;
  margin-bottom: 0.5em;
  caption-side: bottom;
}
 
/* === 列表 === */
ul, ol {
  margin: 1em 0;
  padding-left: 2em;
}
 
li { margin: 0.3em 0; }
li p { text-indent: 0; }
 
dl { margin: 1em 0; }
dt { font-weight: bold; margin-top: 0.8em; }
dd { margin: 0.2em 0 0 2em; }
 
/* === 提示框 === */
.note, .warning, .tip, .info {
  margin: 1.5em 0;
  padding: 0.8em 1em;
  border-left: 4px solid;
  page-break-inside: avoid;
}
 
.note    { border-color: #5b8def; background: #f0f5ff; }
.warning { border-color: #e67e22; background: #fdf3e7; }
.tip     { border-color: #27ae60; background: #ecf9f1; }
.info    { border-color: #888;    background: #f5f5f5; }
 
.note    p::before { content: "📘 注意 — ";  font-weight: bold; }
.warning p::before { content: "⚠️ 警告 — ";  font-weight: bold; }
.tip     p::before { content: "💡 提示 — ";  font-weight: bold; }
.info    p::before { content: "ℹ️ 说明 — ";  font-weight: bold; }
 
.note p, .warning p, .tip p, .info p {
  text-indent: 0;
  margin: 0;
}
 
/* === 横线分隔 === */
hr {
  border: none;
  text-align: center;
  margin: 2em 0;
}
 
hr::after {
  content: "❦";
  font-size: 1.2em;
  color: #888;
}
 
/* === 链接 === */
a {
  color: inherit;
  text-decoration: underline;
}
 
/* === 脚注 === */
a[epub|type~="noteref"] {
  vertical-align: super;
  font-size: 0.75em;
  text-decoration: none;
}
 
aside[epub|type~="footnote"] {
  font-size: 0.9em;
  margin: 1em 0;
  padding: 0.5em;
  border-top: 1px solid #888;
}
 
/* === 章节扉页 === */
.title-page, .dedication, .copyright {
  text-align: center;
  margin: 3em 1em;
  page-break-after: always;
}
 
.dedication {
  font-style: italic;
  margin-top: 30%;
}
 
/* === 索引(多列) === */
.index {
  column-count: 2;
  column-gap: 1.5em;
  font-size: 0.9em;
}
 
.index ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
 
/* === 工具类 === */
.page-break    { page-break-before: always; }
.no-break      { page-break-inside: avoid; }
.center        { text-align: center; }
.no-indent     { text-indent: 0; }
```
 
---
 
## 七、常见组件 HTML 写法
 
### 章节起始
 
```xml
<section epub:type="chapter" id="ch03">
  <h1>第三章 数据结构</h1>
  <p>本章讨论...</p>
</section>
```
 
### 带说明的图
 
```xml
<figure>
  <img src="../images/diagram1.svg" alt="系统架构图"/>
  <figcaption>图 3-1 系统整体架构</figcaption>
</figure>
```
 
### 两图并排对比
 
```xml
<div class="figure-pair">
  <figure>
    <img src="../images/before.png" alt="修改前界面"/>
    <figcaption>修改前</figcaption>
  </figure>
  <figure>
    <img src="../images/after.png" alt="修改后界面"/>
    <figcaption>修改后</figcaption>
  </figure>
</div>
```
 
### 代码块(带语言标注)
 
```xml
<pre><code class="language-python">def hello(name):
    print(f"Hello, {name}!")
</code></pre>
```
 
### API 参数表(用 dl 而不是 table)
 
```xml
<dl class="api-params">
  <dt><code>timeout</code> <em>(int, 可选)</em></dt>
  <dd>超时时间,单位毫秒。默认 <code>3000</code>。</dd>
  
  <dt><code>retries</code> <em>(int, 可选)</em></dt>
  <dd>失败重试次数。默认 <code>0</code>,最大 <code>5</code>。</dd>
</dl>
```
 
### 表格(带响应式包装)
 
```xml
<div class="table-wrap">
  <table>
    <caption>表 4-1 性能对比</caption>
    <thead>
      <tr><th>方案</th><th>吞吐量</th><th>延迟</th></tr>
    </thead>
    <tbody>
      <tr><td>方案 A</td><td>1000 req/s</td><td>50ms</td></tr>
      <tr><td>方案 B</td><td>2500 req/s</td><td>20ms</td></tr>
    </tbody>
  </table>
</div>
```
 
### 脚注
 
```xml
<p>这是正文<a epub:type="noteref" href="#fn1" id="fnref1">[1]</a>。</p>
 
<aside epub:type="footnote" id="fn1">
  <p>[1] 这是脚注内容。<a href="#fnref1">↩</a></p>
</aside>
```
 
### 提示框
 
```xml
<aside class="warning">
  <p>修改此配置前请备份数据库。</p>
</aside>
```
 
### 引用书名(西文与中文)
 
```xml
<!-- 西文书名,用 i 或 cite -->
<p>这段话出自 <cite>The Great Gatsby</cite>。</p>
 
<!-- 中文书名,用书名号即可,需要时也可加 cite -->
<p>这段话出自《围城》。</p>
<p>详见 <cite>《围城》</cite> 第三章。</p>
```
 
### 中文注音(拼音)
 
```xml
<ruby>汉<rt>hàn</rt>字<rt>zì</rt></ruby>
```
 
### 数学公式(MathML + SVG 后备)
 
```xml
<math xmlns="http://www.w3.org/1998/Math/MathML" altimg="../images/eq1.svg">
  <mrow>
    <mi>E</mi>
    <mo>=</mo>
    <mi>m</mi>
    <msup><mi>c</mi><mn>2</mn></msup>
  </mrow>
</math>
```
 
### 强调与区分(语义对照)
 
```xml
<!-- 加重语气 -->
<p>你<em>真的</em>这么想吗?</p>
 
<!-- 重要警示 -->
<p><strong>警告</strong>:此操作不可撤销。</p>
 
<!-- 书名(区分,无强调) -->
<p>他在读 <i>Pride and Prejudice</i>。</p>
 
<!-- 关键词高亮(区分,无重要性) -->
<p>所谓 <b>原型链</b>,是 JavaScript 的继承机制。</p>
 
<!-- 术语首次定义 -->
<p>所谓 <dfn>算法</dfn>,是指解决问题的步骤集合。</p>
 
<!-- 缩写 -->
<p><abbr title="HyperText Markup Language">HTML</abbr> 是网页的基础。</p>
```
 
---
 
## 八、中文排版专项
 
### 字体选择
 
```css
/* 正文用衬线(宋体类) */
body {
  font-family: "Source Han Serif SC", "Noto Serif CJK SC", 
               "Songti SC", "SimSun", "FangSong", serif;
}
 
/* 标题用无衬线(黑体类) */
h1, h2, h3 {
  font-family: "Source Han Sans SC", "Noto Sans CJK SC", 
               "PingFang SC", "Microsoft YaHei", sans-serif;
}
 
/* 代码用等宽 */
code, pre {
  font-family: "SF Mono", "Consolas", "Source Code Pro", monospace;
}
```
 
### 中文不要直接用斜体
 
中文字体几乎没有真正的斜体设计,浏览器算法倾斜效果很丑。改用着重号:
 
```css
:lang(zh) em,
:lang(zh) i,
:lang(zh) cite { 
  font-style: normal; 
  text-emphasis: dot;          /* 着重号 */
  -webkit-text-emphasis: dot;
  text-emphasis-position: under right;  /* 着重号位置 */
}
```
 
### 中英文混排
 
中文与英文/数字之间应留半角空格,提升可读性:
 
- ✅ 我用 Python 写了个脚本,处理了 1000 条数据。
- ❌ 我用Python写了个脚本,处理了1000条数据。
 
可用工具自动处理:[pangu.js](https://github.com/vinta/pangu.js/)、[autocorrect](https://github.com/huacnlee/autocorrect) 等。
 
### 标点压缩与避头尾
 
EPUB 3 阅读器一般会自动处理中文标点压缩与避头尾(行首不出现 `,。!?)」`,行尾不出现 `(「`)。可以在 CSS 中明确开启:
 
```css
body {
  text-spacing: trim-start;  /* 渐进增强 */
  hanging-punctuation: allow-end;
}
```
 
### 首行缩进
 
中文段落传统首行缩进 2 个字符:
 
```css
p { text-indent: 2em; }
```
 
注意紧跟标题、引文、横线后的段落不缩进:
 
```css
h1 + p, h2 + p, h3 + p, hr + p, blockquote + p {
  text-indent: 0;
}
```
 
### 行距
 
中文比西文需要更宽的行距,推荐 `line-height: 1.6 ~ 1.8`,1.7 是常用值。
 
---
 
## 九、避坑清单
 
### 标签使用
 
| 不要 | 改用 |
|---|---|
| `<div>` 包内容 | 用语义标签 `<section>` `<article>` `<aside>` |
| `<div class="title">` | `<h1>` ~ `<h6>` |
| `<br/><br/>` 制造段距 | `margin` |
| 把 `<i>` `<b>` 当过时标签 | 它们语义合法。强调用 `<em>`/`<strong>`,区分用 `<i>`/`<b>` |
| 用 `<em>` 标书名 | 用 `<i>` 或更精确的 `<cite>` |
| 用 `<strong>` 标关键词 | 仅"区分"用 `<b>`,"重要"才用 `<strong>` |
| 用 CSS `font-style: italic` 直接标书名 | 用 `<i>` `<cite>` 标语义,让 CSS 决定显示 |
| `<table>` 做页面布局 | 只用于真正的表格数据 |
| `name="xxx"` 锚点 | `id="xxx"` |
| 表格无 `<thead>` | 必须分 thead / tbody |
| 图片无 `alt` | 必须有 alt 属性 |
| 跳级标题(h2 → h4) | 严格按层级递进 |
 
### CSS 与布局
 
| 不要 | 改用 |
|---|---|
| `display: flex` | `inline-block` / `float` / `text-align` |
| `display: grid` | `columns` / `inline-block` |
| `position: absolute / fixed` | 不用,让内容自然流动 |
| `px` 单位 | `em` / `rem` / `%` |
| 固定宽度 `width: 600px` | `max-width: 100%` |
| `@media` 媒体查询 | 让阅读器自己处理响应式 |
| 强制指定文字颜色 | 留给阅读器,支持夜间模式 |
| 大面积深色背景 | 浅色或不设背景 |
| 仅用颜色传达信息 | 颜色 + 文字/icon 冗余标识 |
| 浅灰文字 `#aaa` | 至少 `#555` 保证对比度 |
| 中文直接用 `font-style: italic` | 用 `text-emphasis` 着重号 |
| 强制行折叠代码 | `overflow-x: auto` 横向滚动 |
| 语法高亮硬写颜色 | 用 `class="language-xx"` 让阅读器决定 |
 
### 图片与字体
 
| 不要 | 改用 |
|---|---|
| WebP / AVIF 图片 | JPEG / PNG / SVG |
| 单图超过 2000px 长边 | 控制在 2000px 以内 |
| WOFF2 字体 | OpenType / TrueType |
| SVG 中用系统字体 | 文字转路径(text to outlines) |
| 图片用绝对定位摆放 | 用 figure + figcaption,让其自然流动 |
 
### 交互与脚本
 
| 不要 | 改用 |
|---|---|
| 嵌入 JS 实现核心功能 | 纯静态 HTML + CSS |
| 嵌入音视频做必需内容 | 当作可选增强 |
| 表单交互 | EPUB 不是网页,不要做表单 |
 
### 文件与结构
 
| 不要 | 改用 |
|---|---|
| 文件名带空格、中文、大写 | 全小写 ASCII,下划线分隔 |
| 用绝对路径 | 全用相对路径 |
| 只做 EPUB 3 nav | 同时保留 NCX (toc.ncx) |
| HTML5 写法(标签不闭合) | 严格 XHTML,全部闭合 |
| 中英文混排不留空格 | 中文与英文/数字之间留半角空格 |
 
### 流程
 
| 不要 | 改用 |
|---|---|
| 跳过 EPUBCheck 校验 | 必须过校验,ERROR 全修 |
| 只在一个阅读器测试 | 至少 4 个:Apple Books / Calibre / Kindle Previewer / KOReader |
| 在小屏不验证表格 | 必须切换字号、横竖屏全测一遍 |
| 不测试夜间模式 | 切换日间/夜间/护眼模式都看一遍 |
 
---
 
## 十、工具链推荐
 
### 编辑与制作
 
- **Sigil**:开源 EPUB 编辑器,可视化 + 直接编辑 XHTML/CSS/OPF。最常用的精修工具
- **Calibre**:管理图书馆 + 格式转换 + 元数据编辑。一键 Markdown / Word → EPUB
- **Pandoc**:命令行神器,Markdown / DOCX / LaTeX → EPUB 3,可指定自定义 CSS
- **Vellum** (Mac 限定,付费):欧美自出版圈最流行,模板漂亮
- **InDesign**:复杂排版的画册类,导出 FXL EPUB 行业标准
 
### 校验与测试
 
- **EPUBCheck**:W3C 官方校验工具,命令行 `java -jar epubcheck.jar yourbook.epub`
- **Apple Books** (Mac/iOS):Apple 生态测试
- **Calibre 自带阅读器**:跨平台测试
- **Kindle Previewer 3** (亚马逊免费):模拟各种 Kindle 设备
- **KOReader**:模拟硬件墨水屏阅读
- **Thorium Reader**:EPUB 3 / FXL 兼容性最好的桌面阅读器
 
### 字体推荐(开源)
 
- **思源宋体 / 思源黑体** (Source Han Serif/Sans):Adobe + Google 联合开发,中日韩多语言
- **Noto Serif/Sans CJK**:Google 出品,与思源是同一字体不同发行
- **JetBrains Mono / Fira Code / Source Code Pro**:代码字体
 
### 资源参考
 
- **Standard Ebooks** (standardebooks.org):公认的现代 EPUB 制作典范,可下载学习其源代码
- **EPUB 3 官方规范** (w3.org/TR/epub-33/):权威文档
- **Pandoc User's Guide**:Pandoc EPUB 输出选项详解
 
---
 
## 十一、验证与测试清单
 
完成 EPUB 后,按顺序检查:
 
### 校验
 
- [ ] EPUBCheck 通过,所有 ERROR 必须修
- [ ] WARNING 看情况,能修尽量修
- [ ] 文件名全小写 ASCII,无空格
- [ ] mimetype 文件是第一个,内容为 `application/epub+zip`,不压缩
- [ ] OPF 中所有文件都列在 manifest
- [ ] OPF 中 spine 阅读顺序正确
- [ ] 同时有 nav.xhtml 和 toc.ncx
 
### 阅读器测试
 
至少在以下阅读器各打开一遍:
 
- [ ] Apple Books (iOS / macOS)
- [ ] Calibre 自带阅读器
- [ ] Kindle Previewer 3 (模拟 Kindle Paperwhite / Oasis 等)
- [ ] KOReader 或 Thorium Reader
- [ ] (可选) 微信读书导入测试
- [ ] (可选) BOOX 等真实墨水屏设备
 
### 内容测试
 
- [ ] 改字号到最小和最大,确认布局不崩
- [ ] 切换横屏/竖屏,确认表格和图片正常
- [ ] 切换夜间模式,确认对比度足够
- [ ] 检查目录跳转、章节内链接、脚注跳转都能用
- [ ] 检查全文搜索能搜到所有正文(图片里的文字搜不到属正常)
- [ ] 长按选词,确认能正常选中(没有 JS 干扰)
- [ ] 检查图片在小屏下不溢出
- [ ] 检查代码块在窄屏下能横向滚动
- [ ] 检查表格在窄屏下处理得当(滚动或自适应)
 
### 元数据完整性
 
- [ ] 标题、作者、语言代码完整
- [ ] ISBN 或唯一标识符
- [ ] 出版日期
- [ ] 封面图正确显示(在 OPF 中同时用旧式和新式两套元数据)
- [ ] 简介(description)
- [ ] 分类标签
 
---
 
## 附录 A:最小可运行 EPUB 示例文件清单
 
最简结构(可作为起点):
 
```
mimetype                  # 内容: application/epub+zip
META-INF/container.xml    # 指向 OPF
OEBPS/content.opf         # 元数据 + 清单 + spine
OEBPS/nav.xhtml           # EPUB 3 目录
OEBPS/toc.ncx             # EPUB 2 目录(兼容)
OEBPS/styles/main.css     # 样式表
OEBPS/text/cover.xhtml    # 封面页
OEBPS/text/ch01.xhtml     # 章节
OEBPS/images/cover.jpg    # 封面图
```
 
### container.xml
 
```xml
<?xml version="1.0"?>
<container version="1.0" 
           xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" 
              media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
```
 
### content.opf 示例
 
```xml
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" 
         version="3.0" 
         unique-identifier="bookid" 
         xml:lang="zh-CN">
  
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx</dc:identifier>
    <dc:title>书名</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:date>2026-05-07</dc:date>
    <dc:publisher>出版者</dc:publisher>
    <dc:description>简介</dc:description>
    <meta property="dcterms:modified">2026-05-07T00:00:00Z</meta>
    <meta name="cover" content="cover-image"/>
  </metadata>
  
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css" href="styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="cover" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch01" href="text/ch01.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  
  <spine toc="ncx">
    <itemref idref="cover"/>
    <itemref idref="nav"/>
    <itemref idref="ch01"/>
  </spine>
  
</package>
```
 
---
 
**文档版本**:2026-05-07  
**适用标准**:EPUB 3.3  
**用途**:制作可重排型电子书,兼顾主流阅读器与墨水屏设备
