---
name: epub-package-ops
description: 在明确授权后合并或拆分 EPUB、修改元数据、替换封面或转换 A-lite，输出候选并审查结构、资源和红线差异；不捎带正文或字体改写。
---

# EPUB 包操作

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### EPUB 包操作

用户明确选择包操作时；多个操作分开执行并分别验证。每个输入先预检。损坏/加密按根 AGENTS 停止，不能靠删除资源绕过保护。

### A-lite 转换

明确需要全页封面式视觉时。先读取目标 XHTML/CSS/OPF 与图片/字体，识别背景、叠字、竖列和版权页；完整规则读 [SPEC §2](../../docs/final/SPEC-实现约束.md)，对照 [叠字 fixture](../../templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml) 或 [单图 fixture](../../templates/epub-style-demo/OEBPS/Text/03c-poster-contain.xhtml)。

## 调什么

### EPUB 包操作

下列为互斥示例，先加 `--dry-run` 审查，再执行所选操作：

```sh
epub run epub.package.merge --input "first.epub" --output "merged.epub" --json "extra_inputs=second.epub,third.epub"
epub run epub.package.split --input "book.epub" --json "output_dir=split-candidates" split_points=0,8
epub run epub.metadata.edit --input "book.epub" --output "candidate.epub" --json metadata_json='{"title":"新书名"}'
epub run epub.cover.replace --input "book.epub" --output "candidate.epub" --json "cover=cover.png"
```

merge 至少两本，extra_inputs 逗号分隔，可选 title。split **不需 --output**，output_dir 必须是尚不存在的目录（dry-run 也需指定但不创建）；切分点是 TOC 目标下标（无目录退化为 spine），不是页码；先核对实际目录与范围。
metadata_json 是内联对象、不是文件路径，字段仅 title/subtitle/author/language/publisher/description/identifier/rights，值为字符串。cover 接受本地图片，Kindle 优先 JPEG/PNG，文件扩展名可接受不等于阅读器支持。

### A-lite 转换

```sh
epub run epub.alite.convert --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.alite.convert --input "before.epub" --output "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

已知卷数时两次都加 `expect_volumes=N`。审查计划后写出，不覆盖原件。

## 返回怎么读

### EPUB 包操作

各能力 id 为 facts 前缀：merge 看 inputs/mergedItems/renamedResources/warnings/mappings/sourceMappings；split 看 segmentPlans/plannedOutputs/outputs/segmentsCreated；metadata 看 fieldsUpdated；cover 看 coverPath/mappings。
`package.refused` 按原因修前提，不清空用户目录来绕过保护。完整

### A-lite 转换

前缀 `epub.alite.convert.`：`posterPages/copyrightPages` 是改写路径，`posterPagesRefined/copyrightPagesRefined/stylesheetsAdded` 是计数，`warnings` 是需复核项。`alite.no-copyright` 表示未找到相邻版权页，不表示应凭空补一页。

## 依据返回怎么判断

### EPUB 包操作

- 对每个新产物跑 package audit，检查 OPF/nav/NCX、封面、正文顺序和段边界；SHA、理由与差异记入书级制作说明。
- 全项 redline 仍运行，但元数据/封面/合并拆分有授权变化，按项解释，不能用普通排版的“全部不变”作验收口径。
- merge 的 `mappings` 仅对应首个 --input，可直接供两文件红线使用；`sourceMappings[]` 以 inputIndex/input 区分每卷的完整映射（含未改名项）。逐输入核对内容、资源与顺序，不声称一次两文件红线覆盖所有输入。
- split 对同一 `@font-face src` 内有有效非空 `local()` 的缺失字体 URL 保留声明并继续，只记录 `split.font-local-fallback` 事件，不产生 warning；系统字体是否可用仍须实测。无此回退的字体、图片/导入 CSS 等断链仍拒绝。
- split 必须核对所有分段合起来的内容、边界与导航；原书与某一段必然有范围差异，不能当一般正文不变比较。检查新增导航 spine 项是否为预期非正文项。
- 换封面只接受已授权的封面/引用变化，metadata 只接受指定字段变化；其他正文、字体、图片仍受保护。门禁失败保留候选，不自动发布。

### A-lite 转换

- 核对改写范围符合目标，保留文字、图片、嵌入字体和大体视觉顺序，不重新设计构图或新增装饰。
- 使用 body.fullpage + section.fullframe；背景放 poster modifier，不放 shell；按 SPEC 保持 box-sizing、无骨架 padding、正确 overflow 与 writing-mode 前缀。完整 CSS 复用 fixture，不在 skill 再维护一套参数。
- 单图卷封保留 poster-fallback 原图，背景 contain，不能 cover 裁边或拉伸；已有叠字必须仍是真文本。
- 规则进 poster.css，只同步实际资源引用；不转 FXL，不用 absolute/vh/vw/padding-ratio 替代方案。
- 红线后仍检查窄屏、大字号和目标阅读器，特别关注裁切、空白、文字丢失；实测规则变化走 demo skill。
