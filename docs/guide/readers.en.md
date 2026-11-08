# Readers

[中文](readers.md) · [Guide home](README.en.md)

Three readers come first: **Apple Books, Kindle and Reeden**. A book that breaks on any of them is not ready. Readest is a secondary target: the basic version only has to read well.

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
- Use JPEG or PNG for images. WebP does not work on Kindle and SVG is unreliable; see [SPEC §5.1](../final/SPEC-实现约束.md#51-图片格式兼容) (Chinese).
- Do not write Kindle-only queries such as `@media amzn-kf8`. If Kindle needs different styles, give it its own reader version (below).

## Reeden

- On a Mac you can preview straight into it: `epub book preview my-book --open=Reeden`.
- Setting `font-size` on `html` or `:root` stops some readers, Reeden included, from changing the font size. The build already rejects it; see [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放) (Chinese).
- Reeden has its own extensions (bleed, full-screen images and more); see [Reeden extensions](../../references/reeden-extensions.md) (Chinese). Use them only in a Reeden version, and only after you have tested them. Keep them out of the generic version.

## Readest

The generic version just has to read well. Readest recognises books by their content, so after re-importing an edited book it may still open the old one. If you see an old result, first make sure you opened the new file, then look at the CSS.

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
