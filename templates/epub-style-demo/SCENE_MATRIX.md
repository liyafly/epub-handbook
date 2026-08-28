# EPUB Style Demo 场景矩阵

本矩阵是 demo EPUB 的执行清单。新增兼容性判断时，先补这里和对应 XHTML，再 build EPUB、跑阅读器验证，最后回写 `docs/final/reader-matrix.yaml` 与最终文档。

| 场景 | XHTML | 主要检查点 | 目标阅读器 |
|---|---|---|---|
| 封面式标题页 | `Text/00-title.xhtml` | 标题页分页、landmark cover、标题居中 | Apple Books / Kindle Previewer / Thorium |
| 普通正文 | `Text/01-body.xhtml` | 段落缩进、行高、引用、着重、图片 figure | 全部 |
| 标准弹注 | `Text/02-ruby-note.xhtml` | `noteref`、同文件 `aside`、回跳、Ruby 行距、图片脚注图标不抬高正文行距 | Apple Books / Thorium / KOReader |
| A-lite 海报 | `Text/03-vertical-alite.xhtml` | `body.fullpage`、`.fullframe padding:0`、背景、竖排标题 | Apple Books / Kindle Previewer |
| 单图卷封 contain 对照 | `Text/03c-poster-contain.xhtml` | `body.poster-bg-contain`、`background-size: contain`、`.poster-fallback`、单页不裁图 | Apple Books / Kindle App / Kindle Previewer |
| 表格与代码 | `Text/04-lists-tables-code.xhtml` | 列表、表格滚动、代码块、kbd | Kindle Previewer / KOReader / Thorium |
| 多看 fallback | `Text/05-legacy-note-fallback.xhtml` | `duokan-footnote`、`ol.duokan-footnote-content`、单注释 | 多看 / 标准阅读器回退 |
| font-family 顺序 + 正文锁定 | `Text/07-font-family-order.xhtml` | 全书正文锁定模式（`fonts.css` 直接 `body` 规则 + OPF `ibooks:specified-fonts=true`）、系统优先、书内优先、楷体混合链、生僻字 fallback | Apple Books / Windows 阅读器 |
| 长段落与中英混排 | `Text/08-long-mixed-flow.xhtml` | 普通正文盒模型、右侧裁切、长 token 换行、大字号标题 | Kindle Previewer / Apple Books |
| Kindle 风险项 | `Text/09-kindle-risk.xhtml` | cover metadata、nav + NCX、PNG、长串、表格、代码 | Kindle Previewer |
| 文字效果合集 | `Text/10-text-effects.xhtml` | `.emp` / `.wavy` / `.dropcap` / Ruby 行距 | 全部 |
| 整页正文竖排 | `Text/14-vertical-body.xhtml` | `body.page-vrl`、`.vrl-section` | Apple Books / KOReader |
| 前置页 | `Text/15-frontmatter.xhtml` | `epub:type="frontmatter copyright-page"`、连续 `p.cp` 保真转录、`dl` 标签/值增强与题献题记 | 全部 |
| 数学公式与 MathML | `Text/16-math.xhtml` | 分式、根式、上下标、矩阵、semantics/TeX annotation；presentation table 保守编号、Grid 增强、可换行方程组；`thead`/`tbody`/`th scope`/真实 `rowspan` 分组的固定布局长 MathML，局部候选相对字号 | Kindle App / Kindle Previewer / Readest / Apple Books / Thorium（新增数据表场景均待复测） |
| 图文环绕与图片尺寸 | `Text/17-image-layout.xhtml` | figure 浮动、25%–35% 百分比宽度、长正文阈值、短段反例、大字号回归；单图由 figure 控制实例宽度，按图框角色 34%/60% 非等宽并排 figure：默认上下、宽屏 Flex 增强、等高居中图像区，内部 img 等比 | Kindle App / Kindle Previewer / Readest / Apple Books / Thorium（新增尺寸场景均待复测） |
| 英文小说正文 | `Text/18-english-fiction.xhtml` | 英文短章标题、首段无缩进、后续段缩进、`::first-letter` 首字、手写体 float 下沉首字、居中插图、摘录、大字号回归 | Readest / Kindle Previewer / Apple Books / Thorium |
| 边框与阴影便签 | `Text/19-border-shadow-notes.xhtml` | solid/dashed/double/left-rule、box-shadow、inset、斜角感、SVG 花边实验、长条投影、不规则边缘、手剪纸边框 fallback | Readest / Kindle Previewer / Apple Books / Thorium |
| 章节头图设置 | `Text/20-chapter-head-image.xhtml` | 小型头图、满栏横幅头图、真实 h1、kicker/副标题、35% 单书 fallback、40% 复测增强类、大字号不裁切、横向不溢出 | Kindle Previewer / Apple Books / Thorium |
| 文白 / 原译对照 | `Text/21-classical-modern.xhtml` | 条目级 section、局部目录、样本式双文本段落、默认上下、短组 40em 以上双 float 增强；文白默认 38/58、原译接近 48/48、原文较长 58/38，单书可后加载覆盖；长组 `.parallel-stack-pair` 上下并允许分页、轻量回目录链接；必测字号 1/3/4/5/6/7，日夜模式，默认/Publisher Font/Bookerly 或 Original 字体；失败态必须上下，不能半宽错位 | Kindle Previewer / Kindle 设备 KFX / Kindle App / Apple Books / Readest / Thorium |
| 章题两列骨架：body background 饰图（待测） | `Text/22-chapter-title-bg.xhtml` | `文心` 系列行 + `『忽然做了大人与古人了』` / `一`；`display:table/table-cell` 的 5.8em 靠右紧凑骨架，题名左、章次右且同顶；窄列逐字下排，`@supports` 同时探测 standard/WebKit/EPUB `vertical-rl`；body background 左下约 25%，可见内容仅章题组件，页面 100% border-box 以观察单 spine 是否分第二页 | Apple Books / Reeden / Kindle Previewer |
| 章题两列骨架：inline absolute 饰图（待测） | `Text/23-chapter-title-inline.xhtml` | 与 22 页同一 `文心` 式 5.8em 章题骨架；inline `<img>` 缩至左下约 25% 并使用 `position:absolute`，不进入正常流、不单独占 spine 页；可见内容仅章题组件与饰图，根/section/body 100% border-box | Apple Books / Reeden / Kindle Previewer |
| 着重号比较：横排（待测） | `Text/24-text-emphasis.xhtml` | 原生标准/EPUB `under right`、原生 Kindle/WebKit `-webkit-...: under` 单值、逐字 ruby 字面圆点、逐字 ruby 空 `rt` + CSS generated dot；另有 `ruby-position: under` 风险对照；可换行短段 | Apple Books / Reeden / Kindle Previewer / Thorium |
| 着重号比较：body 真竖排（待测） | `Text/25-text-emphasis-vertical.xhtml` | `body.page-vrl` 真竖排中复现原生两种 position、`ruby-position: over` 主样本与 `under` 风险对照，记录圆点位置、行距和降级 | Apple Books / Reeden / Kindle Previewer / Thorium |

## 退役对照页

这些页面保留在 `templates/epub-style-demo/retired/` 供历史对照，不进入默认 EPUB 构建。若后续要恢复，必须重新接入 OPF/nav/toc、构建 artifact，并按 reader-matrix 流程复测。

| 场景 | XHTML | 退役原因 |
|---|---|---|
| 海报全幅对照 | `retired/03b-poster-fullbleed.xhtml` | 主路径改为 contain 单图卷封；全幅 cover 裁切风险只保留历史对照。 |
| 多条 fallback | `retired/06-multi-legacy-note-fallback.xhtml` | 单条 fallback 已覆盖结构规则；多条组合不再占用主 demo。 |
| 章首结构 | `retired/11-chapter-opening.xhtml` | 已由 `Text/20-chapter-head-image.xhtml` 和前置页场景吸收。 |
| 小说体综合 | `retired/12-literary-fiction.xhtml` | 已拆分到英文小说、文白对照、边框便签和前置页等更具体场景。 |
| 多看富文本 fallback | `retired/13-duokan-rich-fallback.xhtml` | 多看兼容主路径保留 `Text/05-legacy-note-fallback.xhtml`。 |

## 外部人工场景

以下场景需要仓库外的授权素材，不进入默认 demo EPUB。验证时应复制 demo 到临时工作区，加入本地授权资源，再把结论回写 `docs/final/reader-matrix.yaml`。

| 场景 | Fixture 形态 | 主要检查点 | 目标阅读器 |
|---|---|---|---|
| C1-body 正文角色完整覆盖 + 双版本 | 临时加入有明确授权的 `OEBPS/Fonts/st-all.ttf`、局部 `kt/fs` 角色、OPF font item、`@font-face` 和直接 body 锁定；从同一内容基线构建正文自由/锁定两版及原字体/子集字体对照 | 正文角色按继承覆盖全部实际文字与标点、`fontspec=forceAll`、两版除字体白名单外一致；核对 U+201C/U+201D、U+3007/U+25CB、CSS `quotes/content` 生成字符，默认字号和大字号均不裁切 | Apple Books / Kindle Previewer / Thorium |

## 打包与记录

```sh
sh templates/epub-style-demo/build.sh
```

验证完成后，把阅读器名称、版本、构建产物、失败页面、现象、状态和 workaround 写入 `docs/final/reader-matrix.yaml`。
