# Glossary

[中文](glossary.md) · [Guide home](README.en.md)

## EPUB itself

- **EPUB**: an e-book format. Underneath it is a zip file holding HTML (XHTML), CSS, images and a table of contents. Readers lay the text out again for each screen and font size, which is called **reflowable**. Books whose pages have a fixed size are **fixed layout** (pre-paginated). This tool works mainly on reflowable books and also supports books that are fixed layout throughout; mixing both in one book is not supported.
- **EPUB 2 / EPUB 3**: the two versions. The OPF says which: `<package version="2.0">` or `"3.0"`. This tool targets EPUB 3.
- **XHTML**: the files that hold the text; a stricter form of HTML.
- <a id="opf"></a>**OPF**: the book's package file (often `package.opf`). It lists the title and author, every file in the book, and the reading order.
- <a id="manifest"></a>**manifest**: the part of the OPF that lists every file. Readers ignore files that are not listed.
- <a id="spine"></a>**spine**: the part of the OPF that sets the reading order.
- <a id="navxhtml"></a>**nav.xhtml**: the EPUB 3 table of contents.
- <a id="ncx"></a>**NCX**: the EPUB 2 table of contents. Keep both, for Kindle and older readers.
- **`epub:type`**: an attribute that gives an element a meaning, such as `epub:type="footnote"`. Declare `xmlns:epub` on `<html>` before using it.
- **Pop-up note**: tap a note marker and the note opens in a small window in place, without jumping away.
- **DRM**: encryption a store adds to a book. This tool stops when it finds DRM and never decrypts.

## Fonts

- **Embedded font**: a font file packed inside the book. Readers use it when they allow publisher fonts.
- **Font master**: the complete original font file. It lives in the shared `fonts/` library, and `book.json` locks its version by SHA.
- **Subset**: a font cut down to the characters the book uses, which makes it much smaller. The build makes it from the master.

## This tool

- **Book folder**: the working folder for one book, holding `book.json`, `original.epub`, `epub/` and `out/`. See [Quick start](quick-start.en.md#2-the-book-folder).
- **Fallback / enhancement**: first write what every reader can show (the fallback), then add effects that only work where supported (the enhancement). If the enhancement fails, the book still reads well.
- **Recipe**: copyable CSS plus a sample for one special effect. See [Recipes](recipes.en.md).
- **Preset**: a whole set of styles for one kind of book, used once when creating a new book.
- **Reader version / overlay**: a separate build for one reader. It may replace only CSS and the OPF, so its text is the same as the generic version. See [Readers](readers.en.md#one-source-several-reader-versions).
- **Red line**: what packing must never change: text, metadata, reading order, anchors, cover and encryption state. Checked by `epub redline`.
- **Navigation audit**: checks that the package, table of contents and note links work, and that font size is not locked.
- **Receipt**: the SHA of a finished file, recorded in `.epub/` after a successful build. If the file was edited by hand afterwards, the build uses it to refuse to overwrite.

## Readers

- **Send to Kindle**: Amazon's delivery service. It converts the EPUB to Kindle's format in the cloud.
- **Kindle Previewer**: Amazon's desktop preview tool. It only approximates; the device has the final word.
- **KFX / AZW3**: Kindle's own formats. This tool does not produce them directly.
