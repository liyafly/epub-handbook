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
| `fonts.css` | 字体声明 | `@font-face`、字体工具类（如 `.rare`） | 排版、颜色、分页、布局 |
| `base.css` | 正文样式 | 段落、标题、列表、表格、代码、ruby、注释 | A-lite 海报页规则（`body.fullpage` / `.fullframe` / `.poster-*`） |
| `poster.css` | A-lite 海报页 | `body.fullpage`、`body.poster-bg`、`.fullframe`、`.poster-title`、`.poster-subtitle`、`.vcol` | 正文常规段落规则 |

附加规则：
- 海报页 XHTML 必须链接 `fonts.css` + `poster.css`（可按需再链 `base.css`）。
- 正文页 XHTML 必须链接 `fonts.css` + `base.css`。
- OPF manifest 必须分别声明 `fonts.css` / `base.css` / `poster.css`（若项目存在 A-lite 页）。

## 8) 字体链规则

- `font-family` 链最长 4 段：嵌入字体 → 1 个系统中文字体 → 1 个跨平台开源中文字体 → generic family。
- 嵌入字体必须放链首；系统字体仅做未嵌入时兜底。
- 不建议在同一条链里堆叠多个同家族别名（如 `Songti SC` / `STSongti-*`）。
- 生僻字回退建议使用独立类（如 `.rare`）与专用字体，不要塞进 `body` 主链。
