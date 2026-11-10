---
name: epub-audit
description: 只读审查已有 EPUB：包结构与导航、排版风险、文本结构角色、图片版式、字体覆盖。用于清洗前后的预检与复检；不修书，静态结果不等于阅读器验收。
---

# EPUB 只读审查

## 何时用

所有能力的公共返回、授权边界与验收说明见[技能索引](../README.md)。

- 包结构与导航：已有 EPUB 的第一步预检，以及迁移、打包、资源增删改名后的复核。规则见 [SPEC](../../docs/final/SPEC-实现约束.md) §5、§5.8、§10。
- 排版审稿：排版 review、候选比较或问题尚未分类时；新输入先做 package/nav 预检。区分“只审查”与“审查并修复”，已明确的范围不重复确认。
- 文本角色分析：需要确定性逐块证据辅助语义判断时。确认角色后的结构调整交 `epub-special-layout`。
- 图片版式分析：图片过小、裁切、环绕或图注异常时。改前读 [SPEC §5.1、§5.6、§5.10.1](../../docs/final/SPEC-实现约束.md)；章首图转 `epub-special-layout`，全页封面式布局转 A-lite，换封面文件用 `epub-package-ops`。
- 字体覆盖分析：先区分“字体没有字形”与“有字形但回退未到达”。字体链修改交 `epub-cleanup`；边界按 [SPEC §4、§8](../../docs/final/SPEC-实现约束.md)。CLI 调用已配置 provider，发行包不自带它，缺失时不能宣称检测完成。

## 调什么

四个能力都只读，不传 `--output`；参数以 `epub capabilities --id <id>` 为准。

```sh
epub run epub.package.nav.audit --input "book.epub" --json
epub run epub.text.content.analyze --input "book.epub" --json
epub run epub.image.layout.optimize --input "book.epub" --json
epub run epub.font.coverage.analyze --input "book.epub" --json
```

- 排版审稿沿用 nav audit 报告，不重复运行；还须阅读实际改动的 XHTML/CSS/OPF 与资源，扫描不代替语义和视觉 review。兼容结论核对 [reader matrix](../../docs/final/reader-matrix.yaml)。
- 文本分析的 `include_snippets=true` 仅用于本地复核，报告放书级 `.pipeline/`，不把正文写进仓库级 records。
- 字体覆盖的 `profile=ideal-browser|kindle-pessimistic` 默认后者；profile 是模型，不是实际设备测试。

## 返回怎么读

- `actionableFindings[].autoFixable` 只表示机器可判定，不扩大修改授权。`audit.<序号>` 不是稳定分类，用 `detail` / `location` / `kind` 定位。
- 文本分析的 `content.analysis-failed` 无有效分析，`content.source-error` 分析不完整，`content.review-required` 需上下文复核。
- 图片分析的计数不等于错误数，也不是批量改写清单；`warningList` 是扫描缺口，有它则结论不完整。
- 字体覆盖的 `status`（pass/warn/fail）区别于信封 status；`fontcoverage.fail/risk` 是覆盖问题，`fontcoverage.adapter` 是检测器失败，此时无有效覆盖结论。明细（`charInventory`、`chainHealth`、`unresolved`、`textRuns`）结合位置、CSS 继承和字体链查看。

## 依据返回怎么判断

### 包结构与导航

- DRM/未知加密、不可读 ZIP/container/OPF 按根 AGENTS 停止；可修复的版本、properties、导航诊断可进入修复，再验证产物，不要求先手工清零。
- 路径混淆 → `epub-cleanup`（`epub.structure.normalize`）；EPUB2/legacy → `epub-cleanup`（`epub.package.migrate.epub3`）；注释 → `epub.notes.popup.normalize`；合并/拆分/换封面 → `epub-package-ops`。不因需要审计就自动执行写操作。
- `css-root-font-size` error 与 `css-fixed-font-size` warn 是 P1 功能问题（部分阅读器无法调字号），交 `epub-cleanup`；`css-body-font-size` 只是 info，不单独派修。`css-page-color-lock` warn 是夜间/护眼主题对比度风险，须人工判断是否保留，交 `epub-cleanup`。见 SPEC §5.5.1、§7。
- 整本固定版式（SPEC §5.13）：只被固定版式页面引用的样式表不报三项字号 finding。`fxl-viewport-missing`、`fxl-viewport-size-missing` 是 error，补每页 `viewport` 的 width/height；`fxl-aspect-ratio-mixed` warn 核对各页尺寸；`page-spread-on-reflowable` warn 去掉可重排项上的 `page-spread-*`。整本固定版式的 page-list 指向页面文件，不报缺页码标记。
- 不删除 spine 页面来掩盖错误，不猜 `dc:language`，不批量删除字体 metadata 或未识别资源。CSS 字体断链保留声明与 `local()`，不猜替代字体；非字体断链须修复。
- Kindle/legacy 交付保留 NCX；目录层次问题按需读 [文集导航](../../docs/how-to/anthology-navigation.md)。修复保持现有 id 与 mixed-content 正文；有授权删除时记录精确清单及红线差异。

### 排版审稿

- 优先级：P0 损坏/不可读；P1 裁切、注释失联等功能/兼容问题；P2 间距、字体、fallback；P3 清理与一致性。每项给位置、证据、影响和最小修复。
- “好看”需结合书型、页面角色和已有设计：先挑代表性正文、章首、复杂页做候选样例，再决定能否推广。已明确的局部问题不扩大成全书重构。
- 不改正文、章节顺序或图片来规避版式问题。红线失败保留候选，不自动回滚用户改动。未看真实阅读器时只报告静态风险及待验项。

### 文本角色分析

显式语义/tag/class 优先，相邻块关系其次，引号、长度和关键词仅作候选；无 warning 也不是语义正确证明。诗与副标题、古文与普通短段等歧义须看前后文，无法确定保留原结构。字体建议是 `inherit/st/kt/fs/ht/en/mono/tszt-*` 角色，不要求嵌入特定字体。不因分类修改标点、空格或章节顺序。

### 图片版式分析

- 先确认正文图、封面、图标、公式、章首或背景角色。noteref 图片是交互控件，扫描已排除，不当 figure。
- 图文关系未确认时不自动加左右浮动。需要环绕时见 [shape-wrap 配方](../../templates/recipes/shape-wrap/recipe.md)；保留图注、alt、文字和资源，不为整齐裁掉内容。Kindle 风险 SVG/WebP 先确认转码范围。
- 手工修改后跑全项红线和 package audit，并用足够长的周围正文验证普通/大字号及窄屏。
- 图在页底缺笔画而源图完整时，先排除旧导入包，再查分页与容器边距；不以扫描零发现证明无裁切，不先裁图、重采样或给全书统一缩放。案例见[匿名适配复盘](../../docs/pipeline/reader-adaptation-lessons-2026-10.md)。

### 字体覆盖分析

- true-missing → 查可用字库；fallback-not-reached → 改链或局部专用类，不先造字；subset-cut → 查子集字符清单；only-non-embedded → 依赖目标阅读器验证。
- unresolved 非零必须标记分析缺口；系统字体名不能证明 cmap 覆盖，悲观 profile 不能写成 Kindle fail/pass。
- 有字但样式选错（如正文误用斜体，查 `@font-face` 族/字重/样式与阅读器映射）与缺字分开诊断，覆盖分析通过不能排除。
- 分析器不采集 CSS `quotes/content` 生成字符：单独枚举，子集写出后复查 cmap。不得把形似码位（如 〇/○）互换。
- 锁定正文须覆盖最终解析到该角色的全部文字/标点，不只扫 p；少量补字用局部类。修改后重跑覆盖与全项红线。
