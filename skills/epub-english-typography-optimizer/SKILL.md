---
name: epub-english-typography-optimizer
description: 优化英文 EPUB 书籍排版，包括书籍类型判断、英语语言标记、serif 字体链、段落缩进、标题层级、插图、断字、引用/诗歌/信件、Kindle/Readest/Apple Books 大字号回归。用于英文小说、散文、非虚构或英文为主的 EPUB 需要形成可重排、可验证的排版方案时。
---

# EPUB 英文书籍排版优化

## 何时用

- 英文为主的 EPUB 排版与优化。分工边界：中文/CJK 正文用 `epub-typography-optimizer`；图片环绕用 `epub-image-layout-optimizer`；OPF/nav 用 `epub-package-nav-auditor`；大合集/分卷导航见 [docs/how-to/anthology-navigation.md](../../docs/how-to/anthology-navigation.md)；便签/边框见 [docs/how-to/note-box-border-styles.md](../../docs/how-to/note-box-border-styles.md)。
- 先做类型判断：小说/儿童文学（连续 prose、短章、少量插图）；散文/回忆录（小标题与 epigraph 多，保留 blockquote/epigraph 语义）；非虚构（标题层级、列表、表格、脚注、图表多）；诗集/戏剧（保留行分隔和 speaker，不强行缩进）；双语/学术（先定义语言边界和注释策略）。
- 固定目标：所有英文 XHTML 声明 `xml:lang`/`lang`，让断字、朗读和词典规则可用；正文用短 serif 链，不沿用 CJK 字体链；小说/散文首段无缩进、后续段落适度缩进、普通连续段落不堆大段距；非虚构保留标题层级、列表、表格和引用结构，段落节奏更疏；插图默认居中 `figure`，图文并列才走图片环绕专项 skill；`justify`、浮动 drop cap、复杂装饰必须有阅读器复测，不作为未验证主路径。
- 禁止事项：不把英文正文套用中文 2em 缩进和 CJK 字体链；不在未验证 hyphenation 时强制英文 justify；不把章节标题或首字做成图片；不用固定页高、viewport unit 或 absolute positioning 承载普通英文正文；不为版式修改作者原文、引号、拼写或诗行；不删除现有 `lang`、页码锚或语义结构（除非确认是错误残留）。

## 调什么

本 skill 是 AI 分析与手工精排类 skill：读 OPF metadata、XHTML `lang`、CSS、nav/NCX 和样本章节，建立页面角色（封面/title page/copyright/dedication/contents、chapter opener、body chapter、extract/letter/poem/dialog、figure/table/note/appendix）后落地 CSS。改书后必须跑校验组合：

```sh
epub run epub.notes.popup.normalize --input <产物> --json    # 涉及弹注时
epub run epub.style.demo.maintain --input <demo 产物> --json # 涉及 demo 模板时
epub redline --check all <before.epub> <after.epub>          # 每次改书后
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`、`text_files`、`violations`；violations 对应 `error popupnotes` findings。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过。

## 依据返回怎么判断

- 按类型选段落策略：小说——首段无缩进、连续段落缩进（`text-indent: 1.35em`）、段间距接近 0；非虚构——段间距（约 `.45em`）和标题层级更清晰，普通段落可不缩进；诗/戏剧——保留行与 speaker，不强制 justify。
- 字体与断字：英文正文短 serif 链（如 `Georgia, "Times New Roman", "Noto Serif", serif`）；`hyphens: auto` + `-webkit-hyphens: auto`；目标阅读器断字不稳定时保持 `text-align:left`；只有设计复现、small caps、扩展 Latin 或目标平台验证需要时才嵌入英文字体。
- 装饰处理：首字优先用 `::first-letter` 保持正文单词完整；旧式 `<span>` 首字只作兼容 fallback，使用前检查朗读和复制文本；small caps 用真实文本 + CSS，不转图片；复杂 drop cap、ornament、章首图只作增强。
- 插图：小说插图默认居中 `figure`；图文环绕或 caption 异常时切到 `epub-image-layout-optimizer`。
- `findings` 出现 `error`（含 `popupnotes`、`redline.*`）→ 回滚或修复后重跑；`status == approval-required` → 停下来问人。
- 阅读器复测至少覆盖 Readest、Kindle Previewer 和 Apple Books，记录默认字号和大字号两种状态；新规则先落 fixture（`Text/18-english-fiction.xhtml` 基础正文/章首插图/首段与后续段落/`::first-letter`/大字号回归，`Text/08-long-mixed-flow.xhtml` 长英文 token 与混排裁切，`Text/17-image-layout.xhtml` 环绕对照），实测后回写 `docs/final/reader-matrix.yaml`，再沉淀到 SPEC 与手册。
