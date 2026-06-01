# 面向新人 / 不懂 EPUB 用户的可用性 Review（2026-06-01）

## 这份文档是什么

本仓自我定位为「面向新人、不会 EPUB 的人」的工具，覆盖**介绍、排版说明、排版优化、格式清洗**。
本文是一次以「新人第一次接触本仓」为视角的可用性审稿，列出**所有需要修改的点**和**精确改法**，供后续由其他模型/贡献者执行。

执行者无需回看任何对话，本文自包含。每条问题给出：严重度、证据（`file:line`）、修改方案（能给精确 `现状 → 改为` 的都给了）、验收标准。

> 重要边界（执行前必读）：
> - **不要为统一措辞重写历史**。`docs/plans/*`、`docs/experiments/*` 是历史计划与实验快照，AGENTS.md 明确「历史计划和实验快照只在任务明确要求时修改」。本文涉及的 renumber / 改链接**只动 active 文档**（README、`docs/README.md`、`docs/getting-started/` 正文），不动 `docs/plans/` 里对旧文件名的历史引用。
> - 任何触及 `docs/final/` 硬规则、reader-matrix、SPEC、手册、速查表、demo 的改动，都要走 AGENTS.md 的实测闭环；本文大部分条目是**入门层 / README / 导航**的可用性修复，不改硬规则，但仍需跑「最小验证矩阵」。
> - 改完按本文末尾「统一验收」跑校验。

---

## 总览（按严重度）

| ID | 严重度 | 问题 | 类型 | 是否改硬规则 |
| --- | --- | --- | --- | --- |
| R1 | 🔴 高 | 定位矛盾：声称面向新人，却明确「不是初级排版课」，开篇即术语墙 | 内容/定位 | 否 |
| R2 | 🔴 高 | 讲「排版」的手册全仓 0 张图（无 before/after、无渲染截图） | 内容缺口 | 否（截图入仓需配合实测闭环） |
| R3 | 🟠 中 | 入门文件编号（01–08）与推荐阅读顺序冲突；README 内有两份重复清单 | 导航 UX | 否 |
| R4 | 🟠 中 | 「从零做一本新书」其实无 scaffold / 无最小空白模板，demo 是大杂烩 | 功能缺口 | 否（新增模板需走 demo 约定） |
| R5 | 🟠 中 | `docs/plans`(9278 行)+`docs/source`(3390 行) 过程产物淹没真正手册 | 仓库整洁 | 否 |
| R6 | 🟡 低 | `docs/source` 与 `docs/final` 各有一本「完全/终极手册」，新人分不清权威 | 导航/命名 | 否 |
| R7 | 🟡 低 | 两套 CSS 分层方案（模板 8 层 vs 样本 4 文件）未说明区别，易混淆 | 文档澄清 | 否 |
| R8 | 🟡 低 | README 两处写「15 个专项 skill」，实为 13 专项 + 2 主入口 | 事实/措辞 | 否 |
| R9 | 🟡 低 | reader-matrix 实测 pass 仅 7 条、warn 22 条，与「全部实测回写」口号不相称 | 诚实度/期望管理 | 否（不可伪造 pass） |
| R10 | 🟡 低 | README 过载，且「一条命令清洗」承诺被紧随其后的长手动流程削弱 | 信息架构 | 否 |

> 已排除的「伪 bug」：`05-case-study.md` 里的 `tables.css` **不是错误**——它准确描述了自造样本 `city-field-notes`（见 `samples/demo-books/city-field-notes/notes.md:17`）的 CSS 拆分，与模板的 8 层 canonical 方案是两套不同产物。相关澄清归入 R7，不要当成笔误去改。

---

## R1 — 定位矛盾：声称面向新人，却拒绝新人，开篇即术语墙

**严重度**：🔴 高

**证据**：
- `README.md:3` 开篇第一句即术语墙：「围绕『硬约束 + 自造 demo + 阅读器实测 + 自动化 skill』四件套构建……」——`harness`、`red-line gate`、`reader-matrix`、`A-lite` 均未在出现处解释。
- `README.md:9-11` 受众实为「工程师 / 编辑 / maintainer」。
- `README.md:322` 明确「不是初级排版课」。
- 全仓没有一篇白话「什么是 EPUB / 为什么同一本书在不同阅读器会崩 / 这仓库解决什么痛点」的引子；`docs/getting-started/02-anatomy.md:3` 直接进入 zip 目录结构。

**为什么是问题**：真正不懂 EPUB 的人在 README 第一段和 02-anatomy 第一段就被劝退。「介绍」这一层实质缺位。

**修改方案（分两步，建议都做）**：

1. 新增白话引子页 `docs/getting-started/00-what-is-epub.md`，定位「完全不懂 EPUB 的人的第 0 篇」。建议大纲（每段都用日常语言，**先不要出现 harness / red-line / A-lite 等术语**，需要时只用一句白话解释并链到 `glossary.md`）：
   - **EPUB 是什么**：一句话——「一个装着 HTML+CSS 的 zip 包，电子书阅读器读它来显示文字」。
   - **为什么需要它/解决什么痛点**：同一本书在 Apple Books、Kindle、多看上排版会不一样、会崩；这仓库就是教你做「在多个阅读器都不崩」的中文 EPUB，以及把别人做坏的 EPUB 清洗干净。
   - **这仓库帮你做两件事**：①从零学着排一本；②把现成 EPUB 清洗/精排。各给一句话 + 一个入口链接（分别指向 `01-first-epub.md` 和 `docs/pipeline/cleanup-flow.md`）。
   - **你不需要先会什么**：会用命令行复制粘贴即可；术语遇到不懂的查 `glossary.md`。
   - 末尾「下一步 → [01-first-epub.md](01-first-epub.md)」。
   - 篇幅控制 ≤ 120 行。

2. 把 `00-what-is-epub.md` 接入两处导航：
   - `docs/getting-started/README.md` 的「如何使用」清单和「推荐阅读顺序」清单，在最前面加一项（见 R3，两处清单会被合并，加在合并后清单首位）。
   - `docs/README.md:7-16` 入门层清单首位加 `- [00-what-is-epub.md](getting-started/00-what-is-epub.md)`。
   - `README.md:332-334` 文档地图「入门」行无需改（指向目录即可）。

3. 软化 `README.md:322` 的措辞，避免和「面向新人」打架。
   - 现状：`- 不是初级排版课。`
   - 改为：`- 不是零基础排版速成课：会教你做不崩的 EPUB，但不覆盖通用网页 CSS 基础。完全没接触过的人请先读 [docs/getting-started/00-what-is-epub.md](docs/getting-started/00-what-is-epub.md)。`

4. README 开篇加一句「人话」导语，放在 `README.md:3` 那段之前（不要删原段，原段是给工程师看的概述）。建议在第 1 行标题下、第 3 行那段之前插入：
   ```markdown
   > 第一次接触、还不懂 EPUB？先读 [docs/getting-started/00-what-is-epub.md](docs/getting-started/00-what-is-epub.md)，5 分钟搞懂这仓库能帮你做什么。
   ```

**验收**：
- `docs/getting-started/00-what-is-epub.md` 存在且 ≤ 120 行，正文不含未解释的 `harness/red-line/A-lite`。
- `docs/README.md`、`docs/getting-started/README.md`、`README.md` 均有指向它的链接，且链接可达（相对路径正确）。

---

## R2 — 讲「排版」的手册全仓 0 张图

**严重度**：🔴 高

**证据**：
- `find docs -type f \( -iname '*.png' -o -iname '*.jpg' -o -iname '*.svg' -o -iname '*.webp' -o -iname '*.gif' \)` → **空**。
- `docs/final/reader-matrix.yaml` 的实测结论也只有文字 `status`，无截图 artifact 入仓。

**为什么是问题**：首字下沉、图文环绕、弹注、竖排、章首图都是**视觉概念**，纯文字 + CSS 代码对新人门槛极高——他们不知道「排好」长什么样，也无法判断自己改对没有。

**修改方案**：
- 这条**不能凭空生成图片**，需要真实渲染产物。按 AGENTS.md「阅读器实测闭环」：用 `templates/epub-style-demo/` 已有场景，在目标阅读器里截图，落盘到一个新目录（建议 `docs/getting-started/images/` 或 `docs/final/screenshots/`，目录名与放置规则需在执行时确认并在 `docs/README.md` 的目录说明里登记）。
- 最小可行集（优先级从高到低，每张配一句 caption + 对应 demo 场景文件名）：
  1. 普通中文正文（`Text/01-body.xhtml`）在 Apple Books 的样子——作为「正常」基准。
  2. 标准弹注点开前/点开后（`Text/02-ruby-note.xhtml`）。
  3. 图文环绕 before/after（`Text/17-image-layout.xhtml`）。
  4. 章首/章首图（`Text/11-chapter-opening.xhtml` / `Text/20-chapter-head-image.xhtml`）。
  5. 竖排（`Text/14-vertical-body.xhtml`）。
- 截图必须可追溯：在图片旁或一个 `images/README.md` 里记录 reader id+版本、case id、artifact、截图日期（与 reader-matrix 记录口径一致）。
- 把至少前 2 张接入 `02-anatomy.md` 或新的 `00-what-is-epub.md`，让新人第一眼看到「正常 vs 异常」。

**这条需要人参与**（要装阅读器、截图）。执行的模型应：
- 若环境允许构建 demo + 截图，则按上面做；
- 若环境无法截图，则**不要伪造**，而是在 review 回执里标为「待人工补图」，并先把图片占位目录、`images/README.md` 记录模板和文中引用位置准备好。

**验收**：
- 至少 2 张真实截图入仓并被入门页引用；或在无法截图时，留好占位结构 + 明确的 TODO（含每张图对应的 demo 场景与目标 reader）。

---

## R3 — 入门文件编号与推荐阅读顺序冲突 + 两份重复清单

**严重度**：🟠 中

**证据**：
- 文件名编号是 `01..08`，但推荐阅读顺序是 `01,02,03,06,04,07,08,05,glossary`：
  - `docs/getting-started/README.md:7-15`（「如何使用」清单）
  - `docs/getting-started/README.md:59-67`（「推荐阅读顺序」清单，与上面**完全重复**，同一顺序写了两遍）
- 即 `06-test-your-own` 推荐排在 `04-skills` 前、`05-case-study` 推荐排到倒数第二。新人在文件浏览器里按文件名 `01→08` 自然顺序读 = 读错。
- `docs/README.md:11-15` 的入门层清单按**文件名顺序**列（04,05,06,07,08），与推荐顺序又不一致。

**修改方案**：二选一，**推荐方案 A（低风险）**。

### 方案 A（推荐，低风险，不改文件名）

把「编号只是 ID、不代表阅读顺序」这件事讲明，并消除重复清单。

1. 在 `docs/getting-started/README.md` 顶部（标题下、`:5` 那段「如何使用」标题前）加一行说明：
   ```markdown
   > 文件名前缀（01、02…）只是稳定 ID，**不代表阅读顺序**。请按下面的推荐顺序读。
   ```
2. 删除两份重复清单中的一份。保留一份「推荐阅读顺序」清单即可。具体：删掉 `:5-15` 的「## 如何使用」整段（含标题和 9 行清单），保留 `:55-67` 的「## 推荐阅读顺序」；并把 R1 新增的 `00-what-is-epub.md` 加为该清单第 1 项。
   - 合并后清单顺序应为：`00-what-is-epub → 01-first-epub → 02-anatomy → 03-readers → 06-test-your-own → 04-skills → 07-faq → 08-epub2-epub3-compatibility → 05-case-study → glossary`。
3. `docs/README.md:7-16` 入门层清单改为与上面**同一推荐顺序**（而不是文件名顺序），并加入 `00-what-is-epub.md`。

### 方案 B（彻底但高风险，需改文件名）

把文件名重排成阅读顺序：`06→04`、`04→05`、`07→06`、`08→07`、`05→08`（即 test-your-own=04、skills=05、faq=06、epub2-compat=07、case-study=08）。
- 必须同步更新所有 **active** 入站链接：`docs/README.md`、`docs/getting-started/README.md`、以及 `docs/getting-started/*.md` 正文里的互链。
- 执行前先跑全仓引用扫描，逐一改 active 文档（**不要动 `docs/plans/*` 的历史引用**）：
  ```sh
  grep -rn "04-skills\|05-case-study\|06-test-your-own\|07-faq\|08-epub2" --include="*.md" . | grep -v "docs/plans/"
  ```
- 风险：链接面广、易断链。除非有明确收益，否则选方案 A。

**验收**：
- `docs/getting-started/README.md` 只剩一份顺序清单，且与 `docs/README.md` 入门层清单顺序一致。
- 若选 B：仓库内（除 `docs/plans/`）无指向旧文件名的死链；`docs/getting-started/` 内所有相对链接可达。

---

## R4 — 「从零做一本新书」无 scaffold / 无最小空白模板

**严重度**：🟠 中

**证据**：
- `docs/getting-started/01-first-epub.md` 实际流程是「build 现成 demo → 改一行字 → diff」，并没有「新建一本自己的书」。
- `README.md:35`「从零做一本新书」入口指向 `templates/epub-style-demo/`，但该模板是 **22 个 XHTML 的大杂烩**（竖排/海报/MathML/多看 fallback/数学/英文小说等全塞一起，见 `templates/epub-style-demo/OEBPS/Text/` 列表），不是最小起点。新人要自己从中逆向裁剪。
- `scripts/` 下无「新建空白书」生成器（有 demo build、epub3 转换、清洗 pipeline，但没有 scaffold）。

**修改方案**：二选一。

### 方案 A（文档先行，零代码，低风险）

在 `01-first-epub.md` 增一节「做你自己的最小书」，明确告诉新人：
- 复制 `templates/epub-style-demo/` 后**该删哪些文件**只保留最小骨架（mimetype、META-INF/container.xml、OPF、nav.xhtml、toc.ncx、一个 `Text/01-body.xhtml`、一个 `Styles/base.css`）；
- 列出删完后必须在 `package.opf` 的 manifest/spine 和 `nav.xhtml` 里同步删掉的条目（点名要删的 manifest item id / spine itemref），并给一句「删完跑 `xmllint --noout package.opf nav.xhtml toc.ncx` 验证」。
- 这节要给**可照抄的最小 OPF / nav / body 片段**，让新人不依赖大杂烩也能起步。

### 方案 B（提供 scaffold 脚本，中风险）

新增 `scripts/epub_new_book.py`（仅标准库，与现有脚本一致），生成最小可打包 EPUB3 骨架到指定目录：mimetype + container.xml + 最小 OPF（带占位 metadata）+ nav.xhtml + toc.ncx + 一个正文 + base.css。
- 必须配套测试 `scripts/test_epub_new_book.py`，并保证产出能过 `scripts/validate-epub-style-demo.sh` 的通用检查或至少 `xmllint`/epubcheck。
- 在 README「我要做什么」表和 `01-first-epub.md` 接入该入口。
- 走 AGENTS.md「最小验证矩阵」的 Python 脚本一栏。

**建议**：先做方案 A（即时缓解），方案 B 作为后续增强单独立项。

**验收**：
- `01-first-epub.md` 有明确「如何从模板裁出最小书」的可照抄步骤；或新增 scaffold 脚本 + 测试 + 文档入口。
- `README.md:35` 的「从零做一本新书」入口指向这条新内容，而不是直接甩给大杂烩模板。

---

## R5 — 过程产物淹没真正的手册

**严重度**：🟠 中

**证据（行数）**：
- `docs/plans` 共 **9278 行 / 10 个文件**，其中 `handbook-expansion-plan.md` 单文件 **4265 行**。
- `docs/source` **3390 行 / 6 个文件**（「早期推导稿」）。
- 对比真正面向用户的内容：`getting-started` 1098 + `guides` 1119 + `final` 2016 + `pipeline` 1281 ≈ **5514 行**。即 plans+source（≈12668 行）**比全部用户向内容还多一倍多**。

**为什么是问题**：新人 clone 下来，`docs/` 里满屏 plan/review/实验快照/推导稿，找不到重点，信噪比低。

**修改方案**（保守、不删历史）：
1. **不删除**这些文件（它们是审计/历史资产）。在每个「非用户向」目录的 README 顶部加一行显著横幅，明确「贡献者/维护者向，新人可跳过」：
   - `docs/plans/README.md` 顶部加：`> ⚠️ 本目录是计划 / review / 维护记录，**面向贡献者**。第一次接触本仓的新人请从 [docs/getting-started/](../getting-started/) 开始，不需要读这里。`
   - `docs/source/` 若无 README 则新建一个 `docs/source/README.md`，写「早期推导稿，已被 `docs/final/` 取代；保留作溯源，新人勿入」，并解释与 `docs/final/` 的关系（见 R6）。
   - `docs/experiments/`、`docs/architecture/` 同样在各自 README 顶部加一句「非新人路径」。
2. 在 `docs/README.md` 的目录分层说明里，把「入门层 / 工程契约 / 场景指南 / 流水线」与「计划 / 推导 / 实验 / 架构」**视觉上分成两组**（前者「先读」，后者「按需 / 贡献者」），让新人一眼知道哪半边和自己无关。
3. （可选，需用户拍板）把 `docs/plans` 等整体迁到 `docs/_archive/` 或独立分支。**此项默认不做**，除非用户明确要求——迁移会动大量历史引用。

**验收**：
- `docs/plans`、`docs/source`、`docs/experiments`、`docs/architecture` 的 README 顶部都有「非新人路径」横幅。
- `docs/README.md` 的目录说明把「先读」与「按需/贡献者」分了组。

---

## R6 — 两本「权威总册」命名易混

**严重度**：🟡 低

**证据**：
- `docs/source/EPUB 3 制作完全参考手册.md`（1048 行）
- `docs/final/EPUB 3 终极实践手册.md`（1179 行）
- 两个名字都像「最终权威」。AGENTS.md 规定 `docs/source/` 是推导区、不能反向覆盖 `docs/final/`，但新人无法从文件名判断该读哪本。

**修改方案**（不改硬规则，只澄清）：
1. 在 `docs/source/EPUB 3 制作完全参考手册.md` 文件顶部加一句醒目说明（不改正文内容）：
   ```markdown
   > ⚠️ 这是**早期推导稿**，已被 `docs/final/EPUB 3 终极实践手册.md` 取代。当两者冲突时，**以 `docs/final/` 为准**。本文件仅作溯源保留。
   ```
2. 在 R5 新建的 `docs/source/README.md` 里也写明这层「source = 推导 / final = 对外约束」的关系。
3. 在 `docs/final/EPUB 3 终极实践手册.md` 顶部（可选）加一句「本手册是对外定论；推导过程见 `docs/source/`」，方便双向溯源。

**验收**：`docs/source` 的参考手册顶部有「已被 final 取代，以 final 为准」的指向；新人不会误把推导稿当权威。

---

## R7 — 两套 CSS 分层方案未说明区别（澄清，不是 bug）

**严重度**：🟡 低

**背景澄清（执行者务必先理解，避免误改）**：
- 模板 `templates/epub-style-demo/` 的 canonical 分层是 **8 层**：`fonts/base/notes/effects/literary/media/vertical/poster.css`（见 `docs/getting-started/02-anatomy.md:43`、`skills/README.md`、SPEC）。
- 自造样本 `city-field-notes` 的清洗 demo 拆成 **4 个文件**：`base/media/notes/tables.css`（见 `samples/demo-books/city-field-notes/notes.md:17`，`docs/getting-started/05-case-study.md:20` 据此描述）。
- 两者**都对**，但分别属于不同产物。新人同时读到会困惑：「到底该拆几层？哪个是规范？」

**修改方案**（只加一句澄清，**不要改 `tables.css` 这个词**）：
- 在 `docs/getting-started/05-case-study.md` 案例 1（`:20` 附近）加一句脚注/括注：
  ```markdown
  > 说明：这里的 `base/media/notes/tables.css` 是该样本书自身的简化拆分；模板 `templates/epub-style-demo/` 用的是更细的 8 层方案（`fonts/base/notes/effects/literary/media/vertical/poster.css`，见 SPEC §7）。两者都合法，按书的复杂度选。新书从模板起步时以 8 层为参考。
  ```

**验收**：案例页明确两套分层是「不同产物 / 都合法 / 如何选」，不再让新人误以为规范自相矛盾。

---

## R8 — README 两处「15 个专项 skill」措辞不准

**严重度**：🟡 低

**证据**：
- `skills/` 下共 **15 个** skill 目录。
- 其中 `epub-layout-auditor`、`epub-source-intake` 是「主入口」（`README.md:293-294` 自己也这么说），`epub-style-demo-maintainer` 是维护类；真正面向用户的「专项」是 13 个。
- 但 `README.md:17` 写「15 个**专项** skill」、`README.md:296` 写「**专项 15 个**」——把主入口也算进「专项」，措辞自相矛盾。

**修改方案（精确 `现状 → 改为`）**：

1. `README.md:17`
   - 现状：
     ```
     3. **AI 协作 skills** — [skills/](skills/)：15 个专项 skill（结构格式化、CSS 分层、字体、Ruby、Kindle 兼容、弹注、英文小说排版等）。可被 Claude Code / Codex 直接调用，也可由人工照 `SKILL.md` 步骤执行。
     ```
   - 改为：
     ```
     3. **AI 协作 skills** — [skills/](skills/)：共 15 个 skill（2 个主入口 `epub-layout-auditor` / `epub-source-intake` + 13 个专项：结构格式化、CSS 分层、字体、Ruby、Kindle 兼容、弹注、英文小说排版等）。可被 Claude Code / Codex 直接调用，也可由人工照 `SKILL.md` 步骤执行。
     ```

2. `README.md:296`
   - 现状：
     ```
     专项 15 个见 [docs/getting-started/04-skills.md](docs/getting-started/04-skills.md) 反向查表。
     ```
   - 改为：
     ```
     全部 15 个 skill（2 个主入口 + 13 个专项）见 [docs/getting-started/04-skills.md](docs/getting-started/04-skills.md) 反向查表。
     ```

> 执行时顺手核对：`ls -d skills/epub-*/ | wc -l` 应为 15；若数量已变，按实际值改，并保持「主入口 / 专项」口径一致。

**验收**：README 两处的数字与 `skills/` 实际目录数一致，且不再把主入口算成「专项」。

---

## R9 — reader-matrix 实测覆盖偏薄（期望管理）

**严重度**：🟡 低

**证据**：
- `docs/final/reader-matrix.yaml` 中 `status: pass` 仅 **7 条**、`status: warn` **22 条**，无 `fail`/`na`。
- 而 `README.md:3`、`README.md:344` 等处宣传「所有阅读器兼容性结论都从实测回写」「demo → reader-matrix → SPEC → 手册 的实测闭环」。

**为什么是问题**：口号给新人「这是一份成熟的放心兼容清单」的印象，但实际可直接当结论用的 `pass` 很少，大量是 provisional `warn`。**不可为了好看伪造 pass**（AGENTS.md / `03-readers.md:54` 明令「未测过不要写 pass」）。这是期望管理问题，不是数据造假问题。

**修改方案**（只调措辞 + 给读者看 matrix 的正确方式，不动 status 值）：
1. 在 `docs/getting-started/03-readers.md` 介绍 reader-matrix 处，加一句：
   ```markdown
   > matrix 现状以 `warn`（待复测 / provisional）为主，`pass` 较少。`warn` 不代表坏，而是「尚未在该 reader+版本上确认」。把它当「已知风险地图」，别当「全绿放行清单」。
   ```
2. 复核 README 里「所有……都从实测回写」类绝对表述，软化为「兼容性结论**一旦确认就**回写 matrix；matrix 同时记录尚未复测的 `warn` 假设」。具体定位执行时 `grep -n "实测回写\|从实测" README.md docs/final/*.md` 后逐句调整，保持与实际 status 分布一致。

**验收**：新人读到的 reader-matrix 描述与「7 pass / 22 warn」的真实分布一致，不产生「全绿」误解。**不得修改任何 status 值来迎合口号。**

---

## R10 — README 过载，「一条命令」承诺被长手动流程削弱

**严重度**：🟡 低

**证据**：
- `README.md` 共 351 行，单文件塞了：5 分钟跑通、三条路线 A/B/C（含整段 shell）、精排 harness 表、**完整 EPUB diff review 章节（`:194-276`，约 80 行含故障排查表）**。这些与 `docs/getting-started/`、`docs/pipeline/cleanup-flow.md` 大量重复。
- 「真实 EPUB 清洗」先给了一行入口 `epub_cleanup_pipeline.py`（`:111-117`），紧接着又贴 6 段带 `test -f BASE || BASE=...` 的手动路径拼接（`:147-178`），脆弱且吓人，**反而削弱了「一条命令」的承诺**。

**修改方案**：
1. 把 `README.md:194-276` 的「EPUB diff review」整章**迁到 `docs/pipeline/`**（建议新文件 `docs/pipeline/epub-diff-review.md`，或并入 `cleanup-flow.md` 的 diff 段），README 只保留一个「## EPUB diff review」小节 + 一段话 + 链接。
   - 迁移后全仓所有 `#epub-diff-review` 锚点引用都要改成指向新位置。先扫描：
     ```sh
     grep -rn "#epub-diff-review" --include="*.md" . | grep -v "docs/plans/"
     ```
     逐一改 active 文档（README、docs/README.md、getting-started/*、pipeline/*、samples/*）；**不动 `docs/plans/` 历史引用**。
2. 在 `README.md:107-117`「真实 EPUB 清洗」处，把那一行 `epub_cleanup_pipeline.py` 明确标为**推荐主路径**，并把后面 `:119-180` 的手动多步流程**折叠为一句话 + 链接**，指向 `docs/pipeline/cleanup-flow.md`（手动细节属于流水线文档，不该挤在 README）。保留 README 只展示「一条命令 + 它产出什么 + 完整流程见链接」。
3. 复核迁移后 README 行数应显著下降（目标 < 250 行），且「门面」职责清晰：是什么 / 我要做什么（表）/ 5 分钟跑通 / 三个去处链接 / 边界 / 文档地图。

**验收**：
- README 不再内联完整 diff-review 操作章节；`#epub-diff-review` 引用无死链。
- 「真实 EPUB 清洗」段以「一条命令」为主路径，手动细节下沉到 pipeline，不在 README 展开。

---

## 执行顺序建议

1. **先做不依赖外部环境的文本/导航修复**：R8 → R3(方案 A) → R7 → R6 → R5 → R1(文档部分) → R10。
2. **再做需要新增内容的**：R1 的 `00-what-is-epub.md`、R4 方案 A。
3. **最后做需要真实环境/人参与的**：R2（截图）、R4 方案 B（scaffold 脚本）、R9 的措辞复核。
4. R5 第 3 点（整目录归档）和 R4 方案 B 默认**不做**，需用户单独拍板。

## 统一验收（改完必跑，依 AGENTS.md「最小验证矩阵」）

```sh
# 任意改动
git diff --check

# 改了 AI 入口 / 维护文档
python3 scripts/validate_ai_entrypoints.py

# 改了 skills（本轮一般不会，但若动到 skills/*）
python3 scripts/validate_skills_basic.py

# 若新增/改了 Python 脚本（R4 方案 B）
python3 -m py_compile scripts/<new>.py && python3 scripts/test_<new>.py

# 若动到 demo / validator / docs/final（本轮一般不会；动了再跑）
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"

# Markdown 链接 / lint（本仓配置）
# 用 .markdownlint-cli2.jsonc 配置跑 lint；并人工点开本文新增/修改的相对链接确认可达
```

额外人工核查：
- 所有本轮**新增/修改的相对链接逐一可达**（尤其 R1/R3/R10 涉及大量链接）。
- 确认**没有触碰** `docs/plans/*`、`docs/experiments/*` 的历史正文（除非该条明确要求）。
- 确认**没有修改任何 reader-matrix `status` 值**（R9 只改措辞）。

## 回执模板（执行完回填）

| ID | 状态(done/skipped/blocked) | 改了哪些文件 | 验证命令&结果 | 备注 |
| --- | --- | --- | --- | --- |
| R1 | | | | |
| R2 | | | | 需截图，可能 blocked |
| R3 | | | | 选了方案 A 还是 B |
| R4 | | | | 选了方案 A 还是 B |
| R5 | | | | 第3点是否做 |
| R6 | | | | |
| R7 | | | | |
| R8 | | | | |
| R9 | | | | 确认未改 status 值 |
| R10 | | | | 确认锚点无死链 |
