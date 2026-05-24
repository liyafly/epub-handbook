# EPUB Handbook

这个项目用于沉淀 EPUB 3 制作规范、实测方案、转换技能和参考样本，并作为 `epub-pro` 执行引擎的上游规范仓。


## 上下游定位（epub-pro 对接）

- **上游输入**：阅读器实测、第三方参考 EPUB 与历史推导文档。
- **本仓输出**：
  - 手册（人读）
  - 速查表（人读）
  - 技能（半自动执行）
  - 实现约束（机器消费，见 `docs/final/SPEC-实现约束.md`）

## 推荐阅读入口

1. `docs/final/EPUB 3 终极实践手册.md`
   - 最终执行手册。
   - 只保留推荐方案：A-lite 整页海报、字体策略、弹出注释、局部竖排、标准 CSS 写法和制作检查流程。

2. `docs/final/EPUB 3 HTML CSS 属性速查表.md`
   - Markdown 版属性速查表。
   - 用表格整理 HTML 标签、OPF 元数据、CSS 属性、字体、图片、注释、Ruby、竖排和兼容性状态。

3. `docs/final/EPUB 3 HTML CSS 属性速查表.html`
   - HTML 查询版。
   - 适合本地打开后按属性名、标签名、用途或状态搜索，并逐行查看 HTML 预览样张。

4. `docs/final/SPEC-实现约束.md`
   - 机器与实现对接优先阅读。
   - 仅列实现约束（弹注/A-lite/字体/打包），用于执行层实现和回归测试。

5. `docs/final/epub-pro 技术架构 v1.md`
   - 执行层技术蓝图（Swift/Kotlin 双核 + Rust sidecar）。
   - 覆盖目录、接口、子集器策略、测试矩阵、分发与里程碑。
   - 日常样式、模板和 skill 调整不改这个文件；只在执行层架构变化时更新。

6. `docs/guides/skills-and-templates.md`
   - skills 与模板维护建议。
   - 说明哪些内容应该放手册、哪些应该放 skill、哪些应该放可运行模板，以及 harness/hook 的使用方式。

7. `templates/epub-style-demo/`
   - 可快速打包的 EPUB 3 样式 demo。
   - 用于在 Apple Books、Kindle、KOReader、Thorium、Calibre 等环境对比显示效果。

8. `skills/README.md`
   - Codex/Claude Code 技能总入口。
   - 说明已有 EPUB 优化、文本/PDF 源材料接入、专项排版修复和验证命令。

## 目录结构

```text
CLAUDE.md

docs/
  final/
    EPUB 3 终极实践手册.md
    EPUB 3 HTML CSS 属性速查表.md
    EPUB 3 HTML CSS 属性速查表.html
    SPEC-实现约束.md
    epub-pro 技术架构 v1.md
    fixtures.md
    reader-matrix.yaml
  source/
    EPUB 3 制作完全参考手册.md
    EPUB 3 补充：其他 CSS 模块.md
    EPUB 3 补充：列表 - 字体 - HTML 标签速查.md
    EPUB 3 补充：图片与整版海报页.md
    EPUB 3 补充：弹出注释与 Ruby 注音.md
  experiments/
    EPUB 3 章节扉页与竖排实战 · 补充 05.md
  guides/
    README.md
    skills-and-templates.md
  reference/
    README.md

references/
  epubs/
    EPub指南——从入门到放弃 20230418 (赤霓) (Z-Library).epub

skills/
  README.md
  epub-layout-auditor/
    SKILL.md
  epub-source-intake/
    SKILL.md
  epub-english-typography-optimizer/
    SKILL.md
  epub-alite-converter/
    SKILL.md
  epub-popup-footnote-converter/
    SKILL.md
    assets/note.png
  epub-*/agents/openai.yaml

scripts/
  epub_ai_harness.py
  validate_skills_basic.py
  validate_epub_style_demo.py
  validate_popup_notes.py

hooks/
  pre-commit.epub-handbook

templates/
  README.md
  epub-style-demo/
    README.md
    build.sh
    mimetype
    META-INF/container.xml
    OEBPS/
      package.opf
      nav.xhtml
      Styles/base.css
      Text/*.xhtml
      Images/*.png
```

## 各目录职责

`docs/final/` 是正式文档区。后续制作 EPUB 时，优先以这里的终极手册和速查表为准。

`docs/source/` 是早期探索和补充文档区。这些内容保留原始推导、取舍和参考资料，作为溯源材料，不强制代表最终推荐写法。

`docs/experiments/` 是实测记录区。当前 `05` 文档保留 A-lite 海报方案的实践过程，最终结论已经整理进终极手册。

`docs/guides/` 是仓库维护说明区。这里记录 skills、模板、目录职责和后续维护建议，不承载下游技术架构。

`references/epubs/` 是可以纳入版本管理的参考 EPUB 区。当前只放《EPub指南——从入门到放弃》。

`skills/` 是 Codex/Claude Code 技能区。已有 EPUB 优化先走 `epub-layout-auditor`；只有文本、PDF、HTML 或 OCR 结果时先走 `epub-source-intake`；具体问题再切到 CSS、中文字体、英文排版、图片、竖排、弹注、Kindle、OPF/nav 或 A-lite 专项 skill。完整清单见 `skills/README.md`。

`scripts/epub_ai_harness.py` 是 AI 辅助入口，用来判断输入类型、列出结构风险、推荐 skills 和下一步命令：

```sh
scripts/epub_ai_harness.py <epub-or-source-path>
scripts/epub_ai_harness.py <epub-or-source-path> --format json
```

图片压缩、PDF 解析和 OCR 不在本仓实现；本仓只负责记录边界、检查 EPUB 风险和验证排版/结构。

`templates/` 是可运行样式样本区。当前 `templates/epub-style-demo/` 可以用下面命令生成最小 EPUB，用来验证正文、Ruby、弹注、竖排、A-lite、列表、表格、代码、英文正文、图文环绕、章节头图、便签边框和 MathML 样式：

```sh
sh templates/epub-style-demo/build.sh
```

常用校验：

```sh
scripts/validate_skills_basic.py
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
git diff --check
```

## 忽略规则

默认忽略所有 EPUB、demo 工作目录、模板生成目录和解压目录。

唯一例外是：

```text
references/epubs/EPub指南——从入门到放弃 20230418 (赤霓) (Z-Library).epub
```

这本入门 EPUB 会被 Git 看到，方便作为长期参考样本保留。其他 EPUB 继续只作为本地素材，不纳入版本管理。

## 许可证

本仓库采用双许可证：

- `docs/` 下的原创手册、速查表和项目文档采用 Creative Commons Attribution 4.0 International License。
- 仓库内的可复用 HTML/CSS/XML 代码片段、模板和 `skills/` 下的转换技能采用 MIT License。

`references/epubs/` 下的第三方参考 EPUB 不属于本仓库原创内容，不包含在上述授权范围内。详见 `LICENSE` 和 `THIRD_PARTY.md`。
