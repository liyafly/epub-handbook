---
name: epub-typography-optimizer
description: 优化中文/CJK EPUB 排版，包括正文节奏、font-family 链、嵌入字体、生僻字 fallback、标题字体、中西文混排和阅读器安全段落间距。用于文字拥挤、裁切、跨阅读器字体不一致，或按排版 preset 批量应用样式时。
---

# EPUB 字体与正文节奏优化

## 何时用

- 处理 CJK 阅读舒适度和字体兼容性，或按 preset 批量应用排版样式。不要把它和弹注转换、图片环绕、A-lite 海报混在一起；英文为主的 EPUB 使用 `epub-english-typography-optimizer`，不把英文小说套用中文 2em 缩进和 CJK 字体链。
- preset 三选一：`literary-cn`（默认）、`academic-cn`、`classical-annotated-cn`；preset 定义样式层（layers）与注释策略，能力负责把层写入 `Styles/`、更新 XHTML `<link>`、同步 OPF manifest 并报告覆盖度。
- 固定目标（阅读器安全）：
  - 普通正文默认自由，不在 `body` / 普通 `p` 上声明字体；需要显式字体的角色使用短跨平台系统链；默认 `font-family` 链最多 4 段：Apple、Windows、Android/开源 CJK、generic。
  - 稳定的全书结构角色可直接绑定字体；混合角色和局部例外使用角色类；生僻字子集用 `.rare` 等专用类；设计字体用 `.title-tszt`、`.signature-tszt` 等专用类。
  - 正文分两种模式，由 body 是否带 `font-family` 区分——自由模式（默认）：body 与普通正文 p 都不设 `font-family`，给支持用户字体选择的阅读器留切换空间，「自由」不等于包内不能有嵌入字体；锁定模式：`fonts.css` 直接给 `body` 提供字体链，OPF 加 `<meta property="ibooks:specified-fonts">true</meta>`。
  - 裸 `p { font-family:... }` 不作为全书锁定入口：它遗漏列表、引用、表格等正文容器，也会误伤注释段落；只锁定某类正文段落时用 `.bodytext`、`.prose` 等明确角色类。
  - 嵌入字体按 SPEC §8 三种模式选择：模式 A 设计字体角色（链为嵌入字体 + generic）；模式 B 生僻字子集 `.rare`（嵌入字体 + generic）；模式 C 嵌入 + 系统复合链（链最多 5 段，嵌入字体只出现一次）。设计上必须锁定正文或确有生僻字，且打包字体覆盖最终解析到正文角色的全部文字和标点时，才可走 C1-body；覆盖清单按 CSS 继承与局部角色覆盖计算，不能只扫描普通 `p`。只含少数字符的局部补字子集不能走 C1-body，只允许通过 `.rare` 等显式类包住需要补字的字符。
- 正文节奏清单：body line-height 在放大字号时仍舒适；段首缩进只作用于普通正文，不作用于标题、表格、代码、注释或图注；长 URL、标识符、英文文件名需要换行保护；标题分页控制不在短章制造空白页；代码块和表格大字号下仍可读；Ruby 行高给注音留空间。
- 禁止事项：
  - 不把版权字体放进模板或示例；不把多个嵌入字体塞进默认链解决生僻字。
  - 新建字体 alias、文件名与 class 遵循 `docs/final/字体别名命名规范.md`；不另造书名型、品牌型或重复角色名。
  - 有稳定英文 family/PostScript 名时不依赖中文字体显示名；不删除 generic fallback；没有明确阅读器 bug 时不在阅读字体上滥用 `!important`；不把正文做成整页图片。

## 调什么

```sh
# 1) dry-run：只算 coverage 与计划，不写输出
epub run epub.typography.optimize --input <书> --output <新书> --dry-run --json

# 2) 确认后实跑（默认 literary-cn）
epub run epub.typography.optimize --input <书> --output <新书> --json

# 换 preset：
epub run epub.typography.optimize --input <书> --output <新书> --json preset=academic-cn
```

可选 KEY=VALUE：`preset=literary-cn|academic-cn|classical-annotated-cn`（缺省 `literary-cn`）；`preset_dir=<目录>` 仅自定义 preset 库时使用。写型能力：`--output` 必填且指向新文件。需要旧报告形状明细时加 `legacy_report=true`。

改后校验：

```sh
epub redline --check all <before.epub> <after.epub>
epub run epub.font.coverage.analyze --input <新书> --json
```

规则位置也有问题时配合 `epub-css-layering-optimizer`；OPF 只声明实际使用的字体文件。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（缺 `--output`、输出与输入相同、输出已存在、preset 不存在）。
- facts 键前缀 `epub.typography.optimize.`：
  - `preset`、`layers`、`notes`：本次 preset 名、其样式层清单与注释策略。
  - `coverage`：类覆盖度——`usedClasses`（XHTML 用到的 preset 类）、`coveredClasses`（样式表已覆盖的类）、`ratio`、`threshold`，有 `warning` 时表示低于阈值。
  - `stylesheets`、`xhtmlLinks`：涉及的样式表与 `<link>` 数。
  - `manifestItemsAdded`（实跑后）：为样式表新增的 manifest 条目数。
  - `dryRun`：是否 dry-run。
- findings：`warn typography.low-coverage`（覆盖度低于阈值）；run 内置红线失败时出现 `error redline.<check>`（text/anchors）。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（preset apply 报告）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- dry-run `status == approval-required`（退出码 2）→ 先读 `coverage`：`ratio` 低于 `threshold` 或出现 `warning` 说明书内结构与 preset 假设不符，先人工核对角色类再实跑；实跑后仍出现 `warn typography.low-coverage` 时，逐类核对 `usedClasses`/`coveredClasses` 差集，决定补样式还是接受差异。
- 字体归属纪律（人工调整或补样式时）：`@font-face`、只含字体声明的稳定角色选择器和字体工具类移入 `fonts.css`；使用 `aside[epub|type~="footnote"]` 时先声明 `@namespace epub "http://www.idpf.org/2007/ops";`（紧跟可选 `@charset`/`@import`，早于 `@font-face` 和普通规则）；注释字体写在 `fonts.css`，`notes.css` 只保留注释结构与视觉；正文节奏（缩进、行高、标题 margin、blockquote/列表节奏、长 token 换行保护）留在 `base.css`。
- 保持既有书的字体模式：body 与普通正文 p 都无 `font-family` 时视为自由模式，不要替它加直接 body 规则或 `ibooks:specified-fonts`；已锁定的书保持锁定，并检查 body 规则与 OPF meta 成对出现；历史 `body-font-locked` 只兼容保留。局部角色字体不需要 `ibooks:specified-fonts`。
- 同时交付正文自由版与锁定版时，从同一内容基线派生；除字体 CSS、字体资源、OPF 字体 manifest/meta（含 `ibooks:specified-fonts`）及其所需 package `ibooks:` prefix 声明外，XHTML、spine、注释、图片与其他资源必须一致；各 rendition 唯一的 `dcterms:modified` 可反映打包时间，比较时只忽略值，不忽略缺失、多份或格式错误。既有书经用户明确要求在自由模式保留 `ibooks:specified-fonts=true` 的，把理由记入书级报告，不把例外写回模板。
- 删除同一链里重复的同平台别名；`fontspec=forceAll` 打包的字体必须覆盖最终解析到正文角色的全部文字和标点，链仍要短，并在文档或构建元数据中说明策略。
- `findings` 出现 `error redline.*` → 停止：输出保留供人工 diff review，先修源再重跑；`status == approval-required`（非 dry-run 场景出现）→ 停下来问人。
- 验证 fixture（改 demo 模板时对照）：`Text/01-body.xhtml` 普通正文与强调、`Text/07-font-family-order.xhtml` font-family 顺序、`Text/08-long-mixed-flow.xhtml` 裁切/长 token/中西文混排、`Text/10-text-effects.xhtml` Ruby 与文字效果；demo 构建与验证由 `epub-style-demo-maintainer` 处理。
