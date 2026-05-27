# 手册扩展 + 已有 EPUB 清洗流水线 + Diff 工具 — 详细可执行计划

> **文档定位**：本计划由本人产出、交给下一棒（人或模型）逐条执行。**每个任务都给：输入 / 输出 / 样本产物 / 验证命令 / 验收标准 / 时间估算 / 常见陷阱**。下一棒按 §阶段顺序落地。
>
> **作者**：Claude
>
> **日期**：2026-05-26
>
> **决策方向（用户已确认）**：
>
> 1. **手册定位扩为三层**：工程契约保留 + 新增入门层 + 新增批处理流水线层。
> 2. **「已有 EPUB 清洗」上升为核心功能**。
> 3. **EPUB Diff 工具 v1 = 纯静态 Web App**（不是 AI 生成 HTML 报告）。用户「打开 → 选两个 epub → 看 diff」。v1 使用内置 line diff renderer。**只做文件对比，不涉及阅读器效果验收**，不集成 Kindle Previewer 启动按钮。
> 4. **实测覆盖暂缓**。
> 5. **端到端 demo** 原计划使用鲁迅《呐喊》+《唐诗三百首》两本公版书；2026-05-27 执行时改为优先使用 `samples/demo-books/` 自造 EPUB，公版书 demo 暂缓。
>
> **核心约束（不可破）**：本计划的任何阶段都必须遵守 `CLAUDE.md` 的实测回写闭环；`docs/final/` 仍是对外约束层。

---

## 目录

- [§0 关键决策一览（先看这一节）](#0-关键决策一览先看这一节)
- [§1 整体路线图与里程碑](#1-整体路线图与里程碑)
- [§2 Stage 1：手册三层化](#2-stage-1手册三层化)
- [§3 Stage 2：已有 EPUB 清洗成为核心](#3-stage-2已有-epub-清洗成为核心)
- [§4 Stage 3：EPUB Diff Web App（含 UI 设计）](#4-stage-3epub-diff-web-app含-ui-设计)
- [§5 Stage 4：端到端 demo](#5-stage-4端到端-demo)
- [§6 跨阶段约束与不变量](#6-跨阶段约束与不变量)
- [§7 决策与待定](#7-决策与待定)
- [§8 参考资料](#8-参考资料)
- [§9 给下一棒的 Checklist](#9-给下一棒的-checklist)

---

## §0 关键决策一览（先看这一节）

下棒在落地前**必须先理解这五个决策**，否则后续会反复走回头路。

### 0.1 EPUB Diff 工具：纯静态 Web App（流式）

**形态**：

- **纯静态 web app**，单一入口 `tools/epub-diff/index.html`。
- 用户用法：双击 `index.html`（或拖入浏览器）→ 看到两个文件拾取框（支持点击 / 拖拽）→ 选 `before.epub` 和 `after.epub` → 点 `Compare` → 看 diff。
- **完全在浏览器内**完成 epub 解压、解析、diff、渲染。没有后端、没有 Python 调用、没有 AI 生成报告这一步。
- 核心 diff 渲染使用内置 line diff renderer；复杂 inline diff 留到后续版本。
- 浏览器内解压用 **[zip.js](https://gildas-lormeau.github.io/zip.js/)**（`@zip.js/zip.js`，30KB，流式 + 随机访问，**不一次性把整包加载进内存**）。
- 配合 zip.js 自定义 Writer 和本仓增量 SHA-256，资源层边解压边 hash，字节不驻留。
- 设计目标支撑到 **~1.5GB epub**（普通桌面 / 笔记本 / 现代移动浏览器都能跑）。
- **只做文件对比**，不涉及任何阅读器效果验收（不嵌阅读器 / 不渲染 epub / 不启动 Kindle Previewer）。
- 离线可用：所有依赖（zip.js）本地化在 `assets/vendor/` 下。

**浏览器最低版本**：Safari 16.4+ / Chrome 100+ / Firefox 101+ / Edge 100+。

> 注：超出 1.5GB epub 或需要批量并行处理时，再讨论是否包装成桌面 app。当前**不为这种场景预先投入**。

### 0.2 UI 设计基调：YiTong 风格 + Landing Page + EPUB 五层分区

UI 借鉴 YiTong：

- 顶部信息条 + 颜色编码状态。
- 左侧固定导航 / 右侧滚动主区。
- 默认 split 视图，可切 stacked。

但 v1 是 web app，多一个 YiTong 没有的元素：

- **Landing Page（启动页）**：进入工具时先看到两个文件拾取框 + Compare 按钮。

进入比较视图后，按 EPUB 专属五层组织：**结构 / 文本 / 样式 / 资源 / 元数据**。每层独立可折叠。

详见 §4.2。

### 0.3 与清洗流水线（Stage 2）的关系

- Web app **只服务于人**：用户清洗完一本 epub，想看「改前 / 改后到底差在哪」时打开它。
- **CI / 自动化的 red-line gate 由 Stage 2 的 `validate_text_invariance.py` 单独负责**，不依赖 web app。
- AI 在 Stage 2 清洗流程中**不调用** web app；AI 只调用 `validate_text_invariance.py`（红线检查）和具体 cleanup skills。

这样的好处：web app 形态稳定（人用），CI gate 形态稳定（脚本），各管各的，不会因为 web app 重设计影响 CI。

### 0.4 Stage 串行

Stage 1 → 2 → 3 → 4 严格串行。**禁止并行**，理由不变（详见 §1）。

### 0.5 Demo 选择

原计划选择 **鲁迅《呐喊》**（现代汉语小说集）+ **《唐诗三百首》**（古典文本）。执行时用户确认先不做公版书 demo，因为当前样本偏繁体且版式不够丰富。Stage 4 改为自造 `city-field-notes`、`paper-garden` 和 `redline-trap` 三组 EPUB。

### 0.6 依赖最小化

- **Python 端**（Stage 2 用）：仅新增 `lxml`（XHTML / OPF / NCX 解析）。
- **Web app 前端**（Stage 3 用）：本地化的 zip.js + 内置 line diff renderer，**无 Node 运行时**。
- **构建工具**：本地化 assets 时用一次性 `npm pack`（不是运行时依赖）。

**不引入**：tinycss2、Jinja2（v1 web app 路线不再需要 Python 端 HTML 模板）、BeautifulSoup、PyYAML、Pandas、Pillow、任何 Node.js runtime、任何 ML/NLP 库、任何 Web 框架、任何 Electron。

---

## §1 整体路线图与里程碑

```text
            v1.0                v1.1              v1.2              v1.3
─ Stage 1 ─┬─ Stage 2 ────────┬─ Stage 3 ────────┬─ Stage 4 ───────┤
手册三层化  │  清洗 SPEC §10    │  EPUB Diff       │  自造 demo      │
README     │  validate_text    │  Web App         │  Lu Xun        │
入门层 5 篇 │  cleanup-flow     │  index.html      │  Tang poems    │
pipeline 占位│ samples 占位      │  zip.js + line   │  case study   │
            │                  │                  │                │
≈ 2 周      │  ≈ 3 周           │  ≈ 3 周           │  ≈ 1.5 周        │
```

**里程碑产出**：

| 里程碑 | 产出 | 验收信号 |
| --- | --- | --- |
| **M1.0** | Stage 1 完成 | `docs/getting-started/` 5 篇 + README 重写 + `docs/pipeline/` 占位 |
| **M1.1** | Stage 2 完成 | SPEC §10 + `validate_text_invariance.py` + cleanup-flow guide |
| **M1.2** | Stage 3 完成 | `tools/epub-diff/index.html` 可在 Chrome/Safari/Firefox 内打开、选两个 epub、看到五层 diff |
| **M1.3** | Stage 4 完成 | `samples/demo-books/` 三组自造 EPUB + case study |

**总时长**：约 9.5 周（基础任务 + 本次新增的入门 / 清洗扩展任务）。

**Stage 顺序为什么严格串行**：

- Stage 2 SPEC §10 的红线定义 = Stage 3 web app 顶部红线 banner 的判断依据。
- Stage 3 web app 是 Stage 4 demo 的可视化工具。
- Stage 1 的目录结构（`docs/pipeline/` / `samples/demo-books/` / `samples/third-party/` / `tools/`）是 Stage 2/3/4 的承载位置。

---

## §2 Stage 1：手册三层化

### 2.1 目标

让一个完全没接触过本仓的人 / AI，在 **5 分钟**内知道：

1. 仓库是做什么的；
2. 自己应该看哪一层文档；
3. 自己的场景属于哪一类。

### 2.2 三层结构定义

| 层 | 现位置 | 计划位置 | 目标读者 | 内容性质 |
| --- | --- | --- | --- | --- |
| **入门层** | 无 | `docs/getting-started/`（新） + 重写根 README | 第一次来的人 | 概念、示例、5 分钟跑通 |
| **工程契约层** | `docs/final/` | 不动 | 中高级制作者 + AI | 硬规则、SPEC、速查 |
| **批处理流水线层** | 无 | `docs/pipeline/`（新） | 批量操作者 + AI | 流程、CLI、自动化 |

`docs/guides/` 保留作为中间层专题指南。
`docs/source/` 与 `docs/experiments/` 不变。

### 2.3 任务清单

#### S1-T1：重写顶层 `README.md`

**输入**：当前根 `README.md`。
**输出**：根 `README.md`（≤ 100 行）。
**时间估算**：30 分钟。

**完整样本**（drop-in）：

````markdown
# epub-handbook

中文 EPUB 3 制作与 AI 协作工具集。

适合的人：制作中文 EPUB 3 的工程师、想用 AI 帮忙清洗已有 epub 的编辑、想给团队约定 epub 制作规范的 maintainer。

## 我要做什么？

| 我的场景 | 入口 |
| --- | --- |
| 从零做一本新书 | [docs/getting-started/](docs/getting-started/) → `templates/epub-style-demo/` |
| 改造一本现成 EPUB | [docs/pipeline/](docs/pipeline/) → `scripts/validate_text_invariance.py` |
| 想看改前 / 改后差异 | [tools/epub-diff/index.html](tools/epub-diff/index.html) → 浏览器打开 |
| 想看制作规范 | [docs/final/](docs/final/) |
| 给 AI 接入 | [skills/](skills/) + `agents/openai.yaml` |

## 5 分钟跑通

```sh
bash templates/epub-style-demo/build.sh
bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<新建的 .epub>
```

详细教程见 [docs/getting-started/01-first-epub.md](../getting-started/01-first-epub.md)。

## 看改前 / 改后差异

把 `tools/epub-diff/index.html` 用浏览器打开（双击或拖入 Chrome / Safari / Firefox），选两个 epub 文件，点 Compare。详细见 [docs/pipeline/diff-tool.md](../pipeline/diff-tool.md)。

## 这个仓库不是什么

- 不是初级排版课。
- 不是封闭格式（mobi/AZW3）的制作工具。
- 不是 epub.js 阅读器。
- 不是 Kindle 自费出版的运营指南。
- 不是用来检验 epub 在某阅读器里「显示效果」的工具（diff 工具只比文件，不模拟渲染）。

## 整体目录

详见 [docs/README.md](../README.md)。

## 协作

阅读 [CLAUDE.md](../../CLAUDE.md) 了解 AI 协作约定。本仓所有约束变更都走 demo → reader-matrix → SPEC → 手册 → 速查表 → skills 的实测闭环。

## 许可

代码部分 MIT；文档与样本许可参见 [THIRD_PARTY.md](../../THIRD_PARTY.md)。

````

**验收命令**：

```sh
wc -l README.md   # ≤ 100
grep -c "^|" README.md   # ≥ 4 行表格
grep "tools/epub-diff" README.md   # 必须出现
```

**陷阱**：不要把旧 README 的目录树整段保留；目录索引迁到 `docs/README.md`。

---

#### S1-T2：新建 `docs/getting-started/`

**输入**：无。
**输出**：5 个 markdown 文件。
**时间估算**：1–1.5 天。

**目录**：

```text
docs/getting-started/
├── README.md                # 入门总览
├── 01-first-epub.md         # 5 分钟跑通
├── 02-anatomy.md            # epub 结构剖析
├── 03-readers.md            # 阅读器矩阵
├── 04-skills.md             # AI skills 怎么用
└── 05-case-study.md         # 占位，Stage 4 填
```

**`README.md` 内容**（drop-in）：

```markdown
# 入门

本目录给第一次接触本仓的人。

## 如何使用

按顺序读完 5 篇即可：

1. [01-first-epub.md](../getting-started/01-first-epub.md) — 5 分钟做一本最小 EPUB
2. [02-anatomy.md](../getting-started/02-anatomy.md) — 一本 epub 到底由什么组成
3. [03-readers.md](../getting-started/03-readers.md) — 我应该在哪些阅读器上测
4. [04-skills.md](../getting-started/04-skills.md) — AI skills 是什么、怎么用
5. [05-case-study.md](../getting-started/05-case-study.md) — 自造 EPUB 清洗演示案例

读完后，你就能进入：
- 工程契约（[docs/final/](../final/)）查看硬规则与速查表；
- 批处理流水线（[docs/pipeline/](../pipeline/)）做批量清洗；
- Diff 工具（[tools/epub-diff/index.html](../../tools/epub-diff/index.html)）对比改前 / 改后。
```

**`01-first-epub.md` 内容**（drop-in 5 步）：

````markdown
# 5 分钟做一本最小 EPUB

## 你需要

- 一台装了 `zip` 命令的电脑（macOS / Linux 自带；Windows 用 Git Bash 或 WSL）。
- Python 3.10+。
- 一个现代浏览器（Chrome / Safari / Firefox 任一，用来开 diff 工具）。

## 步骤

### 1. 克隆仓库

```sh
git clone https://github.com/<owner>/epub-handbook.git
cd epub-handbook
```

### 2. 构建示例 EPUB

```sh
bash templates/epub-style-demo/build.sh
```

输出会打印一个 .epub 路径，如：

```
templates/epub-style-demo/dist/epub-style-demo-20260526-091234.epub
```

### 3. 跑校验

```sh
EPUB="templates/epub-style-demo/dist/epub-style-demo-20260526-091234.epub"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
```

退出码 0 = 通过。

### 4. 改一段文字，重新构建，再用 diff 工具看看改了什么

```sh
# 编辑 templates/epub-style-demo/OEBPS/Text/01-body.xhtml，改一行文字
bash templates/epub-style-demo/build.sh
ls -t templates/epub-style-demo/dist/*.epub | head -2
```

把 `tools/epub-diff/index.html` 用浏览器打开（直接双击或拖入浏览器），选这两个 epub 做对比。

### 5. 用阅读器打开

- **macOS**：双击文件，用 Apple Books 打开。
- **iOS**：通过 iCloud Drive 或 AirDrop 发到手机。
- **Kindle**：拖入 Kindle Previewer 3（[下载](https://www.amazon.com/Kindle-Previewer/b?ie=UTF8&node=21381691011)）。
- **任何浏览器**：用 [epub.js 在线 reader](https://futurepress.github.io/epubjs-reader/)。

## 下一步

- 看 [02-anatomy.md](../getting-started/02-anatomy.md) 了解 epub 内部结构。
- 看 [docs/final/](../final/) 学习硬规则。
- 看 [skills/](../../skills/) 了解 AI 协作能力。
````

**`02-anatomy.md` / `03-readers.md` / `04-skills.md` 大纲**（下棒按此 outline 写，每篇 ≤ 250 行）：

- `02-anatomy.md`：解释 `mimetype` / `META-INF/container.xml` / `OEBPS/package.opf` / `OEBPS/nav.xhtml` / `OEBPS/toc.ncx` / `OEBPS/Text/*.xhtml` / `OEBPS/Styles/*.css` / `OEBPS/Images/*` / `OEBPS/Fonts/*` 各自角色。配纯文本树（`tree` 输出）。
- `03-readers.md`：4 大阅读器（Apple Books / Kindle / Thorium / Readest）的优劣、下载链接；`reader-matrix.yaml` 怎么读。
- `04-skills.md`：14 个 skill 一行简表；如何在 Claude Code / Codex / Gemini 激活。

**验收命令**：

```sh
for f in docs/getting-started/{README,01-first-epub,02-anatomy,03-readers,04-skills}.md; do
  test -f "$f" || echo "MISSING: $f"
  wc -l "$f"
done
```

每个文件 ≤ 250 行；`01-first-epub.md` 必须有「复制粘贴可跑」的 5 步流程。

**陷阱**：

- 不要嵌入截图（保持纯 markdown）。
- 不要在入门层引用 SPEC 硬规则细节；只指引去 final/。

---

#### S1-T3：新建 `docs/pipeline/` 占位

**输入**：无。
**输出**：`docs/pipeline/README.md`。
**时间估算**：15 分钟。

**完整内容**（drop-in）：

````markdown
# 批处理流水线

把已有 epub 批量处理（清洗、改造、对比、回写）的工作流文档。

## 工作流概览

```

┌────────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐
│  intake    │ -> │ analyze  │ -> │ cleanup  │ -> │   diff   │ -> │ accept │
└────────────┘    └──────────┘    └──────────┘    └──────────┘    └────────┘
   harness        layout-auditor  按 skills      Web App         人工 +
   findings.json  推荐 skills      执行清洗      用户打开看 diff   reader-matrix
                                  validate_text                  回写
                                  红线 gate

```

## 每一步对应的工具

| 步骤 | 工具 | 文档 |
| --- | --- | --- |
| intake | `scripts/epub_ai_harness.py <epub>` | [cleanup-flow.md](../pipeline/cleanup-flow.md) |
| analyze | `epub-layout-auditor` skill | `skills/epub-layout-auditor/SKILL.md` |
| cleanup | 各专项 skill（按 §10 红/黄/绿规则） | [SPEC §10](../final/SPEC-实现约束.md) |
| 红线 gate（CI / 自动化）| `scripts/validate_text_invariance.py` | [SPEC §10.5](../final/SPEC-实现约束.md) |
| diff（人工 review） | `tools/epub-diff/index.html` 浏览器打开 | [diff-tool.md](../pipeline/diff-tool.md) |
| accept | reader-matrix 回写 + 用户确认 | `docs/final/reader-matrix.yaml` |

## 我现在该看什么

- 第一次接触清洗流水线：[../pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)
- AI 改动边界：[../final/SPEC-实现约束.md §10](../final/SPEC-实现约束.md)
- Diff 工具使用：[../pipeline/diff-tool.md](../pipeline/diff-tool.md)
- 真实案例：[../getting-started/05-case-study.md](../getting-started/05-case-study.md)

## 决策记录

按 §7.1 在本目录维护 `decisions.md`，记录每个 Stage 落地前回答的必答题。
````

**验收**：文件存在；链接全部指向真实路径或在本计划其它 Stage 中明确会落地。

---

#### S1-T4：新建 `docs/README.md` 目录索引

**输入**：根 README 中的旧目录树。
**输出**：`docs/README.md`。
**时间估算**：20 分钟。

**完整内容**（drop-in）：

```markdown
# 文档目录

## 入门
- [getting-started/](getting-started/)
  - [01-first-epub.md](../getting-started/01-first-epub.md)
  - [02-anatomy.md](../getting-started/02-anatomy.md)
  - [03-readers.md](../getting-started/03-readers.md)
  - [04-skills.md](../getting-started/04-skills.md)
  - [05-case-study.md](../getting-started/05-case-study.md)

## 工程契约（硬规则）
- [SPEC-实现约束.md](../final/SPEC-实现约束.md)
- [EPUB 3 终极实践手册.md](../final/EPUB 3 终极实践手册.md)
- [EPUB 3 HTML CSS 属性速查表.md](../final/EPUB 3 HTML CSS 属性速查表.md)
- [reader-matrix.yaml](../final/reader-matrix.yaml)

## 专题指南
- [guides/](guides/)
  - 字体策略：[fonts-css-expansion-plan.md](./fonts-css-expansion-plan.md)
  - CSS 分层：[css-layering-plan.md](./css-layering-plan.md)
  - Demo 扩展：[demo-scene-expansion-plan.md](./demo-scene-expansion-plan.md)
  - 文白对照：[classical-modern-layout.md](../guides/classical-modern-layout.md)
  - 英文小说：[english-fiction-layout.md](../guides/english-fiction-layout.md)
  - 章首图：[chapter-head-image.md](../guides/chapter-head-image.md)
  - 便签框：[note-box-border-styles.md](../guides/note-box-border-styles.md)
  - 合集导航：[anthology-navigation.md](../guides/anthology-navigation.md)
  - 多看 fallback：[duokan-footnote-fallback-fix.md](../guides/duokan-footnote-fallback-fix.md)
  - 清洗流程：[cleanup-flow.md](../pipeline/cleanup-flow.md)（Stage 2 落地）
  - 典型脏 epub 模式：[cleanup-patterns.md](../pipeline/cleanup-patterns.md)（Stage 2 落地）
  - 资源优化（图片 / 字体）：[asset-optimization.md](../pipeline/asset-optimization.md)（Stage 2 落地）
  - Diff 工具：[diff-tool.md](../pipeline/diff-tool.md)（Stage 3 落地）

## 批处理流水线
- [pipeline/](pipeline/)

## 工具
- [tools/epub-diff/](../tools/epub-diff/)（Stage 3 落地）

## 推导与实验（非约束）
- [source/](source/)
- [experiments/](experiments/)
```

**验收**：所有引用的文件都真实存在或在本计划其它 Stage 中明确会落地。

---

#### S1-T5：同步 `CLAUDE.md` 的修改优先级

**输入**：当前 `CLAUDE.md`。
**输出**：增补 `docs/getting-started/` 与 `docs/pipeline/`、`tools/` 的位置说明。
**时间估算**：10 分钟。

**新版「修改优先级」段**：

```markdown
1. `templates/`：可运行样式样本和机器消费源。遇到阅读器显示、打包、转换兼容性问题时，先补 demo fixture 并实测。
2. `docs/final/`：对外约束层，任何改动都应视为规范变更；必须由 demo fixture 或明确实测结果支撑。
3. `skills/*/SKILL.md`：自动化行为契约，修改需保持向后兼容。
4. `docs/guides/`：维护说明和工作流建议，不承载下游架构。
5. `docs/getting-started/`：入门教程，可自由补充；遇到与 final/ 冲突时以 final/ 为准。
6. `docs/pipeline/`：批处理流水线工作流文档，与 docs/guides/ 同级。
7. `tools/`：面向用户 / maintainer 的本地工具（如 epub-diff web app）。修改要保证「双击即用」的最小依赖原则。
8. `docs/source/`、`docs/experiments/`：推导与实验区，可自由补充但不应反向覆盖约束层。
9. `samples/third-party/`：公版书样本与许可记录；实体 .epub 不入 git。
```

**验收**：`grep -nE "getting-started|pipeline|samples/third-party|^7\. \`tools/\`" CLAUDE.md` 至少 4 条匹配。

---

#### S1-T6：新建 `docs/getting-started/06-test-your-own.md`

**目的**：教用户「我手头有一本 epub，怎么开始测」。S1-T2 的 01 只跑 demo，没回答这个问题。

**输入**：无。
**输出**：`docs/getting-started/06-test-your-own.md`。
**时间估算**：3–4 小时。

**完整骨架**（drop-in）：

````markdown
# 测自己的 EPUB

跑通 [01-first-epub.md](../getting-started/01-first-epub.md) 之后，你应该能把自己的 epub 跑一遍这套工具链。

## 0. 准备

```sh
mkdir -p work
cp /path/to/your-book.epub work/source.epub
```

**不要原地覆盖**原始 epub。

## 1. 用 epubcheck 跑一次（推荐）

[epubcheck](https://www.w3.org/publishing/epubcheck/) 是 W3C 官方校验工具，是「合法 EPUB」的最低底线。

```sh
# macOS
brew install epubcheck

# 跑校验
epubcheck work/source.epub
```

输出会列出 fatal / error / warning。fatal 必须修；error 一般要修；warning 看情况。

## 2. 用本仓 validator 跑一次

```sh
bash scripts/validate-epub-style-demo.sh --epub work/source.epub
```

**注意**：这个脚本是为 `templates/epub-style-demo/` 设计的，对真实 epub 会报很多「missing fixture token」。这些**对真实 epub 是预期失败**，跳过即可。
真正有用的是它附带的几项通用校验：mimetype、META-INF/container.xml、OPF 完整性、CSS url() 引用。

## 3. 用 epub-diff 工具确认基线

把 `tools/epub-diff/index.html` 拖入浏览器，两边都选 `work/source.epub`：

- 期望：所有五层都「identical」。
- 如果出现差异：说明 diff 工具自身有 bug（极少见）或文件读取不稳定，重新拉取并重试。

这一步确认工具基线 OK，后续比对才有意义。

## 4. 调用 layout-auditor skill 看 findings

如果你装了 Claude Code / Codex / 类似 AI 协作环境，调用：

```text
请使用 epub-layout-auditor 审稿 work/source.epub
```

skill 会输出：
- 页面类型分类（正文 / 弹注 / 图片 / 表格 / ...）。
- 风险清单（P0–P3）。
- 推荐分派到的专项 skill。

## 5. 决定是否清洗

把 §4 的 findings 对照 [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)：

- 红线很多（文本错误、缺核心 metadata）→ **不要清洗**，先回到源头校对。
- 黄线为主（样式 / 字体 / 结构混乱）→ 可以进入清洗流水线。
- 绿线为主（仅格式化噪声）→ 一行命令格式化即可，未必走完整流水线。

## 6. 用阅读器实测

清洗前后都用目标阅读器打开看：

- Apple Books（默认字号 + 大字号）。
- Kindle Previewer（默认 profile + Paperwhite profile + 字号 1/4/7）。
- 多看 / Readest（如目标读者用这些）。

把实测结果记下来；如果你想贡献回本仓，参 [CONTRIBUTING.md](../../CONTRIBUTING.md) 的 reader-matrix 回写规范。

## 7. 卡住了？

去看 [07-faq.md](../getting-started/07-faq.md) 或 [glossary.md](../getting-started/glossary.md)。
````

**验收**：

```sh
test -f docs/getting-started/06-test-your-own.md
grep -q "epubcheck" docs/getting-started/06-test-your-own.md
grep -q "tools/epub-diff" docs/getting-started/06-test-your-own.md
```

---

#### S1-T7：新建 `docs/getting-started/07-faq.md`

**目的**：把新手最高频翻车点单独成页，省去他们去翻 issue 的力气。

**输入**：无。
**输出**：`docs/getting-started/07-faq.md`。
**时间估算**：2–3 小时。

**完整骨架**（drop-in）：

````markdown
# 常见问题

## 环境 / 安装

**Q：跑 `build.sh` 报「zip is required」？**
A：你的系统没有 `zip` 命令。
- macOS / Linux：自带，重启终端试试。
- Windows：用 WSL（Windows Subsystem for Linux）或 Git Bash；纯 cmd / PowerShell 没 `zip`。

**Q：Python 报「No module named 'lxml'」？**
A：装一下：`pip install lxml`。如果 pip 也没有：`python3 -m pip install lxml`。

**Q：Windows 路径里有空格，命令报错？**
A：路径用双引号包：`bash scripts/validate-epub-style-demo.sh --epub "C:/Users/My Books/test.epub"`。

**Q：需要 Node.js 吗？**
A：本仓**运行时不需要**。只有 `tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor assets 时用一次 `npm pack`，抓完就用不到了。

## Validator / 校验

**Q：`validate-epub-style-demo.sh` 对我的 epub 报「missing fixture token」？**
A：那个脚本是为本仓 `templates/epub-style-demo/` 设计的，对真实 epub 会大量误报。真实 epub 用 [epubcheck](https://www.w3.org/publishing/epubcheck/)：
```sh
brew install epubcheck && epubcheck your-book.epub
```

**Q：epubcheck 报「fatal」一类错误？**
A：fatal 是「这甚至不是合法 epub」。检查：
1. zip 是否完整（`unzip -t your.epub`）。
2. 第一个 zip entry 是不是 `mimetype`（且内容是 `application/epub+zip`）。
3. `META-INF/container.xml` 存在且能解析。

## 阅读器

**Q：Kindle Previewer 转换失败 / 文档打不开？**
A：先看 [docs/final/reader-matrix.yaml](../final/reader-matrix.yaml) 里 `kindle_previewer` 的已知 issue。如果你的报错没列在那里：
1. 把 epub 在 Apple Books 打开，如果 Apple Books 也打不开：epub 自身坏的。
2. 如果只 Kindle Previewer 坏：可能踩到 KFX 转换 bug；试试拆出最小复现 case。

**Q：Apple Books 不刷新 / 看到的还是旧版本？**
A：Apple Books 会缓存。把旧版本「移到废纸篓」（在 Books 应用里右键），再重新拖入新文件。

**Q：字体没生效？**
A：按顺序检查：
1. 字体文件在 `OEBPS/Fonts/` 里？
2. OPF manifest 声明了 item？
3. CSS 里 `@font-face` 用的是相对路径？
4. 用户是否在阅读器里开了「使用 Publisher Font」（Kindle 默认关闭）？

## EPUB Diff 工具

**Q：双击 `tools/epub-diff/index.html` 一片空白？**
A：先跑 `bash tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor。

**Q：在 `file://` 协议下打开 index.html，浏览器报 module 加载错误？**
A：部分浏览器（Firefox / Safari 严格）对 `file://` 上的 ES module 有限制。解决：
```sh
cd tools/epub-diff/
python3 -m http.server 8000
# 浏览器打开 http://localhost:8000/
```

**Q：拖了大 epub 进去，浏览器卡死？**
A：v1 支持到约 1.5GB。超过的话目前没有替代方案，记录为开放问题。

## AI 协作

**Q：我没有 Claude Code / Codex / Gemini，怎么用 skills？**
A：[skills/](../../skills/) 本身就是 markdown 文档，可以人工跟着走。不一定要 AI 自动跑。

**Q：skill 把我的正文文字改了？**
A：这是事故，应该回滚。SPEC §10.1 明确说正文文本是红线。同时跑 `scripts/validate_text_invariance.py` 把改动回退，并把这个 case 报给本仓 issue。

## 还没解决？

- 看 [glossary.md](../getting-started/glossary.md) 确认术语理解一致。
- 看 [docs/final/reader-matrix.yaml](../final/reader-matrix.yaml) 是否已经记录。
- 提 issue 时附上：epub 来源（可去识别化）、出错命令、完整输出。
````

**验收**：

```sh
test -f docs/getting-started/07-faq.md
grep -cE "^\*\*Q：" docs/getting-started/07-faq.md   # 期望 ≥ 10 个 Q&A
```

---

#### S1-T8：新建 `docs/getting-started/glossary.md`

**目的**：术语表。EPUB / Kindle / 中文排版圈各自的黑话对新手不友好，单独一页查表。

**输入**：无。
**输出**：`docs/getting-started/glossary.md`。
**时间估算**：2 小时。

**完整骨架**（drop-in，按字母 / 拼音排序，下棒可补充）：

````markdown
# 术语表

按字母顺序。每条一句话定义 + 在本仓哪里有更详细说明。

## A

- **A-lite**：本仓自定义的「轻量增强 EPUB」配置组合（特定 CSS + 字体 + 海报页路径）。详见 [SPEC §3](../final/SPEC-实现约束.md)。
- **AZW3**：Amazon 私有 EPUB 衍生格式，KF8 时代的容器。Kindle Previewer 转换的中间产物之一。

## C

- **container.xml**：`META-INF/container.xml`，告诉阅读器 OPF 位置；EPUB 必备文件。

## D

- **DRM**（Digital Rights Management）：内容加密 / 授权限制。本仓清洗流水线**拒绝处理**有 DRM 的 epub。
- **DocBook / DocType**：XHTML 声明 `<!DOCTYPE html>`；EPUB 3 必填，不能用 XHTML 1.x 的复杂 DTD。

## E

- **epub:type**：EPUB 3 语义注解属性，标记元素角色（如 `epub:type="footnote"`、`epub:type="chapter"`）。Kindle 弹注依赖它。
- **epubcheck**：W3C 官方 epub 校验器；行业 baseline 工具。详见 [01-first-epub.md](../getting-started/01-first-epub.md)。
- **Enhanced Typesetting**：Amazon 的「增强排版」开关，KFX 转换后启用；支持现代 CSS 子集（float、hyphens、ligatures 等）。详见 [docs/final/SPEC-实现约束.md](../final/SPEC-实现约束.md)。

## F

- **fixed-layout**（FXL）：固定版式 EPUB，每页都是一张设计稿；不能重排字号；漫画 / 绘本 / 杂志用。**本仓不做 FXL**。
- **footnote / 弹注**：Kindle / Apple Books 在脚注链接处弹出 tooltip 的能力。本仓有 `epub-popup-footnote-converter` skill。

## K

- **KFX**：Kindle 当前主流容器，KF8 的后继。KDP 上传 EPUB 后内部转 KFX。
- **KF8**：Kindle Format 8，Amazon 第二代格式；现在大多数 KFX 设备也兼容 KF8 子集。
- **KDP**（Kindle Direct Publishing）：Amazon 自助出版平台。

## M

- **manifest**：OPF 中列出所有 epub 内文件的清单。
- **MathML**：数学公式标记语言；Kindle Enhanced Typesetting 在特定标签集合下支持。
- **MOBI**：Amazon 第一代格式；KFX 时代已淘汰，但部分 Kindle 模拟器还支持。
- **多看**：国内主流中文 EPUB 阅读器，对部分 CSS 有自定义扩展。

## N

- **NCX**（`toc.ncx`）：EPUB 2 时代的导航文件；EPUB 3 保留作向后兼容。
- **nav.xhtml**：EPUB 3 的目录文件，声明 `<nav epub:type="toc">`。

## O

- **OPF**（`package.opf`）：EPUB 的「项目文件」，包含 metadata + manifest + spine。
- **OEBPS**：EPUB 内放正文的目录名，按惯例叫这名（不是规范要求）。

## P

- **properties**：OPF manifest item 上的属性，如 `properties="cover-image"`、`properties="svg"`、`properties="mathml"`。阅读器靠这个识别特殊文件。
- **PG**：[Project Gutenberg](https://www.gutenberg.org/)，主流公版书来源。

## R

- **reflowable**：可重排版式（与 fixed-layout 相对），文字会跟随屏宽 / 字号自动重新流动。本仓主路径。
- **Ruby / 注音**：CJK 标注，`<ruby>` 元素配合 `<rt>` 给汉字加注音。
- **reader-matrix**：本仓 `docs/final/reader-matrix.yaml`，记录每个 fixture 在每个 reader 上的实测结果。

## S

- **Sigil**：开源 epub 编辑器；常被 fan-made epub 用，会留下特定的 sigil_toc_id_N 风格 id。
- **spine**：OPF 中声明 epub 的「阅读顺序」。重排或删除 spine 是清洗的红线。
- **SubtleCrypto**：浏览器 Web Crypto API；本仓 web app 用它处理文本等小块 SHA-256，资源层使用增量 SHA-256 writer。

## T

- **Tauri**：Rust + 系统 webview 的桌面 app 框架。本仓的桌面 app 路径**当前不做**。

## V

- **Validator**：校验器。本仓有 `validate-epub-style-demo.sh`、`validate-popup-notes.sh`、`validate_text_invariance.py`。

## X

- **XHTML**：EPUB 正文文件格式；语法上是 XML，比普通 HTML 严格（必须闭标签、属性带引号）。
- **xml:lang**：标记元素语言；如 `xml:lang="zh-CN"`（白话）/ `xml:lang="lzh"`（文言）。

## Z

- **zip.js**：[gildas-lormeau/zip.js](https://gildas-lormeau.github.io/zip.js/)；本仓 web app 用的 ZIP 流式解析库。
````

**验收**：

```sh
test -f docs/getting-started/glossary.md
grep -cE "^## " docs/getting-started/glossary.md   # 期望 ≥ 10 个字母分组
```

---

#### S1-T9：扩展 `docs/getting-started/04-skills.md`，加反向查表

**目的**：04-skills.md 当前是「按 skill 列出」，新人不知道「我想做 X 用哪个」。加一个反向查表。

**输入**：S1-T2 中 04-skills.md 的初稿。
**输出**：04-skills.md 末尾增加「我想做 X，用哪个 skill」一节。
**时间估算**：1 小时。

**新增节内容**（drop-in，按本仓 14 个现有 skill 整理）：

````markdown
## 我想做 X，用哪个 skill？

| 我要做什么 | Skill |
| --- | --- |
| 拿到一本 epub 不知从哪下手，先看大局 | `epub-layout-auditor` |
| 我有 txt / md / PDF，需要先变成 epub source | `epub-source-intake` |
| 把弹注（标准 footnote）做规范 | `epub-popup-footnote-converter` |
| 多看 / 旧版阅读器看不到弹注，加 fallback | `epub-legacy-footnote-fallback` |
| 给生僻字加 Ruby 注音 / 整本竖排 | `epub-vertical-ruby-optimizer` |
| 中英混排排版、字号 / 行距、首字下沉等 | `epub-typography-optimizer` |
| 英文小说专项排版（字体、`::first-letter`、手写体下沉） | `epub-english-typography-optimizer` |
| 图片混排（环绕、海报、章首图）规范化 | `epub-image-layout-optimizer` |
| CSS 文件臃肿 / 内联样式过多，做分层 | `epub-css-layering-optimizer` |
| 弹注 / 文学结构 / 出处规范化 | `epub-literary-structure-formatter` |
| Kindle 转换失败 / Enhanced Typesetting 问题 | `epub-kindle-compatibility-checker` |
| 一份普通 epub 转成 A-lite 增强版（海报 / 字体策略 / 竖排支持） | `epub-alite-converter` |
| OPF / nav.xhtml / toc.ncx 检查与修复 | `epub-package-nav-auditor` |
| 维护本仓的 demo fixture（开发者用，非清洗） | `epub-style-demo-maintainer` |

> 不确定属于哪一类时，先调用 `epub-layout-auditor`，它会告诉你后续派哪一个。
````

**验收**：

```sh
grep -c "epub-" docs/getting-started/04-skills.md   # 期望 ≥ 14
grep -q "我要做什么" docs/getting-started/04-skills.md
```

---

#### S1-T10：扩展 `docs/getting-started/03-readers.md`，加阅读器决策树

**目的**：03-readers.md 当前列了 4 大阅读器，但新人不知道「只能装一个用哪个 / 先测哪个」。

**输入**：S1-T2 中 03-readers.md 的初稿。
**输出**：03-readers.md 末尾增加「我该测哪个」决策树。
**时间估算**：1 小时。

**新增节内容**（drop-in）：

````markdown
## 我该测哪个阅读器？

按场景选：

### 场景 A：你只能选一个

**选 Apple Books（macOS / iOS 自带）**。理由：
- 装好就能用，无下载安装步骤。
- CSS 支持是几大主流阅读器中最完整的，能跑通的 epub 大概率别处也能跑。
- 反过来说：Apple Books 跑不通的 epub，一定有问题。

### 场景 B：目标是 Kindle 商业发行

**必测 Kindle Previewer 3**（[下载](https://www.amazon.com/Kindle-Previewer/b?ie=UTF8&node=21381691011)）。
- 这是 KDP 上传前的官方校验工具，转换报告失败时 KDP 也会拒。
- 至少测三个 profile：电子书阅读器（默认）、Paperwhite、Scribe。
- 至少测三档字号：1（最小）、4（默认）、7（最大）。

### 场景 C：目标是中文读者

加测 **多看阅读**（iOS / Android）和 **Readest**（macOS / Windows / Linux）。
- 多看对中文排版细节（标点挤压、首字调整）更敏感。
- Readest 是新兴跨平台开源阅读器，逐渐成为重排 epub 的对照基准。

### 场景 D：你想做最严肃的兼容性矩阵

按这个顺序测：

1. Apple Books（基线）
2. Kindle Previewer（Kindle 兼容性）
3. Thorium Reader（macOS / Win / Linux；EPUB 3 spec 实现最严格）
4. Readest（跨平台中文友好）
5. 多看（国内中文细节）

> 这个顺序也是本仓 `docs/final/reader-matrix.yaml` 收录的优先级。
````

**验收**：

```sh
grep -q "我该测哪个" docs/getting-started/03-readers.md
grep -q "Apple Books" docs/getting-started/03-readers.md
grep -q "Kindle Previewer" docs/getting-started/03-readers.md
```

---

#### S1-T11：扩展 `docs/getting-started/README.md`，加 do/don't + 「读完入门去哪」

**目的**：让入门 README 不只是导航页，还提供 5–10 条「速记速戒」和后续深入路径。

**输入**：S1-T2 中 getting-started/README.md 的初稿。
**输出**：在 README.md 末尾新增两节。
**时间估算**：1 小时。

**新增内容**（drop-in，追加到 README.md 末尾）：

````markdown
## 速查：一定要做 / 一定不要

**一定要做**：

1. EPUB 第一个 zip entry 必须是 `mimetype`，且内容是 `application/epub+zip`，**不压缩**。
2. 所有正文是可选中的文本，不是图片（FXL 漫画除外，不在本仓范围）。
3. OPF manifest 列出每个 epub 内的文件；spine 决定阅读顺序；缺一不可。
4. 每个章节 XHTML 是合法 XML（标签必须闭合、属性带引号）。
5. 用 `xml:lang` 标记每段的语言（特别是中英混排）。

**一定不要**：

1. 不要把正文文字烤进图片（用户搜不到、机器看不见）。
2. 不要硬编码字号（用 em / % / `font-size: 1.05em` 而不是 `15px`）。
3. 不要在 `body` 上设 line-height（KDP 文档明确反对）。
4. 不要用 `display: flex` / `grid` / `position: absolute` 承载正文（重排不稳定）。
5. 不要把弹注做成普通超链接（要用 `epub:type="footnote"` 和 `aria-describedby`）。

## 读完入门后去哪？

按兴趣分流：

- **字体策略**：[docs/plans/fonts-css-expansion-plan.md](../plans/fonts-css-expansion-plan.md)。
- **弹注 / 注释**：[docs/guides/duokan-footnote-fallback-fix.md](../guides/duokan-footnote-fallback-fix.md) + 看 `epub-popup-footnote-converter` skill。
- **图片混排**：[docs/guides/chapter-head-image.md](../guides/chapter-head-image.md)。
- **英文小说**：[docs/guides/english-fiction-layout.md](../guides/english-fiction-layout.md)。
- **文白对照 / 古典文本**：[docs/guides/classical-modern-layout.md](../guides/classical-modern-layout.md)。
- **合集 / 大部头**：[docs/guides/anthology-navigation.md](../guides/anthology-navigation.md)。
- **Kindle 兼容性专题**：`docs/experiments/` 里所有 `review-*-kindle-*.md`。
- **批量清洗**：[docs/pipeline/](../pipeline/) + [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)。
- **想 review 改前改后差异**：浏览器打开 [tools/epub-diff/index.html](../../tools/epub-diff/index.html)。
- **想贡献回本仓**：[CONTRIBUTING.md](../../CONTRIBUTING.md)。

## 推荐阅读顺序

文件名只是参考编号，实际推荐：

1. [01-first-epub.md](../getting-started/01-first-epub.md) — 5 分钟跑通示例
2. [02-anatomy.md](../getting-started/02-anatomy.md) — 理解 epub 是什么
3. [03-readers.md](../getting-started/03-readers.md) — 知道要测哪些 reader
4. [06-test-your-own.md](../getting-started/06-test-your-own.md) — 测自己的 epub
5. [04-skills.md](../getting-started/04-skills.md) — 了解 AI 协作能力
6. [07-faq.md](../getting-started/07-faq.md) — 卡住了再翻
7. [05-case-study.md](../getting-started/05-case-study.md) — 看自造 EPUB 清洗演示案例
8. [glossary.md](../getting-started/glossary.md) — 随时查术语
````

**验收**：

```sh
grep -q "速查：一定要做" docs/getting-started/README.md
grep -q "读完入门后去哪" docs/getting-started/README.md
grep -q "推荐阅读顺序" docs/getting-started/README.md
```

---

#### S1-T12：新建根 `CONTRIBUTING.md`

**目的**：贡献者（新人 / AI / 老手）需要知道怎么 fork → 改 → PR。当前 `CLAUDE.md` 是给 AI 的契约，不是给人的贡献流程。

**输入**：无。
**输出**：仓库根 `CONTRIBUTING.md`。
**时间估算**：2 小时。

**完整骨架**（drop-in）：

````markdown
# 贡献指南

谢谢想给本仓贡献内容。

## 你可以贡献什么

- **阅读器实测**：在某个 reader / 字号 / profile 下打开本仓 fixture，把结果写进 `docs/final/reader-matrix.yaml`。
- **fixture / 场景**：在 `templates/epub-style-demo/` 添加新场景，覆盖之前没测过的排版情况。
- **bug 修复**：fixture 改得更稳、scripts 修小问题。
- **skill 改进**：`skills/*/SKILL.md` 内容修订（保持 frontmatter 字段名不变）。
- **文档补充**：guides / experiments / 入门。
- **公版书清洗 demo**：按 `samples/third-party/` 规范添加新样本。

## 你**不要**贡献什么

- 受版权保护的 epub（哪怕「能下载到」）。
- 你不能合法分发的字体。
- 不带实测的 reader 兼容性「主张」。
- 改 `docs/final/` 但不补 fixture / reader-matrix。

## 流程

1. **Fork + clone**：
   ```sh
   git clone <your fork URL>
   cd epub-handbook
   ```

2. **建分支**：
   ```sh
   git checkout -b feat/your-topic
   ```

3. **修改**：遵守 [CLAUDE.md](../../CLAUDE.md) 的「修改优先级」。如果你改 `docs/final/`，必须先有 demo fixture + reader 实测支撑。

4. **跑校验**：
   ```sh
   bash templates/epub-style-demo/build.sh
   NEW=$(ls -t templates/epub-style-demo/dist/ | head -1)
   bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/"$NEW"
   bash scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/"$NEW"
   python3 scripts/validate_skills_basic.py
   ```

5. **commit**：
   - 使用 [conventional commits](https://www.conventionalcommits.org/) 风格：`feat:` / `fix:` / `docs:` / `chore:`。
   - 一个 commit 一件事，不大杂烩。
   - commit message 描述「为什么改」而不只是「改了什么」。

6. **PR**：
   - 标题简洁（< 70 字符）。
   - body 说明：动机、改动范围、是否影响 reader-matrix、是否需要新实测。
   - 如果改了 fixture，附上一份新构建的 EPUB（或在 PR 描述里给 build 命令）。

## reader-matrix 回写规范

每条 expectation 必须包含：

```yaml
- reader: <reader_id>           # 见文件顶部 readers 段
  case: <case_id>               # 见文件顶部 cases 段
  status: pass | warn | fail | na
  reader_version: <真实版本号 or "pending-*">
  artifact: <对应的 dist epub 路径>
  issue: <一句话现象>
  action: <你做了什么>
  workaround: <临时回避方法（如有）>
```

不允许在没有实测的情况下写 `pass`。**没测过就写 `warn` + `pending-<reader>-version`**。

## 提 issue 时

附上：

1. 你的环境（OS / Python 版本 / browser）。
2. 复现命令（完整命令行）。
3. 完整错误输出（前后各 10 行 context）。
4. 你期望的行为。

## 行为规范

技术讨论保持就事论事；不歧视；不发广告。

## 许可

提 PR 即视为同意你的贡献按本仓许可证（代码 MIT、文档参照 [THIRD_PARTY.md](../../THIRD_PARTY.md)）发布。
````

**验收**：

```sh
test -f CONTRIBUTING.md
grep -q "reader-matrix" CONTRIBUTING.md
grep -q "conventional commits" CONTRIBUTING.md
```

### 2.4 Stage 1 全量验收清单

- [ ] 根 README ≤ 100 行，三条路径分流清楚 + diff 工具入口可见。
- [ ] `docs/getting-started/` 至少 7 篇齐全（`01–07` + `glossary.md`；`05-case-study.md` 占位）。
- [ ] `docs/getting-started/06-test-your-own.md` 落地，含 7 步流程。
- [ ] `docs/getting-started/07-faq.md` 落地，含 ≥ 10 个 Q&A。
- [ ] `docs/getting-started/glossary.md` 落地，覆盖 EPUB / Kindle / 中文排版核心术语。
- [ ] `04-skills.md` 含反向查表，覆盖 ≥ 14 个现有 skill。
- [ ] `03-readers.md` 含「我该测哪个」决策树。
- [ ] `docs/getting-started/README.md` 含 do/don't 速查 + 「读完入门去哪」 + 推荐阅读顺序。
- [ ] `CONTRIBUTING.md` 落地。
- [ ] `docs/pipeline/README.md` 占位完成，引用了 diff 工具与 `validate_text_invariance.py`。
- [ ] `docs/README.md` 目录索引齐全（含新增页）。
- [ ] `CLAUDE.md` 修改优先级同步。
- [ ] `python3 scripts/validate_skills_basic.py` 退出码 0。
- [ ] `bash scripts/validate-epub-style-demo.sh --epub <最新 dist .epub>` 退出码 0。
- [ ] `markdownlint-cli2 'docs/**/*.md' 'CONTRIBUTING.md' 'README.md'` 通过（按本仓 `.markdownlint-cli2.jsonc` 配置）。

---

## §3 Stage 2：已有 EPUB 清洗成为核心

### 3.1 目标

把「拿到一本已存在的 epub，让 AI 收拾干净并保留改动可追溯」做成一等流程。

**关键约束**：AI 改 epub 时必须有明确「改动边界」。当前 SPEC 没有这套规则。

**与 Stage 3 web app 的边界**：

- Stage 2 负责**机器可执行的红线 gate**（`validate_text_invariance.py`），用于 CI / pre-commit / AI 自检。
- Stage 3 web app 负责**人工可视化 review**，由人在浏览器里打开看。
- 两者**互不依赖**：CI 不调用 web app，web app 不调用 Python。

### 3.2 任务清单

#### S2-T1：新增 SPEC §10「AI 清洗已有 EPUB 的改动边界」

**输入**：当前 `docs/final/SPEC-实现约束.md`。
**输出**：在末尾追加 §10。
**时间估算**：3–4 小时。

**完整 SPEC §10 草稿**（drop-in）：

```markdown
## §10 AI 清洗已有 EPUB 的改动边界

> 本节给 AI 协作代理使用：当输入是一本已存在的 EPUB（而不是从零构造）时，AI 的改动必须落在本节边界内。
> 任何破坏本节约束的改动都视为事故，需要回滚。

### §10.1 红线（绝对不可改）

AI 检测到自己将要触发以下任一改动时，**必须停止并 ask 用户**：

| 红线 | 说明 | 校验方式（自动） |
| --- | --- | --- |
| 正文文本 | 去除标签后的纯文本不允许变化；标点、错别字、通假字一律不动 | `scripts/validate_text_invariance.py before.epub after.epub` |
| `dc:title` / `dc:creator` / `dc:identifier` / `dc:language` | OPF 核心元数据 | 同上脚本（v0.2 起覆盖） |
| spine 阅读顺序 | `<itemref>` 序列不可重排 | 同上 |
| 章节锚点 id | 影响第三方书签、旧链接、阅读器进度 | 同上 |
| 带 `properties="cover-image"` 的封面资源 | 不擅自压缩、转格式、裁切 | 同上 |
| DRM 相关 | 发现 `META-INF/encryption.xml` 或文件无法解压：立即拒绝 | 同上 |

红线触发后的处理：
1. AI 在输出里**明确列出**将要触发的红线条目。
2. 让用户决定：（a）放弃；（b）显式授权（用户必须打出「我授权」三字）；（c）调整范围。
3. **默认行为不得是「自动通过」**。

### §10.2 黄线（默认可改，但人工 review 必须看见）

AI 可自动执行；review 时**通过 `tools/epub-diff/` web app** 可视化确认：

| 黄线 | 说明 |
| --- | --- |
| 类名、内联样式 → 外联 CSS 的迁移 | 不改语义、只移位 |
| manifest `properties` 推断（svg / mathml） | 按文件内容推断 |
| nav.xhtml 结构调整 | 锚点 id 保持，只动结构 |
| 字体策略 | 添加 / 删除 `@font-face`，不替换正文文字 |
| 图片格式转换 | PNG ↔ JPEG，不改尺寸、不裁切 |
| CSS selector 合并 / 拆分 | 渲染效果不变 |
| 非封面资源添加 | 如新增 nav.xhtml |

### §10.3 绿线（可自由改，不必单独通告）

| 绿线 | 说明 |
| --- | --- |
| CSS 缩进 / 注释 / 排序 | 纯格式化 |
| 内部 `div` / `span` wrapper 精简 | 不改 class / id |
| 显式删除已被 grep 确认无引用的死代码 | CSS 孤儿类、孤儿 XHTML |
| zip 压缩等级调整 | 不改文件内容 |
| `xml:space` / 空白处理 | 不改语义文本 |

### §10.4 元规则

- **改动可见性**：任何改动都必须在 web app 中可见；不允许「秘密改动」。
- **校验时机**：每次 AI 改动后立刻跑 `validate_text_invariance.py`，红线触发立即回滚。
- **DRM 检测**：处理前先尝试 `unzip -l`，失败或发现 `encryption.xml` 立刻停止。
- **来源记录**：清洗操作必须有 `notes.md` 记录改了什么、为什么、用哪个 skill。
- **可回滚**：清洗前 epub 保留为 `before/` 备份；不允许就地覆盖。

### §10.5 自动化 gate（CI / pre-commit / AI 自检）

| 检测项 | 命令 | 通过条件 |
| --- | --- | --- |
| 文本红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub` | 退出码 0 |
| DRM 检测 | 同上脚本内置 | 不输出 "DRM detected" |
| 核心 metadata 红线 | 同上（v0.2 起覆盖） | 退出码 0 |
| spine 红线 | 同上（v0.2 起覆盖） | 退出码 0 |
| 封面红线 | 同上（v0.2 起覆盖） | 退出码 0 |

人工可视化 review 通过 `tools/epub-diff/index.html` 完成（不在自动化范畴）。
```

**验收**：

```sh
grep -n "§10" "docs/final/SPEC-实现约束.md"
grep -n "validate_text_invariance" "docs/final/SPEC-实现约束.md"
grep -n "tools/epub-diff" "docs/final/SPEC-实现约束.md"
```

---

#### S2-T2：新增 `scripts/validate_text_invariance.py`

**输入**：两份 epub。
**输出**：退出码 + stderr 差异报告。
**时间估算**：1–1.5 天（含 v0.2 红线扩展）。

**CLI 接口**：

```sh
python3 scripts/validate_text_invariance.py before.epub after.epub [OPTIONS]

OPTIONS:
  --allow-list PATTERN  允许某些文件文本变化
  --output FILE         差异报告写到 FILE（默认 stderr）
  --verbose             打印每个文件的逐段对比
  --redlines all|text   v0.2：检查范围。默认 text；all 覆盖 §10.5 所有项

退出码：
  0  所有红线通过
  1  红线触发
  2  输入错误（文件不存在、不是合法 epub、DRM）
```

**v0.1 范围（Stage 2 必做）**：只覆盖文本红线。
**v0.2 范围（Stage 4 之前补齐）**：扩到 metadata / spine / 封面 / DRM。

**实现要点（伪代码）**：

```python
import hashlib
import re
import zipfile
from pathlib import Path
from lxml import etree

XHTML_NS = "{http://www.w3.org/1999/xhtml}"
BLOCK_TAGS = {f"{XHTML_NS}{t}" for t in ("p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "td", "blockquote", "pre")}

def extract_text_blocks(xhtml_content: bytes) -> list[str]:
    root = etree.fromstring(xhtml_content, parser=etree.XMLParser(recover=True))
    blocks = []
    for elem in root.iter():
        if elem.tag in BLOCK_TAGS:
            text = " ".join(elem.itertext())
            text = re.sub(r"\s+", " ", text).strip()
            if text:
                blocks.append(text)
    return blocks

def block_hashes(blocks: list[str]) -> list[str]:
    return [hashlib.sha256(b.encode("utf-8")).hexdigest() for b in blocks]

def compare_xhtmls(before_zf, after_zf):
    before_xhtmls = {n for n in before_zf.namelist() if n.endswith(".xhtml")}
    after_xhtmls = {n for n in after_zf.namelist() if n.endswith(".xhtml")}
    diffs = []
    for name in sorted(before_xhtmls | after_xhtmls):
        if name not in before_xhtmls or name not in after_xhtmls:
            diffs.append({"path": name, "kind": "added" if name in after_xhtmls else "deleted"})
            continue
        b_hashes = block_hashes(extract_text_blocks(before_zf.read(name)))
        a_hashes = block_hashes(extract_text_blocks(after_zf.read(name)))
        if b_hashes != a_hashes:
            diffs.append({"path": name, "kind": "modified", "before": b_hashes, "after": a_hashes})
    return diffs
```

**测试用例**（必跑，写在 `scripts/test_validate_text_invariance.py`）：

| 用例 | 输入 | 期望 |
| --- | --- | --- |
| TC1 | epub vs 自己 | 退出 0 |
| TC2 | 改一个段落一个字 | 退出 1，定位文件 + 段索引 |
| TC3 | 删一个章节 | 退出 1 |
| TC4 | 增空白段（纯排版） | 退出 0（归一化） |
| TC5 | `<p>` 改 `<div>` 文字不变 | 退出 0 |
| TC6 | 改 CSS / 样式 | 退出 0 |
| TC7 | 非 zip 文件 | 退出 2 |
| TC8 | 带 DRM 的 epub | 退出 2 |
| TC9 | 22 个 XHTML 的 demo epub | < 5 秒 |
| TC10（v0.2）| 改 dc:title | 退出 1 |
| TC11（v0.2）| spine 顺序变 | 退出 1 |

**验收命令**：

```sh
python3 scripts/validate_text_invariance.py \
  templates/epub-style-demo/dist/<latest>.epub \
  templates/epub-style-demo/dist/<latest>.epub
# 退出码 0

python3 scripts/test_validate_text_invariance.py
# 所有测试用例通过
```

**陷阱**：

- 不要用「整 XHTML hash」（CSS 改也会算文本变化）。
- 段落归一化要保留实体（`&nbsp;` / 标点）。

---

#### S2-T3：新增 `docs/pipeline/cleanup-flow.md`

**输入**：无。
**输出**：guide。
**时间估算**：3–4 小时。

**完整骨架**（drop-in）：

````markdown
# EPUB 清洗流水线

> 状态：流程文档；用于把一本已存在的 EPUB 收拾干净。
> 对应 SPEC：[§10 AI 改动边界](../final/SPEC-实现约束.md)。
> 对应工具：`scripts/epub_ai_harness.py`、`scripts/validate_text_invariance.py`、`tools/epub-diff/index.html`。

## 整体流程

```
0. 准备 → 1. 输入判断 → 2. 红线预检 → 3. harness 扫描 →
4. 分派清洗 → 5. 文本校验 → 6. diff 人工 review → 7. 用户确认 → 8. reader-matrix 回写
```

## 0. 准备

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub
```

**不要原地覆盖**原始 epub。

## 1. 输入判断

```sh
unzip -l work/before/source.epub | head
unzip -p work/before/source.epub META-INF/encryption.xml 2>/dev/null
# 输出非空 = 有 DRM，停止
```

## 2. 红线预检

参照 [SPEC §10.1](../final/SPEC-实现约束.md) 列出本次清洗会触发哪些红线。

## 3. harness 扫描

```sh
python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub > work/findings.json
cat work/findings.json | jq .recommended_skills
```

## 4. 分派清洗

按 `recommended_skills` 顺序逐一执行。每次改动后跑：

```sh
python3 scripts/validate_text_invariance.py work/before/source.epub work/after/cleaned.epub
```

退出码 1 立即回滚该次 skill 改动。

## 5. 文本校验（自动 gate）

```sh
python3 scripts/validate_text_invariance.py work/before/source.epub work/after/cleaned.epub --redlines all
# 退出码必须 0
```

## 6. Diff 人工 review

**打开 diff 工具**：

```
双击 tools/epub-diff/index.html
或：把 tools/epub-diff/index.html 拖入 Chrome / Safari / Firefox
```

在工具里：
- 第一个文件拾取框选 `work/before/source.epub`。
- 第二个选 `work/after/cleaned.epub`。
- 点 **Compare**。
- 按结构 / 文本 / 样式 / 资源 / 元数据五层逐层 review。

工具的详细使用见 [diff-tool.md](../pipeline/diff-tool.md)。

> 这一步**只看文件差异**，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。

## 7. 用户确认

把 diff 报告（浏览器截图 / 导出 JSON）发给用户。用户确认后，`work/after/cleaned.epub` 作为交付。

## 8. reader-matrix 回写

如果清洗涉及阅读器兼容性变更，在 `docs/final/reader-matrix.yaml` 增条目，初始 status 一律 warn。

## 常见情况

### 输入 epub 有大量内联样式
→ `epub-css-layering-optimizer`。

### 输入 epub 的弹注不规范
→ `epub-popup-footnote-converter`。

### 输入 epub 缺中文字体策略
→ `epub-typography-optimizer`。

### 输入 epub 是从 PDF / OCR 转的
→ 不在本流程；用 `epub-source-intake`。
````

**验收**：guide 链接全部指向真实路径或本计划其它 Stage 中明确会落地；流程 8 步与 SPEC §10 对齐；明确指出第 6 步「打开 web app」。

---

#### S2-T4：新建 `samples/third-party/` 占位

**输入**：无。
**输出**：目录 + README + .gitignore。
**时间估算**：30 分钟。

**目录结构**：

```text
samples/third-party/
├── README.md
└── .gitignore
```

**`README.md` 内容**（drop-in）：

````markdown
# 第三方 EPUB 样本

本目录收纳真实公版 epub 作为清洗 fixture。

## 收录规则

- 只接受**公版书**或明确许可（CC0 / CC-BY / GFDL / Project Gutenberg）。
- 每本书一个子目录：`samples/third-party/<slug>/`。
- 子目录必须有：
  - `LICENSE.txt`：写明公版状态 + 原始许可。
  - `metadata.yaml`：来源 URL、抓取日期、SHA-256。
  - `fetch.sh`：下载 epub 并校验哈希。
  - `notes.md`：清洗过程记录、diff 关键发现。
- 实体 .epub 文件**不入 git**（被 `.gitignore` 忽略）。

## 现有样本

- 暂无。Stage 4 首轮改用 `samples/demo-books/` 自造 EPUB。

## 复现样本

```sh
cd samples/third-party/<slug>/
bash fetch.sh
```

## 同步 THIRD_PARTY.md

每新增一本样本，必须同步更新仓库根的 [THIRD_PARTY.md](../../THIRD_PARTY.md)。

````

**`.gitignore`**：

```text

*.epub
before/
after/
work/

```

**验收**：

```sh
test -d samples/third-party
test -f samples/third-party/README.md
test -f samples/third-party/.gitignore
```

---

#### S2-T5：`skills/epub-layout-auditor/SKILL.md` 同步

**输入**：当前 SKILL.md。
**输出**：在「输入判断」段加一句。
**时间估算**：10 分钟。

**新增句子**：

```markdown
- 若用户场景是「**改造已有 EPUB**」（而非「从零做新书」），本 skill 是入口；
  按 [SPEC §10](../../docs/final/SPEC-实现约束.md) 的红 / 黄 / 绿规则决定改动范围；
  每次改动跑 `scripts/validate_text_invariance.py` 红线 gate，红线触发立即回滚。
  人工 review 通过 `tools/epub-diff/index.html` 完成（用户在浏览器中可视化）。
```

**验收**：`grep -n "§10\|validate_text_invariance\|tools/epub-diff" skills/epub-layout-auditor/SKILL.md` 至少 3 行。

---

#### S2-T6：`scripts/epub_ai_harness.py` 加 `--mode`

**输入**：现有 harness。
**输出**：新增 `--mode {build,cleanup}`，影响 `recommended_skills` 顺序。
**时间估算**：2–3 小时。

**改动要点**：

- 默认 `--mode build`（兼容旧行为）。
- `--mode cleanup` 时：`epub-layout-auditor` 排首位；`epub-source-intake` 排除（输入是 EPUB）。
- 输出 JSON 增 `mode` 字段。

**验收**：

```sh
python3 scripts/epub_ai_harness.py --mode cleanup templates/epub-style-demo | jq .mode
# "cleanup"
```

---

#### S2-T7：SPEC §10 增 §10.6「能做 / 不能做」清单

**目的**：给用户清晰边界，「我这本 epub 适不适合走清洗流水线」。当前 §10 说改动边界，但没说**具体能修哪些类型的问题**。

**输入**：S2-T1 已落地的 SPEC §10。
**输出**：在 §10 末尾新增 §10.6。
**时间估算**：2 小时。

**完整新增节**（drop-in）：

````markdown
### §10.6 能力清单（What this pipeline can / can't do）

#### 能做（按依赖的 skill 分类）

| 问题模式 | 主路径 skill | 自动化程度 |
| --- | --- | --- |
| 大量内联 `style="..."` → 抽到外联 CSS | `epub-css-layering-optimizer` | 高 |
| 标准 footnote 缺 `epub:type`、缺 `aria-describedby` → 补齐 | `epub-popup-footnote-converter` | 高 |
| 多看 / 旧版阅读器需要弹注 fallback → 添加 fallback 结构 | `epub-legacy-footnote-fallback` | 高 |
| OPF manifest 缺 `properties="svg" / "mathml"` → 内容探测后推断填补 | `epub-package-nav-auditor` | 高 |
| nav.xhtml 缺失 / 结构破损 → 从 spine 重建 | `epub-package-nav-auditor` | 中 |
| toc.ncx 与 nav.xhtml 不同步 → 同步 | `epub-package-nav-auditor` | 高 |
| 字体策略不规范（@font-face 缺失 / 重复 / 引用断链）→ 重建 fonts.css | `epub-typography-optimizer` | 中 |
| 中英混排排版不稳（缺 `xml:lang` / 字号 / 行距）→ 规范化 | `epub-typography-optimizer` | 高 |
| 英文小说首字下沉 / 字体策略 → 规范化 | `epub-english-typography-optimizer` | 中 |
| 图文环绕用 `position: absolute` / `display: flex` → 改为 `figure.img-left/right` + float | `epub-image-layout-optimizer` | 中 |
| 章节首图 / 海报页结构混乱 → 规范化为 `.chapter-head-*` / poster 类 | `epub-image-layout-optimizer` | 中 |
| Ruby 注音不规范 → 规范化 `<ruby><rb><rt>` | `epub-vertical-ruby-optimizer` | 高 |
| Kindle Enhanced Typesetting 转换失败 → 识别并改写不兼容的 CSS | `epub-kindle-compatibility-checker` | 中 |
| 文学结构（出处 / 弹注 / 题献 / 题记）混在一起 → 分层 | `epub-literary-structure-formatter` | 中 |
| 普通 epub 加 A-lite 增强（海报 / 字体策略 / 竖排支持） | `epub-alite-converter` | 中 |

#### 不能做（明确不在范围）

| 问题 | 为什么不做 | 用户该怎么办 |
| --- | --- | --- |
| 修文字错误 / 通假字 / 错字 | SPEC §10.1 红线：正文文本不可变 | 回到源头校对 |
| 多语言翻译 / 译文生成 | 工具不做内容生成 | 找译者 |
| OCR 错误（重 OCR 一次） | 不在「清洗」范围 | 用 `epub-source-intake` 重做 |
| 去图片水印 / 删 DRM | 法律风险 | 找原版授权 |
| 加批注 / 书签 / 高亮 | 不是制作方关心的范畴 | 用阅读器自带功能 |
| 强制改 dc:identifier / dc:title | SPEC §10.1 红线 | 重新申请 ISBN / 自定义 UUID |
| 重排 spine（章节顺序） | SPEC §10.1 红线 | 在 source 阶段决定 |
| 把扫描书的图片 OCR 出来 | 应该走 intake 链路 | 用 `epub-source-intake` |
| 把固定版式（FXL）改为重排 | 内容信息丢失（版式承载语义） | 重新制作 |
| 视觉效果验收（在 reader 里好不好看） | 单独的 `reader-matrix` 流程负责 | 跑 `reader-matrix.yaml` 流程 |

#### 适配性判断（要不要走清洗流水线）

跑 `python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub`，看 findings：

- 找到的问题**多在「能做」清单**：✅ 适合走清洗流水线。
- 找到的问题**多在「不能做」清单**：❌ 不要走，回到源头。
- 一半一半：先做能做的部分，剩下的另开方案。
````

**验收**：

```sh
grep -q "§10.6" docs/final/SPEC-实现约束.md
grep -q "能做 / 不能做" docs/final/SPEC-实现约束.md
```

---

#### S2-T8：新建 `docs/pipeline/cleanup-patterns.md`「典型脏 epub 模式目录」

**目的**：把「拿到一本 epub，怎么识别是哪种问题模式 + 推荐 skill 顺序」变成可查表的指南，是 `epub-layout-auditor` 决策的具体细化。

**输入**：无。
**输出**：`docs/pipeline/cleanup-patterns.md`。
**时间估算**：4–5 小时（核心是模式整理）。

**完整骨架**（drop-in）：

````markdown
# 典型脏 EPUB 模式目录

> 状态：模式 + 推荐 skill 顺序；用作 `epub-layout-auditor` 决策的具体落地参考。
> 对应 SPEC：[§10 能力清单](../final/SPEC-实现约束.md)。

## 怎么用本目录

1. 跑 `python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub`。
2. 对照本目录的「特征」找匹配模式。
3. 按对应「推荐 skill 顺序」执行。
4. 每步后跑 `validate_text_invariance.py` + diff 工具确认。

> 一本 epub 常常同时落入多个模式。建议**优先按红线 → 黄线顺序执行**：先做不会破坏文本的清洗，再做 yellow 类。

## 模式 A：网上下载的扫描书

**特征**：
- 章节几乎全是 `<img>` 引用，文字段落极少。
- 文件里有少量「OCR 噪点」文本（不连贯字符、错断行）。
- 字体策略复杂、缺 ToC。
- 整本体积大（图片占 80%+）。

**判断**：这种 epub 不是「清洗」的范畴；OCR 是另一条链路。

**推荐做法**：
1. 不走本流水线。
2. 用 `epub-source-intake`：把图片 + 残留文本作为 source，重新 OCR → 重做章节 → 出新 epub。
3. 如果 OCR 质量已经可用，再回到本流水线做 §10.1 红线之外的清洗。

## 模式 B：出版社旧版 EPUB 2 → EPUB 3 升级

**特征**：
- OPF 里有 `<spine toc="ncx" />`（EPUB 2 风格）。
- 缺 `nav.xhtml`（只有 `toc.ncx`）。
- 大量 `<p style="...">` 内联样式。
- 标准 footnote 用 `<a href="#fn1">` 但没有 `epub:type="noteref"` / `epub:type="footnote"`。
- 字体直接引用 `font-family: "Songti SC"` 等系统字体，没 `@font-face`。

**推荐 skill 顺序**：
1. `epub-package-nav-auditor` — 重建 nav.xhtml + 同步 toc.ncx。
2. `epub-css-layering-optimizer` — 把内联样式抽到外联 CSS。
3. `epub-popup-footnote-converter` — 给 footnote 加 `epub:type` + `aria-describedby`。
4. `epub-typography-optimizer` — 字体策略规范化、行距、缩进。
5. （可选）`epub-legacy-footnote-fallback` — 兼容多看。

## 模式 C：Fan-made / 自制 EPUB

**特征**：
- 用 Sigil 编辑过：章节 id 是 `sigil_toc_id_N` 风格。
- 字体引用断链（CSS `@font-face` src 找不到对应文件，或字体不在 manifest）。
- 章节锚点不稳（同一章节多个 id、空标题）。
- 部分章节缺 properties 标记。

**推荐 skill 顺序**：
1. `epub-package-nav-auditor` — 修 manifest properties + 锚点 id。
2. `epub-typography-optimizer` — 字体策略修复（断链字体处理）。
3. `epub-css-layering-optimizer` — 拆出 CSS 层。
4. `epub-image-layout-optimizer` —（如有图）图文环绕规范化。

## 模式 D：自己旧作品 / 早期模板

**特征**：
- 用本仓早期模板做的（class 名不规范、缺 SCENE_MATRIX 对应字段）。
- OPF manifest 缺 properties 推断（svg / mathml）。
- 没用 A-lite 增强类。

**推荐 skill 顺序**：
1. `epub-package-nav-auditor` — 补 properties。
2. `epub-layout-auditor` — 跑一次审稿对照新 SPEC。
3. （可选）`epub-alite-converter` — 升级到 A-lite。

## 模式 E：技术书 / 教科书

**特征**：
- 大量 MathML / 公式。
- 多列表格。
- 章节内有大量代码块。
- 索引、术语表、注脚混在一起。

**推荐 skill 顺序**：
1. `epub-package-nav-auditor` — 确认 `properties="mathml"` 声明。
2. `epub-literary-structure-formatter` — 索引 / 术语表结构化。
3. `epub-popup-footnote-converter` — 注脚规范化。
4. `epub-css-layering-optimizer` — 代码块 / 表格样式分层。
5. `epub-kindle-compatibility-checker` — MathML 在 Kindle 上的兼容性专项。

## 模式 F：合集 / 大部头古籍

**特征**：
- 几百到几千个章节 XHTML。
- nav.xhtml 极深（卷 → 篇 → 条目级）。
- 含 Ruby 注音（生僻字）。
- 文白对照 / 多列布局。

**推荐 skill 顺序**：
1. `epub-package-nav-auditor` — 验证条目级锚点 + 同步 nav / ncx。
2. `epub-vertical-ruby-optimizer` — Ruby + 竖排（如有）。
3. `epub-literary-structure-formatter` — 文白对照（参 `classical-modern-layout.md`）。
4. `epub-typography-optimizer` — 古籍字体策略。

## 模式 G：英文 / 双语 epub

**特征**：
- 大段英文正文 + 偶尔中文。
- 章节首字下沉。
- 引文 / extract 大量。

**推荐 skill 顺序**：
1. `epub-english-typography-optimizer` — 英文专项排版。
2. `epub-typography-optimizer` — 中文混排细节。
3. `epub-popup-footnote-converter` — 译注规范化。
4. `epub-image-layout-optimizer` —（如有插图）规范化。

## 模式 H：「看上去没问题」的 epub

**特征**：
- epubcheck 通过、各 reader 都能开。
- 但 `epub-layout-auditor` 报警告：CSS 分层不清、字体策略略乱。

**推荐做法**：

不必清洗。绿线问题不值得动。

## 通用建议

- **永远不要并行执行多个 skill**：每步后跑 `validate_text_invariance.py` 单独确认。
- **遇到红线立即停**，不要寄希望「下一步会修回来」。
- **结果不满意可回滚**：每步产出 `after/step-N.epub`，回退到 step-K 即可。
````

**验收**：

```sh
test -f docs/pipeline/cleanup-patterns.md
grep -cE "^## 模式 [A-Z]：" docs/pipeline/cleanup-patterns.md   # 期望 ≥ 7 个模式
```

---

#### S2-T9：扩展 `scripts/validate_text_invariance.py` 到 v0.2

**目的**：把 v0.2 范围（metadata / spine / cover / DRM 红线）**在 Stage 2 一次性做完**，否则「CI gate 守住所有红线」是不成立的承诺。原 S2-T2 写「v0.2 可延后到 Stage 4 前」，本任务把这部分提前。

**输入**：S2-T2 已落地的 v0.1（仅文本红线）。
**输出**：v0.2 扩展 + 测试用例补全。
**时间估算**：1.5 天。

**新 CLI 接口**：

```sh
python3 scripts/validate_text_invariance.py before.epub after.epub [OPTIONS]

OPTIONS:
  --check {text,metadata,spine,cover,drm,all}  # 默认 all
  --allow-list PATTERN
  --output FILE
  --verbose

退出码：
  0  所有指定的红线 gate 通过
  1  任一红线触发
  2  输入错误
```

**新增校验逻辑**：

1. **metadata 红线**：解析两边 OPF 的 `<metadata>`，对比 `dc:title` / `dc:creator` / `dc:identifier` / `dc:language`。任一变化退出 1。
2. **spine 红线**：解析两边 OPF 的 `<spine>`，逐项对比 `idref` 序列；顺序或元素变化退出 1。
3. **cover 红线**：找带 `properties="cover-image"` 的 manifest item，对比两边 sha256；不一致退出 1。
4. **DRM 检测**：检查 `META-INF/encryption.xml` 是否存在；存在即退出 2 + 提示「DRM detected, refusing to process」。

**新增测试用例**（追加到 `scripts/test_validate_text_invariance.py`）：

| 用例 | 输入 | 期望 |
| --- | --- | --- |
| TC12 | 改 dc:title | 退出 1，输出指明 metadata 红线 |
| TC13 | 改 dc:identifier | 退出 1 |
| TC14 | spine 顺序交换两章 | 退出 1，输出指明 spine 红线 |
| TC15 | spine 删一章 | 退出 1 |
| TC16 | cover 图替换（不同 sha256） | 退出 1 |
| TC17 | 改 dc:subject（非核心 metadata） | 退出 0（不是红线） |
| TC18 | 带 encryption.xml | 退出 2，输出 "DRM detected" |
| TC19 | 只测 `--check text`（其他红线即便触发也忽略） | 仅按 text 范围判断 |

**验收**：

```sh
python3 scripts/test_validate_text_invariance.py
# 期望所有 19 个测试用例通过

python3 scripts/validate_text_invariance.py templates/epub-style-demo/dist/<latest>.epub templates/epub-style-demo/dist/<latest>.epub --check all
# 期望退出码 0
```

**陷阱**：

- DRM 用 encryption.xml 的存在性判断，不解析其内容（解析有版权风险）。
- cover-image 必须按 `properties` 找，不是按文件名（Cover.png 等约定不可靠）。

---

#### S2-T10：扩展 `docs/pipeline/cleanup-flow.md`，加 7 个新章节

**目的**：cleanup-flow 当前只有 8 步流程骨架。这次补全：健康检查（合并到第 1 步）、批量模式、回滚剧本、可信度评估、错误恢复、OCR 识别、标准 notes 模板。

**输入**：S2-T3 已落地的 cleanup-flow.md 骨架。
**输出**：在原 guide 中改写 §1 + 新增 §9–§14。
**时间估算**：1 天。

**改动 1（重写第 1 步）**：把原「输入判断」扩展为「健康检查」：

````markdown
## 1. 健康检查

任何清洗前必须通过的最低门槛。任一项失败立即停止。

```sh
EPUB=work/before/source.epub

# 1.1 zip 健康
unzip -t "$EPUB" >/dev/null && echo "zip OK" || { echo "zip broken"; exit 1; }

# 1.2 第一个 entry 必须是 mimetype，且不压缩
python3 -c "
import zipfile, sys
with zipfile.ZipFile('$EPUB') as z:
    first = z.infolist()[0]
    assert first.filename == 'mimetype', 'first entry must be mimetype'
    assert first.compress_type == zipfile.ZIP_STORED, 'mimetype must be stored'
    assert z.read('mimetype') == b'application/epub+zip', 'mimetype content invalid'
print('mimetype OK')
"

# 1.3 container.xml 合法
unzip -p "$EPUB" META-INF/container.xml | head -5 >/dev/null && echo "container.xml OK" || { echo "container.xml missing"; exit 1; }

# 1.4 OPF 可解析（由 epub_ai_harness 包含）

# 1.5 DRM 检测
unzip -p "$EPUB" META-INF/encryption.xml 2>/dev/null
# 输出非空 = 有 DRM，立即停止

# 1.6 epubcheck 跑一次（W3C 官方）
which epubcheck >/dev/null && epubcheck "$EPUB" || echo "epubcheck not installed; skip"
```

任一步失败：

- zip 损坏 → 重新获取源文件，本流水线不修复物理损坏。
- mimetype 不在第一个 / 被压缩 → 用 `templates/epub-style-demo/build.sh` 的打包方式重打。
- container.xml 缺失 / 坏 → 不是合法 EPUB；建议先到 `epub-source-intake` 修。
- DRM 检测到 → **拒绝处理**，参 SPEC §10.1。
- epubcheck fatal → 不修就别清洗，先解决 fatal。
````

**改动 2（新增第 9–14 节，追加到 §8 之后）**：

````markdown
## 9. 批量模式

一次处理多本 epub 时：

```sh
# 用 GNU parallel 并发，每本一个工作目录
ls /path/to/books/*.epub | parallel -j 4 ./clean-one.sh {}
```

其中 `clean-one.sh` 是把本流程 8 步封装的脚本（每本独立 work/ 子目录）。

**失败重试**：parallel 默认遇错继续。失败的会写到 `failed.log`，跑完后重试：

```sh
cat failed.log | parallel -j 4 ./clean-one.sh {}
```

**报告聚合**：

```sh
# 汇总每本的 notes.md 关键字段
for d in work/*/; do
  echo "## $(basename $d)"
  jq -r '.summary' "$d/diff-summary.json"
done > batch-report.md
```

**建议**：单批次不超过 50 本；超过的话分批，方便人工 review。

## 10. 回滚剧本

清洗中每步产出带时间戳的中间 epub：

```text
work/after/
├── step-1-css-layering.epub
├── step-2-popup-footnote.epub
├── step-3-typography.epub
└── cleaned.epub   # = 最后一步的硬链接 / 复制
```

要回滚到 step-K：

```sh
cp work/after/step-K-*.epub work/after/cleaned.epub
```

然后从 step-(K+1) 开始重新跑后续 skill。

**不要直接修改中间 epub**；中间 epub 是回滚锚点，要保留。

**清理**：清洗完成且用户验收后，可删除 step-*.epub，只留 `cleaned.epub` + `notes.md`。

## 11. 可信度评估（done well 判定）

跑完整流程后，工具应给一个简单评分供用户决定是否接受：

| 指标 | 来源 | 期望 |
| --- | --- | --- |
| 红线触发数 | `validate_text_invariance.py --check all` | **必须 0** |
| 黄线条数 | epub-diff 报告统计 | 任意；记录 |
| 人工 review 必要项 | 黄线中归类为「必须人工 review」的子集 | 记录数量 |
| epubcheck error 数（after） | `epubcheck` | ≤ 输入 epub 的 error 数（不引入新错误） |
| 阅读器兼容性回归 | reader-matrix 复测 | 不变差 |

**自动结论**：

- 红线 0 + 必须 review 项 0 + epubcheck 不变差 → **自动通过**。
- 红线 0 + 有必须 review 项 → **人工 review**。
- 红线 > 0 → **重做**。

把这个判定结果写进 `notes.md`。

## 12. 错误恢复

如果清洗中途 AI 卡住 / API 超时 / 网络断：

**状态文件**：每完成一步写入 `work/state.json`：

```json
{
  "started_at": "2026-05-26T10:00:00Z",
  "input_sha256": "abc123...",
  "completed_steps": [
    {"skill": "epub-css-layering-optimizer", "output": "after/step-1-css-layering.epub", "completed_at": "..."},
    {"skill": "epub-popup-footnote-converter", "output": "after/step-2-popup-footnote.epub", "completed_at": "..."}
  ],
  "next_step": "epub-typography-optimizer"
}
```

**恢复**：再次启动时用 `--resume`：

```sh
./clean-one.sh --resume work/
```

工具读 state.json，跳过已完成的步骤，从 `next_step` 继续。

## 13. OCR-style 脏 epub 识别

特征：

- 章节几乎全是 `<img>` 引用，少量散乱文本。
- 文本里有大量「OCR 噪点」（异常单字符 / 错断行）。
- 文件名常带 `scan` / `ocr` / `_p001` 类。

判定（自动）：

```sh
python3 scripts/epub_ai_harness.py work/before/source.epub | jq '.findings[] | select(.kind == "ocr-residual")'
```

如果检测到，输出建议：

```text
This EPUB appears to be OCR-derived. Cleanup is unlikely to help.
Recommended path: run epub-source-intake to redo OCR, then come back.
```

不强制阻止用户继续，但给出明确警告。

## 14. 标准 `notes.md` 模板

每本清洗的 epub 在工作目录留下 `notes.md`，结构统一如下。**所有 demo 与生产清洗都用同一模板**，不漂移。

````md
# 清洗记录：<书名>

> 日期：<DATE>
> 操作者：<NAME>
> 输入 SHA-256：<sha>
> 输出 SHA-256：<sha>

## 0. 健康检查

- zip：OK
- mimetype：OK
- container.xml：OK
- DRM：无
- epubcheck：N error / N warning

## 1. harness findings

主要 findings：
- ...

## 2. 模式判定

匹配模式（参 [cleanup-patterns.md](../../docs/pipeline/cleanup-patterns.md)）：模式 B。

## 3. 清洗步骤

### Step 1: <skill name>
- dry-run 输出：附 `step-1.dry-run.json`
- 文本红线：pass
- 中间产物：`after/step-1-css-layering.epub`

### Step 2: <skill name>
...

## 4. 完整红线校验

```sh
python3 scripts/validate_text_invariance.py before/source.epub after/cleaned.epub --check all
```
退出码：0 ✅

## 5. Diff 概览

用 `tools/epub-diff/index.html` 看：

- 结构：unchanged
- 文本：identical
- 样式：12 selector 改动（新增 5 / 删除 2 / 修改 5）
- 资源：2 字体删除
- 元数据：core unchanged

## 6. 可信度评估

- 红线触发数：0
- 必须 review 项：0
- epubcheck 变化：error 12 → 0
- 结论：自动通过 ✅

## 7. 已知未解决

- 无 / <issue>
````

**验收**：

```sh
grep -q "## 9. 批量模式" docs/pipeline/cleanup-flow.md
grep -q "## 10. 回滚剧本" docs/pipeline/cleanup-flow.md
grep -q "## 11. 可信度评估" docs/pipeline/cleanup-flow.md
grep -q "## 12. 错误恢复" docs/pipeline/cleanup-flow.md
grep -q "## 13. OCR-style 脏 epub 识别" docs/pipeline/cleanup-flow.md
grep -q "## 14. 标准" docs/pipeline/cleanup-flow.md
grep -q "epubcheck" docs/pipeline/cleanup-flow.md
```

---

#### S2-T11：所有清洗 skill 加 `--dry-run` 约定

**目的**：AI 在真正改 epub 前必须能预览改动。当前 skill 直接写盘，事故难回滚。

**输入**：14 个现有 `skills/*/SKILL.md`。
**输出**：每个 SKILL.md 的「工作流」段补充 `--dry-run` 行为约定。
**时间估算**：1 天（覆盖 14 个 skill）。

**约定**（所有清洗类 skill 必须遵守）：

1. **默认行为是 dry-run**：不写盘，只输出预期改动到 stdout / JSON。
2. **`--commit` 才实际改**：用户审查 dry-run 输出后，加 `--commit` 重跑才生效。
3. **dry-run JSON 输出结构统一**：

   ```json
   {
     "skill": "<skill-id>",
     "input": "work/before/source.epub",
     "expected_output": "work/after/step-N.<skill-id>.epub",
     "changes": [
       {"layer": "structure", "kind": "add", "path": "OEBPS/nav.xhtml", "reason": "missing nav.xhtml"},
       {"layer": "style",     "kind": "modify", "selector": "body", "before_props": {...}, "after_props": {...}},
       {"layer": "resources", "kind": "delete", "path": "OEBPS/Fonts/abc.otf", "reason": "no @font-face reference"}
     ],
     "estimated_redline_risk": "none | low | medium | high"
   }
   ```

4. **改前自检**：dry-run 阶段就应该跑 `validate_text_invariance.py` 模拟，如果预测红线触发：在 changes 里标记 `redline_risk: high`，并在输出里加 `WARN: this change will trigger SPEC §10.1 red line, abort.`。

**SKILL.md 同步**：每个清洗 skill 末尾加一节：

````markdown
## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

调用示例：

```sh
# 预览
<skill-invocation> work/before/source.epub > work/dry-run.json

# 审查
cat work/dry-run.json | jq

# 确认后执行
<skill-invocation> --commit work/before/source.epub
```

dry-run 输出格式见 [docs/pipeline/cleanup-flow.md §11](../../docs/pipeline/cleanup-flow.md)。
````

**适用的 skill**（14 个中清洗相关）：

- `epub-css-layering-optimizer`
- `epub-popup-footnote-converter`
- `epub-legacy-footnote-fallback`
- `epub-vertical-ruby-optimizer`
- `epub-typography-optimizer`
- `epub-english-typography-optimizer`
- `epub-image-layout-optimizer`
- `epub-literary-structure-formatter`
- `epub-kindle-compatibility-checker`
- `epub-alite-converter`
- `epub-package-nav-auditor`

**不适用**（不写盘的审稿 / 维护类）：

- `epub-layout-auditor` — 只读，不写盘
- `epub-source-intake` — 输入不是 epub
- `epub-style-demo-maintainer` — 内部 fixture 维护

**验收**：

```sh
for s in $(ls skills/ | grep -vE "^README|^epub-layout-auditor|^epub-source-intake|^epub-style-demo-maintainer"); do
  grep -q "Dry-run 约定" "skills/$s/SKILL.md" || echo "MISSING: $s"
done
# 输出应为空
```

---

#### S2-T12：新建 `docs/pipeline/skills-matrix.md`「skill × 流程步骤」映射表

**目的**：14 个 skill 各自在清洗流水线哪一步用、是不是清洗专用、和新书制作怎么共用，一张表说清。

**输入**：无。
**输出**：`docs/pipeline/skills-matrix.md`。
**时间估算**：2 小时。

**完整骨架**（drop-in）：

````markdown
# Skills × 流程步骤映射表

> 14 个现有 skill 在「清洗流水线」与「新书制作」中的角色。

## 总表

| Skill | 清洗 | 新书 | 用在哪一步 | 类型 |
| --- | --- | --- | --- | --- |
| `epub-layout-auditor` | ✅ 入口 | ✅ | 清洗 §2 分派；新书 review 前 | 审稿 |
| `epub-source-intake` | ❌（不在 scope） | ✅ | 新书：txt/md/PDF/OCR → source | 接入 |
| `epub-css-layering-optimizer` | ✅ | ✅ | 清洗 §4 黄线；新书 finalize | 清洗 / 制作 |
| `epub-popup-footnote-converter` | ✅ | ✅ | 清洗 §4 黄线；新书弹注 | 清洗 / 制作 |
| `epub-legacy-footnote-fallback` | ✅ | ✅ | 清洗 §4；新书做多看兼容 | 清洗 / 制作 |
| `epub-typography-optimizer` | ✅ | ✅ | 清洗 §4；新书排版细化 | 清洗 / 制作 |
| `epub-english-typography-optimizer` | ✅ | ✅ | 清洗 §4（双语 epub）；新书英文体 | 清洗 / 制作 |
| `epub-image-layout-optimizer` | ✅ | ✅ | 清洗 §4；新书图文 | 清洗 / 制作 |
| `epub-vertical-ruby-optimizer` | ✅ | ✅ | 清洗 §4（古籍 / 日文）；新书竖排 | 清洗 / 制作 |
| `epub-literary-structure-formatter` | ✅ | ✅ | 清洗 §4；新书文白 / 章首 | 清洗 / 制作 |
| `epub-kindle-compatibility-checker` | ✅ | ✅ | 清洗 §4；新书 Kindle 专项 | 清洗 / 制作 |
| `epub-alite-converter` | ⚠（看场景） | ✅ | 新书：普通 epub → A-lite | 制作 |
| `epub-package-nav-auditor` | ✅ | ✅ | 清洗 §4；新书 OPF/nav 校验 | 清洗 / 制作 |
| `epub-style-demo-maintainer` | ❌ | ❌（仅维护本仓） | 本仓 fixture 维护 | 仓库内部 |

## 清洗专用（不适合新书制作直接用）

无。所有清洗 skill 都可在新书制作中复用；区别只在「输入是脏 epub」vs「输入是干净 source」。

## 新书制作专用（清洗不用）

- `epub-source-intake`：清洗的输入就是 EPUB，不需要 intake。
- `epub-alite-converter`：清洗的目标一般不是「升级到 A-lite」，但也不禁止。

## 清洗流水线中 skill 的典型顺序

按 [cleanup-patterns.md](../pipeline/cleanup-patterns.md) 的「模式 B（出版社旧版）」举例：

1. `epub-layout-auditor`（入口审稿，推荐顺序）
2. `epub-package-nav-auditor`（修 OPF / nav 基础）
3. `epub-css-layering-optimizer`（拆 CSS）
4. `epub-popup-footnote-converter`（弹注规范化）
5. `epub-typography-optimizer`（字体 + 排版）
6. （可选）`epub-legacy-footnote-fallback`（多看兼容）
7. `epub-kindle-compatibility-checker`（Kindle 专项）

每两个 skill 之间跑 `validate_text_invariance.py` + dry-run 审查。
````

**验收**：

```sh
test -f docs/pipeline/skills-matrix.md
grep -cE "^\| \`epub-" docs/pipeline/skills-matrix.md   # 期望 14 行 skill
```

---

#### S2-T13：新建 `samples/fixtures-tiny/`「微型 epub 测试 fixture」

**目的**：`validate_text_invariance.py` 和 `epub_diff` 工具的测试，目前都跑完整 demo epub（22+ XHTML）。需要更小、更精准、覆盖边界情况的 fixture。

**输入**：无。
**输出**：`samples/fixtures-tiny/` 目录 + 一组最小 fixture。
**时间估算**：4–5 小时。

**目录结构**：

```text
samples/fixtures-tiny/
├── README.md
├── empty-paragraphs/         # 空 <p> 段
│   ├── source/
│   └── build.sh
├── ruby-only/                # 仅含 <ruby>
├── mathml-only/              # 仅含 <math>
├── multi-lang/               # 中英混排 + xml:lang
├── one-image-one-text/       # 模拟扫描书边界
├── nested-tables/            # 表格嵌套
└── drm-marker/               # 含假的 encryption.xml（仅用于 DRM 检测测试）
```

**每个 fixture 子目录约定**：

- `source/` 是解压状态的 epub（mimetype + META-INF + OEBPS）。
- `build.sh` 一行：把 source 打成 `.epub` 输出到 `dist/`。
- 每本 fixture < 5KB（除非内容必须大）。
- 内容是合法、可解析的最小 EPUB 3。

**`README.md` 内容**（drop-in）：

````markdown
# 微型测试 fixture

为 `scripts/validate_text_invariance.py`、`scripts/test_epub_diff.py`（如有）、
以及 `tools/epub-diff/` 手动验证用。

每个子目录是一个最小但合法的 EPUB 3，覆盖一个或一组特定边界情况。

## 列表

| 名称 | 覆盖的边界 |
| --- | --- |
| `empty-paragraphs/` | XHTML 含空 `<p>` 段；归一化逻辑测试 |
| `ruby-only/` | Ruby 注音；文本提取必须忽略 `<rt>` |
| `mathml-only/` | MathML 命名空间；OPF properties 自动推断 |
| `multi-lang/` | 中英混排 + `xml:lang`；段落级 hash 必须把语言变化也算上 |
| `one-image-one-text/` | 模拟扫描书边界；OCR-style 检测的正样本 |
| `nested-tables/` | 表格嵌套；XHTML 解析的鲁棒性 |
| `drm-marker/` | 含 `META-INF/encryption.xml`（fake）；DRM 检测必须立即拒 |

## 用法

```sh
cd samples/fixtures-tiny/empty-paragraphs/
bash build.sh
ls dist/  # → empty-paragraphs.epub
```

```sh
# 测试 validate_text_invariance.py 的归一化
python3 ../../../scripts/validate_text_invariance.py \
  samples/fixtures-tiny/empty-paragraphs/dist/empty-paragraphs.epub \
  samples/fixtures-tiny/empty-paragraphs/dist/empty-paragraphs.epub
# 期望退出码 0
```

## 不入 git 的部分

- `dist/` 下的 .epub 文件（gitignore）。
- 每次 build 重新生成；source 是真相之源。

## 添加新 fixture

1. 选一个未覆盖的边界。
2. 建子目录 + source/ + build.sh。
3. 在本 README 加一行说明。
4. 更新 `scripts/test_validate_text_invariance.py` 引用它。
````

**验收**：

```sh
test -d samples/fixtures-tiny
test -f samples/fixtures-tiny/README.md
for f in empty-paragraphs ruby-only mathml-only multi-lang one-image-one-text nested-tables drm-marker; do
  test -d "samples/fixtures-tiny/$f" || echo "MISSING: $f"
done
# 输出应为空
```

---

#### S2-T14：新建 `docs/pipeline/asset-optimization.md`（图片与字体优化）

**目的**：清洗流水线遇到「WebP 资源 / 大图 / 未压缩 PNG / 未子集化字体」时怎么办，全用**现有广泛使用的工具**（oxipng / mozjpeg / webp / fonttools / glyphhanger），不写新脚本。

**输入**：无。
**输出**：`docs/pipeline/asset-optimization.md`。
**时间估算**：4–5 小时（核心是命令整理 + 验证表）。

**完整 drop-in 内容**：

`````markdown
# 资源优化：图片与字体

> 状态：操作指南；用现有工具，不写新脚本。
> 对应清洗步骤：[cleanup-flow.md](../pipeline/cleanup-flow.md) §4。
> 对应 SPEC：[§10.2 黄线](../final/SPEC-实现约束.md)（资源转换、字体策略）+ [§10.1 红线](../final/SPEC-实现约束.md)（封面图不可改）。

## 1. 适用范围

**做什么**：

- 把 WebP / AVIF 等非 EPUB 3 core media type 转出到 PNG / JPEG。
- 对 PNG 做无损压缩。
- 对 JPEG 做无损优化（重压缩 + progressive）。
- 字体子集化（中文字体的常见需求）。

**不做什么**：

- **不动 `properties="cover-image"`**（SPEC §10.1 红线）。
- 不改图片内容（裁剪 / 翻转 / 加水印 / 改色调 / 超分）。
- 不引入新打包脚本；全部走系统包管理 + 直接命令。
- 不改图片尺寸（清洗保持视觉一致）。
- 不强制 AVIF（Kindle 不支持）。

## 2. 工具栈

### 图片

| 用途 | 工具 | 安装（macOS） | 安装（Linux） |
| --- | --- | --- | --- |
| PNG 无损压缩 | [oxipng](https://github.com/shssoichiro/oxipng) | `brew install oxipng` | `cargo install oxipng` |
| JPEG 无损优化 | mozjpeg / `jpegtran` | `brew install mozjpeg` | `apt install libjpeg-turbo-progs` |
| WebP 编解码 | `cwebp` / `dwebp` | `brew install webp` | `apt install webp` |
| 通用转换 / dimensions | ImageMagick (`magick`) | `brew install imagemagick` | `apt install imagemagick` |
| 元数据查看 | `exiftool` | `brew install exiftool` | `apt install libimage-exiftool-perl` |

### 字体

| 用途 | 工具 | 安装 |
| --- | --- | --- |
| 子集化 | `pyftsubset`（[fonttools](https://github.com/fonttools/fonttools)） | `pip install fonttools brotli zopfli` |
| 字形扫描 | [glyphhanger](https://github.com/zachleat/glyphhanger) | `npm install -g glyphhanger` |
| WOFF2 压缩 | [google/woff2](https://github.com/google/woff2) `woff2_compress` | `brew install woff2` |
| 字体表检查 | `ttx`（fonttools 自带） | 同 fonttools |

> 这些都是行业内广泛使用的工具，不需要自研。

### GUI 工具（给不想用命令行的人）

如果你不熟命令行，下表是**真无损 + 拖拽 GUI**的替代品。本指南主流程仍以命令行为准（批量 / 可重现 / 能写进 `notes.md` / 可进 CI），但 GUI 适合：

- 一次性 / 一两张图的小任务。
- 拿不准结果时先目视确认。
- 新人入门、还没准备好装命令行工具。

**图片（无损）**：

| 工具 | 平台 | 形态 | 说明 |
| --- | --- | --- | --- |
| [ImageOptim](https://imageoptim.com/) | macOS | 拖拽 app | 公认最好用；多引擎（PNGOUT / OptiPNG / Zopfli / mozjpeg / SVGO 全打包）；**默认无损**；免费开源 |
| [Zipic](https://zipic.app/) | macOS（Windows 即将） | 拖拽 app | 国产；支持 12 种格式（含 WebP / HEIC / AVIF / JXL / APNG）；**支持格式转换**；本地处理不上传；免费 25 张/天 / 一次买断 $29.99 |
| [Squoosh](https://squoosh.app/) | Web | 浏览器内 | Google 出品；纯客户端，文件不上传；支持 PNG / JPEG / WebP / AVIF；可选 OxiPNG / MozJPEG 等编码器 + 无损模式 |
| [Caesium Image Compressor](https://caesium.app/) | macOS / Windows / Linux | 拖拽 app | 免费开源；批量；勾选「Lossless」即走无损路径 |
| [FileOptimizer](https://nikkhokkho.sourceforge.io/static.php?page=FileOptimizer) | Windows | 拖拽 app | 免费；多引擎（PNGOUT / DeflOpt / advpng / mozjpeg）；同时支持图片、字体、PDF |
| [PNGGauntlet](https://pnggauntlet.com/) | Windows | 拖拽 app | 仅 PNG；用 PNGOUT + OptiPNG + DeflOpt 三个引擎跑一遍 |

> Zipic 的默认压缩等级是「视觉无损」（近无损有损），用于 EPUB 出版时建议在设置里把**压缩等级调到最低 / 关闭质量损失**，保留原始像素；并且每次先对一张样本做前后 hash 对比确认。如果是封面或重要插图，仍推荐用 ImageOptim / Squoosh 走严格无损路径。

**看似无损实则有损（不推荐用于 EPUB 出版）**：

| 工具 | 为什么不推荐 |
| --- | --- |
| TinyPNG / TinyJPG | 「智能有损」量化；视觉看不出但像素已变（出版语境保留原像素更稳） |
| JPEGmini | 重压缩 JPEG，技术上有损 |
| Optimage | 默认有损模式；若用，记得切到「Lossless only」 |

**字体子集化（GUI / Web）**：

| 工具 | 形态 | 说明 |
| --- | --- | --- |
| [Transfonter](https://transfonter.org/) | Web | 上传字体 → 选字符集 / 自定义文本 → 一键输出 WOFF2 + CSS；最易上手 |
| [FontSquirrel Webfont Generator](https://www.fontsquirrel.com/tools/webfont-generator) | Web | 老牌；支持子集化 + 多格式输出（WOFF / WOFF2 / TTF / SVG fonts） |
| [Glyphs Mini](https://glyphsapp.com/buy) | macOS（付费） | 专业字体编辑器；可手工挑字符做子集；适合需要精细控制时 |

> ⚠ **版权提醒**：Transfonter / FontSquirrel 是 web 工具，会把字体上传到第三方服务器。商业授权字体 / 客户私有字体**不要上传**；用本地 `pyftsubset` 或 `glyphhanger`。

## 3. 图片处理

### 3.1 EPUB 兼容性回顾

- EPUB 3 core media types：**JPEG / PNG / GIF / SVG**。
- **WebP / AVIF 不是 core**：Kindle KFX 不支持；Apple Books 通过 WebKit 能开但不是 spec 路径。
- 结论：清洗时遇到 WebP / AVIF，**必须转出**到 PNG / JPEG。

### 3.2 WebP → PNG / JPEG

```sh
# 判断 WebP 是否含透明
dwebp -bench input.webp 2>&1 | grep -q "Has alpha" && echo "alpha=yes" || echo "alpha=no"
```

```sh
# 有透明 → PNG（无损）
dwebp input.webp -o output.png

# 无透明 → JPEG（更小）
dwebp input.webp -o /tmp/tmp.png
magick /tmp/tmp.png -quality 85 -interlace Plane -strip output.jpg
rm /tmp/tmp.png
```

**批量**（按透明度自动选目标格式）：

```sh
find OEBPS/Images -name "*.webp" | while read f; do
  out_base="${f%.webp}"
  if dwebp -bench "$f" 2>&1 | grep -q "Has alpha"; then
    dwebp "$f" -o "${out_base}.png"
    echo "→ ${out_base}.png (alpha)"
  else
    dwebp "$f" -o /tmp/$$.png
    magick /tmp/$$.png -quality 85 -interlace Plane -strip "${out_base}.jpg"
    rm /tmp/$$.png
    echo "→ ${out_base}.jpg"
  fi
  rm "$f"
done
```

**转换后必做**：

1. 更新 `OEBPS/package.opf` 中 `<item>` 的 `href` 和 `media-type`。
2. 更新所有 `OEBPS/Text/*.xhtml` 与 `OEBPS/Styles/*.css` 中的 `src` / `url()` 引用。
3. 跑 epubcheck 确认没有断链。

```sh
# 找出还引用 .webp 的位置
grep -rn "\.webp" OEBPS/ | grep -v Binary
```

### 3.3 PNG 无损压缩

```sh
# 单文件
oxipng -o max --strip safe OEBPS/Images/figure-1.png

# 整目录批量（原地覆盖；事先在 work/before 已有备份）
find OEBPS/Images -name "*.png" -exec oxipng -o max --strip safe {} +
```

参数说明：

- `-o max`：最高优化等级（慢但效果最好）。
- `--strip safe`：删元数据但保留 sRGB / gamma 等关键 chunk，**不影响显示**。
- 不要用 `--strip all`：可能破坏 colorspace 信息。
- 不要用有损模式（oxipng 默认就是无损）。

平均压缩 20–40%。

### 3.4 JPEG 无损优化

```sh
# 单文件
jpegtran -copy none -optimize -progressive OEBPS/Images/photo.jpg > /tmp/o.jpg && mv /tmp/o.jpg OEBPS/Images/photo.jpg

# 批量
find OEBPS/Images \( -name "*.jpg" -o -name "*.jpeg" \) | while read f; do
  jpegtran -copy none -optimize -progressive "$f" > "$f.opt" && mv "$f.opt" "$f"
done
```

参数说明：

- `-copy none`：去除 EXIF / 缩略图等元数据。
- `-optimize`：Huffman 表优化。
- `-progressive`：交错编码（小屏渐进加载体感好）。
- **无损**：JPEG 像素不变，只是重压缩。

压缩 5–25%（已经被 Photoshop 压过的 JPEG 收益小）。

### 3.5 GIF / SVG

- **静态 GIF**：转 PNG 后跑 oxipng（更小）：

  ```sh
  magick input.gif -strip output.png
  oxipng -o max --strip safe output.png
  ```

- **动态 GIF**：保留，EPUB 3 允许。
- **SVG**：不在本指南范围（不需要无损压缩；如需 minify，用 [svgo](https://github.com/svg/svgo)，但单独评估）。

### 3.6 决策树

```text
输入是 WebP？  → yes → 有 alpha？→ yes → 转 PNG → oxipng
                          ↓ no  → 转 JPEG（jpegtran -optimize -progressive）

输入是 PNG？   → yes → oxipng -o max --strip safe

输入是 JPEG？  → yes → jpegtran -copy none -optimize -progressive

输入是静态 GIF？→ yes → magick → PNG → oxipng

输入是动态 GIF？→ yes → 保留

输入是 SVG？   → yes → 保留（svgo 单独评估，不在主流程）

输入是 AVIF？  → yes → magick → PNG/JPEG（与 WebP 同路径）
```

### 3.7 红线：cover-image 不动

```sh
# 找出 cover-image，确认它不在你即将处理的列表里
python3 -c "
import xml.etree.ElementTree as ET
root = ET.parse('OEBPS/package.opf').getroot()
ns = {'opf': 'http://www.idpf.org/2007/opf'}
for item in root.findall('.//opf:item', ns):
    if 'cover-image' in (item.get('properties') or ''):
        print('COVER:', item.get('href'))
"
```

**这个文件路径在本指南的所有命令中必须 exclude**。如果原始封面格式是 WebP，**不要自动转**；让操作者人工决定是否改源，然后重新打包。

### 3.8 OPF manifest 同步

格式变更后：

```xml
<!-- before -->
<item id="img1" href="Images/figure-1.webp" media-type="image/webp"/>

<!-- after -->
<item id="img1" href="Images/figure-1.png" media-type="image/png"/>
```

可以用 sed 批量改 media-type 字符串，但建议用 Python 脚本配合 lxml 改 OPF，避免破坏 XML 格式。

参考 `epub-package-nav-auditor` skill 的 OPF 更新逻辑（不要自己重新发明）。

## 4. 字体处理

### 4.1 为什么要子集化

中文字体（思源宋体 / 黑体 / 楷体）单个 5–15MB；一本书实际用的汉字 < 3000。子集化后体积通常降到 **5–15%**，对总包大小影响巨大。

### 4.2 用 glyphhanger（推荐：自动扫描 + 子集化）

```sh
# 安装
npm install -g glyphhanger

# 一步搞定：扫所有 XHTML 提取实际用到的字符，子集化字体，输出 woff2
glyphhanger OEBPS/Text/*.xhtml \
  --formats=woff2 \
  --subset=OEBPS/Fonts/NotoSerifSC.otf
# 输出：NotoSerifSC-subset.woff2 在原字体同目录
```

glyphhanger 内部调用 fonttools，所以也要 `pip install fonttools brotli zopfli`。

### 4.3 用 pyftsubset（手工 / 更精细控制）

```sh
# 1) 扫所有 XHTML 提取唯一字符
find OEBPS -name "*.xhtml" -exec cat {} \; | \
  python3 -c "
import sys, re
text = sys.stdin.read()
# 去掉 XML 标签，只看正文字符
text = re.sub(r'<[^>]+>', '', text)
chars = {c for c in text if ord(c) >= 0x20 and ord(c) != 0x7F}
print(''.join(sorted(chars)))
" > /tmp/used-chars.txt

# 2) 转 unicode 列表
python3 -c "
with open('/tmp/used-chars.txt') as f:
    chars = f.read()
codes = sorted(set(ord(c) for c in chars))
print(','.join(f'U+{c:04X}' for c in codes))
" > /tmp/unicodes.txt

# 3) 子集化
pyftsubset OEBPS/Fonts/NotoSerifSC.otf \
  --unicodes-file=/tmp/unicodes.txt \
  --output-file=OEBPS/Fonts/NotoSerifSC-subset.woff2 \
  --flavor=woff2 \
  --no-hinting \
  --desubroutinize \
  --no-layout-closure
```

参数说明：

- `--flavor=woff2`：输出 WOFF2（最佳压缩）。
- `--no-hinting`：移除 hinting 表，进一步减小（屏幕显示不需要）。
- `--desubroutinize`：解 CFF 子例程，配合 WOFF2 压缩效果更好。
- `--no-layout-closure`：不展开 GSUB/GPOS 闭包，对中文一般不需要。

### 4.4 子集化后必做

1. **改 CSS**：

   ```css
   @font-face {
     font-family: "NotoSerifSC";   /* 保持原 family 名 */
     src: url(../Fonts/NotoSerifSC-subset.woff2) format("woff2");
   }
   ```

   也可以把 `font-family` 改名（`"NotoSerifSC-Book"`）避免和系统同名字体冲突，但要同步改所有引用。

2. **改 OPF manifest**：

   ```xml
   <item id="font-notoserif" href="Fonts/NotoSerifSC-subset.woff2" media-type="font/woff2"/>
   ```

3. **删原字体**：
   ```sh
   rm OEBPS/Fonts/NotoSerifSC.otf
   ```
   注意：先确认 OPF / CSS 不再引用它（`grep -r "NotoSerifSC.otf" OEBPS/`）。

4. **阅读器实测**：
   - 翻几页确认没有「豆腐字」（缺字符显示为方块）。
   - 用 Kindle Previewer + Apple Books 各看一次。

### 4.5 多字体子集化

如果有多个字体（正文宋 + 标题黑 + 楷体）：

```sh
for fname in NotoSerifSC NotoSansSC LXGWWenKai; do
  glyphhanger OEBPS/Text/*.xhtml \
    --formats=woff2 \
    --subset="OEBPS/Fonts/${fname}.otf"
done
```

每个字体单独扫描的代价：所有字体都装了完整字符集（因为我们用同一份 XHTML 扫描），所以三个 subset 字体的合集 = 一份完整 XHTML 字符。不冗余。

### 4.6 WOFF2 vs WOFF vs OTF/TTF

EPUB 3 支持：OTF / TTF / WOFF / WOFF2 / SVG fonts。

| 格式 | 大小（相对 OTF）| 阅读器兼容性 | 推荐 |
| --- | --- | --- | --- |
| WOFF2 | 30–50% | 所有现代 reader | ✅ 主路径 |
| WOFF | 70% | 旧版 / Kindle KFX 个别版本 | 旧 reader fallback |
| OTF / TTF | 100% | 所有 | 体积大不推荐 |
| SVG fonts | — | 已废弃 | ❌ 不用 |

OTF/TTF → WOFF2：

```sh
woff2_compress NotoSerifSC-subset.ttf
# 输出 NotoSerifSC-subset.woff2
```

### 4.7 用户字体覆盖的注意

Kindle / Apple Books 允许用户**关闭出版方字体**（Publisher Font）切到系统字体。这时：

- 子集化字体不显示，用系统字体兜底（这是正常行为）。
- 你的字体策略**不能依赖**子集化字体一定生效。
- `book-song` / `book-kai` 之类的工具类（参 `docs/final/SPEC-实现约束.md` §8）必须保留 family chain 中的系统字体兜底。

## 5. 在清洗流水线中的位置

参 [cleanup-flow.md](../pipeline/cleanup-flow.md)：

- **§1 健康检查**：列出 WebP / 巨大图片 / 大字体作为「黄线候选」。
- **§4 清洗执行**：
  - `epub-image-layout-optimizer` 调用本指南 §3 命令。
  - `epub-typography-optimizer` 调用本指南 §4 命令。
  - 每次操作走 `--dry-run` 预览 → `--commit` 写盘（参 S2-T11 dry-run 约定）。
- **§5 文本校验**：`validate_text_invariance.py --check all` 退出 0；图片转换不应触发文本红线。
- **§6 Diff 报告**：资源层会看到：
  - 字体文件 sha256 变化、可能 path 变化（otf → woff2）。
  - 图片文件 sha256 变化、format 可能变化。
  - **仅 modified 图片**生成缩略图（参 §4.4.4 资源层策略）。
- **§11 可信度评估**：检查 epubcheck 不引入新 error；阅读器实测无豆腐字。

## 6. 验证清单

清洗后跑：

```sh
# 1. 没有遗留 WebP / AVIF
! find OEBPS -name "*.webp" -o -name "*.avif" | grep .

# 2. 所有 PNG / JPEG 可被合法解析
find OEBPS -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" | \
  xargs -I{} sh -c 'identify "{}" >/dev/null 2>&1 || echo "BROKEN: {}"'

# 3. 字体可被 fonttools 解析
find OEBPS -name "*.woff2" -o -name "*.woff" -o -name "*.otf" -o -name "*.ttf" | \
  xargs -I{} python3 -c "
import sys
from fontTools.ttLib import TTFont
try: TTFont(sys.argv[1]).close()
except Exception as e: print(f'BROKEN: {sys.argv[1]}: {e}')
" {}

# 4. OPF manifest media-type 没漂移
grep -E "media-type=\"image/webp\"" OEBPS/package.opf && echo "WebP MIME still in OPF" || echo "OPF clean"

# 5. CSS @font-face / url() 引用都还存在
for f in $(find OEBPS -name "*.css"); do
  grep -oE "url\([^)]+\)" "$f" | sed 's/url(//;s/)//;s/"//g' | while read ref; do
    test -f "$(dirname $f)/$ref" || echo "MISSING ref: $f → $ref"
  done
done

# 6. epubcheck（如安装）
which epubcheck >/dev/null && epubcheck . || echo "epubcheck not installed; skip"
```

## 7. 不做的（避免范围蔓延）

- 不写 image 重排（裁剪 / 翻转）— 用户的图本身不动。
- 不做 AI 增强 / 超分辨率。
- 不做色彩管理 / ICC profile 转换；要求源是 sRGB。
- 不强制 AVIF（Kindle 不支持）。
- 不内置新 binary 工具；全部走 brew / apt / pip / npm。
- 不自动跑：操作者按本指南命令手动执行；AI skill 调用本指南的命令时必须 dry-run（参 S2-T11）。

## 8. 参考资料

### 命令行工具

- [oxipng](https://github.com/shssoichiro/oxipng) — Rust 写的 PNG 优化器，多线程
- [mozjpeg](https://github.com/mozilla/mozjpeg) — Mozilla 维护的 JPEG 编码器，含 `jpegtran`
- [Google WebP precompiled binaries](https://developers.google.com/speed/webp/docs/precompiled) — `cwebp` / `dwebp` 二进制
- [fonttools / pyftsubset](https://github.com/fonttools/fonttools) — Python 字体处理库
- [glyphhanger](https://github.com/zachleat/glyphhanger) — Node 工具，扫 HTML 提取实际用到的字符
- [google/woff2](https://github.com/google/woff2) — `woff2_compress`
- [ImageMagick](https://imagemagick.org/) — 通用图像处理（`magick` 命令）

### GUI 工具（无损图片压缩）

- [ImageOptim](https://imageoptim.com/) — macOS，拖拽即用，多引擎，免费开源
- [Zipic](https://zipic.app/) — macOS（Windows 即将），国产，12 种格式 + 格式转换，本地处理；免费 25 张/天 / 一次买断
- [Squoosh](https://squoosh.app/) — Google 出品的浏览器内压缩工具，纯客户端（文件不上传）
- [Caesium Image Compressor](https://caesium.app/) — 跨平台桌面 app，开源，支持 lossless 模式
- [FileOptimizer](https://nikkhokkho.sourceforge.io/static.php?page=FileOptimizer) — Windows，多格式 + 多引擎
- [PNGGauntlet](https://pnggauntlet.com/) — Windows，PNG 专用

### GUI / Web 工具（字体子集化）

- [Transfonter](https://transfonter.org/) — Web，上传字体 → 出 WOFF2 + CSS
- [FontSquirrel Webfont Generator](https://www.fontsquirrel.com/tools/webfont-generator) — Web，老牌字体子集化与格式转换

### 规范 / 平台

- [EPUB 3 Core Media Types](https://www.w3.org/publishing/epub32/epub-spec.html#sec-cmt-supported) — 哪些图片 / 字体格式是 spec 内的
- [KDP Image Requirements](https://kdp.amazon.com/en_US/help/topic/G201953020) — Amazon Kindle 对图片的要求
- [Apple Books Asset Guide](https://help.apple.com/itc/booksassetguide/) — Apple Books 资源规范
- [W3C EPUB 3.3 — Fonts](https://www.w3.org/TR/epub-33/#sec-fonts) — EPUB 字体支持

### 字体来源（OFL / SIL / 自由协议，可安全子集化和分发）

- [Google Fonts](https://fonts.google.com/) — 绝大多数 OFL，含 Noto 系列
- [Adobe Source Han Serif / Sans](https://github.com/adobe-fonts/source-han-serif) — 思源宋体 / 黑体，开源
- [霞鹜文楷（LXGWWenKai）](https://github.com/lxgw/LxgwWenKai) — 中文楷体，开源
- [GitHub - lxgw/LxgwWenKai-Screen](https://github.com/lxgw/LxgwWenKai-Screen) — 屏幕优化版

### 本仓内交叉引用

- [SPEC-实现约束.md §10](../final/SPEC-实现约束.md) — AI 改动边界，封面图红线
- [cleanup-flow.md](../pipeline/cleanup-flow.md) — 本工具调用所在的清洗流水线
- [cleanup-patterns.md](../pipeline/cleanup-patterns.md) — 典型脏 epub 模式，本指南是其中「资源层」处理的展开
- [fonts-css-expansion-plan.md](fonts-css-expansion-plan.md) — 字体策略全局规划
- [docs/pipeline/skills-matrix.md](../pipeline/skills-matrix.md) — `epub-image-layout-optimizer` / `epub-typography-optimizer` 在流程中的位置
`````

**验收**：

```sh
test -f docs/pipeline/asset-optimization.md
grep -q "oxipng" docs/pipeline/asset-optimization.md
grep -q "pyftsubset" docs/pipeline/asset-optimization.md
grep -q "cover-image" docs/pipeline/asset-optimization.md
grep -cE "^## " docs/pipeline/asset-optimization.md   # 期望 ≥ 8 个一级章节
markdownlint-cli2 docs/pipeline/asset-optimization.md
```

**陷阱**：

- **不要**在示例命令里覆盖原文件而没备份。整个流程已经在 `work/before/` 保留了原 epub，但中途如 `oxipng` 默认原地改，跑前确认 work/after/ 是个副本。
- **不要**误改 cover-image：脚本要排除 `properties="cover-image"` 引用的文件。
- **不要**子集化所有字体到同一份；不同字体（标题 vs 正文）的字符集不同时，子集化要分开做。
- pyftsubset 默认会保留 `name` 表里的字体名；如果你改了 `font-family` 名，记得 CSS 同步改。
- macOS 系统字体（如 Songti SC）**不要**子集化或重新分发；版权问题。子集化只对你**拥有授权**或**OFL/SIL** 等自由协议的字体做（如 Noto / Source Han / 霞鹜文楷）。

### 3.3 Stage 2 全量验收清单

- [ ] SPEC §10 完整（§10.1–§10.5 + 新增 §10.6 能力清单）。
- [ ] `validate_text_invariance.py` v0.2 落地，覆盖文本 / metadata / spine / cover / DRM 五条红线。
- [ ] `scripts/test_validate_text_invariance.py` 19 个测试用例通过。
- [ ] `docs/pipeline/cleanup-flow.md` 完整：§1 健康检查（含 epubcheck）+ §2–§8 主流程 + §9 批量 + §10 回滚 + §11 评估 + §12 恢复 + §13 OCR 识别 + §14 notes 模板。
- [ ] `docs/pipeline/cleanup-patterns.md` 落地，覆盖 ≥ 7 个典型模式。
- [ ] `docs/pipeline/skills-matrix.md` 落地，覆盖 14 个现有 skill。
- [ ] 14 个 SKILL.md 中清洗类 skill 都加了「Dry-run 约定」段。
- [ ] `samples/fixtures-tiny/` 落地，覆盖 7 个边界情况。
- [ ] `docs/pipeline/asset-optimization.md` 落地，覆盖图片转换 / 无损压缩 / 字体子集化；命令全用现有工具（oxipng / mozjpeg / webp / pyftsubset / glyphhanger）。
- [ ] `samples/third-party/` 占位（README + .gitignore）。
- [ ] `skills/epub-layout-auditor/SKILL.md` 同步（含模式目录引用）。
- [ ] `epub_ai_harness.py --mode cleanup` 跑通，含 OCR-style 检测输出。
- [ ] `python3 scripts/validate_skills_basic.py` 退出码 0。
- [ ] `markdownlint-cli2 'docs/**/*.md' 'CONTRIBUTING.md'` 通过。

---

## §4 Stage 3：EPUB Diff Web App（含 UI 设计）

### 4.1 v1 范围

**v1 形态**：

- 入口：`tools/epub-diff/index.html`。
- 用户操作：双击 / 拖入浏览器 → 选两个 epub → 点 Compare → 看 diff。
- **纯前端**（浏览器内运行）：zip.js 流式解压 + JavaScript 解析 + 内置 line diff 渲染。
- 完全离线：所有依赖本地化在 `tools/epub-diff/assets/`。

**v1 必做**：

- 文件拾取 UI（两个 `<input type="file">`）。
- 浏览器内流式解压 epub（zip.js + `Blob.stream()`，不一次性载入整包）。
- 五层 diff 算法（结构 / 文本 / 样式 / 资源 / 元数据），全部在 JS 里实现。
- 五层 UI 渲染（参 §4.2 mockup）。
- 暗色模式（跟随系统）。
- 移动端折叠（< 768px 切单栏）。

**v1 不做**：

- 不做 Python CLI（红线 gate 由 Stage 2 `validate_text_invariance.py` 独立覆盖）。
- 不做 AI 调用入口 / 报告生成 API。
- 不做阅读器启动按钮（不集成 Kindle Previewer / Apple Books）。
- 不做 epub 内嵌渲染（不是阅读器）。
- 不做 pixel-level 渲染对比 / 截图对比（不是效果验收）。
- 不做 conflict resolution。
- 不做后端 / 服务器。
- 不做桌面 app 打包。

### 4.2 UI 设计

#### 4.2.1 启动后第一眼：Landing Page

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│  📘 EPUB Diff                                              [⚙ Theme] [ℹ ?]  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                                                                             │
│                   选择两个 EPUB 文件来对比                                   │
│                                                                             │
│                                                                             │
│        ┌─────────────────────────────────────────────────┐                  │
│        │                                                 │                  │
│        │              📄 before.epub                     │                  │
│        │                                                 │                  │
│        │           [ Choose file ]  no file selected     │                  │
│        │                                                 │                  │
│        └─────────────────────────────────────────────────┘                  │
│                                                                             │
│        ┌─────────────────────────────────────────────────┐                  │
│        │                                                 │                  │
│        │              📄 after.epub                      │                  │
│        │                                                 │                  │
│        │           [ Choose file ]  no file selected     │                  │
│        │                                                 │                  │
│        └─────────────────────────────────────────────────┘                  │
│                                                                             │
│                                                                             │
│                   [   Compare  →   ]                                        │
│                   （ 两个文件都选了之后才可点 ）                              │
│                                                                             │
│                                                                             │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  This tool runs entirely in your browser. Your files never leave            │
│  your device. See README for details.                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

支持「拖拽文件到拾取框」作为快捷方式（HTML5 drag-and-drop API）。

#### 4.2.2 加载中（解压 + 解析阶段）

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│  📘 EPUB Diff                                              [⚙ Theme] [ℹ ?]  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                                                                             │
│                  正在解压和解析…                                             │
│                                                                             │
│                  ┌───────────────────────────────────┐                      │
│                  │ ██████████████░░░░░░░░░░  64%      │                      │
│                  └───────────────────────────────────┘                      │
│                                                                             │
│                  - before.epub: ✅ 解压完成 (12.4 MB)                         │
│                  - after.epub:  ⏳ 解析 OPF…                                  │
│                                                                             │
│                                                                             │
│                  [ Cancel ]                                                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 4.2.3 对比视图（主视图）

桌面端 ≥ 1024px：

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│  📘 EPUB Diff      [⇐ Back]                              [⚙] [⬇ JSON] [ℹ ?]│  ← 粘性 header
├─────────────────────────────────────────────────────────────────────────────┤
│  before:  book-v1.epub  (12.4 MB · 24 XHTML · 8 CSS · 47 images)            │
│  after:   book-v2.epub  (11.8 MB · 24 XHTML · 6 CSS · 47 images)            │
│  ✅ Identical core metadata · ✅ Identical text content · ✅ Identical spine│  ← 关键状态
├──────────────┬──────────────────────────────────────────────────────────────┤
│              │                                                              │
│  Layers      │  ⊞ Structure ──────────────────────────────────────────────  │
│              │     Manifest items: +0 / −0 / ~2                              │
│  Structure   │     Spine order:    unchanged                                 │
│    ~ 2       │     Nav structure:  unchanged                                 │
│              │     NCX:            format-only changes                       │
│  Text        │                                                              │
│    ✓ no change│  ⊞ Text ──────────────────────────────────────────────────   │
│              │     ✅ All 24 XHTML files have identical text content.        │
│  Style       │     (Hash-based segment comparison; details collapsed)        │
│    +5 −2 ~5  │                                                              │
│              │  ⊞ Style ──────────────────────────────────────────────────   │
│  Resources   │     Styles/base.css ────────────────────────────────  [⊞]    │
│    0 deleted │     ┌──────────────────────┬─────────────────────────┐       │
│    2 deleted │     │ before               │ after                   │       │
│  (fonts)     │     │ body {               │ body {                  │       │
│              │     │   font-size: 16px;   │   font-size: 1em;       │       │
│  Metadata    │     │   color: #333;       │ }                       │       │
│    ✓ core ok │     │ }                    │                         │       │
│              │     └──────────────────────┴─────────────────────────┘       │
│              │     (rendered by built-in line diff)                         │
│  ─────────── │                                                              │
│              │     Styles/literary.css ───────────────────────────  [⊞]    │
│  View        │                                                              │
│  ◉ Split     │  ⊞ Resources ─────────────────────────────────────────────   │
│  ○ Stacked   │     Deleted (2)                                              │
│              │       • Fonts/Source Han Sans CN.otf  (3.2 MB)               │
│  Filters     │       • Fonts/Source Han Serif CN.otf (4.1 MB)               │
│  ☐ Hide ✓    │                                                              │
│  ☐ Collapse  │  ⊞ Metadata ─────────────────────────────────────────────   │
│              │     Core (red-line):                                          │
│  Theme       │       dc:title       「呐喊」      「呐喊」      ✅           │
│  ◉ Light     │       dc:creator     「鲁迅」      「鲁迅」      ✅           │
│  ○ Dark      │       dc:identifier  urn:isbn:abc  urn:isbn:abc  ✅           │
│  ○ Auto      │       dc:language    zh-CN          zh-CN        ✅           │
│              │                                                              │
└──────────────┴──────────────────────────────────────────────────────────────┘
   200px sticky                                            主滚动区
```

> **注意**：v1 的状态条只标识「核心字段是否一致」，**不评估「这是否是合法清洗」**。任何「红线」「违规」判断由 Stage 2 `validate_text_invariance.py` 在 CI 中完成；web app 只如实展示差异，让人自己判断。

#### 4.2.4 启动页顶栏的状态描述方式

Stage 2 `validate_text_invariance.py` 已经给 CI 通过 / 不通过的口径。Web app 顶部只保留**事实描述**，不替 CI 下判断：

```text
✅ Identical core metadata · ✅ Identical text content · ✅ Identical spine
```

或：

```text
✏ Text content differs in 3 files · ✅ Core metadata identical · ✅ Spine identical
```

不用「RED LINE」「VIOLATION」等评价词，因为 web app 不做「这是不是合法清洗」的判断，只描述事实。CI 那边有 `validate_text_invariance.py` 给 0/1 答案。

#### 4.2.5 Style 层（核心 diff 渲染）

```text
⊞ Style ────────────────────────────────────────────────────────────────────
   Selector-level summary:
     Total CSS files:  8 → 6  (2 deleted)
     Total selectors:  142 → 128
       + Added:    8
       − Deleted:  22
       ~ Modified: 12
   ─────────────────────────────────────────────────────────────────────────
   Styles/base.css                                            [view raw]  ⊟
   ┌──────────────────────────────────────────────────────────────────────┐
   │ before                            │ after                            │
   ├──────────────────────────────────────────────────────────────────────┤
   │  1  body {                        │  1  body {                       │
   │  2    font-family: serif;         │  2    font-family: serif;        │
   │  3-   font-size: 16px;            │                                  │
   │  4-   color: #333;                │                                  │
   │  5  }                             │  3  }                            │
   │  6                                │  4                               │
   │  7  h1 {                          │  5  h1 {                         │
   │  8    font-size: 1.6em;           │  6    font-size: 1.6em;          │
   │  9  }                             │  7  }                            │
   │                                   │  8+ .new-utility {               │
   │                                   │  9+   margin: 0.5em;             │
   │                                   │ 10+ }                            │
   └──────────────────────────────────────────────────────────────────────┘
   ↑ built-in line diff renderer
   ─────────────────────────────────────────────────────────────────────────
   Styles/literary.css                                        [view raw]  ⊞
   Styles/effects.css ✅ (unchanged)                          [expand]
```

切到 Stacked 视图：

```text
   Styles/base.css                                            [view raw]  ⊟
   ┌──────────────────────────────────────────────────────────────────────┐
   │  1 1  body {                                                          │
   │  2 2    font-family: serif;                                           │
   │  3 -    font-size: 16px;                                              │
   │  4 -    color: #333;                                                  │
   │  5 3  }                                                               │
   │  6 4                                                                  │
   │  7 5  h1 {                                                            │
   │       ...                                                             │
   │  - 8+ .new-utility {                                                  │
   │  - 9+   margin: 0.5em;                                                │
   │  -10+ }                                                               │
   └──────────────────────────────────────────────────────────────────────┘
```

#### 4.2.6 Text 层

正常（无变化）：

```text
⊞ Text ────────────────────────────────────────────────────────────────
   ✅ All 24 XHTML files have identical text content.
   Hash method: SHA-256 over normalized block-level text
   ┌────────────────────────────────────────────────────────────────┐
   │  Text/00-cover.xhtml         sha256:a3f7d9c1...  ↔  same        │
   │  Text/01-intro.xhtml         sha256:b8e2c5d3...  ↔  same        │
   │  Text/05-chapter-3.xhtml     sha256:c1d5f8a2...  ↔  same        │
   │  ...                                                            │
   │  [Show all 24 files]                                            │
   └────────────────────────────────────────────────────────────────┘
```

有变化（事实描述，不评价）：

```text
⊞ Text ────────────────────────────────────────────────────────────────
   ✏ 3 XHTML files have different text content.

   Text/01-intro.xhtml
   ┌────────────────────────────────────────────────────────────────┐
   │  block index 7:                                                │
   │  before: 「客宿山亭，夜半闻钟。童子启户视之，月在松间…」          │
   │  after:  「客宿山亭，夜半闻钟。童子开门看去，月在松间…」          │
   │           inline diff rendered by built-in line diff             │
   └────────────────────────────────────────────────────────────────┘

   Text/05-chapter-3.xhtml
   ...
```

#### 4.2.7 Resources 层

```text
⊞ Resources ───────────────────────────────────────────────────────────
   Summary
     Images: 47 → 47   (0 added, 0 deleted, 0 modified)
     Fonts:   8 → 6    (2 deleted)
     Audio:   0 → 0
     Other:   0 → 0

   Deleted Fonts (2)
   ┌────────────────────────────────────────────────────────────────┐
   │  • Fonts/Source Han Sans CN.otf  (3.2 MB)                       │
   │    sha256: a3f7d9c1...                                          │
   │    Referenced by: Styles/fonts.css → @font-face                │
   │                                                                │
   │  • Fonts/Source Han Serif CN.otf (4.1 MB)                      │
   │    sha256: b8e2c5d3...                                          │
   │    Referenced by: Styles/fonts.css → @font-face                │
   └────────────────────────────────────────────────────────────────┘

   Modified Images (0)
   (none)

   Unchanged Images (47, collapsed)                       [expand]
```

**只有 `modified` 状态的图片**才展示并排缩略图（base64 + `<img>`），其它状态只列文件名 + 大小 + hash：

```text
   ~ Images/cover.png (1.2 MB → 850 KB, dimensions 1200×1800 unchanged)
   ┌────────────┐  ┌────────────┐
   │            │  │            │
   │   before   │  │   after    │
   │  (200×300) │  │  (200×300) │
   │            │  │            │
   └────────────┘  └────────────┘
   → likely re-encoded with higher compression
```

新增 / 删除 / 未变图片**不生成缩略图**，避免大书内存压力：

```text
   + Images/extra-chapter-cover.jpg (640 KB, sha256: c4f7…)
   - Images/old-illustration.png   (1.1 MB, sha256: 9d3a…)
```

#### 4.2.8 Metadata 层

```text
⊞ Metadata ────────────────────────────────────────────────────────────
   Core metadata:
   ┌────────────────────────────────────────────────────────────────┐
   │  field             before          after           status       │
   ├────────────────────────────────────────────────────────────────┤
   │  dc:title          「呐喊」         「呐喊」         ✅            │
   │  dc:creator        「鲁迅」         「鲁迅」         ✅            │
   │  dc:identifier     urn:isbn:abc    urn:isbn:abc    ✅            │
   │  dc:language       zh-CN           zh-CN           ✅            │
   └────────────────────────────────────────────────────────────────┘

   Other metadata:
   ┌────────────────────────────────────────────────────────────────┐
   │  dc:subject       「小说;现代文学」 「现代文学;小说」 (reordered) │
   │  dc:date          unchanged       unchanged                    │
   │  + meta:rendition  (added) "reflowable"                        │
   └────────────────────────────────────────────────────────────────┘
```

#### 4.2.9 暗色模式

按 `prefers-color-scheme: dark` 自动切；用户可在 sidebar 手动覆盖。

| 角色 | Light | Dark |
| --- | --- | --- |
| 背景 | `#f8f7f4` | `#1a1a1a` |
| 文字 | `#2d2d2d` | `#e0e0e0` |
| 状态边线 — 中性 | `#8f9099` | `#666` |
| 状态边线 — 差异 | `#1d70b8` | `#4a90d9` |
| ✅ 绿 | `#00703c` | `#5cb85c` |

内置 line diff renderer 跟随本工具的 light / dark 主题。

#### 4.2.10 移动端折叠（< 768px）

- 侧栏 Layers 折成顶部水平 chip 栏。
- 五层 section 默认全部折叠。
- 移动端强制 Stacked 视图（不水平滚动）。

```text
┌──────────────────────────────────┐
│  📘 EPUB Diff  [⇐]    [⚙] [ℹ]  │
├──────────────────────────────────┤
│  before: book-v1.epub (12.4 MB)  │
│  after:  book-v2.epub (11.8 MB)  │
│  ✅ Core metadata identical       │
├──────────────────────────────────┤
│  [Structure][Text][Style][Res][Meta]│  ← chip 栏（点击跳转）
├──────────────────────────────────┤
│  ⊟ Structure         (tap)       │
│  ⊟ Text              (tap)       │
│  ⊟ Style             (tap)       │
│  ⊟ Resources         (tap)       │
│  ⊟ Metadata          (tap)       │
└──────────────────────────────────┘
```

#### 4.2.11 帮助 / About 弹窗

点 `[ℹ ?]` 弹出：

```text
┌──────────────────────────────────────────────────┐
│  About EPUB Diff                           [×]  │
├──────────────────────────────────────────────────┤
│                                                  │
│  v0.1.0 · runs entirely in your browser          │
│                                                  │
│  This tool compares two EPUB files at the        │
│  file level. It does NOT render the books and    │
│  does NOT verify reading experience.             │
│                                                  │
│  For CI / red-line checks, use:                  │
│  scripts/validate_text_invariance.py             │
│                                                  │
│  For reader compatibility, see:                  │
│  docs/final/reader-matrix.yaml                   │
│                                                  │
│  Source: https://github.com/<owner>/epub-handbook│
│  License: MIT                                    │
│  Diff rendering: built-in line diff (MIT)          │
│  Zip parsing: zip.js (BSD-3)                     │
│                                                  │
└──────────────────────────────────────────────────┘
```

### 4.3 数据流（浏览器内，流式）

```text
[ 用户选两个 .epub（File 对象，未读入内存）]
        │
        ▼
[ zip.js: new ZipReader(new BlobReader(file)) ]
   只读 zip 末尾的 central directory（几 KB），不加载整包
        │
        ▼
[ await reader.getEntries() → 拿到所有 entry 的元信息列表 ]
        │
        ├─ 读 META-INF/container.xml entry → 拿 OPF 路径
        ├─ 读 OPF entry → manifest / spine / metadata
        │
        ▼
[ 五层 diff（按需流式读取每个 entry） ]
   ├─ structure.js: 解 OPF / nav / ncx（小文件，直接读 string）
   ├─ text.js:      逐个 XHTML entry → text → 段落 hash
   ├─ style.js:     逐个 CSS entry → text → selector diff
   ├─ resources.js: 资源 entry → zip.js Writer → IncrementalSha256 流式 hash
   │                 （字节不驻留；只保留 sha256 + 元信息）
   └─ metadata.js:  解 OPF metadata（已在第一步读出）
        │
        ▼
[ 构造 diff 数据对象（轻量 JSON，几 MB 级）]
        │
        ▼
[ render.js + line diff renderer ]
        │
        ▼
[ DOM 渲染 ]
```

**关键点**：

- **不向任何服务器发送数据**。
- **不写本地文件**（只读用户选的 epub）。
- **峰值内存 ≈ 「单个最大 entry」+「diff 状态」**，通常 < 50MB；500MB-1.5GB 的 epub 也能撑。
- **Hash 边流边算**：资源层用 zip.js Writer 把 chunk 喂给 `IncrementalSha256`，字节不进 JS heap。
- 主线程已够用；超大 epub（> 1.5GB）的 Web Worker 异步处理放到 v0.2 follow-up（不阻塞 v1）。

### 4.4 五层 diff 算法（移植到 JavaScript）

#### 4.4.1 结构层

输入：两份 `package.opf`、`nav.xhtml`、`toc.ncx`（通过 zip.js 按 entry 名取出）。

算法：

1. 用 `DOMParser` 解析 OPF，提取 manifest items（按 `id` 配对）。
2. 对比 spine itemref 序列（严格按顺序）。
3. 解析 nav.xhtml 的 `<nav epub:type="toc">` 树结构。
4. 解析 toc.ncx 的 navPoint 序列。

输出（事实描述）：

- 新增 / 删除 / 修改的 manifest items。
- spine 顺序变化（即使内容相同也展示）。
- nav 与 ncx 的层级差异。

#### 4.4.2 文本层

输入：每个 XHTML。
算法：

1. `DOMParser` 解析 XHTML。
2. 遍历所有 `p / h1–6 / li / td / blockquote / pre` 块级元素。
3. 提取 `textContent`，归一化（合并空白），SHA-256（用 [SubtleCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto)）。
4. 对每个 XHTML 文件比对段落 hash 列表。

输出：

- 文件路径 + 每段 hash + 状态。
- 段落变化时，用内置 line diff 渲染该段的行级 diff。

#### 4.4.3 样式层

输入：每个 CSS 文件（通过 zip.js 读出文本）。
算法：

1. 用 [CSSOM API](https://developer.mozilla.org/en-US/docs/Web/API/CSSOM) 或简单正则解析 CSS。
   - **推荐方案**：用 `new CSSStyleSheet()` + `replaceSync(text)`（modern browser 支持），然后遍历 `cssRules`。
   - **退化方案**：正则 `/[^{}]+\{[^}]+\}/g` 抓 selector + body。
2. 对每个 selector 在两边的存在情况和 property 集合做 diff。
3. **同时**保留原始文本，喂给内置 line diff renderer 做 line-level diff。

输出：

- selector 添加 / 删除 / 修改 count。
- 每个 CSS 文件的完整 line diff。
- @media 查询变化。

#### 4.4.4 资源层

输入：所有非 XHTML / 非 CSS 资源（图片、字体、音频等）。

算法：

1. 按 path 配对。
2. **流式 sha256**：用 zip.js 的 `entry.getData()` 接到一个自定义 Writer，把每个 chunk 喂给增量 SHA-256；不在 JS heap 中保留整个字节流。
   - 增量 hash 用 `tools/epub-diff/util/hash.js` 的纯 JS SHA-256，避免再引入 WASM 依赖。
   - 退化方案：单 entry 一次性读取后 `crypto.subtle.digest`，仅适合小 entry。
3. **图片对比策略**（仅适用于 `status: modified`）：
   - **新增 / 删除 / 未变**：不做缩略图，只展示文件名 + 大小 + sha256。
   - **修改**：用 `Image.naturalWidth/naturalHeight` 读 dimensions；用 `canvas.toDataURL('image/webp', 0.7)` 生成缩略图（最大边 200px），before/after 并排展示。
   - **大图（> 5MB）**：跳过缩略图，提示「图片体积较大，仅显示 hash 与大小差异」，避免内存压力。

输出：

- 计数：added / deleted / modified / unchanged。
- 修改了的图片：并排缩略图 + dimensions + 大小变化。
- 其他变化：只列文件名 + sha256 + size。

#### 4.4.5 元数据层

输入：OPF 的 `<metadata>` 段。
算法：

1. 解析所有 `<dc:*>` 和 `<meta>` 元素。
2. 按 name + scheme 配对。
3. 对比 value（字符串）。

输出：

- 核心字段（dc:title / dc:creator / dc:identifier / dc:language）单独高亮。
- 其他字段表格展示。

### 4.5 zip.js 本地化

#### 4.5.1 抓取脚本

`tools/epub-diff/scripts/fetch-vendor.sh`：

```sh
#!/usr/bin/env sh
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ASSETS="$ROOT/assets/vendor"
mkdir -p "$ASSETS"

# pin 具体版本（落地时填实际版本号）
ZIPJS_VERSION="2.8.26"

TMP=$(mktemp -d)
cd "$TMP"

# zip.js (@zip.js/zip.js, BSD-3-Clause)
npm pack "@zip.js/zip.js@$ZIPJS_VERSION"
tar -xzf zip.js-zip.js-*.tgz 2>/dev/null || tar -xzf zip.js-*.tgz
cp package/dist/zip.min.js "$ASSETS/zip.js"
cp package/LICENSE         "$ASSETS/zip.js.LICENSE"

cd -
rm -rf "$TMP"
echo "Fetched vendor assets to $ASSETS/"
ls -lh "$ASSETS/"
```

**不依赖 CDN**；运行时是 `tools/epub-diff/assets/vendor/` 下的本地文件。

zip.js 版本选 `dist/zip.min.js`，保持纯静态工具的依赖最小化；Worker 异步优化留到 v0.2。

#### 4.5.2 HTML 入口结构

`tools/epub-diff/index.html` 关键片段：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>EPUB Diff</title>
  <link rel="stylesheet" href="assets/style.css">
</head>
<body>
  <!-- Landing Page -->
  <div id="landing" class="view active">
    <header>
      <h1>📘 EPUB Diff</h1>
      <button id="theme-toggle">⚙</button>
      <button id="help-toggle">ℹ</button>
    </header>
    <main>
      <h2>选择两个 EPUB 文件来对比</h2>
      <!-- 两个 file picker 同时支持「点击选择」与「拖拽放入」-->
      <label class="file-picker" data-slot="before">
        <span class="picker-label">📄 before.epub</span>
        <input type="file" accept=".epub" id="before-file">
        <span class="file-name">no file selected · 或拖入此处</span>
      </label>
      <label class="file-picker" data-slot="after">
        <span class="picker-label">📄 after.epub</span>
        <input type="file" accept=".epub" id="after-file">
        <span class="file-name">no file selected · 或拖入此处</span>
      </label>
      <button id="compare-btn" disabled>Compare →</button>
    </main>
    <footer>
      This tool runs entirely in your browser. Your files never leave your device.
    </footer>
  </div>

  <!-- Loading -->
  <div id="loading" class="view">
    <progress id="progress"></progress>
    <div id="loading-status"></div>
    <button id="cancel-btn">Cancel</button>
  </div>

  <!-- Diff View -->
  <div id="diff" class="view">
    <header class="sticky">
      <h1>📘 EPUB Diff</h1>
      <button id="back-btn">⇐ Back</button>
      <button id="export-json">⬇ JSON</button>
    </header>
    <div class="info-bar"><!-- 文件信息 + 关键状态 --></div>
    <div class="grid">
      <aside id="layers-nav"></aside>
      <main id="layers-main">
        <section class="layer" id="layer-structure"></section>
        <section class="layer" id="layer-text"></section>
        <section class="layer" id="layer-style"></section>
        <section class="layer" id="layer-resources"></section>
        <section class="layer" id="layer-metadata"></section>
      </main>
    </div>
  </div>

  <!-- About modal -->
  <dialog id="about-modal">...</dialog>

  <script src="assets/vendor/zip.js"></script>
  <script type="module" src="app.js"></script>
</body>
</html>
```

#### 4.5.3 JS 模块拆分

```text
tools/epub-diff/
├── app.js                       # 入口；视图切换、文件拾取、orchestrate
├── parsers/
│   ├── epub.js                  # zip.js ZipReader + container.xml + OPF 解析
│   └── xml.js                   # 通用 DOMParser 包装
├── layers/
│   ├── structure.js
│   ├── text.js
│   ├── style.js
│   ├── resources.js
│   └── metadata.js
├── render/
│   ├── landing.js
│   ├── loading.js
│   ├── diff-view.js
│   ├── line-diff.js           # 内置行级 diff renderer
│   └── theme.js
└── util/
    ├── hash.js                  # SubtleCrypto + incremental SHA-256
    └── format.js                # 字节数 → 人类可读
```

### 4.6 任务清单

#### S3-T1：`tools/epub-diff/` 主体落地

**输入**：无。
**输出**：完整 web app。
**时间估算**：5–7 天。

**目录**：

```text
tools/epub-diff/
├── README.md
├── index.html
├── app.js
├── parsers/
│   ├── epub.js
│   └── xml.js
├── layers/
│   ├── structure.js
│   ├── text.js
│   ├── style.js
│   ├── resources.js
│   └── metadata.js
├── render/
│   ├── landing.js
│   ├── loading.js
│   ├── diff-view.js
│   ├── line-diff.js
│   └── theme.js
├── util/
│   ├── hash.js
│   └── format.js
├── assets/
│   ├── style.css                       # 本仓自定义样式
│   └── vendor/                         # gitignore：fetch-vendor.sh 拉取
│       ├── zip.js
│       └── zip.js.LICENSE
└── scripts/
    └── fetch-vendor.sh
```

**`tools/epub-diff/README.md` 内容**（drop-in）：

````markdown
# EPUB Diff (web app)

A purely browser-based EPUB diff tool. Open `index.html` in any modern
browser, select two EPUB files, click Compare.

## Use

1. Run `scripts/fetch-vendor.sh` once to populate `assets/vendor/`.
2. Double-click `index.html` (or drag into Chrome / Safari / Firefox).
3. Select two `.epub` files (click the picker or drag the files in); click Compare.

## Privacy

All processing happens in your browser. No file ever leaves your device.

## How it scales

The tool uses **zip.js** with streaming reads and an incremental SHA-256
writer for resource entries, so the entire EPUB is **never loaded into
memory**. Only the central directory is read first, and individual
entries are decompressed on demand. Peak memory is roughly the largest
text/CSS entry that must be parsed plus resource hash chunks and diff
state.

Design target: up to ~1.5 GB EPUBs on a typical laptop browser, subject to
the size of individual XHTML / CSS entries that must still be parsed as text.

## Layers

The tool compares two EPUBs across five layers:

- **Structure**: OPF manifest, spine, nav, ncx.
- **Text**: paragraph-level SHA-256 of XHTML text content.
- **Style**: CSS selectors and properties (rendered by the built-in line diff renderer).
- **Resources**: images, fonts, audio (streaming SHA-256).
- **Metadata**: dc:* and meta entries.

## What this tool does NOT do

- Does not render EPUB content / does not act as a reader.
- Does not run any Kindle Previewer / Apple Books integration.
- Does not verify reading experience.
- Does not enforce red-line policy — that's handled by
  `scripts/validate_text_invariance.py` in CI.
- Does not send data to any server.

## Dependencies (bundled)

- [zip.js](https://gildas-lormeau.github.io/zip.js/) (BSD-3-Clause) — streaming ZIP reader

Refetch via:

```sh
bash scripts/fetch-vendor.sh
```

## Browser compatibility

| Browser | Min version | Why |
| --- | --- | --- |
| Chrome / Edge | 100+ | `Blob.stream`, `crypto.subtle`, ES modules |
| Safari | 16.4+ | `CSSStyleSheet.replaceSync` |
| Firefox | 101+ | `CSSStyleSheet.replaceSync` |

All target browsers fully support `<input type="file">` and HTML5
drag-and-drop with `DataTransfer.files`.

## Limits

- Very large EPUBs (> 1.5 GB) may still hit browser memory limits.
  → consider a desktop wrapper only if real demand surfaces; not planned for v1.
- Single entries > 200 MB (e.g., uncompressed scan images) may need
  the streaming hash path; configured by default.

## License

MIT (this tool).

````

**`tools/epub-diff/assets/style.css` 关键内容**（drop-in 部分）：

```css
:root {
  --bg: #f8f7f4;
  --fg: #2d2d2d;
  --border: #d8d0c3;
  --neutral: #8f9099;
  --diff: #1d70b8;
  --green: #00703c;
  --mono: ui-monospace, SFMono-Regular, Menlo, monospace;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1a1a1a;
    --fg: #e0e0e0;
    --border: #444;
    --neutral: #666;
    --diff: #4a90d9;
    --green: #5cb85c;
  }
}

[data-theme="dark"] {
  --bg: #1a1a1a;
  --fg: #e0e0e0;
  --border: #444;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Helvetica Neue", sans-serif;
  background: var(--bg);
  color: var(--fg);
  margin: 0;
}

.view { display: none; }
.view.active { display: block; }

#landing main {
  max-width: 600px;
  margin: 4em auto;
  text-align: center;
}

.file-picker {
  border: 2px dashed var(--border);
  border-radius: 0.6em;
  padding: 2em;
  margin: 1em 0;
}
.file-picker.drag-over {
  border-color: var(--diff);
  background: rgba(29, 112, 184, 0.05);
}

#compare-btn {
  font-size: 1.2em;
  padding: 0.7em 2em;
  margin-top: 1em;
  cursor: pointer;
}
#compare-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

#diff .grid {
  display: grid;
  grid-template-columns: 220px 1fr;
}

#layers-nav {
  border-right: 1px solid var(--border);
  padding: 1em;
  position: sticky;
  top: 6em;
  height: calc(100vh - 6em);
  overflow-y: auto;
}

.layer {
  margin: 1em 2em;
  border: 1px solid var(--border);
  border-radius: 0.4em;
  padding: 1em;
}

@media (max-width: 768px) {
  #diff .grid { grid-template-columns: 1fr; }
  #layers-nav {
    position: static;
    height: auto;
    border-right: none;
    border-bottom: 1px solid var(--border);
    display: flex;
    overflow-x: auto;
  }
}
```

**测试 / 验收**（手动测试矩阵；落地后下棒按此跑一次记录）：

| 用例 | 输入 | 期望 |
| --- | --- | --- |
| WT1 | 同一份 epub 比自己 | 五层都「identical」 |
| WT2 | demo epub 改一段文字 | Text 层显示该段 diff |
| WT3 | demo epub 改 CSS（如 T1 双 float 改动）| Style 层显示 line diff |
| WT4 | demo epub 改 dc:title | Metadata 层核心字段差异显示 |
| WT5 | 删一个字体文件 | Resources 层显示 deleted font |
| WT6 | DRM epub | Landing 报错「Cannot read EPUB」 |
| WT7 | 非 zip 文件 | Landing 报错 |
| WT8 | 大 epub（100MB）| Loading 进度条更新，最终成功 |
| WT9 | 在 Chrome / Safari / Firefox 各开一次 | 都能跑 |
| WT10 | 系统切到暗色 | 自动切暗色主题 |
| WT11 | 拖拽 epub 到 file picker | 接受文件 |
| WT12 | 移动端（resize 到 < 768px）| 折叠为 chip 栏 |
| WT13 | 关掉网络后打开 index.html | 仍能工作（资源全本地） |

**验收命令**：

```sh
bash tools/epub-diff/scripts/fetch-vendor.sh
test -f tools/epub-diff/assets/vendor/zip.js
test -f tools/epub-diff/index.html
test -f tools/epub-diff/README.md
# 浏览器打开做手动 WT1–WT13 验收
```

**陷阱**：

- 浏览器打开 `file://` 协议时，部分 fetch / module 限制；建议在 README 写明「如遇模块加载问题，用 `python3 -m http.server` 在 `tools/epub-diff/` 内启 server」。
- `<input type="file">` 在 Safari 拖拽时需要 `dragover` 阻止默认事件。
- SubtleCrypto 在 `file://` 上仍可用，但部分老浏览器需 `https`。
- OPF 文件名不固定，先读 `META-INF/container.xml`。
- HTML 转义：把 CSS 内容写入内置 line diff renderer 前要正确 escape。

---

#### S3-T2：抓取脚本与 vendor 资源

**输入**：无。
**输出**：`tools/epub-diff/scripts/fetch-vendor.sh` + `assets/vendor/.gitignore`。
**时间估算**：3–4 小时。

**`assets/vendor/.gitignore`**：

```text
*.js
*.css
```

只保留 LICENSE 入 git；JS / CSS 资源由 `fetch-vendor.sh` 生成。

**版本 pin 策略**：

- `fetch-vendor.sh` 顶部明确写 `ZIPJS_VERSION` 变量。
- 不用 `latest`；落地时填具体版本号。
- 升级版本需要明确 commit `chore(vendor): bump zip.js to X.Y.Z`，且 README 更新。

**验收**：

```sh
bash tools/epub-diff/scripts/fetch-vendor.sh
ls -lh tools/epub-diff/assets/vendor/
# 期望看到 zip.js 和对应 LICENSE
```

---

#### S3-T3：`docs/pipeline/diff-tool.md`

**输入**：无。
**输出**：guide。
**时间估算**：3–4 小时。

**完整骨架**（drop-in）：

````markdown
# EPUB Diff 工具

> 状态：v1；浏览器内静态 web app。
> 用途：对比两个 EPUB 文件，**只做文件对比，不涉及阅读器效果验收**。
> 对应入口：`tools/epub-diff/index.html`。

## 用途

输入两份 epub，**人工 review** 文件层差异。

按五层组织：
- **结构**：OPF / spine / nav / ncx。
- **文本**：XHTML 段落级 hash。
- **样式**：CSS selector + property。
- **资源**：图片 / 字体 / 音频。
- **元数据**：dc:* / meta。

## 安装（一次）

```sh
bash tools/epub-diff/scripts/fetch-vendor.sh
```

这会把 zip.js 抓到 `tools/epub-diff/assets/vendor/`。

## 使用

1. 把 `tools/epub-diff/index.html` 用浏览器打开（双击或拖入 Chrome / Safari / Firefox）。
2. 在两个 file picker 各选一个 `.epub`。
3. 点 **Compare**。
4. 按五层逐层 review。

## 浏览器要求

- Chrome / Edge ≥ 90
- Safari ≥ 16
- Firefox ≥ 100

## 离线说明

工具完全离线运行；epub 文件不离开你的设备。

## 如果遇到 `file://` 模块加载问题

部分浏览器（特别是 Firefox / Safari 严格模式）对 `file://` 上的 ES module 有限制。解决：

```sh
cd tools/epub-diff/
python3 -m http.server 8000
# 浏览器打开 http://localhost:8000/
```

## 不做什么

- 不渲染 epub（不是阅读器）。
- 不集成 Kindle Previewer / Apple Books（启动按钮一律没有）。
- 不评估「这是不是合法的清洗」（这由 `scripts/validate_text_invariance.py` 在 CI 完成）。
- 不向任何服务器发数据。

## 五层 review 提示

参 [§4.2 mockup](handbook-expansion-plan.md#42-ui-设计)。

## 与清洗流程集成

见 [cleanup-flow.md](../pipeline/cleanup-flow.md)。

## 故障排查

| 现象 | 原因 / 解决 |
| --- | --- |
| 打开 index.html 一片空白 | `assets/vendor/` 是空的；先跑 `fetch-vendor.sh` |
| 点 Compare 后报错 "Cannot read EPUB" | 输入不是合法 zip / 有 DRM |
| 大 epub 浏览器卡死 | 浏览器内存超限；记录为开放问题，按需评估桌面 app 路径 |
| Safari 拖拽不工作 | 改用「Choose file」按钮 |
| CSS diff 只有行级高亮 | v1 使用内置 line diff renderer |
````

**验收**：

```sh
test -f docs/pipeline/diff-tool.md
grep -q "tools/epub-diff/index.html" docs/pipeline/diff-tool.md
grep -q "fetch-vendor.sh" docs/pipeline/diff-tool.md
```

---

#### S3-T4：根 README 入口

**输入**：S1-T1 已落地的根 README。
**输出**：确认 README 里指向 `tools/epub-diff/index.html` 的入口存在。
**时间估算**：5 分钟。

S1-T1 的 README drop-in 样本已经包含「看改前 / 改后差异」一段。Stage 3 落地时只需确认它仍然存在；不存在则补回。

### 4.7 Stage 3 全量验收清单

- [ ] `tools/epub-diff/` 目录结构完整。
- [ ] `bash tools/epub-diff/scripts/fetch-vendor.sh` 跑通，4 个 vendor 文件齐全。
- [ ] 双击 `tools/epub-diff/index.html`（或 `python3 -m http.server` 后打开）能进入 Landing。
- [ ] Landing → 选两个 demo epub → 点 Compare → 进入 Diff View。
- [ ] WT1–WT13 手动测试用例**至少跑通 1, 2, 3, 7, 9, 13 这六条**（其余可按情况延后到 follow-up）。
- [ ] `docs/pipeline/diff-tool.md` 落地，链接正确。
- [ ] 根 README 指向 `tools/epub-diff/index.html`。
- [ ] `python3 scripts/validate_skills_basic.py` 退出码 0。

---

## §5 Stage 4：端到端 demo

> 执行变更（2026-05-27）：本节原计划使用两本公版书。实际落地改为 `samples/demo-books/` 自造 EPUB，覆盖清洗通过样本和红线失败反例；`samples/third-party/` 仅保留为未来第三方样本占位。

### 5.1 目标

把 Stage 1–3 的所有产出贯穿一遍。

### 5.2 Demo 选择

实际落地样本：

| 样本 | 覆盖场景 | 预期 |
| --- | --- | --- |
| `city-field-notes` | 中英混排、Ruby、脚注、表格、代码、资源改动 | 红线通过 |
| `paper-garden` | 诗段、Ruby、blockquote、列表、竖排增强 | 红线通过 |
| `redline-trap` | 故意改写正文 | 红线失败 |

以下公版书候选保留为历史原计划，首轮暂缓：

| 候选 | 体量 | 覆盖场景 | 许可 | 来源 |
| --- | --- | --- | --- | --- |
| 鲁迅《呐喊》 | ~8 万字 | 现代汉语小说集；多 XHTML；标准弹注 | 公版（中国） | Project Gutenberg |
| 《唐诗三百首》 | ~4 万字 | 古典文本；Ruby；短段落 | 公版 | Project Gutenberg #25196 |

### 5.3 实际任务清单

#### S4-T0：自造 demo 样本入仓

**输出**：

- `scripts/build_demo_epubs.py`
- `samples/demo-books/README.md`
- `samples/demo-books/build.sh`
- `samples/demo-books/*/notes.md`

**验收命令**：

```sh
bash samples/demo-books/build.sh
python3 scripts/validate_text_invariance.py samples/demo-books/dist/city-field-notes-before.epub samples/demo-books/dist/city-field-notes-after-clean.epub --check all
python3 scripts/validate_text_invariance.py samples/demo-books/dist/paper-garden-before.epub samples/demo-books/dist/paper-garden-after-clean.epub --check all
python3 scripts/validate_text_invariance.py samples/demo-books/dist/redline-trap-before.epub samples/demo-books/dist/redline-trap-after-text-changed.epub --check all
```

前两条必须退出码 0；`redline-trap` 必须退出码 1。

#### 原公版书任务（暂缓）

下面 S4-T1 / S4-T2 是原计划内容，保留作未来第三方样本参考；本轮不执行。

#### S4-T1：鲁迅《呐喊》样本入仓

**输入**：无。
**输出**：`samples/third-party/lu-xun-nahan/`。
**时间估算**：1.5–2 天（含跑通管线）。

**目录**：

```text
samples/third-party/lu-xun-nahan/
├── LICENSE.txt
├── metadata.yaml
├── fetch.sh
├── notes.md
├── before/   (gitignored)
└── after/    (gitignored)
```

**`LICENSE.txt`**（drop-in）：

```text
This sample is sourced from Project Gutenberg.

Author: 鲁迅 (Lu Xun, 1881–1936)
Title: 《呐喊》(Call to Arms / Nàhǎn)
Original publication: 1923
Public domain status: 中国 (作者去世 + 50 年 = 1986)；US (PG release)

Project Gutenberg license terms apply:
https://www.gutenberg.org/policy/license.html

The local epub samples (before/, after/) are NOT committed to git.
Re-fetch via fetch.sh; SHA-256 verification embedded.
```

**`metadata.yaml`**（drop-in 模板，URL / SHA 实测后填）：

```yaml
slug: lu-xun-nahan
title: 呐喊
creator: 鲁迅
language: zh-CN
license: public-domain-cn-1986
source:
  provider: Project Gutenberg
  url: https://www.gutenberg.org/ebooks/<PG_ID>
  format: epub-no-images
  fetched_at: <DATE>
  expected_sha256: <SHA256>
notes_file: notes.md
demo_purpose: 现代汉语小说集端到端清洗
```

**`fetch.sh`**（drop-in 模板）：

```sh
#!/usr/bin/env sh
set -eu
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
METADATA="$SCRIPT_DIR/metadata.yaml"
DEST="$SCRIPT_DIR/before/source.epub"

URL=$(grep "^  url:" "$METADATA" | head -1 | sed 's/.*url: *//')
EXPECTED_SHA=$(grep "^  expected_sha256:" "$METADATA" | head -1 | sed 's/.*expected_sha256: *//')

if [ -z "$URL" ] || [ -z "$EXPECTED_SHA" ]; then
  echo "metadata.yaml incomplete; fill url + expected_sha256." >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
curl -L -o "$DEST" "$URL"

ACTUAL=$(shasum -a 256 "$DEST" | awk '{print $1}')
if [ "$ACTUAL" != "$EXPECTED_SHA" ]; then
  echo "SHA mismatch: $ACTUAL != $EXPECTED_SHA" >&2
  rm -f "$DEST"
  exit 1
fi
echo "Fetched and verified: $DEST"
```

**`notes.md` 完整骨架**（drop-in）：

````markdown
# 鲁迅《呐喊》端到端清洗记录

> 日期：<DATE>
> 操作者：<NAME>
> Source：见 `metadata.yaml`
> 流程：见 [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md)

## 0. 抓取与备份

```sh
bash fetch.sh
ls -lh before/source.epub
```

## 1. 输入判断

- 体量：<size>
- XHTML 数：<n>
- CSS：<n>
- 图片：<n>
- DRM：<yes/no>
- 文本红线初检：`python3 ../../../scripts/validate_text_invariance.py before/source.epub before/source.epub` → 0 ✅

## 2. harness 扫描

```sh
python3 ../../../scripts/epub_ai_harness.py --mode cleanup before/source.epub > findings.json
```

主要 findings：
- <issue 1>
- <issue 2>

推荐 skills 顺序：
1. <skill>
2. <skill>

## 3. 清洗执行

按 `recommended_skills` 顺序：

### 3.1 <skill name>
- 改了什么：
- 为什么：
- 文本红线验证：
  ```sh
  python3 ../../../scripts/validate_text_invariance.py before/source.epub after/step-1.epub
  ```
  退出码：0 ✅

### 3.2 <skill name>
...

## 4. 完整文本校验

```sh
python3 ../../../scripts/validate_text_invariance.py before/source.epub after/cleaned.epub --redlines all
```
退出码：0 ✅

## 5. Diff 人工 review

把 `tools/epub-diff/index.html` 用浏览器打开，选 `before/source.epub` 和 `after/cleaned.epub`。

**主要差异**：
- 结构：<>
- 文本：identical ✅
- 样式：<N> selector 改动（详见浏览器截图 `diff-style.png`）
- 资源：<N> 删 / <N> 加
- 元数据：core identical ✅

（如果团队需要存档，把浏览器截图保存到本目录下 `diff-*.png`，gitignore 已忽略 `*.epub`，截图可以入仓。）

## 6. 阅读器实测（可选）

| 阅读器 | 版本 | 默认字号 | 结果 |
| --- | --- | --- | --- |
| Kindle Previewer | 3.104.0 | 4 | pass |
| Apple Books macOS | <ver> | 默认 | pass |

详见 reader-matrix.yaml 新增条目。

## 7. 已知未解决

- <issue>

## 8. 复现

```sh
bash fetch.sh
# 按 §3 的 skills 顺序执行
python3 ../../../scripts/validate_text_invariance.py before/source.epub after/cleaned.epub --redlines all
# 浏览器打开 tools/epub-diff/index.html，选 before/source.epub 和 after/cleaned.epub
```
````

**验收**：

- 目录结构完整。
- `fetch.sh` 能跑通（下载 + SHA 校验）。
- 完整跑过 8 步流程，`notes.md` 实际内容填充。
- 第 5 步必须有「浏览器打开 web app」记录（可截图入仓）。
- `reader-matrix.yaml` 新增条目（可选，按实测）。

---

#### S4-T2：《唐诗三百首》样本入仓

**输入**：无。
**输出**：`samples/third-party/tang-poems-300/`。
**时间估算**：1.5–2 天。

**与 S4-T1 同结构**，只是：

- `slug: tang-poems-300`
- `title: 唐诗三百首`
- `creator: 蘅塘退士 (清代选编)`
- `demo_purpose: 古典诗歌端到端清洗（Ruby + 短段落）`
- Project Gutenberg ID `25196`，URL `https://www.gutenberg.org/ebooks/25196`

清洗重点：

- Ruby（生僻字注音）规范化 → `epub-vertical-ruby-optimizer`
- 标题层级
- 短段落 CSS

---

#### S4-T3：reader-matrix 增 case

**输入**：当前 `docs/final/reader-matrix.yaml`。
**输出**：新增 2 个 case + 至少 4 条 expectations。
**时间估算**：30 分钟。

**新增 cases**：

```yaml
cases:
  # ...（既有保持）
  - id: ext-lu-xun-nahan
    title: 鲁迅《呐喊》端到端清洗 demo
    fixture: samples/third-party/lu-xun-nahan/
  - id: ext-tang-poems-300
    title: 《唐诗三百首》端到端清洗 demo
    fixture: samples/third-party/tang-poems-300/
```

**expectations** 至少覆盖 Kindle Previewer + Apple Books，status 按实测填。

---

#### S4-T4：`docs/getting-started/05-case-study.md`

**输入**：S4-T1 / S4-T2 的 `notes.md`。
**输出**：可阅读 narrative。
**时间估算**：半天。

**结构**：

```markdown
# 真实公版书清洗案例

## 案例 1：鲁迅《呐喊》

[narrative：拿到原始 PG epub 时看到了什么、用了哪些 skill、diff 工具里看到了什么]

详细记录见 `samples/third-party/lu-xun-nahan/notes.md`。

[嵌入 diff 工具截图或关键 ASCII 摘要]

## 案例 2：《唐诗三百首》

[narrative：Ruby 处理 + 短段落 CSS]

详细记录见 `samples/third-party/tang-poems-300/notes.md`。

## 学到了什么

- 一本现成 epub 90% 的问题集中在 <几类>。
- AI 清洗最常用的 skill 是 <列表>。
- Diff 工具最常显示差异的层是 <列表>。
- 我们没解决的：<开放问题>。

## 下一步

- 拿你自己的 epub，跟着 [docs/pipeline/cleanup-flow.md](../pipeline/cleanup-flow.md) 跑一遍。
- 用 [tools/epub-diff/index.html](../../tools/epub-diff/index.html) 看自己的清洗结果。
```

---

#### S4-T5：`THIRD_PARTY.md` 同步

**输入**：现有 `THIRD_PARTY.md`。
**输出**：新增两条。
**时间估算**：10 分钟。

**新增**：

```markdown
## 鲁迅《呐喊》
- 作者：鲁迅 (1881–1936)
- 来源：Project Gutenberg
- 许可：公版（中国 1986+）
- 链接：https://www.gutenberg.org/ebooks/<PG_ID>
- 使用方式：仅在 `samples/third-party/lu-xun-nahan/` 作为清洗 demo；实体 epub 不入 git。

## 唐诗三百首
- 编者：蘅塘退士（清）
- 来源：Project Gutenberg #25196
- 许可：公版
- 链接：https://www.gutenberg.org/ebooks/25196
- 使用方式：仅在 `samples/third-party/tang-poems-300/` 作为清洗 demo；实体 epub 不入 git。

## zip.js
- 项目：Gildas Lormeau (`@zip.js/zip.js`)
- 许可：BSD-3-Clause，见 `tools/epub-diff/assets/vendor/zip.js.LICENSE`
- 用途：`tools/epub-diff/` 浏览器内 epub **流式**解压（central directory + on-demand entry，不一次性载入整包）。
```

### 5.4 Stage 4 全量验收清单

- [x] `samples/demo-books/` 完整。
- [x] 两组合法清洗 demo 能生成并通过 `validate_text_invariance.py --check all`。
- [x] `redline-trap` 能生成并按预期触发文本红线失败。
- [x] `docs/getting-started/05-case-study.md` 落地。
- [x] `THIRD_PARTY.md` 同步 vendor，并说明自造 demo 无第三方许可负担。
- [x] `reader-matrix.yaml` 增 2 个自造 demo case + 4 条待实测 expectations。

原公版书验收项暂缓：

- [ ] `samples/third-party/lu-xun-nahan/` 完整。
- [ ] `samples/third-party/tang-poems-300/` 完整。

---

## §6 跨阶段约束与不变量

### 6.1 不可破契约

1. **CLAUDE.md 闭环不可绕**。
2. `docs/final/` 仍是对外硬约束。
3. `skills/*/SKILL.md` frontmatter 字段名不改。
4. `templates/epub-style-demo/` 是新书制作 fixture，不被清洗 demo 污染。
5. **EPUB 文本完整性是红线**（CI 由 `validate_text_invariance.py` 保护）。
6. **Web app 不做效果验收**：只做文件级 diff，不渲染、不模拟、不集成阅读器。
7. **Web app 不向任何服务器发数据**：完全浏览器内运行。

### 6.2 文档与工具分层（落地后）

```text
docs/getting-started/      ← Stage 1 新增
docs/final/                ← 保持
docs/guides/               ← 保持，扩 2 篇（cleanup-flow + diff-tool）
docs/pipeline/             ← Stage 1 新增
docs/source/               ← 不变
docs/experiments/          ← 不变
samples/third-party/       ← Stage 2 新增 + Stage 4 填充
tools/epub-diff/           ← Stage 3 新增（web app）
```

### 6.3 脚本

```text
scripts/
├── epub_ai_harness.py            ← Stage 2 加 --mode cleanup
├── validate_text_invariance.py   ← Stage 2 新增（v0.1 文本；v0.2 扩 metadata/spine/cover/DRM）
├── test_validate_text_invariance.py ← Stage 2 新增
├── validate_epub_style_demo.py   ← 现有
├── validate_popup_notes.py       ← 现有
└── validate_skills_basic.py      ← 现有
```

**没有** `epub_diff.py`（v1 不再做 Python 端 diff 工具；diff 完全在 web app 中）。

### 6.4 依赖

**Python**（仅 Stage 2）：

`requirements.txt`：

```text
lxml>=4.9
```

**Web app**（仅 Stage 3，运行时通过 fetch-vendor.sh 拉取，不入 git）：

- `@zip.js/zip.js`（pin 具体版本，约 30KB minified；BSD-3-Clause）

**构建工具**：

- `npm`（仅 `fetch-vendor.sh` 一次性拉取用，运行时不依赖）。

**不引入**：tinycss2、Jinja2、BeautifulSoup、PyYAML、Pandas、Pillow、任何 Node.js runtime（zip.js 是浏览器端脚本）、任何 ML/NLP 库、任何 Web 框架、Electron、JSZip（被 zip.js 取代）。

### 6.5 不引入的方向

- 不引入数据库。
- 不引入后端 Web 服务（web app 是纯静态）。
- 不引入并发框架。
- 不引入 ML / NLP 库。
- 不引入桌面 app 打包（v1）。
- 不引入阅读器集成 / Kindle Previewer 启动 / Apple Books 启动。

---

## §7 决策与待定

### 7.1 必答（落地前在 `docs/pipeline/decisions.md` 写答案）

| # | 问题 | 推荐 | 理由 |
| --- | --- | --- | --- |
| Q1 | Stage 顺序 | 严格串行 1→2→3→4 | Stage 3 web app UI 状态条引用 §10 描述；Stage 4 demo 依赖 web app |
| Q2 | Vendor 资源（zip.js）是否入 git | 不入（生成式） | 体积 + license 文件单独入仓即可 |
| Q3 | AI 改动黄线是否可配置 | 不可配置 | 统一硬规则 |
| Q4 | 公版书实体 .epub 入 git | 不入 | 仓库体积 + 版权 / 来源溯源 |
| Q5 | Web app 入口位置 | `tools/epub-diff/` | 与现有 `templates/` 分开（templates 是 fixture，tools 是用户工具）|

### 7.2 可选

| # | 问题 | 建议 |
| --- | --- | --- |
| Q6 | GitHub Actions CI | 对 `validate_text_invariance.py` 加最小 CI（self vs self 退 0） |
| Q7 | `docs/getting-started/` 是否中英双语 | 仅中文 |
| Q8 | 是否补 `samples/third-party/` 第 3 本 | Stage 4 完成后看反馈 |
| Q9 | `validate_text_invariance.py` 是否提供 `--fix` 自动修复 | 不做 |
| Q10 | Web app 是否支持把整份 diff 导出为单 HTML 文件 | 不做，浏览器原生「另存为」凑合用 |
| Q11 | 是否做 GitHub Pages 部署 web app | 不做（保持本仓只是源码，部署交给团队）|

---

## §8 参考资料

### 8.1 Diff 工具相关

1. **YiTong（onevcat）** — Apple 平台 diff 视图（参考）
   <https://github.com/onevcat/YiTong>

2. **opencode-diffs** — Web 端 diff 工具参考
   <https://github.com/oorestisime/opencode-diffs>

### 8.2 浏览器内 epub 解析

1. **zip.js (`@zip.js/zip.js`)**：<https://gildas-lormeau.github.io/zip.js/>
2. **DOMParser MDN**：<https://developer.mozilla.org/en-US/docs/Web/API/DOMParser>
3. **SubtleCrypto MDN**：<https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto>
4. **CSSStyleSheet.replaceSync MDN**：<https://developer.mozilla.org/en-US/docs/Web/API/CSSStyleSheet/replaceSync>

### 8.3 Python 端

1. **Python `lxml`**：<https://lxml.de/>

### 8.4 公版书来源

1. **Project Gutenberg 中文**：<https://www.gutenberg.org/browse/languages/zh>
2. **Wikisource 中文**：<https://zh.wikisource.org/>
3. **Project Gutenberg #25196 唐诗三百首**：<https://www.gutenberg.org/ebooks/25196>

### 8.5 本仓既有

- `CLAUDE.md`
- `docs/final/SPEC-实现约束.md`
- `docs/final/reader-matrix.yaml`
- `scripts/epub_ai_harness.py`
- `skills/epub-layout-auditor/SKILL.md`
- `skills/epub-source-intake/SKILL.md`
- `docs/experiments/classical-parallel-epub-analysis-20260525.md`
- `docs/experiments/review-classical-modern-kindle-20260526.md`

---

## §9 给下一棒的 Checklist

### 9.1 落地前

1. 读完 §0（关键决策一览）。
2. 在 `docs/pipeline/decisions.md` 写下 §7.1 必答题答案，commit 一次（`docs(pipeline): record planning decisions`）。

### 9.2 Stage 1（约 2 周）

1. 按 §2.3 任务清单（基础部分）：
   - S1-T1 重写根 README（带 `tools/epub-diff` 入口）
   - S1-T2 `docs/getting-started/` 5 篇基础文档
   - S1-T3 `docs/pipeline/README.md`
   - S1-T4 `docs/README.md` 索引
   - S1-T5 `CLAUDE.md` 修改优先级同步
2. 按 §2.3 任务清单（扩展部分）：
   - S1-T6 `docs/getting-started/06-test-your-own.md`
   - S1-T7 `docs/getting-started/07-faq.md`
   - S1-T8 `docs/getting-started/glossary.md`
   - S1-T9 `04-skills.md` 反向查表
   - S1-T10 `03-readers.md` 决策树
   - S1-T11 `getting-started/README.md` do/don't + 导航
   - S1-T12 根 `CONTRIBUTING.md`
3. commit 分两个：
   - `feat(docs): add three-layer manual structure`
   - `docs(getting-started): add test-your-own, faq, glossary and contributing guide`
4. 跑 §2.4 验收清单。

### 9.3 Stage 2（约 3 周）

1. 按 §3.2 任务清单（基础部分）：
   - S2-T1 SPEC §10（红/黄/绿/元规则/自动化 gate）
   - S2-T2 `validate_text_invariance.py` v0.1（文本红线）
   - S2-T3 `docs/pipeline/cleanup-flow.md` 8 步骨架
   - S2-T4 `samples/third-party/` 占位
   - S2-T5 `epub-layout-auditor` skill 同步
   - S2-T6 harness `--mode cleanup`
2. 按 §3.2 任务清单（扩展部分）：
   - S2-T7 SPEC §10.6 能力清单
   - S2-T8 `docs/pipeline/cleanup-patterns.md`
   - S2-T9 `validate_text_invariance.py` v0.2（metadata / spine / cover / DRM 红线）
   - S2-T10 `cleanup-flow.md` §1 健康检查 + §9–§14 新章节
   - S2-T11 11 个清洗 skill 加 `--dry-run` 约定
   - S2-T12 `docs/pipeline/skills-matrix.md`
   - S2-T13 `samples/fixtures-tiny/` 7 个微型 fixture
   - S2-T14 `docs/pipeline/asset-optimization.md`（图片转换、无损压缩、字体子集化，全用现有工具）
3. commit 分五个：
   - `feat(spec): add §10 AI cleanup boundary rules and §10.6 capability matrix`
   - `feat(scripts): add validate_text_invariance v0.2 covering all five red lines`
   - `feat(guides): add cleanup-flow with batch / rollback / resilience and patterns catalog`
   - `feat(skills): add dry-run convention and pipeline skills matrix`
   - `docs(guides): add asset-optimization guide for image and font tooling`
4. 跑 §3.3 验收清单。

### 9.4 Stage 3（约 3 周）

1. 按 §4.6 任务清单：
   - S3-T1 `tools/epub-diff/` 主体（HTML + JS 五层 + UI）
   - S3-T2 `fetch-vendor.sh` + vendor 资源
   - S3-T3 `docs/pipeline/diff-tool.md`
   - S3-T4 根 README 入口确认
2. commit 分三个：
    - `feat(tools): add epub-diff web app skeleton`
    - `feat(tools): implement five-layer diff in epub-diff web app`
    - `docs(guides): add epub-diff-tool guide`
3. 跑 §4.7 验收清单（手动 WT1–WT13 至少跑通核心 6 条）。

### 9.5 Stage 4（约 1.5 周）

1. 按 §5.3 任务清单：
    - S4-T1 鲁迅《呐喊》样本
    - S4-T2 《唐诗三百首》样本
    - S4-T3 reader-matrix 增 case
    - S4-T4 case study
    - S4-T5 THIRD_PARTY.md 同步
2. commit 分三个：
    - `feat(samples): add Lu Xun Nahan e2e cleanup demo`
    - `feat(samples): add Tang poems 300 e2e cleanup demo`
    - `feat(docs): add getting-started case study`
3. 跑 §5.4 验收清单。

### 9.6 每个 commit 前必跑

```sh
# 1. 工作区干净
git status

# 2. 既有 demo 不破
bash templates/epub-style-demo/build.sh
NEW=$(ls -t templates/epub-style-demo/dist/ | head -1)
bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/"$NEW"
bash scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/"$NEW"

# 3. skills harness
python3 scripts/validate_skills_basic.py

# 4. 新加的脚本（如在当前 stage 落地了）
[ -f scripts/test_validate_text_invariance.py ] && python3 scripts/test_validate_text_invariance.py

# 5. web app（Stage 3 后）
[ -f tools/epub-diff/index.html ] && echo "web app present; run manual smoke test if changed UI/JS"
```

### 9.7 卡点

任何卡点写进 `docs/pipeline/decisions.md` 续写区，不要私下重写本计划。本计划被实测推翻时**以实测为准**，回头修订本计划。

---

> 本计划是规划层文档，不构成「请直接落地」的承诺。所有 §阶段的 NEW 块、目录结构、CLI 接口都是建议；下一棒按 §6 不变量约束落地时可以调整命名、参数、目录细节。落地后如发现本计划与实测/SPEC 冲突，**以实测和 SPEC 为准**，回过头修订本计划。
