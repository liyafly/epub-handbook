# 英文书籍排版与优化方案

> 状态：基础规则；用于英文小说、散文、普通非虚构、儿童文学和 prose 类图书。复杂教材、重表格书、诗集、戏剧和双语书应在此基础上另开专项 fixture。

## 从 Stuart Little 样本得到的结构观察

该本地参考 EPUB 是典型简单英文小说结构：EPUB 2 package、NCX 目录、单 CSS、章节 XHTML、JPG 插图。正文主要由居中插图、居中章标题、首段无缩进加轻量首字、后续缩进段落组成；CSS 没有 float、absolute positioning 或复杂布局。这说明它的重点不是精装饰，而是让英文 prose 在不同阅读器里稳定重排。

这些观察只用于提炼版式规则，不复制原书正文。

## 完整优化流程

1. 输入审稿
   - 先跑 `scripts/epub_ai_harness.py <path>`，确认 EPUB 版本、OPF/nav/NCX、语言、图片格式、MathML/footnote 风险。
   - 抽 2–3 个代表章节：普通章节、插图章节、标题/列表/引用密集章节。
   - 记录当前 CSS 类：段落类、标题类、插图类、隐藏页码锚和特殊块。

2. 书籍类型判断
   - 小说 / 儿童文学：连续正文为主，插图少而独立。
   - 散文 / 回忆录：连续正文 + epigraph / extract / letter 较多。
   - 非虚构：标题层级、列表、图表、脚注、交叉引用较多。
   - 诗集 / 戏剧：行分隔、speaker、缩进层级是正文内容的一部分。
   - 双语 / 学术：语言切换、注释、术语和引用优先级高。

3. 定排版策略
   - 只给每种页面角色一套类：正文、章首、插图、摘录、诗、信件、列表/表格、注释。
   - 不把阅读器默认可处理的内容做成固定版式。
   - 不复制同一视觉规则到多个 CSS 文件；英文 prose 规则放 `literary.css` 或未来拆出的英文专层。

4. 落 CSS 与 XHTML
   - `lang` / `xml:lang` 先修。
   - 段落类归一：首段、普通段、无缩进段、摘录/引用、诗/戏剧。
   - 图片转成 `figure`，保留 alt/caption。
   - OPF/nav/NCX 同步。

5. 阅读器复测
   - 默认字号和大字号都要看。
   - Readest、Kindle Previewer、Apple Books 至少覆盖目录跳转、章首、正文、插图、引用和一页长正文。
   - 结果写回 `docs/final/reader-matrix.yaml`，未验证保持 `warn`。

## 类型化推荐

1. 语言与 metadata
   - XHTML 或 `body` 必须声明 `xml:lang="en"` / `lang="en"`；具体项目可用 `en-US` / `en-GB`。
   - EPUB 3 交付保留 `nav.xhtml`；Kindle/旧工具链交付同时保留 `toc.ncx` 和 `spine toc="ncx"`。

2. 字体与对齐
   - 英文正文使用短 serif 链：`Georgia, "Times New Roman", "Noto Serif", serif`。
   - 不沿用中文正文的 Songti/SimSun/Noto Serif CJK 链。
   - 简单英文小说优先 `text-align:left`。只有确认目标阅读器断字稳定后，才考虑 `justify`。
   - 加 `hyphens:auto` 和 `-webkit-hyphens:auto`，但不要假设所有阅读器都生效。

3. 段落节奏
   - 首段无缩进，后续正文段落缩进 `1.2em–1.5em`。
   - 普通连续段落段间距接近 0；不要同时使用大段距和大缩进。
   - 换场用居中文本分隔符或语义 `hr.scene-break`，不要用空段落堆间距。

4. 章首
   - 章首图如存在，放在标题前后都可以，但用居中 `figure`。
   - 章标题居中，字号适中，避免 Kindle 大字号下制造整页空白。
   - 罗马数字、章号和标题保留真实文本，不用图片标题。

5. 插图
   - 小说插图默认使用通栏或小幅居中 `figure`，不做环绕主路径。
   - `img` 使用 `max-width:100%; height:auto;`；插图类用 `max-width` 控制视觉大小。
   - 保留 `alt` 和可选 `figcaption`；不要固定页面高度，也不要用 fixed layout 包装普通插图。

6. 首字与小装饰
   - 简单英文小说优先 `::first-letter`，保持正文单词完整。
   - 旧式 span 首字和浮动 drop cap 只作为增强；必须在朗读、复制文本、Kindle Previewer、Readest 和 Apple Books 大字号下复测。

## 不同类型的具体策略

| 类型 | 段落 | 标题 | 插图 | 特别注意 |
|---|---|---|---|---|
| 小说 / 儿童文学 | 首段 0 缩进，后续 `1.2em–1.5em` 缩进，段间距 0 | 居中，字号适中 | 居中 figure | 不强制 justify；首字先用 `::first-letter` |
| 散文 / 回忆录 | 可同小说；章节内小标题后首段 0 缩进 | 保留小标题层级 | 居中或通栏 | epigraph / extract 用 blockquote 或专用类 |
| 非虚构 | 段间距 `.4em–.6em`，可不缩进 | 左对齐层级更清楚 | 图表优先通栏 | 表格/代码需要横向滚动和大字号测试 |
| 诗集 | 不改诗行；行内 `<br/>` 或每行独立块 | 低装饰 | 少量居中 | 不 justify，不自动合并空行 |
| 戏剧 | speaker 与台词分开 | 场景标题清晰 | 少 | speaker 缩进/悬挂缩进要可重排 |
| 双语 / 学术 | 按语言段落分别 `lang` | 层级清楚 | 图表通栏 | 注释、术语和引用优先，不追求花哨视觉 |

## 最小 CSS 模板

```css
.english-fiction {
  font-family: Georgia, "Times New Roman", "Noto Serif", serif;
  line-height: 1.55;
  hyphens: auto;
  -webkit-hyphens: auto;
}

.english-fiction p {
  margin: 0;
  text-indent: 1.35em;
  text-align: left;
}

.english-fiction .en-noindent,
.english-fiction blockquote p {
  text-indent: 0;
}

.en-first-letter::first-letter {
  font-size: 1.75em;
  line-height: .8;
  font-weight: 700;
}

.english-chapter-title {
  margin: 1em 0 1.8em;
  text-align: center;
  font-size: 1.6em;
  font-weight: 400;
}

.en-illustration {
  margin: 1.2em auto;
  text-align: center;
}

.en-illustration img {
  display: block;
  width: auto;
  max-width: 15em;
  margin: 0 auto;
}
```

非虚构起点：

```css
.english-nonfiction {
  font-family: Georgia, "Times New Roman", "Noto Serif", serif;
  line-height: 1.6;
  hyphens: auto;
  -webkit-hyphens: auto;
}

.english-nonfiction p {
  margin: .45em 0;
  text-indent: 0;
  text-align: left;
}

.english-nonfiction h1,
.english-nonfiction h2,
.english-nonfiction h3 {
  text-align: left;
  page-break-after: avoid;
}
```

诗 / 戏剧起点：

```css
.english-verse p,
.english-drama p {
  text-indent: 0;
  text-align: left;
}

.line-group {
  margin: .8em 0;
}

.speaker {
  font-variant: small-caps;
  text-indent: 0;
}
```

## 优化检查清单

- 是否所有英文 XHTML 都有 `lang`。
- 是否仍有中文正文字体链套在英文正文上。
- 是否同时使用大段距和大首行缩进。
- 是否把英文正文强制 justify，但没有断字复测记录。
- 是否有固定 `height`、`vh/vw`、absolute positioning 或整页截图承载正文。
- 是否把章节标题、首字、插图说明做成不可选中的图片。
- 是否在大字号下检查过章首空白、插图尺寸、caption 和长词换行。

## 复测清单

- Readest：目录跳转、首段缩进、插图大小、caption、英文断字和大字号表现。
- Kindle Previewer：KFX 转换日志、章节标题分页、`::first-letter` 首字、大字号下段落是否过密。
- Apple Books：`lang` / hyphenation、原版字体设置、Georgia/Times 链是否被保留、插图和 caption 是否居中。

对应 demo fixture：`templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml`。
