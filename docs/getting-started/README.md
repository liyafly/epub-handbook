# 入门

本目录给第一次接触本仓的人。按顺序读完这些页面，你会知道如何构建 demo、如何检查自己的 EPUB、何时使用 AI skills，以及清洗后怎么用 diff 工具 review。

## 如何使用

1. [01-first-epub.md](01-first-epub.md) - 5 分钟做一本最小 EPUB
2. [02-anatomy.md](02-anatomy.md) - 一本 epub 到底由什么组成
3. [03-readers.md](03-readers.md) - 我应该在哪些阅读器上测
4. [06-test-your-own.md](06-test-your-own.md) - 测自己的 epub
5. [04-skills.md](04-skills.md) - AI skills 是什么、怎么用
6. [07-faq.md](07-faq.md) - 常见问题
7. [05-case-study.md](05-case-study.md) - 自造 EPUB 清洗演示案例
8. [glossary.md](glossary.md) - 术语表

读完后，你就能进入：

- 工程契约（[docs/final/](../final/)）查看硬规则与速查表；
- 批处理流水线（[docs/pipeline/](../pipeline/)）做批量清洗；
- Diff 工具（[tools/epub-diff/index.html](../../tools/epub-diff/index.html)）对比改前 / 改后。

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

- **字体策略**：[docs/plans/fonts-css-expansion-plan.md](../plans/fonts-css-expansion-plan.md)
- **弹注 / 注释**：[docs/guides/duokan-footnote-fallback-fix.md](../guides/duokan-footnote-fallback-fix.md)
- **图片混排**：[docs/guides/chapter-head-image.md](../guides/chapter-head-image.md)
- **英文小说**：[docs/guides/english-fiction-layout.md](../guides/english-fiction-layout.md)
- **文白对照 / 古典文本**：[docs/guides/classical-modern-layout.md](../guides/classical-modern-layout.md)
- **合集 / 大部头**：[docs/guides/anthology-navigation.md](../guides/anthology-navigation.md)
- **批量清洗**：[docs/pipeline/](../pipeline/) + [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)
- **review 改前改后差异**：浏览器打开 [tools/epub-diff/index.html](../../tools/epub-diff/index.html)
- **贡献回本仓**：[CONTRIBUTING.md](../../CONTRIBUTING.md)

## 推荐阅读顺序

文件名只是编号，实际推荐：

1. [01-first-epub.md](01-first-epub.md)
2. [02-anatomy.md](02-anatomy.md)
3. [03-readers.md](03-readers.md)
4. [06-test-your-own.md](06-test-your-own.md)
5. [04-skills.md](04-skills.md)
6. [07-faq.md](07-faq.md)
7. [05-case-study.md](05-case-study.md)
8. [glossary.md](glossary.md)
