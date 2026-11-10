# epub-handbook

[中文](README.md)

Make EPUBs you already own look better, and display properly on Apple Books, Kindle and Reeden. `epub` is a command-line tool: it unpacks a book into a folder you can edit, you change the CSS, and it packs and checks the result. Not a word of the text may change, the table of contents and notes must work, and the font size must stay adjustable; only when every check passes does it write the finished book. The design of each book is yours. The tool makes sure nothing breaks, that effects fall back gracefully when a reader cannot show them, and that each reader can get its own version. It runs on macOS, Windows and Linux, and works only on DRM-free books.

## Install

**Build from source** (needs [Go](https://go.dev/dl/) 1.27 or later):

```sh
git clone https://github.com/liyafly/epub-handbook.git
cd epub-handbook
go build -o epub ./cmd/epub        # on Windows: go build -o epub.exe ./cmd/epub
```

Put the resulting `epub` in a folder on your PATH, or run it by its full path.

**Download a binary**: [GitHub Releases](https://github.com/liyafly/epub-handbook/releases) has archives for macOS, Windows and Linux, with `SHA256SUMS` to verify them. The `epub book` commands arrived after v0.5.0, so build from source until the next release. The macOS binary is not signed, and macOS may block it the first time you run it.

**Font tool (optional)**: you need `epub-font` only if the book embeds fonts. It is a Python program, installed from the repository with [uv](https://docs.astral.sh/uv/):

```sh
uv tool install --editable --reinstall tools-font/epub-font
```

Books without embedded fonts do not need it. See the [font tool notes](tools-font/README.md) (Chinese).

## Five steps

1. Import: `epub book import original.epub my-book` freezes the original, checks it and unpacks it into `my-book/epub/`.
2. Preview: `epub book preview my-book --open` packs an unchecked preview and opens it (on a Mac, prefer `--open=Reeden`; see [Readers](docs/guide/readers.en.md)).
3. Edit styles: change the CSS under `my-book/epub/OEBPS/`, perhaps copying from the [recipes](docs/guide/recipes.en.md), and preview again.
4. Build: `epub book build my-book` runs every check and writes the finished book to `my-book/out/`.
5. Send it to your reader: import it into Apple Books, or use Send to Kindle. See [Readers](docs/guide/readers.en.md).

## User guide

- [Quick start](docs/guide/quick-start.en.md): the full flow, the book folder, what each check means, and what to do when the build refuses
- [Readers](docs/guide/readers.en.md): Apple Books, Kindle, Reeden, and separate versions per reader
- [Recipes](docs/guide/recipes.en.md): copyable effects and how each reader handles them
- [FAQ](docs/guide/faq.en.md)
- [Glossary](docs/guide/glossary.en.md)

## Fonts while reading

To read in your own preferred font, use Reeden or Readest. To keep the book's fonts, use Apple Books (Original font), Kindle (Publisher Font) or Duokan (default font). See [Readers](docs/guide/readers.en.md) for why.

## Maintainers and AI tools

AI tools (Claude Code, Codex and others) and maintainers start with [AGENTS.md](AGENTS.md). The full document index, CLI exit codes, input limits and the capability list are in the [maintainer index](docs/README.md). Maintainer documents are in Chinese. To contribute, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Scope

Mainly reflowable EPUB 3; whole-book fixed layout is also supported (see [SPEC §5.13](docs/final/SPEC-实现约束.md#513-固定版式整本-pre-paginated), Chinese). No scripted interactivity or read-aloud sync (Media Overlays). The tool does not produce MOBI, AZW3 or other Kindle formats, is not a reader, and does not simulate one: real readers have the final word on how a book looks.

## Licence

Free to use. Code, templates and skills are under the [MIT](LICENSES/MIT.txt) licence; documentation is under [CC BY 4.0](LICENSES/CC-BY-4.0.txt). See [LICENSE](LICENSE). Sources and licences of third-party material are in [THIRD_PARTY.md](THIRD_PARTY.md).
