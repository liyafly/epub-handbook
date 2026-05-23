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
4. `03-vertical-alite.xhtml`：A-lite 整页海报样本（title + subtitle）。
5. `03b-poster-fullbleed.xhtml`：海报全幅 cover 对照样本。
6. `04-lists-tables-code.xhtml`：列表、表格、代码块和键盘文本。
7. `05-legacy-note-fallback.xhtml`：在标准弹注结构上叠加多看旧版 fallback。
8. `06-multi-legacy-note-fallback.xhtml`：同一 XHTML 文件内多条 fallback 弹注。
9. `07-font-family-order.xhtml`：系统优先 / 书内优先 / 混合链的 font-family 顺序验证。
10. `08-long-mixed-flow.xhtml`：长段落、中英混排、大字号标题与右侧裁切压力测试。
11. `09-kindle-risk.xhtml`：Kindle Previewer 专项风险项。
12. `10-text-effects.xhtml`：着重号、波浪线、首字下沉与 Ruby。
13. `11-chapter-opening.xhtml`：章首结构。
14. `12-literary-fiction.xhtml`：小说体综合排版。
15. `13-duokan-rich-fallback.xhtml`：多看富文本 fallback 一体页。
16. `14-vertical-body.xhtml`：整页正文竖排。
17. `15-frontmatter.xhtml`：版权、题献与题记。
18. `16-math.xhtml`：数学公式与 MathML。
19. `17-image-layout.xhtml`：figure 图文环绕与 Kindle 大字号回归测试。
20. `18-english-fiction.xhtml`：英文小说正文、首段规则、居中插图与大字号回归测试。
21. `19-border-shadow-notes.xhtml`：边框、阴影、斜角感、SVG 花边实验、长条投影和不规则便签文本框。

OPF 还声明 `Images/cover.png` 为 raster 封面图，用于覆盖 Kindle Previewer 的封面识别检查。
完整覆盖关系见 `SCENE_MATRIX.md`。

## 验证建议

- Apple Books：重点看嵌入样式、Ruby、弹注和 A-lite 分页。
- Kindle / KFX：重点看封面识别、弹注触发、竖排、表格和背景图。
- KOReader / Thorium / Calibre：重点看 CSS 兼容差异和窄屏重排。

这个模板不包含第三方字体和版权图片，便于直接纳入版本管理。
