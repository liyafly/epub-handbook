---
name: epub-source-intake
description: 从非 EPUB 源材料建立 EPUB 制作入口，包括纯文本、Markdown、HTML、PDF、扫描件 OCR 结果和图片素材的抽取、结构化、校对与后续排版验证。用于用户还没有 EPUB，需要先从文本或 PDF 生成可排版 XHTML/source bundle，再进入 EPUB 排版优化流程时。
---

# EPUB 源材料接入

当用户没有现成 EPUB，而是提供文本、Markdown、HTML、PDF 或扫描件时使用这个 skill。目标不是直接做最终精排，而是生成可校对、可验证、可继续排版的 EPUB source bundle。

## 输入判断

- 已有 `.epub`：不要使用本 skill，先用 `epub-layout-auditor`。
- `.txt` / `.md`：走文本结构化，重点是章节、段落、注释和空行语义。
- `.html` / `.xhtml`：走 HTML 清理，重点是语义标签、资源路径和 XML 合法性。
- born-digital PDF：优先抽取文本和图片，再人工抽样校对。
- 扫描 PDF / 图片：先 OCR，再把 OCR 结果作为不可信 source 处理。
- 多源目录：先建立 manifest plan，明确每个文件的角色，再生成 XHTML/Images/CSS。

## 固定目标

输出应当是可继续处理的源材料包，而不是最终交付 EPUB：

- 章节切分清楚，保留原始顺序。
- 正文、标题、注释、图注、表格、公式和页码残留有明确分类。
- 抽取日志记录工具、参数、抽样页和已知风险。
- XHTML 保持真实文本，不把可编辑正文转成图片。
- 图片素材保留原始文件和派生交付文件的对应关系。
- 后续排版交给 `epub-typography-optimizer`、`epub-image-layout-optimizer`、`epub-popup-footnote-converter` 等专项 skill。

## 工作流

1. 运行 harness 初判输入：

```sh
scripts/epub_ai_harness.py <source-path>
```

2. 建立 source map：
   - 输入文件列表。
   - 每个文件的角色：正文、封面、插图、注释、目录、附录、未知。
   - 抽取工具和参数。
   - 需要人工确认的页码、脚注、表格、公式和图片。
3. 抽取文本：
   - 文本/Markdown：保留段落和空行，不先做花哨样式。
   - HTML：清理非语义 wrapper，保留真实标题层级。
   - PDF：优先尝试 `pdftotext` / `mutool` 等外部工具；本项目不内置 PDF 抽取器。
   - 扫描件：先 OCR，记录 OCR 工具、语言、置信度和抽样校对结果。
4. 结构化：
   - 标题转成 `h1`/`h2` 或章节 metadata。
   - 正文段落转成 `p`。
   - 注释先保留成可追踪结构，再交给 popup footnote skill。
   - 表格、公式和图片不要强行扁平成普通段落。
5. 抽样校对：
   - 第一页/目录页。
   - 每个章节边界。
   - 至少一个脚注密集页。
   - 至少一个表格或公式页。
   - 至少一个图片页。
6. 进入排版链路：
   - 字体/正文：`epub-typography-optimizer`
   - 图片：`epub-image-layout-optimizer`
   - 注释：`epub-popup-footnote-converter`
   - 竖排/Ruby：`epub-vertical-ruby-optimizer`
   - OPF/nav：`epub-package-nav-auditor`
   - Kindle：`epub-kindle-compatibility-checker`

## PDF 处理边界

- 本仓不实现 PDF 解析、OCR、图片压缩或版面识别引擎。
- 可以调用用户环境已有工具，但必须记录工具名和版本。
- born-digital PDF 也可能有断行、页眉页脚、连字符、阅读顺序和多栏问题；不要默认抽取结果正确。
- 扫描 PDF 必须标记为 OCR 风险输入，不能直接当作可靠正文。
- PDF 中的页码、脚注编号、图题和表题要单独检查，不要只看首章。

## 图片压缩边界

图片压缩不属于本项目实现范围。本项目只做：

- 标记图片格式风险，例如 WebP、TIFF、CMYK JPEG、SVG-only Kindle 路径。
- 检查 OPF manifest、封面声明、figure 包装和图注。
- 建议外部压缩/转码后回到本项目验证。

外部压缩完成后，使用 `epub-image-layout-optimizer` 和 `epub-kindle-compatibility-checker` 复查。

## 禁止事项

- 不在没有抽样校对的情况下把 PDF/OCR 文本当成最终正文。
- 不自动改写作者文字来修复抽取错误。
- 不丢弃脚注、边注、图注、表格标题或公式。
- 不把 PDF 页面截图当作可重排正文主路径。
- 不把图片压缩结果直接纳入规则结论，除非有阅读器验证。
- 不在本 skill 中决定最终视觉样式；这里只负责接入、结构化和校验入口。

## 验证

接入完成后至少检查：

```sh
scripts/epub_ai_harness.py <source-or-output-path>
scripts/validate_skills_basic.py
git diff --check
```

如果已经形成 EPUB source tree 或 `.epub`，再使用 `epub-package-nav-auditor` 和对应专项 skill 做排版验证。
