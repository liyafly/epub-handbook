# FAQ

[中文](faq.md) · [Guide home](README.en.md)

## Install

**The `epub` command is not found.**
Put `epub` (`epub.exe` on Windows) in a folder on your PATH, or run it by its full path. Inside the repository you can also use `go run ./cmd/epub`, which needs Go 1.27 or later.

**Do I need Python, Node.js or zip?**
No. The `epub book` commands need only the `epub` program. Python is needed only for the [font tool](../../README.en.md#install), and only if the book embeds fonts.

**A path with spaces breaks the command.**
Put the path in double quotes: `epub book build "my book"`.

## Import

**Import says the book is encrypted or has DRM.**
This tool does not decrypt and does not handle DRM-protected books. Use a DRM-free EPUB.

**Import says `not an OEBPS/ layout`.**
The book's files are not in the usual `OEBPS/` folder. The tool has kept the original folder layout as it is, renamed nothing, and builds it normally. Presets and `epub book recipes --add` expect `OEBPS/`, so edit the styles of such a book by hand.

**Import says the font tool is missing.**
The book embeds fonts, and import uses `epub-font` to sort them. Install the [font tool](../../README.en.md#install) and import again.

## Build

**The build refuses to overwrite (`Refusing to overwrite dist`).**
You edited a file in `out/`. See [Quick start](quick-start.en.md#when-the-build-refuses-to-overwrite).

**It says `Another build is active`.**
Another build of the same book is running. A lock left by a build that was killed is usually cleared automatically; if you still see this and are sure no build is running, delete `.epub/build.lock` and try again.

**The navigation audit reports `css-root-font-size`.**
A stylesheet sets `font-size` on `html`, `:root` or `*`, which stops some readers from changing the font size. Delete that declaration. See [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放) (Chinese).

**The build warns at `fixed-role fonts`.**
Text with an assigned font uses characters that font lacks, so readers show them in a fallback font. The output lists each character, the font and the first file; the full list is in `.epub/font-roles.json`. The warning does not stop the build. A fallback sometimes looks fine, so first look at those characters in several real readers. If it looks wrong, use a font with more characters or wrap them in a `.rare` font.

**The build prints many missing-character warnings.**
The font master never had those characters. The count covers the whole book, so an English font lacking Chinese is included, and it does not mean readers will see another font. For characters that are really used, see the `fixed-role fonts` list above.

**The red line says the text changed.**
That is serious: some step changed the words. Do not deliver that file; find the step that did it. `epub redline --check text old.epub new.epub` shows where.

## Display problems

| Symptom | Check first |
| --- | --- |
| Notes do not pop up | Does `<html>` declare `xmlns:epub`, and does the link point at the right `id`? Readers without pop-ups fall back to a normal jump. See [SPEC §1](../final/SPEC-实现约束.md#1-弹注footnote-popup) and [pop-up notes in EPUB 2](../how-to/epub2-popup-note-compatibility.md) (both Chinese) |
| Characters show as boxes | The embedded font lacks them. See [Kindle font rendering](../how-to/kindle-font-rendering-deep-dive.md) (Chinese) |
| A font has no effect | The font file, its OPF entry and the CSS `@font-face` must all match, and the reader must allow publisher fonts |
| No table of contents | The nav item in the OPF needs `properties="nav"`; the NCX `dtb:uid` must match the book's identifier |
| The right edge is cut off | Do not give `body` both `width:100%` and left/right `padding`. See [SPEC §5.5](../final/SPEC-实现约束.md#55-正文页盒模型) |
| Images too big or too small | Put a percentage width on the `figure` and use `width:100%; height:auto` on the image. See [SPEC §5.6](../final/SPEC-实现约束.md#56-图片环绕兼容) |
| Layout breaks at large font sizes | Replace `px` and `vh` with `em` and `%` |
| Images vanish in night mode | Black line art on a transparent background disappears on dark pages. See the [theme-aware art recipe](../../templates/recipes/theme-adaptive-art/recipe.md) |
| Apple Books shows the old version | Delete the old book from the library, then import again |
| Readest shows the old version | See [Readers · Readest](readers.en.md#readest) |
| Kindle conversion fails | If Apple Books cannot open it either, the EPUB itself is broken. Get `epub book build` to pass first |

## Other

**The files end in `.xhtml`. Does that make it EPUB 3?**
Not necessarily; EPUB 2 uses XHTML too. Open the OPF and look at `<package version="…">`. The differences are in [Structure and compatibility](../how-to/epub-structure-and-compat.md) (Chinese).

**How do I compare two EPUBs?**
Use "Compare to another book" in the Calibre editor, or unpack both and compare them in VS Code. See [EPUB diff review](../pipeline/epub-diff-review.md) (Chinese).

**Is fixed layout supported?**
Yes, for books that are fixed layout throughout (every page has a fixed size, `rendition:layout=pre-paginated`). The rules are in [SPEC §5.13](../final/SPEC-实现约束.md#513-固定版式整本-pre-paginated) (Chinese) and a sample is in the [fixed-layout demo](../../templates/fxl-demo/README.md) (Chinese). Mixing reflowable and fixed-layout pages in one book is not supported, and neither is turning a fixed-layout book into a reflowable one. How each reader handles them has not been tested yet.

**Are scripts and read-aloud sync supported?**
No. Scripting (JavaScript) and read-aloud sync (Media Overlays) in EPUB 3 are out of scope.

**Can AI help?**
Yes, but you do not need it. AI tools (Claude Code, Codex and others) start from [AGENTS.md](../../AGENTS.md). The files in `skills/` are plain Markdown that people can follow too.

**Still stuck?**
Open an issue with your system, the command, the full output and what you expected. Do not attach copyrighted books.
