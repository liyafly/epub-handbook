# 英文断词

## 效果

英文段落在行尾自动断词（`hyphens: auto`），减少窄屏和大字号下的大空隙；`orphans`、`widows` 避免段首或段尾只剩一行落在另一页。来源：[SPEC §5.9](../../../docs/final/SPEC-实现约束.md#59-英文小说正文)。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 待测 | 待测 | 待测 | 断词 → 不断词 |

只写好合成样本，没有在任何阅读器里看过。

## 标记结构

见 [sample.xhtml](sample.xhtml)：给英文容器加 `en-hyphenate`。断词靠语言：页面的 `html` 要同时写 `lang="en"` 和 `xml:lang="en"`，否则阅读器不知道按哪种语言的规则断。不在源码里插软连字符（`&shy;`）。

## 增强层

[recipe.css](recipe.css) 写 `-webkit-hyphens`、`-epub-hyphens` 和标准 `hyphens`，再加 `orphans: 2`、`widows: 2`。不设 `text-align: justify`：按 SPEC §5.9，断词在目标阅读器实测稳定前保持左对齐。

## 降级行为

不认 `hyphens` 的阅读器不断词，照常换行；不认 `orphans`、`widows` 的阅读器照常分页。文字都不变。

## 阅读器结果

尚未实测。记录到 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-english-hyphenation`。
