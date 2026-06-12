# Changelog

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
