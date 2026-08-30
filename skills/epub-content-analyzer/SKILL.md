---
name: epub-content-analyzer
description: 分析 EPUB 或文本材料中的结构角色，尤其是中文正文、标题、对话、诗歌、引文、书信、文白对照、注释等边界不清，需要给出字体角色与可重排排版建议时；只读分析，不改写内容。
---

# EPUB 文本内容分析

## 何时用

- 需要先建立确定性证据再判断中文语义时：先用本能力产出逐块角色报告，再结合上下文人工复核。分析阶段只读；低置信结论不得直接改写 XHTML。
- 适用输入：EPUB（扫 spine XHTML），或用 `source_name=` / `source_content=` 直接分析 xhtml/html/markdown/纯文本片段。
- 角色判断优先级：显式标签、`epub:type`、稳定 class 优先；相邻块关系次之；引号、长度、章节词等内容特征只作候选证据。字体建议只使用角色：`inherit`、`st`、`kt`、`fs`、`ht`、`en`、`mono`、`tszt-*`；不要从角色建议推断某个字体文件一定覆盖字符。
- 判断边界：
  - `<h1>`、`figcaption`、`blockquote`、footnote 语义明确 → 可采用高置信角色。
  - 短中文段落、无标点古文、仅含引号的段落 → 保留多个候选并人工复核。
  - 诗句和副标题外观相似 → 查章节位置、相邻段及现有 class，不按字数强判。
  - 文言与译文成对出现 → 先确认块级配对，再建议 `st` / `kt` 角色。
  - 普通正文 → 默认 `inherit`，不自动开启全书字体锁定。
- 禁止事项：
  - 不用一个正则批量决定中文语义；不因排版建议修改正文、标点、空格或章节顺序。
  - 不把低置信候选伪装成阅读器实测结论。
  - 不把 `font_role=st` 理解为必须嵌入某个宋体。
  - 仅在本地书级 `03 制作工作区/.pipeline/` 需要人工核对时启用片段输出；不把正文片段写入 `records/`。

## 调什么

```sh
epub run epub.text.content.analyze --input <书> --json
```

可选 KEY=VALUE：

- `include_snippets=true`：报告附正文片段（仅本地人工核对用）。
- `source_name=<文件名>` + `source_content=<文本>`：分析非 EPUB 的裸片段；`--input` 仍需指向一个有效 EPUB 作为输入锚点。

只读能力，不需要 `--output`。需要旧报告形状明细（逐块 `blocks` 数组：`evidence`、`confidence`、`candidate_roles`、`typography` 等）时加 `legacy_report=true`。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- facts 键前缀 `epub.text.content.analyze.`：
  - `blocks`：分析出的块数。
  - `review_required`：需要人工复核角色的块数。
  - `roles`：角色 → 块数分布。
- findings：
  - `error content.analysis-failed`：输入没有可分析内容（无 spine 正文等），`detail` 汇总原因。
  - `warn content.source-error`：个别源文档解析失败，`location` 指明文件。
  - `warn content.review-required`：有块结构模糊，需要人工复核。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`：逐块 `primary_role`、`candidate_roles`、`confidence`、`review_required`、`evidence`、`typography`（含 `font_role` 建议）、可选 `snippet`。

## 依据返回怎么判断

- `review_required == 0` 且无 `warn` → 报告可信，直接进入分派。
- `review_required > 0` 或 `primary_role=unknown` → 读取前后段落和章节上下文人工复核；示例：报告把 `春风又绿江南岸` 标为 `unknown`，候选含 `body`、`verse`、`subtitle`，应查看它是否位于诗歌容器、标题之后或正文叙述中；缺少结构证据时不添加 class。无法确定就保留原结构。
- 按 `roles` 分布与人工确认结果分派写入能力：
  - 字体链和正文节奏：`epub-typography-optimizer`。
  - 标题、诗歌、书信、文白对照：`epub-literary-structure-formatter`。
  - 注释：`epub-popup-footnote-converter`。
  - 生僻字和真实字形覆盖：`epub-font-coverage-analyzer`。
- `error content.analysis-failed` → 先修输入（EPUB 损坏或无正文），不强行继续；`warn content.source-error` 逐个核对列出文件，结论按「部分文件未分析」如实标注。
- 任何写入动作发生在新候选 EPUB 上，写后跑 `epub redline --check all <before.epub> <after.epub>`。
