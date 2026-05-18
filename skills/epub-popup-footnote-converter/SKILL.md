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
- note body is `<aside epub:type="footnote" role="doc-footnote">`
- backlink is `◎`
- noteref and aside stay in the same XHTML file
- note content text is preserved exactly
- no private note mechanism as the main path

## Conversion Workflow

1. Read the XHTML file containing the note reference and note body.
2. Preserve existing note ids when possible. Normalize only when ids collide or are missing.
3. Replace text markers such as `[1]`, `*`, `注` with the image noteref:

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

4. Convert the note body:

```html
<aside id="footnote-1" epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line"/></div>
  <p class="footnote">
    <a class="footnote-back"
       epub:type="backlink"
       role="doc-backlink"
       href="#note-1">◎</a>
    注释内容。
  </p>
</aside>
```

5. If the source uses the demo shape with `ol.duokan-footnote-content` and `li.duokan-footnote-item`, keep the visual grouping only when needed, but make the popup target the `aside` with the target id.
6. Add `Images/note.png` to OPF manifest if it is not already listed. If the EPUB has no note icon yet, copy this skill's `assets/note.png` into the EPUB `Images/` directory.
7. Add the CSS below to the active stylesheet or merge it into the existing note section.
8. Verify every `href="#footnote-x"` resolves, every backlink resolves, and every note remains in the same XHTML file.

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

## Guardrails

- Do not replace the image icon with plain text unless no icon asset exists and the user approves.
- Do not use `display:none` on the footnote body.
- Do not move notes into a different XHTML file.
- Do not rewrite note prose.
- Do not use `duokan-wavyline`, duokan-only notes, or JS as the main mechanism.
