# 移除 `tools/` + 丰富根 README — Review + 执行计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 `superpowers:executing-plans` 或 `superpowers:subagent-driven-development` 逐 Task 执行。每个 step 都用 `- [ ]` checkbox 标记，按顺序勾完再进下一个 Task。
>
> **作者**：Claude（Opus 4.7）
>
> **日期**：2026-05-28
>
> **执行人**：下一棒模型（人或 AI），假设对本仓零上下文。

**Goal:** 删除整个 `tools/` 目录（连带 `tools/epub-diff/`），把人工 diff review 全面切到 Calibre Editor / VS Code 这类成熟外部工具，并把工作流直接写进根 `README.md` 的 `## EPUB diff review` 段；同时把根 README 整体丰富为多段结构化入口文档，让它取代「分散在多个子文档里再回链」的旧组织。

**Architecture:**
- 把 `tools/` 整目录连根删（包括 `tools/epub-diff/` 与未来工具的占位）。
- 不创建独立的 `docs/guides/epub-diff-*.md` 指南；diff 工作流唯一权威落地在根 README 的锚点 `#epub-diff-review`。
- 全仓所有指向 `tools/epub-diff/...` 的引用一律改为指向根 README 的该锚点。
- 同步 SPEC §10 / CLAUDE.md / skill / 入门 / 流水线 / 样本 README 的硬引用。
- 历史 plan 档（`handbook-expansion-plan.md`、`handbook-expansion-review.md`）正文保留为快照，banner 标注「Stage 3 / 相关 P0 已作废」。

**Tech Stack:** Bash、Git、Markdown。无 JS / Node。验证靠 `grep`、`test`、`wc -l`。

---

## 1. Review：`tools/epub-diff/` 当前实现与移除理由

### 1.1 实测产物清单

```text
tools/epub-diff/
├── README.md                  (44 行)
├── index.html                 (79 行)  — Landing / Loading / Diff 三视图 + About modal
├── app.js                     (164 行) — file picker / AbortController / 五层调度 / JSON 导出
├── parsers/
│   ├── epub.js                (164 行) — zip.js streaming + container.xml + OPF 解析
│   └── xml.js                 (37 行)
├── layers/
│   ├── structure.js           (35 行)
│   ├── text.js                (48 行)  — XHTML 块级 SHA-256
│   ├── style.js               (50 行)  — CSS selector + line diff
│   ├── resources.js           (58 行)  — 流式 SHA-256 + image thumbnail
│   └── metadata.js            (31 行)
├── render/
│   ├── diff-view.js           (149 行)
│   ├── line-diff.js           (27 行)
│   ├── loading.js             (10 行)
│   └── theme.js               (15 行)
├── util/
│   ├── hash.js                (149 行) — 自己实现的 IncrementalSha256
│   └── format.js              (19 行)
├── assets/
│   ├── style.css              (213 行)
│   └── vendor/
│       ├── .gitignore         (3 行)
│       └── zip.js.LICENSE     (28 行)
└── scripts/
    └── fetch-vendor.sh        (21 行)  — npm pack @zip.js/zip.js@2.8.26
```

合计约 **1344 行手写代码**，外加运行时依赖 `@zip.js/zip.js` 2.8.26（BSD-3-Clause，单独抓取）。`tools/` 目录除了 `epub-diff/` 外无其他子目录。

### 1.2 当前实际能力

- **结构层**：manifest / spine / nav / ncx 文件级 added/deleted/modified。
- **文本层**：XHTML 块级 SHA-256，标定首个差异块。
- **样式层**：以 selector 切 CSS；自家 line-diff 行级高亮。
- **资源层**：流式 SHA-256（手写 IncrementalSha256，因 `crypto.subtle.digest` 不增量）。modified 图片缩略图并排。
- **元数据层**：dc:* + `<meta>`。
- **导出**：`epub-diff.json` 平铺结果。
- **隐私**：纯浏览器内。

### 1.3 当前缺陷与维护成本

| 维度 | 现状 | 影响 |
| --- | --- | --- |
| vendor 流程 | 双击 `index.html` 一片空白，必须先 `bash scripts/fetch-vendor.sh` | 入门摩擦：违背仓库「双击即用」原则 |
| `file://` 模块限制 | 多数浏览器禁用 ES module，要起本地 server | 又一道阻力 |
| CSS 块级 diff 粗 | 正则切 `selector { body }`，遇 `@media` / `@font-face` / 嵌套退化 | 真实清洗场景不够看 |
| 文本块定位粗 | 块级 SHA-256，看不到块内 diff，无法精确高亮字符 | 比 Calibre 弱 |
| 大文件性能 | FAQ 写「v1 目标支持约 1.5GB」；单 entry 超限即卡 | 没有兜底 |
| 缩略图对比浅 | 仅并排 `<img>`，无像素 / 体积 / 尺寸 overlay | 比 Calibre / VS Code Image Preview 弱 |
| vendor 升级流程未写 | `handbook-expansion-review.md` §6 已点名 P0 缺失 | 长期负债 |
| 历史承诺破产 | 计划档承诺过 `@pierre/diffs` Web Component；实际用了自家 `line-diff.js`。README / SPEC / plans 之间口径漂移 | 文档不一致 |
| 手写 SHA-256 | `util/hash.js` 149 行 hand-rolled crypto | 维护风险 |
| 总代码量 | 1344 行人工 review UI，不参与任何自动化 gate | 自维护性价比低 |

### 1.4 红线 gate 不依赖本工具

自动化红线全部走 `scripts/validate_text_invariance.py`（见 SPEC §10.5 表），与本 web app 完全独立。本 app **唯一职责是人工可视化 review**。替换它不影响任何自动化通道。

### 1.5 外部成熟替代方案为何更合适

| 能力 | `tools/epub-diff/` v0.1 | Calibre Editor「Compare to another book」 | VS Code（解压后） |
| --- | --- | --- | --- |
| 安装门槛 | 仓库自带；要 `fetch-vendor.sh` + 可能起本地 server | macOS / Windows / Linux 一键安装，多数 epub 制作者已装 | 多数开发者已装 |
| 文件列表 add/del/mod | ✅ 五层平铺 | ✅ 文件树着色 | ✅ 整文件树左右对比 |
| HTML / XHTML 语义 diff | 块级 SHA-256，不到字符级 | ✅ 字符级 + 保留 DOM | ✅ 字符级 |
| CSS diff | selector 级 + 自家 line-diff | ✅ 字符级 + 高亮变更属性 | ✅ 字符级 |
| 图片对比 | 并排缩略图 | ✅ 像素 + 尺寸 + 体积 overlay | ✅ Image Preview 扩展 |
| 大文件性能 | 1.5GB 上限，单 entry 易卡 | 数百 MB 稳定 | 取决于解压目录 |
| 离线 / 隐私 | ✅ | ✅ | ✅ |
| 维护方 | 本仓自维护 | Kovid Goyal / Calibre 社区 | Microsoft / 社区 |

**结论**：在「只比文件、不模拟阅读器」这个边界下，Calibre Editor 的内置功能是上位替代；VS Code 在精细行级对照、批处理脚本协作上更顺手。继续自维护 1344 行 web app 不划算。

### 1.6 移除带来的代价（已确认可接受）

1. 失去本仓自带「双击即用」入口 — 改为「装一次 Calibre / VS Code」。
2. 失去五层平铺 JSON 导出 — 改用 `scripts/validate_text_invariance.py --check all` 退出码 + `shasum -a 256` 列表替代红线证据。
3. `samples/demo-books/dist/` 演示对照不再有「打开 index.html 演示」入口 — 改为「Calibre Compare」说明。

### 1.7 决策摘要

1. **删除 `tools/` 整目录**（连根，不留 placeholder）。
2. **不创建独立 guide**；diff 工作流唯一权威落地在根 `README.md` 的 `## EPUB diff review` 段。
3. **根 README 整体丰富**：从当前 50 行扩到约 200 行的多段结构化入口文档（环境准备 / 仓库做什么 / 五层 review 清单 / 实测回写闭环 / 文档地图等）。
4. **SPEC §10.2 / §10.4 / §10.5** 把「web app」字样改为「外部 diff 工具」并指向 README 锚点。
5. **历史 plan 档** banner 标注「相关 Stage 已作废」，正文保留快照。

---

## 2. 新根 README 完整内容（Task 5 直接落盘）

> 这一节是 Task 5 的**契约源**。下棒**逐字**写入 `README.md`，**不要**改字面（链接、表格、代码块照搬）。

````markdown
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
````

**Anchor 约定**：GitHub 渲染 Markdown 时，`## EPUB diff review` 会生成 fragment `#epub-diff-review`（小写、空格转连字符）。**全仓所有跨文档回链统一用 `#epub-diff-review`**。

---

## 3. 文件变更清单（执行总览）

| # | 类型 | 路径 | 动作 |
| --- | --- | --- | --- |
| 1 | 删除 | `tools/`（整目录） | `git rm -r tools` |
| 2 | 删除 | `docs/pipeline/diff-tool.md` | `git rm` |
| 3 | 重写 | `README.md` | 按 §2 整段替换 |
| 4 | 修改 | `docs/README.md` | 删 tools 链接；删「diff 工具」字眼；指向 README 锚点 |
| 5 | 修改 | `docs/pipeline/cleanup-flow.md` | §6 改 Calibre / VS Code；改头部「对应工具」；改 §11 黄线指标 |
| 6 | 修改 | `docs/pipeline/decisions.md` | Q5 划线；新增 Q7 / Q8 / Q9 决策段 |
| 7 | 修改 | `docs/getting-started/01-first-epub.md` | §4 改 diff 步骤 |
| 8 | 修改 | `docs/getting-started/04-skills.md` | 基本流程第 5 步 |
| 9 | 修改 | `docs/getting-started/05-case-study.md` | 第 5 项、第 6 节链接 |
| 10 | 修改 | `docs/getting-started/06-test-your-own.md` | §3 |
| 11 | 修改 | `docs/getting-started/07-faq.md` | Node.js 问题；删 EPUB Diff 工具段 |
| 12 | 修改 | `docs/getting-started/README.md` | 导言、读完入门去哪、推荐阅读 |
| 13 | 修改 | `docs/final/SPEC-实现约束.md` | §10.2 / §10.4 / §10.5 |
| 14 | 修改 | `samples/demo-books/README.md` | §0 介绍 + Diff 工具演示段 |
| 15 | 修改 | `samples/fixtures-tiny/README.md` | line 5 |
| 16 | 修改 | `skills/epub-layout-auditor/SKILL.md` | 第 14 行 |
| 17 | 修改 | `THIRD_PARTY.md` | 删 zip.js 段 |
| 18 | 修改 | `CLAUDE.md` | 优先级第 9 条、关键约束最后一条 |
| 19 | 修改 | `docs/plans/handbook-expansion-plan.md` | banner 补移除注脚 |
| 20 | 修改 | `docs/plans/handbook-expansion-review.md` | 顶部加 2026-05-28 更新 banner |
| 21 | 修改 | `docs/plans/README.md` | 注册本计划 |
| 22 | 既有 | `docs/plans/2026-05-28-remove-epub-diff.md` | 本文件（已存在） |

**强约束**：Task 21 grep 必须无残留命中（仅本计划本身、历史档正文中带 banner 标识的引用允许保留）。

---

## Task 1：分支与基线确认

**Files:** 仅 git 操作。

- [ ] **Step 1：确认在干净工作目录**

```sh
cd /Users/yafeili/Developer/epub-handbook
git status --porcelain
```
Expected: 空输出。

- [ ] **Step 2：从 main 拉新分支**

```sh
git checkout main
git pull --ff-only
git checkout -b chore/remove-tools-and-enrich-readme
```
Expected: 切换到新分支。

- [ ] **Step 3：确认当前快照**

```sh
test -d tools/epub-diff && echo "epub-diff exists" || echo "MISSING"
test -d tools && echo "tools exists" || echo "MISSING"
test -f docs/pipeline/diff-tool.md && echo "diff-tool.md exists" || echo "MISSING"
ls tools/
```
Expected: 三行 `... exists`；`ls tools/` 输出仅 `epub-diff`（确认 tools 下只有这一个子目录）。

- [ ] **Step 4：若 `ls tools/` 输出多于 `epub-diff`，停下**

如果发现 tools/ 下还有其他工具子目录，停止执行并报告：本计划只覆盖 `tools/epub-diff/`，其他工具需另行评估。

---

## Task 2：删除 `tools/` 整目录

**Files:**
- Delete: `tools/`（连根）

- [ ] **Step 1：`git rm -r tools`**

```sh
git rm -r tools
```
Expected: 约 21 个 tracked 文件被删除（含 `tools/epub-diff/README.md`、`index.html`、`app.js`、`parsers/*`、`layers/*`、`render/*`、`util/*`、`assets/style.css`、`assets/vendor/.gitignore`、`assets/vendor/zip.js.LICENSE`、`scripts/fetch-vendor.sh`）。

- [ ] **Step 2：确认目录消失**

```sh
test -d tools && echo "STILL EXISTS" || echo "removed"
```
Expected: `removed`。

- [ ] **Step 3：commit**

```sh
git commit -m "chore(tools): remove tools/ directory (epub-diff retired)

The in-house browser diff app (~1344 lines + zip.js vendor) is being
retired in favor of Calibre Editor / VS Code. The tools/ root is
removed entirely; future tools will recreate the directory if needed.
Documentation and SPEC references are updated in subsequent commits."
```

---

## Task 3：删除 `docs/pipeline/diff-tool.md`

**Files:**
- Delete: `docs/pipeline/diff-tool.md`

- [ ] **Step 1：`git rm`**

```sh
git rm docs/pipeline/diff-tool.md
```
Expected: 1 个文件 staged for deletion。

- [ ] **Step 2：commit**

```sh
git commit -m "docs(pipeline): drop diff-tool.md (tool removed)"
```

---

## Task 4：重写根 `README.md`

**Files:**
- Modify: `README.md`（整体替换为 §2 内容）

- [ ] **Step 1：备份当前长度**

```sh
wc -l README.md
```
Expected: 50 行附近。记录此数字。

- [ ] **Step 2：整文替换**

把 `README.md` 整个文件内容替换为本计划 §2「新根 README 完整内容」段中**两个 `````markdown` ... ````` ` 围栏之间的完整 Markdown**。

**注意**：§2 围栏里的 `## EPUB diff review` 段内含一处嵌套代码块（`` ```sh `` ）；替换时围栏配对要保持原样。如果使用 Edit 工具一次替换有困难，用 Write 工具直接覆盖。

- [ ] **Step 3：长度校验**

```sh
wc -l README.md
```
Expected: 大致 180–230 行。显著少于 150 说明粘贴漏了 § 段。

- [ ] **Step 4：grep 自检**

```sh
grep -c "epub-diff-review" README.md
grep -c "tools/epub-diff" README.md
grep -c "Calibre" README.md
grep -c "git diff --no-index" README.md
```
Expected:
- `epub-diff-review` 出现 ≥ 2 次（场景表 1 次 + 清洗段 1 次）。
- `tools/epub-diff` 出现 0 次。
- `Calibre` 出现 ≥ 5 次。
- `git diff --no-index` 出现 ≥ 3 次。

- [ ] **Step 5：渲染自检（可选）**

如果有 `glow` 或 `bat`：

```sh
which glow >/dev/null && glow README.md | head -50 || echo "skip render preview"
```

肉眼看 5 个主章节标题（仓库做什么 / 我要做什么 / 准备环境 / 5 分钟跑通 / EPUB diff review）都正确出现。

- [ ] **Step 6：commit**

```sh
git add README.md
git commit -m "docs: rewrite README with diff workflow inline and richer entry points

- Add ## EPUB diff review with Calibre main path + VS Code precise path
  + five-layer review checklist + troubleshooting (~80 lines inline)
- Add ## 仓库做什么 mapping the four pillars (final/pipeline/skills/demo)
- Add ## 准备环境 listing required and recommended tooling
- Add ## 已有 EPUB 清洗 / ## AI Skills / ## 实测回写闭环 summary sections
- Add ## 文档地图 mirroring docs/README.md categories
- All cross-doc references now point to README anchor #epub-diff-review"
```

---

## Task 5：改 `docs/README.md`

**Files:**
- Modify: `docs/README.md`

- [ ] **Step 1：删 tools 链接行**

把：
```markdown
## 样本与工具

- 自造清洗 / diff demo：[../samples/demo-books/](../samples/demo-books/)
- 第三方样本占位：[../samples/third-party/](../samples/third-party/)
- EPUB Diff Web App：[../tools/epub-diff/](../tools/epub-diff/)
```
改成：
```markdown
## 样本与工具

- 自造清洗 / diff demo：[../samples/demo-books/](../samples/demo-books/)
- 第三方样本占位：[../samples/third-party/](../samples/third-party/)
- EPUB diff review 工作流：[../README.md#epub-diff-review](../README.md#epub-diff-review)（Calibre / VS Code）
```

- [ ] **Step 2：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" docs/README.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 3：commit**

```sh
git add docs/README.md
git commit -m "docs: index drops tools/ link, points diff to README anchor"
```

---

## Task 6：改 `docs/pipeline/cleanup-flow.md`

**Files:**
- Modify: `docs/pipeline/cleanup-flow.md`

- [ ] **Step 1：替换头部「对应工具」**

把第 5 行：
```markdown
> 对应工具：`scripts/epub_ai_harness.py`、`scripts/validate_text_invariance.py`、`tools/epub-diff/index.html`。
```
改成：
```markdown
> 对应工具：`scripts/epub_ai_harness.py`、`scripts/validate_text_invariance.py`、外部 diff 工具（Calibre / VS Code，见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)）。
```

- [ ] **Step 2：重写 §6 整段**

把 lines 79-88 段（`## 6. Diff 人工 review`）：
```markdown
## 6. Diff 人工 review

打开 `tools/epub-diff/index.html`：

- 第一个文件拾取框选 `work/before/source.epub`。
- 第二个选 `work/after/cleaned.epub`。
- 点 Compare。
- 按结构 / 文本 / 样式 / 资源 / 元数据五层逐层 review。

这一步只看文件差异，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。
```
改成：
```markdown
## 6. Diff 人工 review

按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 的两条路径做：

- 主路径（推荐）：Calibre Editor → Tweak Book → File → Compare to another book → 选 `work/after/cleaned.epub`。
- 精细路径：`unzip` 解压两侧到 `work/before-extracted` / `work/after-extracted`，再用 `git diff --no-index` 整树概览 / `code --diff` 逐文件 / `shasum -a 256` 列表对资源层。
- 五层覆盖：结构 / 文本 / 样式 / 资源 / 元数据。文本红线已在 §5 卡过，本步只确认人眼看到的改动与红线放行的清洗范围一致。

这一步只看文件差异，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。
```

- [ ] **Step 3：改 §11 黄线指标行**

把第 138 行：
```markdown
| 黄线条数 | epub-diff 报告统计 | 记录 |
```
改成：
```markdown
| 黄线条数 | Calibre Compare 文件树 modified 计数（或 `git diff --no-index --stat` 行数） | 记录 |
```

- [ ] **Step 4：验证**

```sh
grep -n "epub-diff\|epub_diff\|tools/" docs/pipeline/cleanup-flow.md | grep -v "^#" || echo "clean"
```
Expected: `clean`（或仅 banner / 删除线条目命中）。

- [ ] **Step 5：commit**

```sh
git add docs/pipeline/cleanup-flow.md
git commit -m "docs(pipeline): cleanup-flow step 6 uses Calibre/VS Code via README anchor"
```

---

## Task 7：改 `docs/pipeline/decisions.md`

**Files:**
- Modify: `docs/pipeline/decisions.md`

- [ ] **Step 1：把 Q5 标为已撤销**

把：
```markdown
| Q5 | Web app 入口位置 | `tools/epub-diff/` | 与 fixture、docs 分离，作为用户工具 |
```
改成：
```markdown
| Q5 | Web app 入口位置 | ~~`tools/epub-diff/`~~ — 已于 2026-05-28 移除（见下） | 自维护性价比低，外部 Calibre / VS Code 是上位替代 |
```

- [ ] **Step 2：追加 2026-05-28 决策段**

在文件末尾追加：

```markdown
## 2026-05-28 移除 tools/ 与丰富根 README

| # | 问题 | 决策 | 理由 |
| --- | --- | --- | --- |
| Q7 | 是否继续自维护 epub-diff web app | 移除整个 `tools/` 目录 | 1) 1344 行手写代码（含手写 SHA-256）维护成本高；2) Calibre Compare to another book 提供字符级 HTML/CSS diff、图片像素 overlay、文件树着色，是上位替代；3) `file://` 模块限制和 vendor 抓取流程违背仓库「双击即用」原则；4) 红线 gate 与本工具完全解耦，移除不影响自动化 |
| Q8 | 替代工作流落地点 | 不创建独立 guide；diff 工作流唯一权威写进根 `README.md` 的 `## EPUB diff review` 段 | 减少跳转层；让根 README 成为单一入口文档 |
| Q9 | 根 README 是否同步丰富 | 是；扩为多段结构化入口（仓库做什么 / 准备环境 / 5 分钟 / diff review / 清洗 / skills / 实测回写 / 文档地图） | 旧 README 仅 50 行场景表，承载不了「单一入口」定位 |
| Q10 | SPEC §10.2 / §10.4 / §10.5 提到的 web app 怎么改 | 字样全部替换为「外部 diff 工具」，并指向 README 锚点 `#epub-diff-review` | `docs/final/` 是硬约束层，必须与实际工具状态对齐 |

执行记录见 [../plans/2026-05-28-remove-epub-diff.md](../plans/2026-05-28-remove-epub-diff.md)。
```

- [ ] **Step 3：commit**

```sh
git add docs/pipeline/decisions.md
git commit -m "docs(pipeline): record 2026-05-28 tools removal + README enrichment decisions"
```

---

## Task 8：改入门层 — `01-first-epub.md`、`04-skills.md`、`05-case-study.md`

**Files:**
- Modify: `docs/getting-started/01-first-epub.md`
- Modify: `docs/getting-started/04-skills.md`
- Modify: `docs/getting-started/05-case-study.md`

- [ ] **Step 1：改 `01-first-epub.md` §4**

把：
```markdown
### 4. 改一段文字，重新构建，再用 diff 工具看看改了什么

[3 行 sh 代码块]

把 `tools/epub-diff/index.html` 用浏览器打开，选这两个 epub 做对比。
```

把最后一行：
```markdown
把 `tools/epub-diff/index.html` 用浏览器打开，选这两个 epub 做对比。
```
改成：
```markdown
按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 或 VS Code 对比这两个 epub。
```

并把 §4 标题里的「diff 工具」改通用：
```markdown
### 4. 改一段文字，重新构建，再用外部 diff 工具看看改了什么
```

- [ ] **Step 2：改 `04-skills.md` 基本流程第 5 步**

把：
```markdown
5. 用 `tools/epub-diff/index.html` 做人工 diff review。
```
改成：
```markdown
5. 按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 或 VS Code 做人工 diff review。
```

- [ ] **Step 3：改 `05-case-study.md` 第 5 项**

把：
```markdown
5. 用 `tools/epub-diff/index.html` 对比结构、文本、样式、资源、元数据五层。
```
改成：
```markdown
5. 按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 或 VS Code 对比结构、文本、样式、资源、元数据五层。
```

把：
```markdown
- Diff 工具负责让人看见结构、文本、样式、资源、元数据五层变化；它不替代 reader-matrix 实测。
```
改成：
```markdown
- 外部 diff 工具（Calibre / VS Code）负责让人看见结构、文本、样式、资源、元数据五层变化；它不替代 reader-matrix 实测。
```

把：
```markdown
- 用 [tools/epub-diff/index.html](../../tools/epub-diff/index.html) 看自己的清洗结果。
```
改成：
```markdown
- 按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 看自己的清洗结果。
```

- [ ] **Step 4：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" \
  docs/getting-started/01-first-epub.md \
  docs/getting-started/04-skills.md \
  docs/getting-started/05-case-study.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 5：commit**

```sh
git add docs/getting-started/01-first-epub.md docs/getting-started/04-skills.md docs/getting-started/05-case-study.md
git commit -m "docs(getting-started): swap epub-diff refs for README anchor"
```

---

## Task 9：改 `docs/getting-started/06-test-your-own.md`

**Files:**
- Modify: `docs/getting-started/06-test-your-own.md`（§3）

- [ ] **Step 1：重写 §3**

把：
```markdown
## 3. 用 epub-diff 工具确认基线

把 `tools/epub-diff/index.html` 拖入浏览器，两边都选 `work/source.epub`：

- 期望：五层都 identical。
- 如果出现差异：说明 diff 工具或文件读取有问题，重新拉取并重试。
```
改成：
```markdown
## 3. 用外部 diff 工具确认基线（可选）

如果你想确认 diff 工作流就绪，把 `work/source.epub` 拷贝一份做自比：

```sh
cp work/source.epub work/source-copy.epub
```

按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 比较 `work/source.epub` 与 `work/source-copy.epub`。

- 期望：所有文件 unchanged。
- 如果 Calibre 报差异：说明拷贝过程中改动了文件，重新拷贝。
```

- [ ] **Step 2：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" docs/getting-started/06-test-your-own.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 3：commit**

```sh
git add docs/getting-started/06-test-your-own.md
git commit -m "docs(getting-started): step 3 uses external diff via README anchor"
```

---

## Task 10：改 `docs/getting-started/07-faq.md`

**Files:**
- Modify: `docs/getting-started/07-faq.md`

- [ ] **Step 1：改 Node.js 问题**

把：
```markdown
**Q：需要 Node.js 吗？**
A：运行时不需要。只有 `tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor assets 时需要一次 `npm pack`。
```
改成：
```markdown
**Q：需要 Node.js 吗？**
A：本仓运行时不需要。Calibre / VS Code 也不依赖 Node.js。
```

- [ ] **Step 2：替换整段 `## EPUB Diff 工具`**

把：
```markdown
## EPUB Diff 工具

**Q：双击 `tools/epub-diff/index.html` 一片空白？**
A：先跑 `bash tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor。

**Q：`file://` 下模块加载失败？**
A：在 `tools/epub-diff/` 内跑 `python3 -m http.server 8000`，再打开 `http://localhost:8000/`。

**Q：拖了大 epub 进去，浏览器卡死？**
A：v1 目标支持约 1.5GB；超过或单 entry 过大时浏览器仍可能受限。
```
改成：
```markdown
## EPUB Diff

**Q：怎么对比两个 EPUB？**
A：见根 [README.md#epub-diff-review](../../README.md#epub-diff-review)。主路径用 Calibre Editor 的「Compare to another book」；精细路径用 `unzip` + `git diff --no-index` 或 VS Code。

**Q：Calibre 的 Compare 菜单是灰的？**
A：Tweak Book 必须先进入编辑状态；随便存一次（Cmd+S）再 Compare。

**Q：可以批处理多本 EPUB 的 diff 吗？**
A：可以；走精细路径，用 `unzip` + `shasum` + `diff` 的 shell 组合。在 PR 里贴 `git diff --no-index --stat` 输出即可。
```

- [ ] **Step 3：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" docs/getting-started/07-faq.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 4：commit**

```sh
git add docs/getting-started/07-faq.md
git commit -m "docs(getting-started): FAQ uses external diff tools via README anchor"
```

---

## Task 11：改 `docs/getting-started/README.md`

**Files:**
- Modify: `docs/getting-started/README.md`

- [ ] **Step 1：改导言**

把：
```markdown
本目录给第一次接触本仓的人。按顺序读完这些页面，你会知道如何构建 demo、如何检查自己的 EPUB、何时使用 AI skills，以及清洗后怎么用 diff 工具 review。
```
改成：
```markdown
本目录给第一次接触本仓的人。按顺序读完这些页面，你会知道如何构建 demo、如何检查自己的 EPUB、何时使用 AI skills，以及清洗后怎么用外部 diff 工具 review。
```

- [ ] **Step 2：改「读完后」列表里的 Diff 工具行**

把：
```markdown
- Diff 工具（[tools/epub-diff/index.html](../../tools/epub-diff/index.html)）对比改前 / 改后。
```
改成：
```markdown
- 外部 Diff 工具（按 [根 README #epub-diff-review](../../README.md#epub-diff-review) 用 Calibre / VS Code）对比改前 / 改后。
```

- [ ] **Step 3：改「读完入门后去哪」段**

把：
```markdown
- **review 改前改后差异**：浏览器打开 [tools/epub-diff/index.html](../../tools/epub-diff/index.html)
```
改成：
```markdown
- **review 改前改后差异**：按 [根 README #epub-diff-review](../../README.md#epub-diff-review) 用 Calibre / VS Code
```

- [ ] **Step 4：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" docs/getting-started/README.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 5：commit**

```sh
git add docs/getting-started/README.md
git commit -m "docs(getting-started): index points to README diff anchor"
```

---

## Task 12：改 `docs/final/SPEC-实现约束.md`

> **优先级提醒**：本文件是 `docs/final/` 硬约束层。CLAUDE.md 要求改这里时同步检查终极手册和速查表 — 本次仅替换工具名，不动红 / 黄 / 绿规则本身，所以手册与速查表不必动；Task 13 Step 5 会扫一遍兜底。

**Files:**
- Modify: `docs/final/SPEC-实现约束.md`（lines 221、245、263）

- [ ] **Step 1：改 §10.2 标题段**

把第 221 行：
```markdown
AI 可自动执行；review 时通过 `tools/epub-diff/index.html` web app 可视化确认：
```
改成：
```markdown
AI 可自动执行；review 时通过外部 diff 工具（Calibre Editor / VS Code，见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)）可视化确认：
```

- [ ] **Step 2：改 §10.4 元规则第 1 条**

把第 245 行：
```markdown
- 改动可见性：任何改动都必须在 web app 中可见；不允许秘密改动。
```
改成：
```markdown
- 改动可见性：任何改动都必须在外部 diff 工具（Calibre / VS Code）中可见；不允许秘密改动。
```

- [ ] **Step 3：改 §10.5 末尾**

把第 263 行：
```markdown
人工可视化 review 通过 `tools/epub-diff/index.html` 完成，不在自动化范畴。
```
改成：
```markdown
人工可视化 review 通过外部 diff 工具（Calibre Editor 主路径，VS Code + `unzip` 精细路径，见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)）完成，不在自动化范畴。
```

- [ ] **Step 4：验证 SPEC 内无残留**

```sh
grep -n "tools/epub-diff\|epub_diff" docs/final/SPEC-实现约束.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 5：检查终极手册与速查表**

```sh
grep -l "tools/epub-diff\|epub_diff" \
  "docs/final/EPUB 3 终极实践手册.md" \
  "docs/final/EPUB 3 HTML CSS 属性速查表.md" 2>/dev/null || echo "none"
```
Expected: `none`。命中则停下来人工评估再加 step。

- [ ] **Step 6：commit**

```sh
git add docs/final/SPEC-实现约束.md
git commit -m "docs(final): SPEC §10 references external diff tools via README anchor"
```

---

## Task 13：改 `samples/demo-books/README.md`

**Files:**
- Modify: `samples/demo-books/README.md`

- [ ] **Step 1：改第 3 行介绍**

把：
```markdown
本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和 `tools/epub-diff/`，不依赖公版书来源。
```
改成：
```markdown
本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和外部 diff 工具（Calibre / VS Code，见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)），不依赖公版书来源。
```

- [ ] **Step 2：改「Diff 工具演示」段**

把：
```markdown
## Diff 工具演示

打开 `tools/epub-diff/index.html`，选择任意 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。
```
改成：
```markdown
## Diff 演示

按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 用 Calibre Editor 或 VS Code 选 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。
```

- [ ] **Step 3：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" samples/demo-books/README.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 4：commit**

```sh
git add samples/demo-books/README.md
git commit -m "docs(samples): demo-books README points to README diff anchor"
```

---

## Task 14：改 `samples/fixtures-tiny/README.md`

**Files:**
- Modify: `samples/fixtures-tiny/README.md`

- [ ] **Step 1：替换引用**

把：
```markdown
为 `scripts/validate_text_invariance.py`、`tools/epub-diff/` 手动验证和后续脚本测试准备的最小 fixture 目录骨架。
```
改成：
```markdown
为 `scripts/validate_text_invariance.py`、外部 diff 工具（见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)）手动验证和后续脚本测试准备的最小 fixture 目录骨架。
```

- [ ] **Step 2：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" samples/fixtures-tiny/README.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 3：commit**

```sh
git add samples/fixtures-tiny/README.md
git commit -m "docs(samples): fixtures-tiny README points to README diff anchor"
```

---

## Task 15：改 `skills/epub-layout-auditor/SKILL.md`

> **优先级提醒**：CLAUDE.md 说「`skills/*/SKILL.md` 的 frontmatter 字段名不要改」。本次只改 body 第 14 行的引用，frontmatter 不动。

**Files:**
- Modify: `skills/epub-layout-auditor/SKILL.md`

- [ ] **Step 1：替换 review 工具指向**

把第 14 行结尾的：
```markdown
人工 review 通过 `tools/epub-diff/index.html` 完成。
```
改成：
```markdown
人工 review 通过外部 diff 工具（Calibre Editor / VS Code，见 [根 README #epub-diff-review](../../README.md#epub-diff-review)）完成。
```

- [ ] **Step 2：扫其他 skill**

```sh
grep -rn "tools/epub-diff\|epub_diff" skills/ || echo "clean"
```
Expected: `clean`。若有命中，对每条做同等替换并加 step。

- [ ] **Step 3：commit**

```sh
git add skills/epub-layout-auditor/SKILL.md
git commit -m "docs(skills): layout-auditor review uses external diff tools"
```

---

## Task 16：改 `THIRD_PARTY.md`

**Files:**
- Modify: `THIRD_PARTY.md`

- [ ] **Step 1：删 zip.js 整段**

删除文件末尾整段（含 `## zip.js` 标题）：
```markdown
## zip.js

- 项目：Gildas Lormeau (`@zip.js/zip.js`)
- 版本：2.8.26（由 `tools/epub-diff/scripts/fetch-vendor.sh` 拉取）
- 许可：BSD-3-Clause，见 `tools/epub-diff/assets/vendor/zip.js.LICENSE`
- 用途：`tools/epub-diff/` 浏览器内 epub 解压。
```

- [ ] **Step 2：验证**

```sh
grep -n "tools/epub-diff\|epub_diff\|zip.js" THIRD_PARTY.md || echo "clean"
```
Expected: `clean`。

- [ ] **Step 3：commit**

```sh
git add THIRD_PARTY.md
git commit -m "docs: drop zip.js third-party entry (tools/ removed)"
```

---

## Task 17：改 `CLAUDE.md`

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1：改「修改优先级」第 9 条**

把：
```markdown
9. `tools/`：面向用户 / maintainer 的本地工具（如 epub-diff web app）。修改要保证「双击即用」的最小依赖原则。
```
改成：
```markdown
9. ~~`tools/`：面向用户 / maintainer 的本地工具（如 epub-diff web app）。修改要保证「双击即用」的最小依赖原则。~~ **已于 2026-05-28 移除整个 `tools/` 目录**；diff review 切到 Calibre / VS Code，工作流写在根 `README.md` 的 `#epub-diff-review` 段。未来要重新引入工具时，新加一条「面向用户 / maintainer 的本地工具」即可。
```

- [ ] **Step 2：改「关键约束」最后一条**

把：
```markdown
- 已有 EPUB 清洗必须遵守 `docs/final/SPEC-实现约束.md` §10：清洗前保留 before，改后运行 `scripts/validate_text_invariance.py`，人工 review 用 `tools/epub-diff/index.html`。
```
改成：
```markdown
- 已有 EPUB 清洗必须遵守 `docs/final/SPEC-实现约束.md` §10：清洗前保留 before，改后运行 `scripts/validate_text_invariance.py`，人工 review 用 Calibre Editor 或 VS Code（见根 `README.md` 的 `#epub-diff-review` 段）。
```

- [ ] **Step 3：验证**

```sh
grep -n "tools/epub-diff\|epub_diff" CLAUDE.md || echo "clean"
```
Expected: `clean`（`tools/` 字符可能出现在划线的历史条目里，是预期的）。

- [ ] **Step 4：commit**

```sh
git add CLAUDE.md
git commit -m "docs(claude): tools section retired; review uses README anchor"
```

---

## Task 18：历史 plan banner 注脚

**Files:**
- Modify: `docs/plans/handbook-expansion-plan.md`（banner 段）
- Modify: `docs/plans/handbook-expansion-review.md`（banner 段）

- [ ] **Step 1：在 `handbook-expansion-plan.md` 落地状态 banner 改 Stage 3 行**

文件开头 blockquote 里：
```markdown
> - Stage 3：`tools/epub-diff/` 已落地为纯静态 app；资源层使用流式 SHA-256，diff 渲染为内置 line diff，支持渐进取消和 modified 图片预览。
```
改成：
```markdown
> - Stage 3：~~`tools/epub-diff/` 已落地为纯静态 app；资源层使用流式 SHA-256，diff 渲染为内置 line diff，支持渐进取消和 modified 图片预览。~~ **已于 2026-05-28 整体移除整个 `tools/` 目录**；diff review 切到 Calibre Editor / VS Code，工作流写在根 `README.md` 的 `#epub-diff-review` 段。执行记录见 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)。本文件正文里仍出现 `tools/epub-diff/` 引用，作为历史快照保留。
```

- [ ] **Step 2：在 `handbook-expansion-review.md` 文件顶部插入 banner**

在 `# ...` 主标题之后（紧跟首个 `## 0` 或同级章节之前）插入：
```markdown
> **2026-05-28 更新**：本 review 中关于 `tools/epub-diff/` 的所有 P0 / P2 条目（流式 SHA-256、@pierre/diffs、AbortSignal、modified 图片缩略图、vendor 升级流程文档）均已**作废**，因为整个 `tools/` 目录于 2026-05-28 整体移除。后续 diff review 走 Calibre / VS Code，见根 `README.md` 的 `#epub-diff-review` 段与 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)。正文保留为历史快照。
>
```

- [ ] **Step 3：commit**

```sh
git add docs/plans/handbook-expansion-plan.md docs/plans/handbook-expansion-review.md
git commit -m "docs(plans): mark epub-diff stage as removed in historical banners"
```

---

## Task 19：注册本计划到 `docs/plans/README.md`

**Files:**
- Modify: `docs/plans/README.md`

- [ ] **Step 1：在「当前计划」列表追加**

在 `- skills-and-templates.md：...` 行之后追加：
```markdown
- `2026-05-28-remove-epub-diff.md`：移除整个 `tools/` 目录、把 diff workflow 写进根 README、丰富 README 的 review + 执行计划
```

- [ ] **Step 2：commit**

```sh
git add docs/plans/README.md
git commit -m "docs(plans): register 2026-05-28 remove-epub-diff plan"
```

---

## Task 20：全仓 grep 收尾验证

**Files:** 无修改；只跑校验。

- [ ] **Step 1：全仓搜 epub-diff 残留**

```sh
grep -RIn "tools/epub-diff\|epub_diff" \
  --exclude-dir=.git \
  --exclude="2026-05-28-remove-epub-diff.md" \
  --exclude="handbook-expansion-plan.md" \
  --exclude="handbook-expansion-review.md" \
  --exclude="decisions.md" \
  --exclude="CLAUDE.md" \
  . || echo "clean"
```
Expected: `clean`。

如果命中：每条人工评估 → 改完再跑这条 → 直到 `clean`。

- [ ] **Step 2：扫 `tools/` 字符串残留**

```sh
grep -RIn "\`tools/\|tools/epub-diff\|tools/$" \
  --exclude-dir=.git \
  --exclude="2026-05-28-remove-epub-diff.md" \
  --exclude="handbook-expansion-plan.md" \
  --exclude="handbook-expansion-review.md" \
  --exclude="decisions.md" \
  --exclude="CLAUDE.md" \
  . || echo "clean"
```
Expected: `clean`。

- [ ] **Step 3：确认 SPEC / 手册 / 速查表口径**

```sh
grep -n "web app" docs/final/*.md || echo "no stale phrase in final/"
```
Expected: 无命中或仅出现在已替换的上下文里。如还有「web app」单独描述 diff 工具，改成「外部 diff 工具」。

- [ ] **Step 4：确认 `tools/` 目录已消失**

```sh
test -d tools && echo "STILL EXISTS" || echo "removed"
```
Expected: `removed`。

- [ ] **Step 5：确认根 README 锚点指向被广泛使用**

```sh
grep -rln "epub-diff-review" \
  README.md docs/ skills/ samples/ CLAUDE.md
```
Expected: 至少 8 个文件命中（README、docs/README、docs/pipeline/cleanup-flow、docs/pipeline/decisions、docs/final/SPEC-实现约束、docs/getting-started/* 中 5 篇、samples/demo-books/README、samples/fixtures-tiny/README、skills/epub-layout-auditor/SKILL、CLAUDE.md）。

- [ ] **Step 6：README 自身校验**

```sh
wc -l README.md
grep -c "^## " README.md
```
Expected:
- `wc -l` 输出 180–230 行。
- `grep -c "^## "` 输出 ≥ 10 个二级标题。

- [ ] **Step 7：commit（如有 fix）**

如 Step 1-3 触发修改，commit 一次：
```sh
git add -A
git commit -m "docs: final sweep — no tools/ or epub-diff references remain"
```

无改动则跳过。

---

## Task 21：提 PR

**Files:** 无文件改动；纯 git / PR 操作。

- [ ] **Step 1：rebase 到最新 main**

```sh
git fetch origin main
git rebase origin/main
```
Expected: 无冲突；如有，逐个解决。

- [ ] **Step 2：推上去**

```sh
git push -u origin chore/remove-tools-and-enrich-readme
```

- [ ] **Step 3：开 PR**

```sh
gh pr create --base main --head chore/remove-tools-and-enrich-readme \
  --title "chore: remove tools/ directory; embed diff workflow in README; enrich README" \
  --body "$(cat <<'BODY'
## Summary

- Delete the entire \`tools/\` directory (was \`tools/epub-diff/\` ~1344 lines + zip.js vendor).
- Switch all human EPUB diff review to Calibre Editor (main path) and VS Code + \`unzip\` (precise path).
- The diff workflow lives **inline in the root README** under \`## EPUB diff review\` — no separate guide.
- Root README expanded from ~50 lines to ~200 lines: structured entry sections (仓库做什么 / 准备环境 / 5 分钟 / diff review / 清洗 / skills / 实测回写闭环 / 文档地图).
- All hard references in \`docs/final/SPEC-实现约束.md\`, \`CLAUDE.md\`, getting-started/, pipeline/, samples/, skills/, THIRD_PARTY.md updated to the README anchor \`#epub-diff-review\`.

## Why

- The web app needed a vendor-fetch step + local HTTP server in many cases, violating the repo's "double-click works" principle.
- Calibre's "Compare to another book" already provides character-level HTML / CSS diff, image overlays, and file-tree coloring — strictly more capable than the v0.1 in-house implementation.
- Red-line gates (\`scripts/validate_text_invariance.py\`) are entirely independent of this tool, so removal does not touch automation.
- Two historical P0 items in \`docs/plans/handbook-expansion-review.md\` (streaming SHA-256 polish and @pierre/diffs integration) are now moot.

## Detailed plan

See \`docs/plans/2026-05-28-remove-epub-diff.md\` for the review and per-task execution log.

## Migration for users

- Old: open \`tools/epub-diff/index.html\` in a browser.
- New: install Calibre 5+ → Tweak Book → File → Compare to another book. Or use VS Code with \`unzip\` + \`git diff --no-index\`. Steps: root README \`#epub-diff-review\`.
BODY
)"
```

- [ ] **Step 4：等 review**

提 PR 后停下，等用户 / 代码 review。**不要**自动 merge。

---

## 自检 Checklist（PR 前最后扫一遍）

- [ ] `tools/` 整目录已不存在（`test -d tools` 返回 false）。
- [ ] `docs/pipeline/diff-tool.md` 已删除。
- [ ] `README.md` 含 `## EPUB diff review`，行数在 180–230 之间，二级标题 ≥ 10 个。
- [ ] `README.md` 内 `Calibre` 出现 ≥ 5 次，`git diff --no-index` 出现 ≥ 3 次。
- [ ] `docs/pipeline/decisions.md` 含 Q7 / Q8 / Q9 / Q10 四条新决策。
- [ ] `docs/plans/handbook-expansion-plan.md` Stage 3 banner 已划掉并指向本计划。
- [ ] `docs/plans/handbook-expansion-review.md` 顶部已加 2026-05-28 更新 banner。
- [ ] `docs/plans/README.md` 列出本计划。
- [ ] `docs/final/SPEC-实现约束.md` §10.2 / §10.4 / §10.5 都已替换。
- [ ] `CLAUDE.md` §修改优先级第 9 条与「关键约束」最后一条都已替换。
- [ ] `skills/epub-layout-auditor/SKILL.md` 第 14 行已替换；其他 SKILL.md 中无残留。
- [ ] `THIRD_PARTY.md` 不再有 zip.js 段。
- [ ] 全仓 grep `tools/epub-diff` / `epub_diff` 仅在允许的历史档（本计划、handbook-expansion-plan 正文、handbook-expansion-review 正文、decisions.md 的删除线条目、CLAUDE.md 的删除线条目）出现。
- [ ] 至少 8 个文件引用 `#epub-diff-review` 锚点。
- [ ] PR 已推到远端，描述指向本计划。

---

## 风险与回滚

| 风险 | 触发条件 | 回滚动作 |
| --- | --- | --- |
| 用户没装 Calibre 且不熟 VS Code | 个别用户反馈 | README 已包含纯 CLI 路径（`unzip` + `git diff --no-index` + `shasum`）；不需要回滚 |
| README 锚点 `#epub-diff-review` 在某些 Markdown 渲染器（非 GitHub）失效 | 第三方 Markdown 渲染场景 | GitHub / VS Code Preview / glow 都按标准把 `## EPUB diff review` 渲染为 `#epub-diff-review`；预期成立 |
| SPEC §10 字样替换破坏既有 CI 解析 | 不太可能（SPEC 是给人和 AI 读的） | 必要时改回更宽泛的「外部 diff 工具」措辞 |
| 历史 plan 正文残留引用让搜索结果误导 | 搜索 `tools/epub-diff` 仍有命中 | 已通过 banner 明示「正文保留为历史快照」；可接受 |
| reader-matrix 或 experiments 引用了 epub-diff | grep 应已覆盖；若漏 | 在 Task 20 Step 1 扫描结果里发现并补改 |
| README 渲染过长影响首页阅读体验 | README 接近 250 行 | 这是可接受的：单一入口文档优先；如果体感过长，可在 PR review 时把 `EPUB diff review` 段折叠为 `<details>` |

整体回滚：`git revert <每个 commit>` 或 `git reset --hard origin/main` 然后重开分支。本分支只动文档与 `tools/`，没有数据迁移、没有 schema 变更。
