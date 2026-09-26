# SPEC-Go 架构

> 本文件是 Go 重写的**第一档硬约束**（等同 `docs/final/` 其它 SPEC）。
> 与旧对话、平台提示词、`archive/` 冲突时，以本文件为准。
> 当前接手状态与开放项见[Go 重写交接](../pipeline/go-rewrite-handoff.md)。Go 重写决策与复审记录归档于
> [`archive/meta/2026-08-30-go-rewrite-decisions.md`](../../archive/meta/2026-08-30-go-rewrite-decisions.md)、
> [`archive/meta/2026-09-go-rewrite-review-log.md`](../../archive/meta/2026-09-go-rewrite-review-log.md)；
> parity gate 与 Python → Go 映射等迁移期材料见 [`archive/meta/2026-09-go-parity-gate.md`](../../archive/meta/2026-09-go-parity-gate.md)。

---

## §0 给 AI 代理：这份文档怎么用

**动手前必读 §1 §2 §3 §4。** 按任务类型跳到 §6 对应模板，照模板改，不要自由发挥。

### 规则 0（最高优先级，无例外）

**禁止修改 `internal/archguard/` 下的任何文件。**

`archguard` 是架构的可执行定义。它红了，意味着**你的改动违反了架构**，不是守卫写错了。
正确反应是改你的代码；错误反应是改守卫、加白名单、注释掉断言、或给测试打 `t.Skip`。

如果你确信守卫本身有误，**停下来报告给人类**，不要自行放宽。

### 本文档的优先级

§1–§5、§8 是**规则**，必须遵守。§6 是**模板**，照抄。§7 仅指向历史归档。
§9 是**理由**，给人看的，你可以跳过。

改 Go 代码看 §1–§6；改 SKILL.md 或文档看 §8。
Go 代码的版本化风格与 API 选择另见 [`SPEC-go-modern-guidelines.md`](SPEC-go-modern-guidelines.md)；动手前须按该文档 §2 读取 `go.mod` 和完整适用规则，但它服从本 SPEC 的架构、EPUB safety、wire schema 与 lossless 约束。

---

## §1 依赖方向

**唯一的架构图。箭头只能向下，禁止向上、禁止跨层回指。**

**层号越小越靠上。规则：只能 import 层号严格更大的包。同层互相 import 一律禁止。**

| 层 | 包 | 职责 |
|:--:|---|---|
| 0 | `cmd/epub` | flag 解析、退出码。零业务逻辑 |
| 1 | `internal/pipeline` | 读契约 → 依 `requires` 排序 → 跑 stage → 汇总报告 |
| 2 | `internal/caps/*` | 一个 capability 一个包。**彼此禁止 import** |
| 3 | `internal/redline`、`internal/report`、`internal/extern` | 红线校验 / 报告构造 / 外部进程边界 |
| 4 | `internal/scan/*` | xhtml / css / opf 扫描器，只产出 `[]Edit` |
| 5 | `internal/book` | 内存中的 EPUB 模型（**含子包**：`internal/book/*` 按前缀同属层 5） |
| 6 | `internal/zipfs`、`internal/editset` | 容器读写（唯一磁盘边界） / 纯字节区间编辑 |

```
cmd/epub → pipeline → caps/* → {redline, report, extern} → scan/* → book → {zipfs, editset}
```

这张表是 `internal/archguard/deps_test.go` 里 `layer` 映射的**同一份事实**。
两者必须逐字一致；改一处必须同步改另一处，且改动需要人类审阅（见 §5.1.1）。

`layerOf` 用**最长前缀匹配**定层，因此新增子包（`internal/scan/<x>`、`internal/book/<x>`）
自动落在父前缀那一层，**不需要也不应该**往上面这张表里加行 —— 加了就与 `layer`
映射的键不再逐字一致。子包的职责登记在 §3 那张表里。

**由此而来的陷阱**：子包与父包同层，而同层互相 import 一律禁止。
`internal/book` 因此**不能** import `internal/book/pypath`（反之亦然）；
需要共用时把它下沉到更大的层号，或由上层调用方传入。

**新增包的流程**：先在本表定义它的层级 → 在 §3 定义它的职责边界 → 才允许写代码。
顺序反了，`TestLayerDirection` 会以"未登记的包"失败。

### 强制手段

- 全部实现放 `internal/`，外部无法 import。
- 层级由 `internal/archguard/deps_test.go` 用 `go/packages` 静态断言。**违反 = 测试红**。
- `internal/caps/a` 需要 `internal/caps/b` 的结果时，**不许 import**。改用契约里的 `requires` 字段，由 `pipeline` 负责排序并把上游结果传进来。

---

## §2 十条不变式

每条格式：**规则** / **为什么** / **守卫**。守卫是断言这条规则的自动化测试。

### INV-1 字节透传

**规则**：未被 `editset` 命中的 ZIP entry，必须用 `zip.Writer.Copy(*zip.File)` 原样搬运。
禁止「解压 → 重新压缩」未修改的 entry。

**为什么**：一本 49MB 的书通常只有几个 XHTML 被改。透传把 I/O 从"整包读 + 整包写"降到"改动部分"，
同时未修改文件字节完全不变，正文不变 gate 天然成立。

**守卫**：`archguard.TestRawPassthrough` — 跑一遍真实 capability，逐 entry 断言未修改项的
`CRC32` / `CompressedSize` / `Method` 与输入完全一致。

### INV-2 无整文档序列化

**规则**：`internal/scan/*` 只允许产出 `[]editset.Edit{Offset, Length, Replacement}`。
**禁止导出任何返回整份文档字节的函数**（`Marshal` / `Serialize` / `Render` / `String() []byte` 等）。

**为什么**：DOM 往返会静默改写命名空间前缀、自闭合标签、实体和 DOCTYPE。
Go 的 `encoding/xml` 尤其严重。只要不整篇重序列化，这类风险归零。

**守卫**：`archguard.TestNoWholeDocSerializer` — `go/ast` 扫描 `internal/scan/...` 的导出符号，
命中禁用名模式即失败。

### INV-3 单次落盘

**规则**：一次运行**只写一次输出 EPUB**。中间态一律留在内存。
只有 `internal/zipfs` 和 `internal/extern` 允许调用 `os.Create` / `os.WriteFile` / `os.OpenFile(写模式)`。

**为什么**：这是替换掉 Python 版 subprocess-per-stage 架构的核心。旧架构每个 stage 整包读写一次，
一本 49MB 的书跑 8 个 stage 产生约 800MB 无谓 I/O。

**守卫**：`archguard.TestSingleWrite` — AST 扫描，白名单外的包出现写句柄调用即失败。

### INV-4 stage 是函数不是进程

**规则**：`internal/caps/**` 禁止 import `os/exec`。所有外部进程调用必须经 `internal/extern`。

**为什么**：保持 stage 可组合、可测试、无进程启动开销。同时把外部工具依赖收敛到一处，
方便统一处理"工具不存在"的降级（`magick` / `oxipng` 在很多机器上没有）。

**守卫**：`archguard.TestNoExecInCaps` + `depguard` linter 配置。

### INV-5 红线闭包

**规则**：`contracts/capabilities/v1/*.json` 里出现过的每个 `redLines` 字符串，
都必须在 `redline.Registry` 里有对应的已注册校验器。

**为什么**：`redLines` 是这个项目的安全底线（`text` / `metadata` / `spine` / `anchors` / `cover` / `drm`）。
契约声明了却没实现，等于静默失去保护。

**守卫**：`archguard.TestRedlineClosure` — 读全部契约，取 `redLines` 并集，断言注册表全覆盖。
**新增契约时如果用了新红线，这个测试会立刻红。**

### INV-6 报告合 schema

**规则**：所有 JSON 输出必须通过 `contracts/schemas/v1/` 对应 schema 的校验。
`report` 包是唯一允许构造对外 JSON 的地方。

**为什么**：报告格式是 agent 与 GUI（未来）的契约。

**守卫**：`archguard.TestReportSchema` — golden 报告逐个过 schema。

### INV-7 无包级可变状态

**规则**：`internal/**` 禁止包级 `var`（可变）。stage 一律写成
`func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error)`。

例外白名单：`error` 哨兵值（`var ErrXxx = errors.New(...)`）、注册表（仅 `init()` 期写入）。

**为什么**：弱模型最常见的跑偏方式就是"加个全局变量传状态"。禁掉之后，数据流被迫走参数和返回值，
stage 保持可并行、可单测。

**守卫**：`archguard.TestNoPackageState`。

### INV-8 skills 是纯文档层

**规则**：`skills/**` 下不得出现任何 `.py` / `.sh`。所有可执行逻辑收口到 Go CLI。

**为什么**：执行面分散是本仓最大的历史负担 —— 同一个能力散落在
Python 脚本、shell 脚本、Swift 实现和 skill 文档里。收口到单一 CLI 之后，
skill 只需回答三件事：何时用、调什么、返回怎么读。

**守卫**：`archguard.TestSkillsHaveNoScripts`

### INV-9 SKILL.md 只引用真实存在的能力

**规则**：SKILL.md 里的 `epub run <capability-id>` 引用，其 id 必须存在于
`contracts/capabilities/v1/`。

**为什么**：这是全仓最容易腐烂的一处 —— 文档写着某命令，CLI 早已改名，
AI 照着跑就炸，而且**失败得很晚、很难归因**。

**守卫**：`archguard.TestSkillCommandsExist`

### INV-10 旧执行面棘轮

**规则**：`tools/parity/legacy-refs.txt` 记录文档里残留的旧执行面引用
（`python3 scripts/*.py`、`scripts/*.sh`）。该基线**只删不增**。

**为什么**：迁移期会很长，中途一定有人（包括 AI）图省事往文档里
再写一条 `python3 scripts/...`。棘轮保证迁移单调收敛。

当前基线：**零条目**（2026-08-29 归零）。`tools/parity/legacy-refs.txt` 保留为空基线文件，
使守卫在终态继续以零容忍扫描，而不是因文件缺失而 bootstrap-skip。

**守卫**：`archguard.TestNoLegacyExecutionSurface`

---

## §3 包职责表

| 包 | 能做 | 禁止 | 谁可以 import 它 |
|---|---|---|---|
| `cmd/epub` | flag 解析、退出码、把参数交给 pipeline | 任何业务逻辑、任何 EPUB 知识 | — |
| `internal/pipeline` | 读契约、依 `requires` 排序、跑 stage、汇总报告 | 直接操作 zip / 字节 | `cmd` |
| `internal/caps/<name>` | 单个 capability 的业务逻辑 | import 同层其它 `caps`、import `os/exec` | `pipeline` |
| `internal/scan/xhtml` | 扫描 XHTML，产出 `[]Edit` | 构造 DOM 树、整文档序列化 | `caps` |
| `internal/scan/css` | 扫描 CSS，产出 `[]Edit` | 同上 | `caps` |
| `internal/scan/opf` | 解析 OPF / container / nav（**只读**结构信息）；从 `[]TocEntry` **生成**新的 nav / NCX 文档文本（`BuildNav`/`BuildNCX`） | 写回 OPF（改 OPF 也走 Edit）；把**解析得来**的文档重序列化 | `caps`, `redline` |
| `internal/editset` | 收集、排序、冲突检测、应用字节区间编辑 | 理解 XML/CSS 语义 | `caps`, `scan` |
| `internal/redline` | 6 条红线校验器 + 注册表 | 修改 book | `pipeline`, `caps` |
| `internal/report` | 构造并序列化 run-report | 业务判断 | 全部上层 |
| `internal/extern` | 起 `magick`/`oxipng`/`java`/`pyftsubset`；工具缺失时降级 | 业务判断 | `caps` |
| `internal/book` | 内存 EPUB 模型：entry 表 + 惰性内容 + 脏标记 | 碰磁盘 | 全部上层 |
| `internal/book/pypath` | Python 侧 `urllib.parse` / `posixpath` / `xml.sax.saxutils` 的路径、URL、转义语义（纯函数，只依赖标准库） | 任何 EPUB 语义判断、任何 I/O | 全部上层（**不含** `internal/book` 自己：同层） |
| `internal/zipfs` | `OpenRaw`/`Copy`/`CreateRaw`；唯一磁盘边界 | 理解 EPUB 语义 | `book` |
| `internal/archguard` | 架构守卫测试 | **任何人不得修改**（见规则 0） | — |

---

## §4 禁止清单

违反以下任何一条，即使编译通过、测试通过，也判定为架构跑偏：

1. ❌ 修改 `internal/archguard/` 下任何文件（规则 0）
2. ❌ 为了让守卫通过而加白名单、`t.Skip`、注释掉断言
3. ❌ 在 `caps` 之间直接 import（要依赖就用契约 `requires`）
4. ❌ 引入 DOM 库做整文档往返（`encoding/xml` 的 `Marshal`、任何 html5 tree builder 的序列化输出）
5. ❌ 把中间结果写到临时文件再读回来（INV-3）
6. ❌ 新增包级可变变量（INV-7）
7. ❌ 在 `caps` 里直接 `exec.Command`（INV-4）
8. ❌ 新能力缺少 golden 测试与全项 redline（§5.2）
9. ❌ 为"顺手"而重构不属于当前任务的包
10. ❌ 新增第三方依赖而未在 §9.3 依赖预算里说明

---

## §5 自动化守卫与新能力验收

### 5.1 archguard

单一文件目录 `internal/archguard/`，把 §2 十条不变式编码成测试。CI 必跑。

```
internal/archguard/
  doc.go               包文档（规则 0 的正式声明）
  helpers_test.go      共享的 AST / 文件遍历工具
  deps_test.go         §1  依赖方向（层级 + caps 互不 import + cmd 保持薄）
  passthrough_test.go  INV-1 字节透传
  serialize_test.go    INV-2 无整文档序列化
  disk_test.go         INV-3 单次落盘
  exec_test.go         INV-4 caps 内禁 os/exec
  redline_test.go      INV-5 红线闭包
  schema_test.go       INV-6 报告合 schema
  state_test.go        INV-7 无包级可变状态
  skill_test.go        INV-8 / INV-9 / INV-10（skill 层与迁移棘轮）
```

**伴随守卫包 `internal/docguard`**：同为纯测试包、位于 §1 层级图之外，接替已删除的 Python
meta-validator（`validate_skills_basic.py` / `validate_contracts.py` / `validate_ai_entrypoints.py`），
守卫文档与契约层的机械规则：

- `TestSkillFrontmatter` / `TestOpenAIYAMLShape` / `TestSkillIndexTables`：SKILL.md frontmatter 只含 `name`、`description`
  且与目录名一致，正文恰为 §8.4 四段且顺序固定；`agents/openai.yaml` 是扁平字符串 `interface:` map 且
  `default_prompt` 提及 `$<skill>`；`skills/README.md` 与 `docs/learn/04-skills.md` 的技能表与目录一一对应。
- `TestFootnoteClassVocabulary`：`skills/*/SKILL.md` 与 `docs/how-to/*.md` 只使用 `SPEC-实现约束.md` §1 声明的弹注 class 词汇。
- `TestContractsValid`：`contracts/capabilities/v1/*.json` 合 `capability-manifest.schema.json`（最小子集校验器），
  文件名 = id、schema 引用存在、`requires` 指向真实 capability、`legacySkillSlugs` 指向真实 skill 目录。
- `TestAIEntrypointsCanonical`：`AGENTS.md` 是唯一维护源，其余入口文档只跳转不复制规则。

守卫红了应修文档或契约；改动 `internal/docguard/` 本身同样需要人类审阅。CI 在 archguard 之后以独立步骤运行
`go test ./internal/docguard/`。

### 5.1.1 守卫的真正牙齿

文档里的"禁止修改 archguard"对弱模型只是软约束。真正的强制来自仓库配置，需一并落地：

- `.github/workflows/` 里 archguard 作为**独立必过 job**，不与其它测试合并，失败信息醒目
- `CODEOWNERS` 把 `internal/archguard/**` 划给人类审阅
- PR 模板加一条勾选项：「本次改动是否触碰 archguard？若是，说明理由」

没有这三样，规则 0 只是一句话。

**落地状态（2026-08-26 已完成）**：

| | 文件 | 作用 |
|---|---|---|
| CI job | `.github/workflows/archguard.yml` | 独立必过 job；PR 触碰 archguard 时额外打警告 |
| 审阅归属 | `.github/CODEOWNERS` | `internal/archguard/`、本 SPEC、棘轮基线、`contracts/` 均需人类审阅 |
| PR 模板 | `.github/pull_request_template.md` | 四项架构自检勾选 + 棘轮进度栏 |

### 5.2 新能力验收

新能力验收 = golden 测试 + 全项 redline。

---

## §6 任务模板

### 6.1 新增或修改一个 capability

实现应保持范围窄，并至少同步维护：

1. `contracts/capabilities/v1/<id>.json` 与 `contracts/parameters/v2/cli.json`；
2. `internal/caps/<name>/` 的实现、单元测试与 golden；
3. `internal/pipeline/register.go` 的注册和参数映射；
4. `internal/pipeline/` 端到端测试及 `testdata/` 中的 v2 报告 golden；
5. 受影响的 SKILL.md 与索引。

新能力验收 = golden 测试 + 全项 redline。

实现签名**固定**为：

```go
package <name>

// Run 执行本 capability。禁止修改 b 之外的任何状态。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
    edits, err := scanPhase(b)      // 1. 扫描：只读 b，产出 []editset.Edit
    if err != nil {
        return report.Result{}, err
    }
    if err := b.Apply(edits); err != nil {   // 2. 应用：唯一的写入口
        return report.Result{}, err
    }
    return report.Result{...}, nil           // 3. 报告：不落盘，交给 pipeline
}
```

**三段式是强制的**：扫描（只读）→ 应用（唯一写点）→ 报告（不落盘）。不要合并、不要打乱顺序。

### 6.2 修改一个 XHTML 变换

只改 `internal/scan/xhtml/` 下对应文件里的**扫描逻辑**，产出更准确的 `[]Edit`。

- ✅ 可以改：匹配什么、替换成什么、区间怎么算
- ❌ 不许改：`Edit` 结构、`editset` 的应用逻辑、任何试图"先解析成树再输出"的写法

### 6.3 新增一条红线

```
1. internal/redline/<name>.go        实现 Validator 接口
2. internal/redline/register.go      注册
3. 在用到它的 contracts/*.json 的 redLines 数组里加名字
```

顺序无所谓，但**三处必须同时到位**，否则 INV-5 守卫会红。

### 6.4 接一个外部工具

只在 `internal/extern/` 加。必须实现：工具不存在时的**显式降级**（返回 `ErrToolMissing`，
由调用方决定是跳过还是失败），不许静默忽略，不许 panic。

---

## §7 历史迁移映射

本节内容已归档至 [`archive/meta/2026-09-go-parity-gate.md`](../../archive/meta/2026-09-go-parity-gate.md)。

## §8 CLI 契约与 skill 层

### 8.1 定位

**Go CLI 是唯一执行面，同时服务人和 AI agent。** `skills/` 退化为纯文档层。

这意味着 CLI 的**返回结构是全系统最重要的接口** —— 它不再只是给人看的日志，
而是 AI 据以决策下一步的输入。设计它的标准因此变了：稳定 > 好看，可判定 > 信息全。

### 8.2 统一返回信封

所有命令返回同一形状，取代现存的 22 种 ad-hoc 报告：

```json
{
  "schemaVersion": "2",
  "capability": "epub.structure.normalize",
  "status": "complete | planned | failed | approval-required | cancelled",
  "input":  {"path": "...", "sha256": "..."},
  "output": {"path": "...", "sha256": "..."},
  "facts":    {},
  "findings": [
    {"level": "error|warn|info", "id": "...", "title": "...",
     "detail": "...", "location": "..."}
  ],
  "events":   [{"step": "...", "status": "...", "message": "..."}],
  "nextCommands": ["epub run epub.package.nav.audit --input out.epub"]
}
```

两个设计要点：

- **`findings[]` 是统一收口**。旧的 lint 数组、text-invariance 纯文本行，全部归到这里。
- **`nextCommands[]` 是给 agent 的**。现存 `epub_refinement_harness.py` 的
  `suggested_next_commands` 已是这个思路，本次提升为全局约定 ——
  CLI 主动告诉 agent 下一步该跑什么，而不是让 agent 猜。

信封的 JSON Schema 落在 `contracts/schemas/v2/envelope.schema.json`，
由 INV-6 守卫。**v1 schema 保留不动**（历史契约；v1 不再有运行时消费者）。迁移期报告兼容方案见历史归档。

### 8.3 命令面

规范形态 —— **SKILL.md 只许用这一种**，因为只有它能被 INV-9 对账：

```
epub run <capability-id> [--input ...] [--output ...] [--dry-run] [--json]
epub capabilities [--json]          列出全部能力及其参数
```

人类用的便捷别名（`epub normalize book.epub`）可以有，但**不进 SKILL.md、不进文档**。
理由：别名是给手指的，`run <id>` 是给机器的；文档面向机器。

### 8.4 SKILL.md 模板

每个 SKILL.md 固定四段，不多不少：

```markdown
## 何时用
（判据，不是功能描述）

## 调什么
epub run <capability-id> --input <书> --output <新书>

## 返回怎么读
status / findings[].level / facts 里本能力特有的字段

## 依据返回怎么判断
findings 里出现 X → 下一步做 Y
status == approval-required → 停下来问人
status == planned → 计划已通过红线且未写出；审阅 facts/findings 后再决定是否实跑
```

第四段是关键：**skill 的价值在"怎么判断"，不在"怎么调用"**。
调用方式 CLI 自己 `--help` 就能说清楚；判断依据说不清楚，AI 就会乱来。

### 8.5 退出码

信封换了，但退出码语义**必须与现状一致** —— pre-commit hook 和 `epub_text_gate.py`
的调用方都依赖它：

| 码 | 含义 |
|:--:|---|
| 0 | 成功，无 error 级 finding；写出能力的成功 dry-run 为 `planned` |
| 1 | 失败、存在 error 级 finding，或取消（status=cancelled） |
| 2 | `approval-required` —— 需要人工批准才能继续 |
| 3 | 用法错误（参数非法、文件不存在） |

---

## §9 设计理由（人类阅读，AI 可跳过）

### 9.1 为什么是 Go 而不是 Rust / TS / Swift

决策依据是四条约束的加权：跨平台单文件分发、构建占盘、AI 辅助迭代速度、（已排除的）移动端。

- Rust 领域库更强（`lol_html` 流式改写、`lightningcss`），但 `target/` 轻松 1.5–3GB，
  且 AI 生成的 Rust 编译不过的比例显著更高。移动端排除后，它的最大优势失效。
- Swift 已实测 `swift/.build` 占 302MB（仅 3 个依赖），Windows 支持痛苦，且缺 CSS 解析库
  导致仓库里手写了 800+ 行 CSS parser。已决定删除。
- TS/Bun 的 `parse5` + `postcss` 生态最贴合，但需要运行时或 60–90MB 的编译产物。
- Go 的 `archive/zip` 自 1.17 起提供 `Writer.Copy` / `File.OpenRaw` / `Writer.CreateRaw`，
  **恰好就是 INV-1 需要的原语**。这是最终倾向 Go 的具体技术理由，而非泛泛的"Go 简单"。

### 9.2 为什么 INV-2 能抵消 Go 的 XML 短板

Go 的 `encoding/xml` 往返丢信息严重，本来是选 Go 的最大风险。
但正文不变 gate 本来就要求做**字节区间外科手术**而非整篇重序列化——
一旦采用那个架构，"序列化器弱"这个短板就不再有作用面。
风险被架构消解，而不是被语言解决。

### 9.3 依赖预算

目标：**标准库优先**。现有 Python 侧 17k 行是纯 stdlib，Go 侧应保持同等克制。

允许清单（新增需在此登记并说明）：
- `archive/zip`、`encoding/json`、`regexp` — stdlib
- `golang.org/x/text` — 仅 `unicode/norm`（redline 文本归一化对齐 Python
  `unicodedata.normalize("NFC")`）与 `encoding/ianaindex`（structure_normalize
  的 decode_text 编码链回编）。2026-08-29 登记。
- `github.com/tdewolff/parse/v2` v2.8.16 — CSS Syntax Level 3 lexer/parser
  仅用于 CSS 语法诊断与 token/span adapter 的保守扫描；不用其序列化样式表。
  上游项目采用 MIT 许可。
- `golang.org/x/sys` v0.36.0 — 仅 `internal/zipfs/rename_noreplace_{darwin,linux,windows}.go`
  使用，在唯一磁盘写边界做原子"不覆盖"重命名：darwin 走 `unix.RenameatxNp` +
  `RENAME_EXCL`，linux 走 `unix.Renameat2` + `RENAME_NOREPLACE`，windows 走
  `windows.MoveFileEx`（不带 `MOVEFILE_REPLACE_EXISTING`）；三者均不回退到 `os.Rename`。
  其他包不得 import。2026-09-01 登记。上游项目采用 BSD-3-Clause 许可
  （已核对 `go.sum` 与模块 `LICENSE`）。
- JSON Schema 校验库 — 仅 `archguard` 和 `report` 测试用，不进主二进制

**注意**：Go 的 `regexp` 是 RE2，不支持 lookahead / lookbehind / 反向引用。
现存 Python 侧仅 4 个文件用到，需手工改写为显式匹配。

### 9.4 不迁移的部分

字体子集化依赖 `fonttools`，Go 和 Rust 都没有能替代的成熟库
（`hb-subset` 是 C，`klippa`/`skrifa` 未成熟）。
`tools-font/epub-font/` **保持 Python 独立 provider**，由正式 capability
`epub.font.subset` 经 `internal/extern` 起子进程调用；provider 不进入 Go 发行包。能力只把验证后的
manifest 字体 entry 写入内存候选，最终 ZIP 仍由 Go pipeline 在红线通过后一次写出。
`tools-font/coverage-detector/` 也保持 Python 独立项目，由 `internal/extern` 调用。这与
`AGENTS.md` 现有策略一致。
