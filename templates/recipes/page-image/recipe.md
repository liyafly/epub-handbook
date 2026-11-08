# 整页图降级

## 效果

书名页、章首页这类装饰页，在普通阅读器里用真实文字排版；只在 Kindle 版换成一张预先截好的整页图。只用在 Kindle 上确实显示不好的那几页（固定页骨架、竖排浮动、叠层背景、文字压图），正文不转图。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 不显示整图，走文字版（待测） | 显示整页图；合成样本转换通过，目测待测 | 不显示整图，走文字版（待测） | Kindle 整页图 → 其他阅读器用文字版 |

## 标记结构

同一页里放两份：一张 `img.page-image`，一个包住真实排版的 `.page-image-text`。见 [sample.xhtml](sample.xhtml)。

- `alt` 写完整的页面文字（书名、作者等），图里的字不能检索、不能放大。
- 标题留在文字版里，目录照常指向这一页。
- 图片必须是书里的资源（OPF 有声明），两个版本的包里都会有它；通用版只是不显示。

## 增强层

两半 CSS：

- 通用版：[recipe.css](recipe.css) 只有一条 `.page-image { display: none; }`，用 `epub book recipes page-image --add <书目录>` 追加到 `design.css`。
- Kindle 变体：把 [reader.css](reader.css) 的内容写进 `variants/kindle/OEBPS/Styles/reader.css`。它显示整图、隐藏 `.page-image-text`。书里还没有 `reader.css` 时先运行 `epub book reader-layer <书目录>`。

## 做法

1. 先在书里把这一页用文字排好，普通阅读器里确认效果。
2. `epub book fonts <书目录>` 把锁定的字体注入源树，否则截图会用系统回退字体（`epub-art page` 会报 `missingFonts`）。
3. 截图：`epub-art page <书目录>/epub/OEBPS/Text/<页>.xhtml --output <书目录>/epub/OEBPS/Images/<名>.jpg --engine webkit`。默认 1264×1680；书名页一类竖长页可加 `--size 1200x1800`。输出不透明，Kindle 不会把透明处压成白底。
4. 在 OPF 里声明这张图，在页面里加 `img.page-image`，按上节加两半 CSS。
5. 截图用的脚本、源 HTML 和验证截图放书根 `素材/`。
6. `epub book build <书目录>`，在 Kindle Previewer 里看 Kindle 版这一页，再在通用阅读器里确认文字版没变。

## 降级行为

不认 `display: none` 的阅读器会同时显示图和文字，内容不丢。Kindle 版图片按页宽铺满、高度自动；大字号下图片不跟着放大，这是整页图的代价。夜间模式下图片保持原色。

## 阅读器结果

合成样本 `dist/page-image-kindle.epub` 用 Kindle Previewer 4.0.4 命令行转换：Enhanced Typesetting Supported、Conversion Success、0 error、0 质量问题（只有缺封面声明的 W14016 提示）。这是转换结果，不是目测验收。一本书的书名页用这个做法在 Kindle Previewer 中通过，证据留在书级目录。至少两本书验证前只作候选。见 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-page-image`。
