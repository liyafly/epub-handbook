# EPUB 风格预设

六套预设是**新书起点**，不是给已有书注入的整书主题。新书想从某个书型开始时，建书时复制一份进书级 `design.css`，之后当作本书自己的 CSS 自由修改，不再与预设同步。组合效果优先查[配方目录](../recipes/README.md)。预设只提供 `Styles/` CSS、`preset.json` 的层顺序和说明，不改正文文本或结构。

预设即本目录下的六个目录：`plain-cn` / `literary-cn` / `academic-cn` / `classical-annotated-cn` / `poetry-cn` / `fiction-en`。

## 新书起点用法

```sh
sh templates/book-starter/new-book.sh work-epub/<book> --from-preset literary-cn
```

按 `preset.json` 的 `layers` 顺序，把除 `fonts.css`、`notes.css` 外的各层追加到新书 `design.css`，文件头注明“起点：preset <name> @ <手册提交>”。`notes.css` 单独复制为新书的 `Styles/notes.css`，并自动加入 OPF 与各章 link（base → notes → design）。`fonts.css` 不复制：字体走 `fonts.json` 与共享字体库流程。不带 `--from-preset` 时行为不变。复制后在书级 `制作说明.md` 记下选用的预设，视觉不满意就直接改 `design.css`。

预设选择器面向本仓 class 体系（`.font-st`、`.chapter-head`、`.parallel-float-pair` 等），新书正文需用这些类名才会生效。

## 存量兼容

已用 `epub.typography.optimize preset=<name>` 注入过预设的书可继续用该能力维护；新工作不再使用它，也不给没有预设层的已有书注入预设。能力本身本轮保留。存量书维护时的注意点：

```sh
epub run epub.typography.optimize --input input.epub --output output.epub --json --dry-run preset=literary-cn
```

先看 dry-run 的 `coverage`（低于 30% 表示原书尚未迁入该体系，直接应用很可能静默无效）。整书应用保留已有 `Styles/fonts.css` 原始字节和 OPF 字体元数据，保持原书自由／锁定模式；正文链与元数据冲突、散落在其他样式层或存在需人工判断的导入／动态／内联正文链时会拒绝处理。自定义预设根目录用 `preset_dir=<路径>`，限 1–32 个不重复层，JSON 至多 1 MiB、单层 CSS 至多 4 MiB、合计至多 16 MiB，输入必须是普通文件。可用 `epub run epub.style.demo.maintain catalog=true collection=presets query=academic` 搜索预设目录（只列出，不验证、不应用）。

预设遵守 `docs/final/SPEC-实现约束.md` §7 的 CSS 分层和加载顺序。新增预设时：

1. 建立 `<name>/preset.json`、`README.md` 和 `Styles/`。
2. `preset.json` 的 `layers` 按 `fonts.css`、`base.css`、其余场景层排序。
3. 单 CSS 文件不超过 400 行；不得在页面级设置颜色或背景。
4. 用 `epub run epub.typography.optimize` 的 `--dry-run` 验证新预设，再跑 `epub redline` 红线 gate 和 `epub run epub.notes.popup.normalize` 弹注校验。

> **实测状态**：这些预设的结构合规由自动化 validator 保证；视觉效果尚未经
> `reader-matrix` 实测，应用后应标记为待实测，直到按阅读器实测闭环回写。
