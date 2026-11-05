# Recipes

[中文](recipes.md) · [Guide home](README.en.md)

A recipe is a piece of CSS you can copy plus a minimal sample, for one special effect. Every recipe writes the fallback first and the enhancement second, so the book still reads well when a reader ignores the enhancement.

Recipes live in [`templates/recipes/`](../../templates/recipes/README.md) (Chinese). Each folder has:

- `recipe.md`: the effect, the markup, how it falls back, and test results;
- `recipe.css`: the styles to copy;
- `sample.xhtml`: a minimal sample showing the structure and class names to use.

To use one:

```sh
epub book recipes                         # list recipes and their fallbacks
epub book recipes drop-cap                # print the recipe CSS
epub book recipes drop-cap --add my-book  # append it to the book's design.css
```

Then add the class names to your XHTML as in `sample.xhtml`, and preview. If the book has no `design.css`, copy the printed CSS to the end of its own stylesheet.

## How each reader handles them

| Recipe | Apple Books | Kindle | Reeden | Generic fallback |
| --- | --- | --- | --- | --- |
| [Shape wrap](../../templates/recipes/shape-wrap/recipe.md) | Text follows the outline on the first spread (later pages not checked) | Falls back to a rectangular wrap | Untested | Outline → rectangle → block image on narrow screens |
| [Drop cap](../../templates/recipes/drop-cap/recipe.md) | Reading area blank; cause not found | Plain and sunk initials both fine | Untested | Initial → plain paragraph |
| [Emphasis marks](../../templates/recipes/text-emphasis/recipe.md) | Reading area grey; cannot judge | Underline and dots both shown; readable | Untested | Native emphasis dots → underline |
| [Theme-aware art](../../templates/recipes/theme-adaptive-art/recipe.md) | Image with its own white backing visible in dark theme | White-backed image visible; night mode untested | Untested | Image carries its own white backing |
| [Parallel text](../../templates/recipes/parallel-pair/recipe.md) | Reading area grey; cannot judge | Original and translation stacked | Untested | Two columns on wide screens → stacked |

What this evidence covers:

- results come from synthetic samples, not real books;
- Apple Books means version 9.0 on macOS; Kindle means Kindle Previewer 4.0.3, not a book delivered by Send to Kindle;
- iPhone, Reeden and Send to Kindle are not tested yet. Every "Untested" cell is on the to-test list.

Details, screenshots and Readest results for each cell are in the matching `recipe.md` and the [reader matrix](../final/reader-matrix.yaml).

## Book-type presets

[Presets](../../templates/style-presets/README.md) (Chinese) are another starting point: a whole set of styles for one kind of book. You use one once, when creating a new book (`epub book new my-book --from-preset literary-cn`). After that the copied styles belong to the book, and you change them freely.
