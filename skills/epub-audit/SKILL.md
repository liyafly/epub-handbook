---
name: epub-audit
description: 只读审查已有 EPUB：包结构与导航、排版风险、文本结构角色、图片版式、字体覆盖。用于清洗前后的预检与复检；不修书，静态结果不等于阅读器验收。
---

# EPUB 只读审查

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### 包结构与导航

已有 EPUB 的第一步预检，以及迁移、打包、资源增删改名后的复核。包规则见 [SPEC](../../docs/final/SPEC-实现约束.md) §5、§5.8、§10。

### 排版审稿

排版 review、候选比较或问题尚未分类时使用；新输入先做 package/nav 预检。按用户请求区分“只审查”与“审查并修复”，不重复确认已明确的范围。

### 文本角色分析

需要确定性逐块证据辅助语义判断时使用。字体缺字用 coverage skill，确认角色后的结构调整用 literary skill。

### 图片版式分析

图片过小、裁切、环绕或图注异常时。改前读 [SPEC §5.1、§5.6、§5.10.1](../../docs/final/SPEC-实现约束.md)；章首图转 literary skill，全页封面式布局转 A-lite，换封面文件用 package operator。

### 字体覆盖分析

先区分“字体没有字形”与“有字形但回退未到达”。字体链修改交 typography skill，角色不明先 content analyzer。按 [SPEC §4、§8](../../docs/final/SPEC-实现约束.md) 判断覆盖边界；CLI 自动调用已配置 provider，但发行包不自带它，缺失时不能宣称检测完成。

## 调什么

### 包结构与导航

```sh
epub run epub.package.nav.audit --input "book.epub" --json
```

只读，不传 `--output`。修复后对新产物重跑。

### 排版审稿

```sh
epub run epub.layout.audit --input "book.epub" --json
```

只读，无额外参数。还须阅读实际改动的 XHTML/CSS/OPF 与相关资源；扫描不代替语义和视觉 review。按问题读取 [SPEC](../../docs/final/SPEC-实现约束.md) 对应章节，兼容结论核对 [reader matrix](../../docs/final/reader-matrix.yaml)。

### 文本角色分析

```sh
epub run epub.text.content.analyze --input "book.epub" --json
```

只读。`include_snippets=true` 仅用于本地复核，报告放书级 `.pipeline/`，不把正文写进仓库级 records。
裸片段可用 `source_name=` + `source_content=`，但当前仍需有效 EPUB 作为 `--input` 锚点；纯源材料先用 source-intake。

### 图片版式分析

```sh
epub run epub.image.layout.optimize --input "book.epub" --json
```

这是只读扫描，无 `--output`；压缩、色彩转换和转码使用外部工具，完成后再复查资源与版式。

### 字体覆盖分析

```sh
epub run epub.font.coverage.analyze --input "book.epub" --json
```

只读，可选 `profile=ideal-browser|kindle-pessimistic`，默认后者；profile 是模型，不是实际设备测试。

## 返回怎么读

### 包结构与导航

- 前缀 `epub.package.nav.audit.`：`summary`（OPF、资源/spine 计数、版本/语言）、`auditStatus`、`findingsByLevel`、`recommendedSkills`、`actionableFindings`。
- `actionableFindings[]` 包含 `kind/file/locator/params/lane/autoFixable/confidence/evidence`；`autoFixable` 只表示机器可判定，不扩大修改授权。
- `audit.<序号>` 不是稳定问题分类，用 `detail` / `location` 或结构化 `kind` 定位。`nextCommands` 随发现变化；`toolAvailability` 当前只探测 EPUBCheck。

### 排版审稿

前缀 `epub.layout.audit.`：`summary`、`auditStatus`、`findingsByLevel`、`recommendedSkills`、`actionableFindings`。后者给出位置、证据、置信度与 `autoFixable`；可自动修复标记不等于用户已授权。当前 `toolAvailability` 仅探测 EPUBCheck。

### 文本角色分析

- 前缀 `epub.text.content.analyze.`：`blocks`、`review_required`、`roles`、`blockList[]`、`sourceErrors`。
- `blockList[]` 每项含 source、locator、primary_role、candidate_roles、confidence、review_required、evidence、typography；`sourceErrors` 列出未解析文件。
- `content.analysis-failed` 无有效分析；`content.source-error` 分析不完整；`content.review-required` 需上下文复核。

### 图片版式分析

前缀 `epub.image.layout.optimize.`：`imageFindings[]` 给出 file、selector、image、scene、finding、candidates；`warningList` 是扫描缺口；`findings/warnings` 是计数。
候选类别包括 lone-image-no-figure、caption-detached、float-width-risk、missing-alt、chapter-head-image-candidate、fullpage-image-alite-candidate。计数不等于错误数，也不是批量改写清单；

### 字体覆盖分析

- 前缀 `epub.font.coverage.analyze.`：`profile`、`status`（pass/warn/fail，区别于信封 status）、`summary`。
- 同一前缀下的 `charInventory`、`chainHealth`、`unresolved`、`textRuns` 是明细（如 `facts["epub.font.coverage.analyze.charInventory"]`）；结合位置、CSS 继承和字体链查看。
- `fontcoverage.fail/risk` 是覆盖问题；`fontcoverage.adapter` 是检测器失败，此时无有效覆盖结论。provider 成功返回报告后才可能有 `detectorExitCode/Stderr`。

## 依据返回怎么判断

### 包结构与导航

- DRM/未知加密、不可读 ZIP/container/OPF 按根 AGENTS 停止；可修复的版本、properties、导航诊断可进入对应修复，再验证产物，不要求把所有输入错误先手工清零。
- 路径混淆 → structure normalizer；EPUB2/legacy → migrator；注释 → popup skill；包合并/拆分/换封面 → package operator。不要因需要审计就自动执行写操作。
- 不删除 spine 页面来掩盖错误，不猜 `dc:language`，不批量删除字体 metadata 或未识别资源。CSS 字体断链保留声明与 `local()`，不猜替代字体；非字体断链须修复。
- 检查唯一 nav、spine 顺序、引用 fragment、MathML/SVG properties 和封面声明；Kindle/legacy 交付保留 NCX。目录层次问题按需读 [文集导航](../../docs/how-to/anthology-navigation.md)。
- 修复保持现有 id 与 mixed-content 正文，不能依靠浏览器 HTML 容错。新增/删除资源同步 manifest、spine 和导航的实际依赖；有授权删除时记录精确清单及红线差异。

### 排版审稿

- 优先级：P0 损坏/不可读；P1 裁切、注释失联等功能/兼容问题；P2 间距、字体、fallback；P3 清理与一致性。每项给位置、证据、影响和最小修复，避免笼统“优化一下”。
- 用 `recommendedSkills` 与 [索引](../README.md) 选择最窄分支；已明确的局部问题不扩大成全书重构。
- “好看”需结合书型、页面角色和已有设计：先挑代表性正文、章首、复杂页做候选样例，检查层次、节奏、留白、字体一致性及大字号/窄屏，再决定能否推广。
- 不改正文、章节顺序或图片来规避版式问题。红线失败保留候选，定位后修复；不自动回滚用户改动。未看真实阅读器时只报告静态风险及待验项。

### 文本角色分析

显式语义/tag/class 优先，相邻块关系其次，引号、长度和关键词仅作候选。即使无 warning 也不能当成语义正确证明。诗与副标题、古文与普通短段等歧义须看前后文；无法确定保留原结构。
字体建议是 `inherit/st/kt/fs/ht/en/mono/tszt-*` 角色，不要求嵌入特定字体；普通正文默认 inherit。仅将确认结论交专项 skill，不因分类修改标点、空格或章节顺序。

### 图片版式分析

- 先确认正文图、封面、图标、公式、章首或背景角色。noteref 图片是交互控件，扫描已排除，不把它包装成 figure。
- 图文关系未确认时不自动加左右浮动。需要环绕时用 `figure.img-left/right` 承载 float 与百分比宽度，内图 `width:100%; height:auto`；25%–35% 只是实测起点。
- 保留图注、alt、文字和资源；不为整齐裁掉内容。JPEG/PNG 为生产主路径，Kindle 风险 SVG/WebP 先确认转码范围。
- 有扫描 warning 则结论不完整；手工修改后跑全项红线和 package audit，并用足够长的周围正文验证普通/大字号及窄屏，短段落不足以证明环绕失败。

### 字体覆盖分析

- true-missing → 查可用字库；fallback-not-reached → 改链或局部专用类，不先造字；subset-cut → 查子集字符清单；only-non-embedded → 依赖目标阅读器验证。
- unresolved 非零必须标记分析缺口；系统字体名不能证明 cmap 覆盖，悲观 profile 不能写成 Kindle fail/pass。
- 分析器不采集 CSS `quotes/content` 生成字符：单独枚举，子集写出后复查 cmap。不得把形似码位（如 〇/○）互换。
- 普通正文默认自由；少量补字用局部类，锁定正文须覆盖最终解析到该角色的全部文字/标点，不只扫 p。修改后重跑覆盖与全项红线。
