# 英文正文排版与优化方案

> 状态：基础规则；用于英文小说、散文、普通非虚构、儿童文学和 prose 类图书。大合集/分卷导航见 `anthology-navigation.md`；便签、边框和阴影见 `note-box-border-styles.md`。

## 核心判断

英文书和中文书的 EPUB 排版方法大体一致：先判断文本角色，再用少量稳定类处理正文、章首、引文、诗、信件、插图、注释和附录。差异主要集中在字体、首段、缩进、断字和英文特有的强调方式。

英文正文的默认路径：

- 小说、散文、儿童文学：首段不缩进，后续段落缩进。
- 非虚构：可不缩进，用段间距和标题层级提升扫描效率。
- 章首：可以首字下沉，也可以不用下沉；不用下沉时仍保持首段不缩进。
- 强调：按语义使用 italic、bold、small caps 或手写体/签名字体；不要把普通正文换成花哨字体。
- 字体：普通英文 prose 默认系统 serif 链；只有特定设计场景才嵌入字体。

## 从 Stuart Little 样本得到的结构观察

该本地参考 EPUB 是典型简单英文小说结构：EPUB 2 package、NCX 目录、单 CSS、章节 XHTML、JPG 插图。正文主要由居中插图、居中章标题、首段无缩进加轻量首字、后续缩进段落组成；CSS 没有 float、absolute positioning 或复杂布局。这说明它的重点不是精装饰，而是让英文 prose 在不同阅读器里稳定重排。

这些观察只用于提炼版式规则，不复制原书正文。

## 英文和中文的共同点与差异

共同点：

- 都要先区分正文、标题、引文、题记、信件、诗、插图、注释和附录。
- 都要避免固定页高、绝对定位和把正文烘焙进图片。
- 都要做默认字号和大字号复测。
- 不同文体或情景可以使用不同字体、边框、摘录框或装饰，但装饰不能破坏可重排。

差异：

- 字体链：中文用 CJK serif/sans 链；英文 prose 通常用 serif 链。
- 缩进：中文正文常用 `2em`；英文小说常用 `1.2em-1.5em`，首段不缩进。
- 对齐：中文常用两端对齐；英文在没有稳定 hyphenation 前优先左对齐。
- 首字：英文常见 `::first-letter` 或 drop cap；中文首字下沉只适合特定文学效果。
- 断字：英文需要 `lang` + `hyphens:auto`；中文更关注标点、禁则和 CJK 混排。

## 字体与嵌入策略

默认不必嵌入英文字体。系统 serif 链已经足够完成大多数英文小说和普通非虚构：

```css
.english-fiction {
  font-family: Georgia, "Times New Roman", "Noto Serif", serif;
}
```

需要嵌入字体的场景：

- 特定版本设计必须复现，比如古典短篇集、诗集或品牌化出版物。
- small caps、题签、卷首标题、签名档等需要特定字形。
- 书中有大量扩展 Latin、音标、希腊字母或特殊符号，目标系统字体覆盖不稳。
- 阅读器默认字体破坏全书气质，且目标平台已验证 Publisher Font / 原版字体开关。

嵌入时不要把所有字体塞进 `body`。推荐把正文、small caps、手写体/签名体分开：

```css
.english-body {
  font-family: "BookSerif", Georgia, "Times New Roman", serif;
}

.smallcaps {
  font-family: "BookSerifSC", Georgia, serif;
  font-variant: small-caps;
}

.handwritten-note {
  font-family: "BookHand", "Bradley Hand", cursive;
}
```

OPF 必须声明实际打包的字体文件，并继续保留通用 fallback。Kindle Previewer 要测试 Publisher Font；Apple Books 要测试原版字体设置。

## 段落节奏

小说/散文起点：

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
```

规则：

- 连续正文段间距接近 0，靠缩进分段。
- 章首第一段、标题后第一段、插图后第一段通常不缩进。
- 不要同时使用大段距和大首行缩进。
- 换场用居中文本分隔符或语义 `hr.scene-break`，不要用空段落堆间距。

非虚构起点：

```css
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

## 章首与首字

英文章首有两条安全路径：

1. 不下沉：章标题后首段 `text-indent:0`，后续段落正常缩进。
2. 轻量首字：使用 `::first-letter`，不要拆开单词。

```css
.en-first-letter::first-letter {
  font-size: 1.75em;
  line-height: .8;
  font-weight: 700;
}
```

浮动式 drop cap 只作为增强，必须在 Kindle Previewer、Readest、Apple Books 和大字号下复测。不要用 `<span>T</span>he` 这种拆词方式作为主路径，它会影响复制、朗读和辅助技术。

## 强调与特殊文本

- 斜体：书名、内心独白、外语词、强调语气。按语义用 `em` 或项目专用类，不要全书滥用。
- 加粗：警示、术语第一次出现、教材提示。普通小说中少用。
- small caps：人物信件抬头、铭文、报纸标题、古典书籍装饰。需要字体支持时再嵌入。
- 手写体/签名体：只用于短签名、便条、题词。正文不能依赖手写体。
- 等宽体：代码、档案编号、电报、机器输出。用短 monospace 链。

## 插图

- 小说插图默认使用通栏或小幅居中 `figure`，不做环绕主路径。
- `img` 使用 `max-width:100%; height:auto;`；插图类用 `max-width` 控制视觉大小。
- 保留 `alt` 和可选 `figcaption`。
- 不固定页面高度，也不用 fixed layout 包装普通插图。

```css
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

## 类型化推荐

| 类型 | 段落 | 标题 | 特别注意 |
|---|---|---|---|
| 小说 / 儿童文学 | 首段 0 缩进，后续 `1.2em-1.5em` 缩进，段间距 0 | 居中或轻装饰 | 不强制 justify；首字先用 `::first-letter` |
| 散文 / 回忆录 | 可同小说；章节内小标题后首段 0 缩进 | 保留小标题层级 | epigraph / extract 用 blockquote 或专用类 |
| 非虚构 | 段间距 `.4em-.6em`，可不缩进 | 左对齐层级更清楚 | 表格/代码需要横向滚动和大字号测试 |
| 诗集 | 不改诗行；行内 `<br/>` 或每行独立块 | 低装饰 | 不 justify，不自动合并空行 |
| 戏剧 | speaker 与台词分开 | 场景标题清晰 | speaker 缩进/悬挂缩进要可重排 |
| 双语 / 学术 | 按语言段落分别 `lang` | 层级清楚 | 注释、术语和引用优先，不追求花哨视觉 |

## 工作流

1. 先跑 `scripts/epub_ai_harness.py <path>`，确认 EPUB 版本、OPF/nav/NCX、语言、图片格式和脚注风险。
2. 抽 2-3 个代表章节：普通章节、插图章节、标题/列表/引用密集章节。
3. 修 `lang` / `xml:lang`。
4. 归一段落类：首段、普通段、无缩进段、摘录/引用、诗/戏剧。
5. 图片转 `figure`，保留 alt/caption。
6. 同步 OPF/nav/NCX。
7. 在 Readest、Kindle Previewer、Apple Books 跑默认字号和大字号复测。
8. 结果写回 `docs/final/reader-matrix.yaml`，未验证保持 `warn`。

## 检查清单

- 是否所有英文 XHTML 都有 `lang`。
- 是否仍有中文正文字体链套在英文正文上。
- 是否同时使用大段距和大首行缩进。
- 是否把英文正文强制 justify，但没有断字复测记录。
- 是否有固定 `height`、`vh/vw`、absolute positioning 或整页截图承载正文。
- 是否把章节标题、首字、插图说明做成不可选中的图片。
- 是否在大字号下检查过章首空白、插图尺寸、caption 和长词换行。

对应 demo fixture：`templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml`。
