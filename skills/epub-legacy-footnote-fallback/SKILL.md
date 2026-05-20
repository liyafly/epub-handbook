---
name: epub-legacy-footnote-fallback
description: Add a compatibility fallback for Duokan and Zhangyue/iReader popup notes while keeping the project's standard EPUB 3 grouped footnote structure as the primary output.
---

# EPUB Legacy Footnote Fallback

Use this skill only when the target EPUB must preserve popup-note compatibility for Duokan and Zhangyue/iReader legacy readers. This is a compatibility layer, not the default project note pattern.

## Fixed Target

Keep the project's standard structure as the primary shape:

- noteref is an `<a>` with `epub:type="noteref"` and `role="doc-noteref"`
- noteref `href` points to a note `li` in the same XHTML file
- each XHTML file has one grouped note body: `<aside epub:type="footnote" role="doc-footnote">`
- all local notes are grouped inside `ol.footnote-list`
- each note target is a `li.footnote-item`
- backlink is `◎`

Add legacy hooks on top of that same structure:

- add `duokan-footnote` to the noteref anchor
- add `duokan-footnote-content` to each note `li` (not the grouped `ol`)
- add `duokan-footnote-item` to each note `li`
- put a note icon image inside the noteref anchor
- add `zhangyue-footnote` and `zy-footnote="plain text note"` to that image

Do not create a second note body for the fallback.

## XHTML Pattern

```html
<p>
  正文文字
  <sup>
    <a id="note-1"
       class="noteref-icon duokan-footnote"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-1">
      <img class="zhangyue-footnote"
           alt="纯文本注释内容"
           zy-footnote="纯文本注释内容"
           src="../Images/note.png"/>
    </a>
  </sup>
  继续正文。
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line"/></div>
  <ol class="footnote-list">
    <li class="footnote-item duokan-footnote-item duokan-footnote-content" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        富文本注释内容。
      </p>
    </li>
  </ol>
</aside>
```

## Conversion Workflow

1. Start from a file that already follows, or can be converted to, the standard `aside > ol.footnote-list > li.footnote-item` pattern.
2. Preserve ids when possible. Ensure noteref ids and note target ids are unique inside the XHTML file.
3. Add `duokan-footnote` to each noteref anchor without removing `epub:type`, `role`, `id`, or `href`.
4. Ensure the noteref anchor contains an image icon. Use `../Images/note.png` when adding a new asset for legacy fallback.
5. Add `class="zhangyue-footnote"` and a `zy-footnote` attribute to that image.
6. Build the `zy-footnote` value from a plain-text version of the note body: strip markup, collapse whitespace, and XML-escape attribute characters.
7. Add `duokan-footnote-content` to each `li.footnote-item`; do not put it on `ol`.
8. Add `duokan-footnote-item` to each `li.footnote-item`.
9. Add `Images/note.png` to the OPF manifest if missing.
10. Verify all href/backlink targets resolve inside the same XHTML file.

## CSS

Merge these rules into the active note CSS when the EPUB does not already style these classes:

```css
.noteref-icon,
a.duokan-footnote {
  text-decoration: none;
}

.noteref-icon img,
a.duokan-footnote img,
img.zhangyue-footnote {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

.duokan-footnote-content {
  margin-top: 0;
}

.footnote-item.duokan-footnote-content {
  margin: 0;
}

.footnote-item.duokan-footnote-item {
  list-style-type: none;
}
```

If the source does not already use `.footnote-line`, add a visible separator with either a `footnote-line` rule or a border on `.duokan-footnote-content`; do not use both in the same visual path.

## Guardrails

- Do not use this skill for the normal project output; use `epub-popup-footnote-converter` instead.
- Do not remove neutral classes such as `footnote-list` and `footnote-item`.
- Do not keep only `duokan-*` classes.
- Do not duplicate note prose in a second visible note list.
- Do not place `duokan-footnote-content` on grouped `ol`; otherwise Duokan may pop the entire list.
- Do not use JavaScript or `display:none` note bodies.
- Do not put HTML markup inside `zy-footnote`; it must be plain text.
- If a note contains rich markup, keep the rich version in the grouped note body and put only a plain-text summary in `zy-footnote` and `alt`.

## Validation Fixture

Use these local reference shapes:

- `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml` for a compact compatibility example.
- `templates/epub-style-demo/OEBPS/Text/06-multi-legacy-note-fallback.xhtml` for multiple fallback notes sharing one grouped list in the same XHTML file.


### Multi-note validation

- In one XHTML file with multiple notes, each trigger must open only its targeted `li` content.
- Zhangyue path should use only `zy-footnote` plain text from the icon attribute.
- Standard EPUB path must resolve by href -> target id exactly.
