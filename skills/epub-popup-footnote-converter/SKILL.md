---
name: epub-popup-footnote-converter
description: 规范 EPUB 普通/旧式注释为同文件 grouped popup notes，并验证图标、目标与回跳。popup normalize 仅校验；已识别旧注释可随 EPUB3 迁移转换，其他按授权人工处理。
---

# EPUB 标准弹注

## 何时用

标准注释转换、迁移后复核或弹注失联。先完整读 [SPEC §1](../../docs/final/SPEC-实现约束.md)，参照 [标准 fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)；多看旧版额外兼容另走 legacy skill。

## 调什么

```sh
# 名字含 normalize，但当前只校验，不改书
epub run epub.notes.popup.normalize --input "book.epub" --json
```

已授权 EPUB3 迁移时，用 migrator 转换经识别的 plain/Sigil/Duokan 结构；它还会改 package/nav/shell，并非“仅弹注”写工具。仅授权弹注时人工修改对应资源，不借此扩大到全书迁移。修改后再跑本命令和全项 redline。

## 返回怎么读

前缀 `epub.notes.popup.normalize.`：`noterefs`、`text_files`、`violations`；`error popupnotes` 的 title/location 给出文件和问题。
迁移报告的 `plainNotesConverted/duokanNotesNormalized/warnings` 只表示识别/处理结果，不证明所有原注释都被识别。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 同一 XHTML 中：图片 noteref anchor（epub:type、role、唯一 id）→ li.footnote-item 的 id → ◎ backlink 返回 trigger；每文件最多一个 aside[epub:type=footnote]，内有 ol.footnote-list。声明 epub namespace。
- 保留已有图标 src/alt；仅缺图标时使用 [note.png](assets/note.png) 并补 manifest。结构/视觉写 notes.css，字体声明写 fonts.css，图标规则限定作用域，不影响普通 sup。
- 不逐条创建 aside，不搬到跨文件 note body，不 display:none、不复制第二份注释、不改注释文字。
- 模糊结构先逐条配对；Sigil 分组内有无法匹配的 aside 或附加内容时不做部分合并。尽量保留原 id。
- 零违反还须对照原 noteref/note 数量、正文与每个触发目标，避免“没有被扫描到”误作正确。红线只允许表示控件差异，注释正文必须保持；多条 note 的弹窗范围需目标阅读器实测。
