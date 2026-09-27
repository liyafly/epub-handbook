# 可复用 EPUB 版式配方

这里按要制作的页面角色索引 demo 中的真实 XHTML 源码。配方展示结构、CSS 层、试用范围和降级方式；Markdown 目录本身不渲染 EPUB，也不代表任何阅读器已经通过。完整 fixture 覆盖见 [SCENE_MATRIX.md](SCENE_MATRIX.md)，阅读器证据见 [`docs/final/reader-matrix.yaml`](../../docs/final/reader-matrix.yaml)。

## 怎样试一页

先用 catalog 找到来源页，再打开下表链接核对 XHTML 结构和 CSS。局部预览以同一个 frozen EPUB 为输入，输出到新的候选路径：

```sh
epub run epub.style.demo.maintain catalog=true query=poetry
epub capabilities --id epub.typography.optimize

epub run epub.typography.optimize --input "$BASE" --output "$CANDIDATE" --dry-run --json \
  preset=poetry-cn 'scope_paths=["OEBPS/Text/29-poetry.xhtml"]'
```

确认 dry-run 报告中的目标文件、CSS 动作和字体模式后，再对相同 `$BASE` 与新路径实跑；运行全项 redline、导航审计和人工 diff review。局部预设只向选中的 spine 页追加独立 CSS，不改正文或共享样式。诗歌、书信、对白等角色 class 必须已存在；预设不会自动把普通段落转换成角色结构。

此命令是通用示意，`$BASE` 必须是冻结过的 demo EPUB，`$CANDIDATE` 必须是不存在的新路径。页面专用限制见各配方。静态结构检查、候选红线和真实阅读器显示分别记录；新页面的字号、窄屏、主题和分页仍待按矩阵实测。

## 配方

### 1. 素净中文正文

- **源码和定位：** [`01-body.xhtml`](OEBPS/Text/01-body.xhtml)，章节 `section[epub:type=chapter]` 中普通 `p`、`blockquote`、`figure`。
- **样式层：** `base.css`；保留真实段落和语义标签，不为默认主题设置页面颜色。
- **试用范围：** `OEBPS/Text/01-body.xhtml`。同时检查长段落和混排压力页 [`08-long-mixed-flow.xhtml`](OEBPS/Text/08-long-mixed-flow.xhtml)。
- **限制和降级：** 缩进与两端对齐不适合所有语言或短段落；遇到长 token、窄屏或大字号问题时保留自然换行并优先让用户设置生效。
- **阅读器状态：** 待测，见 reader matrix。

### 2. 轻量章首

- **源码和定位：** [`20-chapter-head-image.xhtml`](OEBPS/Text/20-chapter-head-image.xhtml) 的 `.chapter-header`、`h1`、kicker 和图；[`28-chapter-opening-block.xhtml`](OEBPS/Text/28-chapter-opening-block.xhtml) 是不带图的真实标题对照。
- **样式层：** `literary.css`，标题使用真实 `h1`，头图使用 `figure` / `img`。
- **试用范围：** 在目标章首 spine 页局部试用，核对图的实际比例和大字号标题。
- **限制和降级：** 装饰图可移除，标题仍须可见；宽幅图片不应被裁切，标题组不应锁定固定页高。
- **阅读器状态：** 头图旧记录为待复测；块级章首页待验证。

### 3. 题记与献词

- **源码和定位：** [`15-frontmatter.xhtml`](OEBPS/Text/15-frontmatter.xhtml) 后半部 `.dedication` 和 `.epigraph`；长出处样式见 [`21-classical-modern.xhtml`](OEBPS/Text/21-classical-modern.xhtml) 的 `.parallel-source`。
- **样式层：** `literary.css`。
- **试用范围：** `OEBPS/Text/15-frontmatter.xhtml`。
- **限制和降级：** 题记、出处按文档顺序保留；居中、楷体只是增强，去掉 CSS 后仍可读。
- **阅读器状态：** 待测。

### 4. 场景分隔

- **源码和定位：** [`18-english-fiction.xhtml`](OEBPS/Text/18-english-fiction.xhtml) 的 `.en-break`；本样例用真实字符，不用伪元素生成关键文字。
- **样式层：** `literary.css` 的 `.scene-break` / `.scene-break-text` 是对应通用角色。
- **试用范围：** 对已存在的场景边界元素追加显式 class，不要把普通标点批量识别成分隔符。
- **限制和降级：** 分隔符文字本身必须留在 XHTML；CSS 线条、间距可以丢失。
- **阅读器状态：** 待测。

### 5. 诗歌

- **源码和定位：** [`29-poetry.xhtml`](OEBPS/Text/29-poetry.xhtml)，容器 `.poetry`、各节 `.stanza`；短诗定位 `#poem-short`，长行定位 `#poem-long-line`。
- **样式层：** `poetry-cn` 预设的 `literary.css`；每节使用真实段落和 `<br />` 表达原始诗行。
- **试用范围：** `OEBPS/Text/29-poetry.xhtml`。对照普通前置正文仍保持常规段落排版。
- **限制和降级：** 只有原稿已确定分行/分节时才使用；长行应自然折行，长诗不能整体禁止分页。不要把普通 `p` 自动重排成诗行。
- **阅读器状态：** 新增页面，待默认字号、大字号、窄屏和跨页实测。

### 6. 书信与日记

- **源码和定位：** [`30-letter.xhtml`](OEBPS/Text/30-letter.xhtml)，`.letter` 容器、称谓、正文、`.letter-close`、`.letter-signature` 和日期。
- **样式层：** `literary.css`。
- **试用范围：** `OEBPS/Text/30-letter.xhtml`。页面包含短字段和长信段落。
- **限制和降级：** 落款对齐、左边线可丢失；长信允许跨页，不对整封信设置 `page-break-inside: avoid`。
- **阅读器状态：** 新增页面，待默认字号、大字号、窄屏和跨页实测。

### 7. 访谈与对白

- **源码和定位：** [`31-dialogue.xhtml`](OEBPS/Text/31-dialogue.xhtml)，`.dialog` 中每段话用真实的 `.dialog-speaker` 文本标出发言者。
- **样式层：** `literary.css`。
- **试用范围：** `OEBPS/Text/31-dialogue.xhtml`，包含短轮次和长回答。
- **限制和降级：** 发言者不依赖颜色、图标或 CSS 伪内容；复制文本和去样式阅读时仍按源序交替。
- **阅读器状态：** 新增页面，待默认字号、大字号和窄屏实测。

### 8. 引用与资料卡

- **源码和定位：** [`19-border-shadow-notes.xhtml`](OEBPS/Text/19-border-shadow-notes.xhtml) 的 `.note-box`、`.note-left-rule`、`.note-double` 和 `.note-long-shadow`。
- **样式层：** `effects.css`。
- **试用范围：** 先用短卡，再选长卡核对分页；如原书使用语义引用，优先保留 `blockquote`。
- **限制和降级：** 阴影、圆角和花边只做增强；卡片不能整体禁止分页以至于长内容留白或溢出。
- **阅读器状态：** 既有阅读器记录为 warn / 待复测；不可外推为新 preset 的结论。

### 9. 单图与图注

- **源码和定位：** [`17-image-layout.xhtml`](OEBPS/Text/17-image-layout.xhtml) 的 `figure` / `figcaption`，另见 [`01-body.xhtml`](OEBPS/Text/01-body.xhtml) 中的基础图文。
- **样式层：** `base.css` 和 `media.css`。
- **试用范围：** 核对原始 `src`、`alt` 和图注后再局部试用；横向、纵向图各检查一例。
- **限制和降级：** 按比例缩放，不猜写 `alt`、不裁切信息图；图注必须为文本且紧邻图片。
- **阅读器状态：** 既有页面仍有字号和尺寸待复测项。

### 10. 表格与代码

- **源码和定位：** [`04-lists-tables-code.xhtml`](OEBPS/Text/04-lists-tables-code.xhtml) 的 `.table-wrap`、带 `thead` 的表格和 `pre > code`。
- **样式层：** `base.css`、`media.css`。
- **试用范围：** 同时检查短表、长代码行、大字号及复制文本顺序。
- **限制和降级：** 保留表格语义表头；横向滚动不是所有阅读器都支持，代码行可换行或由读者缩放，不能隐藏关键信息。
- **阅读器状态：** 待测。

### 11. 古文与原译对照

- **源码和定位：** [`21-classical-modern.xhtml`](OEBPS/Text/21-classical-modern.xhtml)，`.parallel-entry` 和 `.parallel-pair`，样例锚点 `#entry-mountain-bell`、`#entry-river-letter`。
- **样式层：** `literary.css`，角色用 `.classical-text` / `.modern-text`。
- **试用范围：** 比较短组和长组，核对每组原文先于译文。
- **限制和降级：** 小屏、大字号和长段落采用源序上下排列；并排只作宽视口增强，不使用表格承载正文。
- **阅读器状态：** 历史记录有待复测项目；新候选需按精确 SHA 重新记录。

### 12. 英文小说

- **源码和定位：** [`18-english-fiction.xhtml`](OEBPS/Text/18-english-fiction.xhtml)，根 `lang="en"`、`.english-fiction`、`.en-noindent`、`.en-extract` 和图注。
- **样式层：** `fiction-en` 预设；`font-en-serif` 只在原 XHTML 显式标记时作为可选增强。
- **试用范围：** 从实际英文 spine 页小范围试用；确认该页语言后再用 `epub.typography.english.optimize` 修补缺失声明。
- **限制和降级：** 不把英文规则套到中文段落；保持长单词可换行，不默认依赖断字或浮动首字。
- **阅读器状态：** 有旧 artifact 的 warn 记录；需要用新 artifact 和确切阅读器版本复测。

新增页面的正文均为本仓自造的中性样本。没有截图是有意的：源码索引不伪装成视觉预览；真实阅读器结果应按 artifact SHA 写入 reader matrix。
