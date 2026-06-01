# EPUB diff review

要对比改前 / 改后两个 EPUB（清洗前后、模板改动前后等），本仓推荐两条本地路径，二选一或组合使用。两条路径都在本地运行，文件不离开设备。

红线层（正文文本 / 核心 metadata / spine / 章节锚点 / 封面）由 `scripts/validate_text_invariance.py` 兜底，与本节工具无关。红线先跑，diff review 是人工补看其余四层。

## 主路径：Calibre Editor（推荐）

Calibre 自带的「Compare to another book」提供字符级 HTML / CSS diff、图片像素 overlay 和文件树着色。Calibre 5.x 及以上版本均支持。

1. 把 `before.epub` 拖入 Calibre 主程序书库（或直接 File → Open with → Edit book…）。
2. 选中该书 → 右键 → **Tweak Book**（快捷键 `T`）。
3. Tweak Book 窗口 → 顶部菜单 **File → Compare to another book…**。
4. 选 `after.epub` → 自动打开两栏比较视图。
5. 左侧文件树着色：绿 added / 红 deleted / 黄 modified；点击任一文件进入字符级 diff。
6. 图片差异：双击图片节点弹出像素 + 尺寸 + 体积 overlay。
7. 字体 / 音频等二进制：Calibre 只显示「内容不同」，要核对 SHA-256 走精细路径。

完成后把结论抄到工作目录的 `notes.md`，按 [cleanup-flow.md §16](cleanup-flow.md) 的标准模板组织。

## 精细路径：VS Code + `unzip`

适合：单文件逐行核对、PR 内贴可粘贴的 diff、批处理多本 EPUB、shell 脚本里嵌套。

```sh
# 1. 解压
mkdir -p work/before-extracted work/after-extracted
unzip -q before.epub -d work/before-extracted
unzip -q after.epub  -d work/after-extracted

# 2. 整树概览（不需要 git 仓库）
git diff --no-index --stat work/before-extracted work/after-extracted

# 3. 单文件字符级 diff（中英文混排都能看清）
git diff --no-index --color-words \
  work/before-extracted/OEBPS/Text/01-body.xhtml \
  work/after-extracted/OEBPS/Text/01-body.xhtml

# 4. VS Code 内对照单文件
code --diff \
  work/before-extracted/OEBPS/Styles/base.css \
  work/after-extracted/OEBPS/Styles/base.css

# 5. VS Code 整树侧边栏（需扩展 moshfeu.compare-folders）
code work/before-extracted work/after-extracted
# 然后命令面板 → Compare Folders: Compare With ...

# 6. 资源层 SHA-256 列表
( cd work/before-extracted && find . -type f -exec shasum -a 256 {} + ) | sort > work/before.sha256
( cd work/after-extracted  && find . -type f -exec shasum -a 256 {} + ) | sort > work/after.sha256
diff -u work/before.sha256 work/after.sha256
```

Linux 上 `shasum -a 256` 等价于 `sha256sum`，输出列序兼容。

## 五层 review 清单

不论用 Calibre 还是 VS Code，都必须覆盖五层。文本红线由自动化 gate 兜底，其余四层人工看。

| 层 | 看什么 | 主路径（Calibre） | 精细路径（VS Code） | 自动化兜底 |
| --- | --- | --- | --- | --- |
| 结构 | OPF manifest / spine / nav.xhtml / toc.ncx 文件级 add/del/mod | 左侧文件树颜色 | `git diff --no-index --stat` | `validate_text_invariance.py --check spine` |
| 文本 | XHTML 正文是否真的不变（red line） | 字符级 diff | `git diff --no-index --color-words *.xhtml` | `validate_text_invariance.py --check text`（必须 0） |
| 样式 | CSS selector 增删、属性变更 | 字符级 diff | `--color-words *.css` 或 `code --diff` | — |
| 资源 | 图片 / 字体 / 音频 SHA-256 与体积 | 像素 + 尺寸 overlay | `shasum -a 256` 列表 diff | `validate_text_invariance.py --check cover`（封面红线） |
| 元数据 | dc:* / `<meta>` 字段 | OPF 字符级 diff | 同上对 `*.opf` | `validate_text_invariance.py --check metadata`（必须 0） |

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
- 不替代红线 gate；红线永远靠 `validate_text_invariance.py`。
- 不向外网传文件；本节所有命令本地执行。
