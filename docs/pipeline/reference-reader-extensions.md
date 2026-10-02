# 多看长屏封面与 Reeden 阅读器扩展参考

> 核对日期：2026-10-02。状态：本地 EPUB 结构分析、用户现象记录与官方文档对照；阅读器行为尚待独立实测。
>
> 本文用于识别存量书的扩展资源和选择后续验证场景。它不新增实现约束，也不代替 [SPEC-实现约束](../final/SPEC-实现约束.md) 或 [阅读器实测矩阵](../final/reader-matrix.yaml)。第三方来源和许可边界见 [来源说明](../../references/reeden-extensions.md) 与 [THIRD_PARTY.md](../../THIRD_PARTY.md)。

## 1. 结论与证据等级

匿名本地样本 `local-cover-sample-01` 同时打包普通封面 `cover.png` 与长屏图片 `cover~slim.png`。标准页面引用和 EPUB2 封面元数据都指向普通图；封面 spine 项另带 `duokan-page-fullscreen`。用户报告多看显示长屏图，其他阅读器显示普通图。

这支持“多看整屏图片路径通过 `~slim` 文件名关联备用图”的机制推断。**本次没有找到说明 `~slim` 选择算法的官方文档，也没有做移除标记、改名或换设备的对照实验；不能把它写成已验证的通用制作规则。** 上一轮讨论中的两图命名做法应作为复现实验候选保存。

Reeden 的官方文档明确列出多看标记的兼容别名，包括整屏图片、弹注、单图题注、画廊和出血。这说明这些名称并不必然只由多看识别；是否表现一致仍需按阅读器、版本、模式和具体产物实测。[Reeden 扩展全集](https://docs.reeden.app/ebook_spec_compatibility)

| 证据 | 本次已取得的内容 | 能支持的结论 |
| --- | --- | --- |
| 包内结构 | OPF、封面 XHTML、资源尺寸、引用扫描与 SHA-256 | 两张图如何登记；标准路径明确引用哪张图 |
| 用户观察 | 多看显示 slim 图，其他阅读器显示普通图；未附版本、设备和截图 | 记录现象，支持提出文件名关联假设 |
| 官方文档 | Reeden 0.1 Draft 扩展说明与兼容名称 | 上游宣称的语法和行为；不等于本仓 fixture 验收 |
| 未取得 | 多看文件名规则、长宽比阈值、跨版本对照、Reeden 对 slim 的支持证据 | 不作兼容 pass，不推断所有设备都会选择 slim |

## 2. 本地封面样本

输入身份见 [THIRD_PARTY.md](../../THIRD_PARTY.md) 的 `local-cover-sample-01` 条目；不记录具体书名、源文件名或作者信息。只读检查的 EPUB SHA-256 为：

```text
619ba5fbd51b9b3b4a34355427c70ad37bd3ee814c75966a863dcc427a074304
```

| 包内资源 | 已核对事实 |
| --- | --- |
| `OEBPS/content.opf` | `package version="2.0"`；普通封面 meta 指向 manifest ID `cover.png`；guide 指向 `Text/cover.xhtml` |
| `OEBPS/Text/cover.xhtml` | 内联 SVG 的 `viewBox="0 0 1080 1567"`；唯一 image 的 `xlink:href="../Images/cover.png"`；未加载外部 CSS |
| `OEBPS/Images/cover.png` | 1080 × 1567；892,643 字节；约 1:1.45 |
| `OEBPS/Images/cover~slim.png` | 1080 × 2400；1,251,783 字节；约 1:2.22 |

关键 OPF 片段如下，仅展示登记和关联方式：

```xml
<meta name="cover" content="cover.png"/>
<!-- manifest -->
<item id="cover.png" href="Images/cover.png" media-type="image/png"/>
<item id="cover.xhtml" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
<item id="coverslim.png" href="Images/cover~slim.png" media-type="image/png"/>
<!-- spine -->
<itemref idref="cover.xhtml" properties="duokan-page-fullscreen"/>
```

扫描包内 XHTML、HTML、CSS、SVG、OPF、XML 和 NCX 后，`~slim` / `coverslim` 只出现在上述 manifest 登记中。没有页面直接引用长图，没有媒体查询切换，也没有脚本切换。`coverslim.png` 是 manifest ID；可疑的关联约定位于实际文件名 `cover~slim.png`。

不实现该扩展的阅读器，按此 XHTML 的显式引用应加载 `cover.png`；实际尺寸、留白和分页仍由其 SVG/页面渲染决定。`duokan-page-fullscreen` 标记表达整屏页意图，本身既不指定 slim 资源路径，也不会产生一张长屏图片。

资源复核值：

| 成员 | SHA-256 |
| --- | --- |
| `OEBPS/content.opf` | `48e9795a1bcef4068b5b551ea8dac76f925254d5541bf5b33af6265d70a067b5` |
| `OEBPS/Text/cover.xhtml` | `e03bee7a0ad2575bed79c101782547dbe7afb432423cdab0632d11d268fa6950` |
| `OEBPS/Images/cover.png` | `6a033205ce55af7d03a72526e3932a111faa609da01810b5d8c4a652137c40a2` |
| `OEBPS/Images/cover~slim.png` | `14ed50ae1f4c04649f9b035eff29d7eefe804910182771c62cdc52215efa8ca5` |

## 3. Reeden 文档对我们的启发

### 3.1 保留标准显示路径，再验证阅读器增强

Reeden 扩展说明自标为 **0.1 Draft**，主张标准优先、正文不依赖脚本或网络、同一语义选择一个名称。对本仓的启发是先保证普通 XHTML、图片和链接可独立使用，再按发行目标验证增强；不因别名列表存在而批量堆叠所有 class 或 property。[电子书扩展](https://docs.reeden.app/ebook_spec)

Reeden 给出的 EPUB3 写法使用 `reeden:*` vocabulary，并在 OPF `package` 声明：

```xml
prefix="reeden: https://reeden.app/epub/vocab/#"
```

这是 Reeden 扩展词汇，不是 EPUB 标准整屏属性。旧书无前缀的 `duokan-page-*` 能被某些阅读器识别，也不等于它们属于 EPUB3 默认词汇。以后评估 EPUB3 候选时，分别验证语法合规和阅读器识别；前缀声明不能证明多看支持新的 Reeden 名称。[Reeden 全屏插图](https://docs.reeden.app/ebook_spec_fullscreen)、[W3C EPUB3 前缀机制](https://www.w3.org/TR/epub-33/#sec-prefix-attr)

### 3.2 将整屏显示、跨页裁切和图片选择分开核对

| Reeden 文档中的名称 | 上游文字描述 | 对本仓的启发 |
| --- | --- | --- |
| `reeden:page-fullscreen` | 整屏完整显示图片，必要时放大贴边，留白使用阅读主题底色 | 名称不能直接等同于 CSS `background-size:cover` 或四边无留白；检查文字边缘是否完整 |
| `reeden:page-fullscreen-spread` | 完整包含图片；小图保留原始尺寸 | 单独测试小图放大策略，不仅测试与屏幕大小接近的大图 |
| `reeden:page-fullscreen-spread-left` / `-right` | 显示跨页图的左侧或右侧区域 | 跨页区域展示与单张封面完整显示是不同场景 |
| `duokan-page-*`、`reeden-page-*` | 上述 prefixed 名称的兼容写法 | 只记录上游别名关系，不推导不同阅读器行为相同 |

上游将这些 property 放在 **spine/itemref**，`idref` 对应 manifest 中的 XHTML ID；页面仍保留一张主要图片，便于独立显示。这个分工有助于把“如何显示一页”与“选择哪张资源”分开排查。它没有解释 `~slim` 自动选择。[全屏插图](https://docs.reeden.app/ebook_spec_fullscreen)

本仓单图卷封已有 contain + 原图 fallback 路径，见 [SPEC §2](../final/SPEC-实现约束.md) 与 [scene 03c](../../templates/epub-style-demo/OEBPS/Text/03c-poster-contain.xhtml)。Reeden 文档提示可以增加对照实验，但当前不能用私有整屏模式替换通用 contain 主路径。

### 3.3 题图出血比全局负边距更值得做局部实验

Reeden 提供 `reeden-bleed`，兼容 `duokan-bleed`，用于题图、横幅或背景向指定边缘延伸。其文档说明：顶部效果与图片所在位置有关；局部宽度仍被尊重；双页时按物理页处理，不跨中缝。[页面出血](https://docs.reeden.app/ebook_spec_bleed)

对本仓 [scene 20](../../templates/epub-style-demo/OEBPS/Text/20-chapter-head-image.xhtml) / [scene 28](../../templates/epub-style-demo/OEBPS/Text/28-chapter-opening-block.xhtml) 的启发是：如目标书确有贴边题图需求，可以在图片容器上做局部增强对照，普通正文保留自身安全区。不从该扩展推导全书零边距，也不把顶部出血解释为自动把章节中部的图片搬到页首。

### 3.4 标准弹注与既有多看结构值得直接复测

Reeden 优先推荐 `epub:type="noteref"` / `footnote`，也支持集中式 `ol` / `li`，并把 `duokan-footnote`、`duokan-footnote-content`、`duokan-footnote-item` 列为兼容写法。它还提醒保持有效 href/id 关联，避免永久隐藏脚注内容。[富文本脚注](https://docs.reeden.app/ebook_spec_footnote)

本仓 [scene 02](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml) 和 [scene 05](../../templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml) 已提供标准语义与集中式 fallback；`notes.css` 未永久隐藏注释正文。最有价值的下一步是复测现有结构能否弹出、是否错配注释、能否回跳，而不是另造一份 Reeden 注释正文。

上游简例与本仓结构并不完全相同：本仓每章保留一个 aside，链接可指向其内部 li；上游标准简例直接指向 aside。这个差异需要实测，不能仅凭文档认定兼容。上游数字标记示例也不改变本仓图标主路径或正文校订授权要求，仍按 [SPEC §1、§10](../final/SPEC-实现约束.md) 执行。

### 3.5 单图题注和夜间字图有明确使用场景

| 扩展 | 文档事实 | 值得验证的用途与降级方式 |
| --- | --- | --- |
| 单图预览 | `reeden-image-single` 用于关联一张主图与题注；兼容 `duokan-image-single`、`duokan-image-maintitle`、`duokan-image-subtitle` | 地图、关系图、信息图预览能携带题注；普通阅读器仍能看到图片和真实题注。不能由文档推断任意 figure/figcaption 都被识别。[单图预览](https://docs.reeden.app/ebook_spec_image_single) |
| 夜间字图 | `reeden-dark-mode-invert: true` / `auto`，兼容 `dark-mode-invert`；声明作用于实际 img，适合透明背景单色字图 | 仅对确需图片的符号做日夜对照；彩色插画、照片和封面不列入统一反色。[夜间反色](https://docs.reeden.app/ebook_spec_dark_mode_invert) |
| 图片画廊 | 外层 gallery、直接子 cell、每帧图与题注；兼容多看 class | 如有图集目标再做候选；标准降级应按源序显示全部帧，不让正文依赖滑动才能出现。[图片画廊](https://docs.reeden.app/ebook_spec_image_gallery) |
| 扩展音频 | 标准 audio/source 叠加状态图和播放交互 | 当前封面与清洗任务无需引入；以后有朗读片段需求再检查控件与标准降级。[扩展音频](https://docs.reeden.app/ebook_spec_audio) |

这里的用途和降级方式是本仓研究建议，不是新增的能力或阅读器通过结论。尤其夜间符号应优先保留真文字或 MathML，不能为了反色效果把现有可编辑内容改成图片。

## 4. 对资源清理与改名的提醒

本样本提示：静态 XHTML/CSS 引用图不能穷尽阅读器可能采用的资源选择方式。`cover~slim.png` 仅在 manifest 出现，应记为“疑似由阅读器隐式关联”，不能仅凭缺少显式页面引用判定无用。

后续如评估工具或单书操作，可以分别检查以下风险；本次没有验证当前 CLI 是否存在这些问题，也没有修改实现：

| 操作 | 待核对风险 |
| --- | --- |
| 清理未引用图片 | 删除了阅读器通过文件名选择的备用图 |
| 结构规范化 / 改名 | 普通图与 `~slim` 的同名关系被破坏，显式 href 虽正确但私有路径失效 |
| 替换普通封面 | 长屏备用图仍是旧版，导致两个阅读器显示不同版本封面 |
| EPUB3 迁移 | 兼容 property 被丢弃，或未区分私有标记的语法与视觉行为 |

是否保留、改名或同步备用封面，需要单书决策与对照证据；本文不引入永久资源白名单，也不扩大封面修改授权。

## 5. 后续最小实验建议

以下是候选验证设计，**本次未生成这些 demo 或执行阅读器测试**。图像应使用自造、可区分的普通图和长图，例如大号 A / B 标识与四边刻度，避免把参考书图片复制到模板。

| 优先级 | 实验组 | 要回答的问题 |
| --- | --- | --- |
| P1 | “有 / 无 slim” × “有 / 无全屏标记”的四组对照；另加有标记但将 slim 改为无关联名称的一组 | 多看是否采用备用图；是否由标记与命名共同触发；单独控制两个变量 |
| P1 | 对同一组分别测试普通 img 与样本式内联 SVG | 多看识别单图时是否处理 SVG 包裹；不能只测 img 就外推样本行为 |
| P1 | 同一产物在多看、Reeden 和一个不依赖该扩展的标准阅读器中导入；覆盖手机长屏、平板与横屏 | 两图选择是否与平台、方向、比例相关；其他阅读器是否也识别扩展 |
| P2 | Reeden prefixed 整屏模式与多看旧标记分组；加入小图与边缘文字 | 放大、留白、跨页区域、单页分页分别如何表现 |
| P2 | 原样复测 scene 02 / 05、题图 scene 20 / 28，再做局部出血候选 | 既有弹注结构能否使用；局部增强是否影响正文安全区 |
| P3 | 自造地图题注、透明单色符号及日夜模式对照 | 放大题注是否关联正确；字图反色与普通降级是否可读 |

验收按 [demo README](../../templates/epub-style-demo/README.md)、[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 与根 [AGENTS.md](../../AGENTS.md) 的既有流程：冻结候选与 SHA，构建并审计，保留 redline 和 CI EPUBCheck 结果，再记录设备、阅读器完整版本、方向、字号、主题、测试副本 identity 与截图/日志。标准有效性、转换成功、真实阅读器行为分开报告。

具备可复核证据后才更新 reader-matrix；确认通用规则需要改变时，再同步 SPEC、手册、速查表和相关 skills。目前可吸收的是问题拆分、保留显式降级内容和最小对照实验方法；具体私有语法仍作为目标阅读器候选研究。

## 6. 当前已有、缺少与值得借鉴的部分

以下覆盖判断来自 2026-10-02 对活跃 demo、样式、相关能力实现和 reader-matrix 的静态对照。“已有”表示有结构或实现基础，不表示已经通过 Reeden 验收。当前矩阵没有 `reader: reeden` 的独立验收项；scene 28 的待测说明提到 Reeden，但不能代替该阅读器的测试记录。

| 主题 | 本仓已有基础 | 当前差距 | 建议 |
| --- | --- | --- | --- |
| [标准 CSS 与文字效果](https://docs.reeden.app/ebook_spec_compatibility) | `base.css` 的缩进；`effects.css` 的标准波浪线和装饰颜色 | 未有 Reeden 对这些 fixture 的独立证据 | 先测试标准规则，保留普通下划线降级；没有必要仅为 Reeden 换成私有波浪线名称 |
| [富文本弹注](https://docs.reeden.app/ebook_spec_footnote) | scene 02 / 05；`epub.notes.popup.normalize` 检查；`epub.notes.legacy-fallback` 为合规结构补多看 class | 本仓单 aside 内指向 li 的路径尚待 Reeden 实测；能力没有 Reeden 专项别名模式 | 优先验证既有标准与多看结构，不另建一份注释，也不将现有多看 class 全部改名 |
| [整屏图片 / 封面](https://docs.reeden.app/ebook_spec_fullscreen) | scene 03c 的 contain + 原图 fallback；A-lite 页面骨架 | 没有活跃的 Reeden 整屏 / 跨页 property 对照 fixture；没有 slim 选择证据 | 增加候选对照页，分别检查完整性、放大、留白和跨页区域；通用封面仍保留标准资源引用 |
| [题图出血](https://docs.reeden.app/ebook_spec_bleed) | scene 20 / 28 的题图、横幅和背景角色 | 未提供 `reeden-bleed` / `duokan-bleed` 专项样例 | 值得优先借鉴：仅对目标题图容器加局部贴边增强，正文边距继续独立管理 |
| [单图预览题注](https://docs.reeden.app/ebook_spec_image_single) | scene 17 的 figure / figcaption、图框尺寸和真实题注 | 没有预览题注识别用的 single / main-title / subtitle 标记样例 | 值得优先借鉴：试验在既有语义结构上挂识别 class；验证题注能否进入大图预览，不复制第二份题注 |
| [夜间单色字图](https://docs.reeden.app/ebook_spec_dark_mode_invert) | 标准图片和数学内容结构 | 没有字图角色的反色声明及日夜对照 fixture | 对已有透明单色符号图做可选局部增强；真文字 / MathML 优先；照片和彩色封面不统一反色 |
| [图片画廊](https://docs.reeden.app/ebook_spec_image_gallery) | 图文与并排图片结构 | 没有 gallery / cell 的逐帧交互样例 | 有具体图集需求再研究；当前并排图不因能做画廊就全部改为滑动交互 |
| [状态图音频](https://docs.reeden.app/ebook_spec_audio) | 本次检查的活跃 style demo 没有该扩展场景 | 没有状态图 / 控件 / 普通音频降级验证 | 低优先级，等有音频内容目标再补 |

我们目前最缺的是三类读者体验的识别与证据：局部题图贴边、大图预览保留题注、夜间小型符号图保持可读。弹注和通用图片排版已有基础，应先复测和补齐缺口。

这组 Reeden 扩展文档没有提供本仓字体角色、文白对照、诗歌 / 书信 / 对白、复杂章题、Ruby / 竖排和 MathML 排版的完整配方；这些场景继续参考本仓 SPEC 与 fixture。这里描述的是所阅文档的范围，不能推断 Reeden 应用不支持这些内容。

## 7. 适配逻辑的建议顺序

### 7.1 先确定效果由谁实现

缩进、行高、普通图片宽度等可以由 EPUB 的 CSS 表达。弹窗、携带题注的大图预览、逐帧画廊、阅读器主题下的字图反色及 spine 整屏处理，需要阅读器识别 HTML class、CSS 扩展属性或 OPF property。加入标记是在向阅读器提供信息，并不在书内实现同样的交互程序。[扩展总览](https://docs.reeden.app/ebook_spec)、[扩展全集](https://docs.reeden.app/ebook_spec_compatibility)

因此适配以页面角色和实际缺口为单位：一个题图容器、一张信息图、一类字图或一个注释列表；不全书套用阅读器标记。

### 7.2 复用标准结构与已存在的兼容别名

先让原样的 scene 02 / 05 / 03c / 17 / 20 在 Reeden 中运行，再比较效果。对多阅读器候选，现有标准弹注加 `duokan-footnote*` 已有上游兼容声明，应先验证该组合；不用同时给同一元素堆叠 Reeden 和多看同义 class。[富文本脚注](https://docs.reeden.app/ebook_spec_footnote)

单图题注可以先试把识别 class 挂在现有 figure / figcaption 上，让语义结构、源序和真实题注保持一致。上游展示的是 div / p，尚未证明 figure / figcaption 的组合也能识别；如实验失败，记录实际要求，再决定最小结构调整。[单图预览](https://docs.reeden.app/ebook_spec_image_single)

Reeden 专项目标的新 OPF 整屏候选可以采用带已声明前缀的 `reeden:*`。多看旧 property 单独作兼容对照，不能因上游给出别名就默认其无前缀写法通过 EPUB3 校验，也不能假定多看能识别 Reeden 新名称。候选中的扩展语法与正式通用发行包分别验证。[全屏插图](https://docs.reeden.app/ebook_spec_fullscreen)

### 7.3 按现有样式职责局部增强

题图出血归题图角色，单图和字图规则归图片角色，弹注归 `notes.css`，整屏页的 OPF 标记归对应 spine 项。沿用 [SPEC §7 的 CSS 分层](../final/SPEC-实现约束.md)，具体选择器只命中目标元素，加载范围只覆盖相关页面。

Reeden 的反色属性应直接命中实际 img，容器上的声明不会自动传给后代；顶部出血也不会把章节中部图片搬到页首。理解这两种作用范围，比只照抄属性名称更重要。[夜间反色](https://docs.reeden.app/ebook_spec_dark_mode_invert)、[页面出血](https://docs.reeden.app/ebook_spec_bleed)

官方夜间字图示例中出现 `height: 1 em`；数值与单位分离不是有效的该长度写法，制作候选时应使用 `1em`。这也说明上游示例需要语法核对，不能直接作为本仓通过的样式模板。[夜间反色示例](https://docs.reeden.app/ebook_spec_dark_mode_invert)

### 7.4 验证普通显示与增强显示，再决定是否共用一个包

从同一份内容源生成通用候选与局部增强候选，在 Reeden 与主要标准阅读器中对照。重点检查整图及边缘文字、图题配对、注释跳转 / 回跳、日夜可读性、横竖屏、单 / 双页和大字号。对画廊，普通路径仍应能按源序看到每一帧；对弹注，普通路径仍能访问同一份注释。

增强候选若在其他发行目标中保持稳定降级，才评估合并到通用包；若产生裁切、重复内容或分页问题，保留目标版本与通用版本两个明确候选，不让目标阅读器的需要扩散为全书基础样式。

### 7.5 工具化放在行为确认之后

当前能力可以校验标准弹注、补多看 fallback、给图片布局建议；没有可直接调用的 Reeden 专项预设或自动适配能力。本文不新增假定存在的参数或命令。

后续如进入实现，先把已复现问题与通过候选冻结为自造 fixture，再评估：审计是否能提示正确的挂载位置；清理 / 改名 / 封面替换是否保留已确认的资源关联；显式选择的增强是否需要最小字节范围编辑。由原样复测失败确定最小修复，不从网页别名表直接批量改书。

建议推进顺序是：**既有弹注和单图复测 → 题图出血与预览题注候选 → 整屏 / slim 对照 → 有实际字图需求时补日夜模式 → 有图集或音频需求时再扩展交互**。这是一项建议，尚未实施，也没有修改 SPEC 或阅读器通过状态。
