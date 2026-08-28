---
name: epub-content-analyzer
description: 用于分析 EPUB、XHTML、HTML、Markdown 或纯文本中的结构角色，尤其是中文正文、标题、对话、诗歌、引文、书信、文白对照、注释等边界不清，需要给出字体角色与可重排排版建议时。
---

# EPUB 文本内容分析

先用确定性报告建立证据，再结合上下文判断中文语义。分析阶段只读；低置信结论不得直接改写 XHTML。

## 工作流

1. 已有 EPUB 先完成 preflight；加密、损坏或 OPF/container 异常时停止。
2. 运行：

   ```sh
   python3 scripts/epub_content_analyzer.py <input> --format json
   ```

   仅在本地书级 `03 制作工作区/.pipeline/` 需要人工核对时加 `--include-snippets`；不把正文片段写入 `records/`。
3. 按 `evidence` 和 `confidence` 复核候选角色：
   - 显式标签、`epub:type`、稳定 class 优先。
   - 相邻块关系次之。
   - 引号、长度、章节词等内容特征只作候选证据。
4. `review_required=true` 或 `primary_role=unknown` 时读取前后段落和章节上下文；无法确定就保留原结构。
5. 字体建议只使用角色：`inherit`、`st`、`kt`、`fs`、`ht`、`en`、`mono`、`tszt-*`。不要从角色建议推断某个字体文件一定覆盖字符。
6. 人工确认后再分派写入能力：
   - 字体链和正文节奏：`epub-typography-optimizer`
   - 标题、诗歌、书信、文白对照：`epub-literary-structure-formatter`
   - 注释：`epub-popup-footnote-converter`
   - 生僻字和真实字形覆盖：`epub-font-coverage-analyzer`

## 判断边界

| 情况 | 行为 |
| --- | --- |
| `<h1>`、`figcaption`、`blockquote`、footnote 语义明确 | 可采用高置信角色 |
| 短中文段落、无标点古文、仅含引号的段落 | 保留多个候选并人工复核 |
| 诗句和副标题外观相似 | 查章节位置、相邻段及现有 class，不按字数强判 |
| 文言与译文成对出现 | 先确认块级配对，再建议 `st` / `kt` 角色 |
| 普通正文 | 默认 `inherit`，不自动开启全书字体锁定 |

## 示例

报告把 `春风又绿江南岸` 标为 `unknown`，候选含 `body`、`verse`、`subtitle`。应查看它是否位于诗歌容器、标题之后或正文叙述中；在缺少结构证据时不添加 class。

## 禁止事项

- 不用一个正则批量决定中文语义。
- 不因排版建议修改正文、标点、空格或章节顺序。
- 不把低置信候选伪装成阅读器实测结论。
- 不把 `font_role=st` 理解为必须嵌入某个宋体。

## 验证

```sh
python3 scripts/test_epub_content_analyzer.py
python3 scripts/validate_skills_basic.py
python3 scripts/validate_contracts.py
python3 scripts/validate_python_entrypoint_inventory.py
git diff --check
```
