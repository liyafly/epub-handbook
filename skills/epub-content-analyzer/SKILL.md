---
name: epub-content-analyzer
description: 只读识别 EPUB 正文、标题、对话、诗歌、引文、书信、文白与注释角色，给出证据及字体角色建议。用于结构含混时；不改正文，不证明实际字体覆盖。
---

# EPUB 文本角色分析

## 何时用

需要确定性逐块证据辅助语义判断时使用。字体缺字用 coverage skill，确认角色后的结构调整用 literary skill。公共返回见 [索引](../README.md)。

## 调什么

```sh
epub run epub.text.content.analyze --input "book.epub" --json
```

只读。`include_snippets=true` 仅用于本地复核，报告放书级 `.pipeline/`，不把正文写进仓库级 records。
裸片段可用 `source_name=` + `source_content=`，但当前仍需有效 EPUB 作为 `--input` 锚点；纯源材料先用 source-intake。

## 返回怎么读

- 前缀 `epub.text.content.analyze.`：`blocks`、`review_required`、`roles`。
- `facts.blockList[]`：source、locator、primary_role、candidate_roles、confidence、review_required、evidence、typography；`facts.sourceErrors` 列出未解析文件。
- `content.analysis-failed` 无有效分析；`content.source-error` 分析不完整；`content.review-required` 需上下文复核。

## 依据返回怎么判断

显式语义/tag/class 优先，相邻块关系其次，引号、长度和关键词仅作候选。即使无 warning 也不能当成语义正确证明。诗与副标题、古文与普通短段等歧义须看前后文；无法确定保留原结构。
字体建议是 `inherit/st/kt/fs/ht/en/mono/tszt-*` 角色，不要求嵌入特定字体；普通正文默认 inherit。仅将确认结论交专项 skill，不因分类修改标点、空格或章节顺序。
