# Anthology A-lite Compact Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refine an existing anthology EPUB with contained full-page A-lite volume posters, cleaner copyright pages, scoped CSS consolidation, compact pipeline reports, and a four-file local delivery bundle.

**Architecture:** Extend the existing CSS cleaner with an opt-in scoped-local consolidation pass, then add a focused anthology refinement writer that only changes proven poster/copyright page pairs. Keep package red lines intact, add a demo fixture for the contained poster variant, and make the one-command pipeline persist only one summary report by default.

**Tech Stack:** Python 3 standard library, EPUB ZIP/XML/XHTML/CSS, shell validators, Kindle Previewer CLI.

---

### Task 1: Merge Local CSS Safely

**Files:**
- Modify: `scripts/epub_css_cleanup.py`
- Modify: `scripts/test_epub_css_cleanup.py`

- [ ] Add fixture coverage for two disjoint local stylesheets and assert that opt-in cleanup emits one `Styles/clean-scoped-local.css`, body classes `css-local-01` / `css-local-02`, scoped selectors, updated links, and removed manifest items.
- [ ] Add `merge_scoped_local_css: bool = False` to `clean_epub_css(...)` and CLI flag `--merge-scoped-local-css`.
- [ ] Identify merge candidates after ordinary deduplication: referenced parseable CSS excluding `clean-shared-*`, `clean-scoped-local.css`, `epub3-enhancements.css`, and stylesheets shared by most XHTML pages.
- [ ] Skip candidate stylesheets whose referencing XHTML sets overlap. Prefix supported selectors with `body.css-local-NN`, add stable body classes, rewrite links, remove old CSS items, and add one scoped-local manifest item.
- [ ] Record `scoped_local_stylesheets_merged`, `scope_classes_added`, and warnings in the JSON report.
- [ ] Run:

```sh
python3 scripts/test_epub_css_cleanup.py
python3 -m py_compile scripts/epub_css_cleanup.py
```

Expected: both commands exit `0`.

### Task 2: Add Anthology Refinement Writer

**Files:**
- Create: `scripts/epub_anthology_refinement.py`
- Create: `scripts/test_epub_anthology_refinement.py`

- [ ] Build a synthetic EPUB fixture with two spine pairs: one poster page containing exactly one JPEG and one following copyright page with `.cp` plus `ul.list > li.i`.
- [ ] Implement dry-run discovery and `--write-output`: preserve spine/nav/NCX/text/image bytes, add `Styles/anthology-refinement.css`, rewrite only proven page pairs, and report poster/copyright counts.
- [ ] Emit `fullpage poster-bg poster-bg-volume-NNN`, `.fullframe`, `.poster-fallback`, `background-size: contain`, page-break protections, copyright body class, and compact copyright list styles.
- [ ] Run:

```sh
python3 scripts/test_epub_anthology_refinement.py
python3 -m py_compile scripts/epub_anthology_refinement.py
```

Expected: both commands exit `0`.

### Task 3: Compact Pipeline Reports

**Files:**
- Modify: `scripts/epub_cleanup_pipeline.py`
- Modify: `scripts/test_epub_cleanup_pipeline.py`

- [ ] Change `Step` so structured JSON outputs are embedded in `reports/pipeline.json`.
- [ ] Add `keep_step_reports: bool = False` and CLI flag `--keep-step-reports`.
- [ ] Persist only `pipeline.json` by default; preserve `normalize-dry-run.json` because it is an explicit review artifact.
- [ ] Verify `--keep-step-reports` restores the previous per-step report files.
- [ ] Run:

```sh
python3 scripts/test_epub_cleanup_pipeline.py
python3 -m py_compile scripts/epub_cleanup_pipeline.py
```

Expected: both commands exit `0`.

### Task 4: Add Demo Evidence and Update Contracts

**Files:**
- Create: `templates/epub-style-demo/OEBPS/Text/03c-poster-contain.xhtml`
- Modify: `templates/epub-style-demo/OEBPS/Styles/poster.css`
- Modify: `templates/epub-style-demo/OEBPS/package.opf`
- Modify: `templates/epub-style-demo/OEBPS/nav.xhtml`
- Modify: `templates/epub-style-demo/OEBPS/toc.ncx`
- Modify: `templates/epub-style-demo/README.md`
- Modify: `templates/epub-style-demo/SCENE_MATRIX.md`
- Modify: `scripts/validate_epub_style_demo.py`
- Modify: `docs/final/reader-matrix.yaml`
- Modify: `docs/final/SPEC-实现约束.md`
- Modify: `docs/final/EPUB 3 终极实践手册.md`
- Modify: `docs/final/EPUB 3 HTML CSS 属性速查表.md`
- Modify: `docs/final/EPUB 3 HTML CSS 属性速查表.html`
- Modify: `skills/epub-alite-converter/SKILL.md`
- Modify: `skills/epub-css-layering-optimizer/SKILL.md`

- [ ] Add a contained raster poster fixture beside the existing `cover` comparison.
- [ ] Extend validator assertions for `poster-bg-contain`, `background-size: contain`, fallback image, and the `@supports` fallback rule.
- [ ] Record the new reader-matrix case as `warn` until visual checks support a stronger result.
- [ ] Update final contracts and skills to distinguish `contain` text-bearing posters from `cover` crop-tolerant fullbleed posters.
- [ ] Run:

```sh
sh templates/epub-style-demo/build.sh
DEMO_EPUB=$(ls -t templates/epub-style-demo/dist/*.epub | head -1)
scripts/validate-epub-style-demo.sh --epub "$DEMO_EPUB"
scripts/validate-popup-notes.sh --epub "$DEMO_EPUB"
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
python3 scripts/validate_skills_basic.py
python3 scripts/validate_ai_entrypoints.py
```

Expected: all commands exit `0`.

### Task 5: Update Pipeline Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/pipeline/cleanup-flow.md`
- Modify: `docs/pipeline/refinement-harnesses.md`
- Modify: `docs/pipeline/oneclick-epub3-converter.md`
- Modify: `docs/pipeline/css-cleanup-system-fonts.md`
- Modify: `docs/pipeline/README.md`

- [ ] Document compact reports as the default and `--keep-step-reports` as debugging mode.
- [ ] Document `--merge-scoped-local-css` and the anthology refinement writer.
- [ ] Describe the four-file delivery bundle and the rule that historical debug directories remain local.
- [ ] Run:

```sh
git diff --check
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
```

Expected: all commands exit `0`.

### Task 6: Refine the Existing Target EPUB

**Local artifacts only:**
- Input: `work-epub/css-clean-system-fonts/after/final.epub`
- Intermediate: `work-epub/anthology-alite-compact/intermediate/css-consolidated.epub`
- Delivery: `work-epub/anthology-alite-compact/delivery/final.epub`
- Delivery metadata: `summary.json`, `notes.md`, `reader-check.txt`

- [ ] Run scoped CSS consolidation and confirm CSS count changes from `12` to `4`.
- [ ] Run anthology refinement dry-run, inspect the JSON plan, then write output and confirm `16` posters plus `16` copyright pages.
- [ ] Write only the four delivery files.
- [ ] Verify ZIP, preflight, popup notes, all red lines, OPF/nav XML, CSS links, image hashes, CSS count `5`, and delivery file inventory.
- [ ] Run Kindle Previewer CLI with temporary output outside `delivery/`, summarize status into `reader-check.txt`, then remove temporary KPF/log artifacts.
- [ ] Attempt GUI inspection with Computer Use; record success or the concrete skip reason.

### Task 7: Final Verification and Publish

**Files:**
- Review all tracked changes and local delivery files.

- [ ] Run focused regression tests:

```sh
python3 scripts/test_epub_css_cleanup.py
python3 scripts/test_epub_anthology_refinement.py
python3 scripts/test_epub_cleanup_pipeline.py
python3 scripts/test_epub3_oneclick_converter.py
python3 scripts/test_epub_cleanup_harnesses.py
python3 scripts/test_validate_popup_notes.py
python3 scripts/test_validate_text_invariance.py
python3 scripts/validate_skills_basic.py
python3 scripts/validate_ai_entrypoints.py
git diff --check
```

- [ ] Commit source, tests, docs, templates, and contracts in intentional groups.
- [ ] Push `codex/anthology-alite-delivery-cleanup` to `origin`.
