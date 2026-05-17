# EPUB 3 补充：其他 CSS 模块

> 主手册（`EPUB3_制作完全参考手册.md`）的速查式补充（四）。
> 适用范围与主手册一致：可重排 EPUB 3.3，固定版式 (FXL) 不在内。
> 目标阅读器：Apple Books 4.x、Thorium 3.x、Calibre 7.x、Kindle Previewer 3 / KFX、Kobo、KOReader 2024+。
>
> 系列其他文档：
> - 列表与字体 → `EPUB3_补充_列表与字体.md`
> - 弹注与 Ruby → `EPUB3_补充_弹注与Ruby.md`
> - 图片与海报页 → `EPUB3_补充_图片与海报页.md`

---

## 目录

1. [夜间模式自适应（`prefers-color-scheme`）](#一夜间模式自适应prefers-color-scheme)
2. [首字下沉（`::first-letter`）](#二首字下沉first-letter)
3. [章首页样式](#三章首页样式)
4. [装饰横线（分节符）](#四装饰横线分节符)
5. [墨水屏适配](#五墨水屏适配)
6. [链接显示](#六链接显示)
7. [表格强化](#七表格强化)
8. [段落与文本](#八段落与文本)
9. [竖排（中文古典 / 诗词 / 武侠类书）](#九竖排中文古典--诗词--武侠类书)
10. [CSS 自定义属性（CSS 变量）](#十css-自定义属性css-变量)
11. [媒体查询响应屏幕尺寸](#十一媒体查询响应屏幕尺寸)
12. [锚点偏移 `scroll-margin`](#十二锚点偏移-scroll-margin)
13. [行内语义标签 CSS（补主手册 §六遗漏）](#十三行内语义标签-css补主手册-六遗漏)
14. [块引文带署名](#十四块引文带署名)
15. [对主手册的补丁建议（本主题相关）](#十五对主手册的补丁建议本主题相关)
16. [阅读器兼容矩阵（节选）](#十六阅读器兼容矩阵节选)

---

## 一、夜间模式自适应（`prefers-color-scheme`）

EPUB 阅读器开启夜间主题时通常会把作者颜色全覆盖，但**作者可以主动适配**，让特定元素在夜间有不同表现：

```css
/* 引文盒（日间深灰边、浅灰底；夜间浅灰边、暗灰底） */
blockquote {
  border-left: 3px solid #888;
  background: #f5f5f5;
  padding: 0.5em 1em;
}
@media (prefers-color-scheme: dark) {
  blockquote { border-left-color: #ccc; background: #2a2a2a; }
}

/* 代码块（同理） */
pre {
  background: #f0f0f0;
  border-left: 3px solid #666;
}
@media (prefers-color-scheme: dark) {
  pre { background: #1e1e1e; border-left-color: #888; color: #ddd; }
}

/* 透明 PNG 反相，见 §8.4 */
```

## 二、首字下沉（`::first-letter`）

章首段落首字加大、突出：

```css
.chapter > p:first-of-type::first-letter {
  float: left;
  font-size: 3.2em;
  line-height: 1;
  padding: 0.05em 0.1em 0 0;
  font-family: "Songti SC", "Source Han Serif SC", serif;
  font-weight: bold;
}

/* 中文首字下沉建议关 italic、关着重号 */
.chapter > p:first-of-type::first-letter {
  font-style: normal;
  text-emphasis: none;
}
```

兼容：Apple Books / Thorium / Calibre / KFX / KOReader 全员支持 `::first-letter`。

## 三、章首页样式

```css
/* 章首页：标题居中、上下留白、强制分页 */
.chapter-opener {
  text-align: center;
  margin-top: 25%;
  page-break-before: always;
}
.chapter-opener .chapter-number {
  font-size: 1.2em;
  letter-spacing: 0.2em;
  color: #888;
  margin-bottom: 0.5em;
}
.chapter-opener h1 {
  font-size: 2.2em;
  font-weight: normal;
  margin: 0.5em 0 2em;
  letter-spacing: 0.05em;
  border: none;            /* 抵消主手册 §六中 h1 的下划线 */
}
.chapter-opener .epigraph {
  font-style: italic;
  font-size: 0.95em;
  margin: 2em 15%;
  text-align: left;
}
```

```xml
<section epub:type="chapter" class="chapter-opener">
  <p class="chapter-number">第三章</p>
  <h1>数据结构</h1>
  <p class="epigraph">「先把数据组织好，算法自然就出来了。」 —— Linus Torvalds</p>
</section>
```

## 四、装饰横线（分节符）

主手册 §六给了一种 `hr::after { content: "❦" }`。常用替代：

```css
hr.dot   { border: none; text-align: center; }
hr.dot::after   { content: "· · ·"; letter-spacing: 0.6em; color: #888; }

hr.flower { border: none; text-align: center; }
hr.flower::after { content: "✿"; font-size: 1.2em; color: #888; }

hr.line  {
  border: none;
  border-top: 1px solid #888;
  width: 30%;
  margin: 2em auto;
}
```

## 五、墨水屏适配

| 做 | 不要做 |
|---|---|
| 用 `#000` / `#222` / `#555` 等高对比度灰 | 用 `#aaa` / `#bbb` 浅灰文字（墨水屏几乎看不见） |
| 用线条 / 边框 / 加粗 / 间距区分层级 | 用颜色区分层级（墨水屏只有灰阶） |
| 给图片留白边 | 给图片做阴影（墨水屏渲染慢） |
| `font-weight: bold` 区分标题 | `text-shadow` 区分文字（墨水屏不显示） |
| 段间距用 `margin` | 用 `<br/><br/>` 凑空白 |

## 六、链接显示

```css
/* 内文链接：保留下划线（无障碍）、跟随阅读器主题色 */
a { color: inherit; text-decoration: underline; }

/* 章节跳转链接（目录、章首），不加下划线 */
nav a, .chapter-opener a { text-decoration: none; }

/* 脚注返回链接，更小一号 */
a[epub|type~="backlink"] { font-size: 0.85em; text-decoration: none; }
```

## 七、表格强化

主手册 §六有基础表格。补**长表格 / 跨页表头重复**：

```css
table { width: 100%; border-collapse: collapse; }

/* 跨页时表头重复（Apple Books、Thorium 支持） */
thead { display: table-header-group; }
tfoot { display: table-footer-group; }

/* 长表格在窄屏下横向滚动包装 */
.table-wrap { overflow-x: auto; -webkit-overflow-scrolling: touch; }

/* 隔行变色（墨水屏请用 #f0f0f0 而非更浅） */
tbody tr:nth-child(even) { background: #f5f5f5; }
@media (prefers-color-scheme: dark) {
  tbody tr:nth-child(even) { background: #2a2a2a; }
}
```

## 八、段落与文本

主手册已给 `p { text-indent: 2em }`。补（按支持度分两段）：

```css
/* —— 全员支持 —— */
p {
  -webkit-hyphens: auto;
  hyphens: auto;
  widows: 2;
  orphans: 2;                    /* 防段末孤行 */
}
```

```css
/* —— 渐进增强（部分阅读器忽略不报错） —— */
p {
  hanging-punctuation: allow-end;   /* 仅 Apple Books / KOReader */
  hyphenate-limit-chars: 6 3 3;     /* CSS Text 4，仅 Safari 17+ / Chrome 109+ 内核 */
}
```

| 属性 | Apple Books | Thorium | Calibre | KFX | KOReader | 备注 |
|---|---|---|---|---|---|---|
| `widows` / `orphans` | ✅ | ✅ | ✅ | ⚠️ | ✅ | 段末孤行控制 |
| `hyphens: auto` | ✅ | ✅ | ✅ | ✅ | ✅ | 西文断字 |
| `hyphenate-limit-chars` | ✅ Safari 17+ | ⚠️ Chromium 109+ | ⚠️ 视版本 | ❌ | ❌ | 写了不报错，不支持就忽略 |
| `hanging-punctuation` | ✅ | ❌ | ❌ | ❌ | ⚠️ | WebKit 独占特性 |

## 九、竖排（中文古典 / 诗词 / 武侠类书）

EPUB 3 标准支持竖排，靠两条 CSS：`writing-mode: vertical-rl`（从右到左竖排）+ `text-orientation`。**只对需要竖排的章节加 class，不要全局开**。

```css
.vertical {
  writing-mode: vertical-rl;           /* 竖排，从右往左 */
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;     /* 老 KFX 兼容 */
  text-orientation: mixed;             /* 汉字立排，西文/数字旋转 */
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  line-height: 1.8;
  height: 100vh;                       /* 让竖排撑满屏 */
}

/* 竖排时 ruby 自动跑到字右侧；着重号位置也要调 */
.vertical em,
.vertical :lang(zh) em {
  text-emphasis-position: over right;
}

/* 数字 / 短英文要"直立"成竖排（"百年孤独 100 年"中的 100） */
.vertical .upright { text-combine-upright: all; }
```

```xml
<section class="vertical" epub:type="chapter">
  <h1>第一章 江南</h1>
  <p>江南有<span class="upright">120</span>处名胜……</p>
</section>
```

**OPF 翻页方向**（中文竖排书全书右起，应在 `<spine>` 加属性）：

```xml
<spine page-progression-direction="rtl">
  <itemref idref="cover"/>
  <itemref idref="ch01"/>
  ...
</spine>
```

`rtl` 表示"右到左翻页"（与竖排阅读顺序一致）。Apple Books / Thorium / Kindle KFX / 多看都依据这条决定翻页手势方向；不写默认 `ltr`。

| 阅读器 | 竖排支持 | 备注 |
|---|---|---|
| Apple Books | ✅ | 自动右滑翻页 |
| Thorium | ✅ | 现代 Chromium 内核完整支持 |
| Calibre Viewer | ⚠️ | 渲染正确但翻页方向视版本 |
| Kindle KFX（中文 / 日文书） | ✅ | 必须配合 `page-progression-direction="rtl"` |
| KOReader | ⚠️ | 显示正确，部分版本翻页方向反 |
| 多看 | ✅ | 国内厂商对古典书较重视 |

## 十、CSS 自定义属性（CSS 变量）

主题色 / 字号 / 间距集中管理，2025 全员支持（`var()` 是 CSS Variables Level 1，2017 起广泛支持）：

```css
:root {
  --color-text: #222;
  --color-mute: #888;
  --color-accent: #8b4513;       /* 引文边、章号 */
  --color-bg-quote: #f5f5f5;

  --font-serif: "Songti SC", "SimSun", "Source Han Serif SC", serif;
  --font-sans:  "PingFang SC", "Microsoft YaHei", "Source Han Sans SC", sans-serif;
  --font-mono:  "SF Mono", "Consolas", monospace;

  --space-paragraph: 0;
  --space-section: 2em;
  --line-height-cn: 1.75;
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-text: #ddd;
    --color-mute: #999;
    --color-bg-quote: #2a2a2a;
  }
}

body { color: var(--color-text); font-family: var(--font-serif); line-height: var(--line-height-cn); }
blockquote { background: var(--color-bg-quote); border-left: 3px solid var(--color-accent); }
h1, h2, h3 { font-family: var(--font-sans); }
code, pre { font-family: var(--font-mono); }
```

| 阅读器 | `var()` 支持 |
|---|---|
| Apple Books | ✅ |
| Thorium | ✅ |
| Calibre | ✅ |
| Kindle KFX | ✅ |
| KOReader | ✅ |

不要在 `@font-face` 内部用 `var()`——`@font-face` 的 `src` 不接受变量。

## 十一、媒体查询响应屏幕尺寸

主手册建议"不写媒体查询，让阅读器处理"——对**正文段落**是对的，但**多列、并排图、复杂表格**在手机上往往需要回退单列：

```css
/* 默认：宽屏，两图并排 */
.figure-pair figure {
  display: inline-block;
  width: 48%;
  margin: 0 0.5%;
}

/* 窄屏（典型手机 < 600px）：单列堆叠 */
@media (max-width: 600px) {
  .figure-pair figure { display: block; width: 100%; margin: 0.5em 0; }
  .index { column-count: 1; }                  /* 索引多列回退 */
  .float-left, .float-right {
    float: none; width: 100%; margin: 1em 0;   /* 绕排回退 */
  }
}

/* 极窄屏（< 400px）字号略减 */
@media (max-width: 400px) {
  body { font-size: 0.95em; }
}
```

| 阅读器 | `@media (max-width)` 是否触发 |
|---|---|
| Apple Books | ✅ 按容器宽度 |
| Thorium | ✅ |
| Calibre Viewer | ✅ |
| Kindle KFX | ⚠️ 老固件不计算；新固件按页面宽度 |
| KOReader | ✅ |

**注意**：`@media` 的"宽度"是阅读器内容区宽，不是设备宽——读者放大字号、双栏切换时也会触发。这正是想要的行为。

## 十二、锚点偏移 `scroll-margin`

点击脚注链接跳转后，目标行常被阅读器顶栏遮挡。给可跳转锚点加上 `scroll-margin-top` 抬高视野：

```css
/* 任何被锚点引用的元素：跳转后上方留 1em 空隙 */
[id^="fn"],
[id^="ref"],
section[epub|type~="chapter"] > h1,
section[epub|type~="chapter"] > h2 {
  scroll-margin-top: 1.5em;
}
```

| 阅读器 | 支持 |
|---|---|
| Apple Books | ✅ iOS 14+ |
| Thorium | ✅ |
| Calibre | ✅ |
| Kindle KFX | ⚠️ 部分固件忽略 |
| KOReader | ✅ |

## 十三、行内语义标签 CSS（补主手册 §六遗漏）

主手册 §三速查表里列了 `<kbd>` `<var>` `<samp>` `<q>` `<del>` `<ins>`，但 §六只给了 `mark` / `code` 的 CSS。补齐：

```css
/* 键盘按键 */
kbd {
  font-family: var(--font-mono, "SF Mono", monospace);
  font-size: 0.85em;
  padding: 0.1em 0.4em;
  border: 1px solid var(--color-mute, #888);
  border-radius: 3px;
  background: var(--color-bg-quote, #f5f5f5);
  box-shadow: 0 1px 0 var(--color-mute, #888);
  white-space: nowrap;
}

/* 程序变量 / 数学变量 */
var {
  font-style: italic;
  font-family: "Times New Roman", "Songti SC", serif;
}

/* 程序输出 */
samp {
  font-family: var(--font-mono, monospace);
  font-size: 0.9em;
  background: var(--color-bg-quote, #f5f5f5);
  padding: 0 0.2em;
}

/* 短引文（行内）：自动加引号，主手册没显式声明 */
q { quotes: "「" "」" "『" "』"; }
q::before { content: open-quote; }
q::after  { content: close-quote; }
:lang(en) q { quotes: "\201C" "\201D" "\2018" "\2019"; }  /* 英文 "" '' */

/* 删除 / 插入（修订标记） */
del { text-decoration: line-through; color: var(--color-mute, #888); }
ins { text-decoration: underline; text-decoration-style: dotted; }

/* 缩写（主手册 §六只给了 border-bottom；补悬停说明） */
abbr[title] {
  border-bottom: 1px dotted;
  text-decoration: none;
  cursor: help;
}
```

`<q>` 的中英文引号区分依赖 `<html lang="zh-CN">` 与 `<span lang="en">` 的标记；详见 §5.3。

## 十四、块引文带署名

主手册 §六给了 blockquote 基础样式。补**带署名引文**的语义写法：

```xml
<figure class="quote">
  <blockquote>
    <p>未经省察的人生不值得过。</p>
  </blockquote>
  <figcaption>—— <cite>苏格拉底</cite>，《申辩篇》</figcaption>
</figure>
```

```css
figure.quote {
  margin: 1.5em 2em;
  page-break-inside: avoid;
}
figure.quote blockquote {
  margin: 0;
  padding: 0.5em 1em;
  border-left: 3px solid var(--color-accent, #888);
}
figure.quote blockquote p { text-indent: 0; }
figure.quote figcaption {
  text-align: right;
  font-size: 0.9em;
  color: var(--color-mute, #666);
  margin-top: 0.5em;
}
figure.quote cite { font-style: italic; }
```

`<cite>` 标作品名比 `<i>` 语义更准（主手册 §四已强调）。

---

## 十五、对主手册的补丁建议（本主题相关）

| 主手册位置 | 改动 |
|---|---|
| §五 CSS 速查表 / 字体与文字 | 补 `ruby-position` `ruby-align` 行（详见 `EPUB3_补充_弹注与Ruby.md` §二.2） |
| §六 基础样式表（顶部） | 在 `body` 上方加 `:root { --color-text: ...; --font-serif: ... }` 变量块（引 §十） |
| §六 基础样式表 | 新增夜间模式（§一）/ 首字下沉（§二）/ 表头重复（§七） |
| §六 基础样式表 / 链接 | 用 §六 链接 4 类规则替换原通用 `a` 样式 |
| §七 常见组件 | 补章首页结构（§三）、装饰横线 4 种（§四） |
| §八 中文排版 | 补一节"墨水屏适配"（引 §五） |
| §九 避坑清单 / CSS 与布局 | 把 "不写媒体查询" 改为 "正文不写媒体查询；多列 / 并排 / 绕排可写 `@media (max-width: 600px)` 回退单列"（引 §十一） |
| §十一 验证测试清单 | 补"切夜间模式确认 prefers-color-scheme 生效"；竖排书加测"翻页方向是否正确" |

---

## 十六、阅读器兼容矩阵（节选）

| 特性 | Apple Books 4.x | Thorium 3.x | Calibre 7.x | Kindle KFX | KOReader 2024+ |
|---|---|---|---|---|---|
| `prefers-color-scheme: dark` | ✅ | ✅ | ✅ | ⚠️ Colorsoft / 新固件 | ✅ |
| `::first-letter` 首字下沉 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `writing-mode: vertical-rl`（竖排） | ✅ | ✅ | ⚠️ 翻页方向 | ✅ KFX 需 OPF rtl | ⚠️ |
| `text-combine-upright`（数字立排） | ✅ | ✅ | ⚠️ | ✅ | ⚠️ |
| `var()` CSS 变量 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `@media (max-width)` 媒体查询 | ✅ | ✅ | ✅ | ⚠️ 新固件 | ✅ |
| `scroll-margin-top` | ✅ iOS 14+ | ✅ | ✅ | ⚠️ | ✅ |
| `:has()` 选择器 | ✅ Safari 15.4+ | ✅ Chromium 105+ | ✅ | ❌ | ⚠️ 2024+ |
| `widows` / `orphans` 防孤行 | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| `hanging-punctuation` 标点悬挂 | ✅ | ❌ | ❌ | ❌ | ⚠️ |
| `hyphenate-limit-chars` | ✅ Safari 17+ | ⚠️ Chromium 109+ | ⚠️ | ❌ | ❌ |
| `quotes` + `content: open-quote` | ✅ | ✅ | ✅ | ✅ | ✅ |

> ✅ 正常 ⚠️ 部分版本 / 部分场景 ❌ 不支持

---

**文档版本**：2026-05-15
**对应标准**：EPUB 3.3
**关联文档**：`EPUB3_制作完全参考手册.md`（主手册）、《EPub指南——从入门到放弃》（赤霓，2023-04-18）