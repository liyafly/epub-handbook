# Go CLI 重构架构与迁移蓝图（前期背景）

> 状态：**已由 `docs/final/SPEC-go-architecture.md` 收口；本文件仅保留前期设计背景**
>
> Go 重写的依赖方向、不变式、任务模板、命令面和删除顺序以
> [`docs/final/SPEC-go-architecture.md`](../final/SPEC-go-architecture.md) 为准；当前进度见
> [`go-rewrite-handoff.md`](go-rewrite-handoff.md)。本文件与 SPEC 冲突时不得执行本文件。
>
> 决策日期：2026-08-22
>
> 适用对象：仓库维护者、Codex、Claude Code 及其他能够修改本仓库的 AI agent
>
> 目标产物：面向 Windows、macOS、Linux 的一个公开 Go CLI，以及随发行包交付的 contracts、skills 和少量私有 provider

本文件是从现有 Python/Swift 实现迁移到 Go CLI 的详细执行蓝图。根目录
`AGENTS.md` 仍是架构总纲和协作约束的唯一维护源；本文件负责记录目标实现、边界、阶段、
验收门槛和删除旧代码的顺序。发生冲突时，EPUB 行为以 `templates/`、`docs/final/`、
`reader-matrix.yaml` 和相关 skill 为准，迁移不能借机改写已经验证过的产品规则。

## 1. 已确定的决策

以下决策不再作为实现任务中的开放问题：

1. 公开 CLI 和主要执行核心使用 **Go**。
2. 采用**模块化单体**，只发布一个用户直接调用的命令 `epub-handbook`；不设计动态插件 ABI。
3. Windows 是第一优先平台，同时正式支持 macOS 和 Linux。
4. 项目只面向桌面电脑工作流；GUI、iOS、Android 和其他移动端 target 不在范围内。
5. Swift 已冻结，不再承接新 capability；达到本文件定义的 Go parity gate 后删除 `swift/` 和 `gui/`。
6. Python 不一次性删除。迁移期它是行为基线和差分 oracle；迁移完成后只保留确有生态优势的
   字体 provider、必要的仓库维护脚本和尚未达到 parity 的临时入口。
7. 最终发行物追求“**一个公开命令**”，不强求“磁盘上只有一个文件”。字体 provider 可以作为
   `libexec` 私有组件随平台包一起交付，用户不需要自行安装 Python、`uv` 或 Rust。
8. EPUB ZIP、OPF、nav、XHTML、普通文本分析、红线校验和大部分结构修改应原生进入 Go。
9. 字体 cmap、子集化、variable font、WOFF/WOFF2 等能力优先复用 FontTools；不因为主程序使用
   Go 就重造字体工具链。
10. CSS 使用 Go 业务层和纯 Go 语法解析器。第一候选是
    [`github.com/tdewolff/parse/v2/css`](https://pkg.go.dev/github.com/tdewolff/parse/v2/css)，
    但必须先通过 §11.2 的 lossless spike；失败时按规定顺序评估替代项，不允许退回正则解析复杂 CSS。
11. Rust 不进入首版主工程。未来若真实 fixture 证明 Go CSS 路径不足，可以增加独立 Rust sidecar，
    但不得通过 CGO/FFI 嵌进 Go 主进程。
12. CLI 核心不调用模型 API、不要求网络，也不把某个 agent 产品写死。AI 通过 contracts 和 skills
    调用确定性 capability，自迭代由有界状态机和机器报告支撑。

## 2. 目标与非目标

### 2.1 目标

- 为普通用户和 AI agent 提供同一个稳定 CLI。
- 发行包解压后即可运行，尤其不能要求 Windows 用户配置开发工具链。
- 所有公开 capability 可发现、可版本化、可用 JSON 调用、可审计。
- 默认不原地修改唯一输入，所有写操作支持 dry-run、计划摘要和新输出路径。
- 未修改的 EPUB ZIP entry 尽量 raw-copy；内存占用与被修改的最大文本资源相关，而不是与整本书解压后大小相关。
- 工作区、报告、输入摘要、输出摘要和每次 gate 结果可以供 agent 下一轮继续判断。
- 允许 safe pipeline 自迭代，但必须限制迭代次数、检测无进展并保留完整 journal。
- 每个旧实现只有在相应 capability 达到行为、契约、安全和多平台 parity 后才能删除。
- contracts、skills、实现和发行包之间有自动一致性检查。

### 2.2 非目标

- 不做 EPUB 阅读器或浏览器排版引擎。
- 不根据 CSS parser 的结果推断 Kindle、Apple Books、Thorium、KOReader 的最终视觉表现；结论仍需真实阅读器实测。
- 不绕过 DRM，不猜测未知加密资源，不弱化正文不变 gate。
- 不做长期运行的 daemon、远程服务或内置 MCP server；这些可以在稳定 CLI 之上另建 adapter。
- 不在首轮重构中追求所有代码都是 Go。
- 不把 CSS 压缩、浏览器 vendor prefix、现代 Web syntax lowering 作为 EPUB 清洗默认动作。
- 不保留 Swift 作为未来移动端预留层。
- 不让 skill 直接依赖 `internal/` 包、旧 Python 文件路径或私有 provider 的位置。

## 3. 当前基线与迁移原则

当前仓库有 22 个 capability manifest，Python 提供最广的 CLI/验证基线，Swift 已实现部分 archive、
redline、popup note 和 CSS cleanup 能力，字体覆盖分析位于 `tools-font/coverage-detector/`。现有实现
仍是迁移期的事实基线，但不再代表目标架构。

迁移必须遵循以下原则：

- **先冻结 fixture，再移植代码。** 每个 capability 先收集成功、失败、边界和恶意输入样本。
- **先只读，后写入。** archive/preflight/inspection 先稳定，再实现重写器。
- **先计划，后提交。** transformer 必须产生可序列化 plan；apply 验证输入 digest 和 plan digest。
- **差分只比较语义事实。** ZIP 时间戳、压缩流和无关格式可能不同；正文、manifest、spine、链接、
  anchor、声明顺序等受保护事实必须明确比较。
- **未知即保留或停止。** 不可识别的 CSS、XML、加密或编码不得被“尽力修复”后静默写回。
- **一项 capability 一次迁移。** 不允许以“统一重写”为理由同时替换多个尚无 parity fixture 的入口。
- **旧实现只减不增。** 除修复安全事故和维护 parity fixture 外，不再向 Swift 增加 capability。

## 4. 目标发行形态

平台发行包使用如下逻辑结构：

```text
epub-handbook-<version>-<os>-<arch>/
├── epub-handbook[.exe]          # 唯一公开命令
├── libexec/
│   └── font-provider[.exe]      # 可选，平台自包含，不加入 PATH
├── share/
│   └── epub-handbook/
│       ├── contracts/
│       ├── skills/
│       └── presets/
├── LICENSES/
├── SBOM.spdx.json
└── checksums.txt
```

约束：

- Go 主程序默认 `CGO_ENABLED=0`；若未来某项依赖需要 CGO，必须新建 ADR 并证明所有目标平台的构建、
  签名和干净机安装仍可复现。
- `libexec` 是实现细节。skill 只能调用 `epub-handbook run <capability-id>`，不得直接调用 provider。
- Go 程序通过可执行文件相对路径、安装前缀或显式环境覆盖查找 share/libexec；不得假设当前工作目录。
- 开发 checkout 可以从仓库根目录读取 contracts/skills；发行模式优先使用随包资源。
- `epub-handbook doctor --format json` 必须报告资源位置、provider 版本、可执行性和缺失依赖。
- 不把 Python 环境压成 base64 后在每次运行时临时解压；provider 应在构建期成为平台自包含目录或二进制。

## 5. 目标仓库布局

Go module 放在仓库根目录，避免再增加 `go/` 这一层，也便于 contracts、skills 和 testdata 使用稳定相对位置：

```text
go.mod
go.sum
cmd/
└── epub-handbook/
    └── main.go
internal/
├── app/                 # use-case 编排，不包含 CLI 文案
├── cli/                 # 参数解析、stdout/stderr、exit code
├── contracts/           # manifest/schema 加载和强类型 request/result
├── capability/          # registry、descriptor、handler 接口
├── archive/             # ZIP 扫描、限制、raw copy、writer
├── package/             # container.xml、OPF、manifest、spine、nav/NCX
├── document/            # XHTML/XML token、anchor、link、text projection
├── stylesheet/          # CSS adapter、模型、分析、patch
├── inspection/          # 只读检查器
├── validation/          # redline、正文不变、lint gate
├── transform/           # plan/apply 与具体结构修改
├── pipeline/            # 有界状态机、恢复、无进展检测
├── workspace/           # journal、digest、atomic commit
├── provider/            # 私有 provider 协议与进程管理
├── release/             # 安装布局、资源发现、版本信息
└── testkit/             # 只供测试使用的 fixture/build helper
testdata/
├── epub/
├── css/
├── xhtml/
├── contracts/
└── malicious/
contracts/               # 机器契约唯一源
skills/                  # agent 指令唯一源
scripts/                 # 迁移期 Python oracle 与仓库维护入口
tools-font/              # 独立字体 provider 源码
```

### 5.1 依赖方向

只允许以下方向：

```text
cmd -> cli -> app -> capability implementations
                     |
                     +-> archive/package/document/stylesheet
                     +-> validation/workspace/provider/contracts
```

- `archive` 不依赖 `package`，只认识 ZIP entry。
- `package` 可以依赖 `archive`，不依赖具体 capability。
- `validation` 可以读取 domain model，但不能反向调用 CLI。
- `provider` 只实现通用进程协议；字体业务 adapter 放在 provider 的子包或具体 capability 中。
- capability 之间不直接调用 CLI。复合 pipeline 通过 Go handler 接口复用能力。
- `internal/` 不读取 `docs/final/` 文本来决定规则；可执行规则必须显式编码并由 demo/evidence 测试约束。

### 5.2 依赖选择规则

- 标准库能清晰完成的功能优先标准库，例如 `archive/zip`、`crypto/sha256`、`encoding/json`、`log/slog`。
- CLI 首版使用 `flag.FlagSet` 组织子命令，不为了命令框架引入全局状态和隐式配置。
- 第三方库必须有可再分发许可证、版本 pin、维护迹象、错误返回，并能在 Windows/macOS/Linux 的
  `CGO_ENABLED=0` 构建中使用。
- 第三方类型不能穿透 repository adapter。业务包只使用本仓库定义的接口和结构。
- 新增依赖的 PR 必须记录用途、替代方案、许可证和移除成本。

## 6. 核心运行模型

### 6.1 Capability handler

所有 capability 在 Go 内部实现同一概念接口：

```go
type Handler interface {
    Describe() Descriptor
    Plan(context.Context, Request) (Plan, error)
    Apply(context.Context, Plan) (Result, error)
}
```

- detector、validator 和只读 planner 的 `Apply` 执行读取并产出报告，不写 EPUB。
- transformer 的 `Plan` 必须完整列出预期写入、删除、移动和 gate；`Apply` 不得重新推导不同计划。
- `Plan` 带输入 SHA-256、capability/version、options、权限、计划摘要和 plan digest。
- apply 前再次计算输入 SHA-256；不一致返回 stale-plan 错误。
- 每个 handler 显式声明 `readOnly`、`network`、`providers`、`resourceLimits` 和 `redLines`。

### 6.2 公共 CLI

机器稳定表面：

```sh
epub-handbook version --format json
epub-handbook catalog --format json
epub-handbook doctor --format json
epub-handbook run <capability-id> --request request.json --result result.json
epub-handbook plan <capability-id> --request request.json --plan plan.json
epub-handbook apply --plan plan.json --result result.json
epub-handbook pipeline cleanup --request request.json --result result.json
epub-handbook skills list --format json
epub-handbook skills install --target <target> --destination <directory>
```

可以提供面向人的 convenience subcommand，但它们必须立即转换成同一个 request 并调用 app 层，不能形成
第二套实现。例如 `inspect book.epub` 只是 `run epub.package.nav.audit` 的简写。

机器调用约束：

- stdout 恰好输出一个 JSON document；日志、进度和诊断写 stderr。
- `--result` 指定文件时，stdout 返回包含结果路径、digest 和最终状态的小型 envelope。
- 不询问交互问题，不弹窗，不使用颜色；是否为 TTY 不改变 JSON 结构。
- 所有文件参数接受绝对路径和 `file:` URI；进入 app 层前规范化为绝对路径。
- 输出路径必须显式指定。首版不支持原地覆盖；`--force` 只能替换明确输出路径，不能跳过 redline。
- request 中未声明的环境变量不影响结果；locale、timezone 和 ZIP 时间策略必须显式或固定。
- `catalog` 可以列出尚未安装 provider 的 capability，但必须标记 `availability` 和原因。

### 6.3 Exit code

| code | 意义 |
| ---: | --- |
| 0 | capability 完成，所有阻断 gate 通过；warning 可以存在 |
| 1 | 输入可处理，但检查发现阻断问题或产物未通过 domain validation |
| 2 | CLI 用法、JSON 或 schema 错误 |
| 3 | 输入、工作区、环境或 provider 不可用 |
| 4 | 安全策略阻断，例如未知加密、路径穿越、stale plan、未授权写入 |
| 5 | 内部错误或违反不变量 |
| 130 | 收到取消信号，事务未提交 |

JSON 中同时写稳定字符串 error code，例如 `validation.failed`、`provider.unavailable`、
`archive.path_traversal`；agent 不得通过解析人类错误文案判断分支。

## 7. Contracts 与版本策略

现有 `contracts/capabilities/v1/*.json` 是 capability 身份来源，不能从 Go 函数名反向生成 ID。

### 7.1 请求最小字段

统一 request 至少包含：

```json
{
  "schemaVersion": "1",
  "capability": "epub.structure.normalize",
  "capabilityVersion": "1.0.0",
  "runId": "optional-caller-id",
  "inputs": [
    {
      "uri": "file:///absolute/path/input.epub",
      "kind": "epub",
      "contentDigest": "sha256:..."
    }
  ],
  "output": {
    "uri": "file:///absolute/path/output.epub"
  },
  "options": {},
  "limits": {},
  "permissions": {
    "write": true,
    "network": false
  }
}
```

现有只接受单一 `artifact-reference` 的 manifest 暂时保留。多输入 capability 在 contracts v2 中增加
统一 request schema；不能用逗号路径或隐藏环境变量绕过 schema。

### 7.2 Result envelope

结果至少包含：

- `schemaVersion`、`capability`、`capabilityVersion`、`runId`；
- `status: complete | failed | blocked | canceled`；
- 输入与输出 artifact reference 及 SHA-256；
- `findings[]`，每项有稳定 code、severity、artifact、location 和 evidence；
- `actions[]`，区分 planned/applied/skipped；
- `gates[]`，包含 gate 名称、结果、证据路径；
- `metrics`，包含耗时、峰值内存（可测时）、读取/写入 entry 数和字节数；
- `providerRuns[]`，包含 provider 名称、协议版本、工具版本和状态；
- workspace 和 journal 的位置。

### 7.3 版本规则

- 增加可选字段是向后兼容变更，更新 schema patch/minor 版本。
- 删除字段、改变含义、改变默认行为或枚举值需要新 schema major 目录。
- capability ID 一旦公开不得改名；替代时新增 ID，并在旧 manifest 写 deprecation 信息。
- Go struct、JSON schema 和至少一组 golden JSON 必须三方一致。
- release CI 必须对发行包中的 contracts 与仓库源做 digest 一致性检查。

## 8. Workspace、事务与恢复

每次运行创建独立目录：

```text
work/<book-or-run>/runs/<run-id>/
├── request.json
├── environment.json
├── plan.json
├── journal.jsonl
├── input/
│   └── artifact.json
├── staging/
├── reports/
├── output/
│   └── artifact.json
└── result.json
```

事务状态固定为：

```text
created -> inspected -> planned -> applying -> validating -> committed
                                      |             |
                                      +-> aborted <-+
```

要求：

- journal 只追加，每条记录有 sequence、UTC timestamp、stage、event code 和相关 digest。
- `plan.json` 完成后不可修改；重新计划生成新 run。
- 写 EPUB 时先写与目标同文件系统的临时文件，完成所有 gate 后 atomic rename。
- 如果目标已存在，默认停止；明确 `--force` 时先将旧目标移动到该 run 的 recoverable backup。
- 收到取消信号时停止调度新工作、关闭 writer、删除未完成临时输出并将 run 标记 canceled。
- `doctor` 或专门的 `workspace inspect` 能识别残留 run；首版不自动继续 applying 中的事务。
- before artifact 只读，任何阶段都不得修改或重新打包它来伪造基线。

## 9. EPUB archive 与内存模型

### 9.1 读取

打开 archive 后先扫描 central directory，只保存 entry metadata，不读取所有 entry 内容。预检至少检查：

- `mimetype` 是否存在、首项、stored 且内容正确；
- duplicate name、绝对路径、`..`、反斜杠歧义、NUL、大小写冲突；
- entry 数量、单 entry 解压大小、总解压大小和压缩比；
- `META-INF/container.xml`、rootfile、OPF 是否可解析；
- encryption.xml、实际目标资源和允许的字体 obfuscation 类型；
- CRC、截断和 ZIP 结构异常。

资源限制必须在解压前后双重检查，不能相信 ZIP header 声明值。

### 9.2 重写

transformer 使用两阶段实现：

1. scan/plan：建立引用图和 action plan；
2. rewrite/commit：按计划写一个新 archive。

未修改 entry 使用 Go `archive/zip.Writer.Copy` raw-copy，避免解压再压缩。修改的 XML/CSS/XHTML entry
才进入有限缓冲；大图片和字体默认不进入 Go heap。`mimetype` 始终第一个、stored、无 extra field。

同一次 capability 只产生一个最终重写 archive。结构规范化不能先写格式化 EPUB、再读回并写第二个
反混淆 EPUB；阶段之间传递 plan，在一个 writer 中提交。

### 9.3 确定性

- entry 顺序默认保留输入顺序，新增 entry 使用稳定排序插入规定位置。
- 未修改 entry 保留 compression method、CRC、mode 和允许保留的 metadata。
- 新增/修改 entry 的时间戳使用 request 指定值或固定 release policy，不使用当前本地时间作为隐式输入。
- 同一输入 digest、同一工具版本、同一 request 应产生相同语义报告；是否要求 ZIP 字节级复现由 capability 声明。

## 10. XML、XHTML 与文本

- container.xml、OPF、nav、NCX 使用 namespace-aware XML token/模型，禁止用正则做结构修改。
- 只读文本投影和结构修改分离。文本投影必须定义空白、Ruby、注释、MathML、SVG 和脚注处理规则。
- 对 XHTML 的修改优先生成最小 token patch；只有明确需要结构化重写的节点才允许序列化局部文档。
- 序列化前后运行非文字 DOM/属性、正文、anchor、nav/NCX label 和引用图 gate。
- UTF-8 BOM、XML declaration、DOCTYPE、实体和 namespace prefix 的处理写进 fixture，不由 parser 默认行为决定。
- `decode(..., errors=replace)` 不允许进入写回路径；无法无损解码时停止或保持原 entry 不变。
- Unicode 分类、正规化和字素规则使用明确版本和 `golang.org/x/text` 中必要的独立包；不得默认改变正文 normalization form。

## 11. CSS 架构

CSS 分成四层，禁止混在一个 cleanup 函数中：

```text
csssource   原始 bytes、encoding、token、source span
cssmodel    rule/declaration/reference 的仓库中立模型
csspolicy   EPUB/reader 规则、finding、建议与计划
cssrewrite  对原始 bytes 应用不重叠 patch
```

### 11.1 Go 原生范围

以下能力进入 Go：

- token/rule/declaration 扫描和语法诊断；
- comment/string/custom property 安全处理；
- `url()`、`@import`、`@font-face src` 的资源引用提取与路径改写；
- `font-family` 等已知声明的读取和精确替换；
- 重复 stylesheet 指纹和结构 shape；
- 明确安全的规则抽取、shared/override 计划；
- XHTML stylesheet link、OPF manifest 与 CSS entry 的一致修改；
- 支持范围内 selector 的 scope prefix；
- Kindle/EPUB 属性规则校验。

默认禁用：

- minify；
- vendor prefix 自动增删；
- 针对现代浏览器的 syntax lowering；
- 未经 reader fixture 支撑的 shorthand 重写；
- 删除“看起来未使用”的规则；
- 对未知 at-rule 或 property 做格式化后写回。

### 11.2 Parser 选型 spike

第一候选为 `github.com/tdewolff/parse/v2/css`。在把它加入正式 `go.mod` 前，AI 必须完成一个独立
spike，并把测试保留为 `internal/stylesheet` 的回归测试。fixture 至少覆盖：

- comment、BOM、`@charset`、CRLF；
- escaped identifier 和 escaped string；
- string/data URL 中的 `; { } , :`；
- `@namespace`、`@font-face`、`@media`、`@supports`、`@page`；
- custom property、`var()`、`calc()`、`!important`；
- namespace selector、attribute selector、pseudo class/element、selector list；
- EPUB 使用的 `epub|type`、竖排、Ruby、字体和分页声明；
- 缺分号、未闭合 comment/string/block 等畸形输入。

硬门槛：

1. no-op parse 的输出 bytes 必须与输入完全相同；
2. 单声明替换只改变目标 source span；
3. 未识别规则原样保留；
4. 所有错误变成结构化 finding，不 panic；
5. fuzz target 能持续运行；
6. 当前 `scripts/test_epub_css_cleanup.py` 和 Swift/Python parity CSS fixture 的语义事实全部通过；
7. Windows、macOS、Linux 的 `CGO_ENABLED=0` 测试通过。

如果失败，依次评估 `modernc.org/css`、纯 tokenizer + 本仓 grammar，最后才评估 Rust sidecar。
禁止在未记录失败 fixture 的情况下凭偏好换库。

### 11.3 Lossless patch

所有 CSS 写操作生成：

```go
type Patch struct {
    Start       int
    End         int
    Replacement []byte
    ReasonCode  string
    Evidence    SourceLocation
}
```

- patch 坐标基于原始 bytes，而不是 rune index。
- patch 必须按位置排序、验证不重叠并倒序应用。
- apply 前验证目标 slice digest，防止 stale offset。
- 如果一次操作会触发整份 stylesheet 重新序列化，必须显式标记 `formattingImpact=whole-document`，
  默认安全策略拒绝执行。
- selector scoping 必须解析 selector list，禁止简单 `strings.Split(",")`。

### 11.4 层叠与 selector

完整 cascade 包含 specificity、source order、`!important`、inheritance、inline style、media condition、
namespace 和 reader 行为。首版不声称 Go core 是浏览器级 cascade engine。

- 普通 selector 只读匹配可以在 spike 中评估 Cascadia 或独立 selector adapter。
- XHTML 写回仍由 XML/token 层完成，不能把 `x/net/html` 重新序列化结果覆盖 EPUB XHTML。
- 字体 coverage 的复杂 selector/cascade 在迁移期继续由字体 provider 负责，并报告 unresolved selector。
- parser 成功不等于阅读器兼容；任何最终规则仍服从 demo 与 reader-matrix。

## 12. 字体与外部 provider

### 12.1 边界

Go core 负责：

- 从 EPUB manifest/CSS 收集字体引用；
- 提取全书字符集合和角色上下文；
- 校验路径、media type、obfuscation metadata 和输出计划；
- 调用 provider、验证 provider 报告 schema、将结果合并进统一 result。

字体 provider 负责：

- cmap、IVS、name/OS2 等表读取；
- coverage、fallback 和缺字分析；
- subset、variable font 实例化；
- WOFF/WOFF2 与受支持字体格式转换；
- 可选 HarfBuzz shaping/subset 验证。

字体 provider 初始实现继续使用 Python + FontTools；其 Python runtime 和依赖在 release build 中锁定并随包交付。
最终用户不得需要 `uv sync`。

### 12.2 Provider 协议

Go 只通过版本化进程协议调用 provider：

```sh
libexec/font-provider --request request.json --result result.json
```

- JSON 中传文件路径和 digest，不传 base64 字体数据。
- request/result schema 位于 `contracts/providers/v1/`。
- provider stdout 不作为协议通道；日志写受限 stderr，结果只写指定文件。
- Go 设置 timeout、最大 stderr、工作目录和允许访问的路径；默认不提供网络。
- provider 返回版本、依赖版本、耗时、输入/输出 digest 和稳定错误码。
- provider 崩溃、超时、结果缺失或 schema 不合法都变成 `provider.*` 错误，不得让 Go 主进程崩溃。
- provider 输出文件仍需由 Go 做路径、安全和格式 gate；不能因它是内置 provider 就信任。

### 12.3 何时允许字体原生化

只有同时满足以下条件才将某项字体能力移入 Go/Rust：

- 有 FontTools oracle 和授权 fixture；
- 对 CJK、variation、collection、WOFF2、损坏表和 license/embedding flags 有覆盖；
- 新实现的报告与产物达到 parity；
- 多平台内存、速度或维护成本有可测收益；
- 不扩大 DRM/obfuscation 边界。

“减少语言数量”本身不是原生化理由。

## 13. 有界自迭代与 AI 调用

主 CLI 是确定性工具，不内置模型。它为任何 agent 暴露以下循环：

```text
inspect -> plan -> apply -> validate -> compare -> journal -> decide
   ^                                                        |
   +---------------- continue with next plan ---------------+
```

`pipeline cleanup` 可以自动执行已证明幂等且无需主观判断的 safe pass；需要内容判断、正文校订或阅读器
实测时必须停下并返回 `needsDecision`。

循环约束：

- 默认最大 3 次 mutation iteration，request 可降低，不能高于仓库策略上限。
- 每轮计算 normalized state digest；连续两轮 digest 相同即 `no-progress` 停止。
- 同一个 action code 对同一目标最多执行一次，除非前一轮明确产生新的前置条件。
- gate 失败后不能自动扩大 allow-list、修改规则或删除 validator。
- AI 建议和 CLI 事实分离：AI 可以生成 request/plan 选择，CLI 报告实际 action/evidence。
- 下一轮只读取上一轮结构化 result、plan、journal 和授权输入，不通过分析 stderr 猜测状态。
- 所有模型生成的自由文本只进入 decision record，不能直接成为 ZIP entry patch。

skills 的调用示例只能使用：

```sh
epub-handbook catalog --format json
epub-handbook run <capability-id> --request <request.json> --result <result.json>
```

不能写 `python3 scripts/...`、`swift run ...` 或 `libexec/...` 作为最终稳定入口。

## 14. 安全与资源限制

所有 archive capability 默认有配置上限，具体默认值由 Phase 2 基准确定并写进 contracts/policy：

- 最大 entry 数；
- 单 entry 与总解压字节数；
- 最大压缩比；
- 最大路径长度与目录深度；
- 单 XML/CSS/XHTML 缓冲上限；
- 最大 finding/action 数，超出时截断并明确 `truncated=true`；
- 最大 provider 运行时间和输出大小；
- 最大 pipeline 迭代数与并发 worker 数。

实现规则：

- worker pool 有界，默认保守；不能为每个 ZIP entry 无限制启动 goroutine。
- context cancellation 必须贯穿 archive reader、handler 和 provider。
- 网络权限默认 `none`；source intake 需要网络时必须是独立 capability 和显式 request 权限。
- 临时文件创建在确定的 workspace，文件权限遵循最小权限。
- 错误和日志不得包含正文片段、密钥、完整字体数据或用户目录之外的环境信息。
- ZIP path 使用 POSIX 规则验证，落盘路径另做平台安全 join；Windows drive/UNC/保留名要有 fixture。

## 15. Skills 与安装

发行包中的 skills 仍以仓库 `skills/` 为唯一源。发布过程只做可验证复制或生成索引，不维护第二份手写内容。

`skills install` 应：

1. 读取随包 skill catalog；
2. 校验目标 agent、CLI 最低/最高兼容版本和所需 capability；
3. 输出 dry-run 安装计划；
4. 只向用户明确指定的 destination 写入；
5. 保留已存在文件，除非 `--replace` 且 digest 匹配可管理来源；
6. 写安装 manifest，支持审计和卸载；
7. 不自动修改 shell profile、系统 PATH 或 agent 全局配置。

skill 可以包含调用 wrapper script，但 wrapper 只负责定位 `epub-handbook`、构造 request 和转发退出码，
不能复制 capability 业务逻辑。安装后的 skill 通过 `catalog` 检查能力是否存在，不根据 CLI 版本猜测。

## 16. 平台与发布矩阵

首个正式 release gate：

| 平台 | 架构 | 级别 | Go 主程序 | 字体 provider |
| --- | --- | --- | --- | --- |
| Windows 11 | amd64 | Tier 1 | 必须 | 必须 |
| macOS | arm64 | Tier 1 | 必须 | 必须 |
| macOS | amd64 | Tier 1 | 必须 | 必须 |
| Linux glibc | amd64 | Tier 1 | 必须 | 必须 |
| Windows | arm64 | Tier 2 | 构建+smoke | provider 完成后升级 |
| Linux | arm64 | Tier 2 | 构建+smoke | provider 完成后升级 |

每个平台 CI 至少验证：

- `version/catalog/doctor`；
- 路径含空格、中文、长文件名；
- CRLF 和 shell quoting；
- 只读 preflight；
- 一个 dry-run transformer；
- 一个真实写入、redline 和 text invariance；
- provider discovery/timeout/error；
- 从发行 archive 解压后在干净目录运行。

发布物使用 `.zip`（Windows）和 `.tar.gz`（macOS/Linux），生成 SHA-256、SBOM 和第三方许可证清单。
Windows signing、macOS codesign/notarization 在公开大规模分发前成为 release gate；包管理器 manifest 在首个稳定
archive 后增加，不阻塞核心迁移。

### 16.1 构建资源约束

二进制体积不是首要指标，但 clean build 的磁盘、缓存和耗时必须可测、可回收：

- Go 主程序和字体 provider 分成独立 build job；构建 Go CLI 不创建或复制 Python virtualenv。
- Go 依赖由 `go.mod`/`go.sum` 锁定；正常开发不提交 `vendor/`，除非后续 ADR 证明离线复现必须使用。
- `go generate` 不能隐式下载工具、模型或大型数据；生成器版本必须锁定，并有 `--check` 或生成结果 diff gate。
- release job 每个 OS/arch 使用独立输出目录，只保留最终 package、symbols（若单独发布）、SBOM 和报告。
- 字体 provider 每个平台只构建一次，再复制进对应 package；不能在每个 Go capability 测试中重复打包 runtime。
- Phase 1 记录无缓存 clean build 的 wall time、peak RSS、workspace bytes 和 Go cache bytes；后续 release 记录趋势。
- CI 可以缓存 module/build cache，但必须另有定期 clean build，防止缓存掩盖未声明的生成依赖。
- 下载依赖完成后，Go 主程序应能在禁网环境构建；release provider 构建所需外部资产必须有 digest 和许可证记录。
- 不要求维护者安装 Docker 才能完成普通 Go build/test；平台 provider 打包可以使用隔离 CI job。
- toolchain 版本在 Phase 1 创建 `go.mod` 时锁定为当时受支持的稳定 Go 版本，并由 CI 和 release metadata 明示；
  升级 toolchain 是独立维护变更，不与 capability 移植混合。

## 17. 测试与性能

### 17.1 测试层次

- unit：domain model、路径、schema、patch、错误码；
- golden：request/result、manifest catalog、计划和报告；
- fixture：真实最小 EPUB/CSS/XHTML/字体场景；
- differential：Go 与 Python/Swift oracle 的语义事实对比；
- property/fuzz：ZIP path、central directory、XML token、CSS token/patch、URI resolution；
- integration：完整 capability 和 pipeline；
- release smoke：平台包与私有 provider；
- reader evidence：仍按 demo/reader-matrix 流程，不由普通 unit test 替代。

### 17.2 基准

Phase 0/1 先记录当前 Python/Swift 基线，再设置阈值，禁止拍脑袋写绝对性能目标。至少测：

- 小/中/大 EPUB wall time；
- cold start；
- peak RSS；
- 临时磁盘峰值；
- raw-copy 与修改 entry 数；
- CSS parser 的 throughput/allocation；
- provider 启动与字体分析时间。

目标性质：

- 只读扫描峰值内存不随 EPUB 总解压大小线性增长；
- 大图片/字体不进入主进程完整 buffer；
- 一次 transformer 最多存在输入 archive、一个临时输出和必要 provider 输出；
- 同一进程复合 pipeline 不通过子进程反复启动 Go 自己；
- 性能回归超过已建立阈值时 CI 产生报告，不能用提高阈值掩盖未知原因。

## 18. Capability 迁移表

阶段编号对应 §19。每项完成后必须在同一 PR 更新状态和证据链接。

| Capability | 类型 | 目标包 | 阶段 | 主要 oracle / 特殊门槛 |
| --- | --- | --- | --- | --- |
| `epub.package.nav.audit` | validator | `inspection/package` | 2 | preflight、OPF/nav/NCX fixture |
| `epub.text.content.analyze` | detector | `inspection/content` | 2 | Python content analyzer、文本投影 fixture |
| `epub.layout.audit` | planner | `inspection/layout` | 3 | AI harness、结构化 finding parity |
| `epub.kindle.compatibility.check` | validator | `validation/kindle` | 3 | demo + Kindle reader evidence |
| `epub.structure.normalize` | transformer | `transform/structure` | 4 | dry-run/path map/text invariance |
| `epub.package.migrate.epub3` | transformer | `transform/epub3` | 4 | migration harness + redlines |
| `epub.metadata.edit` | transformer | `transform/metadata` | 4 | metadata/spine/cover gates |
| `epub.cover.replace` | transformer | `transform/cover` | 4 | cover manifest/guide/nav gates |
| `epub.package.merge` | transformer | `transform/merge` | 4 | 多输入 request v2、anchor/reference graph |
| `epub.package.split` | transformer | `transform/split` | 4 | 多输出 contract、独立 lint |
| `epub.notes.popup.normalize` | transformer | `transform/notes` | 5 | Swift/Python parity、popup validator |
| `epub.notes.legacy-fallback` | transformer | `transform/notes` | 5 | standard/legacy 双路径 fixture |
| `epub.css.layering.optimize` | transformer | `transform/styles` | 5 | §11 parser spike、CSS parity、幂等 |
| `epub.typography.optimize` | transformer | `transform/typography` | 5 | CJK demo/reader matrix |
| `epub.typography.english.optimize` | transformer | `transform/typography` | 5 | English fixture/text invariance |
| `epub.vertical.ruby.optimize` | transformer | `transform/vertical` | 5 | vertical/Ruby demo + reader evidence |
| `epub.alite.convert` | transformer | `transform/alite` | 5 | A-lite demo、fallback gate |
| `epub.literary.structure.format` | transformer | `transform/literary` | 5 | decision record、文本不变 |
| `epub.image.layout.optimize` | planner | `inspection/image` | 5 | figure 主路径与 reader evidence |
| `epub.source.intake` | planner | `inspection/source` | 5 | source bundle audit contract |
| `epub.style.demo.maintain` | planner | `release/demo` | 5 | repo-only capability、完整 demo build |
| `epub.font.coverage.analyze` | detector | `provider/font` | 6 | FontTools/tinycss2 oracle、provider package |

## 19. 分阶段实施计划

### Phase 0：冻结决策和基线

本文件所在变更完成该阶段的文档部分。实现前还需：

- 给每个 capability 登记当前入口、fixture、测试命令和已知差异；
- 固定一组不含版权正文的 benchmark corpus；
- 保存当前 Python/Swift 报告和语义事实，不保存机器绝对路径；
- 将 Swift 标记 frozen，CI 只维持现有 parity，不新增功能；
- 建立迁移状态文件，例如 `contracts/migration/go-capabilities.v1.json`。

退出门槛：22 个 capability 全部有 owner phase、oracle 和至少一个 fixture 或明确缺口。

### Phase 1：Go 骨架与发行烟测

实现：

- 根 `go.mod`；
- `cmd/epub-handbook`；
- `version/catalog/doctor`；
- manifest/schema loader；
- typed error、JSON envelope、exit code；
- 三平台 CI 和最小 release archive；
- contracts/share/libexec discovery。

退出门槛：干净 Windows 11、macOS、Linux 环境能从 release archive 运行三个只读命令；catalog 与现有
22 个 manifest 一致；尚未迁移的能力明确显示 unavailable，不能假装成功。

### Phase 2：Archive、package 与只读基础

实现：

- central directory scanner、安全限制、digest；
- container/OPF/nav/NCX 只读模型；
- `epub.package.nav.audit`；
- 稳定文本投影和 `epub.text.content.analyze`；
- archive/path/XML fuzz tests。

退出门槛：fixture 和 Python oracle 语义 parity；大资源流式测试证明不整本解压进内存；恶意 ZIP corpus
返回稳定安全错误。

### Phase 3：Redline 与只读规划

实现 metadata、spine、anchor、cover、DRM、非文字 DOM/属性、注释、图片和正文不变相关 validator，
再迁移 layout 与 Kindle 检查。

退出门槛：当前清洗流程用到的所有阻断 gate 都能由 Go 只读执行；Python 与 Go 对 golden corpus 的
pass/fail 一致，差异必须有批准记录，不能通过降低规则消除。

### Phase 4：事务重写器与 package transformer

先实现通用 plan/apply/raw-copy/atomic commit，然后按 §18 顺序迁移结构规范化、EPUB3、metadata、cover、
merge、split。每项独立合并。

退出门槛：dry-run 无副作用；stale plan 被拒绝；写入后 lint/redline/text invariance 通过；取消与失败不留下
半成品；峰值临时空间符合 §17 基线。

### Phase 5：CSS、XHTML 与排版能力

先完成 §11 parser spike，再迁移 popup、legacy fallback、CSS layering、typography、vertical/Ruby、A-lite、
literary、image planner、source intake 和 demo maintenance。

退出门槛：每个 transformer 幂等；未知 CSS/XHTML 保留或明确停止；所有关联 demo validator 通过；需要
真实阅读器证据的结论已有对应 reader-matrix 条目。

### Phase 6：字体 provider

实现 provider v1 schema、Go process manager、自包含平台构建和 FontTools provider；迁移 coverage capability。

退出门槛：用户机器无需 Python/uv；三平台 `doctor` 和字体 fixture 通过；provider 缺失、崩溃、超时和恶意
输出均被隔离；许可证/SBOM 完整。

### Phase 7：Skills 切换与候选发布

- 所有可分发 skill 改用统一 Go CLI；
- 增加 skill compatibility 检查和 installer；
- Python/Swift 进入只读 differential CI；
- 制作 release candidate，执行干净机、路径、provider、清洗 pipeline 和 reader smoke。

退出门槛：仓库活跃 skill 不再要求用户执行 Python/Swift；Go release 在 Tier 1 平台完成完整主流程。

### Phase 8：删除 Swift 与收缩 Python

先删除 Swift，再按 capability 删除已替代 Python 入口。删除不是一次大提交。

Swift 删除清单：

- `swift/`；
- `gui/`；
- Swift Package/lock/build artifact；
- CI 中 Swift build/parity job；
- Swift 专属 schema 名称，必要时先迁移到 provider-neutral schema；
- `adapters/`、README、AGENTS 和活跃 docs 中的 Swift 执行说明；
- 忽略规则、release 脚本和只服务 Swift 的 fixture helper。

Python 收缩规则：

- 已由 Go 达到 parity 的公开脚本停止被 skill/adapter 调用，再标记 deprecated，最后删除；
- 差分 oracle 至少跨一个 release candidate 保留；
- 字体 provider 和必要仓库维护脚本不因“完成 Go 化”自动删除；
- `archive/` 历史不为了清零关键字而改写。

最终检查：

```sh
rg -n "swift run|swift/|gui/|Python/Swift|Swift" \
  AGENTS.md README.md docs adapters contracts skills .github scripts
```

命中只允许存在于明确标记的历史、迁移记录或第三方名称中。删除完成后运行根 `AGENTS.md` 规定的完整相关
验证，而不只是 `git diff --check`。

## 20. 任意 AI 的执行协议

任何 AI 接手迁移任务时必须按下面顺序工作：

1. 阅读根 `AGENTS.md`、本文件、目标 capability manifest、相关 skill 和对应规范/demo。
2. 在 §18 和迁移状态文件确认当前阶段；不得跳过未满足的前置 phase。
3. 一次只声明一个 capability 或一个共享基础设施目标。
4. 先列出现有 Python/Swift oracle、fixture、命令、redline 和待保持事实。
5. 先补 fixture/golden，再写 Go 实现。
6. 对 transformer 先实现 plan/dry-run，再实现 apply。
7. 运行 unit、fixture、differential、redline 和平台相关测试。
8. 将差异分类为 bug、允许的序列化差异或需要产品决定；不得自行降低 gate。
9. 更新迁移状态、证据、文档和 catalog；没有证据不能把状态改为 complete。
10. 只有达到本阶段退出门槛后才能删除对应旧入口。

每项状态只使用：

```text
not-started -> fixture-ready -> go-readonly -> go-write -> parity -> released -> legacy-removed
```

状态禁止倒推：有 Go 文件不等于 `go-readonly`，单机测试不等于 `parity`，主分支存在不等于 `released`。

### 20.1 单 capability 完成定义

- manifest 和 schema 已加载且 catalog 可发现；
- JSON request/result golden 已存在；
- 成功、普通失败、安全失败、取消至少各有测试；
- 对应 Python/Swift oracle 差分完成；
- transformer 有 dry-run、stale-plan、atomic output、幂等和 redline 测试；
- peak memory/temp space 有基准；
- Tier 1 CI 通过；
- skill/adapter 只在 release 后切换；
- 迁移状态记录证据路径和替代入口。

### 20.2 AI 禁止事项

- 不得把旧脚本机械翻译成一个巨大 Go 文件。
- 不得删除 validator、fixture 或正文不变检查来获得 parity。
- 不得用正则解析复杂 XML/CSS。
- 不得在错误时静默调用旧 Python，除非 request 明确选择 legacy provider 且结果记录了 provider。
- 不得让不同平台有未记录的功能差异。
- 不得把 runtime 下载依赖作为正常安装流程。
- 不得因为用户不关心二进制大小而忽略内存、临时空间、安全和启动路径。
- 不得在 Phase 8 之前批量删除 Swift/Python 基线。

## 21. 需要保留的决策记录

实现中如需改变下列任一项，必须先在本节追加 ADR 摘要并更新根 `AGENTS.md`：

- Go 不再是公开核心；
- 引入 CGO；
- 引入 Rust/Node/Java 等新 provider；
- 允许原地写 EPUB；
- 修改 exit code 或 request/result major schema；
- 内置模型 API 或网络调用；
- 恢复 GUI/移动端；
- 改变字体 provider 边界；
- 改变 CSS parser 或放弃 lossless patch；
- 改变 Swift/Python 删除门槛。

## 22. 本设计变更明确不做的事情

本文件及其入口同步只确认未来架构：

- 不创建 `go.mod`；
- 不实现 Go 代码；
- 不改 capability schema；
- 不切换任何 skill；
- 不删除 `swift/`、`gui/` 或 Python 脚本；
- 不修改现有 CI 的执行行为。

这些变更必须从 Phase 0/1 开始，按门槛独立实施。
