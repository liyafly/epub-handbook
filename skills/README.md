# Skills

这个目录保存 Codex/Claude Code 可直接读取的 EPUB 排版与转换技能。这里的 skill 是“排版/转换契约”，不是下游 `epub-pro` 的实现代码。

## 语言约定

- 目录名和 frontmatter `name` 使用英文短横线，便于工具触发、路径引用和跨环境迁移。
- `description`、正文和 `agents/openai.yaml` 使用中文，贴合本仓库文档和日常使用语境。
- EPUB、CSS、Kindle、OPF、Ruby、A-lite 等固定术语保留英文关键词，方便检索。

## 推荐使用顺序

1. 先用 harness 判断输入类型：

```sh
scripts/epub_ai_harness.py <epub-or-source-path>
```

2. 已有 EPUB 时，用 `epub-layout-auditor` 做总审稿：看 diff、识别页面类型、列出风险、分派专项 skill。
3. 没有 EPUB、只有文本/PDF/HTML/扫描件时，用 `epub-source-intake` 做源材料接入，再进入排版链路。
4. 再按问题类型使用专项 skill：中文字体、英文排版、CSS 分层、图文、竖排、弹注、Kindle、OPF/nav 等。
5. 修改模板或规则后，运行 demo build 和 validator。
6. 阅读器实测后，把结果回写 `docs/final/reader-matrix.yaml`，再更新 SPEC、手册和速查表。

示例提示：

```text
使用 $epub-layout-auditor 对比 develop 审核当前 EPUB 排版改动，并给出需要补的专项修复。
```

```text
使用 $epub-image-layout-optimizer 修复这本 EPUB 中的 figure 环绕和 Kindle 图片风险。
```

## 当前 Skills

| Skill | 用途 | 主要参考 |
|---|---|---|
| `epub-layout-auditor` | 总入口：审稿、风险分级、分派专项修复 | `docs/final/SPEC-实现约束.md`、`SCENE_MATRIX.md` |
| `epub-source-intake` | 从文本、Markdown、HTML、PDF 或 OCR 结果建立 EPUB 制作入口 | `scripts/epub_ai_harness.py` |
| `epub-css-layering-optimizer` | 维护 `fonts/base/notes/effects/literary/media/vertical/poster.css` 分层 | `docs/final/SPEC-实现约束.md` §7 |
| `epub-typography-optimizer` | 中文正文节奏、字体链、嵌入字体和生僻字 fallback | `Text/07-font-family-order.xhtml`、`Text/08-long-mixed-flow.xhtml` |
| `epub-english-typography-optimizer` | 英文书籍类型判断、serif 链、段落节奏、断字和大字号回归 | `Text/18-english-fiction.xhtml`、`docs/guides/english-fiction-layout.md` |
| `epub-literary-structure-formatter` | 章首、题记、前置页、对话、诗、信件、场景分隔 | `Text/11-chapter-opening.xhtml`、`Text/12-literary-fiction.xhtml`、`Text/15-frontmatter.xhtml` |
| `epub-image-layout-optimizer` | figure 环绕、图注、封面声明、图片格式兼容 | `Text/17-image-layout.xhtml` |
| `epub-vertical-ruby-optimizer` | 竖排正文、Ruby 注音、中西文方向 | `Text/02-ruby-note.xhtml`、`Text/14-vertical-body.xhtml` |
| `epub-kindle-compatibility-checker` | Kindle/KDP 风险、转换日志、WebP/SVG/cover/MathML 检查 | `Text/09-kindle-risk.xhtml` |
| `epub-package-nav-auditor` | OPF manifest/spine、nav、NCX、cover、MathML properties | `OEBPS/package.opf`、`nav.xhtml`、`toc.ncx` |
| `epub-alite-converter` | 封面、卷首、章首或海报页转 A-lite 可重排全页方案 | `Text/03-vertical-alite.xhtml` |
| `epub-popup-footnote-converter` | 普通注释/尾注/旧注释转标准 grouped popup footnote | `Text/02-ruby-note.xhtml` |
| `epub-legacy-footnote-fallback` | 在标准弹注上叠加多看旧版兼容 fallback | `Text/05-legacy-note-fallback.xhtml`、`Text/06-multi-legacy-note-fallback.xhtml` |
| `epub-style-demo-maintainer` | 维护 demo fixture、reader matrix、SPEC 和最终文档同步 | `templates/epub-style-demo/` |

## 怎么运行

判断输入属于“已有 EPUB 优化”还是“源材料接入”：

```sh
scripts/epub_ai_harness.py <path>
scripts/epub_ai_harness.py <path> --format json
```

构建最小样本 EPUB：

```sh
sh templates/epub-style-demo/build.sh
```

验证 demo 结构：

```sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

验证弹注结构：

```sh
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

检查 XML：

```sh
xmllint --noout \
  templates/epub-style-demo/OEBPS/package.opf \
  templates/epub-style-demo/OEBPS/nav.xhtml \
  templates/epub-style-demo/OEBPS/toc.ncx
```

如果本机没有 `xmllint`，仍要运行 Python validator；它会用标准库解析 XML 并检查 manifest/link/MathML 等关键约束。

验证本仓所有 skills 的基础结构：

```sh
scripts/validate_skills_basic.py
```

可选安装本仓 hook 模板：

```sh
cp hooks/pre-commit.epub-handbook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

这个 hook 会运行 `git diff --check --cached`、skill 基础校验，并在 demo 或关键验证脚本变化时自动构建和验证 `templates/epub-style-demo/`。

## 两类常见场景

已有 EPUB：

1. `scripts/epub_ai_harness.py book.epub`
2. `epub-layout-auditor` 判断风险和专项 skill。
3. 用字体、图片、弹注、Kindle、OPF/nav 等专项 skill 修复。
4. 重新打包并运行相关 validator。

没有 EPUB，只有文本或 PDF：

1. `scripts/epub_ai_harness.py source.pdf` 或源目录。
2. `epub-source-intake` 抽取、结构化、抽样校对，产出可排版 source bundle。
3. 形成 XHTML/Images/OPF/nav 后，进入 `epub-package-nav-auditor` 和排版专项 skill。
4. PDF 抽取、OCR 和图片压缩使用外部工具；本仓只记录边界、检查风险和验证 EPUB 结构/排版。

## 维护规则

- 不改 `SKILL.md` frontmatter 的字段名，只保留 `name` 和 `description`。
- `description` 写触发场景和目标能力；正文写固定目标、工作流、禁止事项和验证方式。
- 新增技能先判断是否只是样式样本：如果只是验证一个样式，优先放进 `templates/epub-style-demo/`。
- 修改结构性规则时，同步检查 `docs/final/EPUB 3 HTML CSS 属性速查表.html`、`docs/final/SPEC-实现约束.md` 和 `templates/epub-style-demo/`。
- 不在 skill 里写下游引擎架构或平台分发逻辑。
- `agents/openai.yaml` 是可选 UI metadata，必须和 `SKILL.md` 的用途保持一致。
