---
name: epub-style-demo-maintainer
description: Maintain the epub-style-demo compatibility fixture, reader matrix, final rules, and validation loop when EPUB reader behavior changes or new demo coverage is needed.
---

# EPUB Style Demo Maintainer

Use this skill when changing `templates/epub-style-demo/`, adding reader compatibility cases, or turning a reader finding into final EPUB production rules.

## Fixed Workflow

1. Start with `templates/epub-style-demo/`. Add or update the smallest fixture that exposes the reader behavior.
2. Build the demo:

```sh
sh templates/epub-style-demo/build.sh
```

3. Validate structure with the stdlib-only script:

```sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

4. Record the artifact in `docs/final/reader-matrix.yaml`. Use `warn` with pending reader version when human retest is still needed; do not invent pass/fail.
5. Only after a fixture and matrix entry exist, update `docs/final/SPEC-实现约束.md`, then the final handbook and quick reference table.
6. If the rule affects automation behavior, update relevant `skills/*/SKILL.md` without changing frontmatter keys.

## Current Compatibility Rules

- Image wrapping uses `figure.img-left` / `figure.img-right` as the main path. Float and fixed pixel width belong on `figure`; the nested `img` is `width:100%; height:auto`.
- Direct `img` float is not the main path because it can render too small in some readers.
- Wrapping tests need enough surrounding prose. A short paragraph is a threshold counterexample, not proof that float failed.
- Wavy underline must be split: `text-decoration: underline;` first, then `text-decoration-style: wavy;`. Kindle App fallback is ordinary underline.
- XHTML files containing MathML must have `properties="mathml"` in the OPF manifest.
- MathML coverage should stay inside KDP Enhanced Typesetting and EPUB 3 supported tags unless a new experiment justifies otherwise.
- Duokan legacy fallback uses `ol.footnote-list.duokan-footnote-content`; individual `li.footnote-item` only get `duokan-footnote-item`.

## Validation Expectations

Before committing, run at minimum:

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
git diff --check
```

If `xmllint` is unavailable, the Python validation script still parses XML with stdlib and catches manifest/link/MathML errors.
