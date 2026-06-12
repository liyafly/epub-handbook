# 九、用 book-starter 十分钟出一本书

前面章节教你**理解** EPUB；这一页教你**直接出一本**。

1. 复制骨架：`cp -r templates/book-starter ~/my-book && cd ~/my-book`
2. 改元数据：`OEBPS/package.opf` 的 `dc:title` / `dc:creator` / `dc:identifier`
   （新 UUID 同步到 `OEBPS/toc.ncx` 的 `dtb:uid`）。
3. 写正文：编辑 `OEBPS/Text/01-chapter.xhtml`；新增章节页时同步 manifest、spine、
   `nav.xhtml`、`toc.ncx` 四处。
4. 构建：`sh build.sh`，产物在 `dist/`。
5. 体检：`python3 <仓库路径>/scripts/epub_lint.py dist/<产物>.epub`，error 清零后
   再丢进 Apple Books / Kindle Previewer 实看。

骨架默认自由模式（读者可换字体）。弹注、图片环绕、竖排等场景去
`templates/epub-style-demo/` 找对应页面抄结构；规则依据查
`docs/final/EPUB 3 HTML CSS 属性速查表.md`。

> 溯源：templates/book-starter/README.md；SPEC §7 / §8。
