---
name: epub-typography-optimizer
description: 优化中文/CJK EPUB 排版，包括正文节奏、font-family 链、嵌入字体、生僻字 fallback、标题字体、中西文混排和阅读器安全段落间距。用于文字拥挤、裁切、跨阅读器字体不一致或字体规则需要清理时。
---

# EPUB 字体与正文节奏优化

这个 skill 处理 CJK 阅读舒适度和字体兼容性。不要把它和弹注转换、图片环绕、A-lite 海报混在一起。
英文为主的 EPUB 使用 `epub-english-typography-optimizer`，不要把英文小说套用中文 2em 缩进和 CJK 字体链。

## 固定目标

默认生产排版应当阅读器安全：

- 正文字体使用短的跨平台系统字体链。
- 默认 `font-family` 链最多 4 段：Apple、Windows、Android/开源 CJK、generic。
- 默认不把嵌入字体放进 `body` / `h*`；唯一例外是含生僻字且使用全字符集字体的 C1-body 路径。
- 生僻字子集使用 `.rare` 等专用类。
- 设计字体使用 `.title-special`、`.signature` 等专用类。
- 即使没有嵌入字体，OPF 也保留 `ibooks:specified-fonts` metadata。

## 字体链模式

默认正文使用系统优先链：

```css
body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

除非 fontspec 要求 `forceAll`，嵌入字体只通过显式类使用：

```css
.title-special {
  font-family: "BookTitleFace", serif;
}

.rare {
  font-family: "RareSongSubset", serif;
}
```

嵌入字体按 SPEC §8 的三种模式选择：

- 模式 A：设计字体专用类，链为嵌入字体 + generic。
- 模式 B：生僻字子集 `.rare`，链为嵌入字体 + generic。
- 模式 C：嵌入 + 系统字体复合链，链最多 5 段，嵌入字体只出现一次。

如果确实存在生僻字且 fontspec 使用 `forceAll` 打包全字符集字体，可以走 C1-body 例外。链仍要短，并在文档或构建元数据中说明策略：

```css
body {
  font-family: "BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

子集字库不能走 C1-body；子集只允许通过 `.rare` 等显式类包住需要补字的字符。

## 工作流

1. 读取 `fonts.css`、`base.css`、OPF 字体 manifest 和目标 XHTML。
2. 分类字体用途：
   - 默认阅读字体。
   - 标题字体。
   - 题签/设计字体。
   - 生僻字 fallback。
   - monospace/code 字体。
3. 删除同一链里重复的同平台别名。
4. 把 `@font-face` 和字体工具类移入 `fonts.css`。
5. 把正文节奏留在 `base.css`：
   - 段首缩进与段间距。
   - 行高。
   - 标题 margin。
   - blockquote 和列表节奏。
   - 长英文 token 换行保护。
6. 规则位置也有问题时，配合 `epub-css-layering-optimizer`。
7. OPF 只声明实际使用的字体文件。

## 正文节奏清单

- body line-height 在阅读器放大字号时仍要舒适。
- 段首缩进只作用于普通正文，不作用于标题、表格、代码、注释或图注。
- 长 URL、标识符、英文文件名需要换行保护。
- 标题需要分页控制，但不能在短章制造空白页。
- 代码块和表格在大字号下仍可读。
- Ruby 行高需要给注音留出空间。

## 禁止事项

- 不把版权字体放进模板或示例。
- 不把多个嵌入字体塞进默认链来解决生僻字。
- 不把子集字库挂到 `body` / `h*`。
- 有稳定英文 family/PostScript 名时，不依赖中文字体显示名。
- 不删除 generic fallback。
- 没有明确阅读器 bug 时，不在阅读字体上滥用 `!important`。
- 不把正文做成整页图片。

## 验证 fixture

- `Text/01-body.xhtml`：普通正文和强调。
- `Text/07-font-family-order.xhtml`：font-family 顺序。
- `Text/08-long-mixed-flow.xhtml`：裁切、长 token、中西文混排。
- `Text/10-text-effects.xhtml`：Ruby 和文字效果。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```
