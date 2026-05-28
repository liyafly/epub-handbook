# 手册扩展计划落地 Review 与续做指南
> **2026-05-28 更新**：本 review 中关于 `tools/epub-diff/` 的所有 P0 / P2 条目（流式 SHA-256、@pierre/diffs、AbortSignal、modified 图片缩略图、vendor 升级流程文档）均已**作废**，因为整个 `tools/` 目录于 2026-05-28 整体移除。后续 diff review 走 Calibre / VS Code，见根 `README.md` 的 `#epub-diff-review` 段与 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)。正文保留为历史快照。
>


> **审稿对象**：`docs/plans/handbook-expansion-plan.md`（Claude 草稿，2026-05-26）
> 与本分支 `codex/handbook-expansion-pipeline-diff`（commit `6806868`、`7e0bb36`）相对于 `develop` 的落地差异。
> **审稿日期**：2026-05-27
> **审稿基线**：develop = `d3bc904`；当前分支 HEAD = `6806868`。
> **核心约束**：所有续做项必须遵守 `CLAUDE.md` 的实测回写闭环；`docs/final/` 仍是对外约束层。
>
> **决策提示**（已与用户确认，2026-05-27）：
>
> - 公版书 demo（鲁迅《呐喊》+《唐诗三百首》）**不做**。原计划 §5.2 / §5.3 / §7.1 Q8 中的公版书条目视为永久搁置，本 review 不再把它们列为「缺口」，只把计划文档里残留的公版书表述列入「文档收尾项」。
> - `samples/demo-books/` 自造 EPUB（`city-field-notes` / `paper-garden` / `redline-trap`）是 Stage 4 的最终交付样本，已经入仓且红线脚本通过。

---

## 0. TL;DR

| Stage | 计划任务 | 实际落地 | 评级 |
| --- | --- | --- | --- |
| 1 手册三层化 | 12 个任务（S1-T1..T12） | 全部落地，文件齐全 | ✅ 通过，遗留少量小问题 |
| 2 清洗流水线 | 14 个任务（S2-T1..T14） | 9 项实际可用、4 项「占位」、1 项「文档结构破损」 | ⚠ 需修一个破损 + 把占位写明 |
| 3 EPUB Diff Web App | 4 个任务（S3-T1..T4） | UI / 五层 diff / 文档全在，**两个关键技术承诺未兑现** | ⚠ 与承诺有偏差，必须改 README/计划文档或补足实现 |
| 4 端到端 demo | 5 个任务（S4-T1..T5） | 已按用户决策改成自造 demo，全部跑通 | ✅ 通过 |
| 跨阶段约束 | §6 七条 | 全部遵守 | ✅ 通过 |

**优先级 P0（必须改）**：

1. `docs/pipeline/cleanup-flow.md` 文档结构破损（§15 错位到 §14 模板代码块中）。
2. `tools/epub-diff/` 流式 SHA-256 与 @pierre/diffs 渲染**没有真正实现**，但 README 和 SPEC 文档都按「已实现」描述。要么补足实现，要么把对外承诺降级。

**优先级 P1（高建议）**：

3. `samples/fixtures-tiny/` 现状只有 `.gitkeep`，README 自己也写「目前自动化测试在 test_validate_text_invariance.py 内生成等价 EPUB」。计划对它的承诺要么补 fixture，要么把这个目录在计划/README/SPEC 里降级为「测试 fixture 占位」。
4. `validate_text_invariance.py` v0.2 的「章节锚点 id」红线没有任何脚本覆盖，SPEC §10.1 表格写「人工 review + 专项脚本扩展」，但 SPEC §10.5 把它隐含包含在「全量红线」里。**口径不一致**。
5. `epub_ai_harness.py --mode cleanup` 的 `recommended_skills` 排序在 cleanup 模式下没有按计划「裁掉非清洗 skill」，只是把 `epub-layout-auditor` 顶到首位、把 `epub-source-intake` 移走。

**优先级 P2（应当处理）**：

6. Stage 2 计划中的 `--dry-run` 约定只写进了 SKILL.md 文档，**没有任何 skill 真正实现 `--commit` 模式**；这是契约层而不是脚本层的 gap，但需要明确告诉用户「Dry-run 约定」是 AI 行为契约而不是 CLI 行为。
7. `docs/getting-started/glossary.md` 缺计划文档要求的 EPUB、Enhanced Typesetting、KF8、MOBI、Project Gutenberg #25196 等条目，且没覆盖到「第 10 个字母分组」（当前 14 个）；不算阻塞，但与 §S1-T8 验收条件略有出入。
8. 计划 §5.3 / §7.1 Q8 仍残留鲁迅《呐喊》/ 《唐诗三百首》/ Project Gutenberg 引用。既然已经决策不做，计划文档应在 §5 / §7 / §9 / §8.4 处把这些条目标记为「已搁置」或删除。

---

## 1. Stage 1：手册三层化（M1.0）

### 1.1 已落地

S1-T1 ~ S1-T12 全部产生了实际文件，对应验收命令多数能跑通：

| 任务 | 文件 | 状态 |
| --- | --- | --- |
| S1-T1 README 重写 | `README.md`（50 行 ≤ 100；含 5 列场景表；指向 `tools/epub-diff/index.html`） | ✅ |
| S1-T2 入门 5 篇 | `docs/getting-started/01..05.md` + `README.md` | ✅ |
| S1-T3 pipeline 占位 | `docs/pipeline/README.md`（37 行） | ✅ |
| S1-T4 文档索引 | `docs/README.md`（57 行；含 guides / pipeline / samples / tools / source / experiments） | ✅ |
| S1-T5 CLAUDE.md 同步 | 第 5..7 条已加 `getting-started/` / `pipeline/` / `tools/` | ✅ |
| S1-T6 测自己的 epub | `docs/getting-started/06-test-your-own.md`（72 行；含 epubcheck 段） | ✅ |
| S1-T7 FAQ | `docs/getting-started/07-faq.md`（≥ 12 个 Q&A） | ✅ |
| S1-T8 术语表 | `docs/getting-started/glossary.md`（14 个字母分组） | ✅ 但偏简（见下） |
| S1-T9 skill 反向查表 | `04-skills.md` 含「我想做 X，用哪个 skill」表（14 行） | ✅ |
| S1-T10 reader 决策树 | `03-readers.md` 含「我该测哪个」四场景 | ✅ |
| S1-T11 do/don't | `getting-started/README.md` 含「速查」「读完入门去哪」「推荐阅读顺序」 | ✅ |
| S1-T12 CONTRIBUTING | `CONTRIBUTING.md`（82 行；含 reader-matrix 回写规范） | ✅ |

### 1.2 偏差与建议

**1) `docs/getting-started/01-first-epub.md` §1**

```sh
git clone https://github.com/<owner>/epub-handbook.git
```

`<owner>` 占位符还在。本仓应该有真实 GitHub URL 或者干脆删掉这条命令（让用户自己 fork 后 clone）。

**建议**：删除 §1 整段，或换成中性表述「在本地仓库根目录执行…」。

**2) `docs/getting-started/glossary.md` 偏简**

计划 §S1-T8 给的 drop-in 样本覆盖 `KFX` / `KF8` / `MOBI` / `Project Gutenberg #25196` / `epub:type` 详细子集 / `Enhanced Typesetting`（这一条已有）/ `xml:lang` 详细等。当前 81 行只保留了主干。

**建议**：作为可自由补充内容（CLAUDE.md 优先级 5）。如果保持现状，请在 review 处显式记一笔「与计划样本相比简化」，避免下棒按计划文档比对时认为缺失。

**3) `04-skills.md` 反向查表少一行**

计划样本是 14 行；落地 14 行，但「弹注 / 文学结构 / 出处规范化」与「英文小说专项排版」两行的措辞与原版略有简化（不影响可用性）。

**4) `docs/README.md` 索引漂移风险**

`docs/README.md` 把 guides 列表手写成长清单，没有自动同步。后续新增 / 重命名 guide 时容易漏改。

**建议**：作为下一棒维护项处理，不必现在做；只是把它作为 known issue 标记在 `docs/pipeline/decisions.md`。

### 1.3 续做建议

- Stage 1 不需要新工作。如果要清理，删 01-first-epub.md 的 `<owner>` 占位即可。

---

## 2. Stage 2：已有 EPUB 清洗成为核心（M1.1）

### 2.1 已落地

| 任务 | 落地状态 |
| --- | --- |
| S2-T1 SPEC §10 红/黄/绿/元规则 | ✅ `docs/final/SPEC-实现约束.md` §10.1..§10.5 完整 |
| S2-T2 validate_text_invariance.py v0.1 | ✅ + 实际直接上了 v0.2（合并到一个 script） |
| S2-T3 cleanup-flow.md 骨架 | ✅ 但 §14/§15 结构破损（P0，见 2.2） |
| S2-T4 samples/third-party/ 占位 | ✅ |
| S2-T5 epub-layout-auditor SKILL 同步 | ✅ 已含 §10 / validate / web app 提示 |
| S2-T6 harness `--mode cleanup` | ✅ 行为不完全匹配计划（见 2.2） |
| S2-T7 SPEC §10.6 能力清单 | ✅ |
| S2-T8 cleanup-patterns.md | ✅ 8 个模式（A..H） |
| S2-T9 validate_text_invariance.py v0.2 | ✅ 19 测试用例 → 实际 20 个用例，全部通过 |
| S2-T10 cleanup-flow §1 健康 + §9..§14 | ⚠ §14 模板代码块内嵌入了 §15，结构破了（P0） |
| S2-T11 11 个 skill 加 dry-run 约定 | ⚠ 文档加了，但没人实现 `--commit`（见 2.3） |
| S2-T12 pipeline/skills-matrix.md | ✅ 14 行 |
| S2-T13 samples/fixtures-tiny/ | ⚠ 仅 `.gitkeep`，无 fixture（P1） |
| S2-T14 asset-optimization.md | ✅ 但比计划样本短得多（158 vs 计划 480 行；缺 GUI 工具表、子集化决策树等） |

### 2.2 P0 / P1 问题详解

**P0：`docs/pipeline/cleanup-flow.md` §14/§15 结构破损**

当前文件结构：

```text
## 14. 标准 `notes.md` 模板
```md          ← 代码栅栏开
# 清洗记录：<书名>
...
## 4. 完整红线校验
...
## 15. 自造 demo   ← ❌ 错位在代码栅栏里
bash samples/demo-books/build.sh
...
## 5. Diff 概览     ← 原来是 notes.md 模板的一部分
## 6. 可信度评估
```           ← 代码栅栏关
```

`## 15. 自造 demo` 段被误塞进了 notes.md 模板的代码块中，导致 §15 整段都被显示为「notes.md 模板示例的一部分」。这会让 Markdown 渲染异常、链接失效，且对下棒非常误导。

**修法**：把 §15 提到 §14 之外，并把 notes.md 模板里残留的 §5 / §6 子段还原成「§4 / §5 / §6 都是 notes.md 模板的子节」结构。建议如下重排：

```text
## 14. 标准 `notes.md` 模板
```md
# 清洗记录：<书名>
... 0 ~ 6 节模板内容 ...
```

## 15. 自造 demo（不在 notes.md 模板里）
bash samples/demo-books/build.sh
... 三对样本的命令 ...
```

Edit 时注意 §14 代码块内 notes.md 模板的「## 4 完整红线校验」「## 5 Diff 概览」「## 6 可信度评估」都应保留为模板示例的一部分（在内层 fence）。

**P1：`scripts/epub_ai_harness.py --mode cleanup` 行为**

`apply_workflow_mode` 当前实现：

```python
if report.mode == "cleanup":
    report.skills = [s for s in report.skills if s != "$epub-source-intake"]
    if "$epub-layout-auditor" in report.skills:
        report.skills.remove("$epub-layout-auditor")
    report.skills.insert(0, "$epub-layout-auditor")
```

跑在 `samples/demo-books/dist/city-field-notes-before.epub` 上时，`recommended_skills` 输出了 8 个 skill（基本是「全推荐」），其中 `epub-english-typography-optimizer` 出现在中文 demo 上、`epub-vertical-ruby-optimizer` 出现在不含 ruby 的小段落里。问题在于 `inspect_opf` 的 skill 推断在「内容轻微」时偏过度，没有按计划 §3.2 S2-T6 「输出 JSON 增 `mode` 字段；recommended 排序受 mode 影响」的要求做差异化。

**建议**：

- `--mode cleanup` 时，对 `recommended_skills` 做更精细的「按 findings level 排序」：先 error 类 finding 触发的 skill，再 warn、再 info；同时把 `epub-layout-auditor` 永远放第一。
- 文档层面则需要在 `pipeline/skills-matrix.md` 里写明「harness 推荐是候选清单，不是执行顺序」，避免下棒按字面顺序跑。

**P1：`samples/fixtures-tiny/` 是空占位**

计划 §S2-T13 明确每个子目录里要有 `source/` + `build.sh`，输出 < 5KB EPUB，给 `test_validate_text_invariance.py` 引用。当前 7 个子目录都是 `.gitkeep`，自动化测试改用了「在 Python 测试内即时构造 EPUB」（确实可工作）。这并非 bug，但与计划承诺不一致。

**两种处理路线，选一个并写进 `docs/pipeline/decisions.md`**：

- **降级**：在 `samples/fixtures-tiny/README.md` 顶部加一句「设计目标是手工扩展槽位；自动化测试不依赖本目录」（README 已经有类似说法，可以再正式一些），并把计划 §S2-T13 验收条件改为「目录骨架 + README」。
- **补足**：实际产生 7 个最小 fixture，加 `build.sh`，让 `test_validate_text_invariance.py` 引用至少 1~2 个 fixture（`drm-marker` 显然适合，因为 DRM 测试当前是即时构造的）。

### 2.3 P2 问题：Dry-run 约定只是文档契约

`epub-css-layering-optimizer` 等 11 个 SKILL.md 末尾都加了「Dry-run 约定」段，但本仓的 skill 是 markdown 契约，**没有 CLI**，所以 `<skill-invocation> --commit` 仅是「AI 调用 skill 时应自我约束」的提示，不是真实可执行的 CLI flag。这与计划 §S2-T11 的「dry-run 输出 JSON」并不冲突，但目前没有任何 runtime 在校验这件事。

**建议**：

- 在 `docs/pipeline/skills-matrix.md` 顶部加一行说明：「Dry-run 约定是 AI 行为契约，本仓没有强制 CLI；落地时由调用方约束。」
- 或者写一个轻量校验：让 AI 在 commit 前必须把 `dry-run.json` 留在 `work/` 下，并由 pre-commit 检查（更重，慎做）。

### 2.4 P2 问题：「章节锚点 id」红线无脚本覆盖

`docs/final/SPEC-实现约束.md` §10.1 表格写：

```text
| 章节锚点 id | ... | 人工 review + 专项脚本扩展 |
```

但 §10.5 的自动化 gate 表格只列 text / metadata / spine / cover / drm 五项；`--check all` 在 `validate_text_invariance.py` 中也只覆盖这五项。**章节锚点 id 是红线但 CI 不查**，导致 §10.5 「全量红线」与 §10.1 红线集合不完全一致。

**建议两选一**：

- 把 §10.1 表格里这一项的「校验方式」改为「人工 review；自动化 gate 未覆盖（追踪 issue: 章节锚点 id 红线脚本化）」。
- 或者实现一个 `--check anchors` 选项：对 `<aside epub:type="footnote" id="...">` / `<section id="...">` / `<h1 id="...">` 等的 id 做集合对比；deletion 即触发红线。

`samples/demo-books/redline-trap` 反例当前只触发文本红线；可以延伸出一个 `redline-trap-anchor-changed` 反例测试该路径。

### 2.5 续做清单（Stage 2）

按 P0 → P1 → P2 顺序处理。每项都对应一个独立 commit：

1. **fix(guides): 修 cleanup-flow.md §14/§15 结构** — 把 §15 提出 §14 的 notes.md 模板代码块；不动 §14 模板原意。
2. **feat(scripts): cleanup-mode 推荐排序差异化** — 按 finding level 排序 recommended_skills；JSON 输出增 `findings_by_level` 字段。
3. **docs(samples): 决策 fixtures-tiny 是占位还是补足** — 在 `docs/pipeline/decisions.md` 记录决策；按决策更新 `samples/fixtures-tiny/README.md` 或补 fixture。
4. **docs(spec): §10.1 章节锚点口径与 §10.5 自动化范围对齐** — 改文字或加 `--check anchors`。
5. （可选）**docs(pipeline): 写明 Dry-run 约定为 AI 行为契约**。
6. （可选）**docs(guides): asset-optimization.md 是否补 GUI 工具与子集化决策树**。

---

## 3. Stage 3：EPUB Diff Web App（M1.2）

### 3.1 已落地

| 任务 | 落地状态 |
| --- | --- |
| S3-T1 主体 | ✅ `tools/epub-diff/`：index.html + app.js + 5 个 layer JS + 4 个 render JS + 2 个 util + 2 个 parser + fetch-vendor.sh + style.css |
| S3-T2 vendor 抓取 | ✅ pin 版本 `pierre-diffs@1.2.3`、`@zip.js/zip.js@2.8.26`；`assets/vendor/.gitignore = *.js / *.css`；license 入 git |
| S3-T3 diff-tool.md | ✅ 85 行；含安装、demo 文件指引、`file://` 解决方案 |
| S3-T4 根 README 入口 | ✅ |

### 3.2 P0 问题：流式 SHA-256 没有实际流式化

计划 §4.3 / §4.4.4 / README 都明确写：

> 资源层用 `crypto.subtle.digest('SHA-256', ...)` 接 ReadableStream，字节不进 JS heap …
> 峰值内存 ≈ 「单个最大 entry」+「diff 状态」，通常 < 50MB；500MB-1.5GB 的 epub 也能撑。

**实际实现**（`tools/epub-diff/parsers/epub.js` + `util/hash.js`）：

```js
export async function readEntryBuffer(entry) {
  const blob = await entry.getData(new (requireZip()).BlobWriter());
  return blob.arrayBuffer();        // ← 整个 entry 装入 ArrayBuffer
}

// util/hash.js
export async function sha256Buffer(buffer) {
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  ...
}
```

这是「读整个 entry 进 JS heap → 一次性 digest」的**非流式**实现。对中小 EPUB（< 100MB）没问题，但「1.5GB epub」承诺、「单 entry > 200MB 走 streaming 路径」承诺**都不成立**。

**两个选项**：

**A. 实现真正的流式 hash（推荐）**

zip.js 支持 `Stream` writer / 自定义 writer，结合 [`hash-wasm`](https://github.com/Daninet/hash-wasm) 的 `createSHA256()` 的 `update()` / `digest()` API：

```js
import { createSHA256 } from "hash-wasm";

class IncrementalHasher {
  constructor() { this.hasher = null; }
  async start() { this.hasher = await createSHA256(); }
  write(chunk) { this.hasher.update(chunk); }
  end() { return this.hasher.digest("hex"); }
}

// 然后给 zip.js 写一个 EntryWriter，把 chunk 流式喂给 hasher：
class HashWriter {
  async writeUint8Array(chunk) { hasher.write(chunk); }
  async getData() { return hasher.end(); }
}
```

把 `readEntryBuffer` 拆成 `readEntryHash`（仅算 hash，不留 buffer）和 `readEntryThumbnail`（仅在 status=modified 的图片时才读 buffer 用于缩略图）。这样资源层峰值内存 ≈ chunk size（zip.js 默认 64KB），与计划承诺一致。

**B. 把承诺降级**

如果暂不想引入 `hash-wasm`，把 README + diff-tool.md + handbook-expansion-plan.md 全部改成：

- 「v1 实现是非流式：单 entry 全部读入内存后 hash；建议工作大小 < 300MB EPUB / 单 entry < 100MB。」
- 把 §4.3 / §4.4.4 中的流式描述改为「v0.2 follow-up」。
- 在 `docs/pipeline/decisions.md` 加 Q12「流式 hash 是否落地」= 推迟。

**任选一种**，但**不要保留承诺与实际不符的现状**。

### 3.3 P0 问题：@pierre/diffs 实际没接渲染

计划 §4.2.5 / §4.4.3 / §0.1 都明确说核心 diff 渲染用 @pierre/diffs 的 `<pierre-diff>` Web Component。`tools/epub-diff/scripts/fetch-vendor.sh` 也正确抓取了 `@pierre/diffs@1.2.3` 的 dist。

**实际实现**：

- `render/pierre-diff.js` 的 `loadPierreDiffs()` 用 `await import("../assets/vendor/pierre-diffs.js")` 触发了一次注册（捕获 catch fallback），但**注册后没有任何地方使用 `<pierre-diff>` 元素**。
- 同文件的 `renderLineDiff()` 是一个手写 line-by-line table（无 hunk 算法、无 syntax highlight、无 inline char-level diff）。`render/diff-view.js` 五个 layer renderer 都只调 `renderLineDiff()`。
- 结果：style.js 算了 selector add / delete / modify，但 view 里只把整 CSS 文件做了原始行对比；段落级 inline diff（计划 §4.2.6 描述的）也没接 pierre。

**也就是说**：vendor 抓了，目录里有，但运行时根本不用它。

**两种处理路线**：

**A. 真的接 `<pierre-diff>`**

读 `@pierre/diffs` 的 README，在每个层 renderer 切换调用：

```js
function renderLineDiff(beforeText, afterText) {
  const before = beforeText.split(/\r?\n/);
  const after = afterText.split(/\r?\n/);
  return `<pierre-diff lang="css" before-text="${escape(beforeText)}" after-text="${escape(afterText)}"></pierre-diff>`;
}
```

（API 要按 `@pierre/diffs@1.2.3` 文档校对。当前组件名假设；落地时要看 vendor 解出来后的 web component 真实标签名与 props。）

**B. 删 vendor + 在文档里改成「内置 line diff」**

如果不准备真接 @pierre/diffs，那么：

- `fetch-vendor.sh` 不再抓 pierre-diffs（只留 zip.js）。
- `render/pierre-diff.js` 改名 `render/line-diff.js`。
- README / diff-tool.md / handbook-expansion-plan.md §0.1 / §4.2.5 / §4.4.3 / §4.5 全部把 @pierre/diffs 替换成「内置 line diff renderer」。
- `THIRD_PARTY.md` 删除 `@pierre/diffs` 条目。

**建议**：**优先 A**（保留计划的设计意图；视觉效果上 @pierre/diffs 是这个工具的核心卖点）；如果暂时没法接，至少在 README 标明「@pierre/diffs 集成是 v0.2 工作项，v1 退化为简单 line diff」。

### 3.4 其他偏差（P1 / P2）

**P1: `index.html` script 加载顺序**

```html
<script src="assets/vendor/zip.js"></script>
<script type="module" src="app.js"></script>
```

- zip.js 用 `<script>`（UMD 风格）加载，把 `globalThis.zip` 暴露给 app；这是 zip.js minified build 的合法用法，但与计划 §4.5.2 「zip.js 是 ES module」描述不一致。落地选择更稳，没毛病。
- @pierre/diffs 的 `<script>` 标签不在 index.html 中，依赖 `app.js` 的 `loadPierreDiffs()` 动态 import；如果走 §3.3 A 路线，要确认动态 import 后注册的 web component 是否被后续 innerHTML 重新解析。Web component 注册后写入 innerHTML 是支持的（升级会异步触发），但 view-toggle / back-btn 重新 render 时要确保不会重复注册。

**P1: 没有渐进式取消**

`state.cancelled` 仅在 await 后检查；点 Cancel 会等到当前 layer 跑完才生效。对于大 epub 不友好。

**建议**：在 `diffText` / `diffStyle` / `diffResources` 内部循环里每读完一个 entry 就 `if (signal.aborted) throw new DOMException("Aborted", "AbortError")`，并把 `signal` 作为参数传进去。

**P2: 五层渲染缺 Resources 缩略图**

计划 §4.2.7 / §4.4.4 提到「modified 图片生成 200px 缩略图」。当前 `resources.js` 只返回 `{ size, sha256, type, status }`；`diff-view.js` 也只列文件名。

这是 nice-to-have，不阻塞 v1。**建议**：在 `diff-view.js` 的 `renderResources` 里对 `status === 'modified' && type === 'image' && size < 5 * 1024 * 1024` 的项，读 buffer → `URL.createObjectURL(blob)` → `<img>` 并排。

**P2: 暗色模式手动 toggle**

`render/theme.js` 提供 toggle 按钮，但 calc 默认（`color-scheme: light dark` + `prefers-color-scheme`）已经够用。手动 toggle 是 UX 加分，不必动。

**P2: 计划 §4.2.4 提到顶部状态条用「事实描述，不评价」**

落地的 `diff-view.js` 写：

```js
const status = [
  result.layers.metadata.coreChanged ? "Core metadata differs" : "Core metadata identical",
  ...
];
```

与计划一致 ✅。

### 3.5 续做清单（Stage 3）

按 P0 → P1 → P2 顺序：

1. **fix(tools/epub-diff): 流式 SHA-256 落地 OR README/SPEC 降级承诺**（任选一）。
2. **fix(tools/epub-diff): @pierre/diffs 接入渲染 OR vendor 与文档同步移除**（任选一）。
3. **feat(tools/epub-diff): 渐进式 AbortSignal 取消**。
4. **feat(tools/epub-diff): modified image 缩略图**。
5. **docs(tools/epub-diff): 在 README 里写明 vendor pin 版本与升级流程**（README 当前提到了版本号，但没写「怎么升级」/「升级后跑哪些手测」）。

---

## 4. Stage 4：端到端 demo（M1.3）

### 4.1 已落地

| 任务 | 落地状态 |
| --- | --- |
| S4-T0 自造 demo | ✅ `scripts/build_demo_epubs.py` + `samples/demo-books/build.sh` + 3 个子目录 + `dist/*.epub` + `manifest.json` |
| S4-T3 reader-matrix 增 case | ✅ `synthetic-city-field-notes` / `synthetic-paper-garden` + 至少 4 条 expectations（kindle_previewer + ibooks_macos 各 2，全部 warn + pending-version，正确） |
| S4-T4 case study | ✅ `docs/getting-started/05-case-study.md`（47 行；含三对样本说明） |
| S4-T5 THIRD_PARTY.md | ✅ 已加 @pierre/diffs + zip.js + 「Stage 4 cleanup demos 不增加第三方许可」段 |
| S4-T1/T2（公版书） | ❌ **永久搁置**（用户决策；不算缺口） |

执行：

```sh
$ bash samples/demo-books/build.sh         # 生成 6 个 epub
$ python3 scripts/validate_text_invariance.py \
    samples/demo-books/dist/city-field-notes-before.epub \
    samples/demo-books/dist/city-field-notes-after-clean.epub --check all
exit=0
$ python3 scripts/validate_text_invariance.py \
    samples/demo-books/dist/paper-garden-before.epub \
    samples/demo-books/dist/paper-garden-after-clean.epub --check all
exit=0
$ python3 scripts/validate_text_invariance.py \
    samples/demo-books/dist/redline-trap-before.epub \
    samples/demo-books/dist/redline-trap-after-text-changed.epub --check all
exit=1   # ← 预期失败，输出 "text: modified ... block 2: 清洗可以改变样式，但不能改写..."
```

✅ 三对样本均按预期工作。

### 4.2 偏差与建议

**P2: 计划文档 §5.2 / §5.3 / §7.1 Q8 / §8.4 仍残留鲁迅 / 唐诗描述**

用户已决策不做公版书 demo（写进 `docs/pipeline/decisions.md` 第 17 行）。但 `handbook-expansion-plan.md` 当前还保留：

- §5.2 「以下公版书候选保留为历史原计划，首轮暂缓」（已加 hedge 但占用篇幅）
- §5.3 S4-T1 / S4-T2 完整保留鲁迅《呐喊》/《唐诗三百首》drop-in 模板
- §7.1 Q8「Stage 4 完成后看反馈」
- §8.4 「Project Gutenberg #25196 唐诗三百首」（实际 #25196 是《百家姓》，decisions.md 已记录此实测纠正）
- §9.5 commit 计划仍写 `feat(samples): add Lu Xun Nahan e2e cleanup demo`

**建议**：在计划文档加一个 §5.0 决策框：

```markdown
> **2026-05-27 决策更新**：公版书 demo（鲁迅《呐喊》/《唐诗三百首》）永久搁置；
> Stage 4 唯一交付路径是 `samples/demo-books/` 自造样本。
> 下文 §5.2 表中的公版书行、§5.3 S4-T1/S4-T2 任务详情、§7.1 Q8、§8.4 PG 链接、§9.5 commit 列表
> 中的公版书条目仅作历史记录，不再落地。
```

不要删除原内容（保留为「历史决策」可追溯），只在最显眼处加 banner。

**P2: `samples/demo-books/notes.md` 缺第 6/7 节**

每个子目录的 `notes.md` 当前只有「目标 / 覆盖点 / 验证 / Diff 概览」，没有按计划 §S2-T10 §14 给的 7 节模板（含「健康检查 / harness findings / 模式判定 / 清洗步骤 / 完整红线 / Diff 概览 / 可信度评估 / 已知未解决」）。自造 demo 本来就不走真实清洗流程，所以严格按模板填写不一定有意义。

**建议**：保持现状；只在每份 `notes.md` 顶部加一句「本 demo 由 `scripts/build_demo_epubs.py` 直接构造，不经过实际清洗流程，所以省略标准 notes.md 模板里的 harness / 步骤 / 可信度部分」。

### 4.3 续做清单（Stage 4）

1. **docs(plan): 在 handbook-expansion-plan.md §5 顶部加 2026-05-27 决策 banner**。
2. **docs(samples): 在每份 notes.md 顶部说明不走真实流程**。

---

## 5. 跨阶段约束与计划文档收尾

### 5.1 跨阶段约束（计划 §6）— 全部遵守

| 约束 | 状态 |
| --- | --- |
| CLAUDE.md 闭环不可绕 | ✅ 修 SPEC §10 前已先有 `scripts/test_validate_text_invariance.py` 实测；reader-matrix 新 case 都 warn+pending |
| docs/final/ 仍是硬约束 | ✅ §10 是计划层新增，没和既有 §1..§9 冲突 |
| skills frontmatter 不改 | ✅ 抽样 5 个 skill 检查，仅追加内容 |
| templates/epub-style-demo/ 不被清洗 demo 污染 | ✅ 自造 demo 独立目录 |
| EPUB 文本完整性是红线 | ✅ CI 路径 `validate_text_invariance.py --check text` 退出码语义清晰 |
| Web app 不做效果验收 | ✅ index.html / README / about modal 全部写明 |
| Web app 不向服务器发数据 | ✅ index.html `<meta>` 不外联；fetch-vendor 只在本地跑一次 |

### 5.2 依赖最小化（计划 §6.4）

`requirements.txt` 当前只有 `lxml>=4.9`。但 `validate_text_invariance.py` 实际**用标准库 `xml.etree`**，没用 lxml；脚本顶部注释也写「intentionally uses only the Python standard library so it can run before optional XML dependencies are installed」。

**建议**：

- 把 `requirements.txt` 注释清楚：「`lxml` 是推荐依赖，本仓核心红线脚本用标准库；guides 中某些 OPF / XHTML 解析示例用 lxml」。
- 或者更激进：删除 `requirements.txt`（保留 README / guide 里的「按需 `pip install lxml`」即可）。

`decisions.md` 第 15 行已经记录了这件事；只是 `requirements.txt` 自己没说明。

### 5.3 计划文档（`handbook-expansion-plan.md`）收尾建议

计划本身写得详尽（4554 行），但是「计划已被部分落地、部分修订」的状态没在文档内突出。建议在文档开头加一个 **「2026-05-27 落地状态摘要」** 段：

```markdown
> **落地状态摘要（2026-05-27）**：
>
> - M1.0 Stage 1：✅ 完成
> - M1.1 Stage 2：⚠ 完成主线，遗留两处文档/口径偏差，详见 docs/plans/handbook-expansion-review.md
> - M1.2 Stage 3：⚠ 主体完成，流式 hash 和 @pierre/diffs 集成未落地承诺，详见 review
> - M1.3 Stage 4：✅ 用自造样本完成；公版书 demo 永久搁置
> - 续做项的优先级与具体修法见 docs/plans/handbook-expansion-review.md
```

---

## 6. 给下一棒的执行顺序（按 commit 边界）

下棒按本表逐项 commit。每个 commit 前跑 `python3 scripts/validate_skills_basic.py` 与 `python3 scripts/test_validate_text_invariance.py`。

| # | 优先级 | 类型 | commit 题目 | 涉及文件 |
| --- | --- | --- | --- | --- |
| 1 | P0 | fix | `fix(guides): 修复 cleanup-flow.md §14/§15 结构破损` | `docs/pipeline/cleanup-flow.md` |
| 2 | P0 | feat / docs | 二选一：`feat(tools): 用 hash-wasm 实现流式 SHA-256` OR `docs(plan,tools): 把流式 hash 承诺降级为 v0.2 follow-up` | `tools/epub-diff/util/hash.js`、`parsers/epub.js`、`layers/resources.js`、`README.md`、`docs/pipeline/diff-tool.md`、`docs/plans/handbook-expansion-plan.md` |
| 3 | P0 | feat / docs | 二选一：`feat(tools): 接入 @pierre/diffs Web Component` OR `chore(tools): 移除 @pierre/diffs vendor 并把文档写成内置 line diff` | 同上 + `tools/epub-diff/scripts/fetch-vendor.sh`、`THIRD_PARTY.md` |
| 4 | P1 | fix | `fix(scripts): cleanup 模式按 finding level 排序 recommended_skills` | `scripts/epub_ai_harness.py`、`scripts/test_epub_ai_harness.py` |
| 5 | P1 | docs / feat | `docs(pipeline): 决策 fixtures-tiny 占位还是补足` + 按决策更新 | `samples/fixtures-tiny/*`、`docs/pipeline/decisions.md` |
| 6 | P1 | docs / feat | `docs(spec): §10.1 章节锚点口径与 §10.5 对齐` OR `feat(scripts): 加 --check anchors` | `docs/final/SPEC-实现约束.md`、`scripts/validate_text_invariance.py`、`scripts/test_validate_text_invariance.py` |
| 7 | P2 | docs | `docs(pipeline): 说明 Dry-run 约定是 AI 行为契约` | `docs/pipeline/skills-matrix.md`、各 SKILL.md（可选） |
| 8 | P2 | feat | `feat(tools): AbortSignal 渐进式取消 + modified 图片缩略图` | `tools/epub-diff/app.js`、`layers/*.js`、`render/diff-view.js` |
| 9 | P2 | docs | `docs(plan): 在 handbook-expansion-plan.md 顶部加落地状态摘要 + §5 公版书决策 banner` | `docs/plans/handbook-expansion-plan.md` |
| 10 | P2 | docs | `docs(getting-started): 删除 01-first-epub.md 中 <owner> 占位` | `docs/getting-started/01-first-epub.md` |
| 11 | P2 | chore | `chore(deps): 让 requirements.txt 注释清楚或删除` | `requirements.txt` |

最后跑一次完整自检：

```sh
python3 scripts/validate_skills_basic.py
python3 scripts/test_validate_text_invariance.py
python3 scripts/test_epub_ai_harness.py
bash templates/epub-style-demo/build.sh
NEW=$(ls -t templates/epub-style-demo/dist/ | head -1)
bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/"$NEW"
bash scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/"$NEW"
bash samples/demo-books/build.sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/city-field-notes-before.epub \
  samples/demo-books/dist/city-field-notes-after-clean.epub --check all
```

---

## 7. 不变量（review 期间不可逾越）

- 不要碰 `docs/final/` 的硬规则文本，除非有 reader-matrix 实测支撑（review 修复以「口径对齐」为主，不引入新硬规则）。
- 不要破坏现有自造 demo 的红线脚本退出码语义（前两对必须 0、`redline-trap` 必须 1）。
- 不要因为流式 hash 改动而让小 epub（< 100MB）的 review UX 变差。
- 不要替计划文档「补写」用户没决策过的方向（如「v0.2 桌面 app」之类）。

---

## 8. 不在本 review 范围

- `tools/epub-diff/` 的可访问性（aria）/ 键盘导航深度审计 — 留给单独的 a11y review。
- `samples/demo-books/` 自造 EPUB 的 Kindle Previewer / Apple Books 实测复测 — 留给 reader-matrix 实测回写工作流。
- 计划文档之外的 guides（如 `english-fiction-layout.md`、`fonts-css-expansion-plan.md` 等）的内容审计。
- `epub-handbook` 仓库的 CI（GitHub Actions）配置审计。

---

## 9. 文档目录整理与重新分类（P1，必须执行）

### 9.1 现状盘点

当前 `docs/` 下共有 **46 个 markdown / yaml 文件、约 15500 行**，按目录分布：

| 目录 | 文件数 | 行数 | 当前承载内容 |
| --- | --- | --- | --- |
| `docs/final/` | 6 | ~3000 | 工程契约（SPEC、终极手册、速查表、reader-matrix）+ **epub-pro 架构副本（779 行，与上游约束不同维度）** + **retired fixtures index 11 行（疑似遗骸）** |
| `docs/guides/` | 16 | ~3900 | 现行专题指南、扩展计划、流程文档、工具说明 **四种身份混在一起** |
| `docs/source/` | 5 | ~2860 | 早期推导稿、补充材料 |
| `docs/experiments/` | 5 | ~1210 | 复盘与实测笔记 + **「补充 05」(529 行) 错位**（内容是 source 类） |
| `docs/getting-started/` | 9 | ~700 | 入门层 |
| `docs/pipeline/` | 3 | ~90 | 流水线占位（极小） |
| `docs/architecture/` | 1 | ~10 | 仅 README，引用 final/ 里的 epub-pro 文档 |
| 根 README + 索引 | 2 | ~110 | docs/README.md + 主 README |

**主要问题**：

1. **`docs/guides/` 身份混杂**：16 个文件里包含
   - **(a) 现行专题指南**（应保留）：`english-fiction-layout.md`、`classical-modern-layout.md`、`chapter-head-image.md`、`anthology-navigation.md`、`note-box-border-styles.md`、`duokan-footnote-fallback-fix.md`
   - **(b) 历史 / 未来扩展计划**（应迁出）：`handbook-expansion-plan.md`(4554 行)、`handbook-expansion-review.md`(552 行)、`css-layering-plan.md`、`fonts-css-expansion-plan.md`、`demo-scene-expansion-plan.md`、`skills-and-templates.md`
   - **(c) 清洗流水线流程文档**（应迁到 pipeline/）：`cleanup-flow.md`、`cleanup-patterns.md`、`asset-optimization.md`
   - **(d) 工具说明**（应迁到 pipeline/ 或就近）：`diff-tool.md`
   - `docs/guides/README.md` 自己已经感觉到这种混杂（中段写「落地顺序」给的是 (b) 的执行顺序，对 (a) 类用户毫无意义）。

2. **`docs/architecture/` 归位**：`epub-pro 技术架构 v1.md`（779 行）按 `docs/architecture/README.md` 的声明属于「下游 epub-pro 实现仓的参考副本」，**不是 epub-handbook 的对外约束**；放在 `docs/final/` 会让人误以为它和 SPEC 同级。

3. **`retired fixtures index`**（11 行）：是历史小文件，与 reader-matrix.yaml + SCENE_MATRIX 有内容重叠，且没有入口指向它。

4. **`docs/source/EPUB 3 章节扉页与竖排实战 · 补充 05.md`**（529 行）：从命名和内容看是「source 补充」系列（`docs/source/EPUB 3 补充：...` 同名约定），错位到 experiments/。

5. **`docs/source/` vs `docs/experiments/` 边界不清**：source/ 是早期推导稿（已不再演进，但被 final/ 索引引用），experiments/ 应该是「实验 + 复盘记录」（应该按 review-* 文件命名规则）。当前 experiments/ 里前 4 个文件都是 review-* 风格，第 5 个（补充 05）就违反了这个约定。

6. **`docs/pipeline/` 太空**：3 个文件 90 行，把它升级为「流水线主目录」可以承接 (c) (d) 类文档。

7. **`docs/architecture/` 只剩一个 11 行的 README**：完全可以合并到一个新的 `docs/architecture/` 目录，或干脆删除。

### 9.2 目标目录结构（重组后）

```text
docs/
├── README.md                                 # 总索引（重写）
│
├── getting-started/                          # 入门层 — 不动（共 9 文件）
│   ├── README.md
│   ├── 01-first-epub.md  ...  07-faq.md
│   └── glossary.md
│
├── final/                                    # 工程契约层 — 仅保留对外硬约束（4 文件）
│   ├── SPEC-实现约束.md
│   ├── EPUB 3 终极实践手册.md
│   ├── EPUB 3 HTML CSS 属性速查表.md
│   └── reader-matrix.yaml
│
├── guides/                                   # 专题指南 — 只保留场景化实操（6 文件 + README）
│   ├── README.md                             # 重写：只描述 (a) 类
│   ├── english-fiction-layout.md
│   ├── classical-modern-layout.md
│   ├── chapter-head-image.md
│   ├── anthology-navigation.md
│   ├── note-box-border-styles.md
│   └── duokan-footnote-fallback-fix.md
│
├── pipeline/                                 # 批处理流水线 — 集中流程 + 工具（8 文件）
│   ├── README.md                             # 重写：作为流水线主入口
│   ├── decisions.md                          # 不动
│   ├── skills-matrix.md                      # 不动
│   ├── cleanup-flow.md                       # ← 从 pipeline/cleanup-flow.md 迁
│   ├── cleanup-patterns.md                   # ← 从 pipeline/cleanup-patterns.md 迁
│   ├── asset-optimization.md                 # ← 从 guides/ 迁
│   └── diff-tool.md                          # ← 从 pipeline/diff-tool.md 迁
│
├── plans/                                    # 新建：历史 / 在做 / 未来扩展计划（7 文件）
│   ├── README.md                             # 新建：说明 plans 的角色与读取顺序
│   ├── handbook-expansion-plan.md            # ← 从 guides/ 迁
│   ├── handbook-expansion-review.md          # ← 从 guides/ 迁（本文件自身！）
│   ├── css-layering-plan.md                  # ← 从 guides/ 迁
│   ├── fonts-css-expansion-plan.md           # ← 从 guides/ 迁
│   ├── demo-scene-expansion-plan.md          # ← 从 guides/ 迁
│   └── skills-and-templates.md               # ← 从 guides/ 迁（仓库维护说明）
│
├── architecture/                             # 新建：下游 / 周边架构参考（2 文件）
│   ├── README.md                             # 合并自 docs/architecture/README.md
│   └── epub-pro-v1.md                        # ← 从 final/ 迁，去掉文件名中的空格
│
├── source/                                   # 推导稿（6 文件）
│   ├── EPUB 3 制作完全参考手册.md
│   ├── EPUB 3 补充：...（4 个补充）
│   └── EPUB 3 章节扉页与竖排实战 · 补充 05.md  # ← 从 experiments/ 迁
│
└── experiments/                              # 复盘 / 实测（仅 4 个 review-*）
    ├── classical-parallel-epub-analysis-20260525.md
    ├── review-6d50071.md
    ├── review-ab26e31-kindle-app-20260521.md
    └── review-codex-modify-files-to-execute-in-order-20260525.md

# 删除：
# - retired fixtures index   （11 行，无入口引用；若内容必须保留，先合并到 reader-matrix.yaml 注释或 SCENE_MATRIX）
# - docs/architecture/            （目录整体并入 docs/architecture/）
```

**计数变化**：

- `docs/guides/`：16 → 7（含 README）— 身份单一为「场景化专题实操」。
- `docs/pipeline/`：3 → 8 — 真正承接「批处理流水线」名号。
- `docs/plans/`：0 → 7 — 把 5800+ 行历史扩展计划集中，让 guides 瘦身。
- `docs/architecture/`：0 → 2 — 给下游架构副本一个明确位置。
- `docs/final/`：6 → 4 — 严格成为「上游硬约束」。
- `docs/experiments/`：5 → 4 — 名实相符。
- `docs/source/`：5 → 6 — 收回补充 05。
- `docs/architecture/`：1 → 0（删除）。

### 9.3 分类原则（写进重组后的 docs/README.md）

未来新增文档前，必须先按以下决策树定位：

```text
文档是给谁看的、什么角色？
│
├── 第一次接触本仓的人 / AI → docs/getting-started/
│
├── 对外硬约束（违反等于事故）→ docs/final/
│   只有 SPEC、终极手册、速查表、reader-matrix.yaml 这四类
│
├── 针对某种「场景」的实操指南（英文小说、文白对照、章首图…）
│   答得出「这本书属于哪一类排版」→ docs/guides/
│
├── 围绕「拿到一本现成 epub 怎么处理」的流程 / 工具 / 模式
│   → docs/pipeline/
│
├── 历史 / 在做 / 未来的扩展计划、阶段评审、维护说明
│   特征：包含「Stage」「计划」「TODO」「重构」「迁移」
│   → docs/plans/
│
├── 下游 / 周边项目的架构副本、对接说明
│   → docs/architecture/
│
├── 早期推导稿、补充材料（已不再演进、只作历史归档）
│   → docs/source/
│
└── 某次实测 / 复盘 / 实验的快照
    命名约定：review-<commit-or-tag>-<date>.md 或 <topic>-<date>.md
    → docs/experiments/
```

**强约束**：

1. `docs/final/` 只能放上述 4 类硬约束；新增任何文件前必须能在 CLAUDE.md 优先级 2 里被引用。
2. `docs/guides/` 只放「场景指南」；包含「plan」「review」「flow」「pattern」「tool」字样的，应在 plans/ 或 pipeline/。
3. `docs/plans/` 文件**不直接驱动行为**，只是规划与审稿；当一个 plan 完全落地，可以归档到 plans/archive/（暂不创建，等真有归档需求再加）。
4. `docs/experiments/` 文件名必须带日期或 commit hash，便于按时间检索。
5. 新增目录前先在 `docs/pipeline/decisions.md` 写入决策；不要默默 mkdir。

### 9.4 迁移执行步骤

按以下顺序执行；每步一个独立 commit，便于回滚。

#### Step 1：建新目录骨架（commit: `chore(docs): create plans/ and architecture/ directories`）

```sh
mkdir -p docs/plans docs/architecture
# 这一步只建空目录 + .gitkeep，commit 后看不到效果，但保证后续 git mv 有目标
touch docs/plans/.gitkeep docs/architecture/.gitkeep
```

实际上 `git mv` 会自动创建目录；这一步可以省，但保留作为「先看到结构骨架」的视觉锚点。

#### Step 2：迁移 guides → plans（commit: `chore(docs): move expansion plans from guides/ to plans/`）

```sh
git mv docs/plans/handbook-expansion-plan.md     docs/plans/
git mv docs/plans/handbook-expansion-review.md   docs/plans/
git mv docs/plans/css-layering-plan.md           docs/plans/
git mv docs/plans/fonts-css-expansion-plan.md    docs/plans/
git mv docs/plans/demo-scene-expansion-plan.md   docs/plans/
git mv docs/plans/skills-and-templates.md        docs/plans/
```

迁完后**必须**更新引用：

- `docs/README.md` 索引
- `docs/guides/README.md` 删除迁出文件的描述
- 根 `README.md` 协作段如有引用
- `CLAUDE.md` 优先级表中所有 `docs/guides/<迁出文件>.md` → `docs/plans/<同名>.md`
- `docs/getting-started/README.md` 中「读完入门去哪」的 `fonts-css-expansion-plan.md` 链接
- 任何 `skills/*/SKILL.md` 内对这些 plan 文档的引用（grep for migrated plan-document old paths）

新建 `docs/plans/README.md`：

```markdown
# 计划与审稿

这个目录放：

- **扩展计划**：当前在做或将来要做的多阶段重构 / 新功能。
- **审稿 / review**：对某个阶段的实测对照。
- **维护说明**：仓库自身工作流的说明（skills 维护、模板更新）。

本目录文档**不直接驱动行为**；规则的最终来源仍然是 docs/final/。

## 当前计划

- handbook-expansion-plan.md — 三层手册 + 清洗流水线 + diff 工具的 4 Stage 计划（2026-05-26）
- handbook-expansion-review.md — 上面计划的落地审稿（2026-05-27）
- css-layering-plan.md — Styles/ 八层 CSS 骨架计划
- fonts-css-expansion-plan.md — fonts.css 系统字体优先策略
- demo-scene-expansion-plan.md — demo 模板 10–17 场景扩展
- skills-and-templates.md — skills 维护方式与模板目录约定

## 已归档

暂无。落地完成且无续做的计划，按需移到 plans/archive/。
```

#### Step 3：迁移 guides → pipeline（commit: `chore(docs): move cleanup flow/patterns/asset/diff to pipeline/`）

```sh
git mv docs/pipeline/cleanup-flow.md      docs/pipeline/cleanup-flow.md
git mv docs/pipeline/cleanup-patterns.md  docs/pipeline/cleanup-patterns.md
git mv docs/pipeline/asset-optimization.md     docs/pipeline/asset-optimization.md
git mv docs/pipeline/diff-tool.md         docs/pipeline/diff-tool.md
```

**文件名变化**（去掉 `epub-` 前缀，因为 pipeline/ 上下文已经隐含 epub）：

- `cleanup-flow.md` → `cleanup-flow.md`
- `cleanup-patterns.md` → `cleanup-patterns.md`
- `diff-tool.md` → `diff-tool.md`
- `asset-optimization.md` 保持原名（不带 epub- 前缀）。

迁完后必须更新引用（grep 这些旧路径）：

- `docs/final/SPEC-实现约束.md` §10.5 中的 `tools/epub-diff/` 引用不变；但其他地方如果引用了 pipeline/cleanup-flow 要改。
- `docs/getting-started/04-skills.md`、`05-case-study.md`、`06-test-your-own.md` 都引用了 pipeline/cleanup-flow.md。
- `docs/pipeline/README.md`、`docs/pipeline/skills-matrix.md`、`docs/pipeline/decisions.md` 互相引用。
- `tools/epub-diff/README.md` 引用 `docs/pipeline/diff-tool.md`。
- `samples/demo-books/README.md`、`samples/demo-books/*/notes.md` 引用 cleanup-flow / diff-tool。
- 各 SKILL.md 中对 cleanup-flow / cleanup-patterns / asset-optimization 的链接。

更新 `docs/pipeline/README.md`，把它从「占位」改写为「主入口」：

```markdown
# 批处理流水线

> 把已有 epub 批量处理（清洗、改造、对比、回写）的工作流文档。

## 核心文档（按读取顺序）

1. [cleanup-flow.md](../pipeline/cleanup-flow.md) — 流水线主流程（健康检查 → 红线 gate → diff review → reader-matrix 回写）
2. [cleanup-patterns.md](../pipeline/cleanup-patterns.md) — 典型脏 EPUB 模式识别与 skill 推荐顺序
3. [asset-optimization.md](../pipeline/asset-optimization.md) — 图片与字体优化（清洗流程 §4 附件）
4. [diff-tool.md](../pipeline/diff-tool.md) — EPUB Diff Web App 使用说明
5. [skills-matrix.md](../pipeline/skills-matrix.md) — 14 个 skill 在清洗 / 新书流程中的角色
6. [decisions.md](../pipeline/decisions.md) — Stage 落地决策与偏差记录

## SPEC 对应

清洗流程的硬规则在 [../final/SPEC-实现约束.md §10](../final/SPEC-实现约束.md)。
```

#### Step 4：迁移 final → architecture（commit: `chore(docs): move epub-pro architecture out of final/`）

```sh
git mv "docs/architecture/epub-pro-v1.md" docs/architecture/epub-pro-v1.md
git mv docs/architecture/README.md             docs/architecture/README.md
rmdir docs/architecture
```

`docs/architecture/README.md` 改写为：

```markdown
# 周边 / 下游架构参考

此目录收录 epub-handbook 之外、但需要在仓内保留副本的架构说明。

## 当前条目

- [epub-pro-v1.md](../architecture/epub-pro-v1.md) — 下游 `epub-pro` 实现仓的技术架构正本副本。

## 维护约定

- handbook 是上游规范源；epub-pro 是下游实现。
- 下游架构更新时，同步更新本目录的副本，并在文件顶部注明同步日期与上游 commit。
- 本目录文件**不进 docs/final/**：它们不是 epub-handbook 的对外硬约束。
```

注意：`docs/architecture/epub-pro-v1.md` 里如果有 internal `[XX](../final/SPEC-...)` 之类相对链接，路径不变（深度相同）；如果有 ``[a] -> xx.md`` 之类同目录链接需校对。

#### Step 5：删除 retired fixtures index（commit: `chore(docs): retire retired fixtures index`）

```sh
git rm retired fixtures index
```

先 grep 全仓引用：

```sh
grep -rn "retired fixtures index" --include="*.md" --include="*.yaml" .
```

把任何引用替换为 `docs/final/reader-matrix.yaml` 或 `templates/epub-style-demo/SCENE_MATRIX.md`（按引用上下文判断）。如果 grep 出 0 处引用，直接删；如果有引用，先迁移这一两行内容到 reader-matrix 注释或 SCENE_MATRIX，再删。

#### Step 6：experiments → source 修正（commit: `chore(docs): move 章节扉页与竖排实战 to source/`）

```sh
git mv "docs/source/EPUB 3 章节扉页与竖排实战 · 补充 05.md" "docs/source/EPUB 3 章节扉页与竖排实战 · 补充 05.md"
```

更新引用（grep 文件名）。

#### Step 7：重写顶层索引（commit: `docs: rewrite docs/README and docs/guides/README after restructure`）

- `docs/README.md`：按 9.2 的新结构重排，加 9.3 决策树作为「我该把新文档放哪」段。
- `docs/guides/README.md`：删除已迁出文件的描述，只保留 6 个场景指南；去掉中段那个「落地顺序」（那是给 plans/ 的，不是给 guides/ 的）。
- 根 `README.md`：第二个表「我要做什么？」表里的链接不需要改（指向目录而不是文件）。
- `CLAUDE.md` 优先级表里如果有具体文件路径，按新位置更新（grep `docs/plans/handbook|docs/plans/css-layering|docs/plans/fonts-css-expansion|docs/plans/demo-scene-expansion|docs/plans/skills-and-templates|docs/pipeline/cleanup\|docs/pipeline/diff\|docs/pipeline/asset-optimization\|docs/architecture/epub-pro\|retired fixtures index\|docs/architecture\|docs/experiments/EPUB 3 章节扉页`）。

#### Step 8：跑链接自检（commit-less，本地验证）

```sh
# 找出所有 markdown 内的相对链接，逐条 stat
python3 - <<'PY'
import re, pathlib
broken = []
for md in pathlib.Path("docs").rglob("*.md"):
    text = md.read_text(encoding="utf-8")
    for match in re.finditer(r"\[[^\]]+\]\(([^)#]+\.(?:md|yaml|sh|py|css|js|xhtml))(?:#[^)]*)?\)", text):
        target = (md.parent / match.group(1)).resolve()
        if not target.exists():
            broken.append(f"{md}: -> {match.group(1)}")
for line in broken:
    print(line)
print(f"\n{len(broken)} broken links" if broken else "\nAll links OK")
PY
```

任何 broken 都修了再 push。

#### Step 9：commit `docs/pipeline/decisions.md` 加迁移记录

```markdown
## 2026-05-?? 文档重组

按 docs/plans/handbook-expansion-review.md §9 重组：

- docs/guides/ 拆为 guides（场景）+ plans（计划）+ pipeline（流程）。
- docs/final/ 收回 epub-pro 架构副本到 docs/architecture/。
- docs/architecture/ 并入 docs/architecture/。
- retired fixtures index 删除。
- docs/source/EPUB 3 章节扉页与竖排实战 · 补充 05.md 归位到 source/。
- docs/README.md 加分类决策树。
```

### 9.5 验收

执行完 9.4 步骤后，应满足：

```sh
# 1. 新目录存在
test -d docs/plans
test -d docs/architecture
test ! -d docs/architecture

# 2. guides/ 瘦身
test "$(ls docs/guides/*.md | wc -l)" -le 7   # README + 6 场景指南

# 3. pipeline/ 扩张
test "$(ls docs/pipeline/*.md | wc -l)" -ge 7

# 4. final/ 严格 4 个对外硬约束
test "$(ls docs/final/ | wc -l)" -eq 4

# 5. 旧路径全无遗留
! grep -rn "migrated-plan-old-paths" . --include="*.md" --include="*.yaml" --include="*.py" --include="*.sh"
! grep -rn "docs/pipeline/cleanup\|docs/pipeline/diff\|docs/pipeline/asset-optimization" . --include="*.md" --include="*.yaml" --include="*.py" --include="*.sh"
! grep -rn "retired-final-fixtures-path" . --include="*.md" --include="*.yaml"

# 6. 链接自检
python3 -c "import pathlib,re; [print(md, m.group(1)) for md in pathlib.Path('docs').rglob('*.md') for m in re.finditer(r'\\[[^\\]]+\\]\\(([^)#]+\\.(?:md|yaml))(?:#[^)]*)?\\)', md.read_text('utf-8')) if not (md.parent / m.group(1)).resolve().exists()]"
# 期望输出为空

# 7. markdownlint
markdownlint-cli2 'docs/**/*.md' 'README.md' 'CONTRIBUTING.md' 'CLAUDE.md'
```

### 9.6 不要做的事

- **不要在 9.4 之外再创建 docs/standards/、docs/manuals/、docs/notes/ 等新目录**。本方案的 7 个目录（getting-started / final / guides / pipeline / plans / architecture / source / experiments）已经覆盖所有文档类型。
- **不要把 plans/ 文档「精简」**。它们是历史规划，价值在于完整性；review 文档说什么，原文都要能查到。
- **不要把 source/ 里的早期推导稿合并到 final/**。它们是推导过程而不是约束，合并会污染 final/ 的对外稳定性。
- **不要把本 review 文档（handbook-expansion-review.md）拆散**。它需要作为一份完整审稿留下，便于下棒按 §6 commit 表逐项执行。
- **不要重命名 docs/getting-started/ 的 01..07 numbered 文件**。它们已被根 README、README、glossary、case-study 多处引用，重命名会引起链接雪崩。

### 9.7 与本 review §6 续做表的整合

§6 续做表已经列了 11 个 commit。文档重组（§9.4）应在它们之前完成，因为：

- §6 #1（修复 cleanup-flow.md §14/§15）涉及 `docs/pipeline/cleanup-flow.md`；如果先迁到 `docs/pipeline/cleanup-flow.md` 再修，git 历史更干净。
- §6 #2 / #3（流式 hash + pierre-diffs）涉及 `tools/epub-diff/README.md` 与 `docs/pipeline/diff-tool.md`；如果先迁到 `docs/pipeline/diff-tool.md` 再改，可避免一次冲突。

**推荐合并顺序**：

| 阶段 | 内容 | commit 数 |
| --- | --- | --- |
| A. 重组（§9） | Step 1..9 | 8 |
| B. 修复（§6） | #1 → #11 | 11 |
| C. （可选）归档 | 把 `docs/plans/handbook-expansion-plan.md` + `handbook-expansion-review.md` 移到 `docs/plans/archive/` | 1 |

阶段 C 只在 §6 全部 P0 / P1 都修完、且 plan 不再有可执行项时执行。当前不要做。

---

## 10. 不在本 review 范围（重新声明）

- `tools/epub-diff/` 的可访问性（aria）/ 键盘导航深度审计 — 留给单独的 a11y review。
- `samples/demo-books/` 自造 EPUB 的 Kindle Previewer / Apple Books 实测复测 — 留给 reader-matrix 实测回写工作流。
- 计划文档之外的 guides（如 `english-fiction-layout.md`、`fonts-css-expansion-plan.md` 等）的内容审计。
- `epub-handbook` 仓库的 CI（GitHub Actions）配置审计。
- 文档**内容**层面的合并 / 重写（§9 只做目录与命名的整理，不动文件内部内容；除非是已被识别为 retired 的 `retired fixtures index`）。

---

> 本 review 把当前分支的实测结果与计划文档对照，标记了可验证的偏差。任何与本 review 冲突的实测结果**以实测为准**；发现新偏差请在 `docs/pipeline/decisions.md` 续写区记录，不要直接重写本文件或 plan 文件。
>
> §9 的目录重组在执行后会让本文件自身从 `docs/plans/handbook-expansion-review.md` 移动到 `docs/plans/handbook-expansion-review.md`。届时所有外部对本文件的引用都需要同步更新（README、CLAUDE.md、PR 描述等）。
