---
name: epub-alite-converter
description: 将 EPUB 封面式页面、卷首、章首或海报页转换为项目的 A-lite 可重排全页方案，同时保留原有图文叠加构图、文字、图片和资源。
---

# EPUB A-lite 转换

## 何时用

- 需要把 EPUB 中已有的封面式页面、卷首、章节标题页或海报页转换成项目最终 A-lite 方案时；文集本海报页与相邻版权页的精排也走这里。
- 固定目标：reflowable 页面；`body.fullpage` + `.fullframe`；`min-height: 100%`；`font-size: 16px`；`overflow: hidden`；`page-break-before/after/inside`；背景属于 `body.poster-bg` 或其他 `poster-*` modifier，不属于 `body.fullpage`；既有单图卷封默认 `background-size: contain` 并保留 `.poster-fallback` 原图回退；竖排文字 `writing-mode: vertical-rl`，竖排列 `float: right`。
- 不适用：普通竖排正文页与 inline Ruby 用 `epub-vertical-ruby-optimizer`；弹注、图片环绕、文学结构用对应专项 skill。

## 调什么

```sh
epub run epub.alite.convert --input <书> --output <新书> --json [expect_volumes=N]
```

- 文集本已知卷数时传 `expect_volumes=N`，让能力校验卷数一致。
- 转换涉及弹注、demo 模板或成书后，必须跑校验组合：

```sh
epub run epub.notes.popup.normalize --input <产物> --json   # 涉及弹注时
epub run epub.style.demo.maintain --input <demo 产物> --json # 涉及 demo 模板时
epub redline --check all <before.epub> <after.epub>          # 每次改书后
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误（如缺 `--output`）。
- 本能力特有：`findings` 出现 `warn alite.no-copyright` 表示未找到相邻版权页；`facts` 里需要 `legacy_report=true` 才有每页明细（`poster_pages_refined`、`copyright_pages_refined`、`stylesheets_added`、`poster_pages`、`copyright_pages`、`warnings`，迁移期脚手架），常规运行以 `status` / `findings` / `events` 为准。

## 依据返回怎么判断

- 先读目标页源 XHTML、关联 CSS、OPF manifest/spine 与资源，识别已有构图：背景图、叠加文字块、装饰性叠加图片、竖排列、字体名和嵌入字体用法。保留构图；不重设设计、不重排内容、不改写文字、不替换图片、不新增装饰。
- 页面外壳固定为：

  ```html
  <body class="fullpage poster-bg">
    <section class="fullframe" epub:type="chapter">...</section>
  </body>
  ```

- 竖排叠加文字转浮动竖列，保持三套 writing-mode 前缀与 `text-orientation: mixed`：

  ```css
  .fullframe .vcol {
    writing-mode: vertical-rl;
    -webkit-writing-mode: vertical-rl;
    -epub-writing-mode: vertical-rl;
    text-orientation: mixed;
    -webkit-text-orientation: mixed;
    -epub-text-orientation: mixed;
    float: right;
    text-indent: 0;
    page-break-inside: avoid;
    -webkit-page-break-inside: avoid;
  }
  ```

- 定位转换：保持大体视觉顺序；大块固定 offset 转 `%` margin；标题字号转 `%` 或 `em`；内部基准 `font-size` 保持 `16px`；`.fullframe` 保持 `padding:0`，叠加文字用元素 margin 定位。
- 保留嵌入字体：锁定题名字体时书内字体名优先，通常只接 `serif` / `sans-serif` generic fallback。
- CSS 分层：A-lite CSS 放 `Styles/poster.css`，不写进 `base.css`；OPF manifest 只为实际使用的 assets/CSS/fonts 增减条目；存在 A-lite 时分开声明 `fonts.css` / `base.css` / `poster.css`；保留已有 `nav.xhtml`、`toc.ncx`、`spine toc="ncx"` 与 cover-image metadata。
- 单图卷封：源页是单张包含全部设计内容的卷封图时，用 `poster-bg-contain` 或 `poster-bg-volume-*` modifier；`background-size: contain`，不用会裁图的 `cover` 或会拉伸图片的 `100% 100%`；在 `.fullframe` 内保留 `<img class="poster-fallback">`，并只在 `@supports (background-size: contain)` 中隐藏。
- 禁止事项：不转成 fixed layout；不用 padding-ratio、FXL 或纯整页图片替代项目 A-lite 方案；不因定位困难删除叠加文字；不新增营销式装饰或新的视觉概念；不把私有阅读器 CSS 作为主路径。
- fixture 不变量（对照 `templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml` 与 `Text/03c-poster-contain.xhtml`）：有海报背景时用 `<body class="fullpage poster-bg">`；`body.fullpage` 承载 shell 规则、不写页面级 `color`/`background`/`background-color`；`body.poster-bg` 只写背景图、位置和尺寸；`.fullframe` 包含叠加内容；竖排叠加文字带前缀 fallback；不引入 absolute positioning 或 fixed-layout package metadata；单图卷封保留 `.poster-fallback` 并用 `contain` 防止边缘文字被裁切。
- `status == approval-required` → 停下来问人；`findings` 出现 `error` 或 `redline.*` → 回滚本次改动，修源后重跑；`warn alite.no-copyright` → 人工确认该书是否本就没有相邻版权页。
- 阅读器表现问题按「demo 先行，文档后补」：先在 `templates/epub-style-demo/` 加最小场景，实测后回写 `docs/final/reader-matrix.yaml`，再改 SPEC 与手册。
