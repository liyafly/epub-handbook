---
name: epub-package-ops
description: 在明确授权后合并或拆分 EPUB、修改元数据、替换封面或转换 A-lite，输出候选并审查结构、资源和红线差异；不捎带正文或字体改写。
---

# EPUB 包操作

## 何时用

公共返回、授权边界与验收说明见[技能索引](../README.md)。

- 包操作：用户明确选择时；多个操作分开执行并分别验证。每个输入先预检，损坏/加密按根 AGENTS 停止，不靠删除资源绕过保护。
- A-lite 转换：明确需要全页封面式视觉时。先读目标 XHTML/CSS/OPF 与图片/字体，识别背景、叠字、竖列和版权页；规则读 [SPEC §2](../../docs/final/SPEC-实现约束.md)，对照[叠字 fixture](../../templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml)或[单图 fixture](../../templates/epub-style-demo/OEBPS/Text/03c-poster-contain.xhtml)。

## 调什么

参数以 `epub capabilities --id <id>` 为准。互斥示例，先 `--dry-run` 审查再执行：

```sh
epub run epub.package.merge --input "first.epub" --output "merged.epub" --json "extra_inputs=second.epub,third.epub"
epub run epub.package.split --input "book.epub" --json "output_dir=split-candidates" split_points=0,8
epub run epub.metadata.edit --input "book.epub" --output "candidate.epub" --json metadata_json='{"title":"新书名"}'
epub run epub.cover.replace --input "book.epub" --output "candidate.epub" --json "cover=cover.png"
epub run epub.alite.convert --input "before.epub" --output "candidate.epub" --dry-run --json [expect_volumes=N]
```

- split **不需 --output**，`output_dir` 必须是尚不存在的目录；切分点是 TOC 目标下标（无目录退化为 spine），不是页码，先核对实际目录。
- `metadata_json` 是内联对象、不是文件路径。丛书与排序名用 `series`、`series_position`（数字）、`title_sort`、`author_sort`；值不能为空。
- A-lite 已知卷数时 dry-run 与实跑都加 `expect_volumes=N`。

## 返回怎么读

- `package.refused` 按原因修前提，不清空用户目录来绕过保护。
- `metadata.edit`、`merge`、`cover.replace` 的内置红线会把授权字段变化、合并卷新增内容或封面替换记为 error（exit 1），但候选仍写出。按 [package-operations](../../docs/pipeline/package-operations.md)，用排除对应检查项的显式 redline 复核（metadata 排除 metadata，cover 排除 cover，merge 用 drm,anchors），其余检查项仍须通过。
- merge 的 `mappings` 仅对应首个 `--input`；`sourceMappings[]` 以 inputIndex/input 区分每卷完整映射。
- A-lite：`alite.no-copyright` 表示未找到相邻版权页，不表示应凭空补一页。

## 依据返回怎么判断

### 包操作

- 每个新产物跑 package audit，检查 OPF/nav/NCX、封面、正文顺序和段边界；SHA、理由与差异记入制作说明。
- 元数据/封面/合并拆分有授权变化，全项 redline 按项解释，不能用普通排版的“全部不变”作验收口径。
- merge 逐输入核对内容、资源与顺序，不声称一次两文件红线覆盖所有输入。
- split 对同一 `@font-face src` 内有有效非空 `local()` 的缺失字体 URL 保留声明并继续，只记 `split.font-local-fallback` 事件；系统字体是否可用仍须实测。无此回退的字体、图片/导入 CSS 断链仍拒绝。split 必须核对所有分段合起来的内容、边界与导航，原书与某一段必然有范围差异；检查新增导航 spine 项是否为预期非正文项。
- 换封面只接受已授权的封面/引用变化，metadata 只接受指定字段变化。门禁失败保留候选，不自动发布。
- 需要同时保留常规封面与 slim/多比例资源时，先核对替换计划、mappings 和真实 ZIP 成员；单封面替换可能清理被替代资源。Kindle 重复封面先查 cover-image/legacy cover、guide、spine 与封面页单主图是否一致；桌面显示主图不证明手机自动选中 slim。见 [10 月复盘](../../docs/pipeline/reader-adaptation-lessons-2026-10.md)。

### A-lite 转换

- 改写范围符合目标，保留文字、图片、嵌入字体和大体视觉顺序，不重新设计构图或新增装饰。
- 使用 body.fullpage + section.fullframe；背景放 poster modifier，不放 shell；完整 CSS 复用 fixture，不再维护一套参数。
- 单图卷封保留 poster-fallback 原图，背景 contain，不 cover 裁边或拉伸；已有叠字必须仍是真文本。
- 规则进 poster.css；不转 FXL，不用 absolute/vh/vw/padding-ratio 替代方案。
- 红线后仍查窄屏、大字号和目标阅读器的裁切、空白、文字丢失；实测规则变化走 `epub-reader-verify`。
