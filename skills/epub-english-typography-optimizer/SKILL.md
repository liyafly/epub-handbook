---
name: epub-english-typography-optimizer
description: 优化英文 EPUB 书籍排版，包括书籍类型判断、英语语言标记、serif 字体链、段落缩进、标题层级、插图、断字、引用/诗歌/信件、Kindle/Readest/Apple Books 大字号回归。用于英文小说、散文、非虚构或英文为主的 EPUB 需要形成可重排、可验证的排版方案时。
---

# EPUB 英文书籍排版优化

这个 skill 负责英文为主的 EPUB 排版与优化。中文/CJK 正文仍交给 `epub-typography-optimizer`，图片环绕交给 `epub-image-layout-optimizer`，OPF/nav 交给 `epub-package-nav-auditor`。

## 固定目标

英文书籍排版先做类型判断，再落到最小稳定 CSS：

- 所有英文 XHTML 声明 `xml:lang` / `lang`，让断字、朗读和词典规则可用。
- 正文使用短 serif 链，不沿用 CJK 字体链。
- 小说/散文首段无缩进，后续段落适度缩进，普通连续段落不堆大段距。
- 非虚构保留标题层级、列表、表格和引用结构，段落节奏比小说更疏。
- 插图默认用居中 `figure`；只有需要图文并列时才走图片环绕专项 skill。
- `justify`、浮动 drop cap、复杂装饰都必须有阅读器复测，不作为未验证主路径。
- 大合集、分卷目录和 `(•)` 回目录点按 `docs/guides/anthology-navigation.md` 处理，不混入普通英文正文排版。
- 便签、资料卡和装饰边框按 `docs/guides/note-box-border-styles.md` 处理。

## 类型判断

先把英文书分到一类：

- 小说 / 儿童文学：连续 prose、短章、少量插图。主路径是首段无缩进、后续缩进、居中章题和插图。
- 散文 / 回忆录：类似小说，但章节小标题和 epigraph 较多；保留 blockquote / epigraph 语义。
- 非虚构：标题层级、列表、表格、脚注、图表更多；正文可用段间距辅助扫描。
- 诗集 / 戏剧：保留行分隔和 speaker，不强行缩进成普通段落。
- 双语 / 学术：先定义语言边界和注释策略，再处理字体与脚注。

## CSS 起点

小说 / 散文：

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
```

非虚构：

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
```

## 工作流

1. 读取 OPF metadata、XHTML `lang`、CSS、nav/NCX 和样本章节。
2. 判断书籍类型：小说、散文、非虚构、诗集、戏剧、双语或混合。
3. 建立页面角色：
   - 封面 / title page / copyright / dedication / contents
   - chapter opener
   - body chapter
   - extract / letter / poem / dialog
   - figure / table / note / appendix
4. 按类型选择段落策略：
   - 小说：首段无缩进，连续段落缩进，段间距接近 0。
   - 非虚构：段间距和标题层级更清晰，普通段落可不缩进。
   - 诗/戏剧：保留行与 speaker，不强制 justify。
5. 设置字体和断字：
   - 英文正文短 serif 链。
   - `hyphens:auto` + `-webkit-hyphens:auto`。
   - 如果目标阅读器断字不稳定，保持 `text-align:left`。
   - 只有设计复现、small caps、扩展 Latin 或目标平台验证需要时才嵌入英文字体。
6. 处理装饰：
   - 首字优先用 `::first-letter`，保持正文单词完整。
   - 旧式 `<span>` 首字只作为兼容 fallback，使用前检查朗读和复制文本。
   - small caps 可用真实文本 + CSS，不转图片。
   - 复杂 drop cap、ornament、章首图只作为增强。
7. 处理插图：
   - 小说插图默认居中 `figure`。
   - 图文环绕或 caption 异常时，切到 `epub-image-layout-optimizer`。
8. 更新 fixture / docs：
   - 新规则先落到 `Text/18-english-fiction.xhtml` 或新增更具体 fixture。
   - 阅读器结果写入 `docs/final/reader-matrix.yaml`。

## 专题分流

- 大合集/分卷导航：使用 `docs/guides/anthology-navigation.md`。
- 便签/边框/阴影：使用 `docs/guides/note-box-border-styles.md`。
- 图文环绕：切到 `epub-image-layout-optimizer`。
- OPF/nav/NCX：切到 `epub-package-nav-auditor`。

## 禁止事项

- 不把英文正文套用中文 2em 缩进和 CJK 字体链。
- 不在未验证 hyphenation 时强制英文 justify。
- 不把章节标题或首字做成图片来追求固定视觉。
- 不使用固定页高、viewport unit 或 absolute positioning 承载普通英文正文。
- 不为了版式修改作者原文、引号、拼写或诗行。
- 不删除现有 `lang`、页码锚或语义结构，除非确认它们是错误残留。

## 验证 fixture

- `Text/18-english-fiction.xhtml`：英文小说基础正文、章首插图、首段/后续段落、`::first-letter` 首字、大字号回归。
- `Text/08-long-mixed-flow.xhtml`：长英文 token 与混排裁切。
- `Text/17-image-layout.xhtml`：需要环绕图时的对照 fixture。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
git diff --check
```

阅读器复测至少覆盖 Readest、Kindle Previewer 和 Apple Books；记录默认字号和大字号两种状态。
