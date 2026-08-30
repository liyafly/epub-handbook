# EPUB diff review

要对比改前 / 改后两个 EPUB（清洗前后、模板改动前后等），本仓推荐两条本地路径，二选一或组合使用。两条路径都在本地运行，文件不离开设备。

普通清洗的红线层（正文文本 / 核心 metadata / spine / 章节锚点 / 封面）由 `epub redline` 兜底，与本节工具无关。红线先跑，diff review 是人工补看其余四层。用户明确授权正文校订时，改走下节的结构化决策路径；不能把预期文字变化伪装成 text gate 通过。

## 授权正文校订：静态审阅页 + JSON

当现版与参考版有大量字符级差异、需要用户逐项选择时，Calibre/VS Code 仍用于最终文件 review，但决策入口优先使用本地静态 HTML：

1. 冻结两侧 EPUB、篇章映射和 SHA-256，明确正文提取范围。
2. 为每个差异生成稳定 id、篇章、类型、两侧精确位置、差异片段和上下文。
3. 页面提供 `adopt_reference`、`keep_current`、`manual`、`pending` 四态，支持本地保存和 JSON 导入/导出。
4. 页面与导出 JSON 至少携带 schema version、差异源报告 SHA-256、现版/参考 artifact 身份、item count，以及每项稳定 id、篇章、两侧片段和决策。导入其他报告的 JSON 时必须提示不匹配，真正应用时必须硬失败。
5. `pending`、未决项或缺少 `manual_text` 时禁止生成候选 EPUB；应用器还必须逐项重新核对 id、篇章、现版/参考片段和总数，不能只信文件名或旧 SHA。
6. 应用后证明最终连续正文逐字等于决策合并结果，并生成“现版 → 候选”与“候选 → 参考版”两份 unified diff；第二份不是失败，而是审阅后明确保留的例外清单。
7. 只允许决策 locator 指向的文字节点变化；同一 XHTML 内的 tag 序列和 `id/class/epub:type/href/src/alt/lang`、强调、ruby / rt、pagebreak 等非文字 DOM / 属性必须另做签名。篇名和导航同步若均已授权，可单列 nav.xhtml / toc.ncx 标签白名单，但标签须等于最终篇名，链接和顺序不得变化。

页面和决策 JSON 可能含受版权正文，只能留在书级 `02 校对材料/正文校订/`。手册仓库级 `records/` 只保存不含正文的可复用判断。

## 主路径：Calibre Editor（推荐）

Calibre 自带的「Compare to another book」提供字符级 HTML / CSS diff、图片像素 overlay 和文件树着色。Calibre 5.x 及以上版本均支持。

1. 把 `before.epub` 拖入 Calibre 主程序书库（或直接 File → Open with → Edit book…）。
2. 选中该书 → 右键 → **Tweak Book**（快捷键 `T`）。
3. Tweak Book 窗口 → 顶部菜单 **File → Compare to another book…**。
4. 选 `after.epub` → 自动打开两栏比较视图。
5. 左侧文件树着色：绿 added / 红 deleted / 黄 modified；点击任一文件进入字符级 diff。
6. 图片差异：双击图片节点弹出像素 + 尺寸 + 体积 overlay。
7. 字体 / 音频等二进制：Calibre 只显示「内容不同」，要核对 SHA-256 走精细路径。

完成后把结论写入书根 `制作说明.md`，按 [cleanup-flow.md §16](cleanup-flow.md) 的标准模板组织。

## 精细路径：VS Code + `unzip`

适合：单文件逐行核对、PR 内贴可粘贴的 diff、批处理多本 EPUB、shell 脚本里嵌套。

```sh
# 1. 解压到书级忽略区
DIFF_WORK='work-epub/book-a/03 制作工作区/.pipeline/diff'
mkdir -p "$DIFF_WORK/before-extracted" "$DIFF_WORK/after-extracted"
unzip -q before.epub -d "$DIFF_WORK/before-extracted"
unzip -q after.epub  -d "$DIFF_WORK/after-extracted"

# 2. 整树概览（不需要 git 仓库）
git diff --no-index --stat "$DIFF_WORK/before-extracted" "$DIFF_WORK/after-extracted"

# 3. 单文件字符级 diff（中英文混排都能看清）
git diff --no-index --color-words \
  "$DIFF_WORK/before-extracted/OEBPS/Text/01-body.xhtml" \
  "$DIFF_WORK/after-extracted/OEBPS/Text/01-body.xhtml"

# 4. VS Code 内对照单文件
code --diff \
  "$DIFF_WORK/before-extracted/OEBPS/Styles/base.css" \
  "$DIFF_WORK/after-extracted/OEBPS/Styles/base.css"

# 5. VS Code 整树侧边栏（需扩展 moshfeu.compare-folders）
code "$DIFF_WORK/before-extracted" "$DIFF_WORK/after-extracted"
# 然后命令面板 → Compare Folders: Compare With ...

# 6. 资源层 SHA-256 列表
( cd "$DIFF_WORK/before-extracted" && find . -type f -exec shasum -a 256 {} + ) | sort > "$DIFF_WORK/before.sha256"
( cd "$DIFF_WORK/after-extracted"  && find . -type f -exec shasum -a 256 {} + ) | sort > "$DIFF_WORK/after.sha256"
diff -u "$DIFF_WORK/before.sha256" "$DIFF_WORK/after.sha256"
```

Linux 上 `shasum -a 256` 等价于 `sha256sum`，输出列序兼容。

## 五层 review 清单

不论用 Calibre 还是 VS Code，都必须覆盖五层。普通清洗的文本红线由自动化 gate 兜底；授权正文校订由已核验的决策 artifact 和最终文字重建结果兜底，其余层继续执行原红线。

| 层 | 看什么 | 主路径（Calibre） | 精细路径（VS Code） | 自动化兜底 |
| --- | --- | --- | --- | --- |
| 结构 | OPF manifest / spine / nav.xhtml / toc.ncx 文件级 add/del/mod | 左侧文件树颜色 | `git diff --no-index --stat` | `epub redline --check spine` |
| 文本 | 普通清洗正文不变；授权校订与决策一致 | 字符级 diff | `git diff --no-index --color-words *.xhtml` | 普通：`--check text` 必须 0；授权：决策 SHA/逐项片段/最终文字验证 |
| 样式 | CSS selector 增删、属性变更 | 字符级 diff | `--color-words *.css` 或 `code --diff` | — |
| 资源 | 图片 / 字体 / 音频 SHA-256 与体积 | 像素 + 尺寸 overlay | `shasum -a 256` 列表 diff | `epub redline --check cover`（封面红线） |
| 元数据 | dc:* / `<meta>` 字段 | OPF 字符级 diff | 同上对 `*.opf` | `epub redline --check metadata`（必须 0） |

## 故障排查

| 现象 | 解决 |
| --- | --- |
| Calibre Compare 菜单灰掉 | Tweak Book 必须处于编辑状态；先 `Cmd+S` 存一次再 Compare |
| `git diff --no-index` 报「not a git repository」 | `--no-index` 模式不需要仓库；确认命令完整 |
| `code --diff` 不弹窗 | VS Code 命令行未注册：在 VS Code 里 `Cmd+Shift+P` → `Shell Command: Install 'code' command in PATH` |
| Calibre 看到 modified 但 diff 全空 | EPUB 内文件用了不同 EOL（CRLF vs LF）；用 `git diff --no-index --ignore-cr-at-eol` 复核 |
| `shasum` 在 Linux 报命令缺失 | 改用 `sha256sum`；列序兼容 |

## 不做什么

- 不渲染 EPUB（不是阅读器）；阅读器渲染效果走 reader-matrix 实测。
- 不替代红线 gate；普通清洗正文仍靠 `epub redline`。授权正文校订只替换 text gate 的证明方式，metadata、spine、锚点、封面和 DRM 红线不变。
- 不向外网传文件；本节所有命令本地执行。
