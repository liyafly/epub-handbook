# Changelog

## Unreleased

### Changed

- **仓库分层与文档精简**：根 README 收敛为“做书 / 修书 / 查问题”三条普通人入口，
  AI 与专业维护统一路由到 `AGENTS.md` 和 `scripts/README.md`。
- `docs/learn/` 从线性 00–09 教程收敛为唯一入口、做书页、进阶结构兼容页和按需查页面。
- 已完成的 `docs/meta/`、`docs/experiments/`、`docs/source/` 迁入 `archive/`；
  未跟踪的 `docs/superpowers/` 退出仓库工作树。
- 归档历史流水线决策，修正归档索引、贡献指南与 CI 中残留的旧目录入口。
- 将匿名技术书 v3.1 的长 MathML 表格与图片宽度实测分层回写到脱敏 demo、reader
  matrix、场景指南、手册和排版决策记录；精确 pass 只绑定已验证 artifact SHA，
  新 demo 保持 `warn` 待复测。
- 将匿名插图型英文合集的人工验收结论回填到英文排版指南、终极手册与排版决策记录；
  保留正文自由与展示角色分层方法，不把单书数值或缺少版本信息的反馈写成阅读器 `pass`。
- `gui/` 保持 PARKED，`references/` 保持现状，Python 与 Swift 继续按 capability 并存。

## v0.2.6 - 2026-06-25

> ⚠️ 字体工具仍在完善中

### Added

- `tools-font/` 字体工具目录：
  - `font-preview.html`：单文件离线字体预览工具（拖入 .ttf/.otf，多字体对比，手写 SFNT name 表解析器）
  - `font-coverage-viewer.html`：字体覆盖报告查看器（拖入 JSON + EPUB/字体，生僻字清单、内嵌字体对比、修复建议）
  - `coverage-detector/`：EPUB 字体覆盖检测器 Python CLI（uv + fontTools/tinycss2/lxml，8 模块 2,800+ 行）
- `docs/how-to/kindle-font-rendering-deep-dive.md`：Kindle 字体渲染深度参考
- `docs/meta/`：治理索引桶

### Changed

- **仓库整合**：docs/ 从 84 文件 ~29k 行收敛到 47 文件 ~8k 行
- 目录重排：`getting-started/` → `learn/`，`guides/` → `how-to/`
- 删除 `plans/`、`archive/`、`architecture/`（历史留 git）
- `AGENTS.md` 新增架构分工表，优先级从 8 级压到 3 档
- `README.md` 双引擎执行层体现 Python/Swift parity
- `gui/README.md` 顶部 PARKED 标注
- `docs/superpowers/` 加入 .gitignore

### Fixed

- `kindle-pessimistic` 画像修正：`only-non-embedded` 从 fail → risk（系统字体实际可用）
- resolver：`@font-face` 仅 local() 的不标 embedded
- harvester：`<ruby>` 标签中的 Ext B 生僻字采集
- `requirements.txt` 删除（统一走 uv）

## v0.2.3 - 2026-06-12

### Added

- `scripts/epub_lint.py`：通用 SPEC 规则机检（v0 共 10 条规则，覆盖 §1/§2/§3/§5.7/§5.8/§8），可对任意 EPUB 运行；配套回归测试。
- `templates/book-starter/`：最小成书骨架（标题页 + 一章 + nav + NCX，预装 literary-cn preset，自由模式），新增入门页 09 讲解十分钟出书路径。

### Changed

- AGENTS.md 最小验证矩阵纳入 epub-lint 与 epubcheck 运行政策；skills/README 推荐顺序同步。
- demo-scene-expansion-plan 标注旧 `ibooks:specified-fonts` 口径已被 SPEC §8 取代；demo validator 的 Java 探测对 macOS 占位 java 免疫。

## v0.2.2 - 2026-06-12

### Fixed

- 统一 `ibooks:specified-fonts` 条件规则：修正 SPEC §3、手册 §一 / §4.2、demo fonts.css 注释、typography skill、入门教程中残留的「始终保留」旧表述。
- `epub3_oneclick_converter.py` 不再无条件注入 `ibooks:specified-fonts=true`，改为检测 `body-font-locked` 后按需添加，并补充自由 / 锁定两个回归用例。
- SPEC §8 补全嵌入字体分支（未实测，暂按保守口径添加，待 Apple Books 实测后修订，见 reader-matrix 待测条目），明确正文字体模式为全书级决策；demo 演示书的混合页面口径写入 demo README。
- demo SCENE_MATRIX / README、三个 style preset README 同步新规则；`.body-font-locked` 并入宋体选择器组；reader-matrix 将字体模式行为登记为待实测假设。

## v0.2.1 - 2026-06-10

### Changed

- **Body font is now free by default.** `base.css` no longer sets `font-family` on `body`, letting reader font settings take effect. This is the more reader-friendly behavior seen in well-made Chinese EPUBs.
- `ibooks:specified-fonts` is now conditional: only set to `true` when the publisher opts into font locking via `body.body-font-locked`.

### Added

- `.body-font-locked` utility class in all `fonts.css` presets. Add it to `<body>` to lock the text font to the cross-platform system chain and prevent reader font switching.
- Demo page `07-font-family-order.xhtml` now uses `body-font-locked` to demonstrate the locked mode in action.

### Updated

- SPEC §8 documents the free/locked body font distinction.
- EPUB 3 handbook §三, quick-reference cheatsheet §4.1, and the typography-optimizer skill all reflect the new free-by-default behavior.

## v0.2.0 - 2026-06-10

### Highlights

- Add reusable typesetting decision records with validated JSONL add/list/match commands.
- Add a read-only image layout advisor with traceable candidates, Markdown decision templates, and cleanup-pipeline integration.
- Add literary, classical annotated, and academic Chinese style presets with class coverage analysis and redline-safe application.
- Extract shared standard-library EPUB package helpers into `scripts/epub_lib.py`.
- Archive obsolete plans, tighten popup-note rule drift checks, and clarify the newcomer reading path.

### Fixes

- Align text-invariance checks with NFC Unicode normalization.
- Replace duplicate EPUB3 nav manifest items with one generated nav item.
- Reject zip-slip member paths in popup-note EPUB validation.
- Report detector/read failures to stderr instead of silently dropping them.
- Rewrite `srcset` URLs during structure normalization.

### Hardening

- Make one-click EPUB writes atomic and keep NCX updates transactional.
- Clean failed cleanup-pipeline outputs before reruns.
- Keep cleanup-loop state and `epubcheck_ok` report schema consistent.
- Expand skill contract validation to all 15 skills.

### Tests and CI

- Add regression tests for preflight, EPUB3 migration, refinement, popup-note validation, text invariance, detector failures, and `srcset` rewriting.
- Add Markdown lint and demo-books EPUBCheck gates to CI.
- Run every `scripts/test_*.py` test in CI and trigger it for style-preset changes.
- Document local hook vs CI coverage and docs/final quick-reference HTML sync expectations.

## v0.1.0 - 2026-06-01

Initial public release of the EPUB handbook and cleanup toolkit.

### Highlights

- Documents practical EPUB authoring, typography, compatibility, and reading-system behavior across Apple Books, Kindle, Readium, and Readest.
- Provides EPUB preflight, structure normalization, EPUB3 migration, popup-note validation, refinement recommendations, and redline text-invariance checks.
- Adds an optional CSS cleanup tool for repeated stylesheets and system-first CJK font chains.
- Keeps book-specific cleanup artifacts local and documents the boundary between reusable automation and per-title editorial review.
