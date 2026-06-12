# 入门

本目录给第一次接触本仓的人。按顺序读完这些页面，你会知道如何构建 demo、如何检查自己的 EPUB、何时使用 AI skills，以及清洗后怎么用外部 diff 工具 review。

> 文件名前缀（01、02…）只是稳定 ID，**不代表阅读顺序**。请按下面的推荐顺序读。

读完后，你就能进入：

- 工程契约（[docs/final/](../final/)）查看硬规则与速查表；
- 批处理流水线（[docs/pipeline/](../pipeline/)）做批量清洗；
- 外部 Diff 工具（按 [EPUB diff review](../pipeline/epub-diff-review.md) 用 Calibre / VS Code）对比改前 / 改后。

## 速查：一定要做 / 一定不要

**一定要做**：

1. EPUB 第一个 zip entry 必须是 `mimetype`，且内容是 `application/epub+zip`，不压缩。
2. 所有正文是可选中的文本，不是图片（FXL 漫画除外，不在本仓范围）。
3. OPF manifest 列出每个 epub 内的文件；spine 决定阅读顺序。
4. 每个章节 XHTML 是合法 XML。
5. 用 `xml:lang` 标记每段语言，特别是中英混排。

**一定不要**：

1. 不要把正文文字烤进图片。
2. 不要硬编码字号；优先用 `em` / `%`。
3. 不要在 `body` 上设 line-height。
4. 不要用 `display:flex` / `grid` / `position:absolute` 承载正文。
5. 不要把弹注做成普通超链接；标准弹注要用 `epub:type`。

## 读完入门后去哪？

- **字体策略**：[docs/final/SPEC-实现约束.md §8](../final/SPEC-实现约束.md#8-字体链规则)
- **弹注 / 注释**：[docs/guides/duokan-footnote-fallback-fix.md](../guides/duokan-footnote-fallback-fix.md)
- **EPUB2 外壳弹注兼容实验**：[docs/guides/epub2-popup-note-compatibility.md](../guides/epub2-popup-note-compatibility.md)
- **图片混排**：[docs/guides/chapter-head-image.md](../guides/chapter-head-image.md)
- **英文小说**：[docs/guides/english-fiction-layout.md](../guides/english-fiction-layout.md)
- **文白对照 / 古典文本**：[docs/guides/classical-modern-layout.md](../guides/classical-modern-layout.md)
- **合集 / 大部头**：[docs/guides/anthology-navigation.md](../guides/anthology-navigation.md)
- **批量清洗**：[docs/pipeline/](../pipeline/) + [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)
- **现成 EPUB 精排建议**：[docs/pipeline/refinement-harnesses.md](../pipeline/refinement-harnesses.md)
- **review 改前改后差异**：按 [EPUB diff review](../pipeline/epub-diff-review.md) 用 Calibre / VS Code
- **贡献回本仓**：[CONTRIBUTING.md](../../CONTRIBUTING.md)

## 推荐阅读顺序

| 顺序 | 页面 | 读完你能做什么 |
| --- | --- | --- |
| 00 | [EPUB 是什么](00-what-is-epub.md) | 用白话解释 EPUB、PDF 和阅读器差异 |
| 01 | [5 分钟做一本最小 EPUB](01-first-epub.md) | 构建 demo，并裁出自己的最小书 |
| 02 | [EPUB 内部结构](02-anatomy.md) | 看懂容器、OPF、正文、样式和资源的关系 |
| 03 | [阅读器与测试范围](03-readers.md) | 选择需要验证的阅读器和转换器 |
| 06 | [测试自己的 EPUB](06-test-your-own.md) | 对自己的书运行 validator 与基础检查 |
| 04 | [AI skills](04-skills.md) | 按问题选择主入口和专项 skill |
| 07 | [常见问题](07-faq.md) | 快速排查高频制作与兼容性问题 |
| 08 | [EPUB2 / EPUB3 兼容](08-epub2-epub3-compatibility.md) | 判断版本、导航、XHTML 和弹注 fallback |
| 05 | [清洗案例](05-case-study.md) | 跟完一套 before / after 清洗与 review |
| 术语表 | [glossary](glossary.md) | 随时回查本仓使用的核心术语 |
