# EPUB Text Content Analysis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add a read-only EPUB/source text analyzer that classifies structural roles and recommends font/typography roles, integrate font coverage analysis, repair all skill/reference drift, and prove the result with full regression tests.

**Architecture:** A stdlib-only analysis core and CLI emit stable JSON/Markdown reports. Skills interpret uncertain Chinese content and dispatch existing transformers; font cmap/CSS coverage remains in its independent uv project behind a thin public adapter. Documentation consistency is enforced by a repository validator.

**Tech Stack:** Python 3 standard library, existing EPUB helpers, JSON capability contracts, Markdown skills, uv/pytest for coverage-detector, SwiftPM regression tests.

**Status:** Completed and verified on 2026-06-28.

---

### Task 1: Text block extraction and feature model

**Files:**
- Create: `scripts/epub_content_analysis.py`
- Create: `scripts/test_epub_content_analyzer.py`

- [x] **Step 1: Write failing extraction tests**

```python
def test_extract_xhtml_blocks_preserves_locators_and_context():
  blocks = analyze_xhtml("Text/ch01.xhtml", XHTML_SAMPLE)
  assert blocks[0]["locator"] == "/html[1]/body[1]/h1[1]"
  assert blocks[1]["previous_tag"] == "h1"
```

- [x] **Step 2: Run the focused test and confirm failure**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: import or assertion failure because the analyzer does not exist.

- [x] **Step 3: Implement focused models and extractors**

```python
@dataclass(frozen=True)
class TextBlock:
  source: str
  locator: str
  tag: str
  classes: tuple[str, ...]
  text: str
  language: str | None
  previous_tag: str | None
  next_tag: str | None
```

Implement XML/HTML, Markdown and plain-text extraction without modifying input.

- [x] **Step 4: Run tests and confirm pass**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: extraction and locator tests pass.

### Task 2: Chinese-aware role classification and typography advice

**Files:**
- Modify: `scripts/epub_content_analysis.py`
- Modify: `scripts/test_epub_content_analyzer.py`

- [x] **Step 1: Add failing classification tests**

```python
def test_explicit_structure_beats_content_heuristics():
  result = classify(block(tag="figcaption", text="第一章"))
  assert result.primary_role == "caption"

def test_ambiguous_short_chinese_stays_reviewable():
  result = classify(block(tag="p", text="春风又绿江南岸"))
  assert result.review_required is True
  assert "unknown" in result.candidate_roles
```

Cover body, heading, subtitle, dialogue, quotation, epigraph, verse, letter, list, caption, note, code, classical, modern translation, scene break and unknown.

- [x] **Step 2: Confirm the new tests fail**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: missing classifier/advisor failures.

- [x] **Step 3: Implement evidence-weighted classification**

```python
ROLE_FONT = {
  "body": "inherit", "heading": "ht", "subtitle": "kt",
  "quotation": "kt", "epigraph": "kt", "verse": "kt",
  "code": "mono", "classical": "st", "modern-translation": "kt",
}
```

Explicit semantics receive high confidence; content-only matches remain medium/low and reviewable. Emit evidence strings and reflow-safe paragraph advice.

- [x] **Step 4: Confirm all role tests pass**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: all tests pass.

### Task 3: EPUB/CLI/report/privacy support

**Files:**
- Create: `scripts/epub_content_analyzer.py`
- Modify: `scripts/epub_content_analysis.py`
- Modify: `scripts/test_epub_content_analyzer.py`

- [x] **Step 1: Add failing EPUB and CLI tests**

```python
def test_json_report_omits_raw_text_by_default(tmp_path):
  report = run_cli(make_epub(tmp_path), "--format", "json")
  assert "完整私有正文" not in report.stdout
  assert report.json["summary"]["review_required"] >= 0
```

- [x] **Step 2: Confirm failures**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: CLI entrypoint missing.

- [x] **Step 3: Implement CLI**

Support `--format json|md`, `--output`, `--include-snippets`, EPUB preflight failures, and deterministic report ordering. Reuse `epub_lib.py` for ZIP/container/OPF access.

- [x] **Step 4: Run focused tests**

Run: `python3 scripts/test_epub_content_analyzer.py`
Expected: all CLI/privacy/error tests pass.

### Task 4: Content analyzer skill and capability integration

**Files:**
- Create: `skills/epub-content-analyzer/SKILL.md`
- Create: `skills/epub-content-analyzer/agents/openai.yaml`
- Create: `contracts/capabilities/v1/epub.text.content.analyze.json`
- Modify: `adapters/python/provider-catalog.v1.json`
- Modify: `adapters/python/public-entrypoints.v1.json`
- Modify: `skills/README.md`
- Modify: `docs/learn/04-skills.md`
- Modify: `docs/pipeline/skills-matrix.md`
- Modify: `scripts/validate_skills_basic.py`
- Modify: integration tests for contracts, entrypoints and skills

- [x] **Step 1: Add failing registry assertions**

```python
assert "epub-content-analyzer" in parsed_skill_tables
assert "epub.text.content.analyze" in public_capabilities
```

- [x] **Step 2: Confirm registry tests fail**

Run: `python3 scripts/test_validate_skills_basic.py && python3 scripts/test_validate_contracts.py`
Expected: missing skill/capability failures.

- [x] **Step 3: Add skill, manifest and allow-listed provider**

The skill must explain confidence, Chinese ambiguity, privacy, and the no-write boundary. Register the CLI with `{artifact.path} --format json` arguments.

- [x] **Step 4: Confirm registry tests pass**

Run the skill, contract, inventory, adapter and CLI tests.

### Task 5: Font coverage public integration

**Files:**
- Create: `scripts/epub_font_coverage_adapter.py`
- Create: `scripts/test_epub_font_coverage_adapter.py`
- Create: `skills/epub-font-coverage-analyzer/SKILL.md`
- Create: `skills/epub-font-coverage-analyzer/agents/openai.yaml`
- Create: `contracts/capabilities/v1/epub.font.coverage.analyze.json`
- Modify: provider/inventory/skill indexes and `scripts/epub_refinement_harness.py`

- [x] **Step 1: Write failing adapter and recommendation tests**

```python
def test_adapter_invokes_uv_project_and_returns_json(fake_run):
  assert run_adapter(Path("book.epub"))["schema_version"] == "1.0"

def test_refinement_recommends_coverage_for_embedded_fonts():
  assert "$epub-font-coverage-analyzer" in recommendation_skills(report)
```

- [x] **Step 2: Confirm failures**

Run focused adapter and refinement tests.

- [x] **Step 3: Implement thin subprocess adapter**

Invoke `uv run python -m src.cli <epub> --output <temp.json>` with cwd `tools-font/coverage-detector`; never import its internals from `scripts/`.

- [x] **Step 4: Confirm focused and detector tests pass**

Run: `python3 scripts/test_epub_font_coverage_adapter.py` and `cd tools-font/coverage-detector && uv run pytest -q`.

### Task 6: Split the coverage CSS resolver without behavior change

**Files:**
- Create: `tools-font/coverage-detector/src/css_selectors.py`
- Create: `tools-font/coverage-detector/src/css_fonts.py`
- Modify: `tools-font/coverage-detector/src/resolver.py`
- Modify/add resolver unit tests

- [x] **Step 1: Add public behavior characterization tests**

```python
def test_selector_and_font_face_public_results_are_stable():
  assert resolve_chains(FIXTURE) == EXPECTED_CHAINS
```

- [x] **Step 2: Run characterization tests**

Run: `uv run pytest -q`; expected current tests pass before refactor.

- [x] **Step 3: Move selector parsing and font-face parsing into focused modules**

Keep `resolve_chains` and `build_font_face_registry` import compatibility in `resolver.py`.

- [x] **Step 4: Run the full detector suite**

Run: `uv run pytest -q`; expected no behavior change.

### Task 7: Audit and repair every skill and active reference

**Files:**
- Modify: affected `skills/*/SKILL.md`, `agents/openai.yaml`, templates and validators
- Create: `scripts/validate_docs_consistency.py`
- Create: `scripts/test_validate_docs_consistency.py`

- [x] **Step 1: Add failing consistency tests**

```python
def test_active_font_aliases_and_contract_targets_are_current():
  assert validate_repo(ROOT) == []
```

Checks include missing local targets, forbidden active font aliases, body free/locked rule tokens, Markdown links, HTML cheatsheet wording, skill/index membership and stale removed paths.

- [x] **Step 2: Confirm failures on current repository**

Run the new validator and existing skill validator; expected failures match the audit.

- [x] **Step 3: Repair all reported active files**

Fix the typography contract target, lint docstring, template comments/classes, handbook base CSS, HTML cheatsheet wording, beginner line-height rule, reader-matrix duplicate/date, and ignored local spec links/status where applicable.

- [x] **Step 4: Re-run validators until clean**

Run docs consistency, skills, AI entrypoints, contracts and inventory validators.

### Task 8: Full completion audit

**Files:**
- Modify: `docs/meta/2026-06-28-text-content-analysis-implementation-plan.md` checkbox state only if useful

- [x] **Step 1: Run Python regression tests**

Run every `scripts/test_*.py` with the repository-approved Python environment; retry under `uv run python` if system Python differs.

- [x] **Step 2: Run independent font tests**

Run: `cd tools-font/coverage-detector && uv run pytest -q`.

- [x] **Step 3: Build and validate demo artifacts**

Run demo build, demo validator, popup validator and `epub_lint.py` on the produced artifact.

- [x] **Step 4: Run Swift tests**

Run: `cd swift && swift test`.

- [x] **Step 5: Run repository gates**

Run `git diff --check`, skill/contracts/inventory/AI validators, inspect status, and verify every explicit requirement against current files and outputs.
