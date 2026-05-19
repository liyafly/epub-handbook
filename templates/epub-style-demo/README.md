# EPUB Style Demo

这是一个最小 EPUB 3 样式样本，用来快速检查项目推荐样式在不同阅读器上的显示。

## 生成

```sh
sh templates/epub-style-demo/build.sh
```

脚本会生成类似：

```text
templates/epub-style-demo/dist/epub-style-demo-YYYYMMDD-HHMMSS.epub
```

## 样本页

1. `00-title.xhtml`：封面式标题页。
2. `01-body.xhtml`：中文正文、引用、着重、图片。
3. `02-ruby-note.xhtml`：Ruby 注音和标准弹出注释结构。
4. `03-vertical-alite.xhtml`：局部竖排和 A-lite 海报页。
5. `04-lists-tables-code.xhtml`：列表、表格、代码块和键盘文本。
6. `05-legacy-note-fallback.xhtml`：在标准弹注结构上叠加多看与掌阅读者端 fallback。
7. `06-multi-legacy-note-fallback.xhtml`：同一 XHTML 文件内多条 fallback 弹注。
8. `07-font-family-order.xhtml`：正文系统字体优先的 font-family 顺序验证。

## 验证建议

- Apple Books：重点看嵌入样式、Ruby、弹注和 A-lite 分页。
- Kindle / KFX：重点看弹注触发、竖排、表格和背景图。
- KOReader / Thorium / Calibre：重点看 CSS 兼容差异和窄屏重排。

这个模板不包含第三方字体和版权图片，便于直接纳入版本管理。
