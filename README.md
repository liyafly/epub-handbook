# epub-handbook

中文 EPUB 3 制作与 AI 协作工具集。围绕「硬约束 + 自造 demo + 阅读器实测 + 自动化 skill」四件套构建：所有规则都有 demo fixture 兜底，所有阅读器兼容性结论都从实测回写，所有 AI 行为都按写定的 skill 契约执行。

适合：

- 制作中文 EPUB 3 的工程师与编辑
- 想用 AI 帮忙清洗已有 epub 的人
- 想给团队约定 epub 制作规范的 maintainer

## 仓库做什么

1. **工程契约层** — [docs/final/](docs/final/)：SPEC、终极手册、HTML / CSS 属性速查表、阅读器兼容性实测矩阵 `reader-matrix.yaml`。这是对外硬约束。
2. **清洗流水线** — [docs/pipeline/](docs/pipeline/)：已有 EPUB 的清洗工作流，含红线 gate `scripts/validate_text_invariance.py`、harness 扫描器、典型脏 EPUB 模式识别。
3. **AI 协作 skills** — [skills/](skills/)：14 个专项 skill（CSS 分层、字体、Ruby、Kindle 兼容、弹注、英文小说排版等）。可被 Claude Code / Codex 直接调用，也可由人工照 `SKILL.md` 步骤执行。
4. **可运行 demo** — [templates/](templates/) 与 [samples/demo-books/](samples/demo-books/)：每条规则都必须有 demo 复现，不允许只靠手册推断改规则。
5. **入门教程** — [docs/getting-started/](docs/getting-started/)：第一次接触本仓的人按这里走。

## 我要做什么？

| 场景 | 入口 |
| --- | --- |
| 从零做一本新书 | [docs/getting-started/01-first-epub.md](docs/getting-started/01-first-epub.md) → `templates/epub-style-demo/` |
| 改造一本现成 EPUB | [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md) |
| 看制作硬规则 | [docs/final/SPEC-实现约束.md](docs/final/SPEC-实现约束.md) |
| 查 HTML / CSS 属性 | [docs/final/EPUB 3 HTML CSS 属性速查表.md](docs/final/EPUB%203%20HTML%20CSS%20属性速查表.md) |
| 看阅读器兼容性记录 | [docs/final/reader-matrix.yaml](docs/final/reader-matrix.yaml) |
| 对比改前 / 改后 | [#epub-diff-review](#epub-diff-review) |
| 给 AI 接入 | [skills/](skills/) + [`agents/openai.yaml`](agents/openai.yaml) |
| 看场景化指南 | [docs/guides/](docs/guides/) |
| 维护与贡献 | [CONTRIBUTING.md](CONTRIBUTING.md) + [CLAUDE.md](CLAUDE.md) |

## 准备环境

| 必需 | 用途 |
| --- | --- |
| bash / zip / unzip | 打包 / 解压 EPUB |
| python3 ≥ 3.9 | 红线脚本、harness、validator |
| git | 仓库 + `git diff --no-index` 当 diff 引擎 |

推荐：

- **Calibre 5+** — 主路径 diff review（macOS / Windows / Linux 均有官方安装包）
- **VS Code** — 精细 diff review
- **epubcheck**（W3C 官方）— EPUB 合法性兜底；`brew install epubcheck` 或下载 zip
- **Kindle Previewer 3** — Kindle 转换风险预检
- **Apple Books** — macOS / iOS 实测
- **lxml** — Python 部分解析任务有性能提升；`pip install lxml`

## 5 分钟跑通

```sh
bash templates/epub-style-demo/build.sh
bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<新建的 .epub>
```

详细教程见 [docs/getting-started/01-first-epub.md](docs/getting-started/01-first-epub.md)。

## EPUB diff review

要对比改前 / 改后两个 EPUB（清洗前后、模板改动前后等），本仓推荐两条本地路径，二选一或组合使用。**两条都本地运行，文件不离开设备**。

红线层（正文文本 / 核心 metadata / spine / 章节锚点 / 封面）由 `scripts/validate_text_invariance.py` 兜底，与本节工具无关；红线先跑，diff review 是人工补看其余四层。

### 主路径：Calibre Editor（推荐）

Calibre 自带的「Compare to another book」提供字符级 HTML / CSS diff、图片像素 overlay 和文件树着色。Calibre 5.x 及以上版本均支持。

1. 把 `before.epub` 拖入 Calibre 主程序书库（或直接 File → Open with → Edit book…）。
2. 选中该书 → 右键 → **Tweak Book**（快捷键 `T`）。
3. Tweak Book 窗口 → 顶部菜单 **File → Compare to another book…**。
4. 选 `after.epub` → 自动打开两栏比较视图。
5. 左侧文件树着色：绿 added / 红 deleted / 黄 modified；点击任一文件进入字符级 diff。
6. 图片差异：双击图片节点弹出像素 + 尺寸 + 体积 overlay。
7. 字体 / 音频等二进制：Calibre 只显示「内容不同」，要核对 SHA-256 走精细路径。

完成后把结论抄到工作目录的 `notes.md`，按 [docs/pipeline/cleanup-flow.md §14](docs/pipeline/cleanup-flow.md) 的标准模板组织。

### 精细路径：VS Code + `unzip`

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

### 五层 review 清单

不论用 Calibre 还是 VS Code，都必须覆盖五层。文本红线由自动化 gate 兜底，其余四层人工看。

| 层 | 看什么 | 主路径（Calibre） | 精细路径（VS Code） | 自动化兜底 |
| --- | --- | --- | --- | --- |
| 结构 | OPF manifest / spine / nav.xhtml / toc.ncx 文件级 add/del/mod | 左侧文件树颜色 | `git diff --no-index --stat` | `validate_text_invariance.py --check spine` |
| 文本 | XHTML 正文是否真的不变（red line） | 字符级 diff | `git diff --no-index --color-words *.xhtml` | `validate_text_invariance.py --check text`（必须 0） |
| 样式 | CSS selector 增删、属性变更 | 字符级 diff | `--color-words *.css` 或 `code --diff` | — |
| 资源 | 图片 / 字体 / 音频 SHA-256 与体积 | 像素 + 尺寸 overlay | `shasum -a 256` 列表 diff | `validate_text_invariance.py --check cover`（封面红线） |
| 元数据 | dc:* / `<meta>` 字段 | OPF 字符级 diff | 同上对 `*.opf` | `validate_text_invariance.py --check metadata`（必须 0） |

### 故障排查

| 现象 | 解决 |
| --- | --- |
| Calibre Compare 菜单灰掉 | Tweak Book 必须处于编辑状态；先 `Cmd+S` 存一次再 Compare |
| `git diff --no-index` 报「not a git repository」 | `--no-index` 模式不需要仓库；确认命令完整 |
| `code --diff` 不弹窗 | VS Code 命令行未注册：在 VS Code 里 `Cmd+Shift+P` → `Shell Command: Install 'code' command in PATH` |
| Calibre 看到 modified 但 diff 全空 | EPUB 内文件用了不同 EOL（CRLF vs LF）；用 `git diff --no-index --ignore-cr-at-eol` 复核 |
| `shasum` 在 Linux 报命令缺失 | 改用 `sha256sum`；列序兼容 |

### 不做什么

- 不渲染 EPUB（不是阅读器）；阅读器渲染效果走 reader-matrix 实测。
- 不替代红线 gate；红线永远靠 `validate_text_invariance.py`。
- 不向外网传文件；本节所有命令本地执行。

## 已有 EPUB 清洗

主流程见 [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md)。要点：

- **红线先跑**：`scripts/validate_text_invariance.py before.epub after.epub --check all` 退出码必须为 0。
- **harness 扫描**：`python3 scripts/epub_ai_harness.py --mode cleanup input.epub` 给出 findings 与推荐 skill 顺序。
- **分派 skill**：按 findings 依次跑专项 skill；每步保留中间 epub 作回滚锚点。
- **人工 review**：用上节的 [EPUB diff review](#epub-diff-review)。
- **reader-matrix 回写**：实测有变化时按 [CONTRIBUTING.md](CONTRIBUTING.md) 把结果写回 `docs/final/reader-matrix.yaml`。

## AI Skills

[skills/](skills/) 下每个目录是一个可读契约：判断 / 修复 / 验证三段式 `SKILL.md`。

主入口：

- `epub-layout-auditor` — 总审稿、风险分级、分派专项修复。
- `epub-source-intake` — 从 txt / md / PDF / OCR 等源材料起步。

专项 14 个见 [docs/getting-started/04-skills.md](docs/getting-started/04-skills.md) 反向查表。

无 AI 也可用：`SKILL.md` 本身就是 Markdown 步骤说明，人工跟着走即可。

## 实测回写闭环

任何阅读器 / 打包兼容性判断都不允许只靠手册推断，必须：

```text
1. demo 复现（templates/epub-style-demo/ 加最小场景）
   ↓
2. 构建（templates/epub-style-demo/build.sh）
   ↓
3. 阅读器实测（Kindle Previewer / Apple Books / 多看 / KOReader ...）
   ↓
4. 回写（docs/final/reader-matrix.yaml: pass | warn | fail | na）
   ↓
5. 固化规则（docs/final/SPEC-实现约束.md）
   ↓
6. 同步（终极手册、速查表、相关 skills）
```

详见 [CLAUDE.md](CLAUDE.md) 的「实测回写闭环」段。

## 这个仓库不是什么

- 不是初级排版课。
- 不是封闭格式（mobi / AZW3）的制作工具。
- 不是 epub.js 阅读器。
- 不是 Kindle 自费出版的运营指南。
- 不是阅读器渲染验证工具 — 本仓 diff 只比文件，不模拟渲染；渲染效果靠 reader-matrix 实测。

## 文档地图

| 层 | 路径 | 角色 |
| --- | --- | --- |
| 入门 | [docs/getting-started/](docs/getting-started/) | 第一次接触本仓的人 |
| 工程契约 | [docs/final/](docs/final/) | SPEC、终极手册、速查表、reader-matrix；对外硬约束 |
| 场景指南 | [docs/guides/](docs/guides/) | 英文小说 / 文白对照 / 章首图 / 弹注 fallback 等 |
| 批处理流水线 | [docs/pipeline/](docs/pipeline/) | 拿到一本现成 EPUB 后的流程 |
| 计划 / 审稿 | [docs/plans/](docs/plans/) | 多阶段重构、阶段 review、仓库维护说明 |
| 下游架构 | [docs/architecture/](docs/architecture/) | 周边项目架构副本 |
| 推导 / 实验 | [docs/source/](docs/source/), [docs/experiments/](docs/experiments/) | 早期推导、实测复盘 |

完整索引见 [docs/README.md](docs/README.md)。

## 协作 / 贡献

阅读 [CLAUDE.md](CLAUDE.md) 了解 AI 协作约定。本仓所有约束变更都走 demo → reader-matrix → SPEC → 手册 → 速查表 → skills 的实测闭环。

贡献流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可

代码部分 MIT；文档与样本许可参见 [THIRD_PARTY.md](THIRD_PARTY.md)。
