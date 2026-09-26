# Go 重写交接

> 当前实现规则以 [`docs/final/SPEC-go-architecture.md`](../final/SPEC-go-architecture.md) 为准。本页只记录接手所需的现状、开放项和检查入口。
> 迁移期决策快照见 [`archive/meta/2026-08-30-go-rewrite-decisions.md`](../../archive/meta/2026-08-30-go-rewrite-decisions.md)；逐轮复审证据见 [`archive/meta/2026-09-go-rewrite-review-log.md`](../../archive/meta/2026-09-go-rewrite-review-log.md)。

## 当前状态（2026-09-26）

- Go 单一公开 CLI 与 `internal/` 能力流水线是唯一执行面；`contracts/` 是机器契约来源，`tools-font/` 是独立字体 provider。架构硬约束与守卫要求以 Go 架构 SPEC 为准。
- contracts 与 registry 各有 22 个 capability。当前执行形态为 13 个输出型与 9 个只读型；注册数不代表所有能力都不依赖外部工具，运行状态以 `epub capabilities --json` 为准。
- `--legacy-report` 已移除。CLI 使用 v2 envelope；取消以 `status=cancelled`、exit 1 表示，取消的写出型任务不落盘。
- EPUB 结构与正文验证由 `epub.package.nav.audit`、`epub redline --check all` 和 CI EPUBCheck 组成。不存在独立 `epub_lint.py` 的 Go capability。
- 最近发布基线为 Go CLI `v0.3.0`；后续版本以 GitHub Release 与 CHANGELOG 的实际附件和校验和为准。
- `internal/docguard` 已接替技能 frontmatter、OpenAI YAML 形状、skill 索引、AI 入口和契约结构等元校验。手册、速查表与 SPEC 的语义同步仍需人工核对。
- CI、静态检查和产物验证不代表原生阅读器验收；每项 reader 结论只对 reader-matrix 记录的精确 artifact 与 SHA 生效。

### 执行面基线

| 范围 | 当前实现与边界 |
|---|---|
| CLI | `cmd/epub` 只处理参数和退出码；业务编排在 `internal/pipeline`。 |
| 契约 | `contracts/capabilities/` 定义 capability、权限、requires 与执行形态；v2 envelope 由 schema 和 INV-6 守卫。 |
| EPUB I/O | `internal/book` / `internal/zipfs` 管理有界读取、ZIP entry 透传与一次性写出。 |
| 扫描与编辑 | `internal/scan/{opf,xhtml,css}` 产出字节范围 edits；结构 normalize、EPUB3 OPF 与 XHTML shell/link 按范围写入。EPUB3 注释结构转换仍按能力专属路径处理。 |
| 字体工具 | `tools-font/` 私有于仓库 provider，由 `internal/extern` 调用；不进入 CLI 发行包。 |
| 遗留执行面 | Python 执行脚本与 parity harness 已移除；`tools/parity/legacy-refs.txt` 作为零条目守卫基线保留。 |
| 写出 gate | 单能力按其 gate 写出；`epub clean --approve` 仅在步骤、末次审计和全项红线通过后写出。失败候选默认不保留，显式 `--retain-review-candidate` 时只写 `.review-only.epub`。 |
| 取消 | 取消用 `status=cancelled` 和 exit 1 表示；取消的事务不写出，不能将其伪装成一般书稿错误。 |

### CLI 状态与退出码

| Exit | 含义 | 操作 |
|---:|---|---|
| 0 | complete，或无 error finding 的 planned | `planned` 的 clean 默认只完成审计；选了步骤时表示已检查内存候选但未写出。审阅 findings/facts 与范围后再决定是否批准。 |
| 1 | failed、error finding 或 cancelled | 查看 findings/events；cancelled 不得保留半成品。 |
| 2 | approval-required | 按显式批准要求审阅候选与变更范围后再执行写出。成功 dry-run 使用 `planned` / exit 0。 |
| 3 | 用法错误或输入不存在 | 修正参数、路径或输入类型后重跑。 |

成功的写出 envelope 会记录 output path 与 SHA。普通单能力的失败候选规则依能力而定；`epub clean` 失败时默认无 EPUB output，只有显式 `--retain-review-candidate` 才留下 `.review-only.epub`。每次检查都要确认 output 与 `pipeline.artifactDisposition`，不从 exit code 单独推断。

### 回归优先级

- 对契约、pipeline、编辑器或 scanner 的改动优先跑 `go test ./cmd/... ./internal/...` 与固定 CLI 回归矩阵。
- 对 I/O 限额、取消或 ZIP 写出改动加 race 与失败事务验证，确认超限/取消时没有输出文件。
- 对 reader matrix、demo XHTML/CSS、阅读器文档的改动逐 SHA 重建 demo，并把 maintain、popup、nav audit 与 redline 结果绑定到新 artifact。
- 字体 provider 更新需分别报告 Python 单测、Go external wrapper 行为和字体许可；其中任何一层通过都不能代替另外两层。

### 稳定的执行语义

- 上游 requires stage 是诊断输入；其 failed 状态不单独阻断目标能力。DRM 预检、runner Go error、未实现能力与目标能力失败仍会阻断。
- 输出能力在内存态完成预期变更与契约红线检查。红线 error 会标记 failed；只要 runner 和输出事务成功，仍可能给出候选产物供 diff review。
- `--dry-run` 阻止磁盘输出，但必须完整运行内存变更和相应检查；不能把 dry-run 当作跳过能力执行。
- `epub clean` 默认只审计并生成计划。选择变换用 `--steps`；typography 还必须明确 `--preset` 与 `--scope`。`--approve` 只在步骤、末次审计和全项红线通过时写最终候选。
- 多产物能力必须使用 `output_dir` 契约；只读能力不得接受或建议 `--output`。
- `epub redline --path-map` 接受 normalize、merge、cover 等 envelope 的 `facts.*.mappings`；无改名时成功 envelope 可提供空数组。
- shell 建议命令必须正确引用路径；JSON envelope 使用共享 legacy-compatible marshal 约定，保留稳定输出形状。
- Go 模块声明 `go 1.27`；第三方依赖限于 CSS parser、平台 I/O helper 与 Unicode/text 支持。新增依赖按 SPEC §9.3 先核预算和边界。
- ZIP、单 entry、总解压、book 缓存、封面输入与辅助 JSON 均有限额；读取和写出过程传递 context，超限与取消不返回部分成功。
- CSS 变更只用 `internal/scan/css` 的 lossless parser 和 edit spans；复杂语法不得交给整份文档正则或 serializer。
- 外部 provider 缺失时由 capability 明确跳过或失败；provider 超时、stdout/stderr 上限与取消都应归因到工具运行状态。
- capability 参数格式与执行类型都从 contracts 加载；避免在 CLI、pipeline、skill 文档里另建互相漂移的 id 白名单。
- v2 envelope 的 facts 字段按 capability 命名空间写入；JSON 数组即使为空也保持 `[]`，稳定排序用于输出与 golden。

## 待决策 / 开放项

- Apple Books、Readest、Kindle Previewer 等目标阅读器仍需按 `docs/final/reader-matrix.yaml` 的待测项执行 GUI 实测；不得把构建、EPUBCheck 或浏览器结果记作 reader pass。
- `tools-font/epub-font` 是独立 CLI，不是正式 capability。若要升格为 `epub.font.subset`，需先确定新契约与 SPEC §6.1 设计，并通过 golden 测试和全项 redline。
- Source intake 当前只做可审计盘点；PDF 解析、OCR、图片转码与后续内容抽取不在现有契约范围。扩大范围前需明确输入材料、隐私、许可和输出决策。
- 手册与速查表之间的规则一致性目前没有自动语义守卫；涉及硬规则时按 `AGENTS.md` 同步检查 SPEC、终极实践手册、CSS 速查表和相关 skills。
- 任一 reader 状态需要有真实版本、精确 artifact SHA 和可复核截图或日志；若证据缺一，状态继续留在 warn / na 或 untested，不由工具验证代填。
- EPUBCheck 只在 GitHub Actions 执行。若本地缺少该工具，不得把 nav audit、XML parse 或 ZIP 完整性写成 EPUBCheck 通过。
- 发行包的跨平台构建与 ABI 支持边界以 release workflow、附件和构建日志为准；单机编译成功不能代替目标平台运行证据。
- 字体锁定模式保护既有 `fonts.css` 字节和 OPF `ibooks:specified-fonts` 元数据；不自动推断字体链、补全字符覆盖或声称视觉兼容。
- 若修改 CSS、XHTML、OPF、manifest、font mode 或 ZIP 路径，覆盖 normal / entity / comment / CDATA / malformed / cancellation 等对应 fixture，确认安全拒绝不会留部分 edits。

### 范围与升级规则

- 新增 capability 必须先定公开参数和结果契约，再按 SPEC §6.1 落实现、Go-native 测试、pipeline 注册和 golden；验收 = golden 测试 + 全项 redline。
- 不因历史迁移文档残留引用而恢复旧执行面；`archive/` 中的 Python 行为和旧架构仅为背景证据。
- 字体子集 demo 的现有脚本仍属于 `tools-font/` 独立工具。将其中任一操作纳入 Go 流水线前，必须重新审查隐私、字体许可、可复现性与失败事务语义。

## 决策索引

| 主题 | 当前依据 |
|---|---|
| 架构、依赖方向、lossless edits、capability 模板 | [`SPEC-go-architecture.md`](../final/SPEC-go-architecture.md) |
| Go 版本、语言与 API 风格 | [`SPEC-go-modern-guidelines.md`](../final/SPEC-go-modern-guidelines.md) |
| Agent 执行、验证、EPUB 与 reader 证据边界 | [`AGENTS.md`](../../AGENTS.md) |
| 现有书清洗顺序与红线 | [`cleanup-flow.md`](cleanup-flow.md) |
| 阅读器证据与待复测队列 | [`reader-matrix.yaml`](../final/reader-matrix.yaml) |
| W0–W5 迁移决策快照 | [`2026-08-30-go-rewrite-decisions.md`](../../archive/meta/2026-08-30-go-rewrite-decisions.md) |
| 复审日志与历史实现依据 | [`2026-09-go-rewrite-review-log.md`](../../archive/meta/2026-09-go-rewrite-review-log.md) |

## 接手检查清单

1. 先读根目录 `AGENTS.md`，再按任务读取本页链接的架构 SPEC 与 Go 编程指南；历史归档只用于理解背景，不覆盖当前规则。
2. 检查 `go.mod` 与模块版本，按 Go 编程指南 §2 读取当前工具链适用的版本化语言/API 规则。
3. 确认当前分支、工作区与暂存区状态；审查拟提交的完整 staged 和 unstaged diff，不要把用户文件、字体母版或生成物一起暂存。
4. Go 改动至少运行 `go build ./cmd/... ./internal/...` 与 `go test ./cmd/... ./internal/...`；涉及解析、并发或事务时增加 vet、race 或对应定向测试。
5. 架构、capability、SKILL.md 或执行面改动另跑 `go test ./internal/archguard/ -v` 与 `go test ./internal/docguard/`。
6. 对 Go 行为改动运行固定 CLI 回归，比较信封、退出码、stderr 与 entry 字节差异；每个差异都要有已授权任务作为依据。
7. `internal/archguard/` 禁止改动。怀疑守卫有误时停下交由人审，不改断言、不加豁免、不跳过测试。
8. demo、validator、`docs/final/` 变更按根 `AGENTS.md` 构建模板，并对精确产物运行 maintain、popup、nav audit、redline 与 XML 检查。
9. EPUBCheck 是 CI gate；只引用 CI 对应的提交/产物结果，不把本地静态工具的结果冒充 EPUBCheck。
10. 记录测试所覆盖的 commit、EPUB artifact SHA、工具/reader 版本与未验证边界；不以未实测状态推断兼容性。
11. 提交前运行 `git diff --cached --check` 和 `git diff --check`，分逻辑提交并检查推送后的远端 main SHA。
