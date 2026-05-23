# SPEC-实现约束（执行层）

> 本文件只记录**实现约束**，用于执行层实现与回归测试的约束清单。描述性解释请看《EPUB 3 终极实践手册》。

## 1) 弹注（Footnote Popup）

- `a[epub:type="noteref"]` 与对应 `aside[epub:type="footnote"]` 必须位于**同一 XHTML 文件**。
- 每个章节文件最多一个注释容器：`<aside epub:type="footnote" role="doc-footnote">`。
- `a[epub:type="noteref"]` 必须具有唯一 `id`，供注释回跳定位。
- 多条注释必须使用：`ol.footnote-list > li.footnote-item`。
- 每条注释必须可回跳，默认回跳符号 `◎`（U+25CE）。
- 当需要兼容多看旧版本时，必须在标准结构基础上同步：
  - noteref 锚 `<a>` 增加 `class="duokan-footnote"`，且锚内放注释图标 `<img>`；
  - 注释容器必须使用 `<ol class="footnote-list duokan-footnote-content">`；
  - 每条 `<li class="footnote-item">` 仅挂 `duokan-footnote-item`，禁止在 `<li>` 上重复挂 `duokan-footnote-content`。
- fallback 为次路径，禁止创建第二份注释容器。

## 2) A-lite 页面约束

- 仅允许 reflowable EPUB；v1 不支持 FXL。
- A-lite 页面 CSS 禁用 `position: absolute`。
- A-lite 页面 CSS 禁用 `vh` / `vw` 单位。
- 海报页 `<body>` 必须带 `class="fullpage"`；需要海报背景时必须使用 `class="fullpage poster-bg"`。外层必须是 `<section class="fullframe" epub:type="chapter">`。
- A-lite 根 `html` 必须包含 `width:100%; height:100%; min-height:100%`。
- `body.fullpage` 不允许直接携带 `background-*`；背景必须放在 `body.poster-bg` 或其他 `poster-*` modifier。
- `body.fullpage` 必须包含 `-webkit-text-size-adjust:100%; text-size-adjust:100%`。
- A-lite 页 `html` / `body.fullpage` / `.fullframe` 必须使用 `box-sizing:border-box`，避免 `width:100%` 叠加内外补白后被阅读器裁切。
- `.fullframe` 必须保持 `padding: 0; overflow: visible`；页面留白由内部文字/图形元素的 `margin` 控制，不给整页骨架加 padding。
- A-lite 推荐类白名单：`fullpage` / `poster-bg` / `fullframe` / `poster-title` / `poster-subtitle` / `vcol`。
- 所有可见叠加文本必须为真实文本节点；不允许将全部可见文字仅以图片承载。

## 3) 字体与 OPF

- 无论是否启用书内字体嵌入，OPF `<package>` 都必须在 `prefix` 声明 ibooks 命名空间，并保留：`<meta property="ibooks:specified-fonts">true</meta>`。
- 标题字体来源仅允许：书内嵌入字体 + 通用族回退（serif/sans-serif/monospace 等）。
- 字体策略必须与 `fontspec` 三态一致：`auto | forceAll | none`。

## 4) 子集策略算法（执行层对齐）

`auto` 模式下，子集字符集合 =
1. 全书 XHTML 实际用字
2. 角色映射要求字符（body / heading / quote / rare）
3. 用户 `extraCodepoints`
4. 实现显式声明的额外字符；默认回跳符号 `◎` 走阅读器或系统 fallback，不强制进入各角色字体子集

附加规则：
- 当角色字体本身即为人工子集（rare 专用字库），可按角色策略显式 `none`，避免重复裁切。

## 5) 结构化产物要求

- 输出包必须满足 EPUB `mimetype` 首条且 STORED（无压缩）规则。
- OPF 元数据、manifest/spine 的排序与稳定性必须可复现（便于 golden fixture diff）。
- EPUB 必须声明封面图：manifest 中封面图片 `<item>` 必须带 `properties="cover-image"`，并同步提供 `<meta name="cover" content="..."/>` 兼容 Kindle Previewer。
- 封面图优先使用 JPEG/PNG 等 raster 资源；SVG 可作为正文或海报资源，但不作为 Kindle 兼容封面主声明。
- EPUB 3 必须提供 `nav.xhtml`；需要 Kindle/旧工具链兼容的 demo 或交付包必须同时提供 `toc.ncx`，并在 OPF spine 写 `toc="ncx"`。
- 生成物应回写构建元数据：子集器名称/版本、字形统计、构建时间。

## 5.1) 图片格式兼容

- 书内图片主路径使用 JPEG / PNG。照片、插画优先 JPEG；线稿、截图、图表、注释图标优先 PNG。
- WebP 不进入 Kindle 主路径。2026-05-21 Kindle conversion log 已记录 WebP 样本触发 W14012 / W14015，文件被判定为不支持或无效。
- SVG 只能作为现代 EPUB 增强或源文件保留；面向 Kindle 的生产包如发现空白、变形、转换慢或字体依赖，必须预先栅格化为 JPEG / PNG。
- 面向 Kindle 的图片产物必须使用 sRGB JPEG / PNG，并避免透明、CMYK、TIFF、多帧 GIF 等不稳定输入。

## 5.5) 正文页盒模型

- 普通可重排正文页的 `body` 不允许同时使用 `width:100%` 与左右 `padding`；正文页应保持 auto 宽度，让 padding 计入可用行宽，避免阅读器右侧裁切。
- 正文页如需页面留白，优先使用 `body { margin:0; padding:... }`，并显式设置 `box-sizing:border-box` / `-webkit-box-sizing:border-box`。

## 5.6) 图片环绕兼容

- 图文环绕的通用路径是 `<figure class="img-left|img-right">` 包裹 `<img>` 与可选 `<figcaption>`，把 `float:left/right` 与百分比 `width` 挂在 `figure` 上。
- figure 宽度使用百分比，推荐先落在 `25%–35%`，demo 默认 `30%`；不得使用 `em` 宽度做 Kindle 主路径，避免字号变化改变绕排阈值。
- `25%–35%` 不是标准常量，而是当前 Kindle App / Readest 反馈下的保守起点：图片过宽会压缩剩余文本列，图片过窄会影响可读性，正式书稿必须按目标阅读器、屏幕和字号实测微调。
- 内层图片必须使用 `width:100%; height:auto;`。不要固定高度，也不要依赖 `aspect-ratio` 作为主路径；真实图片让 `height:auto` 保持天然宽高比。
- 环绕样例必须提供足够长的前后正文；短段落无法证明 float 失败，只能作为阈值反例。
- 不使用 direct `img` 直挂 float 作为主路径，避免部分阅读器图片显示过小。

## 5.7) 文字装饰兼容

- 带样式的下划线必须拆成多条声明：先写 `text-decoration: underline;`，再写 `text-decoration-style: wavy;` 等增强属性。
- Kindle App 已实测能显示基础 underline fallback，但不会显示 wavy；这属于预期降级，不再视为样式丢失。

## 5.8) MathML

- 含 MathML 的 XHTML manifest item 必须声明 `properties="mathml"`。
- Kindle 路径只把 MathML 视为 Enhanced Typesetting 能力；目标平台不支持时必须准备文本公式或图片公式 fallback。
- demo 优先覆盖 KDP 支持列表内标签组合，不引入未确认支持的私有数学标签。

## 5.9) 英文小说正文

- 英文书页必须在 HTML 或 `body` 上声明 `xml:lang="en"` / `lang="en"`，让 Apple Books、Readest、Kindle 和其他阅读器有机会启用正确断字和朗读规则。
- 英文正文使用短 serif 链，推荐起点为 `Georgia, "Times New Roman", "Noto Serif", serif`；不要沿用中文 `Songti/SimSun/Noto Serif CJK` 链。
- 简单英文小说的主路径是首段无缩进、后续段落 `1.2em–1.5em` 首行缩进，段间距接近 0；不要同时使用大段间距和大缩进。
- 英文正文不强制 `justify`。未实测 hyphenation 的 Kindle/Readest/Apple Books 路径优先 `text-align:left`；确认断字稳定后才使用 `justify`。
- 章首插图和正文插图默认使用居中 `figure`，宽度用 `max-width` 约束，不固定页高，不把插图做成固定版式页面。
- 首字装饰优先用 `::first-letter`，保持正文单词完整；旧式 span 首字和浮动 drop cap 只作为增强，并必须在朗读、复制文本、大字号和窄屏下复测。

## 5.10) 边框、阴影与便签文本框

- 便签、提示、摘录和资料卡必须保留真实文本；禁止把文字直接烘焙进图片来实现纸张效果。
- 主路径使用 `border`、`border-left`、`background`、`padding`、`margin` 和 `page-break-inside: avoid`。这些属性在 Readest、Apple Books、Kindle Previewer 和旧 WebKit 路径中更稳。
- `box-shadow`、`inset box-shadow`、`outline-offset`、不对称 `border-radius` 只能作为渐进增强；阅读器忽略时不得影响阅读。
- 通用 EPUB 不使用 `transform: rotate()` 旋转便签文本框。Kindle Previewer 3.104（2026-05-23 实测）会在 KFX 增强排版转换中触发内部错误；若某个非 Kindle 发行目标确实需要旋转效果，必须放在该目标专用版本并单独验证。
- 不依赖 `clip-path`、复杂滤镜、CSS mask 或多层伪元素承载关键信息；它们在 EPUB 阅读器中支持不稳定。
- 长文本便签不要追求倾斜效果。需要贴纸感时优先用不对称边框、圆角和投影模拟，避免窄屏下产生裁切或左右溢出。
- 内联 SVG 花边只作为实验验证项，不作为通用推荐边框。若强设计需求必须使用，SVG 只能承载装饰边线并加 `aria-hidden="true"`，正文仍是 HTML 真文本；生产版必须能降级为双线框、左侧竖线框或普通边框。

## 6) Fixture 命名索引（M5 对齐）

- `01-basic-cjk`
- `02-footnotes`
- `03-fontspec-no-subset`
- `04-fontspec-subset`
- `05-vertical-cjk`

> 注：本索引用于 M5 fixture 命名；与 `templates/epub-style-demo/` 的 8 页样本是独立集合。


## 7) CSS 分层约定

| 文件 | 职责 | 允许内容 | 禁止内容 |
|---|---|---|---|
| `fonts.css` | 字体声明 | `@font-face`、字体工具类（默认链 `.book-song` / `.book-hei` / `.book-kai` / `.book-fangsong` / `.book-mono` / `.book-latin-serif` 与对应短别名；嵌入专用类 `.rare` / `.title-special` / `.signature`） | 排版、颜色、分页、布局、元素选择器 |
| `base.css` | 正文基础 | `@page`、`html/body`、`h1–h6`、`p`、`ul/ol/dl`、`table`、`pre/code`、`figure/img`、`a`、`em/strong/q/blockquote`、`ruby/rt/rp` 默认样式 | 弹注 / 文字效果 / 文学结构 / 图文浮动 / 海报 / 竖排类 |
| `notes.css` | 弹注 | `noteref-*`、`footnote-*`、`duokan-footnote-*` 全套 | 字体声明、文字效果、文学结构 |
| `effects.css` | 文字效果 + 便签视觉 | `.emp` / `.wavy` / `.dropcap` / `.scene-break` / Ruby 行距 / `.note-box` 边框阴影类 | 字体声明、弹注、文学结构 |
| `literary.css` | 文学结构 + 前置页 | `.dialog` / `.poetry` / `.letter` / `.chapter-head` / `.epigraph` / `.copyright-page` / `.dedication` / `.epigraph-page` / `.english-fiction` | 弹注、图文浮动、海报、竖排 |
| `media.css` | 图文混排 + 公式 | 图片浮动九宫格、`.figure-grid`、`.math-block` / `.math-inline` | 普通 `figure` / `img` 基础样式 |
| `vertical.css` | 整页正文竖排（非 A-lite） | `body.page-vrl` / `.vrl-section` / `.vrl-title` | 海报规则 |
| `poster.css` | A-lite 海报 | `body.fullpage` / `body.poster-bg` / `.fullframe` / `.poster-title` / `.poster-subtitle` / `.vcol` | 正文段落规则 |

附加规则：
- 加载顺序：`fonts.css → base.css → notes/effects/literary/media/vertical/poster.css`。
- 海报页 XHTML link `fonts.css + poster.css`（如需正文排版再加 `base.css`）。
- 正文页 XHTML 至少 link `fonts.css + base.css`，其他层按场景选用。
- OPF manifest 必须分别声明所有存在于 `Styles/` 的 CSS 文件。
- 单文件不超过 200 行；超过即拆分。
- 跨层依赖通过类名契约，不允许下层文件引用上层组件类。

## 8) 字体链规则

- 同一份 EPUB 默认走跨平台系统字体链，不嵌入字体；嵌入字体仅用于
  (a) 大量生僻字、(b) 设计上必须的特定字体、(c) (a) 与 (b) 同时存在。
- **默认 `font-family` 链 ≤ 4 段**：1 个 Apple 系统字体 → 1 个 Windows 系统字体
  → 1 个 Android / 跨平台开源 CJK 字体 → generic family（serif/sans-serif/monospace）。
- 嵌入字体不允许出现在默认 `body` / `h*` / `code` 等元素选择器链中，必须挂在
  专用类（`.rare` / `.title-special` / `.book-song-deluxe` 等）上。
- 例外（模式 C1-body）：当正文确实含生僻字、且选择嵌入"全字符集"字体（非子集）时，
  允许把该嵌入字体按模式 C1 直接挂在 `body` / `h*` 链上，走单一字体链以保持设计统一。
  启用要求：
  - 嵌入字体必须覆盖书内所有生僻字（至少 GB 18030 / CJK Unified Ideographs + Ext-A，
    按书内实际用字裁切但不再做子集压缩）；子集字库（如 `RareSongSubset`）禁止走本路径，
    必须改用模式 B `.rare` 类；
  - OPF manifest 声明对应字体 item；
  - `fontspec` 切到 `forceAll`（参考 fonts-css-expansion-plan §5）；
  - body / h* 链仍 ≤ 5 段，嵌入字体在第 1 位且只出现 1 次，其后 3 段系统字体，
    链尾 generic family；
  - 示例：`body { font-family: "BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`。
- 专用类按场景使用以下三种模式：
  - **模式 A 设计字体专用**（题签 / 卷头题字 / 签名档）：链 ≤ 2 段，嵌入字体 + generic family；
  - **模式 B 生僻字子集专用**（`.rare` 类）：链 ≤ 2 段，嵌入字体 + generic family；
  - **模式 C 嵌入 + 系统字体复合**：**链 ≤ 5 段**，嵌入字体在链里**只出现 1 次**，位置为第 1 位（C1 设计前置）或倒数第 2 位（C2 嵌入兜底），二选一；中间为 3 段系统字体（Apple + Windows + Android / 开源 CJK），链尾 generic family。
    - C1 示例：`"BookSongDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", serif`
    - C2 示例：`"Songti SC", "SimSun", "Noto Serif CJK SC", "RareSongSubset", serif`
- 同一条链里嵌入字体出现 ≥ 2 次属反模式；若需"设计字形 + 生僻字兜底"双重支援，应拆成两个类（C1 类挂在正文 / 章节，模式 B `.rare` 类用 span 包住生僻字），不要塞进同一条链。
- "一平台一字体名" 允许：Apple `Songti SC` + Windows `SimSun` + Android `Noto Serif CJK SC` 是跨平台覆盖，不算堆叠。
- 不在同一条链里堆叠**同一平台的多个别名**（如 `Songti SC` + `STSongti-SC-Regular`，或 `SimSun` + `宋体`，或 `Microsoft YaHei` + `微软雅黑`，或 `Noto Serif CJK SC` + `Source Han Serif SC`）；只保留各平台最常用的英文名。
- 没有专用类引用的 `@font-face` 必须从 `fonts.css` 删除或保持注释；OPF 不挂对应字体 item。
- `<meta property="ibooks:specified-fonts">true</meta>` 作为通用预防默认始终保留，与是否嵌入字体无关——未嵌字体时它表示"用我指定的系统字体链"，避免 Apple Books 里用户的第三方字体覆盖书内排版。
