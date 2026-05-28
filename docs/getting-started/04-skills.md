# AI Skills 怎么用

`skills/` 里的每个目录都是一个可读契约：它说明某类 EPUB 问题该怎么判断、怎么修、修完怎么验证。即使不用 AI，也可以人工按里面的步骤操作。

## 基本流程

1. 先跑 harness 判断输入类型：

   ```sh
   python3 scripts/epub_ai_harness.py --mode cleanup path/to/book.epub
   ```

2. 已有 EPUB 先用 `epub-layout-auditor` 做总审稿。
3. 按 findings 分派到专项 skill。
4. 每次改动后运行 `scripts/validate_text_invariance.py`。
5. 用 `tools/epub-diff/index.html` 做人工 diff review。

## 当前 skill 一览

| Skill | 用途 |
| --- | --- |
| `epub-layout-auditor` | 总入口：审稿、风险分级、分派专项修复 |
| `epub-source-intake` | 从 txt/md/PDF/OCR 等源材料建立 EPUB source |
| `epub-css-layering-optimizer` | CSS 分层与内联样式迁移 |
| `epub-typography-optimizer` | 中文正文节奏、字体链、嵌入字体策略 |
| `epub-english-typography-optimizer` | 英文小说排版 |
| `epub-literary-structure-formatter` | 章首、题记、文白对照、文学结构 |
| `epub-image-layout-optimizer` | 图片、图注、封面、图文环绕 |
| `epub-vertical-ruby-optimizer` | 竖排正文与 Ruby |
| `epub-kindle-compatibility-checker` | Kindle/KDP 转换风险 |
| `epub-package-nav-auditor` | OPF manifest/spine、nav、NCX |
| `epub-alite-converter` | 普通 epub 转 A-lite 增强方案 |
| `epub-popup-footnote-converter` | 标准 popup footnote |
| `epub-legacy-footnote-fallback` | 多看旧版弹注 fallback |
| `epub-style-demo-maintainer` | 本仓 demo fixture 维护 |

## 我想做 X，用哪个 skill？

| 我要做什么 | Skill |
| --- | --- |
| 拿到一本 epub 不知从哪下手，先看大局 | `epub-layout-auditor` |
| 我有 txt / md / PDF，需要先变成 epub source | `epub-source-intake` |
| 把弹注做规范 | `epub-popup-footnote-converter` |
| 多看 / 旧版阅读器看不到弹注，加 fallback | `epub-legacy-footnote-fallback` |
| 给生僻字加 Ruby 注音 / 整本竖排 | `epub-vertical-ruby-optimizer` |
| 中英混排、字号 / 行距、首字下沉 | `epub-typography-optimizer` |
| 英文小说专项排版 | `epub-english-typography-optimizer` |
| 图片混排规范化 | `epub-image-layout-optimizer` |
| CSS 臃肿 / 内联样式过多，做分层 | `epub-css-layering-optimizer` |
| 弹注 / 文学结构 / 出处规范化 | `epub-literary-structure-formatter` |
| Kindle 转换失败 / Enhanced Typesetting 问题 | `epub-kindle-compatibility-checker` |
| 普通 epub 转 A-lite 增强版 | `epub-alite-converter` |
| OPF / nav.xhtml / toc.ncx 检查与修复 | `epub-package-nav-auditor` |
| 维护本仓 demo fixture | `epub-style-demo-maintainer` |

不确定属于哪一类时，先调用 `epub-layout-auditor`。
