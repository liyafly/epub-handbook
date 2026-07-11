# SPEC-实现约束（执行层）

> 版本：2026-06-03

> 本文件只记录**实现约束**，用于执行层实现与回归测试的约束清单。描述性解释请看《EPUB 3 终极实践手册》。

## 1) 弹注（Footnote Popup）

- 任何使用 `epub:type` 的 XHTML 根 `<html>` 必须声明：`xmlns:epub="http://www.idpf.org/2007/ops"`。
- `a[epub:type="noteref"]` 与对应 `aside[epub:type="footnote"]` 必须位于**同一 XHTML 文件**。
- 每个章节文件最多一个注释容器：`<aside epub:type="footnote" role="doc-footnote">`。
- `a[epub:type="noteref"]` 必须具有唯一 `id`，供注释回跳定位。
- noteref 的可见触发主路径是图片图标。已有本地图标资源时必须保留原 `img src`，不得无差别替换为项目默认图标；只有纯文本、数字上标、`[1]`、`注` 等无图片标记转换时，才补入默认 `Images/note.png`。
- 图片触发器可以保留外层 `<sup>` 作为旧书兼容包裹，但 CSS 必须把它处理为普通行内图标，不做高位数字上标效果；`sup { vertical-align: middle; line-height: 1; }`，图标自身使用 `vertical-align: baseline` 或 `middle`。
- 多条注释必须使用：`ol.footnote-list > li.footnote-item`。
- 每条注释必须可回跳，默认回跳符号 `◎`（U+25CE）。
- 当需要兼容多看旧版本时，必须在标准结构基础上同步：
  - noteref 锚 `<a>` 增加 `class="duokan-footnote"`，且锚内放注释图标 `<img>`；
  - 注释容器必须使用 `<ol class="footnote-list duokan-footnote-content">`；
  - 每条 `<li class="footnote-item">` 仅挂 `duokan-footnote-item`，禁止在 `<li>` 上重复挂 `duokan-footnote-content`。
- fallback 为次路径，禁止创建第二份注释容器。
- 本节默认约束 EPUB3 主包。OPF `version="2.0"` 外壳中叠加 `xmlns:epub`、`epub:type` 或 `<aside>` 只能视为按目标阅读器验证的兼容模式，不得标为严格 EPUB2 标准路径；实操见 `docs/how-to/epub2-popup-note-compatibility.md`。

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
- A-lite 推荐类白名单：`fullpage` / `poster-bg` / `poster-bg-contain` / `poster-bg-volume-*` / `fullframe` / `poster-fallback` / `poster-title` / `poster-subtitle` / `vcol`。
- 从既有 EPUB 继承的单图卷封页，如果源文件没有独立文本节点，必须保留原图 `<img class="poster-fallback">`，并可把同一资源镜像为 `poster-bg-contain` 或 `poster-bg-volume-*` 背景；背景必须使用 `background-size:contain`，禁止改用会裁图的 `cover` 或会拉伸图片的 `100% 100%`。
- 单图卷封页可用 `@supports (background-size: contain)` 在支持背景图的阅读器中隐藏 `.poster-fallback`；不支持背景图时必须回退到原始 `<img>`，不得出现空白页。
- 源文件中已有的可见叠加文本必须保留为真实文本节点；不允许把可编辑文本重新栅格化到图片中。

## 3) 字体与 OPF

- `<meta property="ibooks:specified-fonts">true</meta>` 仅当正文字体锁定时添加；新建锁定版的入口是 `fonts.css` 中直接的 `body { font-family:... }`。添加时 OPF `<package>` 必须同步在 `prefix` 声明 ibooks 命名空间。自由模式（默认）不需要。只在局部角色使用嵌入字体不需要此 meta。既有 EPUB 的 `body-font-locked` class 可兼容保留，但不作为新模板入口。判定规则见 §8。
- 标题字体可用系统链或书内嵌入字体 + 通用族回退；当 `h1` / `h2` 在全书各自只有一个字体角色时可直接绑定，否则使用角色类。
- 字体策略必须与 `fontspec` 三态一致：`auto | forceAll | none`。

## 4) 子集策略算法（执行层对齐）

`auto` 模式下，先按 CSS 继承、局部覆盖和字体链建立角色字符清单，不得只扫描普通 `p`：

1. C1-body 或其他要求嵌入字体完整覆盖的锁定角色：纳入全书 XHTML 中最终解析到该角色的全部可渲染文字和标点
2. 局部设计 / 补字角色：纳入明确由嵌入字体承担的字符；其余字符只有在该局部角色声明了可靠 fallback 时才可排除
3. `quotes`、`content` 等 CSS 生成且最终由该角色渲染的字符；现有覆盖分析器不会从 XHTML 自动采集，执行层须另行枚举
4. 角色映射显式要求字符（body / heading / quote / rare）
5. 用户 `extraCodepoints` 与实现显式声明的额外字符

附加规则：
- 当角色字体本身即为人工子集（rare 专用字库），可按角色策略显式 `none`，避免重复裁切。
- 默认回跳符号 `◎` 若继承 C1-body、嵌入注释字体或其他要求完整覆盖的角色，必须进入该角色子集；只有为它声明了独立的非嵌入 fallback 角色时，才可从对应嵌入字体清单排除。
- 子集化、字体改名和文件扩展名改写不得改变 XHTML 正文码位。视觉相似字符（例如 `〇` U+3007 与 `○` U+25CB）不得按字形自动互换；这类变化属于正文校订，必须走 §10.1.1。
- 子集写出后必须重新核对角色字符清单与字体 `cmap`；“子集命令成功”不等于该角色全部实际用字已覆盖。

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

## 5.6.1) 文白对照左右兼容

- 文白对照、原文/译文对照的基础结构必须按源序保留真实文本：标题、出处、原文、译文、回目录锚点都不可图片化。
- 主路径结构应接近大部头文白书：一组 `.parallel-pair` 直接包含原文段落和译文段落。基础状态必须上下；短组可用 `.parallel-float-pair`，但双侧 `float:left/right` 只能放在宽屏 `@media (min-width: 40em)` 增强里。默认文白起点为 `38%/58%`；原译篇幅接近时可加 `.parallel-ratio-balanced` 使用 `48%/48%`；原文较长时可加 `.parallel-ratio-source-wide` 使用 `58%/38%`。单书可在后加载 CSS 中自定义比例，但两栏总和建议不超过 `96%`，给 gutter 留至少 `4%`。Kindle 电子墨水、小屏、大字号、长组、大字号探针或已知风险组应保持上下。
- 不依赖 `overflow:hidden` BFC 形成右侧列；KF8/KFX 对这条路径不稳定。不要为了标签和列容器把每组正文包得过重，除非确有多段注记需求。
- 分页保护只能用于短组增强类：`.parallel-float-pair { page-break-inside: avoid; -webkit-page-break-inside: avoid; break-inside: avoid; }`；`.parallel-pair` 默认允许长对照自然切页，`.parallel-stack-pair` / `.parallel-pair-allow-break` 必须显式允许切页，避免 Kindle 产生大空白页。
- 不使用 `table`、`display:flex`、`grid`、absolute positioning 或固定版式承载正文对照。阅读器忽略 float、屏幕过窄或大字号列宽不足时，必须退回源序上下显示。
- 不引入 `amzn-kf8` / `amzn-mobi` 媒体查询；不要把左右对照只写在 `@media (orientation: landscape)` 内。Kindle Previewer / KFX 对 flex 与 orientation 组合不应作为主路径。
- 不强求任何 Kindle 窄屏 pair 左右对照；长段、字号探针或已知风险 pair 用 `.parallel-stack-pair` 保持上下。
- Kindle 用户字体覆盖后宋楷差异可能消失，原文/白话至少要靠段落顺序、出处和上下文独立可读；生产书可在条目级补充说明，但不要在每个短 pair 里堆标签。
- Kindle 专用 AZW3/MOBI 成品中可见 `table-layout: fixed` 左右对照先例，但通用 EPUB / KDP 源文件不把 table 当正文对照主路径；只有明确只交付 Kindle 成品格式并逐设备验收时，才作为专用例外。

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
- 首字装饰优先用 `::first-letter`，保持正文单词完整；旧式 span 首字和浮动 drop cap 只作为增强，并必须在朗读、复制文本、大字号和窄屏下复测。下沉首字若需要特殊字体，生产 EPUB 使用授权嵌入字体并声明 OPF font item；demo 可用系统手写体链代替。

## 5.10) 边框、阴影与便签文本框

- 便签、提示、摘录和资料卡必须保留真实文本；禁止把文字直接烘焙进图片来实现纸张效果。
- 主路径使用 `border`、`border-left`、`background`、`padding`、`margin` 和 `page-break-inside: avoid`。这些属性在 Readest、Apple Books、Kindle Previewer 和旧 WebKit 路径中更稳。
- `box-shadow`、`inset box-shadow`、`outline-offset`、不对称 `border-radius` 只能作为渐进增强；阅读器忽略时不得影响阅读。
- 通用 EPUB 不使用 `transform: rotate()` 旋转便签文本框。Kindle Previewer 3.104（2026-05-23 实测）会在 KFX 增强排版转换中触发内部错误；若某个非 Kindle 发行目标确实需要旋转效果，必须放在该目标专用版本并单独验证。
- 不依赖 `clip-path`、复杂滤镜、CSS mask 或多层伪元素承载关键信息；它们在 EPUB 阅读器中支持不稳定。
- 长文本便签不要追求倾斜效果。需要贴纸感时优先用不对称边框、圆角和投影模拟，避免窄屏下产生裁切或左右溢出。
- 内联 SVG 花边只作为实验验证项，不作为通用推荐边框。若强设计需求必须使用，SVG 只能承载装饰边线并加 `aria-hidden="true"`，正文仍是 HTML 真文本；生产版必须能降级为双线框、左侧竖线框或普通边框。

## 5.11) 章节头图

- 普通可重排章节头图属于章首结构，写入 `literary.css`，不归入正文图文混排的 `media.css`。
- 头图必须是装饰或栏目识别；章节标题、kicker、副标题必须保留为真实 XHTML 文本。
- 小型章标使用保守百分比宽度并加 `max-width`：约 `35%`；空间充足且已复测时，可用增强类到约 `40%`。
- 满栏横幅头图可使用 `width:100%; max-width:100%`，只铺满正文内容栏，不要求突破阅读器页边距。
- 内层图片使用 `height:auto`；横幅高度由源图宽高比控制。不固定高度，不使用 `vh/vw`、absolute positioning 或大段顶部空白模拟固定页。
- 如果章首需要强视觉首屏、背景图或大面积叠字，应改走 A-lite 海报方案；不要把普通章节头图扩大成伪固定版式。

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
| `fonts.css` | 字体声明与角色绑定 | `@font-face`、只含字体声明的角色选择器（`body` / `h1` / `body.preface` / 注释角色等）、字体工具类（`.font-st` / `.font-ht` / `.font-kt` / `.font-fs` / `.font-mono` / `.font-en-serif`；局部类 `.rare` / `.title-tszt` / `.signature-tszt`） | 非字体排版、颜色、分页、布局；角色混杂的裸 `p` / `div` / `span` 选择器 |
| `base.css` | 正文基础 | `@page`、`html/body`、`h1–h6`、`p`、`ul/ol/dl`、`table`、`pre/code`、`figure/img`、`a`、`em/strong/q/blockquote`、`ruby/rt/rp` 默认样式、`.has-ruby` 行距兜底 | 弹注 / 文字效果 / 文学结构 / 图文浮动 / 海报 / 竖排类 |
| `notes.css` | 弹注 | `noteref-*`、`footnote-*`、`duokan-footnote-*` 全套 | 字体声明、文字效果、文学结构 |
| `effects.css` | 文字效果 + 便签视觉 | `.emp` / `.wavy` / `.dropcap` / `.note-box` 边框阴影类 | 字体声明、弹注、文学结构 |
| `literary.css` | 文学结构 + 前置页 | `.dialog` / `.poetry` / `.letter` / `.scene-break` / `.chapter-head` / `.chapter-head-art` / `.chapter-head-banner` / `.chapter-header` / `.epigraph` / `.copyright-page` / `.dedication` / `.epigraph-page` / `.english-fiction` / `.classical-modern` / `.parallel-entry` / `.parallel-pair` / `.parallel-float-pair` / `.parallel-stack-pair` | 弹注、普通图文浮动、海报、竖排 |
| `media.css` | 图文混排 + 公式 | 图片浮动九宫格、`.figure-grid`、`.math-block` / `.math-inline` | 普通 `figure` / `img` 基础样式 |
| `vertical.css` | 整页正文竖排（非 A-lite） | `body.page-vrl` / `.vrl-section` / `.vrl-title` | 海报规则 |
| `poster.css` | A-lite 海报 | `body.fullpage` / `body.poster-bg` / `.fullframe` / `.poster-title` / `.poster-subtitle` / `.vcol` | 正文段落规则 |

附加规则：
- 加载顺序：`fonts.css → base.css → notes/effects/literary/media/vertical/poster.css`。
- 海报页 XHTML link `fonts.css + poster.css`（如需正文排版再加 `base.css`）。
- 正文页 XHTML 至少 link `fonts.css + base.css`，其他层按场景选用。
- 若 `.note-box` 容器视觉继续增长并让 `effects.css` 超过 400 行，优先拆出 `decoration.css` 承载便签/资料卡边框阴影；`effects.css` 保留文字效果。超过 500 行必须拆分。
- OPF manifest 必须分别声明所有存在于 `Styles/` 的 CSS 文件。
- `html`、普通 `body`、`body.fullpage`、普通标题、图注和引用不设置页面级 `color` / `background` / `background-color`，避免覆盖阅读器夜间模式、护眼模式和用户主题；局部组件可保留必要的边框、阴影和背景装饰。
- 单文件 400 行预警、500 行硬上限；超过 500 行必须按职责拆分或迁入已有正确层。
- 跨层依赖通过类名契约，不允许下层文件引用上层组件类。

## 8) 字体链规则

> **设计意图**：正文默认自由，显式字体角色优先使用短系统链，嵌入字体仅作特定需求专用。原因：(1) 嵌入字体增加包体；(2) 阅读器与系统字体已覆盖多数常用 CJK 字符；(3) 默认正文不由书内 `body` 强制指定字体，给支持用户字体选择的阅读器保留切换空间。只有系统字体确实缺字，或设计上必须固定正文、标题、题签等角色字形时才嵌入。

- 同一份 EPUB 的普通正文默认不声明 `font-family`；需要显式字体的局部角色优先走短跨平台系统链。嵌入字体仅用于
  (a) 大量生僻字、(b) 设计上必须的特定字体、(c) (a) 与 (b) 同时存在。
- **正文字体分两种模式**，由全书生效的 `body` 是否有 `font-family` 区分：
  - **自由模式（默认）**：`body` 和普通正文 `p` 都不设 `font-family`，正文交给阅读器默认字体；在支持用户字体选择的阅读器中保留切换空间。标题、注释、题签等局部角色仍可单独绑定字体。
  - **锁定模式**：在 `fonts.css` 直接写 `body { font-family:... }`，普通正文 `p` 通过继承获得该字体链；OPF 必须同步加 `<meta property="ibooks:specified-fonts">true</meta>`。
  - `base.css` 不承担字体绑定；自由模式的 `fonts.css` 不写直接 `body` 字体规则，锁定模式的角色绑定写在 `fonts.css`。
- 裸 `p { font-family:... }` **不作为全书锁定入口**：它只覆盖段落，不能覆盖列表、引用、表格等其他正文容器，还会误伤同为 `p` 的注释。只需锁定某类正文段落时，应使用 `.bodytext`、`.prose` 等明确角色类。
- **角色选择器规则**：结构元素在全书只有一个稳定字体角色时，可直接使用 `body`、`h1`、`h2`、`body.preface`、`aside[epub|type~="footnote"]` 等选择器；同一元素承担多种角色、只在少数位置换字形或需要例外时，必须改用明确角色类。使用 `epub|type` 这类命名空间选择器的 CSS 必须声明 `@namespace epub "http://www.idpf.org/2007/ops";`；它紧跟可选的 `@charset` / `@import`，并早于 `@font-face` 和普通规则。`fonts.css` 中这些规则只写字体属性，不混入布局、颜色和分页。
- **`font-family` 链 ≤ 4 段**：1 个 Apple 系统字体 → 1 个 Windows 系统字体
  → 1 个 Android / 跨平台开源 CJK 字体 → generic family（serif/sans-serif/monospace）。
- 嵌入字体可直接绑定到稳定的全书角色：例如正文 `body`、统一层级标题 `h1/h2`、前言 `body.preface` 或注释容器。C1-body 必须由嵌入字体覆盖该角色经过继承与局部覆盖后真正承担的全部实际用字、标点和 CSS 生成字符；局部角色可以使用明确的 fallback，但必须单独枚举嵌入字体承担的字符并验证剩余字符确实落到该 fallback。CSS URL、OPF manifest 与字体文件必须闭环。注释字体绑定写在 `fonts.css`，`notes.css` 只保留注释结构与视觉。不要把嵌入字体直接挂到角色混杂的裸 `p` / `div` / `span`。
- 生僻字子集（如 `tszt-rare`）禁止挂到 `body` / `h*`，必须使用模式 B `.rare` 包住实际字符。
- 模式 C1-body 用于设计上需要锁定正文，或正文含生僻字且正文角色字体覆盖全部实际用字的场景。启用要求：
  - 嵌入字体必须覆盖最终解析到正文角色的全部字符；可以是完整字体，也可以是按正文角色全部实际用字生成的完整角色子集；局部补字子集（如 `tszt-rare`）禁止走本路径，必须改用模式 B `.rare` 类；
  - OPF manifest 声明对应字体 item；
  - `fontspec` 切到 `forceAll`；
  - body / h* 链仍 ≤ 5 段，嵌入字体在第 1 位且只出现 1 次，其后 3 段系统字体，
    链尾 generic family；
  - 示例：`body { font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`。
- 专用类按场景使用以下三种模式：
  - **模式 A 设计字体专用**（题签 / 卷头题字 / 签名档）：链 ≤ 2 段，嵌入字体 + generic family；
  - **模式 B 生僻字子集专用**（`.rare` 类）：链 ≤ 2 段，嵌入字体 + generic family；
  - **模式 C 嵌入 + 系统字体复合**：**链 ≤ 5 段**，嵌入字体在链里**只出现 1 次**，位置为第 1 位（C1 设计前置）或倒数第 2 位（C2 嵌入兜底），二选一；中间为 3 段系统字体（Apple + Windows + Android / 开源 CJK），链尾 generic family。
    - C1 示例：`"st-design", "Songti SC", "SimSun", "Noto Serif CJK SC", serif`
    - C2 示例：`"Songti SC", "SimSun", "Noto Serif CJK SC", "tszt-rare", serif`
- 同一条链里嵌入字体出现 ≥ 2 次属反模式；若需"设计字形 + 生僻字兜底"双重支援，应拆成两个类（C1 类挂在正文 / 章节，模式 B `.rare` 类用 span 包住生僻字），不要塞进同一条链。
- "一平台一字体名" 允许：Apple `Songti SC` + Windows `SimSun` + Android `Noto Serif CJK SC` 是跨平台覆盖，不算堆叠。
- 不在同一条链里堆叠**同一平台的多个别名**（如 `Songti SC` + `STSongti-SC-Regular`，或 `SimSun` + `宋体`，或 `Microsoft YaHei` + `微软雅黑`，或 `Noto Serif CJK SC` + `Source Han Serif SC`）；只保留各平台最常用的英文名。
- 没有任何角色选择器或角色类引用的 `@font-face` 必须从 `fonts.css` 删除或保持注释；OPF 不挂对应字体 item。
- `<meta property="ibooks:specified-fonts">true</meta>` 仅当正文字体锁定时添加；自由模式下不设置此 meta，局部角色的 `@font-face` 不单独触发此 meta。这是本仓当前可审计的包结构 policy；Apple Books 各版本中的字体切换与局部嵌入字体表现仍列在 `reader-matrix.yaml` 的 `07-font-family-order` 待复测项，不冒充实测结论。
- 既有 EPUB 若因历史兼容或用户明确要求在正文自由模式保留该 meta，必须在书级报告记录理由，并用 `epub_lint.py --allow-free-body-ibooks-meta` 显式豁免；该例外不得写回 starter、preset 或新书模板。
- 正文字体模式是**全书级决策**：同一本新建生产书要么全书自由（`body` 与普通正文 `p` 都无字体规则，OPF 无此 meta），要么全书锁定（直接 `body` 规则，OPF 加一份 meta）。不按页混用；既有书的 `body-font-locked` class 只作为兼容输入保留，不向新模板扩散。
- 同一本书同时交付正文自由版与锁定版时，两版必须从同一个已定稿内容基线派生。允许差异只限字体 CSS、字体资源、OPF 中与字体有关的 manifest/meta（含 `ibooks:specified-fonts`），以及该 meta 所需的 `<package prefix>` 中 `ibooks:` 声明；另允许每个 rendition 唯一的 `dcterms:modified` 反映各自实际打包时间。双版本比较可忽略该字段的值，但不得忽略缺失、多份或非 UTC `YYYY-MM-DDThh:mm:ssZ`。同一批成对构建应优先只取一次 `BUILD_TIMESTAMP` / `SOURCE_DATE_EPOCH` 传给两版，以减少无意义 diff；分步打包时的合法时间差不视为内容漂移。核心 `dc:*` metadata、spine、XHTML、注释、图片和其他资源必须一致。任何超出此白名单的差异都要另行授权并记录。
- 新建字体 alias、文件名和 class 必须遵循 [字体别名命名规范](字体别名命名规范.md)：使用 `en` / `st` / `kt` / `fs` / `ht` / `tszt-*` 等角色缩写；不得新增 `Book*`、`RareSong*` 或 `.book-*` 字体命名。

## §10 AI 清洗已有 EPUB 的改动边界

> 本节给 AI 协作代理使用：当输入是一本已存在的 EPUB（而不是从零构造）时，AI 的改动必须落在本节边界内。
> 任何破坏本节约束的改动都视为事故，需要回滚。

### §10.1 红线（默认不可改；显式授权才可转入专门分支）

AI 检测到自己将要触发以下任一改动时，必须停止并询问用户：

| 红线 | 说明 | 校验方式（自动） |
| --- | --- | --- |
| 正文文本 | 去除标签后的纯文本不允许变化；标点、错别字、通假字一律不动 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check text` |
| `dc:title` / `dc:creator` / `dc:identifier` / `dc:language` | OPF 核心元数据 | `--check metadata` |
| spine 阅读顺序 | `<itemref>` 序列不可重排 | `--check spine` |
| 章节锚点 id | 影响第三方书签、旧链接、阅读器进度 | `--check anchors` |
| 带 `properties="cover-image"` 的封面资源 | 不擅自压缩、转格式、裁切 | `--check cover` |
| DRM 相关 | 发现 `META-INF/encryption.xml` 或文件无法解压：立即拒绝 | `--check drm` |

红线触发后的处理：

1. AI 在输出里明确列出将要触发的红线条目。
2. 让用户决定：放弃、显式授权、或调整范围。
3. 默认行为不得是自动通过。

### §10.1.1 授权正文校订分支

只有用户明确授权修改正文、标点或字词时，才允许进入本分支。授权不会把普通正文不变 gate 变成“通过”，也不得用宽泛 allow-list 隐藏实际文字差异。

1. 将现版与参考版作为只读输入，记录 EPUB SHA-256、篇章到 XHTML 的映射和参考版本；参考版只提供候选文字，不自动成为正确答案。
2. 在生成差异前写清提取范围：篇名、小标题、正文、篇末日期、注释入口、注释正文、图片、图注，以及 nav.xhtml / toc.ncx 中的导航标签分别包含或排除哪些。所有被排除结构必须另做签名或红线校验；若授权修改篇名，导航标签是否同步也必须单独授权并 review，链接目标不得随之漂移。
3. 每个差异项必须有稳定 id、篇章、差异类型、精确 locator、现版/参考片段和必要上下文。含正文片段的报告、静态审阅 HTML 与决策 JSON 只能保存在本地 `work/<book>/reports/`，不得进入仓库级 `records/`。
4. 审阅状态至少包括 `adopt_reference`、`keep_current`、`manual`、`pending`。应用前必须满足：`pending=0`、未决项为 0、选择 `manual` 的项目均有最终文字。
5. 决策 JSON 必须携带 schema 版本、差异源报告 SHA-256、现版/参考 artifact 身份、item count 和逐项决策。应用器必须重新生成或重新核对差异片段；SHA、id、篇章、片段或数量任一不符即停止。
6. 只写出新的候选 EPUB，不覆盖现版或参考版。应用后必须证明最终连续正文等于决策合并结果，并生成“现版 → 候选”与“候选 → 参考版”两份 diff，后者用于显示明确保留的例外。
7. `validate_text_invariance.py --check text` 与 `--check all` 在本分支会如实报告授权文字变化，不能作为通过 gate。必须改跑 `--check metadata,spine,cover,drm,anchors`，并额外验证：只允许决策 locator 指向的文字节点变化；目标 XHTML 的非文字 DOM / 属性签名（tag 序列、`id` / `class` / `epub:type` / `href` / `src` / `alt` / `lang`、ruby / rt 和 pagebreak 等）保持不变。若篇名同步已获授权，可把对应 nav.xhtml / toc.ncx 标签列入成员白名单，但必须证明标签等于最终篇名且链接、顺序不变；注释入口、注释正文、图片引用和其他排除结构仍须保持原样。
8. 若同时生成正文自由版与锁定版，必须应用同一份决策 JSON，并断言两版目标正文完全一致；字体差异继续遵守 §8 的双版本白名单。

### §10.2 黄线（默认可改，但人工 review 必须看见）

AI 可自动执行；review 时通过外部 diff 工具（Calibre Editor / VS Code，见 [EPUB diff review](../pipeline/epub-diff-review.md)）可视化确认：

| 黄线 | 说明 |
| --- | --- |
| 类名、内联样式 -> 外联 CSS 的迁移 | 不改语义、只移位 |
| manifest `properties` 推断（svg / mathml） | 按文件内容推断 |
| nav.xhtml 结构调整 | 锚点 id 保持，只动结构 |
| 字体策略 | 添加 / 删除 `@font-face`，不替换正文文字 |
| 图片格式转换 | 非封面资源可转换，不能裁切或改内容 |
| CSS selector 合并 / 拆分 | 渲染效果应保持 |
| 非封面资源添加 | 如新增 nav.xhtml |

### §10.3 绿线（可自由改，不必单独通告）

| 绿线 | 说明 |
| --- | --- |
| CSS 缩进 / 注释 / 排序 | 纯格式化 |
| 内部 `div` / `span` wrapper 精简 | 不改 class / id |
| 显式删除已被 grep 确认无引用的死代码 | CSS 孤儿类、孤儿 XHTML |
| zip 压缩等级调整 | 不改文件内容 |
| `xml:space` / 空白处理 | 不改语义文本 |

### §10.4 元规则

- 改动可见性：任何改动都必须在外部 diff 工具（Calibre / VS Code）中可见；不允许秘密改动。
- 校验时机：每次 AI 改动后立刻跑 `validate_text_invariance.py`。普通清洗触发红线立即回滚；进入 §10.1.1 后，预期的授权文字差异由决策 artifact 验证，不因此回滚，但其余红线或任何未获授权差异仍立即回滚。
- DRM 检测：处理前先尝试 `unzip -l`，失败或发现 `encryption.xml` 立刻停止。
- 来源记录：清洗操作必须有 `notes.md` 记录改了什么、为什么、用哪个 skill。
- 可回滚：清洗前 epub 保留为 `before/` 备份；不允许就地覆盖。

### §10.5 自动化 gate（CI / pre-commit / AI 自检）

| 检测项 | 命令 | 通过条件 |
| --- | --- | --- |
| 文本红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check text` | 退出码 0 |
| DRM 检测 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check drm` | 不输出 `DRM detected` |
| 核心 metadata 红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check metadata` | 退出码 0 |
| spine 红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check spine` | 退出码 0 |
| 章节锚点红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check anchors` | 退出码 0 |
| 封面红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check cover` | 退出码 0 |
| 全量红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check all` | 退出码 0 |

上表适用于普通清洗。进入 §10.1.1 后，正文变化由已核验的决策 artifact 负责证明；其余红线仍必须逐项通过，不得声称“全量红线通过”。

人工可视化 review 通过外部 diff 工具（Calibre Editor 主路径，VS Code + `unzip` 精细路径，见 [EPUB diff review](../pipeline/epub-diff-review.md)）完成，不在自动化范畴。

### §10.6 能力清单（What this pipeline can / can't do）

#### 能做

| 问题模式 | 主路径 skill | 自动化程度 |
| --- | --- | --- |
| 大量内联 `style="..."` -> 抽到外联 CSS | `epub-css-layering-optimizer` | 高 |
| 标准 footnote 缺 `epub:type`、缺 `aria-describedby` -> 补齐 | `epub-popup-footnote-converter` | 高 |
| 多看 / 旧版阅读器需要弹注 fallback | `epub-legacy-footnote-fallback` | 高 |
| OPF manifest 缺 `properties="svg" / "mathml"` | `epub-package-nav-auditor` | 高 |
| nav.xhtml 缺失 / 结构破损 | `epub-package-nav-auditor` | 中 |
| toc.ncx 与 nav.xhtml 不同步 | `epub-package-nav-auditor` | 高 |
| 字体策略不规范 | `epub-typography-optimizer` | 中 |
| 中英混排排版不稳 | `epub-typography-optimizer` | 高 |
| 英文小说首字下沉 / 字体策略 | `epub-english-typography-optimizer` | 中 |
| 图文环绕用不稳定布局 | `epub-image-layout-optimizer` | 中 |
| Ruby 注音不规范 | `epub-vertical-ruby-optimizer` | 高 |
| Kindle Enhanced Typesetting 转换失败 | `epub-kindle-compatibility-checker` | 中 |
| 文学结构混在一起 | `epub-literary-structure-formatter` | 中 |
| 普通 epub 加 A-lite 增强 | `epub-alite-converter` | 中 |

#### 不能做

| 问题 | 为什么不做 | 用户该怎么办 |
| --- | --- | --- |
| 未经用户授权自动判断并修文字错误 / 通假字 / 错字 | 正文默认不可变，工具不能自行充当校对者 | 回到源头校对；若用户已有参考版并明确授权，按 §10.1.1 逐项审阅和应用 |
| 多语言翻译 / 译文生成 | 工具不做内容生成 | 找译者 |
| OCR 错误（重 OCR 一次） | 不在清洗范围 | 用 `epub-source-intake` 重做 |
| 去图片水印 / 删 DRM | 法律风险 | 找原版授权 |
| 加批注 / 书签 / 高亮 | 不是制作方范畴 | 用阅读器自带功能 |
| 强制改 dc:identifier / dc:title | 核心 metadata 红线 | 重新规划 source |
| 重排 spine | 阅读顺序红线 | 在 source 阶段决定 |
| 把固定版式改为重排 | 信息可能由版式承载 | 重新制作 |
| 视觉效果验收 | reader-matrix 负责 | 跑 reader-matrix 流程 |

> 注：当前仍有部分 demo case 未实测，清单见 `reader-matrix.yaml` 的 `untested_cases` 段；这些场景的渲染表现尚无实测背书，引用其规则前请优先补测并回写。

#### 适配性判断

跑 `python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub`，看 findings：

- 找到的问题多在「能做」清单：适合走清洗流水线。
- 找到的问题多在「不能做」清单：不要走，回到源头。
- 一半一半：先做能做的部分，剩下的另开方案。
