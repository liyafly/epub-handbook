# Skills

这个目录保存 Codex、Claude Code 和其他 AI 代理可直接读取的 EPUB 排版与转换技能。代理必须先阅读根目录 `AGENTS.md`，再按任务选择专项 skill。skills 是纯文档层：所有可执行逻辑收口到唯一公开命令 `epub`（Go CLI），SKILL.md 只描述「何时用、调什么、返回怎么读、依据返回怎么判断」。

## 语言约定

- 目录名和 frontmatter `name` 使用英文短横线，便于工具触发、路径引用和跨环境迁移。
- `description`、正文和 `agents/openai.yaml` 使用中文，贴合本仓库文档和日常使用语境。
- EPUB、CSS、Kindle、OPF、Ruby、A-lite 等固定术语保留英文关键词，方便检索。

## 命令形态

SKILL.md 只允许一种调用形态（可被守卫对账）：

```sh
epub run <capability-id> --input <书> --output <新书> --json [KEY=VALUE ...]
epub redline --check all <before.epub> <after.epub>   # 红线两文件比对（flag 必须写在两个路径之前）
epub capabilities [--json]                            # 列出全部能力及 Go 实现状态
```

返回是统一 JSON 信封：`status`（`complete | failed | approval-required`）、`findings[].level`（`error | warn | info`）、能力特有 `facts`、给 agent 的 `nextCommands[]`。退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（停下来问人）；3 用法错误。

## 推荐使用顺序

1. 用 `epub capabilities --json` 确认能力清单与实现状态；契约位于 `contracts/capabilities/v1/`。
2. 已有 EPUB 时，先用 `epub-layout-auditor` 做总审稿（配合 `epub run epub.layout.audit --input <书> --json`）：看 diff、识别页面类型、列出风险、分派专项 skill。
3. 没有 EPUB、只有文本/PDF/HTML/扫描件时，用 `epub-source-intake` 做源材料接入（人工 + AI 流程），再进入排版链路。
4. 结构脏或文件名混淆的书先走 `epub-structure-normalizer` 的双阶段流程（dry-run 人工 review → 实跑 → `epub redline --check all --path-map <normalize 报告>`）。
5. 再按问题类型使用专项 skill：EPUB3 迁移、中文字体、英文排版、CSS 分层、图文、竖排、弹注、Kindle、OPF/nav、A-lite 等。
6. 改书后跑该 skill「调什么」里列出的校验组合（弹注校验、demo 校验、红线）；构建 demo 用 `sh templates/epub-style-demo/build.sh`，产物在模板自己的 `dist/`。
7. 阅读器实测后，把结果回写 `docs/final/reader-matrix.yaml`，再更新 SPEC、手册和速查表。

示例提示：

```text
使用 $epub-layout-auditor 对比 develop 审核当前 EPUB 排版改动，并给出需要补的专项修复。
```

```text
使用 $epub-structure-normalizer 先 dry-run 审查这本 EPUB 的结构改动，实跑后用红线校验确认正文不变。
```

## 当前 Skills

| Skill | 用途 | 能力（capability） |
|---|---|---|
| `epub-layout-auditor` | 总入口：审稿、风险分级、分派专项修复 | `epub.layout.audit` |
| `epub-content-analyzer` | 只读识别正文、标题、对话、诗歌、引文、书信、文白等结构角色，并给出字体与排版建议 | `epub.text.content.analyze` |
| `epub-source-intake` | 从文本、Markdown、HTML、PDF 或 OCR 结果建立 EPUB 制作入口（人工 + AI 流程） | `epub.source.intake` |
| `epub-structure-normalizer` | 先格式化资源目录，再按 OPF manifest id 做文件名反混淆；dry-run 审查 + 红线 `--path-map` | `epub.structure.normalize` |
| `epub3-migrator` | 旧 EPUB 先规划再迁移到 EPUB3，并执行正文红线和产物验证 | `epub.package.migrate.epub3` |
| `epub-css-layering-optimizer` | 维护 `fonts/base/notes/effects/literary/media/vertical/poster.css` 分层 | `epub.css.layering.optimize` |
| `epub-typography-optimizer` | 中文正文节奏、字体链、嵌入字体和生僻字 fallback（preset 见能力参数） | `epub.typography.optimize` |
| `epub-font-coverage-analyzer` | 只读检查嵌入字体 cmap、字体链命中、生僻字和 Kindle 回退风险 | `epub.font.coverage.analyze` |
| `epub-english-typography-optimizer` | 英文书籍类型判断、serif 链、段落节奏、断字和大字号回归 | `epub.typography.english.optimize` |
| `epub-literary-structure-formatter` | 章首、章节头图、题记、前置页、对话、诗、信件、文白对照、场景分隔 | `epub.literary.structure.format` |
| `epub-image-layout-optimizer` | figure 环绕、图注、封面声明、图片格式兼容 | `epub.image.layout.optimize` |
| `epub-vertical-ruby-optimizer` | 竖排正文、Ruby 注音、中西文方向 | `epub.vertical.ruby.optimize` |
| `epub-kindle-compatibility-checker` | Kindle/KDP 风险、转换日志、WebP/SVG/cover/MathML 检查 | `epub.kindle.compatibility.check` |
| `epub-package-nav-auditor` | OPF manifest/spine、nav、NCX、cover、MathML/SVG properties | `epub.package.nav.audit` |
| `epub-package-operator` | 合并、拆分、元数据和封面写操作；始终输出新 EPUB | `epub.package.merge` / `epub.package.split` / `epub.metadata.edit` / `epub.cover.replace` |
| `epub-alite-converter` | 封面、卷首、章首或海报页转 A-lite 可重排全页方案 | `epub.alite.convert` |
| `epub-popup-footnote-converter` | 普通注释/尾注/旧注释转标准 grouped popup footnote | `epub.notes.popup.normalize` |
| `epub-legacy-footnote-fallback` | 在标准弹注上叠加多看旧版兼容 fallback | `epub.notes.legacy-fallback` |
| `epub-style-demo-maintainer` | 维护 demo fixture、reader matrix、SPEC 和最终文档同步 | `epub.style.demo.maintain` |

个别能力 Go 实现仍在迁移中：`epub run` 该能力时 findings 会出现 `warn capability.not-implemented`，此时以对应 skill 描述的人工/分析流程为准，`epub capabilities` 可随时确认最新状态。

## 两类常见场景

已有 EPUB：

1. `epub run epub.layout.audit --input book.epub --json` 总审，`epub-layout-auditor` 判断风险和专项 skill。
2. 结构先规范化：`epub-structure-normalizer` 双阶段（dry-run review → 实跑 → 红线 `--path-map`）。
3. 用字体、图片、弹注、Kindle、OPF/nav 等专项 skill 修复。
4. 每次改书后 `epub redline --check all <before> <after>`，再做人工 diff review（Calibre Editor 或 VS Code）。

没有 EPUB，只有文本或 PDF：

1. `epub-source-intake` 按人工 + AI 流程抽取、结构化、抽样校对，产出可排版 source bundle。
2. 形成 XHTML/Images/OPF/nav 后，进入 `epub-package-nav-auditor` 和排版专项 skill。
3. PDF 抽取、OCR 和图片压缩使用外部工具；本仓只记录边界、检查风险和验证 EPUB 结构/排版。

## 维护规则

- 不改 `SKILL.md` frontmatter 的字段名，只保留 `name` 和 `description`。
- `SKILL.md` 固定四段：`## 何时用`、`## 调什么`、`## 返回怎么读`、`## 依据返回怎么判断`；调用命令只写 `epub run <capability-id>` 形态，引用的能力 id 必须在 `contracts/capabilities/v1/` 真实存在。
- `description` 写触发场景和目标能力；`agents/openai.yaml` 只用扁平字符串 metadata，必须和 `SKILL.md` 的用途保持一致。
- 新增技能先判断是否只是样式样本：如果只是验证一个样式，优先放进 `templates/epub-style-demo/`。
- 修改结构性规则时，同步检查 `docs/final/EPUB 3 HTML CSS 属性速查表.html`、`docs/final/SPEC-实现约束.md` 和 `templates/epub-style-demo/`。
- 不在 skill 里写下游引擎架构或平台分发逻辑；skills/ 下不新增可执行脚本。
