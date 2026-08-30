---
name: epub-layout-auditor
description: 审核 EPUB、XHTML、CSS、OPF、nav 和模板改动的排版质量、阅读器兼容性与项目规则覆盖。用于 EPUB 排版优化前后、格式 review、识别缺失技能、分派到专项 EPUB skills、输出优先级修复计划。
---

# EPUB 排版审稿

## 何时用

- 作为 EPUB 排版工作的第一轮入口：先判断改了什么、风险在哪里、该由哪个专项 skill 处理；适用于排版优化前后、格式 review、对比基线分支或 fixture 的场景。
- 典型场景是「改造已有 EPUB」：按 `docs/final/SPEC-实现约束.md` §10 的红/黄/绿规则决定改动范围；每次改动后跑 `epub redline --check all <before.epub> <after.epub>` 红线 gate，红线触发立即回滚；人工 review 用外部 diff 工具（Calibre Editor / VS Code，见 `docs/pipeline/epub-diff-review.md`）。典型脏 epub 模式对照 `docs/pipeline/cleanup-patterns.md`。
- 审稿范围：读取改动过的 XHTML、CSS、OPF、nav、NCX、图片/字体资源，并与 `docs/final/SPEC-实现约束.md`、`docs/final/EPUB 3 终极实践手册.md`、`docs/final/EPUB 3 HTML CSS 属性速查表.md`、`templates/epub-style-demo/SCENE_MATRIX.md`、`docs/final/reader-matrix.yaml` 对照。先与用户确认只要审稿，还是审稿后直接修复。
- 禁止事项：
  - 不改写正文、不重排章节、不替换图片、不重新设计整本书，除非用户明确要求。
  - 不虚构阅读器 pass/fail；未复测时写 `warn` 或待验证说明；没有 fixture 或 matrix 记录时，不把兼容例外直接写进最终手册。
  - 局部 XHTML/CSS 能解决时，不做大范围重构；不把可重排内容转成固定版式来绕过排版问题。

## 调什么

机器可判定的结构扫描入口（只读）：

```sh
epub run epub.layout.audit --input <书> --json
```

无额外 KEY=VALUE 参数；需要旧报告形状（`recommended_skills`、`suggested_commands`、`actionable_findings`、`findings_by_level`）时加 `legacy_report=true`。涉及 demo fixture / reader-matrix 的验证由 `epub-style-demo-maintainer` 处理，不在本 skill 展开。

审稿结论落地为修复时，按第四段分派到最窄的专项 skill；每次写型改动后对产物跑：

```sh
epub redline --check all <before.epub> <after.epub>
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令（迁移期可能仍带旧执行面命令形态，仅供人参考）。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- facts 键前缀 `epub.layout.audit.`：`summary`（`zip_entries`、`manifest_items`、`spine_items`、`media_counts`、`opf`，存在时还有 `obfuscated_filenames`、`package_version`、`language`）、`input_kind`。
- findings：ID 形如 `audit.<序号>`，`title` 是检查结论，`location` 是相关资源。覆盖 manifest/spine/nav/NCX 完整性、封面声明、CSS 引用、MathML/SVG properties、文件名混淆、EPUB2 版本、noteref 无同文件 aside、疑似扫描书等结构信号。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（旧 AI 审稿报告 JSON：findings、findings_by_level、recommended_skills、suggested_commands、actionable_findings 等）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- findings 按优先级整理成修复计划：
  - P0：EPUB 无效、manifest 链接断裂、页面不可读、缺关键资源。
  - P1：阅读器兼容回归、Kindle 转换风险、注释目标不匹配。
  - P2：版式脆弱、CSS 分层漂移、字体链欠整理、fallback 弱。
  - P3：润色、命名一致性、重复规则、文档未同步。
- 分派规则（用户要修复时，切到最窄的专项 skill，保持改动范围可控）：
  - `epub-css-layering-optimizer`：CSS 文件边界、加载顺序、规则归位。
  - `epub-typography-optimizer`：中文正文节奏、字体链、嵌入字体策略。
  - `epub-english-typography-optimizer`：英文书籍类型判断、英文段落节奏、serif 链、hyphenation 和大字号回归。
  - `epub-literary-structure-formatter`：章首、前置页、对话、诗、信件。
  - `epub-image-layout-optimizer`：figure 环绕、图注、图片格式、封面资源。
  - `epub-vertical-ruby-optimizer`：竖排正文、Ruby、文字方向。
  - `epub-package-nav-auditor`：OPF manifest/spine、nav、NCX、metadata、资源引用。
  - `epub-kindle-compatibility-checker`：Kindle/KDP 静态风险与转换日志跟进。
  - `epub-popup-footnote-converter`：标准 grouped popup notes。
  - `epub-legacy-footnote-fallback`：在标准弹注上叠加多看旧版 fallback。
  - `epub-alite-converter`：可重排全页海报 / 封面式 A-lite 页面。
  - `epub-style-demo-maintainer`：fixture、reader matrix 与最终规则维护。
- 审稿清单（逐项核对，机器扫描只覆盖结构部分，其余靠阅读改动内容）：
  - XHTML 能按 XML 解析，链接只指向存在的本地资源；每个被引用的 CSS 都在 OPF manifest 声明并至少被一个 XHTML 使用；spine 的每个 idref 都能找到 manifest item，阅读顺序符合 nav/NCX 预期。
  - 注释与 noteref 位于同一 XHTML，且每个文件只使用一个 grouped aside。
  - 普通正文页不同时使用 `width:100%` 与左右 padding；图片环绕使用 `figure.img-left` / `figure.img-right`，不直接 float `img`。
  - Kindle 主路径避免 WebP 和未经验证的 SVG-only 封面路径；含 MathML 的 XHTML 在 OPF 中声明 `properties="mathml"`；CSS 改动留在正确层级文件中。
  - 阅读器结论必须能追溯到 fixture、matrix、转换日志或人工测试记录。
- `status == failed` 或 `findings` 出现 `error` → 先修结构再谈排版；`epub redline` 未通过 → 回滚或修复后重跑；`status == approval-required` → 停下来问人。
