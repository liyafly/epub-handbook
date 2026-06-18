# 仓库改进执行计划（2026-06-10）

> **给执行者（人或 LLM）**：本计划可独立执行，不依赖产生它的对话。开始任何任务前，先读根目录 `AGENTS.md` 并遵守其「最小验证矩阵」。每个任务用 checkbox（`- [ ]`）跟踪，做完一个任务提交一次 commit。
>
> 若交给 LLM 执行，建议提示词：
> 「先读取仓库根目录 AGENTS.md，再打开 docs/plans/2026-06-10-repo-improvement-execution-plan.md，从任务 W1.1 开始逐个执行。每个任务：先读"目标/文件/步骤"，执行后逐条核对"验收标准"，全部通过才允许 commit 并进入下一任务。验收不过就修复，不要跳过。」

**目标**：在不引入版权风险的前提下，完成仓库的文档治理、代码去重、发版收口，并新增三个把"人工审美判断"变便宜、变可复用的能力（排版决策记录、图片位置候选生成器、风格预设）。

**方法**：全部沿用本仓既有纪律——纯标准库脚本、dry-run 优先、红线 gate 兜底、demo 实测回写闭环。新增脚本一律"只建议不擅改"或"改动可回滚 + 红线验证"。

**范围**：

- 立即执行：W1 文档治理、W2 公共模块提取、W3 排版决策记录、W4 图片位置候选生成器、W5 风格预设、W6 入门体验快赢项、W7 v0.2.0 发版收口。
- 暂缓执行（已细致规划，等触发条件满足后再做）：D 公版书真实样本、E 开源字体转正、F 阅读器实测与截图补全。

---

## 设计共识（2026-06-10 讨论定稿，约束后续所有任务的范围）

精排的决策空间按「大道至简」收敛：机器负责一致性，人负责少数几次品味决策，决策被记录复用。每本书真正需要品味的决策只有四个——字体配对（一次）、章首处理（一次）、文本角色样式（一次）、逐图选位置（从菜单选）。

**单书决策树（推荐逻辑就这一棵树，不做更复杂的推荐系统）**：

```text
书是什么类型？（文学 / 古文译注 / 学术说明）→ 选预设（W5）
有注释吗？ → 弹注标准化（现有 oneclick converter + epub-popup-footnote-converter skill）
有图吗？   → 逐图出候选菜单（W4），人点选，结果落决策记录（W3）
```

**场景能力的投入排序**：

| 场景 | 定位 | 投入决定 |
| --- | --- | --- |
| 弹注 | 价值最高、审美含量最低；古文译注与说明类书籍的刚需，现有工具已覆盖大半 | 最高优先：规则收口（W1.2）+ 预设内样式（W5），流程打磨为主，不需要新造轮子 |
| A-lite | 必要能力——几乎每本书都可能有章节分页整页图 / 卷封，A-lite 是其标准处理 | 进候选菜单（W4 第 6 检测项）；转换实现已有 `scripts/epub_anthology_refinement.py` |
| 便签（note-box） | 限定场景：信件、说明文档 | 只在对应预设内提供样式，不做通用推广 |
| 图文浮动 | 限定场景：翻译 / 文白对照 | 只在古文译注预设与对照 guide 范围内使用 |
| 旋转便签等装饰花活 | 兼容性风险大于收益（SPEC 已因 Kindle 禁 `transform`） | 不投入 |

**章首图素材政策**：章首固定一个开头图是预设的标准能力，按「图片槽位」设计——仓库提供结构和样式（`.chapter-head-art` 等既有类），图片由使用者自备放入。素材分两层：**个人排版自用的成书**不对外传播，使用者可用自己找到的图片，无版权传播问题；**仓库内置随 MIT 公开分发的 demo / 预设示例素材**必须自制或有明确许可。工具和文档不得把示例图写死成必需依赖。

---

## 0. 执行总纪律

每个任务结束时，按改动类型至少运行（来自 `AGENTS.md` 最小验证矩阵）：

| 改动类型 | 命令 |
| --- | --- |
| 任意改动 | `git diff --check` |
| 维护文档 / AI 入口 | `uv run python scripts/validate_ai_entrypoints.py` |
| skills | `uv run python scripts/validate_skills_basic.py` |
| Python 脚本 | 对应 `scripts/test_*.py` + `uv run python -m py_compile <改动文件>` |
| demo / validator / docs/final | `bash templates/epub-style-demo/build.sh` 后跑 `scripts/validate-epub-style-demo.sh --epub <artifact>` 和 `scripts/validate-popup-notes.sh --epub <artifact>` |

全量测试一条命令：

```sh
for t in scripts/test_*.py; do uv run python "$t" || { echo "FAIL: $t"; break; }; done
```

推荐执行顺序与依赖：

```text
W1（文档治理）──┐
W2（公共模块）──┼─→ W7（v0.2.0 发版，最后做）
W3（决策记录）──┤
W4（图片候选）──┤   W4 的建议格式依赖 W3 的决策 schema，先做 W3
W5（风格预设）──┤
W6（入门快赢）──┘
D / E / F：等触发条件，互相独立；E 的视觉验收依赖 F
```

价值优先级（来自设计共识）：弹注链路（W1.2 + W5 预设内样式）最高，其次 W5、W4；W3 轻量长线积累。W2 A 期是工程基建，建议在 W4/W5 动工前完成，便于两者直接复用 `epub_lib`；B 期机会主义推进，不单独排期。

---

## W1. 文档治理

### W1.1 归档引用已删除 `tools/` 的历史计划

**目标**：`tools/` 已于 2026-05-28 删除，但 `docs/plans/handbook-expansion-plan.md`（约 50+ 处）和 `docs/plans/handbook-expansion-review.md`（约 20 处）仍引用 `tools/epub-diff/`。按 AGENTS.md「历史计划只在任务明确要求时修改，不重写历史」，本任务**不改正文**，只归档加注。

**文件**：

- 移动：`docs/plans/handbook-expansion-plan.md` → `docs/plans/archive/handbook-expansion-plan.md`
- 移动：`docs/plans/handbook-expansion-review.md` → `docs/plans/archive/handbook-expansion-review.md`
- 修改：`docs/plans/README.md`（索引从「当前计划」移到「已归档」）

**步骤**：

- [ ] `mkdir -p docs/plans/archive`（若已存在跳过），`git mv` 两个文件进去。
- [ ] 在两个文件**最顶部**（H1 之前）各加一段：

```markdown
> **已归档（2026-06-10）**：本文为历史计划快照。文中引用的 `tools/epub-diff/` 已于 2026-05-28 整体移除，人工 diff review 现行方案见 `docs/pipeline/epub-diff-review.md`。正文按"不重写历史"原则保留原样。
```

- [ ] 更新 `docs/plans/README.md`：把这两条从「当前计划」小节移到「已归档」小节，路径改为 `archive/` 前缀，并沿用现有条目的「一句话说明 +（日期）」格式。
- [ ] 全仓搜索确认没有其他**非归档**文档仍引用 tools/：`grep -rn "tools/epub-diff" --include="*.md" . | grep -v "docs/plans/archive/"`，期望只剩 `docs/plans/2026-05-28-*` 三个本身就是讲移除过程的归档类文档（如有，也一并 `git mv` 进 `archive/` 并更新索引）。

**验收标准**：

1. `grep -rn "tools/epub-diff" --include="*.md" . | grep -v archive` 输出为空。
2. `docs/plans/README.md` 的「当前计划」不再含这两个文件。
3. `uv run python scripts/validate_ai_entrypoints.py` 通过；`git diff --check` 干净。

### W1.2 弹注规则防漂移（guides 收口 + skills 机器查重）

**目标**：弹注结构规则目前散在 `docs/final/SPEC-实现约束.md` §1、`docs/guides/duokan-footnote-fallback-fix.md`、`docs/guides/epub2-popup-note-compatibility.md` 和多个 `skills/*/SKILL.md` 里。处理策略分两类：**guides 是说明文**，定义性段落收口为指向 SPEC §1 的引用；**skills 是自包含的行为契约**（仓库承诺"无 AI 也可用，人工照 SKILL.md 步骤执行"），定义保留在原地，改用**机器交叉校验**防漂移——从 SPEC §1 提取规范 token 集合，CI 断言 skills/guides 里出现的同族 token 都在集合内。

**文件**：

- 修改：`docs/guides/duokan-footnote-fallback-fix.md`
- 修改：`docs/guides/epub2-popup-note-compatibility.md`
- 修改：`scripts/validate_skills_basic.py`（新增 SPEC 交叉校验）
- 修改：`scripts/test_validate_skills_basic.py`（新增校验的测试）

**步骤**：

- [ ] 先盘点：`grep -rln "duokan-footnote\|noteref-icon\|footnote-list" docs/guides skills`，列出所有复述弹注结构的文件。
- [ ] 对每个 guide：保留场景说明和操作步骤；凡是**逐字复述结构定义**（必须有哪些 class、aside/ol/li 形状）的段落，压缩为一句「完整结构定义以 `docs/final/SPEC-实现约束.md` §1 为准」+ 保留该 guide 特有的差异点（如多看 fallback 的增量部分）。
- [ ] 对每个 skill：**内容不删**（保持自包含），仅在"判断"段开头加一行指向 SPEC §1 的引用作为权威源声明。frontmatter 只保留 `name` 和 `description`，不要动。
- [ ] **写交叉校验的测试**（先于实现）：在 `scripts/test_validate_skills_basic.py` 加用例——① 构造一个含 `duokan-footnote-typo` 这类不在规范集合内 token 的临时 markdown，断言校验报错并指出文件与 token；② 现有 skills/guides 全量通过。
- [ ] **实现交叉校验**：在 `scripts/validate_skills_basic.py` 加一个检查项——用正则从 SPEC §1 提取 `noteref-*`、`footnote-*`、`duokan-footnote-*` 三族 class 名构成规范集合；扫描 `skills/*/SKILL.md` 与 `docs/guides/*.md`，凡出现这三族前缀的 token 必须在集合内，否则报错（抓住改名、拼写错和单方面新增）。
- [ ] 跑 `uv run python scripts/test_validate_skills_basic.py` 和 `uv run python scripts/validate_skills_basic.py`。
- [ ] 改动如触及 `docs/final/`（本任务预期不触及），按 AGENTS.md 同步检查手册和速查表。

**验收标准**：

1. guides 中不再有逐字复述的弹注结构定义，只有引用 + 各自增量；skills 内容保持自包含且带权威源声明。
2. 交叉校验生效：人为在任一 skill 中把 `duokan-footnote-content` 改成 `duokan-footnote-contents` 后 `validate_skills_basic.py` 必须报错（验证后还原）。
3. `uv run python scripts/validate_skills_basic.py` 与其测试通过。
4. demo 构建与两个弹注/样式 validator 仍通过（规则值没被改错）：`bash templates/epub-style-demo/build.sh && EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)" && bash scripts/validate-epub-style-demo.sh --epub "$EPUB" && bash scripts/validate-popup-notes.sh --epub "$EPUB"`。

### W1.3 历史推导与过时架构文档加状态标注

**目标**：`docs/plans/archive/css-layering-plan.md`（八层 CSS）和 `docs/plans/archive/fonts-css-expansion-plan.md`（字体链推导）与 SPEC §7/§8 存在重复维护风险；`docs/architecture/epub-pro-v1.md` 已确认过时（2026-06-10 决定），不再作为任何设计依据。三份都不删（保留参考价值），但要明确状态。

**步骤**：

- [ ] 在两个推导文件顶部（H1 之后第一段前）各加：

```markdown
> **从属关系**：本文是推导/计划记录。分层与字体链的现行硬规则以 `docs/final/SPEC-实现约束.md` §7 / §8 为准；两者冲突时本文不作为依据。
```

- [ ] 在 `docs/architecture/epub-pro-v1.md` 顶部（H1 之前）加：

```markdown
> **已过时（2026-06-10）**：本架构蓝图不再维护，不作为下游设计依据；按"不重写历史"原则正文保留原样。
```

- [ ] `git diff --check` + `uv run python scripts/validate_ai_entrypoints.py`。

**验收标准**：三个文件首屏可见状态声明；validate_ai_entrypoints 通过。

---

## W2. 公共模块提取（`scripts/epub_lib.py`）

**目标**：`parse_xml`、`local_name`、`norm_join`、`write_epub`、`read_epub_files`、`split_props`、QName 工具 `q` 和命名空间常量目前在 4–8 个脚本里各有副本，且 `epub_css_cleanup.py:17` 与 `epub_anthology_refinement.py:16` 把 `epub3_oneclick_converter.py` 当事实共享库导入。本任务建立唯一公共模块，**行为不变**，仍只用标准库。

**分两期执行**：**A 期（必做）**修复现有三角依赖——建 `epub_lib` 并迁移 `epub3_oneclick_converter` / `epub_css_cleanup` / `epub_anthology_refinement` 三个现有参与者，这是在修真问题（validator/精排工具不该从一个 converter 导入）；**B 期（机会主义）**其余 9 个脚本只在因其他任务动到时顺手迁移，不单独排期——它们的副本可能各有微妙差异，专门做一轮大手术的风险收益比不划算。

**文件**：

- 新建：`scripts/epub_lib.py`
- 新建：`scripts/test_epub_lib.py`
- A 期修改：`epub3_oneclick_converter.py`、`epub_css_cleanup.py`、`epub_anthology_refinement.py`
- B 期（机会主义，动到才迁）：`epub3_migration_harness.py`、`epub_package_tool.py`、`epub_structure_tool.py`、`epub_refinement_harness.py`、`epub_ai_harness.py`、`validate_popup_notes.py`、`validate_text_invariance.py`、`validate_epub_style_demo.py`、`build_demo_epubs.py`

**关键约束**：

- 副本之间可能已有行为漂移。**每删一个本地副本前，先 diff 它与 `epub_lib` 版本**；行为不同的（比如某脚本的 `write_epub` 固定了 ZIP 时间戳），保留本地版本并在其 docstring 注明「与 epub_lib 的差异及原因」，不强行统一。
- 迁移以 `epub3_oneclick_converter.py` 的实现为基准（它已被两个脚本依赖，是事实标准）。

**步骤**：

- [ ] **第 1 步：建测试。** 新建 `scripts/test_epub_lib.py`，仿照 `scripts/test_epub_text_gate.py` 的风格（`sys.path.insert` + 纯 assert，可独立运行）。至少覆盖：
  - `local_name("{http://www.w3.org/1999/xhtml}div") == "div"`，无命名空间时原样返回；
  - `norm_join("OEBPS/Text", "../Images/a.png") == "OEBPS/Images/a.png"`；
  - `split_props("nav scripted") == ["nav", "scripted"]`，空串/None 返回 `[]`；
  - `parse_xml`：合法 XML 返回 Element，非法 XML 抛 `ET.ParseError`；
  - `read_epub_files` + `write_epub` 往返：构造最小 EPUB（mimetype 首位 STORED、container、OPF、一个 XHTML），写出后用 `zipfile` 读回，断言 ① `infolist()[0].filename == "mimetype"` 且 `compress_type == zipfile.ZIP_STORED` ② 其余条目内容逐字节相等。
- [ ] **第 2 步：跑测试确认失败**（模块还不存在）：`uv run python scripts/test_epub_lib.py`，预期 `ModuleNotFoundError`。
- [ ] **第 3 步：建模块。** 从 `epub3_oneclick_converter.py` **剪切**（不是复制）以下内容到 `scripts/epub_lib.py`：命名空间常量（`OPF_NS`、`OPF_URI`、`DC_URI`、`CONTAINER_NS`、`XHTML_NS` 等现有定义）、`parse_xml`、`local_name`、`norm_join`、`opf_path_from_container`、`rel_href`、`read_epub_files`、`write_epub`、`split_props`、`q`。模块 docstring 写明「本模块只用 Python 标准库；被多脚本共享，改动需跑全部 scripts/test_*.py」。
- [ ] **第 4 步**：`epub3_oneclick_converter.py` 顶部改为 `from epub_lib import ...`，并保留 `from epub_lib import *` 之外的**显式 re-export**（`epub_css_cleanup`/`epub_anthology_refinement` 仍从它导入，先不破坏）。跑 `uv run python scripts/test_epub3_oneclick_converter.py` + `test_epub_lib.py`，全过后 commit。
- [ ] **第 5 步**：把 `epub_css_cleanup.py:17-26` 和 `epub_anthology_refinement.py:16-29` 的导入源从 `epub3_oneclick_converter` 改为 `epub_lib`；跑各自测试；然后删掉第 4 步的临时 re-export，再跑一遍三个测试。commit。
- [ ] **第 6 步（A 期收尾）**：全量回归 `for t in scripts/test_*.py; do uv run python "$t" || break; done`，再跑一次 demo 构建 + 两个 validator + `samples/demo-books/build.sh`。A 期到此完成，可以开工 W4/W5。
- [ ] **B 期（机会主义，不单独排期）**：剩余 9 个脚本，只在因其他任务（W4/W5 实现、bug 修复等）动到某脚本时顺手迁移——删副本前先做副本 diff（见关键约束）、改 import、跑该脚本测试、单独 commit。CLI 签名统一为 `def main(argv: list[str] | None = None) -> int`（内部 `argv = sys.argv[1:] if argv is None else argv`）同样只在动到时顺手改；目前三种风格：13 个 `main(argv)`、1 个已是目标风格（`epub_cleanup_loop.py`）、3 个 `main()`（`build_demo_epubs.py`、`validate_ai_entrypoints.py`、`validate_skills_basic.py`）。

**验收标准**：

1. A 期：被移函数在 `epub_lib.py` 有唯一定义；`epub_css_cleanup.py` 与 `epub_anthology_refinement.py` 不再从 `epub3_oneclick_converter` 导入（`grep -n "from epub3_oneclick_converter import" scripts/*.py` 为空）。B 期长期目标（不阻塞验收）：`grep -c "def parse_xml" scripts/*.py` 合计收敛到 1，`local_name`、`write_epub`、`norm_join`、`split_props` 同理（允许有 docstring 注明差异的豁免副本，每个豁免须在 epub_lib.py 模块 docstring 中列出清单）。
2. 全部 `scripts/test_*.py` 通过；demo 构建与 validator 通过；`samples/demo-books/build.sh` + `validate_text_invariance.py`（before/after-clean，`--check all`，退出码 0）通过。
3. 所有脚本 `uv run python scripts/<x>.py --help` 行为与迁移前一致（抽查 5 个）。
4. 每个迁移 commit 信息形如 `refactor(scripts): migrate <script> to epub_lib`，单 commit 只动一个脚本。

---

## W3. 排版决策记录机制

**目标**：现在每次人工 diff review 的判断（图放哪、接受/拒绝哪条建议、为什么）做完就丢。建立一个最小决策记录系统：结构化落盘 → 可查询 → 下一本书可复用。这是把"个人审美"逐步固化为可执行规则的入口，也为 handshake planner 提供历史依据。

**设计要点（先读懂再动手）**：

- **两层存储**：
  - 书级：`work/<book>/reports/decisions.json` —— 单次清洗过程中的决策，跟 work 目录走，不入 git。
  - 仓库级：`records/typeset-decisions.jsonl` —— 人工确认值得复用的决策（`scope: global`），入 git。
- **版权与隐私红线**：记录里**禁止存正文文本**。`context` 只允许选择器、class 名、结构形状、阅读器名；`book` 字段是本地别名（可留空），不要求真实书名。这条写进 README 和校验逻辑。
- **决策条目 schema（v1）**：

```json
{
  "id": "dec-0001",
  "date": "2026-06-10",
  "source": "manual-review",
  "book": "book-a",
  "scene": "image-layout",
  "finding": "lone-image-no-figure",
  "context": {"selector": "div.pic > img", "readers": ["apple-books", "kindle"]},
  "candidates": ["figure.img-left", "figure.img-right", "figure-fullwidth"],
  "chosen": "figure.img-right",
  "rationale": "图注偏长，右浮动后与正文行长关系更稳",
  "scope": "global",
  "reusable": true
}
```

  字段约束：`source ∈ {manual-review, handshake, refinement}`；`scene ∈ {image-layout, popup-note, font-chain, chapter-head, poster, vertical, css-layering, other}`；`scope ∈ {book, global}`；`id` 在文件内唯一，格式 `dec-\d{4}`。

**文件**：

- 新建：`scripts/epub_decision_log.py`（CLI：`add` / `list` / `match`）
- 新建：`scripts/test_epub_decision_log.py`
- 新建：`records/README.md`（说明用途、schema、隐私红线、与 work/ 书级记录的关系）
- 新建：`records/typeset-decisions.jsonl`（初始为空文件或含 1 条示例）
- 修改：`docs/pipeline/cleanup-flow.md`（在人工 diff review 一节加"review 结论落决策记录"一步）
- 修改：`AGENTS.md`「文档落点」表加一行 `records/`（排版决策记录，机器可读）

**CLI 规格**：

```sh
# 追加一条（交互最少化：全部参数化；缺必填项报错退出码 2）
uv run python scripts/epub_decision_log.py add \
  --file records/typeset-decisions.jsonl \
  --scene image-layout --finding lone-image-no-figure \
  --candidates figure.img-left,figure.img-right,figure-fullwidth \
  --chosen figure.img-right \
  --rationale "图注偏长，右浮动更稳" \
  --scope global --source manual-review

# 列出（支持 --scene / --finding / --scope 过滤，--format json|md）
uv run python scripts/epub_decision_log.py list --file records/typeset-decisions.jsonl --scene image-layout

# 匹配：给定 finding kind，按 scene+finding 精确匹配返回历史决策（供 planner/人参考）
uv run python scripts/epub_decision_log.py match --file records/typeset-decisions.jsonl \
  --scene image-layout --finding lone-image-no-figure --format json
```

**步骤**：

- [ ] **第 1 步：写测试。** `scripts/test_epub_decision_log.py` 至少覆盖：
  - `add` 在空文件上写入合法 JSONL（读回 `json.loads` 成功、字段齐全、id 为 `dec-0001`）；
  - 第二次 `add` 自增为 `dec-0002`；
  - `add` 缺 `--chosen` 退出码 2，文件未被改动；
  - `add` 的 `--rationale` 含超过 80 个字符时正常接受（理由可以长），但 `context` 传入含 `text=` 键时拒绝（隐私红线：禁止正文字段），退出码 2；
  - `list --scene image-layout` 只返回匹配条目；
  - `match` 命中返回 JSON 数组，未命中返回 `[]` 且退出码 0。
- [ ] **第 2 步**：跑测试确认失败（脚本不存在）。
- [ ] **第 3 步**：实现 `scripts/epub_decision_log.py`。要求：只用标准库；写入前整文件校验（任何一行损坏即拒绝 add 并报哪一行）；写入用「临时文件 + os.replace」原子替换；`main(argv: list[str] | None = None) -> int`。
- [ ] **第 4 步**：跑测试至全过，`py_compile` 干净，commit。
- [ ] **第 5 步**：写 `records/README.md`（用途、两层存储、schema 全字段表、**schema 版本：当前为 1，未来字段变更须在此记录迁移说明**、隐私红线加粗、示例命令）；创建 `records/typeset-decisions.jsonl` 放 1 条标注为示例的条目（`book: "example"`）。
- [ ] **第 6 步**：改 `docs/pipeline/cleanup-flow.md`：在人工 diff review 步骤后追加一步「把值得复用的判断用 `epub_decision_log.py add --scope global` 落到 `records/typeset-decisions.jsonl`；只属于这本书的判断落 `work/<book>/reports/decisions.json`（`--file` 指过去即可）」。改 `AGENTS.md` 文档落点表。跑 `uv run python scripts/validate_ai_entrypoints.py`。
- [ ] **第 7 步（可选增强，单独 commit）**：`epub_cleanup_loop.py` 加 `--decisions <path>` 参数——handshake 模式下把 `match` 到的历史决策以 `"hints"` 数组附进 `plan-request.json`（只附加信息，不改变白名单和红线逻辑）。补对应测试：带 `--decisions` 时 plan-request 含 hints 字段；不带时无该字段；hints 不影响 rules planner 行为。

**验收标准**：

1. `scripts/test_epub_decision_log.py` 全过；全量 `scripts/test_*.py` 无回归。
2. `records/typeset-decisions.jsonl` 每行可被 `json.loads` 解析（验收命令：`uv run python -c "import json,sys;[json.loads(l) for l in open('records/typeset-decisions.jsonl') if l.strip()]"`）。
3. 隐私红线可执行：构造含 `--context text=正文片段` 的 add 调用被拒绝。
4. `cleanup-flow.md` 和 `AGENTS.md` 的新增段落通过 `validate_ai_entrypoints.py`。
5. （若做第 7 步）handshake 一轮 dry 流程中 `plan-request.json` 出现 hints，且不带 `--decisions` 时行为与旧版逐字节一致。

---

## W4. 图片位置候选生成器

**目标**：图片位置最终由人定，但"找出有问题的图 + 给出候选方案 + 标注各阅读器风险"可以自动化。新建一个**只读建议**脚本：输入 EPUB，输出每张问题图片的 2–3 个候选布局和风险说明，人从菜单里选，选择结果可直接落 W3 的决策记录。

**先决条件**：W3 已完成（候选输出的 `scene/finding` 字段与决策 schema 对齐）。

**检测项（v1 共 6 个，全部基于标准库 XML/正则，不渲染）**：

| finding kind | 判定 | 候选方案 |
| --- | --- | --- |
| `lone-image-no-figure` | `<img>` 不在 `<figure>` 内（封面页、A-lite 海报页除外——按 spine properties 和 `body.fullpage`/`body.poster-bg` 识别后排除） | `figure.img-left` / `figure.img-right` / `figure-fullwidth` |
| `caption-detached` | 图片元素的下一个兄弟是 `<p>` 且其文本 ≤ 30 字符，或 class 含 `caption`/`tu-zhu` | 并入 `<figure><figcaption>` |
| `float-width-risk` | 浮动图 `width` 不在 SPEC 规定的 25%–35% 区间，或 `float`/百分比宽度写在 `<img>` 而非 `<figure>` 上 | 按 SPEC §图文环绕修正（float 与百分比宽度放 `<figure>`，内部 `<img>` 用 `width:100%; height:auto`） |
| `missing-alt` | `<img>` 无 `alt` 属性 | 补 `alt`（提示人工填写内容） |
| `chapter-head-image-candidate` | 图片是某 XHTML `<body>` 的第一个内容子元素且该文件在 nav 中是章节入口 | 维持现状 / 转 `chapter-head-art`（指向 `docs/guides/chapter-head-image.md`） |
| `fullpage-image-alite-candidate` | 某 spine XHTML 的内容基本只有一张图（`<body>` 下唯一内容元素是 img/figure，正文字符 ≤ 20）——典型即章节分页图、卷封 | A-lite contain / A-lite fullbleed / 维持普通整页 figure；转换实现参考 `scripts/epub_anthology_refinement.py`，规则见 SPEC A-lite 条目 |

每个候选附 `risk` 字段，内容取自 SPEC 与 reader-matrix 的已知结论（如：Kindle 对 `transform` 的限制、A-lite 适用条件）。**风险文案必须能追溯到 SPEC 条目或 reader-matrix 条目 id，不允许凭空写**；查不到出处的写 `"risk": "未实测，见 reader-matrix 待验证项"`。

**输出 schema**：

```json
{
  "version": "1",
  "epub": "input.epub",
  "findings": [
    {
      "scene": "image-layout",
      "finding": "lone-image-no-figure",
      "file": "OEBPS/Text/ch03.xhtml",
      "selector": "body > div:nth-of-type(2) > img",
      "image": "OEBPS/Images/fig-07.jpg",
      "candidates": [
        {"id": "figure.img-left", "summary": "左浮动 25–35% 宽", "risk": "SPEC §图文环绕；短段落环绕会塌，见 demo 17-image-layout 反例"},
        {"id": "figure.img-right", "summary": "右浮动 25–35% 宽", "risk": "同上"},
        {"id": "figure-fullwidth", "summary": "通栏 figure + figcaption", "risk": "最稳，所有 reader-matrix 已测阅读器无已知问题"}
      ]
    }
  ]
}
```

**文件**：

- 新建：`scripts/epub_image_layout_advisor.py`（CLI：`<input.epub> [--format json|md] [--report <path>]`；**绝不写 EPUB**）
- 新建：`scripts/test_epub_image_layout_advisor.py`
- 修改：`docs/pipeline/refinement-harnesses.md`（新增一节介绍该脚本与 refinement harness 的分工：refinement 是全局建议，advisor 是图片专项 + 候选菜单）
- 修改：`README.md` 脚本速查表加一行

**步骤**：

- [ ] **第 1 步：写测试。** 用内存构造最小 EPUB（参考 `scripts/test_epub_text_gate.py` 的 `_make_min_epub` 写法），每个检测项至少一正一反：
  - 裸 `<img>` → 命中 `lone-image-no-figure`；包在 `<figure>` 里 → 不命中；
  - 海报页（`body class="poster-bg"`）里的裸 img → **不**命中（排除规则生效）；
  - img 后跟 12 字短 `<p>` → 命中 `caption-detached`；后跟 80 字长段 → 不命中；
  - `<img style="float:left;width:50%">` → 命中 `float-width-risk`；`<figure class="img-left" style="width:30%">` 内 `<img style="width:100%">` → 不命中；
  - 无 alt → 命中 `missing-alt`；
  - 整页图 XHTML（body 内唯一内容是一张 img、正文字符 ≤ 20）→ 命中 `fullpage-image-alite-candidate`；普通正文页内含图 → 不命中；
  - 输出整体可 `json.loads`，顶层含 `"version": "1"`（下游 GUI/自动化按它判断契约兼容性），每个 finding 的 `scene/finding` 值在 W3 schema 的枚举里。
- [ ] **第 2 步**：跑测试确认失败。
- [ ] **第 3 步**：实现。解析复用 `epub_lib`（W2 产物）；遍历 spine 内 XHTML；selector 生成用「标签 + nth-of-type」路径即可，不必完整 CSS selector 引擎。`main(argv: list[str] | None = None) -> int`，无 findings 时退出码 0 并输出空数组，有 findings 也是 0（它是建议器不是 gate）。
- [ ] **第 4 步**：跑测试至全过；对 `samples/demo-books/dist/*.epub`（先 `bash samples/demo-books/build.sh`）和 demo 模板 EPUB 各跑一次，人工抽查输出合理。commit。
- [ ] **第 5 步**：写 `--format md` 报告：按文件分组，每图列候选表格，结尾附一行可直接复制的 `epub_decision_log.py add` 命令模板（预填 scene/finding/candidates，留 `--chosen` 和 `--rationale` 给人）。补一条 md 格式的测试断言（含 "epub_decision_log.py add" 字样）。
- [ ] **第 6 步：接线（防孤儿化——新建建议器不接入流程就会重蹈 `epub_css_cleanup` 游离的覆辙）**：① `epub_cleanup_pipeline.py` 在 refinement 阶段后追加 advisor 调用，结果并入 `reports/pipeline.json`（新增 `image_layout_advisor` 节；提供 `--no-image-advisor` 开关），在 `test_epub_cleanup_pipeline.py` 补断言该节存在与开关生效；② `skills/epub-image-layout-optimizer/SKILL.md` 的"判断"段加一行：机器入口为 `scripts/epub_image_layout_advisor.py`（frontmatter 不动），跑 `uv run python scripts/validate_skills_basic.py`。
- [ ] **第 7 步**：更新两处文档，跑 `validate_ai_entrypoints.py`，commit。

**验收标准**：

1. 测试全过；全量 `scripts/test_*.py` 无回归。
2. `uv run python scripts/epub_image_layout_advisor.py <demo.epub> --format json | python3 -m json.tool` 成功，输出中每条 risk 要么含 SPEC/demo/reader-matrix 出处，要么是"未实测"声明。
3. 跑前后输入 EPUB 的 SHA-256 不变（只读验证：`shasum -a 256` 前后对比）。
4. md 报告末尾的 add 命令模板可直接粘贴执行（人工试一条，写入 `work/` 下的临时决策文件成功）。
5. 接线验收：对 demo 书跑 `epub_cleanup_pipeline.py`，`reports/pipeline.json` 含 `image_layout_advisor` 节，`--no-image-advisor` 时无该节；`epub-image-layout-optimizer` 的 SKILL.md 引用该脚本且 `validate_skills_basic.py` 通过。

---

## W5. 风格预设（style presets）

**目标**：把"个人审美"从开放问题变成菜单选择。一个预设 = 在 SPEC §7 八层 CSS 框架内的一组成品选择（字体链方案 + 章首处理 + 便签视觉 + 部分排版参数）。v1 交付 3 个预设（`literary-cn` 文学向 / `classical-annotated-cn` 古文译注向 / `academic-cn` 学术说明向，对应设计共识决策树的三种书型）和一个应用工具（dry-run 优先）。

**设计约束（必须满足）**：

- 预设只产出 `Styles/` 八层框架内的 CSS 文件，**层职责严格遵守 SPEC §7 的允许/禁止矩阵**（fonts.css 只放字体、base.css 不放弹注等）。
- 不设置页面级 `color`/`background`（SPEC §7 附加规则，保护夜间模式）。
- 应用工具只改三类东西：`Styles/*.css` 内容、OPF manifest 的 CSS 声明、XHTML `<head>` 里的 `<link>`。**正文一字不碰**，因此 `validate_text_invariance.py --check all` 必须天然通过——这是硬验收。
- 单 CSS 文件 400 行预警、500 行硬上限（SPEC §7）。
- **适用性前提（必须写进工具输出和文档）**：预设 CSS 瞄准本仓 class 体系（`.book-song`、`.chapter-head`、`.parallel-float-pair` 等）。对未迁入该体系的原始 EPUB 套预设不会报错但**静默无效**——选择器选不中任何元素，红线照样全绿，这比报错更坑。因此 `apply --dry-run` 必须输出 coverage：统计书内 spine XHTML 实际使用的 class 中被预设样式覆盖的比例，低于 30% 时打印警告「该书尚未迁入本仓 class 体系，请先走 cleanup pipeline（oneclick 会注入 typography palette）」。

**预设目录格式**：

```text
templates/style-presets/
  README.md                  # 预设是什么、怎么加新预设、与 SPEC §7 的关系
  literary-cn/
    preset.json              # 机器可读清单（见下）
    README.md                # 设计意图、适用书型、效果描述
    Styles/                  # 完整八层（缺省层可省略，工具按 preset.json 处理）
      fonts.css
      base.css
      notes.css
      effects.css
      literary.css
      media.css
  classical-annotated-cn/
    ...
  academic-cn/
    ...
```

`preset.json` 格式（v1 保持扁平；用 JSON 而非 YAML 的原因：Python 标准库无 yaml 解析器，避免引入依赖）：

```json
{
  "name": "literary-cn",
  "version": "1",
  "description": "中文文学向：宋体正文链、楷体引文、章首大标题留白、便签弱边框",
  "layers": ["fonts.css", "base.css", "notes.css", "effects.css", "literary.css", "media.css"],
  "base_font_chain": "book-song",
  "notes": "视觉效果未经 reader-matrix 实测前，应用后状态标记为待实测"
}
```

**文件**：

- 新建：`templates/style-presets/README.md`、`templates/style-presets/literary-cn/`、`templates/style-presets/classical-annotated-cn/`、`templates/style-presets/academic-cn/`
- 新建：`scripts/epub_style_preset_tool.py`（CLI：`list` / `show <name>` / `apply <input.epub> --preset <name> --output <out.epub> [--dry-run]`）
- 新建：`scripts/test_epub_style_preset_tool.py`
- 修改：`README.md`（脚本速查表 + 「三条可执行路线」不动，加一小节）、`docs/pipeline/refinement-harnesses.md`（哪一步可以套预设）

**步骤**：

- [ ] **第 1 步：先做 CSS 内容。** 以 `templates/epub-style-demo/OEBPS/Styles/` 现有八层为基底复制三份，按预设意图改参数：
  - `literary-cn`：正文链 `.book-song` 为默认（body 继承），blockquote/epigraph 用楷体类，章首 `chapter-head` 上留白加大，`note-box` 用细边框低对比（便签按设计共识只服务信件场景）；
  - `classical-annotated-cn`：正文宋体链，译文/注释用楷体类形成层次，文白对照启用 `.parallel-float-pair`（图文浮动按设计共识只在此预设的对照场景使用），弹注样式强化（只调 notes.css 视觉，弹注结构一律不动、仍以 SPEC §1 为准）；
  - `academic-cn`：正文链宋体、标题黑体类，表格/代码样式权重提高，章首紧凑，便签用规整直角边框（服务说明文档场景）；
  - 三个预设都包含章首图槽位：`.chapter-head-art` 样式齐备，图片文件由使用者自备放入（见设计共识的素材政策），预设内不内置示例位图；
  - 三份都逐条对照 SPEC §7 的禁止列（例如确认 fonts.css 内无任何排版规则）。
- [ ] **第 2 步**：写 `preset.json` ×3 和四个 README（总 README + 每预设一个）。
- [ ] **第 3 步：写工具测试。** 覆盖：
  - `list` 输出三个预设名；`show literary-cn` 输出 description；`show` 不存在的名字退出码 2；
  - `apply --dry-run`：输出将替换/新增的文件清单 JSON（含 `coverage` 字段，0–1 浮点），且**输入 EPUB 字节不变**；
  - coverage 机制：用本仓 palette class 的书 → coverage 高于阈值、无警告；class 全是随机原始名的书 → coverage 低于 0.3 且 stderr 出现"先走 cleanup pipeline"警告文案；
  - `apply` 实写：输出 EPUB 中 ① `Styles/` 含预设全部层 ② OPF manifest 对每个 CSS 都有声明 ③ spine 内每个正文 XHTML 的 `<head>` link 至少含 `fonts.css + base.css` 且顺序符合 SPEC §7 加载顺序 ④ mimetype 仍是首位 STORED；
  - 对 apply 前后跑 `validate_text_invariance.py` 等价逻辑（直接调 `epub_text_gate` 的接口）断言正文不变；
  - 输入含同名旧 CSS 时：旧文件被替换且 dry-run 清单里标记为 `replace` 而非 `add`。
- [ ] **第 4 步**：跑测试确认失败 → 实现工具（复用 `epub_lib` 读写；XHTML link 改写参考 `epub_xhtml_transforms.py` 的纯字符串变换思路，不重序列化整文档）→ 测试全过。commit。
- [ ] **第 5 步：端到端验证。** `bash samples/demo-books/build.sh`，对 `city-field-notes-before.epub` 应用 `literary-cn`，然后：
  - `uv run python scripts/validate_text_invariance.py before.epub preset-applied.epub --check all` 退出码 0；
  - `bash scripts/validate-popup-notes.sh --epub preset-applied.epub` 通过；
  - Calibre Editor 或 VS Code 抽查 diff（确认只动了 Styles/OPF/link）。
- [ ] **第 6 步**：文档更新 + `validate_ai_entrypoints.py`。在 `templates/style-presets/README.md` 显著位置写明：「预设的**视觉效果**尚未经 reader-matrix 实测（见暂缓任务 F）；结构合规已由 validator 保证」。

**验收标准**：

1. 工具测试全过、全量回归无失败。
2. 端到端：对 demo 书应用任一预设后，红线 gate 退出码 0；弹注 validator 通过；diff 中不出现任何 `Text/*.xhtml` 正文行变化（link 行除外）。
3. 每个预设的每个 CSS 文件 ≤ 400 行（`wc -l` 验收），且人工对照 SPEC §7 矩阵无越层内容。
4. `--dry-run` 模式下输入文件 SHA-256 前后一致。
5. README/pipeline 文档更新后 `validate_ai_entrypoints.py` 通过。
6. coverage 机制可用：对未迁入 palette 的原始书 dry-run 出现低 coverage 警告；对 demo（palette）书不出现警告。

---

## W6. 入门体验快赢项（不依赖截图的部分）

**目标**：执行 `docs/plans/2026-06-01-newcomer-onboarding-review.md` 中不依赖截图的项。R2（配图）依赖暂缓任务 F，不在本任务内。

**步骤**：

- [ ] **R3 编号与阅读顺序**：不重命名文件（避免链接大面积失效）。在 `docs/getting-started/` 的入口文件（有 README.md 则用之，没有则在 `docs/README.md` 的入门段）放一张明确的阅读顺序表：00 → 01 → 02 → 03 → 06 → 04 → 07 → 08 → 05 → glossary，每行一句"读完你能做什么"。核对 `00-what-is-epub.md` 末尾的"下一步"指向 01。
- [ ] **R1 术语墙**：在 `00-what-is-epub.md` 和 `01-first-epub.md` 中，OPF、manifest、spine、nav、NCX 五个词的**首次出现处**加 glossary 锚点链接（glossary 文件已存在于 getting-started）。不改术语本身。
- [ ] **R5 过程产物淹没手册**：已部分由 W1.1 归档解决。剩余动作：在 `docs/README.md` 索引中把 `docs/plans/`、`docs/source/`、`docs/experiments/` 三项明确标注「面向贡献者，新人无需阅读」。
- [ ] 跑 `uv run python scripts/validate_ai_entrypoints.py`，检查改动文件内的相对链接（`grep -o '](\.\./[^)]*' <file>` 逐个确认目标存在，或用编辑器链接检查）。

**验收标准**：

1. 新人从 `docs/getting-started/` 入口能看到一张完整阅读顺序表，顺序与 onboarding review 建议一致。
2. 五个核心术语在 00/01 首次出现处都有 glossary 链接，且链接目标锚点真实存在。
3. `validate_ai_entrypoints.py` 通过；改动文件无死链。

---

## W7. v0.2.0 发版收口（最后执行）

**目标**：`CHANGELOG.md` 的 `v0.2.0 - [待发布]` 段已写好内容；审计计划约定"全部验证通过后改日期打 tag"。本任务逐条核验后发版。

**步骤**：

- [ ] 逐条核验 CHANGELOG v0.2.0 的声明（每条至少一个证据命令）：
  - NFC 红线：`grep -n "unicodedata" scripts/validate_text_invariance.py` 非空，且 `uv run python scripts/test_validate_text_invariance.py` 通过；
  - 多 nav 收敛：`grep -n "removed_nav" scripts/epub3_migration_harness.py` 非空 + `test_epub3_migration_harness.py` 通过；
  - zip-slip：`grep -n "safe_extractall" scripts/validate_popup_notes.py` 非空 + `test_validate_popup_notes.py` 通过；
  - detector stderr：`test_epub_ai_harness.py` 通过；
  - srcset：`grep -n "rewrite_srcset" scripts/epub_structure_tool.py` 非空 + `test_epub_structure_tool.py` 通过；
  - CI 两个 gate：打开 `.github/workflows/build-epub-demo.yml`，确认含 markdownlint 与 demo-books EPUBCheck 步骤；若缺失，先补齐再发版（这是审计计划 §4 的遗留项）。
- [ ] 全量测试 + demo 构建 + 两个 validator + `git diff --check` 全绿。
- [ ] 把 CHANGELOG 的 `## v0.2.0 - [待发布]` 改为 `## v0.2.0 - <当天日期>`，并把本计划已完成的工作流（W1–W6 中实际完成的）按既有条目风格补进 v0.2.0 或新开 v0.3.0 段（若 W1–W6 在 v0.2.0 tag 之后才完成，则进 v0.3.0；执行时按实际时序判断，原则：**tag 内容必须与 tag 时刻的代码一致**）。
- [ ] `git tag v0.2.0 && git push origin v0.2.0`（推送前确认远端分支策略；若仓库走 PR 流程，tag 在合并 main 后打）。

**验收标准**：

1. CHANGELOG v0.2.0 每条声明都有对应通过的证据命令（建议把命令和输出摘要记进 commit message 或 PR 描述）。
2. tag 指向的提交上，全量测试和 demo validator 全绿。
3. `git tag -l` 含 v0.2.0；CHANGELOG 无 `[待发布]` 残留。

---

# 暂缓任务（已规划，等触发条件）

> 以下三项**现在不做**。每项写明触发条件、完整做法和验收标准，触发后可直接执行，无需重新规划。

## D. 公版书真实样本接入（暂缓）

**触发条件**：找到至少一本**确认公版**且可下载 EPUB 的书。

**为什么重要**：当前所有 demo 都是自造的、太干净；pipeline 从未在真实混乱 EPUB 上验证过。`samples/third-party/` 目录规则已齐备但一本书都没有。

### D.1 选书与版权判断（必须先做对）

- **首选来源（按风险从低到高）**：
  1. **Standard Ebooks**（standardebooks.org）：整书 CC0/公有领域声明，EPUB3 制作质量高 → 适合当"精排参照样本"（看人家怎么排）。
  2. **Project Gutenberg**（gutenberg.org）：美国公有领域为主，EPUB 质量参差 → 适合当"清洗对象样本"（脏数据）。注意 PG 的商标条款：去除其品牌内容后再分发，或只用 fetch.sh 引导用户自行下载、不再分发。
  3. **维基文库中文**（zh.wikisource.org）：文本多为 PD-old 或 CC BY-SA，**逐篇核对许可标签**；通常无现成 EPUB，需自行打包，正好测 `epub-source-intake` skill。
- **判断规则（写进 metadata.yaml 的 `license_rationale` 字段）**：
  - 原作者逝世年限满足相关法域要求（中国大陆：逝世后 50 年；注意以**作者**而非出版年计算）；
  - **翻译、校注、新序跋有独立版权**——选公版译本或无译注版本；
  - 来源站点的许可声明截图或 URL 存档。
- **推荐起步组合**：1 本 Standard Ebooks 英文小说（参照）+ 1 本 PG 或维基文库中文公版书（清洗对象）。

### D.2 收录流程（遵循 `samples/third-party/README.md` 既有规则）

- [ ] 建 `samples/third-party/<book-slug>/`，内含四件套：
  - `fetch.sh`：`curl -L` 下载 + `shasum -a 256 -c` 校验（哈希写死在脚本里）；**实体 .epub 不入 git**（.gitignore 已覆盖，执行时确认）。
  - `metadata.yaml`：标题、作者、来源 URL、抓取日期、SHA-256、许可类型、`license_rationale`。
  - `LICENSE.txt`：来源站的许可原文或链接存档。
  - `notes.md`：这本书选进来要测什么（脏点清单：混淆文件名？EPUB2？无 nav？图片混乱？）。
- [ ] 更新根 `THIRD_PARTY.md`：来源、作者、许可、链接（AGENTS.md 关键约束的硬要求）。
- [ ] 对清洗对象书走完整流程并留痕（AGENTS.md「已有 EPUB 固定流程」七步）：preflight → normalize dry-run → 人工确认 → 迁移 → 精排建议 → 红线 → diff review，全部报告留在 `work/<book-slug>/reports/`（不入 git），结论摘要写进 `notes.md`。
- [ ] **回收价值**：把 pipeline 在真书上暴露的问题逐条登记——detector 没识别的脏模式补进 `docs/pipeline/cleanup-patterns.md`，值得自动化的开 issue 或追加到本计划。人工判断过的排版决策用 W3 的工具落 `records/typeset-decisions.jsonl`。

**验收标准**：

1. `samples/third-party/<slug>/` 四件套齐全；`bash fetch.sh` 在干净环境可重复下载且哈希校验通过。
2. `THIRD_PARTY.md` 有对应条目；仓库内 `git ls-files | grep '\.epub$'` 为空。
3. 清洗对象书有完整七步留痕，红线 gate 退出码 0，`notes.md` 含"发现的工具缺口"小节（哪怕写"无"）。
4. 至少 1 条来自真书的全局决策进入 `records/typeset-decisions.jsonl`。

## E. 开源字体转正 + 子集化（暂缓）

**触发条件**：D 完成后（有真书需要嵌字验证），或决定激活 SCENE_MATRIX 的 C1 外部场景时。

**为什么重要**：SCENE_MATRIX 唯一的"外部人工场景" C1（全字符集字体链）卡在"需授权素材"。思源宋体/黑体、霞鹜文楷都是 OFL-1.1，可以无风险转正；字体子集化是中文 EPUB 最实际的痛点之一（全量 CJK 字体十几 MB，子集后常 <1MB）。

### E.1 字体引入

- [ ] 选型：思源宋体（Source Han Serif SC，OFL-1.1）为正文衬线，霞鹜文楷（LXGW WenKai，OFL-1.1）为楷体层。下载 OTF/TTF 官方 release。
- [ ] **字体文件不入 git**（体积大）：建 `templates/epub-style-demo/fonts-external/fetch-fonts.sh`，模式同 D 的 fetch.sh（URL + SHA-256 写死）；`.gitignore` 加 `fonts-external/*.otf` `*.ttf` `*.woff2`。
- [ ] 许可入档：`LICENSES/` 目录加 OFL-1.1 全文（命名 `OFL-1.1-SourceHanSerif.txt` 等），`THIRD_PARTY.md` 加条目（名称、版权人、许可、来源链接）。OFL 要求随字体分发许可文本——**嵌入字体的 EPUB 内也要带**（放 `OEBPS/Fonts/` 旁或 colophon 注明）。

### E.2 子集化（外部工具，本仓不内置——与 imagemagick/oxipng 同一姿势）

- [ ] 提取书内用字：写 `scripts/epub_font_charset_report.py`（标准库；遍历 spine XHTML 提取 text 节点字符集合，输出去重后的 unicode 列表文件 + 字数统计）。配 `test_epub_font_charset_report.py`（最小 EPUB 含已知字符集合，断言输出完全一致；含 ruby/rt 内字符也被收集）。
- [ ] 子集化提供**两条路径，主路径为命令行脚本**，由使用者按自己的工具链选择：

  **主路径（推荐）：fonttools/pyftsubset**（不进 requirements，用 uv 临时环境）。只替换字体二进制这一个文件，其余内容零接触——最小 diff、确定性、离线可审计，与本仓纪律一致：

```sh
uv run --with fonttools pyftsubset SourceHanSerifSC-Regular.otf \
  --text-file=work/<book>/reports/charset.txt \
  --flavor=woff2 \
  --output-file=OEBPS/Fonts/SourceHanSerifSC-subset.woff2
```

  **备选路径：Calibre `ebook-polish --subset-fonts`**，给已经在用 Calibre 工具链的人。注意三条边界（写进 guide）：① Calibre **转换**（ebook-convert / 入库自动转换）会重写样式、把 class 扁平化为 `calibre1/calibre2`，**绝不可用于本流程**；② Calibre Editor 手动保存是保守的，不会主动改样式（本仓 diff review 选它正因如此）；③ `ebook-polish` 只执行所点的操作、不改正文标记，但会整包重新打包，可能产生 OPF/打包层的附带 diff（manifest 顺序、空白归一化），diff review 时会混入噪音——这是它列为备选而非主路径的原因。

  无论哪条路径，产物一律过 `validate_text_invariance.py --check all` + 人工 diff review，两条路径因此同样安全，差别只在 diff 的干净程度。把两条路径、上面的选择标准和"子集后必须人工翻检生僻字是否缺字"的提醒一起写进 `docs/guides/`（新建 `docs/guides/font-embedding-subsetting.md`：选型 → 取字集 → 子集化（双路径） → fonts.css 接线 → OPF 声明 → 验证）。
- [ ] fonts.css 接线遵守 SPEC §7/§8：`@font-face` 只进 fonts.css；嵌入专用类用既有的 `.rare` / `.title-special` / `.signature` 槽位；字体链顺序不破坏系统字体优先策略（嵌入字体作为链尾兜底还是专用类，按 SPEC §8 现行规则执行，冲突时以 SPEC 为准）。
- [ ] OPF manifest 声明字体条目（`font/woff2` 或 `application/font-sfnt`），**不做字体混淆**（obfuscation 会触发 preflight 的加密标记路径，且 OFL 不要求）。
- [ ] 激活 C1：按 `templates/epub-style-demo/SCENE_MATRIX.md` 外部场景说明，临时加入子集字体构建 demo，跑两个 validator；把 C1 从"外部人工场景"改写为可复现场景（fetch-fonts.sh + 构建说明），SCENE_MATRIX 同步更新。

**验收标准**：

1. `fetch-fonts.sh` 可重复执行且哈希校验通过；`git ls-files` 中无字体二进制。
2. `LICENSES/` 与 `THIRD_PARTY.md` 含 OFL 条目；嵌字 demo EPUB 内含 OFL 文本或 colophon 声明（人工开包检查）。
3. `epub_font_charset_report.py` 测试全过；对 demo 书输出的字符集与人工抽样一致。
4. 嵌字 demo 通过 `validate-epub-style-demo.sh --epub`（注意现有检查项"fonts.css 不含活跃 ../Fonts/ 引用"是针对**无字体骨架**的——激活 C1 时该检查需按 SCENE_MATRIX 的外部场景约定调整 validator 或加豁免开关，调整本身要有测试）。
5. 子集字体的视觉验证（缺字、fallback 顺序）登记进 reader-matrix —— 依赖任务 F，未实测前在 SCENE_MATRIX 标注"待实测"。

## F. 阅读器实测与截图补全（暂缓）

**触发条件**：你方便接触目标阅读器环境时（macOS Apple Books 本机即可开始；Kindle Previewer 3 需安装；Readest/KOReader/多看按可得性）。**E 的视觉验收依赖本任务。**

**为什么重要**：reader-matrix 现状 6 pass / 23 warn / 13 个核心场景从未实测，而仓库的全部公信力建立在"规则有实测兜底"上；同时全仓 0 张截图（onboarding review R2）。本任务一石二鸟。

### F.1 准备

- [ ] 以 `docs/final/reader-matrix.yaml` 的 `untested_cases` 清单为唯一待办源（当前含：00-title-page、01-basic-cjk、02-ruby-standard-footnote、03-alite-poster、03b-fullbleed、03c-contain、04-table-code、07-font-family-order、08-long-mixed-flow、11-chapter-opening、12-literary-fiction、13-duokan-rich-fallback、14-vertical-body、15-frontmatter；以文件实际内容为准）。
- [ ] 构建当批 artifact：`bash templates/epub-style-demo/build.sh`，记录产物文件名（它带时间戳，是回写时的 artifact 字段）。
- [ ] 建截图目录与命名规范：`docs/final/assets/screenshots/<case-id>--<reader>--<version>.png`，例如 `02-ruby-standard-footnote--apple-books--macos-15.4.png`。**尺寸纪律**：长边 ≤ 1200px、单图 ≤ 250KB（`magick input.png -resize 1200x1200\> -quality 85 out.png` 或 macOS 截图后用 `sips -Z 1200`），防仓库膨胀。

### F.2 每用例执行模板（逐 case × 逐 reader 重复）

- [ ] 在目标阅读器打开当批 artifact，翻到该 case 对应页（SCENE_MATRIX 列了每个场景的 XHTML 文件与检查点）。
- [ ] 按 SCENE_MATRIX 的"主要检查点"逐条核对：渲染正确 → 截图存档 → 记 `pass`；渲染异常 → 截图 + 记录现象 → 记 `fail`；该阅读器不适用 → `na`。
- [ ] 立即回写 `docs/final/reader-matrix.yaml`，每条必填（AGENTS.md 硬约束）：`status`、artifact 文件名、阅读器名称和**版本号**、现象描述、处理动作或待复测项。信息不全的只能记"待验证假设"。
- [ ] fail 的条目按 AGENTS.md 实测闭环走：若推翻现有 SPEC 结论，修 SPEC（手册、速查表、相关 skills 同步），必要时补 demo 场景。

### F.3 优先级与批次

按影响面排序，分三批，每批做完即回写、即 commit，不攒批：

1. **第一批（中文书核心）**：01-basic-cjk、02-ruby-standard-footnote、15-frontmatter、11-chapter-opening —— Apple Books + Kindle Previewer。
2. **第二批（差异化能力）**：03 系列海报三件、14-vertical-body、13-duokan-rich-fallback —— Apple Books + 多看/Readest（竖排和多看 fallback 是这批的重点）。
3. **第三批（长尾）**：04、07、08、12、00 + 既有 23 条 warn 的复测（Kindle Previewer 的 9 条 pending-version 优先）。

### F.4 截图反哺手册（onboarding R2）

- [ ] 实测截图就位后，在 `docs/getting-started/00-what-is-epub.md`、`02-anatomy`（或对应文件）和 `docs/final/EPUB 3 终极实践手册.md` 的关键规则处插入 before/after 或正例截图（相对路径引用 `docs/final/assets/screenshots/`），每张图配一句"看什么"。
- [ ] 跑 `validate_ai_entrypoints.py` + 抽查图片链接有效。

**验收标准**：

1. `reader-matrix.yaml` 的 `untested_cases` 清空，或剩余项逐条标注了明确的不可测原因（如"本机无该阅读器"→ 保留为待测并写明缺什么）。
2. pass 比例显著提升且**每条 pass/fail 都有 artifact + 阅读器版本 + 截图路径**，无"裸结论"。
3. `grep -c 'status: warn' docs/final/reader-matrix.yaml` 相比执行前下降（执行前先记录基线数字）。
4. 入门文档和手册合计新增 ≥ 8 张实测截图，全部 ≤ 250KB，链接有效。
5. 所有因实测推翻的规则都完成了 SPEC → 手册 → 速查表 → skills 的同步检查（AGENTS.md 关键约束）。

---

## 后续展望：图形工作台（方向定稿 2026-06-10，不在本计划范围）

清洗工作台 GUI 经多轮讨论定稿，记录在此防止后续重议或设计漂移。

**总原则（承重墙，不可破）**：

- **GUI 是壳，本仓 Python 管线是唯一大脑**：GUI 通过子进程调用 `scripts/`（`uv run python ...`），状态一律从 work-dir 内的 JSON 报告读取（监听文件，不解析 stdout），不在 GUI 侧重新实现任何管线逻辑。所有状态落在 work-dir 文件里，GUI 自身无状态、可随时关闭重开，CLI 与 GUI 始终可互换。GUI 代码里出现一行管线知识即为设计错误。
- **接口契约 = W3/W4/W5 的 JSON 输出 + 既有 handshake 文件协议（`plan-request.json` / `plan.json`）**。本计划落地的 schema 质量直接决定 GUI 成本；advisor 输出带 `version` 字段，决策记录 schema 版本记录在 `records/README.md`。决策写入也走 `epub_decision_log.py add` 子进程，使校验与原子写只存在一份。

**形态定稿：原生双端各自维护（放弃 Web 内核与跨平台框架）**。用户规模小、自己慢维护，原生单端体验上限最高；契约层保证 UI 重复只发生在视图代码，管线知识零重复。

- **macOS 优先**：AppKit 骨架（`NSSplitViewController` 三栏：会话列表 / 内容区 / 检查器，`NSToolbar` / `NSMenu`，WKWebView 预览）+ SwiftUI 面板（经 `NSHostingView`：findings 列表、候选卡片、预设选择器含 coverage 条、决策表单、设置页）。引擎侧独立 Swift Package（Contracts 镜像五个 JSON 契约并校验 `version`；EngineDriver 用 `Process` + DispatchSource/FSEvents 监听 `reports/`），可拿本仓 fixture JSON 做无 GUI 单元测试。WKWebView 与 Apple Books 同为 WebKit 系，预览即近似主要目标阅读器——预览窗必须标注「当前引擎 ≈ 哪些阅读器」；`takeSnapshot` 按任务 F 的命名规范存档截图。**不上 App Sandbox**（需起 uv 子进程、访问任意 work-dir），分发走 Developer ID + 公证；设置页含仓库路径 / uv 路径配置与启动健康检查。
- **Windows 暂缓，候选已定**：首选 **WinUI 3**（已接受其试错成本；`WebView2` 是框架内置控件、集成度最佳），备选 WPF（求稳、文档与 AI 辅助语料最厚）；Avalonia 仅在 Linux 成为真实需求时考虑（其开源核心无内置 WebView）。配套：CommunityToolkit.Mvvm、CliWrap、System.Text.Json、`FileSystemWatcher`。预览引擎标注换为「Chromium ≈ Thorium / Readest 系」。
- **已评估并放弃的路线**（连同理由记录，避免回头重议）：Tauri——其 IPC 底层就是 WKScriptMessageHandler / WebView2 host objects（由框架代维护），且仍需自行监督 Python 进程，等于平添一个 Rust 运行时；Swift→WASM——无 Swift 核心可编译、浏览器沙箱无法承载 Python 大脑、SwiftUI 无 Web 渲染路径；Web 内核 + 薄壳（pywebview / localhost HTTP / Docker 服务形态）——搁置但不堵死，引擎契约不变，将来真需要内网服务时再给 Python 侧补 web 前端，与原生端互不影响。`docs/architecture/epub-pro-v1.md` 已过时，不作为 GUI 设计依据。

**落点与顺序**：

- GUI 代码不进本仓（见 README「这个仓库不是什么」的边界）；架构定稿后按 AGENTS.md 惯例在 `docs/architecture/` 放一份新副本。
- 动工顺序：先落地 W2 A 期 + W3 + W4 + W5（契约稳定）→ 用「md 报告 + 粘贴命令」纯 CLI 闭环在一两本真书上验证交互模式 → 写一页平台无关的 UX 流程规格（页面、状态、动作，作为两端共同实现依据）→ macOS MVP（打开 work-dir → 跑管线 → 图片候选菜单 → 决策落盘）→ 第二轮加预览对比、预设 coverage、实测助手 → Windows 端在 macOS 端交互验证完成后按 WinUI 3 启动。

---

## 全局完成定义（Definition of Done）

全部立即执行任务（W1–W7）完成后，以下命令在仓库根目录全绿：

```sh
git diff --check
uv run python scripts/validate_ai_entrypoints.py
uv run python scripts/validate_skills_basic.py
for t in scripts/test_*.py; do uv run python "$t" || { echo "FAIL: $t"; exit 1; }; done
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
bash samples/demo-books/build.sh
uv run python scripts/validate_text_invariance.py \
  samples/demo-books/dist/city-field-notes-before.epub \
  samples/demo-books/dist/city-field-notes-after-clean.epub --check all
```

外加人工核对四条：

1. `docs/plans/README.md` 的「当前计划」里没有指向死目录的条目；本计划自身在 W1–W7 完成后从「当前计划」移到「已归档」。
2. `records/typeset-decisions.jsonl` 存在且 ≥ 1 条合法条目，`cleanup-flow.md` 有落决策的步骤。
3. `git tag -l` 含 v0.2.0。
4. 暂缓任务 D/E/F 的触发条件原样保留在本文档中，未被误执行。
