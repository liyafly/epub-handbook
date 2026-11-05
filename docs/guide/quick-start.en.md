# Quick start

[中文](quick-start.md) · [Guide home](README.en.md)

Start from an EPUB you already have and get a first improved version in about ten minutes. If `epub` is not installed yet, see [Install](../../README.en.md#install).

## 1. Import

```sh
epub book import original.epub my-book
```

This step:

- copies the book to `my-book/original.epub`, makes it read-only and records its SHA in `book.json`;
- checks the package and its table of contents. It stops on DRM, encryption or a damaged file, and never tries to decrypt;
- unpacks the book into `my-book/epub/`;
- moves any complete embedded fonts into a shared `fonts/` library next to the book folder and locks their versions by SHA in `book.json`. Fonts that already look subsetted stay where they are. This step needs the [font tool](../../README.en.md#install).

When an import stops, `original.epub` and the reports are kept so you can see why. Common cases are in the [FAQ](faq.en.md#import).

To write a new book from scratch, use `epub book new my-book`. Add `--from-preset literary-cn` to start from one of the book-type presets.

## 2. The book folder

```text
my-book/
  book.json       # the only config: title, output name, fonts, reader versions
  original.epub   # the frozen original; read-only, leave it alone
  epub/           # the unpacked book; the only place you edit
  out/            # finished files; appears after a successful build
  .epub/          # hidden: reports, receipts, previews, failed builds; safe to clean
```

You edit `epub/`, and now and then `book.json`. Nothing else.

## 3. Preview

```sh
epub book preview my-book --open
```

This packs `.epub/preview.epub` and opens it with your default app. On a Mac, prefer `--open=Reeden`: the default is usually Apple Books, which adds another copy on every import, so keep it for the final check.

Previews are fast because they **skip every check** and never touch `out/`. Use them to look, not to deliver.

## 4. Edit the styles

The stylesheets live in a folder under `epub/OEBPS/` (imported books keep their original layout; new books use `Styles/`). Change one thing at a time and preview again.

- For ready-made effects, copy from the [recipes](recipes.en.md).
- Use `em` or `%` for font sizes and `%` for image widths. Never set `font-size` on `html` or `:root`: some readers then ignore the reader's font-size setting, and the build will reject it.
- Do not fix text and page colours for the whole page, so night mode keeps working.

Do not change the text itself. The build checks that it still matches the original.

## 5. Build

```sh
epub book build my-book
```

The build runs these checks in order and stops at the first failure. Each check prints one line; `PASS` means it passed:

| Check | What it means |
| --- | --- |
| Font injection | Takes each font from the shared library by the SHA in `book.json`; stops if a file does not match |
| Font subset | Keeps only the characters the book uses and makes sure none were lost. Characters missing from the original font only warn |
| Navigation audit | Package, table of contents and note links work, and font size is not locked |
| Red line | Text, metadata, reading order, anchors, cover and encryption state are unchanged by packing |
| Text match | Every reader version has exactly the same text as the generic version |
| Receipt | Records the SHA of every file in `out/`, so a file you edited by hand is never silently overwritten |

`WARN` lines are reminders that do not stop the build, such as missing characters in a font (only characters the text actually uses count) or a book over 50 MB (Send to Kindle may reject it). A final `INFO` line compares the result with your previous build, for information only. Everything is summarised in `.epub/report.html`; open it in a browser.

If anything fails, nothing in `out/` changes. The failed attempt stays in `.epub/build.*`; `epub book clean my-book` removes old ones and keeps the latest.

## 6. Send it to your reader

The finished book is in `my-book/out/`: `book.epub` by default, plus files such as `book-kindle.epub` if you set up reader versions. See [Readers](readers.en.md) for how to send them.

## When the build refuses to overwrite

If you edited an EPUB in `out/` directly (say, a quick fix in Calibre or Sigil), the next build refuses to overwrite it with `Refusing to overwrite dist`. That is deliberate: your edit would otherwise be lost. You have two options:

- **Drop the edit**: delete that file from `out/` and build again.
- **Keep the edit**: make the same change in `epub/`, then run `epub book accept-dist my-book`. It rebuilds from `epub/`, checks that every file matches the one in `out/` byte for byte, and only then records a new receipt.

## Other commands

| Command | Use |
| --- | --- |
| `epub book fonts my-book` | Put the complete fonts into `epub/` so your editor can show them |
| `epub book fonts my-book --update-lock` | After replacing a font master and checking it, update the SHA lock |
| `epub book build my-book --variant kindle` | Build only one reader version |
| `epub book import … --git` | Create a separate Git repository in the book folder (optional) |

Leave out the book folder and the command uses the current directory. Books in the older folder layout (`03 制作工作区/`) work with the same commands; the tool detects the layout.
