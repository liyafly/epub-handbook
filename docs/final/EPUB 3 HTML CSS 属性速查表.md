# EPUB 3 HTML / CSS 属性速查表

> 版本：2026-05-18  
> 用途：作为 EPUB 3 中文书制作时的速查源文档。  
> 维护规则：先保留现有主手册与补充篇中已整理的标准能力；后续实测不稳定的，在“状态/备注”列降级或删除。

---

## 一、状态说明

| 状态 | 含义 |
|---|---|
| 推荐 | 当前项目主路径，直接使用 |
| 可用 | 标准或主流支持，可按场景使用 |
| 渐进增强 | 写了不影响基础阅读，不支持时自动忽略 |
| 条件可用 | 需特定阅读器、版本或场景 |
| 待实测 | 已纳入清单，但还需要后续实践确认 |

---

## 二、HTML 元素速查

### 2.1 文档结构

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<html xml:lang="zh-CN">` | XHTML 根元素与语言 | 推荐 | 每个 XHTML 都写 |
| `<head>` | 元信息容器 | 推荐 | 放 `title`、`meta`、CSS |
| `<meta charset="utf-8"/>` | 字符集 | 推荐 | 每个 XHTML 都写 |
| `<title>` | 页面标题 | 推荐 | 阅读器与调试都用得到 |
| `<link rel="stylesheet">` | 引入 CSS | 推荐 | 字体、正文、海报可分文件 |
| `<body>` | 页面主体 | 推荐 | 海报页用 `body.fullpage` |
| `<section>` | 章节 / 语义分区 | 推荐 | 配 `epub:type` |
| `<article>` | 独立文章 | 可用 | 文集、附录、单篇文章 |
| `<header>` | 章节头部 | 可用 | 章题、题记组合 |
| `<footer>` | 尾部 / 来源 | 可用 | 引文来源、章节署名 |
| `<nav epub:type="toc">` | EPUB 目录 | 推荐 | `nav.xhtml` 必备 |
| `<nav epub:type="landmarks">` | 地标 | 推荐 | 封面、目录、正文开始 |

### 2.2 标题与正文

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<h1>`–`<h6>` | 标题层级 | 推荐 | 不跳级 |
| `<p>` | 正文段落 | 推荐 | 中文正文默认 `text-indent: 2em` |
| `<span>` | 行内样式容器 | 推荐 | 着重、波浪线、语言片段 |
| `<div>` | 无语义块容器 | 可用 | 排版包装；不替代章节语义 |
| `<hr>` | 分节符 / 横线 | 可用 | 可配装饰类 |
| `<br/>` | 行内换行 | 条件可用 | 诗歌、地址、特殊短行 |

### 2.3 文本语义

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<em>` | 强调 | 推荐 | 中文 CSS 显示为着重号 |
| `<strong>` | 重要 | 推荐 | 语义强重要性 |
| `<i>` | 区分文本 | 可用 | 术语、外文、书名；作品名优先 `<cite>` |
| `<b>` | 视觉区分 | 可用 | 关键词但非强调语气 |
| `<cite>` | 作品名 / 来源 | 推荐 | 书名、篇名、来源 |
| `<q>` | 行内引用 | 推荐 | 用 `quotes` 控制中英文引号 |
| `<blockquote>` | 块引用 | 推荐 | 长引用 |
| `<small>` | 附属小字 | 可用 | 版权、题下注 |
| `<mark>` | 标记文本 | 可用 | 少量高亮 |
| `<abbr title="">` | 缩写说明 | 可用 | 可点线提示 |
| `<dfn>` | 定义项 | 可用 | 术语首次定义 |
| `<del>` | 删除文本 | 可用 | 修订痕迹 |
| `<ins>` | 插入文本 | 可用 | 修订痕迹 |
| `<sup>` | 上标 | 推荐 | 注释图标、脚注 |
| `<sub>` | 下标 | 可用 | 化学式、数学 |

### 2.4 代码与技术文本

| 元素 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<code>` | 行内代码 | 推荐 | 等宽字体 |
| `<pre>` | 代码块 | 推荐 | 保留空白，横向滚动 |
| `<kbd>` | 键盘输入 | 可用 | UI 操作说明 |
| `<samp>` | 程序输出 | 可用 | 命令输出 |
| `<var>` | 变量 | 可用 | 程序 / 数学变量 |

### 2.5 列表与表格

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<ol>` | 有序列表 | 推荐 | 支持 `start`、`reversed` |
| `<ul>` | 无序列表 | 推荐 | 普通项目符号 |
| `<li>` | 列表项 | 推荐 | CSS 清除首行缩进 |
| `<dl>` | 定义列表 | 推荐 | 术语表、API 参数 |
| `<dt>` | 术语 | 推荐 | 常加粗 |
| `<dd>` | 解释 | 推荐 | 可多个解释对应一个术语 |
| `<table>` | 表格数据 | 可用 | 只用于真实数据表 |
| `<thead>` | 表头 | 推荐 | 长表格可重复表头 |
| `<tbody>` | 表体 | 推荐 | 数据行 |
| `<tfoot>` | 表尾 | 可用 | 汇总 |
| `<tr>` | 表格行 | 推荐 |  |
| `<th>` | 表头单元格 | 推荐 |  |
| `<td>` | 数据单元格 | 推荐 |  |

### 2.6 图片与媒体

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<figure>` | 图文组 | 推荐 | 图片 + 说明 |
| `<img src alt>` | 图片 | 推荐 | `alt` 必填 |
| `<figcaption>` | 图片说明 | 推荐 | 放在 `<figure>` 内 |
| `<svg>` | 矢量图 | 可用 | 图标、线稿 |
| `<picture>` | 响应式图片 | 条件可用 | KFX 可能只取 `<img>` |
| `<source>` | 图片候选源 | 条件可用 | 与 `<picture>` 配合 |
| `<audio>` | 音频 | 条件可用 | 可选增强 |
| `<video>` | 视频 | 条件可用 | 可选增强 |

### 2.7 EPUB 语义与弹注

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `epub:type="chapter"` | 章节语义 | 推荐 | 放在 `<section>` |
| `epub:type="cover"` | 封面 | 推荐 | 封面页 |
| `epub:type="bodymatter"` | 正文开始 | 推荐 | landmarks |
| `epub:type="toc"` | 目录 | 推荐 | nav |
| `<a epub:type="noteref" role="doc-noteref">` | 注释触发 | 推荐 | 项目采用图片图标触发 |
| `<aside epub:type="footnote" role="doc-footnote">` | 注释容器 | 推荐 | 每个 XHTML 一个，包住本文件全部注释 |
| `<ol class="footnote-list">` | 注释列表 | 推荐 | 放在 footnote aside 内 |
| `<li class="footnote-item" id="footnote-1">` | 单条注释目标 | 推荐 | noteref 的 `href` 指向这里 |
| `<a epub:type="backlink" role="doc-backlink">` | 注释返回 | 推荐 | 项目采用 `◎` |
| `<section epub:type="footnotes">` | 注释组 | 可用 | 不作为项目弹注主路径 |

### 2.8 Ruby 注音

| 元素 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<ruby>` | 注音容器 | 推荐 |  |
| `<rb>` | 基字 | 推荐 | 多字词注音更稳定 |
| `<rt>` | 注音文本 | 推荐 | 拼音、外文读音 |
| `<rp>` | 后备括号 | 推荐 | 不支持 ruby 时显示括号 |

### 2.9 MathML

| 元素 / 属性 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `<math>` | MathML 根 | 条件可用 | Kindle Enhanced Typesetting 支持；manifest 需 `properties="mathml"` |
| `<mrow>` / `<mi>` / `<mn>` / `<mo>` / `<mtext>` | 基础公式结构 | 条件可用 | 变量、数字、运算符、文本 |
| `<mfrac>` / `<msqrt>` / `<mroot>` | 分式与根式 | 条件可用 | 16 号 demo 覆盖 |
| `<msub>` / `<msup>` / `<msubsup>` / `<mmultiscripts>` | 上下标与复杂脚标 | 条件可用 | 避免未确认支持的扩展标签 |
| `<mover>` / `<munder>` / `<munderover>` / `<menclose>` | 上下标记与围框 | 条件可用 | 适合积分、向量、框选 |
| `<mfenced>` / `<mtable>` / `<mtr>` / `<mlabeledtr>` / `<mtd>` | 括号、矩阵与带编号行 | 条件可用 | 复杂表格需实测 |
| `<semantics>` / `<annotation>` | 语义与备用源码 | 条件可用 | 可放 TeX annotation |

---

## 三、OPF / EPUB 包属性速查

### 3.1 `package`

| 属性 | 值 | 状态 | 备注 |
|---|---|---|---|
| `xmlns` | `http://www.idpf.org/2007/opf` | 推荐 | OPF 命名空间 |
| `version` | `3.0` | 推荐 | EPUB 3 |
| `unique-identifier` | `bookid` | 推荐 | 对应 `dc:identifier id` |
| `xml:lang` | `zh-CN` | 推荐 | 全书语言 |
| `prefix` | `rendition: ... ibooks: ...` | 推荐 | 使用 rendition / ibooks 元数据时必须 |

### 3.2 metadata

| 元素 / 属性 | 值 / 例子 | 状态 | 备注 |
|---|---|---|---|
| `<dc:identifier id="bookid">` | `urn:uuid:...` | 推荐 | 全书唯一标识 |
| `<dc:title>` | 书名 | 推荐 |  |
| `<dc:creator>` | 作者 | 推荐 |  |
| `<dc:language>` | `zh-CN` | 推荐 |  |
| `<dc:date>` | `2026-05-18` | 可用 | 出版 / 制作日期 |
| `<dc:publisher>` | 出版者 | 可用 |  |
| `<dc:description>` | 简介 | 可用 |  |
| `dcterms:modified` | ISO 时间 | 推荐 | EPUB 3 必备 |
| `rendition:layout` | `reflowable` | 推荐 | 全书默认可重排 |
| `rendition:orientation` | `auto` | 推荐 | 不锁横竖屏 |
| `rendition:spread` | `auto` | 推荐 | 不强制单双页 |
| `ibooks:specified-fonts` | `true` | 推荐 | Apple Books 嵌入字体声明 |
| `<meta name="cover">` | `cover-img` | 推荐 | 兼容封面识别 |
| `spine toc="ncx"` | `toc` / `ncx` item id | 推荐 | Kindle / 旧工具链兼容目录 |

### 3.3 manifest media-type

| 资源 | media-type | 状态 |
|---|---|---|
| XHTML | `application/xhtml+xml` | 推荐 |
| nav.xhtml | `application/xhtml+xml` + `properties="nav"` | 推荐 |
| XHTML with MathML | `application/xhtml+xml` + `properties="mathml"` | 条件可用 |
| NCX | `application/x-dtbncx+xml` | 推荐 |
| cover image | `image/jpeg` / `image/png` + `properties="cover-image"` | 推荐 |
| CSS | `text/css` | 推荐 |
| JPEG | `image/jpeg` | 推荐 |
| PNG | `image/png` | 推荐 |
| WebP | `image/webp` | 条件可用：现代阅读器增强；Kindle conversion log 已确认不适合作主路径 |
| SVG | `image/svg+xml` | 条件可用：现代 EPUB 增强或源文件；Kindle 生产包优先栅格化 |
| TTF | `font/ttf` | 推荐 |
| OTF | `font/otf` | 推荐 |
| WOFF | `font/woff` | 条件可用 |
| WOFF2 | `font/woff2` | 条件可用 |
| MP3 | `audio/mpeg` | 条件可用 |
| MP4 | `video/mp4` | 条件可用 |

### 3.4 spine

| 属性 | 值 | 状态 | 备注 |
|---|---|---|---|
| `toc` | `ncx` | 推荐 | 兼容旧阅读器 |
| `page-progression-direction` | `ltr` | 推荐 | 全书横排 |
| `page-progression-direction` | `rtl` | 条件可用 | 整本竖排书 |

全书横排、局部竖排页面仍保持 `ltr`。

---

## 四、CSS 字体与文本属性速查

| 属性 / 规则 | 推荐值 / 用法 | 状态 | 备注 |
|---|---|---|---|
| `@font-face` | 定义书内字体 | 推荐 | 正文、标题、生僻字可分开 |
| `font-family` | 默认 Apple + Windows + Android/开源 + generic（≤ 4 段，不嵌字体）；嵌入字体仅走专用类（模式 A / B / C，详见 SPEC §8） | 推荐 | 正文字体主路径 |
| `font-style` | `normal` / `italic` | 推荐 | 中文强调用着重号 |
| `font-weight` | `normal` / `bold` / `400` / `700` | 推荐 | 嵌入字重需匹配 |
| `font-size` | `em` / `%` | 推荐 | 正文避免固定 px |
| `line-height` | `1.6`–`1.9` | 推荐 | 中文常用 1.7 |
| `line-height` | `1.45`–`1.65` | 推荐 | 英文小说常用；简单重排书先取 1.55 左右 |
| `text-indent` | `2em` | 推荐 | 中文正文 |
| `text-indent` | `1.2em`–`1.5em` | 推荐 | 英文小说后续段落；首段用 `0` |
| `text-align` | `left` / `center` / `right` / `justify` | 推荐 | 中文正文常用 `justify`；英文未验证断字时优先 `left` |
| `text-justify` | `inter-ideograph` | 可用 | 部分阅读器忽略 |
| `letter-spacing` | `0.04em` 等 | 可用 | 标题、竖排题签 |
| `word-break` | `break-all` | 条件可用 | 中文长串可用，正文谨慎 |
| `hyphens` | `auto` | 可用 | 西文断字 |
| `-webkit-hyphens` | `auto` | 可用 | Apple Books / WebKit 兜底 |
| `widows` / `orphans` | `2` | 可用 | 防孤行 |
| `hanging-punctuation` | `allow-end` | 渐进增强 | Apple Books / KOReader 较好 |
| `hyphenate-limit-chars` | `6 3 3` | 渐进增强 | 支持有限 |

### 4.1 正文字体链

```css
/* 默认（不嵌字体） */
body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

/* 模式 C1-body（含生僻字 + 全字符集嵌入字体） */
body {
  font-family: "BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

> 第一组是默认路径；第二组仅当全书含生僻字、且嵌入"全字符集"字体（`fontspec=forceAll`）时启用，详见 SPEC §8 模式 C1-body。

### 4.2 标题 / 特殊字体

```css
/* 模式 A：题签 / 卷头题字 / 签名档（嵌入设计字体；链 ≤ 2 段） */
.poster-title,
.title-kai,
.inscription {
  font-family: "BookTitleKai", serif;
}

/* 模式 B：生僻字子集字库（链 ≤ 2 段） */
.rare {
  font-family: "RareSongSubset", serif;
}
```

---

## 五、CSS 排版与盒模型属性速查

| 属性 | 推荐值 / 用法 | 状态 | 备注 |
|---|---|---|---|
| `margin` | `em` / `%` | 推荐 | 段距、图距 |
| `padding` | `em` / `%` | 推荐 | 容器内距 |
| `body width + padding` | 避免 `width:100%` 叠加左右 `padding` | 必须 | 普通正文页防右侧裁切 |
| `border` | `1px solid #888` | 推荐 | 分隔线、表格 |
| `border-left` | 引文边线 | 推荐 | 块引文 |
| `border-style` | `solid` / `dashed` / `double` / `dotted` | 推荐 | 便签、资料卡、摘录框 |
| `border-radius` | 小半径 / 不对称半径 | 可用 | 代码、kbd、不规则便签 |
| `outline` / `outline-offset` | 细线外框 | 渐进增强 | 不规则便签效果；可被忽略 |
| `box-sizing` | `border-box` | 推荐 | A-lite 页面 |
| `width` | `%` / `auto` / 固定 `px` | 推荐 | 环绕 figure 主路径推荐 `25%–35%`，按阅读器实测微调；表格可用 `%` |
| `max-width` | `100%` | 推荐 | 图片 |
| `height` | `auto` | 推荐 | 图片 |
| `min-height` | `100%` / `90%` | 推荐 | A-lite |
| `overflow` | `hidden` / `auto` | 推荐 | A-lite / 表格滚动 |
| `overflow-x` | `auto` | 推荐 | 代码块、长表格 |
| `display` | `block` / `inline-block` / `table-*` | 推荐 | EPUB 兼容稳定 |
| `float` | `left` / `right` | 推荐 | 图文环绕通用路径：float 挂 `<figure>`，宽度用 `25%–35%` 百分比；英文首字主路径用 `::first-letter`。 |
| `clear` | `none` / `right` | 可用 | A-lite 竖排列 |
| `box-shadow` | `.2em .2em 0 #ddd` / `inset 0 0 .4em #ddd` | 渐进增强 | 便签阴影；必须有 border/background 兜底 |
| `transform` | `rotate(-1deg)` | 风险 | 不放入通用 Kindle 版本；Kindle Previewer 3.104 实测可能触发增强排版转换内部错误 |
| SVG 花边框 | 内联 SVG + `aria-hidden="true"` + 普通边框兜底 | 渐进增强 | 贝塞尔曲线花边；失效时降级为双线框或左侧竖线 |
| `position` | `relative` | 可用 | 列表 marker |
| `position` | `absolute` | 条件可用 | 正文不用；A-lite 不用 |

---

## 六、CSS 分页属性速查

| 属性 | 推荐值 | 状态 | 备注 |
|---|---|---|---|
| `page-break-before` | `always` | 推荐 | 章首页、A-lite |
| `page-break-after` | `always` / `avoid` | 推荐 | A-lite / 标题避断 |
| `page-break-inside` | `avoid` | 推荐 | A-lite、图、表、引用 |
| `-webkit-page-break-before` | `always` | 推荐 | Apple Books 兼容 |
| `-webkit-page-break-after` | `always` | 推荐 | Apple Books 兼容 |
| `-webkit-page-break-inside` | `avoid` | 推荐 | Apple Books 兼容 |
| `break-before` | `page` | 渐进增强 | 可与 page-break 并写 |
| `break-after` | `page` / `avoid` | 渐进增强 | 可与 page-break 并写 |
| `break-inside` | `avoid` | 渐进增强 | 可与 page-break 并写 |

---

## 七、CSS 列表属性速查

| 属性 / 值 | 用途 | 状态 | 备注 |
|---|---|---|---|
| `list-style-type: disc` | 实心圆点 | 推荐 |  |
| `circle` | 空心圆 | 可用 |  |
| `square` | 方块 | 可用 |  |
| `decimal` | 数字 | 推荐 |  |
| `decimal-leading-zero` | 01、02 | 可用 |  |
| `lower-roman` / `upper-roman` | 罗马数字 | 可用 | 老 KFX 注意 |
| `lower-alpha` / `upper-alpha` | 英文字母 | 可用 |  |
| `cjk-ideographic` | 一二三 | 可用 | KFX 不稳时用 counter |
| `hiragana` / `katakana` | 日文假名 | 条件可用 | 日文书 |
| `none` | 隐藏标记 | 推荐 | 自定义 marker |
| `counter-reset` | 重置计数器 | 推荐 | 中文编号 |
| `counter-increment` | 增加计数器 | 推荐 | 中文编号 |
| `counter()` | 输出计数器 | 推荐 | `counter(c, cjk-ideographic)` |

---

## 八、CSS 图片与媒体属性速查

| 属性 | 推荐值 / 用法 | 状态 | 备注 |
|---|---|---|---|
| `max-width` | `100%` | 推荐 | 图片自适应 |
| `height` | `auto` | 推荐 | 保持比例 |
| `vertical-align` | `baseline` / `middle` | 推荐 | 行内图标 |
| `object-fit` | `contain` / `cover` | 条件可用 | 部分阅读器 |
| `aspect-ratio` | 固定比例盒 | 渐进增强 | 不作为环绕图片主路径；真实图片用 `height:auto` 保持比例 |
| `background-image` | `url(...)` | 推荐 | A-lite 背景 |
| `background-repeat` | `no-repeat` | 推荐 | 海报背景 |
| `background-position` | `left bottom` / `center center` | 推荐 | 海报背景 |
| `background-size` | `80% auto` / `cover` | 推荐 | 海报背景 |
| `filter: invert(1)` | 夜间透明图反相 | 渐进增强 | 透明 PNG 可用 |

---

## 九、CSS 竖排属性速查

| 属性 | 推荐值 | 状态 | 备注 |
|---|---|---|---|
| `writing-mode` | `vertical-rl` | 推荐 | 局部竖排、A-lite |
| `-webkit-writing-mode` | `vertical-rl` | 推荐 | Apple Books |
| `-epub-writing-mode` | `vertical-rl` | 推荐 | 老 EPUB/KFX |
| `text-orientation` | `mixed` | 推荐 | 汉字直立，西文旋转 |
| `-webkit-text-orientation` | `mixed` | 推荐 | Apple Books |
| `-epub-text-orientation` | `mixed` | 推荐 | 老 EPUB/KFX |
| `text-combine-upright` | `all` | 可用 | 短数字直立 |

全书横排、局部竖排：

```xml
<spine toc="ncx" page-progression-direction="ltr">
```

```css
.vrl-section {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  line-height: 1.8;
}
```

---

## 十、CSS 文字效果属性速查

| 属性 | 推荐值 / 用法 | 状态 | 备注 |
|---|---|---|---|
| `text-emphasis` | `filled dot` | 推荐 | 中文着重号 |
| `-webkit-text-emphasis-style` | `filled dot` | 推荐 | Apple Books |
| `-epub-text-emphasis-style` | `filled dot` | 推荐 | EPUB 兼容 |
| `text-emphasis-position` | `under` | 推荐 | 横排中文 |
| `text-decoration` | `underline` | 推荐 | 波浪线基础兜底；Kindle 只显示这层 |
| `text-decoration-style` | `wavy` / `dotted` / `solid` | 渐进增强 | 标准波浪线；Kindle App 退化为普通 underline |
| `text-decoration-color` | `#c03030` | 可用 | 波浪线颜色 |
| `text-decoration-thickness` | `1px` | 可用 | 线宽 |
| `text-underline-offset` | `0.12em` | 可用 | 下划线偏移 |
| `quotes` | `"「" "」" "『" "』"` | 推荐 | 中文 `<q>` |
| `content: open-quote` | 自动开引号 | 推荐 | `<q>` |
| `content: close-quote` | 自动闭引号 | 推荐 | `<q>` |

---

## 十一、Ruby CSS 属性速查

| 属性 | 推荐值 | 状态 | 备注 |
|---|---|---|---|
| `ruby-position` | `over` | 可用 | 横排注音在上 |
| `ruby-align` | `center` | 推荐 | 居中 |
| `rt font-size` | `0.5em` | 推荐 | 注音字号 |
| `rt line-height` | `1` | 推荐 | 防撑行 |
| `rp display` | `none` | 推荐 | 支持 ruby 时隐藏括号 |
| `p:has(ruby)` | `line-height: 2` | 渐进增强 | 新 WebKit / Chromium |
| `.has-ruby` | `line-height: 2` | 推荐 | 老阅读器兜底 |

---

## 十二、弹出注释速查

| 项 | 推荐写法 | 状态 | 备注 |
|---|---|---|---|
| 触发元素 | `<a epub:type="noteref" role="doc-noteref">` | 推荐 | 放图片图标 |
| 触发图标 | `<img alt="注" src="../Images/note.png"/>` | 推荐 | 项目默认 |
| 注释容器 | `<aside epub:type="footnote" role="doc-footnote">` | 推荐 | 每个 XHTML 一个 |
| 注释列表 | `<ol class="footnote-list">` | 推荐 | 承载本文件全部注释 |
| 单条注释 | `<li class="footnote-item" id="footnote-1">` | 推荐 | noteref 跳转目标 |
| 返回符号 | `◎` | 推荐 | demo 实践采用 |
| 返回属性 | `epub:type="backlink" role="doc-backlink"` | 推荐 |  |
| noteref id | `note-1` | 推荐 | 全文唯一 |
| footnote id | `footnote-1` | 推荐 | 放在 `li.footnote-item` 上，与 href 对应 |

---

## 十三、表格 CSS 属性速查

| 属性 | 推荐值 | 状态 | 备注 |
|---|---|---|---|
| `border-collapse` | `collapse` | 推荐 | 表格边线合并 |
| `width` | `100%` | 推荐 | 表格占满内容区 |
| `thead display` | `table-header-group` | 可用 | 长表格跨页表头 |
| `tfoot display` | `table-footer-group` | 可用 | 长表格表尾 |
| `overflow-x` | `auto` | 推荐 | `.table-wrap` 横向滚动 |
| `nth-child(even)` | 隔行背景 | 可用 | 墨水屏用浅灰 |

---

## 十四、夜间模式 / 媒体查询 / 变量速查

| 特性 | 推荐写法 | 状态 | 备注 |
|---|---|---|---|
| 夜间模式 | `@media (prefers-color-scheme: dark)` | 渐进增强 | 阅读器可能覆盖作者色 |
| 窄屏 | `@media (max-width: 600px)` | 可用 | 并排图、表格回退 |
| 极窄屏 | `@media (max-width: 400px)` | 可用 | 小幅调字号 |
| CSS 变量 | `:root { --name: value; }` | 可用 | 主流支持 |
| 变量调用 | `var(--name)` | 可用 | 不放在 `@font-face src` |
| 锚点偏移 | `scroll-margin-top` | 可用 | 跳转后避开顶栏 |

---

## 十四点五、多看 fallback 属性速查

| 项目 | 写法 | 作用 |
|---|---|---|
| 多看触发类 | `class="duokan-footnote"` | noteref 兼容触发 |
| 多看条目类 | `class="duokan-footnote-item"` | 兼容列表项 |
| 多看内容类 | `<ol class="footnote-list duokan-footnote-content">` —— 两个类同挂在 `<ol>` 上，不允许只挂 `duokan-footnote-content` | 弹窗内容匹配点 |

## 十五、A-lite 属性速查

| 属性 / 结构 | 推荐值 | 状态 | 备注 |
|---|---|---|---|
| `body.fullpage` | 页面根类 | 推荐 | 整页海报 |
| `.fullframe` | 内容框 | 推荐 | 承载叠加文字 / 图片 |
| `font-size` | `16px` | 推荐 | A-lite 内部基准 |
| `body.poster-bg` | 背景 modifier | 推荐 | 海报背景容器 |
| `.fullframe` | `padding: 0` | 推荐 | A-lite 骨架 |
| `.fullframe` | `overflow: visible` | 推荐 | 避免竖排列被骨架裁切 |
| `min-height` | `100%` | 推荐 | fullpage |
| `body.fullpage` | `overflow: hidden` | 推荐 | 限制整页背景与分页 |
| `page-break-before` | `always` | 推荐 | 单独成页 |
| `page-break-after` | `always` | 推荐 | 单独成页 |
| `page-break-inside` | `avoid` | 推荐 | 避免内部断页 |
| `background-image` | 海报背景 | 推荐 | 放在 body |
| `writing-mode` | `vertical-rl` | 推荐 | 竖排标题 |
| `float` | `right` | 推荐 | 多列从右往左堆 |

---

## 十六、兼容记录速查

| 特性 | Apple Books | Thorium | Calibre | Kindle KFX | KOReader | 状态 |
|---|---|---|---|---|---|---|
| `.ttf/.otf @font-face` | 可用 | 可用 | 可用 | 可用 | 可用 | 推荐 |
| `ibooks:specified-fonts` | 必需 | N/A | N/A | N/A | N/A | 推荐 |
| A-lite 海报 | 实测可用 | 可用 | 可用 | 实测可用 | 可用 | 推荐 |
| 图片图标弹注 | 可用 | 可用 | 可用 | 可用 | 可用 | 推荐 |
| `text-emphasis` | 可用 | 可用 | 可用 | 可用 | 可用 | 推荐 |
| `figure + float + width(%)` | 可用 | 可用 | 可用 | 可用 | 可用 | 推荐 |
| MathML | 可用 | 可用 | 可用 | Enhanced Typesetting | 视版本 | 条件可用 |
| `text-decoration-style: wavy` | 可用 | 可用 | 可用 | 退化为 underline | 视版本 | 渐进增强 |
| `ruby + rt` | 可用 | 可用 | 可用 | KFX 可用 | 可用 | 推荐 |
| `writing-mode: vertical-rl` | 可用 | 可用 | 可用 | KFX 可用 | 可用 | 推荐 |
| legacy fallback（多看） | 条件可用 | 条件可用 | 条件可用 | 不适用 | 条件可用 | 按需 |
| CSS 变量 | 可用 | 可用 | 可用 | 可用 | 可用 | 可用 |
| `@media (max-width)` | 可用 | 可用 | 可用 | 视版本 | 可用 | 可用 |
| `prefers-color-scheme` | 可用 | 可用 | 可用 | 视设备 | 可用 | 渐进增强 |
| `:has()` | 新版可用 | 可用 | 可用 | 不稳定 | 视版本 | 渐进增强 |

---

## 十七、HTML 查询版

本速查表另有 HTML 版：

```text
EPUB 3 HTML CSS 属性速查表.html
```

HTML 版用于本地打开、搜索属性名、标签名、用途和状态。
