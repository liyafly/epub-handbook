---
name: epub3-migrator
description: 把 EPUB2 或 legacy EPUB 的 package/nav、XHTML shell 和已识别注释迁移为 EPUB3，保留原文件与正文。需审查 dry-run，不是局部样式修复或阅读器验收。
---

# EPUB3 迁移

## 何时用

预检确认需要版本/结构迁移时；已经符合 EPUB3 且仅有局部排版问题，不重复迁移。范围包含 package metadata/properties、nav（保留 NCX）、XHTML shell、已识别注释与可选基础排版，不嵌入新字体。

## 调什么

```sh
epub run epub.package.nav.audit --input "before.epub" --json
epub run epub.package.migrate.epub3 --input "before.epub" --output "candidate.epub" --dry-run --json
# 审查后，保持与 dry-run 相同的选项
epub run epub.package.migrate.epub3 --input "before.epub" --output "candidate.epub" --json
epub run epub.notes.popup.normalize --input "candidate.epub" --json
epub run epub.package.nav.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

按用户明确范围选择 `no_popup_notes=true` / `no_typography=true`，不要用默认值悄悄扩展为未授权排版。输出必须是新路径。

## 返回怎么读

前缀 `epub.package.migrate.epub3.`：`packageVersionBefore`、`navEntries`、`xhtmlFilesUpdated`、manifest/metadata 改动计数、`plainNotesConverted`、`duokanNotesNormalized`、`warnings` 与开关回显。
输入/输出 SHA 在信封 input/output，不在 facts。公共状态见 [索引](../README.md)。

## 依据返回怎么判断

- 审查 warnings、注释识别范围和样式/导航计划；模糊注释保留转人工，不部分猜转。不要把转换计数等同于原注释总数。
- DRM/未知加密停止；保守重写失败读取具体事件，不绕过保护或覆盖输出重试。
- 实跑红线失败保留候选，逐项分析；元数据版本等预期迁移变化与意外正文/顺序变化分开解释，不能用宽泛豁免掩盖。
- 产物预检与注释验证后，人工 diff OPF/nav/NCX、封面、spine、注释及 CSS；需要精排才分派专项 skill，迁移成功不是阅读器通过。
