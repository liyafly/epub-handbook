# EPUB Capability Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate EPUB test/package primitives, split the three oversized Python entrypoints by capability, and expose concrete package/migration harnesses and skills without breaking existing CLI/import behavior.

**Architecture:** Keep `epub_package_tool.py`, `epub3_oneclick_converter.py`, and `epub_ai_harness.py` as stable façades. Move focused logic into `epub_package/`, `epub3_conversion/`, and `epub_ai/`; put shared archive/URI/XML primitives in `epub_lib.py`; route concrete write workflows through dedicated harnesses and two concise skills.

**Tech Stack:** Python 3 standard library, unittest/script regression tests, JSON capability manifests, Markdown skills, existing uv/pytest and SwiftPM validation.

**Status:** Completed and verified on 2026-06-30.

---

### Task 1: Shared EPUB test fixture

**Files:**
- Create: `scripts/test_support/__init__.py`
- Create: `scripts/test_support/epub_fixture.py`
- Create: `scripts/test_epub_fixture.py`
- Modify: all `scripts/test_*.py` files with local ZIP-writing fixture helpers

- [x] Write a failing test that imports `EpubFixture`, verifies `mimetype` is first/stored, and verifies an explicit malformed-mimetype option.
- [x] Run `uv run python scripts/test_epub_fixture.py`; confirm import failure.
- [x] Implement `EpubFixture.add_text()`, `add_bytes()`, `write()`, and `write_epub()` with deterministic member order.
- [x] Run the focused test and confirm pass.
- [x] Migrate each local fixture helper to the shared writer while preserving test-specific OPF/XHTML declarations.
- [x] Run every migrated test file and confirm pass.

### Task 2: Consolidate shared archive, XML, and URI primitives

**Files:**
- Modify: `scripts/epub_lib.py`
- Modify: `scripts/test_epub_lib.py`
- Modify: `scripts/epub_package_tool.py`
- Modify: `scripts/epub_structure_tool.py`

- [x] Add failing tests for traversal/absolute/duplicate ZIP members, external URI detection, relative URI resolution, quoting, and direct-child lookup.
- [x] Run `uv run python scripts/test_epub_lib.py`; confirm missing API failures.
- [x] Implement `validate_archive_path`, `read_epub_archive`, `find_child`, `is_external_uri`, `resolve_relative_path`, and `quote_archive_path` in `epub_lib.py`.
- [x] Replace duplicated helpers in package/structure tools with imports or error-preserving wrappers.
- [x] Run `test_epub_lib.py`, `test_epub_package_tool.py`, and `test_epub_structure_tool.py`.

### Task 3: Split package operations behind the stable façade

**Files:**
- Create: `scripts/epub_package/__init__.py`
- Create: `scripts/epub_package/models.py`
- Create: `scripts/epub_package/package_io.py`
- Create: `scripts/epub_package/references.py`
- Create: `scripts/epub_package/navigation.py`
- Create: `scripts/epub_package/merge.py`
- Create: `scripts/epub_package/split.py`
- Create: `scripts/epub_package/metadata.py`
- Create: `scripts/epub_package/cover.py`
- Modify: `scripts/epub_package_tool.py`
- Modify: `scripts/test_epub_package_tool.py`

- [x] Add failing façade tests asserting public functions come from focused modules while old imports still work.
- [x] Characterize JSON reports and error classes for merge/split/metadata/cover before moving code.
- [x] Move data models and pure helper groups first; re-export them from the façade.
- [x] Move one operation per red/green cycle: merge, split, metadata, then cover.
- [x] Reduce the façade to argparse, JSON serialization, compatibility exports, and command dispatch.
- [x] Run `uv run python scripts/test_epub_package_tool.py` after every operation move.

### Task 4: Add concrete package harnesses and skill

**Files:**
- Create: `scripts/epub_package_merge_harness.py`
- Create: `scripts/epub_package_split_harness.py`
- Create: `scripts/epub_metadata_edit_harness.py`
- Create: `scripts/epub_cover_replace_harness.py`
- Create: `scripts/test_epub_package_harnesses.py`
- Create: `skills/epub-package-operator/SKILL.md`
- Create: `skills/epub-package-operator/agents/openai.yaml`

- [x] Add failing CLI tests for required output, structured JSON, non-overwrite behavior, and delegation to focused operations.
- [x] Implement the four thin harnesses and run the focused tests.
- [x] Add failing skill/index assertions before creating the skill.
- [x] Create and validate `epub-package-operator`; ensure it routes audit requests back to `epub-package-nav-auditor`.
- [x] Run `validate_skills_basic.py` and its tests before moving to the next skill.

### Task 5: Split EPUB3 conversion behind the stable façade

**Files:**
- Create: `scripts/epub3_conversion/__init__.py`
- Create: `scripts/epub3_conversion/models.py`
- Create: `scripts/epub3_conversion/package.py`
- Create: `scripts/epub3_conversion/navigation.py`
- Create: `scripts/epub3_conversion/xhtml.py`
- Create: `scripts/epub3_conversion/notes.py`
- Create: `scripts/epub3_conversion/converter.py`
- Modify: `scripts/epub3_oneclick_converter.py`
- Modify: `scripts/test_epub3_oneclick_converter.py`

- [x] Add failing façade ownership/re-export tests and characterize the current conversion report.
- [x] Move `ConversionReport`, package pass, navigation pass, XHTML pass, and notes pass in separate red/green cycles.
- [x] Move orchestration to `converter.py`; keep the legacy script's CLI and exports.
- [x] Run `uv run python scripts/test_epub3_oneclick_converter.py` after each move.

### Task 6: Add EPUB3 apply harness and skill

**Files:**
- Create: `scripts/epub3_migration_apply_harness.py`
- Create: `scripts/test_epub3_migration_apply_harness.py`
- Create: `skills/epub3-migrator/SKILL.md`
- Create: `skills/epub3-migrator/agents/openai.yaml`

- [x] Add failing tests for explicit output, before/after digests, plan/apply report fields, and non-overwrite refusal.
- [x] Implement the apply harness over `epub3_conversion.converter.convert_epub`.
- [x] Add failing skill/index assertions before creating `epub3-migrator`.
- [x] Create and validate the skill with preflight, dry-run, apply, text-invariance and artifact validation steps.
- [x] Run skill tests/validator before changing any other skill.

### Task 7: Split the AI router

**Files:**
- Create: `scripts/epub_ai/__init__.py`
- Create: `scripts/epub_ai/model.py`
- Create: `scripts/epub_ai/detectors.py`
- Create: `scripts/epub_ai/report.py`
- Create: `scripts/epub_ai/routing.py`
- Modify: `scripts/epub_ai_harness.py`
- Modify: `scripts/test_epub_ai_harness.py`

- [x] Add failing tests for module ownership plus compatibility imports of `DETECTORS`, `detector`, `collect_actionable_findings`, `Report`, and `inspect_path`.
- [x] Move model/builders, detector registry/functions, report rendering, and routing in separate cycles.
- [x] Keep CLI parsing in the façade and re-export compatibility symbols.
- [x] Run `uv run python scripts/test_epub_ai_harness.py` after every move.

### Task 8: Capability, adapter, index, and docs registration

**Files:**
- Create: five manifests under `contracts/capabilities/v1/`
- Modify: `adapters/python/public-entrypoints.v1.json`
- Modify: `skills/README.md`
- Modify: `docs/learn/04-skills.md`
- Modify: `docs/pipeline/skills-matrix.md`
- Modify: `docs/pipeline/package-operations.md`
- Modify: `docs/pipeline/oneclick-epub3-converter.md`
- Modify: `scripts/validate_skills_basic.py`
- Modify: contract/inventory/skill tests

- [x] Add failing assertions for all five capability IDs, five harness paths, and two skill names.
- [x] Add manifests with `requiresWriteAccess=true`; do not add mutating operations to the single-artifact provider allow-list.
- [x] Register harness entrypoints and update active indexes and operation docs.
- [x] Run contract, inventory, skill, docs consistency and AI entrypoint validators.

### Task 9: Full verification and completion audit

- [x] Run every `scripts/test_*.py` with `uv run python`.
- [x] Run `cd tools-font/coverage-detector && uv run pytest -q`.
- [x] Run `cd swift && swift test`.
- [x] Build the demo and run style, popup, EPUB lint and XML validators on the artifact.
- [x] Run Markdown lint, docs/skills/contracts/inventory/AI validators and `git diff --check`.
- [x] Verify old CLI `--help`, old imported functions, new harness `--help`, skill discovery and capability manifests.
- [x] Inspect `git status` and audit every requirement in the design before marking complete.

No commit or push is performed unless the user explicitly requests it.
