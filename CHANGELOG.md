# Changelog

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
