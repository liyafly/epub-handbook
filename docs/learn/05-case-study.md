# 自造 EPUB 清洗演示案例

本页汇总 Stage 4 的自造 before / after EPUB。`templates/cleanup-demo-books/dist/` 下的生成产物由本地脚本生成，不随仓库提交；每个样本目录提供 `notes.md` 复现流程。

## 先生成样本

```sh
bash templates/cleanup-demo-books/build.sh
```

输出目录：`templates/cleanup-demo-books/dist/`。

## 案例 1：明日城巡游手记

样本记录：[templates/cleanup-demo-books/city-field-notes/notes.md](../../templates/cleanup-demo-books/city-field-notes/notes.md)。

### 做了什么

| 维度 | before | after | 为什么这样改 |
| --- | --- | --- | --- |
| CSS 结构 | 单个大 CSS 文件，正文、表格、脚注、图片规则混在一起 | 拆成 `base.css` / `media.css` / `notes.css` / `tables.css` | 每种职责一个文件，改脚注不碰表格，改图片不影响正文 |
| 内联样式 | 章节标题用 `style="font-size:22px"` 等内联样式 | 迁移到 `base.css` 的 `h1`/`h2` 选择器 | 内联样式很难被用户字号设置覆盖，且无法统一调整 |
| 图片 | 直接 `img` 无外层容器 | `figure` 包裹，加 `figcaption` | 图片与说明文字在语义上绑定，阅读器能更好处理 |
| 弹注 | 普通 `<a href="#note-1">` 跳转链接，无回跳 | 双向链接 + 回跳，保留标准 `<div>` 容器 | 不引入 EPUB 3 语义（`epub:type`），保持兼容；但补了回跳让读注释后能回到正文 |

CSS 拆分的关键原则：**拆，不是叠。** 不是把旧 CSS 复制四份再分别加新规则，而是按职责将旧文件中的每条规则归类到一个文件。拆分后，一个文件只做一件事：`base.css` 管正文节奏和标题，`notes.css` 管弹注和回跳，`media.css` 管图片和图注，`tables.css` 管表格样式。改脚注时改 `notes.css` 就行，不用担心误触正文排版。

> 说明：这里的 `base/media/notes/tables.css` 是该样本书自身的简化拆分；模板 `templates/epub-style-demo/` 用的是更细的 8 层方案（`fonts/base/notes/effects/literary/media/vertical/poster.css`，见 SPEC §7 和 [02-anatomy.md](02-anatomy.md) §5）。两者都合法，按书的复杂度选。新书从模板起步时以 8 层为参考。

### 红线 gate：validate_text_invariance.py 做了什么

```sh
python3 scripts/validate_text_invariance.py before.epub after.epub
```

这个脚本检查以下几个维度，任何一个不通过都视为红线触发：

| 维度 | 检查方式 | 为什么是红线 |
| --- | --- | --- |
| 文本内容 | 提取所有 XHTML 的文本节点，做归一化比较 | 清洗不能改正文一个字 |
| 元素存在性 | 比对 before/after 的元素标签序列 | 不能增删段落、标题、注释文本 |
| spine 顺序 | 比较两个 OPF 的 `<spine>` 子元素顺序 | 阅读顺序不能改变 |
| 核心 metadata | 比较 dc:identifier、dc:title、dc:creator、dc:language | 书的基本身份不能变 |

它不检查的内容：CSS 属性值变化、文件名变化、目录结构变化——这些属于黄线（允许改但需人工确认）或绿线（格式化噪声）。

### 用外部 diff 工具看五层变化

清洗完成后，用 Calibre Editor 的 "Compare to another book" 或 VS Code 查看差异。每层看什么：

| 层 | 看什么 | 正常 | 异常 |
| --- | --- | --- | --- |
| 结构 | 文件增删、目录变化 | 资源文件增加了 CSS 分片或规范化了文件名 | 正文 XHTML 数量变了、spine 顺序变了 |
| 文本 | 每个 XHTML 的文本节点 | 完全一致 | 任何一个字的变化 |
| 样式 | CSS 规则 | 规则拆分到不同文件、选择器更规范、内联样式消失 | 选择器改了却影响了更多元素、删了不该删的规则 |
| 资源 | 图片、字体文件 | 图片无损、字体未变 | 图片被重新压缩、字体文件被替换 |
| 元数据 | OPF metadata | 格式规范化（如 date 格式统一）、补齐了缺失字段 | 标题或作者变了、identifier 变了 |

> 详细操作指南见 [EPUB diff review](../pipeline/epub-diff-review.md)。

## 案例 2：纸上花园观察录

样本记录：[templates/cleanup-demo-books/paper-garden/notes.md](../../templates/cleanup-demo-books/paper-garden/notes.md)。

本案例重点看诗段、Ruby、blockquote、列表和宽屏竖排增强。after 只调整 CSS 和非封面插图资源，文本红线应通过。

## 反例：红线变更反例

样本记录：[templates/cleanup-demo-books/redline-trap/notes.md](../../templates/cleanup-demo-books/redline-trap/notes.md)。

这对文件故意改写正文，用来验证红线 gate 会失败。它不是合法清洗结果，只用于教学和 diff 文本层演示。

当你运行 `validate_text_invariance.py` 对比这对文件时，脚本会输出具体的差异位置和内容，让你看到红线 gate 是如何工作的。

## 学到了什么

- 现成 EPUB 的清洗要先守红线，再谈样式整理。
- CSS 拆分按职责，不是按来源；一个文件一种职责。
- `validate_text_invariance.py` 是自动化红线 gate，覆盖文本、元素、spine、metadata 四个维度。
- 外部 diff 工具（Calibre / VS Code）负责让人看见结构、文本、样式、资源、元数据五层变化。它不是自动的——需要人看懂每层的变化是否合理。
- 自造 demo 可以覆盖脚注、表格、代码、Ruby、竖排增强和红线反例，不受第三方来源质量限制。

## 下一步

- 拿你自己的 epub，跟着 [cleanup-flow.md](../pipeline/cleanup-flow.md) 跑一遍。
- 按 [EPUB diff review](../pipeline/epub-diff-review.md) 看自己的清洗结果。
