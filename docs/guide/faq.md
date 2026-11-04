# 常见问题

[English](faq.en.md) · [指南首页](README.md)

## 安装

**提示找不到 `epub` 命令。**
把 `epub`（Windows 为 `epub.exe`）放进 PATH 里的目录，或者用完整路径运行。在仓库里也可以用 `go run ./cmd/epub`，需要 Go 1.27 或更新版本。

**需要装 Python、Node.js 或 zip 吗？**
不需要。`epub book` 系列命令只用一个 `epub` 程序。只有嵌入字体时才要装 Python 写的 [字体工具](../../README.md#安装)。

**路径里有空格，命令出错。**
给路径加上双引号：`epub book build "我的 书"`。

## 导入

**导入时说书有加密或 DRM。**
本工具不解密，也不处理有 DRM 的书。请换一本无 DRM 的 EPUB。

**导入时说 `Non-OEBPS layout`。**
这本书的文件不在常见的 `OEBPS/` 目录下。加 `--legacy-layout` 重新导入，工具会按原样保留它的目录结构，不改文件名。

**导入时说缺少字体工具。**
书里嵌了字体，导入要用 `epub-font` 盘点它们。装好 [字体工具](../../README.md#安装) 再导入。

## 构建

**构建拒绝覆盖成品（`Refusing to overwrite dist`）。**
你改过 `out/` 里的文件。处理方法见 [快速开始](quick-start.md#构建拒绝覆盖成品时)。

**提示 `Another build is active`。**
同一本书已经有一次构建在跑。如果确定没有，那是上次中断留下的锁：删掉 `.epub/build.lock` 再试。

**导航审计报 `css-root-font-size`。**
样式表在 `html`、`:root` 或 `*` 上写了 `font-size`，这会让部分阅读器调不了字号。删掉这条声明即可，见 [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放)。

**构建时出现大量缺字警告。**
这是字体母版本来就没有这些字，阅读器会用默认字体显示它们。警告不会阻断构建。在意的话换一个字更全的字体。

**红线报正文变了。**
这是严重问题，说明某一步改动了文字。不要交付这个文件，找出是哪一步改的。可以用 `epub redline --check text 旧.epub 新.epub` 定位。

## 显示问题

| 现象 | 先查什么 |
| --- | --- |
| 注释点了不弹出 | `<html>` 上是否声明了 `xmlns:epub`，链接是否指向正确的 `id`。不支持弹窗的阅读器会退化为普通跳转，见 [SPEC §1](../final/SPEC-实现约束.md#1-弹注footnote-popup) 和 [EPUB2 中的弹注写法](../how-to/epub2-popup-note-compatibility.md) |
| 字变成方块或缺字 | 嵌入的字体没有这些字。见 [Kindle 字体渲染](../how-to/kindle-font-rendering-deep-dive.md) |
| 字体没生效 | 字体文件、OPF 里的登记、CSS 的 `@font-face` 三处要对上；阅读器还要允许使用书内字体 |
| 打开后没有目录 | OPF 里的 nav 项要带 `properties="nav"`；NCX 的 `dtb:uid` 要和书的标识一致 |
| 右边被裁掉一截 | `body` 不要同时写 `width:100%` 和左右 `padding`，见 [SPEC §5.5](../final/SPEC-实现约束.md#55-正文页盒模型) |
| 图片太大或太小 | 宽度用百分比写在 `figure` 上，图片用 `width:100%; height:auto`，见 [SPEC §5.6](../final/SPEC-实现约束.md#56-图片环绕兼容) |
| 调大字号后版面挤坏 | 把 `px`、`vh` 改成 `em`、`%` |
| 夜间模式下插图看不清 | 透明底的黑线图在深色背景上会消失，见 [主题插图配方](../../templates/recipes/theme-adaptive-art/recipe.md) |
| Apple Books 显示的还是旧版 | 先在书库里删掉旧书，再重新导入 |
| Readest 显示的还是旧版 | 见 [阅读器 · Readest](readers.md#readest) |
| Kindle 转换失败 | 如果 Apple Books 也打不开，是 EPUB 本身坏了，先让 `epub book build` 全部通过 |

## 其他

**文件名是 `.xhtml`，是不是就是 EPUB 3？**
不一定。EPUB 2 也用 XHTML。打开 OPF 看 `<package version="…">`。版本差异见 [结构与兼容](../how-to/epub-structure-and-compat.md)。

**怎么对比两个 EPUB？**
用 Calibre 编辑器的"与另一本书比较"，或者解包后用 VS Code 对比。见 [EPUB diff review](../pipeline/epub-diff-review.md)。

**我想让 AI 帮忙。**
可以，但不是必需的。AI 工具（Claude Code、Codex 等）先读 [AGENTS.md](../../AGENTS.md)。`skills/` 里的文件是普通 Markdown，人也可以照着做。

**还是没解决？**
提 issue 时附上系统、命令、完整输出和你期望的结果。不要附受版权保护的书。
