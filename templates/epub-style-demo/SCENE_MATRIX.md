# EPUB Style Demo 场景矩阵

本矩阵是 demo EPUB 的执行清单。新增兼容性判断时，先补这里和对应 XHTML，再 build EPUB、跑阅读器验证，最后回写 `docs/final/reader-matrix.yaml` 与最终文档。

| 场景 | XHTML | 主要检查点 | 目标阅读器 |
|---|---|---|---|
| 封面式标题页 | `Text/00-title.xhtml` | 标题页分页、landmark cover、标题居中 | Apple Books / Kindle Previewer / Thorium |
| 普通正文 | `Text/01-body.xhtml` | 段落缩进、行高、引用、着重、图片 figure | 全部 |
| 标准弹注 | `Text/02-ruby-note.xhtml` | `noteref`、同文件 `aside`、回跳、Ruby 行距 | Apple Books / Thorium / KOReader |
| A-lite 海报 | `Text/03-vertical-alite.xhtml` | `body.fullpage`、`.fullframe padding:0`、背景、竖排标题 | Apple Books / Kindle Previewer |
| 海报全幅对照 | `Text/03b-poster-fullbleed.xhtml` | `body.poster-bg-fullbleed`、`background-size: cover`、整页满铺 | Apple Books / Kindle App / Kindle Previewer |
| 表格与代码 | `Text/04-lists-tables-code.xhtml` | 列表、表格滚动、代码块、kbd | Kindle Previewer / KOReader / Thorium |
| 多看 fallback | `Text/05-legacy-note-fallback.xhtml` | `duokan-footnote`、`ol.duokan-footnote-content`、单注释 | 多看 / 标准阅读器回退 |
| 多条 fallback | `Text/06-multi-legacy-note-fallback.xhtml` | 多 noteref 指向同一 `aside` 内不同 `li` | 多看 / 标准阅读器回退 |
| font-family 顺序 | `Text/07-font-family-order.xhtml` | 系统优先、书内优先、楷体混合链、生僻字 fallback | Apple Books / Windows 阅读器 |
| 长段落与中英混排 | `Text/08-long-mixed-flow.xhtml` | 普通正文盒模型、右侧裁切、长 token 换行、大字号标题 | Kindle Previewer / Apple Books |
| Kindle 风险项 | `Text/09-kindle-risk.xhtml` | cover metadata、nav + NCX、PNG、长串、表格、代码 | Kindle Previewer |
| 文字效果合集 | `Text/10-text-effects.xhtml` | `.emp` / `.wavy` / `.dropcap` / Ruby 行距 | 全部 |
| 章首结构 | `Text/11-chapter-opening.xhtml` | 章首图、标题副标题、卷头引文 | Apple Books / Thorium |
| 小说体综合 | `Text/12-literary-fiction.xhtml` | 对话、诗节、场景分隔、信件块 | 全部 |
| 多看富文本 fallback | `Text/13-duokan-rich-fallback.xhtml` | `ol.duokan-footnote-content` + `li.duokan-footnote-item` | 多看 / 标准阅读器 |
| 整页正文竖排 | `Text/14-vertical-body.xhtml` | `body.page-vrl`、`.vrl-section` | Apple Books / KOReader |
| 前置页 | `Text/15-frontmatter.xhtml` | `epub:type="frontmatter"`、题献与题记 | 全部 |
| 数学公式与 MathML | `Text/16-math.xhtml` | KDP MathML 支持标签：分式、根式、上下标、矩阵、semantics/annotation | Kindle App / Kindle Previewer / Apple Books / Thorium |
| 图文环绕 | `Text/17-image-layout.xhtml` | figure 浮动、25%–35% 百分比宽度、长正文阈值、短段反例、大字号回归 | Kindle App / Readest / Apple Books / Thorium |

## 打包与记录

```sh
sh templates/epub-style-demo/build.sh
```

验证完成后，把阅读器名称、版本、构建产物、失败页面、现象、状态和 workaround 写入 `docs/final/reader-matrix.yaml`。
