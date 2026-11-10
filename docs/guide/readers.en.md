# Readers

[中文](readers.md) · [Guide home](README.en.md)

Three readers come first: **Apple Books, Kindle and Reeden**. A book that breaks on any of them is not ready. Readest is a secondary target: the basic version only has to read well.

## Choosing a font while reading

| You want | Reader | Setting |
| --- | --- | --- |
| Your own preferred font | Reeden, Readest | Use the book's free version and change the font in the reader |
| The book's own fonts | Apple Books | Choose the Original font |
| The book's own fonts | Kindle | Choose Publisher Font |
| The book's own fonts | Duokan | Choose the default font |

Why: when you change the font in Apple Books, specially designed fonts such as letters and headings are replaced too. Reeden changes only the body and keeps those fonts. This has not been recorded separately for Readest.

With those settings, a locked version shows its embedded body font. A free version has no embedded body font, so the body uses the reader's default. Production rules are in [SPEC §8](../final/SPEC-实现约束.md#8-字体链规则) (Chinese).

## Which devices to check

- **iPhone**: the narrowest screen, where recipes use their fallback. This is the one that decides.
- **macOS**: a wide screen, where you see the enhancements.

Check both. iPad is not tested separately.

## Apple Books

- On a Mac, double-click the EPUB to import it. To get it onto an iPhone, use AirDrop or iCloud Drive.
- Apple Books caches old versions. Delete the old copy from your library before importing a new one.
- Every import adds another copy, so use Reeden for day-to-day previews and save Apple Books for the final check.

## Kindle

- Send the EPUB with **Send to Kindle** (app, web or email). Amazon converts it in the cloud, and **the book that arrives on your device is the one that counts**.
- Kindle Previewer is only a local approximation; its result can differ from Amazon's conversion.
- Send to Kindle has a file-size limit. See Amazon's current documentation for the number.
- Every build runs a static Kindle check on each output. The terminal shows one line with the error, warn and info counts; details are in `.epub/report.html`. It only reports and does not block, except that an error (a WebP image, for example) fails the build of a reader version named `kindle`, and `out/` stays unchanged. `readers` in `book.json` is only a note and does not affect the check.
- Use JPEG or PNG for images. WebP does not work on Kindle and SVG is unreliable; see [SPEC §5.1](../final/SPEC-实现约束.md#51-图片格式兼容) (Chinese).
- When only styles differ, prefer `@media amzn-kf8` in the generic version; other readers skip it. The Kindle check that runs on every build reports a warning for it, which is expected. When content must differ too, use a reader version (below). Do not use this query for parallel text; see [SPEC §5.6.1](../final/SPEC-实现约束.md#561-文白对照左右兼容) (Chinese).

## Reeden

- On a Mac you can preview straight into it: `epub book preview my-book --open=Reeden`.
- Setting `font-size` on `html` or `:root` stops some readers, Reeden included, from changing the font size. The build already rejects it; see [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放) (Chinese).
- Reeden has its own extensions (bleed, full-screen images and more); see [Reeden extensions](../../references/reeden-extensions.md) (Chinese). Use them only in a Reeden version, and only after you have tested them. Keep them out of the generic version.

## Readest

The generic version just has to read well. Readest recognises books by their content, so after re-importing an edited book it may still open the old one. If you see an old result, first make sure you opened the new file, then look at the CSS.

## Duokan

Not a main target. Reset its layout settings before checking: custom spacing or first-line indent can break flush-left lines and line spacing, which is easy to mistake for a style bug. No CSS can target Duokan alone, so any Duokan-only change needs its own version, such as the rule-free [letter](../../templates/recipes/letter-paper/recipe.md) package.

## Fixed layout

In a book that is fixed layout throughout, every page has a fixed size; the reader scales whole pages, and readers cannot change the font size or line spacing. Reeden, Apple Books and Kindle can open such books, but reader versions, spread behavior and the Kindle delivery route have not been tested and recorded yet; see [SPEC §5.13](../final/SPEC-实现约束.md#513-固定版式整本-pre-paginated) (Chinese). The text layer is exactly what readers see, so proofread it page by page before delivery.

## One source, several reader versions

Start with one generic `book.epub`, and write every special style as "fallback first, enhancement second". For most books that is enough.

When one reader shows the generic version correctly but badly, give that reader its own version. Each version is "generic + one overlay":

- an overlay is a folder that mirrors the layout of `epub/`;
- it may contain **only CSS files and the OPF**. XHTML, images, fonts or navigation files in an overlay make the build fail;
- so every version has the same text, and the build checks that word for word.

In `book.json`:

```json
{
  "version": 1,
  "title": "My Book",
  "output": "book.epub",
  "fonts": [],
  "readers": ["apple", "kindle", "reeden"],
  "variants": {
    "kindle": { "overlay": "variants/kindle" }
  }
}
```

Then put the Kindle styles in `my-book/variants/kindle/OEBPS/Styles/reader.css`. The build writes `out/book.epub` and `out/book-kindle.epub`.

- `output` sets a custom file name; the default is `book-<name>.epub`.
- `"fonts": []` in a version means "embed no fonts, let the reader choose". Leave it out to use the generic fonts.
- `readers` is a note for people; the tool does not act on it. Only entries under `variants` produce files.
- If any version fails a check, nothing in `out/` is updated.

### reader.css

New books from `epub book new` come with an empty `Styles/reader.css`, loaded last in every chapter. A reader version only has to replace that one file to override the rules before it.

Books imported with `epub book import` get one too (next to the book's existing stylesheets). For a book imported earlier, run `epub book reader-layer my-book`: it lists the file in the OPF and links it as the last stylesheet of every chapter; if any page cannot be edited safely, no file is changed.

Do not copy a whole stylesheet into the overlay and edit it: you would have to keep two copies in step. The overlay's `reader.css` holds only the rules that differ for that version.

### Decorative pages that look wrong on Kindle

If a title page or chapter opener cannot be made to look right on Kindle, give the Kindle version a full-page image of it instead: see the [page-image](../../templates/recipes/page-image/recipe.md) recipe. Other readers keep the text version; body text is never turned into images.
