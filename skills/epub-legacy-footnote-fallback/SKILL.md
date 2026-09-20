---
name: epub-legacy-footnote-fallback
description: 仅在明确要求兼容多看旧版时，为标准 EPUB3 grouped footnote 人工叠加 legacy hooks；不创建第二份注释、不替代标准弹注，当前无自动 runner。
---

# EPUB 旧版弹注 Fallback

## 何时用

目标包含 Duokan legacy 才使用。先读 [SPEC §1](../../docs/final/SPEC-实现约束.md) 与 [兼容 fixture](../../templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml)；非标准结构先交 popup skill。

## 调什么

`epub.notes.legacy-fallback` 当前未实现；人工叠加 hooks 后只读验证：

```sh
epub run epub.notes.popup.normalize --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

## 返回怎么读

前缀 `epub.notes.popup.normalize.`：`noterefs/text_files/violations`；`error popupnotes` 的 title/location 指向问题。零违反只代表结构合格，不证明旧多看交互正确。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 标准属性/中性类保留；anchor 加 duokan-footnote 且内含图标；ol.footnote-list 加 duokan-footnote-content；li.footnote-item 仅加 duokan-footnote-item，不把 content 类放 li。
- 同文件一个 aside/ol，noteref 指向唯一 li，◎ backlink 返回原 trigger；不得复制可见 note list、display:none 隐藏正文或用 JS。
- 保留现有图标 src/alt，缺少才复用项目资源并同步 manifest。样式并入活动 notes.css，分隔线只留一套，不影响普通上标。
- 多 note 页面逐个点击确认只打开对应 li；红线失败保留候选分析，注释文字不得借兼容修改。EPUB2 外壳只按 [兼容指南](../../docs/how-to/epub2-popup-note-compatibility.md) 记录目标版本实测，不能标为严格 EPUB2 标准。
