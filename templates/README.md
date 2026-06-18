# Templates

这个目录是仓库唯一的可运行 EPUB 样本 / 模板入口，用来生成样式 demo、清洗 demo、starter 书壳和后续手工 fixture。

## 当前模板

| 目录 | 用途 | 打包命令 |
|---|---|---|
| `epub-style-demo` | 验证中文正文、列表、表格、代码、Ruby、弹注、局部竖排、A-lite 海报页 | `sh templates/epub-style-demo/build.sh` |
| `cleanup-demo-books` | 自造 before / after 清洗样本，用于红线 gate 和 diff review | `bash templates/cleanup-demo-books/build.sh` |
| `book-starter` | 从零做一本最小 EPUB 的 starter 模板 | `bash templates/book-starter/build.sh` |
| `style-presets` | 可复用 CSS 风格预设 | 不单独打包 |
| `fixtures-tiny` | 手工扩展用极简 fixture 槽位 | 按子目录自建 |

## 使用原则

- 模板用于显示验证、清洗演示和 starter 复现，不承载下游技术架构。
- 模板不依赖外部包，默认只使用 shell 和系统 `zip`。
- 生成的 `.epub` 产物默认放在模板自己的 `dist/` 目录。
- 新增样式规则时，优先补一个可打开的模板页面，再把稳定结论写回手册或 skill。
