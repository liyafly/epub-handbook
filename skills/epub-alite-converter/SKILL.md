---
name: epub-alite-converter
description: Convert EPUB cover-like, volume-opening, chapter-opening, or poster-style pages to the project's A-lite reflowable full-page scheme while preserving the existing text/image overlay composition and assets.
---

# EPUB A-lite Converter

Use this skill when converting an existing EPUB cover-like page, volume page, chapter title page, or poster page into the project's final A-lite scheme.

## Fixed Target

Use the project A-lite scheme exactly:

- reflowable EPUB page
- `body.fullpage`
- `.fullframe`
- `min-height: 100%`
- `font-size: 16px`
- `overflow: hidden`
- `page-break-before/after/inside`
- `background-image` on `body.fullpage` when the page has a full-page background
- `writing-mode: vertical-rl` for vertical text
- `float: right` for vertical columns
- no FXL conversion
- no `vh` / `vw`
- no absolute positioning

## Workflow

1. Read the source XHTML, linked CSS, OPF manifest/spine, and assets for the target page.
2. Identify the existing composition:
   - background image
   - overlay text blocks
   - overlay decorative images
   - vertical columns
   - font names and embedded font usage
3. Preserve the composition. Do not redesign, reorder, rewrite copy, swap images, or invent new ornaments.
4. Convert the page shell:

```html
<body class="fullpage poster-bg">
  <section class="fullframe" epub:type="chapter">
    ...
  </section>
</body>
```

5. Convert vertical overlay text to floated vertical columns:

```css
.fullframe .vcol {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  float: right;
  text-indent: 0;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}
```

6. Convert positioning:
   - keep relative visual order
   - convert large fixed offsets into `%` margins
   - convert title sizes into `%` or `em`
   - keep internal base `font-size: 16px`
7. Preserve embedded fonts. For locked title fonts, use the book's internal font name first, normally with only `serif`/`sans-serif` as generic fallback.
8. Update OPF manifest only for assets/CSS/fonts that are actually used.
9. Validate by reading the resulting XHTML/CSS and checking that no required overlay text/image was dropped.

## A-lite CSS Skeleton

```css
@page { margin: 0; padding: 0; }

html {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
}

body.fullpage {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  font-size: 16px;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;
  box-sizing: border-box;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-page-break-inside: avoid;
  overflow: hidden;
  background-repeat: no-repeat;
  background-position: left bottom;
  background-size: 80% auto;
}

.fullframe {
  width: 100%;
  height: auto;
  min-height: 90%;
  margin: 0;
  padding: 0;
  overflow: hidden;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}
```

## Guardrails

- Do not convert to fixed layout.
- Do not replace the user's A-lite scheme with padding-ratio, FXL, or full-image-only pages.
- Do not remove overlay text just because it is difficult to position.
- Do not add marketing-style decorations or new visual concepts.
- Do not introduce private reading-system CSS as the main path.
