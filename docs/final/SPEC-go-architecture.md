# SPEC-Go 架构

> 本文件是 Go 重写的**第一档硬约束**（等同 `docs/final/` 其它 SPEC）。
> 与旧对话、平台提示词、`archive/` 冲突时，以本文件为准。
>
> **迁移状态（2026-08-29）**：W0–W5 全部完成。16 个 A 类 capability 已在 Go 侧注册并通过
> parity gate；棘轮（§2 INV-10）归零；`scripts/`、`adapters/`、`swift/`、`gui/`、
> `tools/parity/` 的迁移脚手架与 Python 环境文件已按 §7.5 顺序删除；仅保留零条目的
> `tools/parity/legacy-refs.txt`，使 INV-10 在终态继续扫描而不是 bootstrap-skip。§5.2 的 `--legacy-report`
> 与 §7 的迁移映射保留为历史依据；`epub_lint.py` 无对应契约，其职责由
> `epub.package.nav.audit` + `epub redline` + CI EPUBCheck 承担（裁决见交接文档）。

---

## §0 给 AI 代理：这份文档怎么用

**动手前必读 §1 §2 §3 §4。** 按任务类型跳到 §6 对应模板，照模板改，不要自由发挥。

### 规则 0（最高优先级，无例外）

**禁止修改 `internal/archguard/` 下的任何文件。**

`archguard` 是架构的可执行定义。它红了，意味着**你的改动违反了架构**，不是守卫写错了。
正确反应是改你的代码；错误反应是改守卫、加白名单、注释掉断言、或给测试打 `t.Skip`。

如果你确信守卫本身有误，**停下来报告给人类**，不要自行放宽。

### 本文档的优先级

§1–§5、§8 是**规则**，必须遵守。§6 是**模板**，照抄。§7 是**映射表**，查表用。
§9 是**理由**，给人看的，你可以跳过。

改 Go 代码看 §1–§6；改 SKILL.md 或文档看 §8 和 §7.3。
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
| 5 | `internal/book` | 内存中的 EPUB 模型 |
| 6 | `internal/zipfs`、`internal/editset` | 容器读写（唯一磁盘边界） / 纯字节区间编辑 |

```
cmd/epub → pipeline → caps/* → {redline, report, extern} → scan/* → book → {zipfs, editset}
```

这张表是 `internal/archguard/deps_test.go` 里 `layer` 映射的**同一份事实**。
两者必须逐字一致；改一处必须同步改另一处，且改动需要人类审阅（见 §5.1.1）。

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

**为什么**：报告格式是 agent、GUI（未来）和 parity gate 三方的契约。

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

当前基线：**124 处引用，分布在 41 个 markdown 文件**。

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
| `internal/scan/opf` | 解析 OPF / container / nav（**只读**结构信息） | 写回 OPF（改 OPF 也走 Edit） | `caps`, `redline` |
| `internal/editset` | 收集、排序、冲突检测、应用字节区间编辑 | 理解 XML/CSS 语义 | `caps`, `scan` |
| `internal/redline` | 6 条红线校验器 + 注册表 | 修改 book | `pipeline`, `caps` |
| `internal/report` | 构造并序列化 run-report | 业务判断 | 全部上层 |
| `internal/extern` | 起 `magick`/`oxipng`/`java`/`pyftsubset`；工具缺失时降级 | 业务判断 | `caps` |
| `internal/book` | 内存 EPUB 模型：entry 表 + 惰性内容 + 脏标记 | 碰磁盘 | 全部上层 |
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
8. ❌ 不写 parity 测试就迁移一个 capability（§5）
9. ❌ 为"顺手"而重构不属于当前任务的包
10. ❌ 新增第三方依赖而未在 §9.3 依赖预算里说明

---

## §5 自动化守卫与 parity gate

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

### 5.2 parity gate（重写期间的核心安全网）

Go 实现和现存 Python 实现对**同一批 EPUB** 跑同一个 capability，比对输出。

```
tools/parity/
  run.sh              对每本样本书跑 python 版与 go 版，产出两份 EPUB + 两份报告
  compare.go          比对规则见下
```

比对分三级，**全绿才允许删对应的 Python 脚本**：

| 级别 | 比对内容 | 要求 |
|---|---|---|
| P1 | 输出 EPUB 的**文本块哈希**（复用 `validate_text_invariance.py` 的分块与归一化规则） | 完全一致 |
| P2 | 报告输出（忽略时间戳与绝对路径） | 完全一致 |

P2 有个绕不开的矛盾：Go 版**故意**换了输出信封（§7.3），却又需要逐字节比对来保证正确性。

**对策 —— `--legacy-report` 脚手架**：迁移期内，每个 Go capability 额外支持一个
隐藏 flag，按旧脚本的原始形状输出报告。parity harness 只用这个 flag，
正式输出走 §9.2 的新信封。

- 好处：P2 保持**逐字节**强度，不降级成"语义等价"这种模糊判据
- 移除触发条件：对应 Python 脚本删除时，同步删掉该 capability 的 `--legacy-report`
- 这是**唯一被批准的临时脚手架**。不要用同样理由再引入第二个
| P3 | 输出 EPUB 逐 entry 的 `CRC32` | 允许差异，但每处差异必须在 `tools/parity/allow.md` 里有书面理由 |

P3 允许差异是因为 Go 版会**更少**改动字节（透传），这是预期的改进而非回归。

### 5.3 迁移完成的定义

一个 capability 判定"迁移完成"，必须同时满足：
1. Go 实现存在且注册
2. parity gate 三级达标
3. 契约里 `redLines` 全部有对应校验器（INV-5 自动保证）
4. 对应 Python 脚本已删除

在此之前 Python 脚本**不许删**。

---

## §6 任务模板

### 6.1 新增/迁移一个 capability

**只碰这 5 处，不要动别的文件：**

```
1. contracts/capabilities/v1/<id>.json      写契约（kind / redLines / requires / permissions）
2. internal/caps/<name>/<name>.go           实现 Run()
3. internal/caps/<name>/<name>_test.go      单测 + golden
4. internal/caps/register.go                加一行注册
5. testdata/<name>/                         golden 输入输出
```

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

## §7 Python → Go 迁移映射

### 7.1 迁移单元总览

`contracts/capabilities/v1/` 共 **22 个** capability，按有无 Python 实现分三类：

| 类别 | 数量 | 处理方式 |
|---|---|---|
| A. 有 Python 实现，需迁 Go | 16 | 逐个走 §6.1 模板 + §5.2 parity gate |
| B. 纯 AI skill，无专属实现 | 6 | 不建 `caps/` 包，但**依赖的两个通用校验器必须变成 CLI 子命令** |
| C. 能走统一 adapter 路由的 | 3 | 见下方警告 |

B 类（不建 `caps/` 包）：`epub.kindle.compatibility.check`、`epub.literary.structure.format`、
`epub.notes.legacy-fallback`、`epub.typography.english.optimize`、
`epub.vertical.ruby.optimize`、`epub.style.demo.maintain`。

> 这 6 个没有专属脚本，但**不等于没有执行需求**：它们依赖
> `scripts/validate-epub-style-demo.sh`（被 12 处引用）和
> `scripts/validate-popup-notes.sh`（9 处）。
> 这两个 shell 校验器是全仓引用最密集的入口，**必须优先变成 Go 子命令**。

> ⚠️ **`adapters/python/provider-catalog.v1.json` 只登记了 3 条**，而
> `public-entrypoints.v1.json` 覆盖 16 个 capability。也就是说现状里**只有 3 个 capability
> 能真正通过统一 envelope 路径跑起来**，其余靠直接调脚本 CLI。
> Go 版的 `pipeline` 必须让 **全部 16 个** 走同一条路由 —— 这是重写要修掉的现存缺陷，
> 不要照搬这个分裂。

### 7.2 Python 模块 → Go 包映射

| Python | 行数 | → Go 包 | 备注 |
|---|---|---|---|
| `epub_lib.py` | 211 | `internal/book` + `internal/scan/opf` | zip I/O 部分下沉到 `zipfs`，OPF 操作归 `scan/opf` |
| `epub_package/core.py` | 682 | `internal/book` + `internal/caps/{cover,metadata,merge,split}` | `navigation/package_io/references` 是空转发层，**不要复制这层** |
| `epub3_conversion/core.py` | 1185 | `internal/caps/migrate_epub3` | `converter/navigation/notes/package/xhtml` 同为转发层，合并掉 |
| `epub_ai/core.py` | 537 | `internal/caps/audit` + `internal/report` | `Report` 累积器 → `report` 包 |
| `epub_xhtml_transforms.py` | 184 | `internal/scan/xhtml` | **已经是字符串级最小 diff 变换，明确标注 "no DOM reserialize"** |
| `epub_css_cleanup.py` | 507 | `internal/scan/css` + `internal/caps/css_cleanup` | |
| `epub_structure_tool.py` | 829 | `internal/caps/structure_normalize` | |
| `validate_text_invariance.py` | 515 | `internal/redline`（全部 6 条） | 见 §7.3 陷阱 2 |
| `epub_lint.py` | 510 | `internal/caps/lint` | 见 §7.3 陷阱 1 |
| `epub_content_analysis.py` | 586 | `internal/caps/content_analyze` | |
| `epub_cleanup_pipeline.py` | 438 | `internal/pipeline` | **整个 subprocess 编排层消失**，这是本次重写的主要收益点 |
| `epub_cleanup_loop.py` | 711 | `internal/pipeline`（循环控制） | |
| `epub_text_gate.py` | 51 | **删除** | 它只是 `validate_text_invariance.py` 的 subprocess 封装；Go 里直接函数调用 |
| `epub_ai_harness.py` / `epub_package_tool.py` / `epub3_oneclick_converter.py` | 17/68/19 | **删除** | 三个都是 "Backward-compatible CLI façade"，零逻辑 |
| `tools-font/coverage-detector/` | — | **不迁**，保持 Python | 依赖 `fonttools`，见 §9.4；经 `internal/extern` 调用 |

> **反模式警告**：Python 侧有大量「转发层」（`epub3_conversion/{navigation,notes,package,xhtml}.py`、
> `epub_package/{navigation,package_io,references}.py`、三个 façade 脚本），
> 函数体只有 `return core.xxx(...)`。**Go 版不要复制这层结构。**
> 它们是历史兼容包袱，不是架构。

> **重复实现已裁决（2026-08-26）：保留 `epub3_conversion`，`epub3_migration_harness.py` 作废。**
>
> | | A `epub3_migration_harness.py` | B `epub3_conversion/` |
> |---|---|---|
> | 规模 | 438 行 | 1465 行 |
> | 能力 | 版本号、`dcterms:modified`、nav 生成、spine idref —— 四件事 | 上述全部 **+** 多看注释、Sigil 遗留注释、正文字体锁定感知、封面 properties、guide→landmarks、媒体类型规范化、XHTML 外壳规范化 |
> | 契约指向 | 无 | `epub.package.migrate.epub3` → `epub3_migration_apply_harness.py` |
> | pipeline 调用 | 无 | `epub_cleanup_pipeline.py` → `epub3_oneclick_converter.py` |
> | 测试 | 173 行 | 510 行 |
>
> 决定性依据是第三、四行：**A 在执行链路上是孤儿**，只被文档和自身测试引用；
> 契约与 pipeline 都走 B。而 B 超出的部分恰是本手册的领域核心（多看注释、字体锁定、弹出注释）。
>
> **A 唯一独有的是 plan 模式**（列出将做的动作而不写盘）。它作为
> `--dry-run` 特性保留，但不构成保留整套实现的理由。
> Go 侧只实现 B 的行为，A 连同 `test_epub3_migration_harness.py` 一并删除。

> **架构红利：`--dry-run` 对每个 capability 都近乎免费。**
> §6.1 强制的三段式（扫描 → 应用 → 报告）天然支持它 ——
> 跑扫描阶段、跳过 `b.Apply(edits)`、把 `edits` 摘要进报告即可。
> 因此 `--dry-run` 是**全局 flag，不是某个 capability 的特性**，
> 由 `pipeline` 统一实现，`caps` 不需要各自处理。

### 7.3 报告格式：从「逐字节保持」改为「统一信封」

> **本节的结论在 2026-08-26 反转过一次，理解这个反转很重要。**

早先的结论是「legacy 报告格式必须逐字节保持，不要顺手规整」，理由是下游解析会碎。
但下游只有两类：**互相调用的 Python 脚本**，和**告诉 AI 该跑什么的 SKILL.md**。
既然二者全部重写，这个理由就不成立了 —— 那两个「陷阱」不是要保护的契约，
是要清掉的历史包袱。

**现行结论：Go 版统一到 §9.2 的单一信封。**

被统一掉的两个畸形格式：

| 旧形态 | 出处 | 归到信封的哪里 |
|---|---|---|
| JSON 顶层是**数组** | `epub_lint.py` | `findings[]` |
| **纯文本行 + 退出码**，无 JSON | `validate_text_invariance.py` 与 7 个 `validate_*.py` | `findings[]` + `status` + 退出码 |

**代价与对策**：这样一来 parity gate 的 P2 就没法再逐字节比对了。
对策见 §5.2 —— 迁移期加一个 `--legacy-report` 临时脚手架，让 P2 保持满强度，
迁移结束时随 `scripts/` 一起删掉。

**仍然不许动的**：退出码语义。`epub_text_gate.py` 之外还有 pre-commit hook
依赖退出码，信封换了但 0/非 0 的含义必须一致（细则见 §9.5）。

### 7.4 迁移波次

| 波次 | 内容 | 完成判据 |
|---|---|---|
| W0 | `zipfs` + `book` + `editset` + `archguard` 全绿 | INV-1 行为测试通过；49MB 样本书透传 I/O 实测 |
| W1 | `redline` 六条 + `report` | INV-5 闭包成立；陷阱 2 的纯文本格式逐字节一致 |
| W2 | `scan/{xhtml,css,opf}` + 3 个只读 capability（audit / lint / content_analyze） | parity P1+P2 全绿 |
| W3 | 写入型 capability（structure_normalize / css_cleanup / migrate_epub3 / 4 个 package 操作） | parity 三级全绿 |
| W4 | `pipeline` 编排 + `cmd/epub` + §9.2 信封 | 端到端 parity 全绿 |
| W5 | 重写 19 个 SKILL.md + 41 个文档；两个 shell 校验器变子命令 | 棘轮归零（`legacy-refs.txt` 清空）；**此时才允许删 `scripts/`** |

W0 是唯一必须由高能力模型完成的波次（架构地基 + 守卫）。W1–W3 每个 capability 都被
§6.1 模板和 archguard 夹住，适合派发给较低能力模型逐个推进。

W5 是纯文档改写，被 INV-9（引用的能力必须存在）和 INV-10（棘轮只减不增）夹住，
同样适合低能力模型批量推进 —— 改错了会立刻红。

> **`scripts/` 的删除时机**：必须等到 W5 棘轮归零。
> 只要还有一个文档写着 `python3 scripts/...`，删了就是制造断链。

---

### 7.5 终态仓库形态

Go 重写完成后，仓库只剩**文档层 + Go 实现 + 明确不迁的 Python 工具**。

**保留**

| 目录 | 终态内容 |
|---|---|
| `docs/` | 文档层。唯一的说明来源 |
| `skills/` | **只有 SKILL.md**（INV-8 守卫） |
| `contracts/` | capability 契约 + schemas（v2 为正式，v1 迁移期保留） |
| `templates/` | demo fixture。第一档硬约束的证据来源，不可删 |
| `references/` | 样本 EPUB（49M）。测试与 parity 的输入 |
| `records/` | 排版决策记录 |
| `archive/` | 第三档参考 |
| `tools-font/coverage-detector/` | Python + fonttools，**明确不迁**（§9.4） |
| `cmd/` + `internal/` | Go 实现 |
| `.github/` + `hooks/` | CI 与 git hook（hook 内容改为调 `epub` CLI） |

**删除**

| 目录 | 体积 | 删除时机 | 依据 |
|---|---|---|---|
| `swift/` | 303M | **立即可删** | 已裁决；执行链路无下游依赖 |
| `gui/` | 14 个文件 | 立即，随 `swift/` | 已 PARKED，且只依赖 `swift/` |
| `scripts/` | 75 py + 3 sh | **W5 棘轮归零后** | 只要还有一处文档/hook 引用它，删了就是断链 |
| `adapters/` | 24K | W4 后 | 被 `epub capabilities` 子命令取代 |
| `.venv` / `uv.lock` / `pyproject.toml` / `.python-version` | 21M | `scripts/` 删除后 | `tools-font/` 有独立的 uv 项目，不受影响 |
| `tools/parity/` | — | W5 完成后 | 删除迁移脚手架；保留零条目的 `legacy-refs.txt` 作为终态守卫输入 |

**顺序约束**（不可交换）：

1. `swift/` + `gui/` —— 无前置，随时可删。收回 303M，是构建占盘问题的最大单笔
2. `scripts/` —— 必须等棘轮归零
3. `adapters/` —— 必须等 CLI 的 `capabilities` 子命令可用
4. Python 环境文件 —— 必须等 `scripts/` 删完
5. `tools/parity/` —— 最后

### 7.6 三个执行面必须同时收敛

`scripts/` 的引用不止在 SKILL.md 里。删除前这三处都要清零：

| 执行面 | 规模 | 是否被棘轮覆盖 |
|---|---|---|
| SKILL.md 与文档 markdown | 142 处 / 44 文件 | ✅ |
| `hooks/pre-commit.epub-handbook` | 7 处 | ✅（后补覆盖） |
| `adapters/python/*.v1.json` 的 provider catalog | 2 个文件 | ❌ 由 W4 的 CLI 取代，不走棘轮 |

> git hook 是最容易被漏掉的一处 —— 它不是 markdown，早期版本的棘轮扫不到它。
> 现在已纳入。**新增执行面时先问：棘轮扫得到吗？**

---

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
  "status": "complete | failed | approval-required",
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
由 INV-6 守卫。**v1 schema 保留不动**，供迁移期的 `--legacy-report` 使用。

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
```

第四段是关键：**skill 的价值在"怎么判断"，不在"怎么调用"**。
调用方式 CLI 自己 `--help` 就能说清楚；判断依据说不清楚，AI 就会乱来。

### 8.5 退出码

信封换了，但退出码语义**必须与现状一致** —— pre-commit hook 和 `epub_text_gate.py`
的调用方都依赖它：

| 码 | 含义 |
|:--:|---|
| 0 | 成功，无 error 级 finding |
| 1 | 失败，或存在 error 级 finding |
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
- JSON Schema 校验库 — 仅 `archguard` 和 `report` 测试用，不进主二进制

**注意**：Go 的 `regexp` 是 RE2，不支持 lookahead / lookbehind / 反向引用。
现存 Python 侧仅 4 个文件用到，需手工改写为显式匹配。

### 9.4 不迁移的部分

字体子集化依赖 `fonttools`，Go 和 Rust 都没有能替代的成熟库
（`hb-subset` 是 C，`klippa`/`skrifa` 未成熟）。
`tools-font/coverage-detector/` **保持 Python 独立项目不动**，由 `internal/extern` 起子进程调用。
这与 `AGENTS.md` 现有策略一致。
