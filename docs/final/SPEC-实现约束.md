# SPEC-实现约束（执行层）

> 版本：2026-06-03

> 本文件只记录**实现约束**，用于执行层实现与回归测试的约束清单。描述性解释请看《EPUB 3 终极实践手册》。

## 0) 总则：三层结构与验收

规则分三层，约束强度递减：

| 层 | 内容 | 统一程度 | 保证方式 |
| --- | --- | --- | --- |
| 1 安全底座 | 只有下列五项 | 统一 | 尽量写成 gate |
| 2 配方 | 首字、异形环绕、文白对照、着重号、主题插图等；每个写明增强、回退与证据 | 按需选用 | 逐配方实测，见 [配方目录](../../templates/recipes/README.md) |
| 3 书级设计 | 字体、节奏、章首与细节差异 | 不统一 | 制作说明记录决策 |

安全底座五项（用户决策，不再扩充）：

1. 阅读器可调字号（§5.5.1，gate `css-root-font-size`）。
2. 正文文字不变（§10.1，`epub redline --check text`）。
3. 默认不锁页面级配色，保留夜间与护眼主题（§7，gate `css-page-color-lock`）；书级设计确需固定文字色或底色时可保留，在制作说明写明理由。
4. 图片宽度用百分比（§5.6、§5.11），不以 `em` / 固定像素决定版面。
5. 导航与注释可用（§1、`epub.package.nav.audit`）。

整本 `pre-paginated` 的书（§5.13）豁免第 1 项（可调字号）和第 4 项（图片宽度用百分比），其余三项照常。

验收三态：每个目标阅读器对每个增强只判**增强生效 / 退回可读 / 坏掉**。增强可以失败，但必须失败成可读；只有「坏掉」阻断交付。`reader-matrix.yaml` 新条目可选加 `outcome: enhanced | degraded | broken`，与既有 `status`（`pass | warn | fail | na`）并存；旧条目不迁移，守卫不变。

规则依据：每条硬规则应能说出依据——实测（`reader-matrix` / fixture）、用户决策或推导。推导得来的规则是待验证假设，允许说明理由后不照做。书级调参数值（环绕宽度、文白比例、头图宽度等）是配方默认起点，来源见各 `recipe.md`，不是硬规则。

## 1) 弹注（Footnote Popup）

- 任何使用 `epub:type` 的 XHTML 根 `<html>` 必须声明：`xmlns:epub="http://www.idpf.org/2007/ops"`。
- `a[epub:type="noteref"]` 与对应 `aside[epub:type="footnote"]` 必须位于**同一 XHTML 文件**。
- 每个章节文件最多一个注释容器：`<aside epub:type="footnotes">`，不带 `role="doc-footnote"`。EPUB 3 SSV 中 `footnote` 只表示**单条**注释，`footnotes` 表示注释组；DPUB-ARIA 的 `doc-footnote` 也只表示单条。整组容器不得标成 `footnote`，否则部分阅读器点一条注释会弹出整组。
- **两条路径按书选择，制作说明中记录选了哪条**：书必须严格符合 EPUB 3 语义（面向标准阅读器、Kindle 转换或明确要求规范合规）时走标准结构；其他书默认走多看兼容回退（下方旧 grouped 结构加 `duokan-*`）。多看回退已在部分阅读器上观察到可用，包括 Apple Books，见 `reader-matrix.yaml` 的 `05-duokan-fallback`。同一本书只用一条路径，不得混用。
- 每条注释是一个独立的 `<aside class="footnote-item" id="…" epub:type="footnote" role="doc-footnote">`，noteref 指向它的 `id`。不要只把容器改成 `footnotes` 而让单条注释留在没有语义的 `li` 上：Apple Books 只把 `aside[epub:type="footnote"]` 当作弹注目标，其他阅读器可能退化为跳转。
- `a[epub:type="noteref"]` 必须具有唯一 `id`，供注释回跳定位。
- noteref 的可见触发主路径是图片图标。已有本地图标资源时必须保留原 `img src`；把纯文本、数字上标、`[1]`、`注` 等标记转成图片 noteref 时，须经正文修改授权，由制作方提供图标并同步 OPF manifest。EPUB3 迁移不会转换这些标记或注入默认 `Images/note.png`。
- 图片触发器可以保留外层 `<sup>` 作为旧书兼容包裹，但 CSS 必须把它处理为普通行内图标，不做高位数字上标效果；`sup { vertical-align: middle; line-height: 1; }`，图标自身使用 `vertical-align: baseline` 或 `middle`。
- 图标前后都断行时，检查最终生效的 `display`：外层链接 `inline-block`、内层图片 `block` 的组合在目标阅读器中可能退化为独立块。优先在原有图标规则中只将图片改为 `display:inline`，保持尺寸、链接和正文不变，再做目标阅读器对照；这不是对所有块级图片的禁令，也不等于已确认阅读器内部原因。匿名样本与排查步骤见 [手册 §7.2.1](EPUB%203%20终极实践手册.md)。
- 多条注释放在容器内唯一的 `div.footnote-list` 中：`aside[epub:type="footnotes"] > div.footnote-list > aside.footnote-item`。
- 旧 grouped 结构（`aside[epub:type="footnote"][role="doc-footnote"] > ol.footnote-list > li.footnote-item`）只作为下方多看 fallback 的承载结构保留；非多看场景下 `epub.notes.popup.normalize` 对它给出 `popupnotes.legacy-container` warn，新书和改版书一律用标准结构。
- 每条注释必须可回跳，默认回跳符号 `◎`（U+25CE）。
- 当需要兼容多看旧版本时，暂时改用上述旧 grouped 结构，并同步（基于标准结构的多看写法尚未实测，`epub.notes.legacy-fallback` 遇到标准结构会报 `notes-fallback.no-grouped-list` 并停止）：
  - noteref 锚 `<a>` 增加 `class="duokan-footnote"`，且锚内放注释图标 `<img>`；
  - 注释容器必须使用 `<ol class="footnote-list duokan-footnote-content">`；
  - 每条 `<li class="footnote-item">` 仅挂 `duokan-footnote-item`，禁止在 `<li>` 上重复挂 `duokan-footnote-content`。
- fallback 为次路径，禁止创建第二份注释容器。
- 本节默认约束 EPUB3 主包。OPF `version="2.0"` 外壳中叠加 `xmlns:epub`、`epub:type` 或 `<aside>` 只能视为按目标阅读器验证的兼容模式，不得标为严格 EPUB2 标准路径；实操见 `docs/how-to/epub2-popup-note-compatibility.md`。

## 2) A-lite 页面约束

- A-lite 页面只用于可重排书；固定版式见 §5.13。
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
- 浅色文字压在背景图上时，带背景图的同一个 `poster-*` 元素必须同时声明接近图片主色的深色 `background-color`。背景图没加载或被阅读器去掉时，浅色字仍然可读；不能只靠图片提供对比度。

## 3) 字体与 OPF

- `<meta property="ibooks:specified-fonts">true</meta>` 只在锁定版添加，入口是 `fonts.css` 中直接的 `body { font-family:... }`。添加时 OPF `<package>` 必须同步在 `prefix` 声明 ibooks 命名空间。自由版不加；只在局部角色使用嵌入字体也不加。既有 EPUB 的 `body-font-locked` class 可兼容保留，但不作为新模板入口。规则见 §8。
- 标题等非正文角色指定字体时，使用书内嵌入字体 + 一个系统通用族；当 `h1` / `h2` 在全书各自只有一个字体角色时可直接绑定，否则使用角色类。见 §8。
- 字体策略必须与 `fontspec` 三态一致：`auto | forceAll | none`。
- **既有 EPUB 拆分容错**：同一 `@font-face` 的同一 `src` 声明包含有效非空 `local()` 时，缺失字体 URL 可保留原声明并继续拆分，仅记信息事件，不记 warning；这不证明阅读器具备该系统字体。其他声明里的 local()、字符串/注释伪命中、缺失图片/导入 CSS 等不能获得豁免。新书模板仍不得制造悬空字体资源；此条只定义拆分是否阻断，不替代交付和阅读器验收。

## 4) 子集策略算法（执行层对齐）

`auto` 模式下，先按 CSS 继承、局部覆盖和字体链建立角色字符清单，不得只扫描普通 `p`：

1. 锁定正文或其他要求嵌入字体完整覆盖的角色：纳入全书 XHTML 中最终解析到该角色的全部可渲染文字和标点
2. 局部设计 / 补字角色：纳入明确由嵌入字体承担的字符；其余字符只有在该局部角色声明了可靠 fallback 时才可排除
3. `quotes`、`content` 等 CSS 生成且最终由该角色渲染的字符；现有覆盖分析器不会从 XHTML 自动采集，执行层须另行枚举
4. 角色映射显式要求字符（body / heading / quote / rare）
5. 用户 `extraCodepoints` 与实现显式声明的额外字符

附加规则：
- 当角色字体本身即为人工子集（rare 专用字库），可按角色策略显式 `none`，避免重复裁切。
- 默认回跳符号 `◎` 若继承锁定正文、嵌入注释字体或其他要求完整覆盖的角色，必须进入该角色子集；只有为它声明了独立的非嵌入 fallback 角色时，才可从对应嵌入字体清单排除。
- 子集化、字体改名和文件扩展名改写不得改变 XHTML 正文码位。视觉相似字符（例如 `〇` U+3007 与 `○` U+25CB）不得按字形自动互换；这类变化属于正文校订，必须走 §10.1.1。
- 子集写出后必须重新核对角色字符清单与字体 `cmap`；“子集命令成功”不等于该角色全部实际用字已覆盖。
- 完整字体统一保存在共享字体库（默认 `work-epub/fonts/`，`EPUB_FONT_LIBRARY` 可指定），来源与许可见库内 `字体清单.tsv`。书级 `03 制作工作区/fonts.json` 以 `target` 指定 OPF/CSS 的包内路径，以 `from` 和完整 `sha256` 锁定库中母版；构建只向临时源树注入全量字体，再子集化。既有子集字体保留在源树，不冒充全量母版、不入共享库；母版已有的缺字给出字符与文件定位 warning，子集引入的覆盖回归仍阻断。发行候选只能由书根转发到统一构建器的 `build.sh` 或 `epub.font.subset` 生成，禁止直接打包解包源树。详见[书级工作区指南](../pipeline/book-workspace.md)。
- `epub-font subset` 默认 `usage: "book"`，每个字体按全书字符集生成。显式 `usage: "css"` 时，provider 按已识别的 CSS 字体族分配角色字符；未知选择器将受影响字体扩大到全书，脚本、无法解析的样式表导入、XML stylesheet processing instruction、SVG `use` 或无法证明字符变体完整时拒绝此模式。多个 `@font-face` 若共用 `font-family`，在权重等细分未验证前使用同一字符集。CSS 角色映射报告不代替逐字体 `cmap` / 独立覆盖检查或真实阅读器验收。

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

## 5.5.1) 阅读器字号缩放

- 可重排书的书内 CSS 不得在 `html` / `:root` 上声明 `font-size` 或 `font` 简写，**任何值都不行**（包括 `100%`、`1em`、`62.5%`、`16px`）。多数自定义字号的阅读器把用户字号写在根元素上；书内样式表在同一元素上的声明会在层叠中覆盖它，表现为“部分阅读器调字号无效”，而按缩放比例处理字号的阅读器不受影响，因此只在部分阅读器复现。两本存量书的 CSS reset 与 `html { font-size: 100% }` 均已实测导致此问题，删除后恢复；[字号缩放探针](../../templates/font-scale-probe/README.md)在 Reeden 测试版上复现：仅 `html { font-size: 100% }` 页不放大（reader-matrix `probe-font-scale`，无截图，记为 warn 观察）。修法对所有阅读器通用，不需要 Reeden 专用分支。
- 普通 `body` 上的 `font-size: 1em` / `100%` 在同一探针中未锁定字号，且尚无阅读器复现，不作禁止；它默认继承即等于 `100%`，新写 CSS 不必声明，存量书无需为此单独改动。正文字号差异放在 `p`、角色类或内容容器上，用 `em` / `%`。A-lite 海报页 `body.fullpage` 的固定基准（§2）是有意锁定，不在此列。
- 常见来源是通用 CSS reset（`html, body, div, … { font-size: 100% }`）和模板 `html { font-size: 100% }`；清洗存量书时必须把字号声明从 `html` / `:root` / `*` 上删除（reset 列表里的 `body` 可一并删除），保留其余 reset 属性。
- 单独的 `*` 选择器同样命中 `html`，按 `html` 处理。
- 其它元素不用绝对字号（`px` / `pt` 等，或 `small`、`medium` 等绝对关键字）与视口字号（`vw` / `vh`）：它们不随根字号变化，阅读器放大时这部分文字不动。A-lite 海报页（`.fullpage`）与 `0` / `1px` 隐藏文字惯用法除外。Reeden 测试版会把 `p { font-size: 16px }` 一起放大，但 WebView 类阅读器按 CSS 语义不会，不能据此放宽。
- 自动 gate：`epub run epub.package.nav.audit` 对 `html` / `:root` / `*` 字号声明报 error（`css-root-font-size`，书级构建阻断）；对普通 `body` 报 info（`css-body-font-size`，仅提示冗余）；对其它元素的绝对/视口字号报 warn（`css-fixed-font-size`，人工复核后改为相对单位或说明理由）。
- 证据边界与实测方法见[字号缩放探针](../../templates/font-scale-probe/README.md)；`body` 若在其它阅读器复现锁定，再升级为 warn / error。

## 5.6) 图片环绕兼容

- 图文环绕的通用路径是 `<figure class="img-left|img-right">` 包裹 `<img>` 与可选 `<figcaption>`，把 `float:left/right` 与百分比 `width` 挂在 `figure` 上。
- figure 宽度使用百分比；不得使用 `em` 宽度做 Kindle 主路径，避免字号变化改变绕排阈值。具体百分比是配方默认起点（含取值范围与理由），见 [shape-wrap 配方](../../templates/recipes/shape-wrap/recipe.md)，须按目标阅读器、屏幕和字号实测微调。
- 内层图片必须使用 `width:100%; height:auto;`。不要固定高度，也不要依赖 `aspect-ratio` 作为主路径；真实图片让 `height:auto` 保持天然宽高比。
- 环绕样例必须提供足够长的前后正文；短段落无法证明 float 失败，只能作为阈值反例。
- 不使用 direct `img` 直挂 float 作为主路径，避免部分阅读器图片显示过小。

## 5.6.1) 文白对照左右兼容

- 文白对照、原文/译文对照的基础结构必须按源序保留真实文本：标题、出处、原文、译文、回目录锚点都不可图片化。
- 主路径结构应接近大部头文白书：一组 `.parallel-pair` 直接包含原文段落和译文段落。基础状态必须上下；短组可用 `.parallel-float-pair`，但双侧 `float:left/right` 只能放在宽屏 `@media (min-width: 40em)` 增强里。两栏比例与 `.parallel-ratio-*` 变体是配方默认起点，见 [parallel-pair 配方](../../templates/recipes/parallel-pair/recipe.md)，单书可自定，但须给 gutter 留空。Kindle 电子墨水、小屏、大字号、长组、大字号探针或已知风险组应保持上下。
- 不依赖 `overflow:hidden` BFC 形成右侧列；KF8/KFX 对这条路径不稳定。不要为了标签和列容器把每组正文包得过重，除非确有多段注记需求。
- 分页保护只能用于短组增强类：`.parallel-float-pair { page-break-inside: avoid; -webkit-page-break-inside: avoid; break-inside: avoid; }`；`.parallel-pair` 默认允许长对照自然切页，`.parallel-stack-pair` / `.parallel-pair-allow-break` 必须显式允许切页，避免 Kindle 产生大空白页。
- 不使用 `table`、`display:flex`、`grid`、absolute positioning 或固定版式承载正文对照。阅读器忽略 float、屏幕过窄或大字号列宽不足时，必须退回源序上下显示。
- 不引入 `amzn-kf8` / `amzn-mobi` 媒体查询；不要把左右对照只写在 `@media (orientation: landscape)` 内。Kindle Previewer / KFX 对 flex 与 orientation 组合不应作为主路径。
- 不强求任何 Kindle 窄屏 pair 左右对照；长段、字号探针或已知风险 pair 用 `.parallel-stack-pair` 保持上下。
- Kindle 用户字体覆盖后宋楷差异可能消失，原文/白话至少要靠段落顺序、出处和上下文独立可读；生产书可在条目级补充说明，但不要在每个短 pair 里堆标签。
- Kindle 专用 AZW3/MOBI 成品中可见 `table-layout: fixed` 左右对照先例，但通用 EPUB / KDP 源文件不把 table 当正文对照主路径；只有明确只交付 Kindle 成品格式并逐设备验收时，才作为专用例外。

## 5.6.2) 异形环绕（渐进增强）

- 使用四层结构：L0 是普通 block figure；L1 为 figure 的百分比宽度与左右 float，内图 `display:block; width:100%; height:auto`；L2 为按图指定的 `shape-outside: polygon(...)` 与 `shape-margin`；L3 在窄屏媒体查询中取消 float。L1/L2 使用同一宽度，先保证矩形回退可读，不依赖 `@supports` 切换宽度。
- 增强主路径只用显式 `polygon()`，不用 PNG alpha 的 `url(image)` 推断。左右浮动时按实际图片坐标镜像；只需准确描正文一侧的边缘，另一侧闭合到盒边。点数以足够描述形状为准，通常不超过约 20–40 个；L 形可更少。
- 必须明确参考盒。裸 polygon 默认参考 margin-box；历史独立样本显式使用 `content-box`，其百分比对应无 padding/border 的图像内容盒。可复制配方沿用此路径，保留正文一侧的普通 margin 作为矩形回退间距；不要以 `shape-margin` 替代所有回退间距。若选择默认 margin-box，须将坐标按实际盒模型重算，不能直接搬内容盒坐标。参见 [CSS Shapes §6.1](https://www.w3.org/TR/css-shapes-1/#shape-outside-property)。
- 轮廓 figure 不放 figcaption；图注移到不参与轮廓的普通流结构。保证后续连续正文足够长，确认凹口、逐行边缘、无重叠及图下恢复整行；窄屏、大字号与深色模式分别验证。
- [独立样本](../../templates/shape-wrap-demo/README.md) 的历史 v3，在 Apple Books 与 Readest 的所记版本/普通字号中沿轮廓环绕；Kindle Previewer 按矩形回退。结论仅绑定 [reader matrix 的 shape-wrap-polygon](reader-matrix.yaml)，不推导实体 Kindle、手机或新 CSS 组合的通过。生产设计主动退回矩形并不代表轮廓能力失效。

## 5.6.3) 版权页与前置书目信息

- 版权页必须保留为可选择、可检索、可朗读的真文本；不得用整页扫描图替代。原书已有的 CIP、书名、著译者、出版发行、印刷、版次、印次、书号、定价和版权声明应保持原有信息与顺序，不得为“补齐字段”凭空添加内容。
- 主容器使用 `section.frontmatter.copyright-page` 并声明 `epub:type="frontmatter copyright-page"`。存量书保真转录的主路径是按原页阅读顺序使用连续 `p`，用 scoped class 和普通 `hr` 分组；不为追求结构感强拆原有行序。没有可见标题时给主容器补 `aria-label="版权信息"`。
- 原生电子书或标签—值关系明确的资料可使用 `h1` / `h2` 与 `dl` / `dt` / `dd`。标签和值的基础 DOM 必须按阅读顺序排列；CSS Grid 只作宽屏增强，阅读器忽略后仍须线性可读。
- 不使用 `table` 伪造版权页书目信息，也不依赖 absolute positioning、固定页高或固定坐标还原纸页。
- 使用相对字号和间距，不锁定页面背景色、前景色或固定版心。可按角色给版权页、标题和标签使用授权字体，但不因单一版权页需求强制整本正文锁定字体。
- 版权页清洗默认只调整语义结构和排版，不改动书目事实。发现 ISBN、版次、日期或定价疑点时进入书目人工复核，不依据 OCR 猜测改写。

## 5.7) 文字装饰兼容

- 带样式的下划线必须拆成多条声明：先写 `text-decoration: underline;`，再写 `text-decoration-style: wavy;` 等增强属性。
- Kindle App 已实测能显示基础 underline fallback，但不会显示 wavy；这属于预期降级，不再视为样式丢失。

## 5.8) MathML

- 含 MathML 的 XHTML manifest item 必须声明 `properties="mathml"`。
- 正文中可明确判定的行内数学表达使用 `display="inline"` 的 MathML，只给 `math` 绑定数学字体，不把数学字体施加给中文段落。需要可审计公式源的生产流程中，每个公式使用 `semantics` 包裹表达树，并保存非空的 `annotation encoding="application/x-tex"`；注释不得成为重复可见文本。
- MathML 只表达公式语义；公式编号、前后说明和多个公式的页面并列关系使用 HTML/CSS。编号基础 DOM 采用公式在前、编号在后的线性顺序。
- 通用 demo 可用 `.eq-grid` 做居中公式 + 右侧编号的渐进增强。单一 EPUB 同时交付 Kindle 与 Readest、并已逐目标实测时，可使用 scoped 的真实 HTML `.eq-table` 作为保守外层；表格必须声明 `role="presentation"`，编号留在 MathML 外，且不得把该结论扩张为所有 Kindle 版本通用。
- 不把 `mlabeledtr` 作为跨阅读器公式编号主路径。`mtable` 只承载矩阵、分段函数或方程组内部结构，不设置固定 `width`；方程组大括号使用可伸展的普通 `<mo>{</mo>`。
- 一行含说明、方程组和推导结果时可用 `.sys-row` Flex + `flex-wrap:wrap`，但源序必须在 Flex 被忽略后仍然可读；禁止 absolute positioning 承载编号或关键信息。
- Kindle 路径只把 MathML 视为 Enhanced Typesetting 能力；目标平台不支持时必须准备文本公式或图片公式 fallback。
- demo 优先覆盖 KDP 支持列表内标签组合，不引入未确认支持的私有数学标签。Grid/Flex 公式布局必须在目标阅读器和大字号下单独复测；未实测时只记为 `warn` / 待验证假设。

## 5.9) 英文小说正文

- 英文书页必须在 HTML 或 `body` 上声明 `xml:lang="en"` / `lang="en"`，让 Apple Books、Readest、Kindle 和其他阅读器有机会启用正确断字和朗读规则。
- 英文正文可使用短 serif 链，系统字体优先、嵌入字体第二或第三位，如 `Georgia, "en", serif`；规则见 §8。不要沿用中文链。
- 简单英文小说的主路径是首段无缩进、后续段落 `1.2em–1.5em` 首行缩进，段间距接近 0；不要同时使用大段间距和大缩进。
- 英文正文不强制 `justify`。未实测 hyphenation 的 Kindle/Readest/Apple Books 路径优先 `text-align:left`；确认断字稳定后才使用 `justify`。
- 章首插图和正文插图默认使用居中 `figure`，宽度用 `max-width` 约束，不固定页高，不把插图做成固定版式页面。
- 首字装饰优先用 `::first-letter`，保持正文单词完整；旧式 span 首字和浮动 drop cap 只作为增强，并必须在朗读、复制文本、大字号和窄屏下复测。下沉首字若需要特殊字体，使用授权嵌入字体 + 一个系统通用族回退，并声明 OPF font item；无字体资源的 demo 只作结构对照，不能作为指定字形通过的证据。

## 5.9.1) 书信

书信结构用 `.letter-paper`，完整接入见 [letter-paper 配方](../../templates/recipes/letter-paper/recipe.md)。默认 demo 的 `.letter` 边线样式是另一个场景，不从它推导本节结论。

- 格式：称呼、祝愿顶格；问候、正文、祝语引导空两格；署名、日期右对齐。全部用 CSS 实现，不在正文里写空格或方框，不预先拆行。
- 字体：书信是固定字体角色，绑定专用别名 `tszt-letter`，不随正文换字；换字体只改 `fonts.json` 的指向。
- 横线：每段文字包在 `span.letter-rule` 里，用行内 `border-bottom` 逐行画线，段落可以拆页，长短信同一写法。空行用 `.letter-gap`。不得改回段落或容器背景，也不得改成渐变，原因见配方的“坑点”。
- 顶格写 `0em`，`text-indent` 与 `duokan-text-indent` 都写。不带单位的 `0` 不作为多看路径。
- 回退：Kindle 与其他阅读器同包，经 `@media amzn-kf8` 去掉横线，`epub.kindle.compatibility.check` 的 `kindle.css-amzn-media-query` warn 是预期结果。§5.6.1 对这个媒体查询的限制只适用于文白对照。多看单独出无横线变体，叠加 `reader-duokan.css`，因为没有 CSS 能单独识别多看。
- 验收：先恢复阅读器默认版式。多看调过“版式”里的间距或首行缩进后，顶格和横线的现象不能作为样式结论。

## 5.10) 边框、阴影与便签文本框

- 便签、提示、摘录和资料卡必须保留真实文本；禁止把文字直接烘焙进图片来实现纸张效果。
- 主路径使用 `border`、`border-left`、`background`、`padding`、`margin` 和 `page-break-inside: avoid`。这些属性在 Readest、Apple Books、Kindle Previewer 和旧 WebKit 路径中更稳。
- `box-shadow`、`inset box-shadow`、`outline-offset`、不对称 `border-radius` 只能作为渐进增强；阅读器忽略时不得影响阅读。
- 通用 EPUB 不使用 `transform: rotate()` 旋转便签文本框。Kindle Previewer 3.104（2026-05-23 实测）会在 KFX 增强排版转换中触发内部错误；若某个非 Kindle 发行目标确实需要旋转效果，必须放在该目标专用版本并单独验证。
- 不依赖 `clip-path`、复杂滤镜、CSS mask 或多层伪元素承载关键信息；它们在 EPUB 阅读器中支持不稳定。
- 长文本便签不要追求倾斜效果。需要贴纸感时优先用不对称边框、圆角和投影模拟，避免窄屏下产生裁切或左右溢出。
- 内联 SVG 花边只作为实验验证项，不作为通用推荐边框。若强设计需求必须使用，SVG 只能承载装饰边线并加 `aria-hidden="true"`，正文仍是 HTML 真文本；生产版必须能降级为双线框、左侧竖线框或普通边框。

## 5.10.1) 并排插图与图题对齐

- 两幅或多幅独立插图并排时，各图仍使用独立 `figure` / `figcaption`，外层使用 Flex 或 Grid；不使用 HTML `table` 伪造非表格版面。
- 原书要求不同宽高比的图在同一行垂直居中且图题底对齐时，给每个 `figure` 增加等高图像区：图像区使用 Flex 居中，`figure` 自身纵向排列并拉伸，图题放在图像区之后。忽略 Flex 后，各图仍按源序独立可读。
- 窄屏或大字号下允许退化为上下排列。只有目标阅读器实测不支持上述布局、且必须保留同一行时，才可使用声明 `role="presentation"` 的布局表格作为目标版本例外；不得把图片 `alt`、图题或阅读顺序只交给 CSS。

## 5.11) 章节头图

- 普通可重排章节头图属于章首结构，写入 `literary.css`，不归入正文图文混排的 `media.css`。
- 头图必须是装饰或栏目识别；章节标题、kicker、副标题必须保留为真实 XHTML 文本。
- 小型章标使用保守百分比宽度并加 `max-width`；具体取值与增强类上限见 [章节头图 how-to](../how-to/chapter-head-image.md)，属书级调参。
- 满栏横幅头图可使用 `width:100%; max-width:100%`，只铺满正文内容栏，不要求突破阅读器页边距。
- 内层图片使用 `height:auto`；横幅高度由源图宽高比控制。不固定高度，不使用 `vh/vw`、absolute positioning 或大段顶部空白模拟固定页。
- 如果章首需要强视觉首屏、背景图或大面积叠字，应改走 A-lite 海报方案；不要把普通章节头图扩大成伪固定版式。

## 5.12) 竖排纵中横

> 依据：推导，待验证。下列写法来自 CSS Writing Modes 与 EPUB 3.0 前缀，尚无本仓库阅读器实测；结论待 `recipe-tate-chu-yoko` 实测后更新。

- 竖排正文里两三位的数字用 `<span class="tcy">` 包住，不改文字、不插空格。四位以上不做纵中横，保持侧躺或改用汉字数字（后者是正文改动，需要授权）。
- `.tcy` 依次写 `-webkit-text-combine: horizontal`、`-epub-text-combine: horizontal`、`text-combine-upright: all`，标准写法放最后。
- 回退：不认这些属性时数字侧躺，仍按原序可读。横排页面里 `.tcy` 不起作用，不另写横排规则。
- 写法与样例见 [tate-chu-yoko 配方](../../templates/recipes/tate-chu-yoko/recipe.md)。

## 5.13) 固定版式（整本 pre-paginated）

> 依据：一本外部生产书的结构（只读摘要，书级材料留在本地）和所有者“在 Reeden、Apple Books、Kindle 上都能显示”的口头确认；阅读器版本、平台、Kindle 送达方式和截图都没有记录。下面凡涉及阅读器行为的条文都是待验证。

- 范围：只支持整本固定版式，OPF 写全局 `<meta property="rendition:layout">pre-paginated</meta>`。可重排书里夹几页固定版式的混排不做；把固定版式改成可重排也不做（§10.6）。
- 每页一个 XHTML，`head` 里写 `<meta name="viewport" content="width=W, height=H"/>`，W、H 是 CSS 像素整数。全书各页宽高比应一致。
- 对开：OPF 写 `rendition:spread`（`none`、`landscape`、`both` 或 `auto`），spine 项用 `page-spread-left` / `page-spread-right` 指定左右页。`page-spread-*` 只用在固定版式项上。
- 文字尽量保留为真文字：图片只做底图或插图，文字放在绝对定位的元素里、用授权嵌入字体。页面内用 `px` 定位和定字号是正常写法（§0 豁免第 1、4 项）。
- 文字层就是读者看到的文字。底图是空白纸时，文字层漏掉的标点或字在屏幕上同样缺失；交付前要校对文字层，改字按 §10.1.1 走授权。
- 嵌入子集字体的 cmap 可能把没保留的码位映射到空字形（宽度 0）。查字体覆盖要看字形是否有轮廓，不能只看 cmap。
- Kindle：送达方式（Send to Kindle、USB、Kindle Previewer）和已知限制待验证；CFF 轮廓的 OTF 是否在固定版式里触发警告也待验证。
- 阅读器结论记在 `reader-matrix.yaml` 的 `fxl_epub` lane 与 `external-production-fxl`，没有截图或日志只能是 `warn`。

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
| `design.css` | 书级组合设计（双层 starter） | 本书选用的配方、角色类与视觉；注明配方来源与本书改动 | 字体声明与绑定、覆盖安全底座、未验证的整书主题强制色 |
| `notes.css` | 弹注 | `noteref-*`、`footnote-*`、`duokan-footnote-*` 全套 | 字体声明、文字效果、文学结构 |
| `effects.css` | 文字效果 + 便签视觉 | `.emp` / `.wavy` / `.dropcap` / `.note-box` 边框阴影类 | 字体声明、弹注、文学结构 |
| `literary.css` | 文学结构 + 前置页 | `.dialog` / `.poetry` / `.letter` / `.scene-break` / `.chapter-head` / `.chapter-head-art` / `.chapter-head-banner` / `.chapter-header` / `.epigraph` / `.copyright-page` / `.dedication` / `.epigraph-page` / `.english-fiction` / `.classical-modern` / `.parallel-entry` / `.parallel-pair` / `.parallel-float-pair` / `.parallel-stack-pair` | 弹注、普通图文浮动、海报、竖排 |
| `media.css` | 图文混排 + 公式 | 图片浮动九宫格、`.figure-grid`、`.math-block` / `.math-inline` | 普通 `figure` / `img` 基础样式 |
| `vertical.css` | 整页正文竖排（非 A-lite） | `body.page-vrl` / `.vrl-section` / `.vrl-title` | 海报规则 |
| `poster.css` | A-lite 海报 | `body.fullpage` / `body.poster-bg` / `.fullframe` / `.poster-title` / `.poster-subtitle` / `.vcol` | 正文段落规则 |

附加规则：
- EPUB 样式表不得包含 `direction` 或 `unicode-bidi`；XHTML 内容需要基准方向时使用 HTML `dir` 属性。EPUB 3.3 §6.3.1.2 禁止这两个 CSS 属性，EPUBCheck 对禁用属性发出 CSS-001。
- 加载顺序：`fonts.css → base.css → notes/effects/literary/media/vertical/poster.css`。
- 海报页 XHTML link `fonts.css + poster.css`（如需正文排版再加 `base.css`）。
- 常规分层正文页 link `fonts.css + base.css`，其他层按场景选用。无字体声明/绑定的双层 starter 只 link `base.css → design.css`；需要字体时另加 `fonts.css` 并置于最前。已有多层书不为统一文件数而合并。`design.css` 达到行数上限时按职责拆分；标准弹注仍用 `notes.css`。
- 若 `.note-box` 容器视觉继续增长并让 `effects.css` 超过 400 行，优先拆出 `decoration.css` 承载便签/资料卡边框阴影；`effects.css` 保留文字效果。超过 500 行必须拆分。
- OPF manifest 必须分别声明所有存在于 `Styles/` 的 CSS 文件。
- `html`、普通 `body`、`body.fullpage`、普通标题、图注和引用不设置页面级 `color` / `background` / `background-color`，避免覆盖阅读器夜间模式、护眼模式和用户主题；局部组件可保留必要的边框、阴影和背景装饰。这是默认做法，不是禁令：书级设计（如封面页、章首页底色或正文定制文字色）确需保留时，在书级制作说明记录理由即可，warn 不阻断构建。
- 自动 gate：`epub run epub.package.nav.audit` 对 `html` / `:root` / `*` / 普通 `body`（含带类名的 body）声明固定的 `color` / `background` / `background-color` 报 warn（`css-page-color-lock`，人工判断后改为继承或说明理由）；值为 `inherit` / `transparent` / `none` / `initial` / `unset` / `currentcolor` 不报；主体复合选择器含 `.poster-*` 类（A-lite 背景 modifier）豁免，`body.fullpage` 本身不豁免。存量书常见，只报告不改书。
- 单文件 400 行预警、500 行硬上限；超过 500 行必须按职责拆分或迁入已有正确层。
- 跨层依赖通过类名契约，不允许下层文件引用上层组件类。

## 8) 字体链规则

> **设计意图**：一本书最多两个版本。自由版把正文交给阅读器，读者用自己习惯的字体看书；锁定版的正文只用嵌入字体。中文的书信、诗歌、标题、注释等角色都用嵌入字体，不写系统字体名。英文和代码是例外，可以系统字体优先。

### 正文：两个版本

- **自由版**：`body` 和普通正文 `p` 都不写 `font-family`，OPF 不加 `ibooks:specified-fonts`。
- **锁定版**：只在 `fonts.css` 写 `body { font-family: "st", serif; }`，普通正文 `p` 继承；OPF 同步加 `<meta property="ibooks:specified-fonts">true</meta>`，`fontspec` 切到 `forceAll`。
  - 嵌入字体必须覆盖最终解析到正文角色的全部实际用字、标点和 CSS 生成字符。可以是完整字体，也可以是按这些字符生成的子集；局部补字子集（如 `tszt-rare`）不能挂到 `body` / `h*`。
- **不用系统字体链锁定中文正文**：系统字体名在各设备上不一定存在，实际命中不可预期。
- 日常按锁定版制作和校对，自由版在定稿时构建。两版的差异规则见下文“双版本”。
- 版本是**全书级决策**，不按页混用。既有书的 `body-font-locked` class 只作为兼容输入保留，不向新模板扩散。
- 裸 `p { font-family:... }` **不作为全书锁定入口**：它只覆盖段落，不能覆盖列表、引用、表格等其他正文容器，还会误伤同为 `p` 的注释。只需锁定某类正文段落时，应使用 `.bodytext`、`.prose` 等明确角色类。`base.css` 不承担字体绑定。

### 中文非正文角色：只用嵌入字体

- 书信、诗歌、标题、题签、序言、注释等角色要指定字形时，链只写两段：嵌入字体 alias + 一个系统通用族（`serif` / `sans-serif` / `monospace`），如 `.letter-paper { font-family: "tszt-letter", serif; }`。末尾的通用族只是字体失效时的兜底，不再追加其他字体名或正文嵌入字体。
- 角色没有指定字形时，不写 `font-family`，跟随正文。**不写系统字体名**；只写系统字体名不算指定了字形。
- 生僻字只用 `.rare { font-family: "tszt-rare", serif; }` 包住实际字符，不追加正文字体。
- 嵌入字体应覆盖该角色承担的字符。缺字时阅读器会用回退字体显示，有的显示正常，所以缺字**只警告，不阻断构建，要在多个真实阅读器里实际查看**。`epub book build` 的 `fixed-role fonts` 列出这些字：字体链第一段是嵌入字体的文字里，这条链的嵌入字体都没有的字。第一段是系统字体的链（如英文的 `Georgia, "en", serif`）和不写字体的正文不查。按每页实际加载的样式表判断，包括 ASCII 和标点；CSS 生成的文字不查。

### 英文和代码：系统字体优先

- 英文和代码可以系统字体优先，嵌入字体放第二或第三位，链尾写通用族，最多 4 段，如 `Georgia, "en", serif`、`"SF Mono", Consolas, "mono", monospace`。
- 斜体用真斜体：嵌入英文字体要单独带 `font-style: italic` 的字体文件，否则阅读器会把正体机器斜切。
- 中文不用斜体。英文链里夹中文时，中文也会被斜切；中文强调用着重号。

### 阅读器换字的实际结果

“固定”指默认设置下的显示，不代表读者换字后仍然保持。EPUB 无法禁止阅读器的全局覆盖：

| 阅读器 | 读者换字后 | 证据 |
| --- | --- | --- |
| Apple Books | 正文和所有角色字体**全部被替换**，自由版、锁定版、加不加 `ibooks:specified-fonts` 都一样 | `probe-font-role`，2026-11-09 用户实测 |
| Reeden | 角色字体保持 | 同上，仅测自由版 |
| Kindle、多看、Readest | 未记录 | — |

这决定了两版的用途，阅读说明见 [阅读器指南](../guide/readers.md)：

- **自由版给想换字体的读者**，推荐 Reeden、Readest。
- **不想换字体时**，用 Apple Books 的“原始字体”、Kindle 的“出版商字体”或多看的“默认字体”。这时锁定版显示嵌入的正文字体，自由版的正文是阅读器默认字体。

### 选择器和资源

- **角色选择器**：结构元素在全书只有一个稳定字体角色时，可直接使用 `body`、`h1`、`h2`、`body.preface`、`aside[epub|type~="footnote"]` 等选择器；同一元素承担多种角色、只在少数位置换字形或需要例外时，必须改用明确角色类。使用 `epub|type` 这类命名空间选择器的 CSS 必须声明 `@namespace epub "http://www.idpf.org/2007/ops";`；它紧跟可选的 `@charset` / `@import`，并早于 `@font-face` 和普通规则。
- `fonts.css` 中这些规则只写字体属性，不混入布局、颜色和分页。注释字体绑定写在 `fonts.css`，`notes.css` 只保留注释结构与视觉。不要把嵌入字体直接挂到角色混杂的裸 `p` / `div` / `span`。
- CSS URL、OPF manifest 与字体文件必须闭环。没有任何选择器引用的 `@font-face` 必须删除或保持注释，OPF 不挂对应字体 item。
- 同一条链里嵌入字体只出现 1 次。需要“设计字形 + 生僻字”时拆成两个类，不塞进同一条链。
- 不堆叠同一字体的多个别名（如 `Georgia` + `Georgia Pro`）。
- 新建字体 alias、文件名和 class 必须遵循 [字体别名命名规范](字体别名命名规范.md)：使用 `en` / `st` / `kt` / `fs` / `ht` / `tszt-*` 等角色缩写；不得新增 `Book*`、`RareSong*` 或 `.book-*` 字体命名。

### OpenType 特性

> 依据：推导，待验证。尚无本仓库阅读器实测；结论待 `recipe-opentype-features` 用真字体在书级实测后更新。

- 旧式数字、小型大写、连字只用于带这些特性的授权嵌入字体：每个类同时写 `font-variant-*` 和对应的 `font-feature-settings`。不认或字体没有该特性时显示普通字形。
- `font-feature-settings` 不叠加。一个元素需要多个特性时写一个组合类，把特性写进同一条声明，不靠多个类叠加。
- 子集化保留全部 OpenType 特性，provider 无需额外配置。公共合成包不带字体，不能作为字形证据。
- 写法与样例见 [opentype-features 配方](../../templates/recipes/opentype-features/recipe.md)。

### 双版本

- 两版必须从同一个已定稿内容基线派生。允许差异只限字体 CSS、字体资源、OPF 中与字体有关的 manifest/meta（含 `ibooks:specified-fonts`），以及该 meta 所需的 `<package prefix>` 中 `ibooks:` 声明；另允许每个 rendition 唯一的 `dcterms:modified` 反映各自实际打包时间。双版本比较可忽略该字段的值，但不得忽略缺失、多份或非 UTC `YYYY-MM-DDThh:mm:ssZ`。同一批成对构建应优先只取一次 `BUILD_TIMESTAMP` / `SOURCE_DATE_EPOCH` 传给两版，以减少无意义 diff；分步打包时的合法时间差不视为内容漂移。核心 `dc:*` metadata、spine、XHTML、注释、图片和其他资源必须一致。任何超出此白名单的差异都要另行授权并记录。
- 既有 EPUB 若因历史兼容或用户明确要求在自由版保留 meta，必须在书级报告记录理由，并在书根 `制作说明.md` 显式记录豁免条目；该例外不得写回 starter、preset 或新书模板。

### 存量书

本节不追溯已交付的书。`epub.css.layering.optimize` 处理存量书时，删除旧系统字体声明（`SimSun`、`SimHei`、`STKaiti`、`cnepub`），让这些文字跟随正文；书内 `@font-face` 字体的绑定不动。

## §10 AI 清洗已有 EPUB 的改动边界

> 本节给 AI 协作代理使用：当输入是一本已存在的 EPUB（而不是从零构造）时，AI 的改动必须落在本节边界内。
> 任何破坏本节约束的改动都视为事故，需要回滚。

### §10.1 红线（默认不可改；显式授权才可转入专门分支）

AI 检测到自己将要触发以下任一改动时，必须停止并询问用户：

| 红线 | 说明 | 校验方式（自动） |
| --- | --- | --- |
| 正文文本 | 去除标签后的纯文本不允许变化；标点、错别字、通假字一律不动 | `epub redline --check text before.epub after.epub` |
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
3. 每个差异项必须有稳定 id、篇章、差异类型、精确 locator、现版/参考片段和必要上下文。含正文片段的报告、静态审阅 HTML 与决策 JSON 只能保存在本地书级 `02 校对材料/正文校订/`，不得进入手册仓库级 `records/`。
4. 审阅状态至少包括 `adopt_reference`、`keep_current`、`manual`、`pending`。应用前必须满足：`pending=0`、未决项为 0、选择 `manual` 的项目均有最终文字。
5. 决策 JSON 必须携带 schema 版本、差异源报告 SHA-256、现版/参考 artifact 身份、item count 和逐项决策。应用器必须重新生成或重新核对差异片段；SHA、id、篇章、片段或数量任一不符即停止。
6. 只写出新的候选 EPUB，不覆盖现版或参考版。应用后必须证明最终连续正文等于决策合并结果，并生成“现版 → 候选”与“候选 → 参考版”两份 diff，后者用于显示明确保留的例外。
7. `epub redline --check text` 与 `--check all` 在本分支会如实报告授权文字变化，不能作为通过 gate。必须改跑 `epub redline --check metadata,spine,cover,drm,anchors`，并额外验证：只允许决策 locator 指向的文字节点变化；目标 XHTML 的非文字 DOM / 属性签名（tag 序列、`id` / `class` / `epub:type` / `href` / `src` / `alt` / `lang`、ruby / rt 和 pagebreak 等）保持不变。若篇名同步已获授权，可把对应 nav.xhtml / toc.ncx 标签列入成员白名单，但必须证明标签等于最终篇名且链接、顺序不变；注释入口、注释正文、图片引用和其他排除结构仍须保持原样。
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
- 校验时机：每次 AI 改动后立刻跑 `epub redline`。普通清洗触发红线立即回滚；进入 §10.1.1 后，预期的授权文字差异由决策 artifact 验证，不因此回滚，但其余红线或任何未获授权差异仍立即回滚。
- DRM 检测：S1 以 `epub run epub.package.nav.audit` 判定；DRM 与未知加密立刻停止。`encryption.xml` 只含目标已不存在的 stale 引用时，可由 normalize 移除；仅在确认标准字体 obfuscation 且获明确授权时，按 AGENTS 单独处理。
- 来源记录：清洗操作必须在书根 `制作说明.md` 记录改了什么、为什么、用哪个 skill。
- 可回滚：清洗前 epub 保留为 `before/` 备份；不允许就地覆盖。

### §10.5 自动化 gate（CI / pre-commit / AI 自检）

| 检测项 | 命令 | 通过条件 |
| --- | --- | --- |
| 文本红线 | `epub redline --check text before.epub after.epub` | 退出码 0 |
| DRM 检测 | `epub redline --check drm before.epub after.epub` | 不输出 `DRM detected` |
| 核心 metadata 红线 | `epub redline --check metadata before.epub after.epub` | 退出码 0 |
| spine 红线 | `epub redline --check spine before.epub after.epub` | 退出码 0 |
| 章节锚点红线 | `epub redline --check anchors before.epub after.epub` | 退出码 0 |
| 封面红线 | `epub redline --check cover before.epub after.epub` | 退出码 0 |
| 全量红线 | `epub redline --check all before.epub after.epub` | 退出码 0 |
| 阅读器字号缩放 | `epub run epub.package.nav.audit --input after.epub --json` | 无 `css-root-font-size` error；`css-fixed-font-size` warn 已修改或说明（§5.5.1） |
| 页面级配色锁定 | `epub run epub.package.nav.audit --input after.epub --json` | `css-page-color-lock` warn 已修改或说明理由（§7） |

上表适用于普通清洗。进入 §10.1.1 后，正文变化由已核验的决策 artifact 负责证明；其余红线仍必须逐项通过，不得声称“全量红线通过”。

人工可视化 review 通过外部 diff 工具（Calibre Editor 主路径，VS Code + `unzip` 精细路径，见 [EPUB diff review](../pipeline/epub-diff-review.md)）完成，不在自动化范畴。

### §10.6 能力清单（What this pipeline can / can't do）

#### 能做

| 问题模式 | 主路径 skill | 自动化程度 |
| --- | --- | --- |
| 大量内联 `style="..."` -> 抽到外联 CSS | `epub-cleanup` | 人工（CLI 不抽取内联样式） |
| 标准 footnote 缺 `epub:type`；ARIA 关联需人工核对 | `epub-cleanup`（`epub.package.migrate.epub3`） | 人工；migrate 不补缺失的 `epub:type`，只为已有 `aside[epub:type="footnote"]` 补 `role`、规范化 Duokan class |
| 多看 / 旧版阅读器需要弹注 fallback | `epub-special-layout`（`epub.notes.legacy-fallback`） | 标准弹注检查后可补已识别 legacy class；交互仍需阅读器实测 |
| OPF manifest 缺 `properties="svg" / "mathml"` | `epub-cleanup`（`epub.package.migrate.epub3`） | 中（迁移时标记已识别特性） |
| nav.xhtml 缺失 / 结构破损 | `epub-cleanup`（`epub.package.migrate.epub3`） | 中（可识别输入由迁移生成；其他结构人工修复） |
| toc.ncx 与 nav.xhtml 不同步 | `epub-cleanup`（`epub.package.migrate.epub3`） | 人工（nav.audit 不比对 NCX 与 nav；migrate 只在缺 nav 时由 NCX 生成） |
| 字体策略不规范 | `epub-cleanup`（`epub.css.layering.optimize` 整理分层并删除旧系统字体声明；其余字体链在书级 CSS 人工调整） | 人工 |
| 中英混排排版不稳 | `epub-cleanup`（书级 CSS 人工调整，参考 [配方目录](../../templates/recipes/README.md)） | 人工 |
| 英文小说首字下沉 / 字体策略 | `epub-special-layout`（`epub.typography.english.optimize`） | 只读检出；语言声明可修补，drop cap 与字体排版人工决定 |
| 图文环绕用不稳定布局 | `epub-audit`（`epub.image.layout.optimize`） | 只读检出，人工修复 |
| Ruby 注音不规范 | `epub-special-layout`（`epub.vertical.ruby.optimize`） | 高 |
| Kindle Enhanced Typesetting 转换失败 | `epub-reader-verify`（`epub.kindle.compatibility.check`） | 只读静态检查；转换器与阅读器问题需人工验证 |
| 文学结构混在一起 | `epub-special-layout`（`epub.literary.structure.format`） | 中（角色须人工确认） |
| 普通 epub 加 A-lite 增强 | `epub-package-ops`（`epub.alite.convert`） | 中 |

#### 不能做

| 问题 | 为什么不做 | 用户该怎么办 |
| --- | --- | --- |
| 未经用户授权自动判断并修文字错误 / 通假字 / 错字 | 正文默认不可变，工具不能自行充当校对者 | 回到源头校对；若用户已有参考版并明确授权，按 §10.1.1 逐项审阅和应用 |
| 多语言翻译 / 译文生成 | 工具不做内容生成 | 找译者 |
| OCR 错误（重 OCR 一次） | 不在清洗范围 | 外部 OCR 重做 source，再用 `epub-source-intake` 盘点 |
| 去图片水印 / 删 DRM | 法律风险 | 找原版授权 |
| 加批注 / 书签 / 高亮 | 不是制作方范畴 | 用阅读器自带功能 |
| 强制改 dc:identifier / dc:title | 核心 metadata 红线 | 重新规划 source |
| 重排 spine | 阅读顺序红线 | 在 source 阶段决定 |
| 把固定版式改为重排 | 信息可能由版式承载 | 重新制作 |
| 视觉效果验收 | reader-matrix 负责 | 跑 reader-matrix 流程 |

> 注：当前仍有部分 demo case 未实测，清单见 `reader-matrix.yaml` 的 `untested_cases` 段；这些场景的渲染表现尚无实测背书，引用其规则前请优先补测并回写。

#### 适配性判断

跑 `epub run epub.package.nav.audit --input "$CUR" --json`，看 findings：

- 找到的问题多在「能做」清单：适合走清洗流水线。
- 找到的问题多在「不能做」清单：不要走，回到源头。
- 一半一半：先做能做的部分，剩下的另开方案。
