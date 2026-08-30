---
name: epub-source-intake
description: 从非 EPUB 源材料建立 EPUB 制作入口，包括纯文本、Markdown、HTML、PDF、扫描件 OCR 结果和图片素材的抽取、结构化、校对与后续排版验证。用于用户还没有 EPUB，需要先从文本或 PDF 生成可校对、可验证、可继续排版的 source bundle，再进入 EPUB 排版优化流程时。
---

# EPUB 源材料接入

## 何时用

- 用户没有现成 EPUB，而是提供文本、Markdown、HTML、PDF 或扫描件。目标不是直接做最终精排，而是生成可校对、可验证、可继续排版的 EPUB source bundle。
- 输入判断：已有 `.epub` 时不用本 skill，先用 `epub-layout-auditor`；`.txt`/`.md` 走文本结构化（章节、段落、注释和空行语义）；`.html`/`.xhtml` 走 HTML 清理（语义标签、资源路径、XML 合法性）；born-digital PDF 优先抽取文本和图片再人工抽样校对；扫描 PDF/图片先 OCR，把 OCR 结果当不可信 source；多源目录先建 manifest plan 明确每个文件角色。
- PDF 处理边界：本仓不实现 PDF 解析、OCR、图片压缩或版面识别引擎；可调用用户环境已有工具，但必须记录工具名和版本。born-digital PDF 也可能有断行、页眉页脚、连字符、阅读顺序和多栏问题，不要默认抽取结果正确；扫描 PDF 必须标记为 OCR 风险输入；PDF 中的页码、脚注编号、图题和表题要单独检查，不要只看首章。
- 图片压缩边界：图片压缩不属于本项目实现范围。只做：标记图片格式风险（WebP、TIFF、CMYK JPEG、SVG-only Kindle 路径）、检查 OPF manifest/封面声明/figure 包装/图注、建议外部压缩转码后回到本项目验证。
- 禁止事项：不在没有抽样校对的情况下把 PDF/OCR 文本当最终正文；不自动改写作者文字修复抽取错误；不丢弃脚注、边注、图注、表格标题或公式；不把 PDF 页面截图当可重排正文主路径；不把图片压缩结果直接纳入规则结论（除非有阅读器验证）；不在本 skill 决定最终视觉样式——这里只负责接入、结构化和校验入口。

## 调什么

源材料接入是人工 + AI 流程，以人工流程为准：

1. 按书级项目约定建工作区（`01 源文件/`、`02 校对材料/`、`03 制作工作区/`，见 [docs/pipeline/book-workspace.md](../../docs/pipeline/book-workspace.md)），把入选源文件保留在 `01 源文件/` 并记录 SHA-256；编辑只发生在制作工作区。
2. 写来源记录：输入文件列表、每个文件的角色（正文、封面、插图、注释、目录、附录、未知）、抽取工具和参数、需要人工确认的页码/脚注/表格/公式/图片。
3. 抽取与结构化后，可用能力入口初判（见下）；形成 EPUB source tree 或 `.epub` 后再交给 `epub-package-nav-auditor` 与排版专项 skill。

```sh
epub run epub.source.intake --input <源文件目录或文件> --json
```

注意：该能力 Go 实现迁移中，当前运行只做用法与输入检查——对非 EPUB 输入（`.txt`/`.pdf`/目录）会得到 `error input.invalid-epub`（`status: failed`，退出码 1），对 EPUB 输入会在 findings 里得到 `warn capability.not-implemented`。两者都表示抽取与结构化逻辑尚未接入，应以人工流程为准，不要把任何一次运行当作"接入完成"。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- 迁移期的两种预期返回：`warn capability.not-implemented`（events 显示 skipped，表示能力未接入）；`error input.invalid-epub`（`detail` 说明输入不是 zip/EPUB，表示 CLI 尚不支持纯源文件输入）。
- 除这两种返回外，本 skill 的"结果"是人工产出的 source bundle 与来源记录本身，不是命令输出。

## 依据返回怎么判断

- `findings` 出现 `capability.not-implemented` 或 `input.invalid-epub` → 按人工流程继续：抽取文本（文本/Markdown 保留段落和空行；HTML 清理非语义 wrapper、保留真实标题层级；PDF 优先尝试 `pdftotext` / `mutool` 等外部工具并记录版本；扫描件先 OCR，记录工具、语言、置信度和抽样校对结果），结构化（标题转 `h1`/`h2` 或章节 metadata；正文段落转 `p`；注释先保留成可追踪结构，再交给 `epub-popup-footnote-converter`；表格、公式和图片不强行扁平成普通段落）。
- 结构化质量判据：章节切分清楚、保留原始顺序；正文、标题、注释、图注、表格、公式和页码残留有明确分类；抽取日志记录工具、参数、抽样页和已知风险；XHTML 保持真实文本，不把可编辑正文转成图片；图片素材保留原始文件与派生交付文件的对应关系。
- 抽样校对必须覆盖：第一页/目录页；每个章节边界；至少一个脚注密集页；至少一个表格或公式页；至少一个图片页。
- 进入排版链路的分流：字体/正文 → `epub-typography-optimizer`；图片 → `epub-image-layout-optimizer`；注释 → `epub-popup-footnote-converter`；竖排/Ruby → `epub-vertical-ruby-optimizer`；OPF/nav → `epub-package-nav-auditor`；Kindle → `epub-kindle-compatibility-checker`；英文排版 → `epub-english-typography-optimizer`。
- 外部图片压缩完成后，用 `epub-image-layout-optimizer` 和 `epub-kindle-compatibility-checker` 复查，再以 `epub redline --check all <before> <after>` 确认改动范围。
