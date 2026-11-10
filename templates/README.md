# Templates

这个目录是仓库唯一的可运行 EPUB 样本 / 模板入口，用来生成样式 demo、清洗 demo、starter 书壳和后续手工 fixture。

## 当前模板

| 目录 | 用途 | 打包命令 |
|---|---|---|
| `epub-style-demo` | 验证中文正文、列表、表格、代码、Ruby、弹注、局部竖排、A-lite 海报页 | `sh templates/epub-style-demo/build.sh` |
| `cleanup-demo-books` | 自造 before / after 清洗样本，用于红线 gate 和 diff review | `bash templates/cleanup-demo-books/build.sh` |
| `book-starter` | `epub book new` 内置的起始书；旧布局书的 `build.sh` 转发入口 | `epub book new work-epub/<book>` |
| `style-presets` | 六个书型起点样例 | 不单独打包 |
| `recipes` | 组合效果配方的 CSS、最小 XHTML 与证据边界 | `python3 templates/recipes/build.py` |
| `shape-wrap-demo` | 独立轮廓环绕合成场景及历史阅读器证据 | `sh templates/shape-wrap-demo/build.sh` |
| `fxl-demo` | 整本固定版式：封面、跨页对开、图上压字、纯文字页（SPEC §5.13） | `sh templates/fxl-demo/build.sh` |
| `font-role-probe` | 字体角色探针：哪些字体真的固定、哪些会被换字覆盖（SPEC §8）；字体不入库，本地提供 | `uv run --no-project --with fonttools python templates/font-role-probe/build.py --fonts <fonts.json>` |
| `font-scale-probe` | 字号缩放探针：哪些书内 CSS 会让“调字号”失效（SPEC §5.5.1） | `sh templates/font-scale-probe/build.sh` |

## 使用原则

- 模板用于显示验证、清洗演示和 starter 复现，不承载下游技术架构。
- 模板构建入口用 shell 加系统 `zip`，或只依赖 Python 3 标准库的脚本（`recipes`、`cleanup-demo-books`、`shape-wrap-demo`）；`font-role-probe` 另需 fontTools。这些脚本只生成自造样本，不属于用户 EPUB 执行面。
- 生成的 `.epub` 产物放在模板自己的 `dist/` 目录，被 Git 忽略；构建日志等临时文件同样不入库，可随时删除重建。
- `shape-wrap-demo/evidence/` 与 `cleanup-receipt.json` 是有意只留在本地的历史阅读器证据，不要清理。
- 新增样式规则时，优先补一个可打开的模板页面，再把稳定结论写回手册或 skill。
