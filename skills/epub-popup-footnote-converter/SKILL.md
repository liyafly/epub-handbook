---
name: epub-popup-footnote-converter
description: Convert EPUB note references and endnotes into the project's standard popup footnote structure using an image note icon trigger and an ◎ backlink, while preserving note content.
---

# EPUB Popup Footnote Converter

Use this skill when converting plain footnotes, endnotes, old duokan-style notes, or text-only noteref markers into the project's final popup footnote pattern.

## Fixed Target

Use this structure:

- noteref is an `<a>` with `epub:type="noteref"` and `role="doc-noteref"`
- noteref content is an image icon, normally `../Images/note.png`; this skill bundles `assets/note.png` as the default icon
- each XHTML file has one grouped note body: `<aside epub:type="footnote" role="doc-footnote">`
- all notes in that XHTML file are grouped inside `ol.footnote-list`
- each note target is a `li.footnote-item` with the target `id`
- noteref `href` points to the corresponding `li.footnote-item` id, not to a separate per-note aside
- backlink is `◎`
- noteref, target `li`, and containing aside stay in the same XHTML file
- note content text is preserved exactly
- no private note mechanism as the main path

## Conversion Workflow

1. Read the XHTML file containing the note reference and note body.
2. Preserve existing note ids when possible. Normalize only when ids collide or are missing.
3. Replace text markers such as `[1]`, `*`, `注` with the image noteref. The `href` must point to the final `li.footnote-item` target id:

```html
<sup>
  <a id="note-1"
     class="noteref-icon"
     epub:type="noteref"
     role="doc-noteref"
     href="#footnote-1">
    <img alt="注" src="../Images/note.png"/>
  </a>
</sup>
```

4. Convert all note bodies in the XHTML file into one grouped aside:

```html
<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>

  <ol class="footnote-list">
    <li class="footnote-item" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        注释内容。
      </p>
    </li>
  </ol>
</aside>
```

5. If the source uses legacy `duokan-*` note classes, keep the grouped `ol/li` structure, but rename to neutral classes such as `footnote-list` and `footnote-item`. Do not keep `duokan-*` classes as the main output.
6. Add `Images/note.png` to OPF manifest if it is not already listed. If the EPUB has no note icon yet, copy this skill's `assets/note.png` into the EPUB `Images/` directory.
7. Add the CSS below to the active stylesheet or merge it into the existing note section.
8. Verify every noteref `href="#footnote-x"` resolves to a `li.footnote-item`, every backlink resolves, every file with notes has exactly one grouped footnote aside, and every note remains in the same XHTML file.

## CSS

```css
sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon {
  text-decoration: none;
}

.noteref-icon img {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

.footnote-line {
  width: 60%;
  height: 1px;
  margin: 1.5em 0 1em -0.5em;
  border: none;
  border-top: 1px solid #777;
}

.footnote-list {
  margin: 0;
  padding: 0;
  list-style-type: none;
  text-align: left;
}

.footnote-item {
  margin: 0.4em 0;
  padding: 0;
  list-style-type: none;
}

.footnote {
  margin: 0.4em 0;
  text-indent: 0;
  font-size: 0.9em;
  line-height: 1.35;
  text-align: left;
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
```

## CSS placement

- Footnote CSS must be written to `Styles/notes.css` in this repository's layered demo.
- Do not write footnote CSS into `poster.css`.
- `@font-face` and font utility classes belong in `Styles/fonts.css`.

## Guardrails

- Do not replace the image icon with plain text unless no icon asset exists and the user approves.
- Do not use `display:none` on the footnote body.
- Do not move notes into a different XHTML file.
- Do not emit one aside per note when a file contains multiple notes; group them in one aside with `ol/li`.
- Do not rewrite note prose.
- Do not use `duokan-wavyline`, duokan-only notes, or JS as the main mechanism.
- If the target EPUB needs Duokan legacy compatibility, apply `epub-legacy-footnote-fallback` after this conversion instead of inventing new private attributes.

## Validation Fixture

Use `templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml` as the local reference shape for popup footnotes. A converted file should preserve the same broad invariants:

- noteref `href` points to a `li.footnote-item` in the same XHTML file.
- the file has one grouped `aside epub:type="footnote"` for all local notes.
- backlinks use `epub:type="backlink"` and `role="doc-backlink"`.
- the note trigger uses an image icon when the EPUB has or can receive the icon asset.

Run the stdlib-only validator after conversion:

```sh
scripts/validate-popup-notes.sh
```

For a built artifact:

```sh
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```
