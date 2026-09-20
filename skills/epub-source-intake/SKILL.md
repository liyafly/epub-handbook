---
name: epub-source-intake
description: 对非 EPUB 文本、Markdown、HTML、PDF、扫描件和图片做只读盘点，建立可审计 source bundle 计划；不解析 PDF/OCR、不压缩图片、不直接生成 EPUB。
---

# EPUB 源材料接入

## 何时用

没有成品 EPUB、需要先整理源材料时使用；已有 EPUB 直接预检。抽取、OCR、压缩由外部工具负责，工具名/版本/参数必须记录。OCR 与 PDF 抽取文本均是待校对来源，不是最终正文。工作区规则见 [book-workspace](../../docs/pipeline/book-workspace.md)。

## 调什么

```sh
epub run epub.source.intake --input "source directory" --json
```

输入为目录或普通文件，不接受设备/FIFO/socket。可选 `max_files=5000`（正整数）。只读，无 `--output`；dry-run 也不写文件、不要求批准。
跳过隐藏项与嵌套符号链接；输入根软链会解析一次。SHA-256 在遍历时计算，不改输入。

## 返回怎么读

- 前缀 `epub.source.intake.`：`files[]`（相对路径、role、size、sha256、risks）、`roleCounts`、`workspacePlan`、`plan`、`blockers`。
- `intake.empty` / `intake.too-many-files` 是失败；`intake.unreadable-file/dir` 即使配合 `complete` 也表示盘点不完整。未读到文件的 size=0/空 SHA 不等于空文件。
- PDF、图片格式、编码、嵌套压缩包、符号链接风险在 findings 与逐文件 risks 中。BOM/CRLF 是保真处理提示。
- 已有 EPUB 会产生 `intake.already-epub` 与最多 5 条预检建议；纯源材料的 `nextCommands` 可为空，这是正常情况。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 先解决读取失败、超限等缺口；只记录已核实范围，不把部分清单宣称为完整来源。
- 原样冻结入选原件及 SHA；抽取、转码、去 BOM/LF 统一只做派生副本，原件与派生物对应关系写入 `制作说明.md`。
- 根据材料类型结构化章节、段落、注释、图表、公式；抽样覆盖目录、章节边界及实际存在的脚注密集页/图表/公式页。保留阅读顺序与真实文本，不截图替代可重排正文，不自动校改作者文字。
- 无可结构化正文时说明缺口并确认来源；嵌套包解包或外部 OCR/转码后重新盘点。图片派生物再交图片/package skill 验证。
- 形成 EPUB 后先预检，再判断是否需要 normalize/迁移及精排。`plan` 是建议，不是必须执行全部步骤的授权；核对 `nextCommands` 的路径与范围后再用。
