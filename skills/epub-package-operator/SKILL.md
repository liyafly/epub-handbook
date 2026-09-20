---
name: epub-package-operator
description: 按明确授权合并或拆分 EPUB、修改元数据、替换封面，输出新产物与报告。只审计时不用；操作改变的红线须逐项解释，不捎带正文或字体改写。
---

# EPUB 包操作

## 何时用

用户明确选择包操作时；多个操作分开执行并分别验证。每个输入先预检。损坏/加密按根 AGENTS 停止，不能靠删除资源绕过保护。

## 调什么

下列为互斥示例，先加 `--dry-run` 审查，再执行所选操作：

```sh
epub run epub.package.merge --input "first.epub" --output "merged.epub" --json "extra_inputs=second.epub,third.epub"
epub run epub.package.split --input "book.epub" --json "output_dir=split-candidates" split_points=0,8
epub run epub.metadata.edit --input "book.epub" --output "candidate.epub" --json metadata_json='{"title":"新书名"}'
epub run epub.cover.replace --input "book.epub" --output "candidate.epub" --json "cover=cover.png"
```

merge 至少两本，extra_inputs 逗号分隔，可选 title。split **不需 --output**，output_dir 必须是尚不存在的目录（dry-run 也需指定但不创建）；切分点是 TOC 目标下标（无目录退化为 spine），不是页码；先核对实际目录与范围。
metadata_json 是内联对象、不是文件路径，字段仅 title/subtitle/author/language/publisher/description/identifier/rights，值为字符串。cover 接受本地图片，Kindle 优先 JPEG/PNG，文件扩展名可接受不等于阅读器支持。

## 返回怎么读

各能力 id 为 facts 前缀：merge 看 inputs/mergedItems/renamedResources/warnings/mappings/sourceMappings；split 看 segmentPlans/plannedOutputs/outputs/segmentsCreated；metadata 看 fieldsUpdated；cover 看 coverPath/mappings。
`package.refused` 按原因修前提，不清空用户目录来绕过保护。完整公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 对每个新产物跑 package audit，检查 OPF/nav/NCX、封面、正文顺序和段边界；SHA、理由与差异记入书级制作说明。
- 全项 redline 仍运行，但元数据/封面/合并拆分有授权变化，按项解释，不能用普通排版的“全部不变”作验收口径。
- merge 的 `mappings` 仅对应首个 --input，可直接供两文件红线使用；`sourceMappings[]` 以 inputIndex/input 区分每卷的完整映射（含未改名项）。逐输入核对内容、资源与顺序，不声称一次两文件红线覆盖所有输入。
- split 对同一 `@font-face src` 内有有效非空 `local()` 的缺失字体 URL 保留声明并继续，只记录 `split.font-local-fallback` 事件，不产生 warning；系统字体是否可用仍须实测。无此回退的字体、图片/导入 CSS 等断链仍拒绝。
- split 必须核对所有分段合起来的内容、边界与导航；原书与某一段必然有范围差异，不能当一般正文不变比较。检查新增导航 spine 项是否为预期非正文项。
- 换封面只接受已授权的封面/引用变化，metadata 只接受指定字段变化；其他正文、字体、图片仍受保护。门禁失败保留候选，不自动发布。
