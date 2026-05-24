# CSS 分层指南

> 状态：仅文档；待执行模型按本文落地到 `templates/epub-style-demo/OEBPS/Styles/`、`package.opf`、各 XHTML `<head>` 的 link 顺序，以及 `docs/final/SPEC-实现约束.md §7`。  
> 上位约束：现有 SPEC §7 已有 3 层（`fonts.css` / `base.css` / `poster.css`），但 `base.css` 在 demo 扩张后会超过 300 行；本文档把它拆成可独立维护、可按页选择性引入的更细粒度文件。  
> 协同：[demo-scene-expansion-plan.md §4](./demo-scene-expansion-plan.md) 改为按本文层级安放视觉类，不再把所有内容写进 `base.css`。

---

## 1. 目标

1. **单文件控制在 400–500 行以内**：400 行作为维护预警，500 行作为硬上限；不要为了卡行数压缩 CSS，优先保持可读格式。
2. **每层只承担一个明确意图**：字体声明、正文骨架、弹注、文字效果、文学结构、图文与公式、海报、竖排。
3. **每个 XHTML 只 link 自己用到的层**：不该让"读小说体"的页面被迫加载海报页的规则。
4. **跨层依赖通过类名契约**：上层（如 `notes.css`）依赖下层（如 `base.css` 的 `body` 字体）的 CSS 变量与基础选择器；不允许下层引用上层组件类。
5. **可回退**：执行模型在落地前可选择"4 文件版"（最小拆分）或"8 文件版"（完整拆分）。两版都符合 SPEC §7 的精神。

---

## 2. 推荐分层（8 文件版）

```text
OEBPS/Styles/
├── fonts.css        — 字体声明 + 字体工具类
├── base.css         — 正文基础（reset + body + h* + p + ul/ol/dl + table + pre/code + figure + img + a + ruby + em/strong/q + blockquote）
├── notes.css        — 弹注（noteref / footnote-line / footnote-list / footnote-item / duokan fallback）
├── effects.css      — 文字效果 + 便签视觉（.emp / .wavy / .dropcap / .note-box）
├── literary.css     — 文学结构 + 前置页（.dialog / .poetry / .letter / .scene-break / .chapter-head / .chapter-head-art / .chapter-head-banner / .epigraph / .copyright-page / .dedication / .epigraph-page）
├── media.css        — 图文混排 + 公式（.img-left / .img-right / .img-center / 九宫格定位 / .figure-grid / .math-block / .math-inline）
├── vertical.css     — 整页正文竖排（body.page-vrl / .vrl-section / .vrl-title）
└── poster.css       — A-lite 海报页（body.fullpage / body.poster-bg / .fullframe / .poster-title / .poster-subtitle / .vcol）
```

### 2.1 每个文件的职责契约

| 文件 | 允许 | 禁止 |
|---|---|---|
| `fonts.css` | `@font-face`、字体工具类：默认链 `.book-song`/`.song` / `.book-hei`/`.hei` / `.book-kai`/`.kai`/`.kaiti` / `.book-fangsong`/`.fangsong` / `.book-mono`/`.mono` / `.book-latin-serif`；嵌入专用类 `.rare` / `.title-special` / `.signature`；向后兼容别名 `.book-body` / `.book-heading` / `.book-kaiti` | 排版（margin / padding / line-height / text-* 等）、颜色、分页、布局、元素选择器（`q` / `blockquote` / `body` / `h1`-`h6` 等都属 base.css 范畴） |
| `base.css` | `@page`、`html/body` 盒模型与字体链、`h1–h6` / `p` / `ul/ol/dl` / `table` / `pre/code/kbd/samp` / `figure/img/figcaption` / `a` / `em/strong/q/blockquote` / `ruby/rt/rp` 默认样式、`.has-ruby` 行距兜底 | 弹注类、文字效果类、文学结构类、图文浮动类、海报类、竖排类 |
| `notes.css` | `sup` / `.noteref-icon` / `a.duokan-footnote` / `.footnote-line` / `.footnote-list` / `.footnote-item` / `.duokan-footnote-content` / `.duokan-footnote-item` / `.footnote` / `.footnote-back` | 与 `base.css` 重复的 `sup` / `a` 基础样式（这里只加 footnote 相关增量） |
| `effects.css` | `.emp` / `.wavy` / `.dropcap` / `.dropcap-host` / `.note-box` 边框阴影类（含 SVG 花边实验、长条投影） | `.kaiti` 的字体声明（那是 fonts.css 的事；这里只能加排版微调如 `.kaiti { text-indent: 0; }` 这种） |
| `literary.css` | `.dialog` / `.dialog-speaker` / `.poetry` / `.poetry .stanza` / `.letter` / `.letter-*` / `.scene-break` / `.scene-break-text` / `.chapter-head` / `.chapter-head-art` / `.chapter-head-banner` / `.chapter-header` / `.chapter-title` / `.decorated-chapter-title` / `.sptxt` / `.epigraph` / `.epigraph-source` / `.copyright-page` / `.dedication` / `.dedication-text` / `.epigraph-page` / `.copyright-statement` | 弹注 / 文字效果 / 图文 / 海报 / 竖排 |
| `media.css` | `.img-left` / `.img-right` / `.img-center` / `.img-top` / `.img-bottom` / `.img-tl` / `.img-tr` / `.img-bl` / `.img-br` / `.figure-grid` / `.clear-both` / `.math-block` / `.math-inline` / `.math-fraction` / `.math-sqrt`（必要时给 MathML 兜底） | 普通 `figure` / `img` 基础样式（那是 `base.css`） |
| `vertical.css` | `body.page-vrl` / `.vrl-section` / `.vrl-title` / 竖排专用的 ruby 微调 | 与 `base.css` 重复的横排规则、A-lite 海报规则 |
| `poster.css` | `@page { margin:0 }`、`body.fullpage` / `body.poster-bg` / `.fullframe` / `.poster-title` / `.poster-subtitle` / `.vcol` | 正文段落规则（那是 `base.css`） |

### 2.2 加载顺序与依赖

CSS link 顺序必须遵循下层 → 上层（后写覆盖先写）：

```
fonts.css → base.css → notes.css / effects.css / literary.css / media.css / vertical.css / poster.css
```

- `fonts.css` 永远第一。
- `base.css` 永远紧随其后（提供 body 字体链、`box-sizing`、`@page`）。
- `notes / effects / literary / media / vertical / poster` 之间**无强依赖**，按需引入即可。同一页同时引入多个时，按字母序排列以便 diff。

### 2.3 每页应当 link 的文件（参考矩阵）

| Demo 页 | fonts | base | notes | effects | literary | media | vertical | poster |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| 00 标题页 | ✓ | ✓ | | | | | | |
| 01 普通正文 | ✓ | ✓ | | | | | | |
| 02 Ruby + 标准弹注 | ✓ | ✓ | ✓ | ✓ | | | | |
| 03 A-lite 海报 | ✓ | | | | | | | ✓ |
| 04 列表/表格/代码 | ✓ | ✓ | | | | | | |
| 05 多看 fallback 单条 | ✓ | ✓ | ✓ | | | | | |
| 06 多看 fallback 多条 | ✓ | ✓ | ✓ | | | | | |
| 07 font-family 顺序 | ✓ | ✓ | | | | | | |
| 08 长段中英混排 | ✓ | ✓ | | | | | | |
| 09 Kindle 风险 | ✓ | ✓ | | | | | | |
| 10 文字效果合集 | ✓ | ✓ | | ✓ | | | | |
| 11 章首结构 | ✓ | ✓ | | ✓ | ✓ | | | |
| 12 小说体综合 | ✓ | ✓ | | ✓ | ✓ | | | |
| 13 多看 fallback 富文本一体 | ✓ | ✓ | ✓ | ✓ | ✓ | | | |
| 14 整页正文竖排 | ✓ | ✓ | | ✓ | | | ✓ | |
| 15 版权 / 题献 / 题记 | ✓ | ✓ | | | ✓ | | | |
| 16 简单数学公式 | ✓ | ✓ | | | | ✓ | | |
| 17 图文九宫格 | ✓ | ✓ | | | | ✓ | | |
| 18 英文小说正文 | ✓ | ✓ | | | ✓ | | | |
| 19 边框与阴影便签 | ✓ | ✓ | | ✓ | | | | |
| 20 章节头图设置 | ✓ | ✓ | | | ✓ | | | |

> 注：03 A-lite 海报不引 `base.css`，因为 `poster.css` 自带骨架；如果海报页里也要排正文段落，再加 `base.css` 即可（终极手册 §六点五 已说明）。

---

## 3. 退化方案：4 文件版（最小拆分）

如果执行模型希望尽量不动现有 OPF / link 结构，可以采用最小拆分：

```text
OEBPS/Styles/
├── fonts.css        — 字体（同 §2.1）
├── base.css         — 正文基础 + 弹注 + 文字效果 + Ruby 行距（吸收 notes + effects）
├── components.css   — 文学结构 + 图文 + 公式 + 前置页 + 竖排（吸收 literary + media + vertical）
└── poster.css       — A-lite 海报（同 §2.1）
```

- 适用：书内场景较少、不会增长太多。
- 缺点：`components.css` 会落到 300 行以上，长期来看仍要进 §2 的 8 文件版。
- 加载顺序：`fonts → base → components → poster`。

> 项目走"demo 先行"工作流时建议直接采用 §2 的 8 文件版，避免日后再迁移；最小拆分仅用于和外部成稿合作时降低改动面。

---

## 4. OPF manifest 与 link 写法

### 4.1 manifest

demo 包采用 8 文件分层后，OPF manifest 应包含以下 8 条 CSS item（**保留** 现有 3 项 + **新增** 5 项）：

```xml
<!-- 现有 3 项（保留） -->
<item id="css-fonts"      href="Styles/fonts.css"      media-type="text/css"/>
<item id="css-base"       href="Styles/base.css"       media-type="text/css"/>
<item id="css-poster"     href="Styles/poster.css"     media-type="text/css"/>

<!-- 新增 5 项 -->
<item id="css-notes"      href="Styles/notes.css"      media-type="text/css"/>
<item id="css-effects"    href="Styles/effects.css"    media-type="text/css"/>
<item id="css-literary"   href="Styles/literary.css"   media-type="text/css"/>
<item id="css-media"      href="Styles/media.css"      media-type="text/css"/>
<item id="css-vertical"   href="Styles/vertical.css"   media-type="text/css"/>
```

- 即使某个文件**未被任何 XHTML 引用**，只要它出现在 `Styles/` 里就必须挂 manifest（EPUBCheck 要求所有打包文件可声明）。
- 反过来，manifest 里**不存在**的 CSS 文件不允许在 XHTML 里 link。
- 真实书可按需裁剪：没有 A-lite 页就不创建 `poster.css`，没有竖排正文就不创建 `vertical.css`；磁盘上不存在的文件不挂 manifest。

### 4.2 每个 XHTML 的 `<head>` link 写法

按 §2.2 顺序、按 §2.3 矩阵选用。例子：

13 号"多看 fallback 富文本一体页"：
```xml
<link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
<link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
<link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>
<link rel="stylesheet" type="text/css" href="../Styles/notes.css"/>
```

03 号 A-lite 海报：
```xml
<link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
<link rel="stylesheet" type="text/css" href="../Styles/poster.css"/>
```

---

## 5. SPEC §7 同步改写清单

`docs/final/SPEC-实现约束.md §7` 当前表只有 3 行（fonts / base / poster），需改写为 8 行版本。给执行模型的替换原文：

```
## 7) CSS 分层约定

| 文件 | 职责 | 允许内容 | 禁止内容 |
|---|---|---|---|
| `fonts.css` | 字体声明 | `@font-face`、字体工具类（默认链 `.book-song` / `.book-hei` / `.book-kai` / `.book-fangsong` / `.book-mono` / `.book-latin-serif` 与对应短别名；嵌入专用类 `.rare` / `.title-special` / `.signature`） | 排版、颜色、分页、布局、元素选择器 |
| `base.css` | 正文基础 | `@page`、`html/body`、`h1–h6`、`p`、`ul/ol/dl`、`table`、`pre/code`、`figure/img`、`a`、`em/strong/q/blockquote`、`ruby/rt/rp` 默认样式、`.has-ruby` 行距兜底 | 弹注 / 文字效果 / 文学结构 / 图文浮动 / 海报 / 竖排类 |
| `notes.css` | 弹注 | `noteref-*`、`footnote-*`、`duokan-footnote-*` 全套 | 字体声明、文字效果、文学结构 |
| `effects.css` | 文字效果 + 便签视觉 | `.emp` / `.wavy` / `.dropcap` / `.note-box` 边框阴影类（含 SVG 花边实验、长条投影） | 字体声明、弹注、文学结构 |
| `literary.css` | 文学结构 + 前置页 | `.dialog` / `.poetry` / `.letter` / `.scene-break` / `.chapter-head` / `.chapter-head-art` / `.chapter-head-banner` / `.epigraph` / `.copyright-page` / `.dedication` / `.epigraph-page` / `.english-fiction` | 弹注、图文浮动、海报、竖排 |
| `media.css` | 图文混排 + 公式 | 图片浮动九宫格、`.figure-grid`、`.math-block` / `.math-inline` | 普通 `figure` / `img` 基础样式 |
| `vertical.css` | 整页正文竖排（非 A-lite） | `body.page-vrl` / `.vrl-section` / `.vrl-title` | 海报规则 |
| `poster.css` | A-lite 海报 | `body.fullpage` / `body.poster-bg` / `.fullframe` / `.poster-title` / `.poster-subtitle` / `.vcol` | 正文段落规则 |

附加规则：
- 加载顺序：`fonts.css → base.css → notes/effects/literary/media/vertical/poster.css`。
- 海报页 XHTML link `fonts.css + poster.css`（如需正文排版再加 `base.css`）。
- 正文页 XHTML 至少 link `fonts.css + base.css`，其他层按场景选用。
- OPF manifest 必须分别声明所有存在于 `Styles/` 的 CSS 文件。
- `html`、普通 `body`、`body.fullpage`、普通标题、图注和引用不设置页面级 `color` / `background` / `background-color`，避免覆盖阅读器夜间模式、护眼模式和用户主题；局部组件可保留必要的边框、阴影和背景装饰。
- 单文件 400 行预警、500 行硬上限；超过 500 行必须按职责拆分或迁入已有正确层。
- 跨层依赖通过类名契约，不允许下层文件引用上层组件类。
```

---

## 6. 落地步骤（给执行模型）

1. 在 `templates/epub-style-demo/OEBPS/Styles/` 下新建 `notes.css`、`effects.css`、`literary.css`、`media.css`、`vertical.css` 五个文件（保留 `fonts.css` / `base.css` / `poster.css`）。
2. 按 §2.1 表格把现 `base.css` 里的相关规则**剪切**到对应文件：
   - 弹注相关 → `notes.css`（含 `sup` / `.noteref-icon` / `a.duokan-footnote` / `.footnote-*`、`.duokan-footnote-*`）；
   - 现 `base.css` 已存在的 footnote 段全部迁出。
3. 删除现 `base.css` 中重复的 `.demo-list / table / pre / .title-page / @media (max-width: 420px)` 块（line 234–414 大约整段是重复的）。
4. 按 [demo-scene-expansion-plan.md §4](./demo-scene-expansion-plan.md)（§4.2 notes.css、§4.3 effects.css、§4.4 literary.css、§4.5 media.css、§4.6 vertical.css）追加 10–17 各页所需视觉类到对应文件。
5. 更新所有 XHTML 的 `<head>`，按 §2.3 矩阵 link 文件。
6. 更新 OPF manifest，新增 5 个 CSS item。
7. 跑 `sh templates/epub-style-demo/build.sh`，校验：
   ```bash
   # 每个 CSS 文件 400 行预警、500 行硬上限
   for f in templates/epub-style-demo/OEBPS/Styles/*.css; do
     lines=$(wc -l < "$f")
     [ "$lines" -gt 400 ] && echo "WARN $f ($lines lines)"
     [ "$lines" -gt 500 ] && echo "OVERSIZE $f ($lines lines)"
   done
   # 每个 XHTML link 的所有 css 都在 manifest 里
   # 每个 manifest 声明的 css 都真实存在于磁盘
   ```
8. 同步 §5 给出的 SPEC §7 改写。

---

## 7. 自检清单

- [ ] `Styles/` 下出现 8 个文件（或最小拆分 4 个）。
- [ ] 任一 CSS 文件 ≤ 500 行（400 行提示规划拆分）。
- [ ] `fonts.css` 内**没有**任何排版规则。
- [ ] `base.css` 内**没有**任何 `.emp / .wavy / .dropcap / .footnote-* / .dialog / .img-left / .math-* / .vrl-* / .poster-*` 之类的组件类。
- [ ] `poster.css` 内**没有**普通段落规则。
- [ ] 每个 XHTML 的 link 顺序符合 `fonts → base → notes/effects/literary/media/vertical/poster`。
- [ ] OPF manifest 包含所有现存 CSS 文件；磁盘上没有"manifest 未声明"的 CSS。
- [ ] SPEC §7 已替换为本文 §5 的 8 行新表。
