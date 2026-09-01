# Go 重写交接

> 状态快照：**W0–W5 全部完成，迁移收尾** · 2026-08-30 · 工具链 Go 1.27
>
> 规范来源是 [`docs/final/SPEC-go-architecture.md`](../final/SPEC-go-architecture.md)（第一档硬约束）。
> 本文件是**状态与决策记录**，不是约束。两者冲突时以 SPEC 为准。

| 指标 | 当前值 |
|---|---|
| 包测试 | 28 个包由 `go test ./...` 覆盖；本轮触及的迁移 oracle 测试已改为 Go-native fixture / golden，不以缺文件 skip 代替回归验证 |
| 守卫测试 | archguard 全绿；INV-10 棘轮已归零，零条目基线继续执行零容忍扫描 |
| CLI ready 能力 | 16 / 22（其余 5 个 B 类纯 AI skill + `epub.source.intake` 待决策） |
| 旧执行面引用 | **0**（149 → 0；零条目基线保留，仅迁移脚手架删除） |
| 信封 schema | v2 `contracts/schemas/v2/envelope.schema.json` 已落地；INV-6 按 golden 的 schemaVersion 分发校验（§8） |
| 已删除 | `swift/`（303M）、`gui/`、`scripts/`（1.4M）、`adapters/`、`.venv`/`uv.lock`/`pyproject.toml`/`.python-version`、`mise.toml`、`tools/parity/` 中除零基线外的迁移脚手架 |

## 0. 进度快照（2026-08-30 凌晨，W5 收尾）

**CLI 端到端可用**：`go run ./cmd/epub {run <id> | capabilities | redline}`。
统一信封（schemaVersion 2）、退出码 0/1/2/3、契约驱动的 requires 校验均已落地。
真书（49MB《EPub指南》）parity 达标的能力见下表；全部 16 个 A 类能力均注册并 ready。

| capability | parity 证据 |
|---|---|
| epub.package.nav.audit | legacy JSON 全键一致（11 findings/skills/commands/detectors） |
| epub.layout.audit | 同上（build 模式投影） |
| epub.text.content.analyze | 全量 blocks 逐键一致（15826 块；修了 pre-order 块序 + Python isdigit 个位数值规则） |
| epub.image.layout.optimize | legacy 报告逐键一致 |
| epub.structure.normalize | apply 后 **817/817 entry CRC 逐字节一致**；报告仅路径字段差异 |
| epub.package.merge | fixture + 真书自合并全 entry 逐字节（仅 dcterms:modified 时间戳） |
| epub.package.split | 全 entry 逐字节；OPF 语义一致（Go 保留原字节 vs Python 整树重排） |
| epub.metadata.edit | 非 OPF entry 逐字节；OPF 经 ET 规范化语义一致 |
| epub.cover.replace | 真书 parity 测试全绿（16 个 cover 图引用重写 + SVG viewBox） |
| epub.font.coverage.analyze | legacy 报告全键一致（经 extern 调 uv + tools-font） |
| epub.notes.popup.normalize | 真书 39 条错误措辞逐字一致；**W5 补齐**：校验核心按 oracle 重写（backlink 收集范围、target⊆note-list、exact role 匹配、duokan 模式、urlsplit/unquote 图标解析），12 类错误措辞 + 通过路径 parity 全绿 |
| epub.package.migrate.epub3 | **W5 补齐**：exec-parity 双用例（默认 + --no-typography），conversion 报告逐字节 + 除 OPF 外全 entry 逐字节 |
| epub.css.layering.optimize | parity 全绿（apply/scoped-merge 双用例） |
| epub.typography.optimize | parity 全绿（apply/dry-run 双用例） |
| epub.alite.convert | **W5 收尾**：修复半成品（编译错误、INV-7 正则表、报告缺 harness 键、`</head>` 替换语义、CLASS_RE/SRC_RE 精确形状、BODY_RE count=1）；单卷/双卷/无版权页 warning/expect_volumes 报错四用例 parity 全绿；OPF 差异登记于 `tools/parity/allow.md`（已随脚手架删除，差异语义由 pyCanonicalXML 测试保证） |
| epub.style.demo.maintain | 源树/产物双模式；scene 28 的 manifest/spine/nav/NCX、共享 CSS、块级标题和无 inline/img 契约同时校验源码与构建产物，21 个 Go-native 测试全绿 |

**B 类（5 个，纯 AI skill，无专属实现，按 §7.1 不建 caps 包）**：
kindle.compatibility.check、literary.structure.format、notes.legacy-fallback、
typography.english.optimize、vertical.ruby.optimize —— 依赖的两个通用校验器已变为
Go 能力（`epub.style.demo.maintain` / `epub.notes.popup.normalize`）。

**W5 交付内容**：
- 19 个 SKILL.md 按 §8.4 四段模板改写为 `epub run` 形态；`agents/openai.yaml` 同步。
- 42 个文档（docs/ + templates/）与根文档（AGENTS/README/CONTRIBUTING/CLAUDE）旧引用清零。
- `hooks/pre-commit.epub-handbook` 改调 `go test ./internal/archguard/` + epub CLI；
  `.github/workflows/build-epub-demo.yml` 换 Go 工具链（swift job 删除），
  `.github/CODEOWNERS` 与 PR 模板同步。
- `epub redline` 补 `--path-map`（对齐 validate_text_invariance 的改名映射，
  在 pipeline 层载入，cmd 层保持零 redline 知识）；修复 `--legacy-report` flag
  未透传进 Args 的 W4 遗留缺口。

**关键语义决策（迁移期确定）**：
1. `epub run <id>` 是**单能力执行**（与 Python oracle 路由一致），requires 只做
   存在性与无环校验；上游结果经 Upstream 注入但默认不自动执行。
2. 输出**先写盘、后红线校验**（对齐 Python「写 after → gate → 人工 review」）；
   红线 error 把状态降为 failed、退出码 1，但输出保留供人工 diff review。
3. 契约标 transformer 但执行面只读的能力（popup.normalize）用 `registerReadOnly` 覆盖；
   多产物能力（split）用 `registerMultiOutput`；无 EPUB 输入能力（style.demo.maintain）
   用 `registerNoBook`（--input 为空/目录 → 源树模式，文件 → 产物模式）。
4. 全局 flag（input/output/dry_run/legacy_report）由 pipeline 注入 Args。

**过程中抓到的 Python 语义怪癖**（Go 已复刻）：
- `urlsplit` 剥离 URL 首尾 C0 控制符与空格（真书里存在 `src=" ../Images/note.png"`）。
- Python `str.isdigit()` 只认个位数值（①-⑨ 算、⑩❿⒑ 不算、〇 不算）。
- ElementTree 序列化的全部字节细节（单引号声明、`<tag />`、属性转义表）。
- `ensure_stylesheet_link` 以「href 子串已在文本」为幂等判据，且会把 `</head…>` 匹配
  整段替换为 `</head>`（不是原样保留）。
- Python regex `\b` 的词字符只有 `[A-Za-z0-9_]`——`data-class="x"` 中的 `class=`
  依然命中 CLASS_RE。
- 弹注校验中 role 属性是整串精确比较（`role="doc-noteref extra"` 不算 noteref），
  epub:type 与 class 才是分词包含。

**架构产物**：`internal/{zipfs,editset,book,redline,report,extern}` +
`internal/scan/{opf,xhtml,css}`（opf 含 spans.go 区间树）+
`internal/caps/*`（15 包）+ `internal/pipeline` + `cmd/epub`。
依赖：`golang.org/x/text`（NFC + ianaindex，已登记 SPEC §9.3）。

---

## 1. 四个决策及其依据

每个决策都由仓库实测数据支撑，不是偏好。依据留在这里，是为了让后来者能重新审视而不是盲从。

### 1.1 用什么语言 → **Go**

候选是 Go / Rust / TS / Swift。移动端排除后，Rust 的最大优势（iOS + Android FFI）失效，
而它在两个明确担忧上都最差：`target/` 轻松 1.5–3GB，AI 生成的代码编译不过的比例显著更高。

决定性的技术理由不是「Go 简单」，而是 `archive/zip` 自 1.17 起提供
`Writer.Copy` / `File.OpenRaw` / `Writer.CreateRaw`——未修改的 entry 可带着原压缩数据
字节级透传。**这恰好就是正文不变 gate 需要的原语。**

### 1.2 慢在哪里 → **架构，不是语言**

旧 pipeline 是 subprocess 编排器：每个 stage 起新 Python 进程、整包读 ZIP、整包写 ZIP、退出。
49MB 的书跑 8 个 stage 产生约 **800MB 无谓 I/O** 加 8 次解释器冷启动。

换语言但照搬架构，这 800MB 一字不少。因此重写必须同时改架构——这是 INV-1 与 INV-3 存在的原因。

补充事实：主 pipeline 那 17,137 行是**纯标准库**（`zipfile` + `xml.etree` + `re`），
`pyproject.toml` 声明了 lxml 但 scripts 里一次没用。CPU 密集部分连 C 加速都没开。

### 1.3 Swift 怎么办 → **删除**

实测 `swift/.build` 占 **303MB**，仅 3 个依赖。Windows 支持痛苦，且缺 CSS 解析库
导致仓库里手写了 800 余行 CSS parser（`CSSCleanupPrimitives` 408 行 + `CSSCleanupPlanner` 400 行）。
执行链路上无下游依赖，已删除。

### 1.4 两套 EPUB3 迁移留哪个 → **`epub3_conversion`**

| | A `epub3_migration_harness.py` | B `epub3_conversion/` |
|---|---|---|
| 规模 | 438 行 | 1465 行 |
| 能力 | 版本号、`dcterms:modified`、nav 生成、spine idref——四件事 | 上述全部 **+** 多看注释、Sigil 遗留注释、正文字体锁定感知、封面 properties、guide→landmarks、媒体类型规范化、XHTML 外壳规范化 |
| 契约指向 | 无 | `epub.package.migrate.epub3` → `epub3_migration_apply_harness.py` |
| pipeline 调用 | 无 | `epub_cleanup_pipeline.py` → `epub3_oneclick_converter.py` |
| 测试 | 173 行 | 510 行 |

决定性依据是第三、四行：**A 在执行链路上是孤儿**，只被文档和自身测试引用；
契约与 pipeline 都走 B。而 B 超出的部分恰是本手册的领域核心。

A 唯一独有的是 plan 模式，作为全局 `--dry-run` 保留，不构成保留整套实现的理由。
Go 侧只实现 B 的行为，A 已随 `scripts/` 删除。

---

## 2. 核心设计：架构编码成可执行守卫

这份方案的前提是：**后续修改要能交给能力较低的模型执行且不偏移架构。**

弱模型不会因为读懂了设计说明就不跑偏，只会因为改歪了编译不过、测试红才不跑偏。
所以重心不是架构图，是 `internal/archguard/` 里的 14 个守卫测试。

配套的是 **规则 0：禁止修改 archguard**——守卫红了意味着改动违反了架构，不是守卫写错了。
错误反应是改守卫、加白名单、注释掉断言、打 `t.Skip`。

### 2.1 十条不变式（终态）

| | 规则 | 堵住的跑偏 | 状态 |
|---|---|---|---|
| INV-1 | 未修改 entry 必须 `zip.Writer.Copy` 透传 | 退回整包读写，800MB I/O 回来 | 已验证 |
| INV-2 | `scan/*` 只产 `[]Edit`，禁导出整文档序列化 | DOM 往返静默破坏正文不变 | 已验证 |
| INV-3 | 一次运行只写一次盘 | 中间态落盘，退回旧架构 | 已验证 |
| INV-4 | `caps` 禁 `os/exec` | 外部依赖散落各处 | 已验证 |
| INV-5 | 契约里每条红线必须有注册校验器 | 声明了红线却没实现 | 已验证 |
| INV-6 | 报告必须过 schema | 输出格式漂移 | 已验证（golden 已落） |
| INV-7 | 禁包级可变 `var` | 弱模型最爱的「加个全局传状态」 | 已验证 |
| INV-8 | `skills/` 不得有 `.py` / `.sh` | 执行面重新分散 | 已验证 |
| INV-9 | SKILL.md 只能引用真实存在的 capability id | 改名后文档失效，AI 照着跑就炸 | 已验证 |
| INV-10 | 旧执行面引用棘轮，只删不增 | 迁移中途有人图省事再写一条 | **已归零**（保留零条目基线，守卫继续扫描） |

### 2.2 守卫已验证会开火

空仓库上全绿证明不了任何事。因此造了 9 处故意违规逐条验证——
向上 import、caps 互相 import、cmd 不薄、`os.Create`、`os/exec`、`archive/zip` 外泄、
整文档序列化、包级全局变量、未登记的新包——**全部命中，且定位到文件与行号**。

红线闭包单独验证：只注册 3/6 条时，精确点名缺失的 `anchors` / `cover` / `drm`，
并列出每条由哪些契约声明。`register.go` 白名单也确认生效——注册表放行，
换个文件名同样的变量就被抓。

W5 期间守卫又抓到两处真实违规：
`cmd/epub` 直接 import `internal/redline`（TestCmdIsThin，已下沉到 pipeline 层修复）；
alite 半成品的包级正则表（TestNoPackageState，已迁入 register.go）。

### 2.3 守卫的真正牙齿

文档里那句「禁止修改 archguard」对弱模型只是软约束——它完全可以删掉断言让测试变绿。
真正的强制来自三处仓库配置，均已落地：

| 文件 | 作用 |
|---|---|
| `.github/workflows/archguard.yml` | 独立必过 job，故意不与其它测试合并；触碰 archguard 时额外告警 |
| `.github/CODEOWNERS` | 守卫、SPEC、`contracts/` 均需人类审阅 |
| `.github/pull_request_template.md` | 架构自检勾选项 |

---

## 3. 棘轮：迁移进度的唯一硬指标（已完成）

旧执行面引用**只许减少，不许新增**。基线在 `tools/parity/legacy-refs.txt`，
从 149 处（46 文件）逐步删除，2026-08-29 归零；`scripts/` 随之删除，
`tools/parity/` 的 allow/parity 脚手架一并删除。零条目的 `legacy-refs.txt`
必须继续保留，使守卫在终态仍执行零容忍扫描；缺失基线会触发 bootstrap-skip，
不再视为可接受的完成态。

---

## 4. 推进路线（全部完成）

| 波次 | 内容 | 状态 |
|---|---|---|
| W0 | `zipfs` + `book` + `editset` + INV-1 行为测试 | ✅ 49MB 样本书透传 I/O 实测通过 |
| W1 | 六条红线校验器 + `report` | ✅ INV-5 闭包成立 |
| W2 | `scan/{xhtml,css,opf}` + audit / lint / content_analyze | ✅ parity P1+P2 全绿 |
| W3 | structure_normalize / css_cleanup / migrate_epub3 + 四个 package 操作 | ✅ parity 三级全绿 |
| W4 | `pipeline` + `cmd/epub` + 统一返回信封 | ✅ 端到端可用 |
| W5 | 19 个 SKILL.md + 42 个文档；两个 shell 校验器变 Go 能力；棘轮归零；按序删除旧执行面 | ✅ 2026-08-29 完成 |

### 4.1 终态仓库形态（已达成）

| 目录 | 状态 |
|---|---|
| `swift/` `gui/` | ✅ 已删（303M） |
| `scripts/` | ✅ 已删（W5 棘轮归零后） |
| `adapters/` | ✅ 已删（被 `epub capabilities` 取代） |
| `.venv` / `uv.lock` / `pyproject.toml` / `.python-version` / `mise.toml` | ✅ 已删（`tools-font/` 独立 uv 项目不受影响） |
| `tools/parity/` | ✅ 迁移脚手架已删；仅保留零条目 `legacy-refs.txt` 供 INV-10 持续扫描 |
| `docs/` `skills/` | 留。skills 只剩 SKILL.md + openai.yaml |
| `contracts/` `templates/` | 留。契约与 demo fixture，第一档硬约束的证据来源 |
| `references/` | 留。样本 EPUB（49M），测试与 parity 的输入 |
| `records/` `archive/` | 留。排版决策记录与第三档参考 |
| `tools-font/coverage-detector/` | 留。Python + fonttools，**明确不迁**，经 `internal/extern` 调用 |
| `cmd/` `internal/` `.github/` `hooks/` | 留。Go 实现与 CI（hook 已改调 Go 守卫 + CLI） |

---

## 5. 两处反直觉结论

### 5.1 报告格式的政策反转过一次

早先结论是「legacy 报告格式必须逐字节保持，不要顺手规整」，理由是下游解析会碎。
但下游只有两类：互相调用的 Python 脚本，和告诉 AI 该跑什么的 SKILL.md——
**二者全部重写后，这个理由不成立了**。

于是 `epub_lint.py` 的数组顶层、`validate_text_invariance.py` 的纯文本输出，
从「要保护的契约」变成「要清掉的包袱」，统一到 SPEC §8.2 的单一信封。
代价是 parity gate 没法再逐字节比对，对策是迁移期 `--legacy-report` 脚手架
（见 §6 遗留项 2：脚手架的 CLI 入口仍在，但比对对象已随 `scripts/` 删除）。

仍然不许动的是**退出码语义**——pre-commit hook 依赖它，信封换了但 0/非 0 的含义必须一致。
`epub redline` 子命令对齐的是 `validate_text_invariance.py` 的 legacy 退出码
（含输入缺失时的 2），这与信封命令的 0/1/2/3 是两套并存语义，均已在文档注明。

### 5.2 三段式带来两个意外红利

任务模板强制 capability 写成「扫描（只读）→ 应用（唯一写点）→ 报告（不落盘）」。
这个约束本是为了压缩弱模型的自由度，但顺带解决了两件事：

**其一**，Go 的 `encoding/xml` 往返丢信息本是选 Go 的最大风险。
而只要不整篇重序列化，这个短板就没有作用面——**风险被架构消解，而不是被语言解决**。

**其二**，`--dry-run` 变得近乎免费：跑扫描、跳过 `b.Apply(edits)`、把 edits 摘要进报告即可。
因此它是 `pipeline` 统一实现的**全局 flag**，各个 capability 不用各自处理。

另有一个佐证：`epub_xhtml_transforms.py` 的 docstring 明确写着
*"minimal-diff XHTML string transforms (no DOM reserialize)"*——
字节级外科手术不是新发明的架构，是把仓库里已有的正确做法固化成全局约束。

---

## 6. 遗留与待决策（接手者从这里开始）

1. **`epub.source.intake` 未实现（需人类决策）**。契约存在、CLI 显示 pending
   （review sweep 后 `epub run` 它返回 `failed` / 退出码 1，见 §8.2）。
   它需要 pipeline 支持非 EPUB 输入（源文件目录/PDF 等），且"接入"本身大部分是
   人工+AI 流程（见 `skills/epub-source-intake/SKILL.md`，已如实注明现状）。
   决策点：值得为它建 Go 实现吗，还是维持人工流程、仅保留契约？
2. **`--legacy-report` 脚手架未拆除**。SPEC §5.2 的移除触发条件（对应 Python 脚本删除）
   已满足，但该脚手架深度织入 caps 包与其测试（2026-08-30 实测：117 处引用、
   37 个文件，其中 48 处在测试断言里——测试经 `Facts["legacyReport"]` 以 legacy
   形状断言语义，拆除须逐包重设计断言面）。作为独立后续任务：把断言迁移到
   `Result.Facts` 后，删除各 cap 的 `LegacyReport` 参数、`legacyReport` 结构体
   与 CLI flag。
3. **Python 元校验器随 `scripts/` 消失**：`validate_skills_basic.py`（frontmatter/
   openai.yaml 形状）、`validate_contracts.py`（契约 schema）、
   `validate_docs_consistency.md`（手册/速查表同步）、`validate_ai_entrypoints.py`。
   其职责部分由 archguard INV-8/9/10 与 CI 承担，**文档同步检查完全回到人工**；
   SKILL.md frontmatter 形状与契约 JSON 形状目前同样无守卫。如需恢复，建议按
   SPEC §6.1 建成 Go 守卫（放 `internal/archguard/` 或独立包，需人类审阅）。
4. **`epub_lint.py` 无对应能力**（SPEC §7.2 映射表列了 `internal/caps/lint`，
   但 22 个契约里没有 lint id）。现行裁决：产物检查 =
   `epub run epub.package.nav.audit` + `epub redline --check all` + CI EPUBCheck，
   已写入文档。若认为仍缺独立 lint 能力，需先补契约再按 §6.1 实现。
5. **发行为前置**：CLI 目前以 `go run ./cmd/epub` 或 `go build -o epub ./cmd/epub`
   分发，尚无 release 流程（跨平台构建、版本号注入）。
6. ~~本分支改动尚未提交~~ 已解决：W5 改动按逻辑分块提交完毕；
   review sweep 改动见 §8。

---

## 7. 本次会话（W5 收尾）改动的文件

| 范围 | 说明 |
|---|---|
| `internal/caps/alite/` | 收尾移植 + parity |
| `internal/caps/popupnotes/` | 校验核心按 oracle 重写 + parity（原实现零测试且规则偏离） |
| `internal/caps/migrate_epub3/` | 补 exec-parity 双用例 |
| `internal/caps/styledemo/` | 新包：699 行 demo 校验器移植（agent 完成） |
| `internal/pipeline/{register,run}.go` | alite 注册；noBook 机制；legacy_report 透传；redline path-map 下沉 |
| `cmd/epub/main.go` | `epub redline --path-map` |
| `skills/`（19 个） | §8.4 四段模板改写（两个 agent 完成） |
| `docs/`（24 个）+ `templates/`（6 个） | 旧引用清零（两个 agent 完成） |
| 根文档 + `hooks/` + `.github/` | AGENTS/README/CONTRIBUTING/CLAUDE、pre-commit、CI、CODEOWNERS、PR 模板 |
| 删除 | `swift/` `gui/` `scripts/` `adapters/` `tools/parity/` 迁移脚手架（零基线除外）和 Python 环境文件 |

---

## 8. Review sweep（2026-08-30 晚）

迁移完成后对全仓做了一轮 review，修复批次如下：

1. **skill 文档陈旧注记清零**：`epub.style.demo.maintain` 已 ready，但
   `epub-style-demo-maintainer`、`epub-kindle-compatibility-checker`、
   `epub-english-typography-optimizer`、`epub-vertical-ruby-optimizer`、
   `epub-legacy-footnote-fallback` 五个 SKILL.md 与 `skills/README.md` 仍写
   "迁移中 / warn capability.not-implemented"，已全部改为现状
   （双模式、无需 `--output`，facts 形状按实跑核对）。
2. **pending 能力信封语义反转**：`epub run <pending-id>` 原返回
   `status: complete` + 退出码 0（仅 warn finding），现在返回
   `status: failed` + 退出码 1 + `error capability.not-implemented`；
   消息不再指向已删除的 Python oracle。`TestRunPendingCapabilityFails`
   锁定该语义。理由：只看 status/退出码的调用方不应把"未执行"当成功。
3. **INV-6 补上 v2 信封**：此前守卫只校验根 `testdata/` 下 3 个 v1 形状
   golden，而生产输出全是 v2 信封（SPEC §8.2 声称的
   `contracts/schemas/v2/envelope.schema.json` 不存在）。本轮落地该 schema，
   新增 `testdata/envelope/` 三个真实 CLI 捕获的 golden（源树校验、
   pending 拒绝、nav audit findings），并把 `archguard/schema_test.go`
   改为按 golden 的 schemaVersion 分发（v1 → v1 schema，v2 → v2 schema）。
   **这是规则 0 下的守卫演进**：只扩了覆盖面（v1 路径原样保留），
   变更经仓库所有者明确授权；已做负向验证（未知字段与非法枚举会被拦截）。
4. **CHANGELOG**：补 v0.3.0 条目（此前停在 v0.2.10，Go 重写没有记录）。
5. **复查后不动的项**：normalize dry-run 的 format 事件消息 `dry_run=false`
   是 Python parity 语义（阶段 1 刻意始终执行，dry-run 只作用于阶段 2），
   报告字段在逐字节 parity 范围内，不改。
| `docs/final/SPEC-go-architecture.md` | 头部加迁移完成标注（规则文字未动） |
