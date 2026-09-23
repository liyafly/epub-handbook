# Go 重写历史复审日志

> 本文件归档原交接文档 §6–§11.10。当前状态与开放项请看 [`docs/pipeline/go-rewrite-handoff.md`](../../docs/pipeline/go-rewrite-handoff.md)；迁移期的决策快照见 [`2026-08-30-go-rewrite-decisions.md`](2026-08-30-go-rewrite-decisions.md)。以下内容保留复审过程与证据，作为历史记录。

## 6. 遗留与待决策（接手者从这里开始）

1. **`epub.source.intake` —— 已解决（2026-09-07，仓库所有者决策：最小盘点 planner，
   不含 PDF 解析 / OCR / 图片转码）**。`internal/caps/sourceintake/` 对目录或单个文件做
   只读遍历：按扩展名分角色、流式 SHA-256、UTF-8/BOM/CRLF、非核心图片格式与 CMYK JPEG
   头部探测，PDF/嵌套压缩包/已有 EPUB 只作标记；facts 给出 `files[]`、`roleCounts`、
   `workspacePlan` 与合 execution-plan schema 的 `plan`/`blockers`。pipeline 新增
   `registerSourceInput`（`--input` 可为目录或任意文件，永不 `book.Open`，绝对路径以
   `source_path` 传入，`epub.style.demo.maintain` 的 noBook 语义不变）。抽取与结构化仍是
   人工 + AI 流程（`skills/epub-source-intake/SKILL.md` 已按实跑行为改写）。
2. **`--legacy-report` 脚手架 —— 已拆除（2026-09-04）**。SPEC §5.2 的移除触发条件（对应
   Python 脚本删除）满足后，该脚手架曾深度织入 caps 包与其测试（2026-08-30 实测：117 处
   引用、37 个文件，其中 48 处在测试断言里）。2026-09-04 完成拆除：各 cap 的
   `LegacyReport` 参数、`legacyReport` 结构体、`facts.legacyReport`、pipeline 的
   `legacy_report` 透传与 CLI flag 全部删除；曾只在 legacy 报告里出现的数据提升为正式
   `facts` 键，测试改为断言正式 facts / findings；`epub redline --path-map` 直接接受
   `--json` 信封（读取 `*.mappings` facts），SKILL.md / cleanup-flow / AGENTS.md 的
   path-map 流程同步改写。
3. **Python 元校验器由 Go 守卫接替**：`internal/docguard`
   （`TestSkillFrontmatter` / `TestOpenAIYAMLShape` / `TestSkillIndexTables` /
   `TestAIEntrypointsCanonical` / `TestFootnoteClassVocabulary`）与 archguard
   `TestContractsValid` 覆盖技能 frontmatter、OpenAI YAML 形状、索引表、入口能力与契约。
   手册和速查表同步仍靠人工复核；不得以文档守卫尚未覆盖为由放宽 `archguard`。
4. **`epub_lint.py` 无对应能力**（SPEC §7.2 映射表列了 `internal/caps/lint`，
   但 22 个契约里没有 lint id）。现行裁决：产物检查 =
   `epub run epub.package.nav.audit` + `epub redline --check all` + CI EPUBCheck，
   已写入文档。若认为仍缺独立 lint 能力，需先补契约再按 §6.1 实现。
5. **CLI 发布**：Go CLI 已发布 `v0.3.0`。新增发布流程或平台产物时，先对照当前 GitHub Release、工作流与版本注入实现，不能沿用本节旧快照中的“尚无 release”判断。
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

---

## 9. 链语义修复（2026-09-04）

**ff30a1a 破坏了什么**：该提交让 `epub run <id>` 执行整条 requires 链（本身正确），
但把上游 stage 的 `Status == failed` 或任一 error finding 当作阻断（`stageFailed` +
`break`），并把「红线必须在落盘前通过、失败不创建输出文件」写进了 pipeline。
几乎所有能力 `requires` `epub.package.nav.audit`，而 nav.audit 在任何真书上都会
报 error（如 `Inline SVG XHTML item missing properties="svg"`），于是：

- 真书（49MB《EPub指南》）：`epub run epub.structure.normalize --dry-run` 事件停在
  `epub.package.nav.audit:failed`，目标 stage 从未运行，`status: failed` / 退出码 1 /
  `output: null`；`migrate.epub3`、`font.coverage.analyze` 等同样全部不可用。
  AGENTS.md 记录的 audit → normalize → migrate → redline → 人工 diff review 流程在任何
  不完美的书上都跑不通。
- `epub run epub.metadata.edit --input demo.epub --output out.epub metadata_json='{"title":"x"}'`
  （templates/epub-style-demo/dist）：事件 `metadata-write:completed`、
  `redline:failed(1 findings)`，`out.epub` 不存在——metadata.edit / merge /
  cover.replace / migrate.epub3 在自身契约里声明了 `metadata` / `spine` / `cover` 红线
  却有意改动这些内容，因此永远无法产出文件，与 §0 决策 2 和
  `docs/pipeline/cleanup-flow.md`「产物仍会写出供 review」相悖。

**改了什么**（只在 `internal/pipeline/run.go`）：

- 链的最后一个元素是目标，其余是上游。上游按 §0 决策 1 记为非阻断诊断
  （`facts["<id>.status"]`、`facts["<id>.findings"]`、`info upstream.diagnostics`、
  `<id>:completed` 事件）；runner Go error 与未实现能力仍然阻断。
- 红线结果与落盘解耦：红线 error / 校验器错误仍 failed / 退出码 1，但唯一一次
  `WriteToContext` 只被 DRM / runner 错误 / 未实现 / 目标 stage 失败 / dry-run 阻断；
  `write-output` 事件 message 注明 `retained for human diff review despite redline findings`。

**修复后的真书实跑**：`epub run epub.structure.normalize`（apply）事件为
`nav.audit:completed(diagnostic: 2 error, 9 warn)` → `epub.structure.normalize:completed`
→ `redline:failed(12 findings)` → `write-output:completed`，`output.sha256` 有值。
这 12 条 `redline.text` 是**真实信号**而非链问题：normalize 的 format 阶段
（Python 语义：dry-run 也在内存应用）改写了 Chapter11-2 / Chapter12-2 / Chapter8-6
正文代码样例里的路径字符串（如 `<text src="../Text/Chapter2-2.xhtml#\2"/>`），
应由人工 diff review 裁决，属 normalize 的独立待办，不在本次修复范围。

**测试**：`internal/pipeline/contract_test.go` 的
`TestRunUpstreamStatusFailedIsDiagnosticNotBlocking`、
`TestRunUpstreamErrorFindingIsDiagnosticNotBlocking`、`TestRunUpstreamRunnerErrorStillBlocks`、
`TestRunRedlineFailureWritesOutputButFails`、`TestRunRedlineValidatorErrorWritesOutputButFails`
取代了编码旧语义的四个测试；`internal/pipeline/chain_semantics_test.go` 新增真书回归
`TestRunRealBookNormalizeDryRunNotBlockedByNavAudit` 与 `TestRunMetadataRedlineWritesOutputButFails`。
DRM preflight 测试与 `TestRunPendingCapabilityFails` 语义不变。

---

## 10. `--legacy-report` 拆除后的复审修复（2026-09-07）

对 2026-09-04 拆除脚手架那一轮做对抗性复审，修掉四类缺陷：

1. **golden 不再依赖开发机 PATH**（HIGH）：`testdata/navaudit/native-golden.json` 里的
   `toolAvailability.epubcheck` 与 `nextCommands` 末行取决于本机有没有 `epubcheck`，
   `brew install epubcheck` 就会让 `TestNativeFixtureGolden` 无故变红（CI 侥幸通过：
   workflow 把 EPUBCheck 当 jar 取，且在 `go test` 之后）。改法是给 `navaudit` 加
   `toolProbe`（默认 `externToolProbe` → `extern.LookPath`，仍不碰 `os/exec`，INV-4 保持），
   导出的 `Run` 走默认实现，测试用不可注入的内核 `run(...)` 固定探测结果。
   新增 `TestToolAvailabilityFollowsInjectedProbe`（true/false 两支）与
   `TestRunDefaultsToExternProbe`（断言默认实现就是 extern）。
2. **`--path-map` 认不出映射时报错而不是静默空映射**（MEDIUM）：`redline.LoadPathMap`
   曾对 `{"schemaVersion":"2","facts":null}`、无 `facts`、`*.mappings` 不是数组、
   顶层数组、`{}` 等一律返回空 map + nil error。后果是把 FAILED normalize 的信封喂进
   `epub redline --path-map` 会得到满屏"文件缺失 / 新增文件"而不是清晰的输入错误。
   现在这些形状一律返回 `ErrInput`（CLI 退出码 3）；**空数组仍然合法**——未改名的成功
   normalize 会输出 `"mappings": []`（`nonNilMappings`）。回归见
   `internal/redline/pathmap_test.go` 的 `TestLoadPathMapRejectsEnvelopesWithoutMappings`
   （12 种形状）与 `TestLoadPathMapAcceptsEmptyMappingList`。
3. **facts 数组不再序列化成 null**（MEDIUM）：`navaudit` 的 `actionableFindings` 用
   `var out []detectorFinding` 累积，空时输出 `null`，与 SKILL.md 声明的数组形状矛盾
   （`| length` 会炸）。改为空切片字面量，`nextCommands` 同样改成 `make(..., 0, n)`；
   `testdata/envelope/pending-capability.report.json` 已按实跑重新捕获（`[]`）。
   `internal/caps/structure_normalize` 的 `stages[]` 经核查不存在同样问题：`stageReport`
   的唯一构造点（`structure_normalize.go:190`）已把 `Mappings` / `Warnings` 初始化为空
   切片，实跑输出即 `[]`；只补了锁定该形状的 `TestStageSlicesSerializeAsArrays`。
4. **文档回到与代码一致**（MEDIUM/LOW）：`refinement-harnesses.md` 曾称
   `facts.toolAvailability` 探测 `magick` / `oxipng` / `pngquant` / `jpegoptim` / `svgo`，
   实际只探测 `epubcheck`；`epub.font.coverage.analyze` 的 `detectorExitCode` /
   `detectorStderr` 曾被写成"detector 异常时出现"，实际只在**成功路径**设置——adapter
   失败会返回 Go error，`pipeline/run.go` 连同 facts 一起丢弃，失败信息只在
   `findings[].fontcoverage.adapter`。§5.1 关于"脚手架 CLI 入口仍在"的说法与 §6 遗留项 2
   自相矛盾，已按 §6 更正。

### 10.1 同一轮的其余修复

5. **`epub.source.intake` 的健壮性边界**：`--input` 指向符号链接目录时先解析根再遍历
   （树内符号链接仍只记录不跟随）；非常规文件（FIFO / 设备文件）在 pipeline 计算信封
   SHA-256 之前就以退出码 3 拒绝，此前会无限读取挂死；不可读子目录记
   `warn intake.unreadable-dir` 并跳过该子树，已盘点结果照常给出（此前整份盘点连同
   facts 一起丢弃），不可读文件带 `unreadable` risk 且不参与 blocker 推断；单文件输入
   不再把文件名拼接两次；`max_files` 非正整数经新增的 `pipeline.UsageError` 通道以退出码
   3 拒绝，不再静默回退默认值；命令里的路径按需 shell 引号化。
6. **`epub.structure.normalize` 区域扫描的收尾**：`<?xml-stylesheet …?>` 的 `href`
   重新参与改写（区域化初版整段跳过全部 PI，这条合法样式表引用改名后会静默断链）；
   扫描器遇到 DOCTYPE 内部子集、HTML 空注释 `<!-->` 与标签内不配对引号时不再静默放弃
   文档剩余部分，前两者正确跳过，最后一种仍停止但给出带文件名与字节偏移的
   `structure.warning`；`url()` / `@import` 只在 `style=""` 属性值里改写，`title` / `alt`
   等读者可见文本不再被当成 CSS。样本书输出字节不变。
7. **nextCommands 组装规则**（SPEC §8.2）：pipeline 此前无条件用自己的静态文案覆盖
   `Result.NextCommands`，navaudit 依 findings 算出的整份命令表从未到达调用方。现在能力
   自身的建议优先（`status: failed` 时同样给出——那正是"该跑什么来修"），去重并剔除
   「再跑一遍本能力」的自引用条目，能力没给建议时才退回静态文案。只读能力的
   `--dry-run` 不再建议不存在的 `--output`。`testdata/envelope/nav-audit-findings.report.json`
   已按实跑重新捕获（1 → 7 条命令，并补齐拆除脚手架后新增的正式 facts 键）。
8. **已解决（见 §11.2）**：`epub.package.migrate.epub3` 的同类转义正文误改
   （改写字符数据里的 `class="duokan-footnote"`）已修；区域扫描器已按本条的判断下沉到
   `internal/scan/xhtml`（层 4），两个 capability 共用一份。

---

## 11. 重构逻辑复审（2026-09-07 第二轮）

对整个 Go 重构面做了一轮以「守卫绿 ≠ 正确」为前提的复审：`go build` / `go vet` /
`go test ./...` 与三个守卫包在复审开始时全部是绿的，因此重点放在**测试为什么是绿的**、
以及**守卫覆盖不到的地方**。以下四条是修掉的，§11.5 是留给所有者裁决的。

### 11.1 `text` 红线漏掉行内元素里的正文（CRITICAL，迁移回归）

`redline.ExtractTextBlocks` 的流式实现只把字符数据写进最内层打开元素的缓冲，
子元素闭合时**没有并回父帧**。后果按形状实测：

| 输入 | 修复前的块 | oracle / 修复后 |
|---|---|---|
| `<p>第一段落。</p>` | `第一段落。` | 同 |
| `<p><em>强调整段。</em></p>` | **（无块）** | `强调整段。` |
| `<p>前<em>中</em>后</p>` | `前后` | `前中后` |
| `<p><a href="x">链接文字</a></p>` | **（无块）** | `链接文字` |
| `<li><a href="x">第一章</a></li>` | **（无块）** | `第一章` |
| `<p>汉字<ruby>字<rt>zì</rt></ruby>注音。</p>` | `汉字注音。` | `汉字字注音。` |

即：任何被 `<em>` / `<a>` / `<span>` / `<strong>` 包裹的正文都不进块哈希；整段被行内
元素包裹时该段不产出块，**可以被整段删除而红线零 findings**。

这是**迁移回归**，不是复刻 Python 的怪癖。git 历史里的 oracle
（`scripts/validate_text_invariance.py` 的 `element_text_without_ignored`）是
`node.text` → 递归子元素 → `node.tail` 的完整递归收集，只在 `IGNORED_TEXT_TAGS`
与 note-control 锚点剪枝并保留其 tail。

之所以一直没被发现：`CheckText` 唯一的命中用例
（`internal/redline/inprocess_test.go`）用的 fixture 三个 `<p>` 全是直接字符数据，
恰好落在没坏的那条路径上。

修法是子帧闭合时把**未归一化**的缓冲并回父帧（`normalizeText` 仍只在块产出时对拼好的
整串做一次）。样本书 Chapter12-2 的块数 558 → 578；`structure.normalize` /
`typography.optimize` / `css.layering.optimize` 实跑仍是 0 findings —— 只扩覆盖面，
未引入误报。回归见 `internal/redline/textblocks_test.go`。

### 11.2 `migrate.epub3` 改写作者正文 + 区域扫描器下沉

§10 第 8 条记的缺陷已修。`class="duokan-…"` 三条替换针不含尖括号，对全文生效，
于是「讲解多看注释写法」的正文被当成标记改掉（尖括号转义了，属性部分是裸字节）。
样本书实跑 8 条 `error redline.text`，产物虽保留供 review，但这条能力在该书上不可用。

同一条记录里已经判断出正确修法：区域扫描器是通用的，应按 SPEC §1 下沉到
`internal/scan/xhtml`。照此执行——`markup_regions.go` → `internal/scan/xhtml/regions.go`，
导出 `ScanRegions` / `Region` / `RegionTag` / `RegionStyle` / `RegionStylesheetPI` /
`ScanComplete`（只返回区间，不返回文档字节，INV-2 不受影响）。顺带解决了一个反直觉的
现状：**`internal/scan/xhtml` 在此之前零消费者**——SPEC 指定的 XHTML 扫描层是空的，
而 5 个 caps 各自手写了自己的扫描器（见 §11.5）。

保持全文替换的两条不受影响：`duokanAside` 需要字面 `<aside`，`>⊙</a>` 需要字面 `>` 与
`</a>`，转义正文都不可能命中。实跑：`failed` / 退出码 1 / 8 findings →
`complete` / 退出码 0 / 0 findings，`duokanNotesNormalized` 15 → 7。

### 11.3 红线校验器的命中路径此前没有任何可执行断言

`internal/redline/parity_test.go` 的 9 个 `TestParity*` 在 oracle 随 `scripts/` 删除后
**无条件 `t.Skip`**（`os.Stat(script)` 检查在任何分支之前）。同样形状的还有
`caps/{cover,merge,metadata,split}` 的 7 个用例，合计 16 个测试在任何机器、任何 CI 上
永久跳过并报绿。

后果：`anchors` / `metadata` / `spine` / `cover` / `drm` 五条红线只有「干净的书零
findings」这条放行路径被覆盖（`inprocess_test.go` 结尾的 `clean` 断言），**命中路径
一条断言都没有**。INV-5 只保证校验器被注册，不保证它还会开火——一个永不开火的校验器
可以通过全部测试。`AllowList` 与 `--path-map` 的比对语义同样只在死用例里。

已把这 9 个用例改写为 Go-native 断言（fixture 本来就是完整的，缺的只是 oracle）：
期望值按 `checks.go` 各校验器的语义手写，不回填实现输出；`--path-map` 与 `--allow-list`
另带**反向对照**（不给选项时必须开火），证明是选项而不是巧合让它通过；补了 spine 改序、
cover 缺失、字体混淆需显式授权（`AllowFontObfuscation` 两支）、非法 `--check`、
输入缺失等此前无覆盖的分支。

`caps/{cover,merge,metadata,split}` 的 7 个 skip 尚未处理，见 §11.5。

### 11.4 契约面与守卫牙齿

- **用法错误吞掉信封**：`--json` 在退出码 3 上输出零字节，与 SPEC §8.2「所有命令返回
  同一形状」相悖。现在给出合 v2 schema 的最小信封（`status: failed` + 单条
  `error usage.invalid-argument`），退出码仍是 3。
- **同类输入错误退出码不一致**：`max_files` 走 `UsageError`（3），而 `split_points` /
  `expect_volumes` 用裸 `fmt.Errorf`（1）。已统一。
- **改名映射出不了信封**：`merge` / `cover.replace` 的改名只走内部 `Result.Renames`，
  信封里只有计数，`epub redline --path-map` 在合并/换封面之后无映射可用；而
  `skills/epub-package-operator/SKILL.md` 让 agent 去读信封里并不存在的 `renames` 字段。
  两者现在都给出与 normalize 同形状的 `facts.mappings`，SKILL.md 改为实际键名。
- **`fontcoverage` 的错误归因**：`extern.Run` 的 error 被丢弃，进程起不来时用零值
  `CmdResult` 报出 `exit code 0`。已改为显式失败。
- **数组形状的 facts 序列化成 `null`**：`merge.warnings` / `merge.inputs`
  （`append([]string(nil), …)` 对空切片返回 nil）、`sourceintake.files`（空目录是
  `intake.empty` 这条一等错误路径）、pipeline 的 `modified_entries`。
- **守卫牙齿**：`archguard.yml` 的「守卫是否被改动」检测在默认浅克隆下
  `git diff <base> HEAD` 必然失败、警告**永不触发**（SPEC §5.1.1 三件牙齿之一形同虚设）；
  已加 `fetch-depth: 0` 并在 base commit 取不到时显式报错。检测范围与 `CODEOWNERS`
  一并覆盖 SPEC §5.1 同样声明为「需人类审阅」的 `internal/legacy_surface/`、
  `internal/docguard/` 与 INV-10 棘轮基线（此前两者都只覆盖 `internal/archguard/`）。

### 11.5 所有者裁决与执行结果（2026-09-07 第二轮收尾）

§11.5 原本是六条待裁决项。仓库所有者逐条裁定后已全部执行，结果记在 §11.6；
仅 §11.6 末尾两条仍是开放项。原始待裁决清单保留在下面，便于回看当时的判断依据。

#### 原始待裁决清单

1. **同一缺陷类还在 4 个写入型能力里**：`cover` / `merge` 的
   `transformResource`→`subNameQuoteURI`/`subCSSURL`、`typography` 的
   `typoLinkRe`/`typoHeadEndRe`、`alite` 的 `headEndRe`/`addClassToTag` 仍对全文做
   裸匹配（`href=` / `src=` / `url(` / `</head>` 都不要求前置 `<`）。区域扫描器现在是
   共享的，逐个改造是机械工作，但每个都要各自复核 parity 与计数语义，不宜与本轮混做。
   风险排序：`cover`/`merge`（对每个 manifest 项无条件跑）> `typography`（`</head>`
   命中注释里的示例）> `alite`（只作用于自动识别出的封面/版权页）。
2. **caps 之间的重复实现约 2500–3000 行**，其中约 1100 行是逐字节副本：
   `merge/pytool.go` ↔ `split/pytool.go`（389 行，仅 package 名不同）、
   `merge/navtoc.go` ↔ `split/navtoc.go`（360 行，同）、
   `typography/opfedit.go` ↔ `css_cleanup/opfedit.go`（112 行，同）、
   `typography/pyutil.go` ↔ `css_cleanup/pyutil.go`（~270 行相同）、
   `{cover,merge,split}/refs.go`（~250 行相同）、`{cover,merge,split,metadata}/pkgio.go`
   的 `readPackage`、`structure_normalize/xmlmini.go` ↔ `migrate_epub3/xmlmini.go`
   （~600 行，文件头自述「整份私有拷贝」）。
   成因是 §1 的「同层禁止互相 import」+ 缺一个共享原语落点，而不是有人偷懒。
   出路与 §11.2 相同：`internal/scan/` 下按前缀已属层 4，新增子包**不需要改 archguard**
   的 layer 表（前缀匹配已覆盖），因此可以合法地建一个共享包。注意 `scan/*` 之间同层
   不能互相 import，共享包要设计成叶子。
3. **`caps/{cover,merge,metadata,split}` 的 7 个 oracle skip**：与 §11.3 同因，
   fixture 完整、只缺 oracle。SVG 封面缩放的 viewBox 数学、merge 冲突改名重写、
   metadata 字段写入、split 段独立性目前只有浅层 facts 断言。
4. **`ctx` 在 16 个 capability 里有 14 个完全未使用**，`internal/extern.Run` 没有 ctx
   参数且用 `exec.Command`；`report.StatusCancelled` 与 v2 schema 的 `cancelled` 枚举
   从未被赋值。今天 `cmd/epub` 只传 `context.Background()`，所以是潜在缺口而非现网问题
   —— 但任何 `--timeout` / Ctrl-C 处理都会立刻踩到。
5. **契约不描述执行形态**：`registerReadOnly` / `registerMultiOutput` / `registerNoBook` /
   `registerSourceInput` 四类是 Go 侧的 id 白名单，契约里没有对应字段，也没有守卫对账；
   其中两条与契约直接矛盾——`epub.notes.popup.normalize` 契约写 `kind: transformer` +
   `requiresWriteAccess: true` 却注册为只读，`epub.style.demo.maintain` 契约写
   `planner` + `requiresWriteAccess: true` 却永不写盘。若认为 `contracts/` 是机器契约的
   唯一事实来源，这里需要补一个字段（会动 `capability-manifest.schema.json` 的
   `required` 与 `additionalProperties: false`，属 CODEOWNERS 范围）。
6. **59 个生产函数不可达**（`deadcode -test`），最大一簇是 `css_cleanup` 的 ~30 个
   （`consolidateScopedLocalCSS` / `formatScopedRules` / `rewriteCSSLinks` / `addBodyClass*`
   等约 600 行）——作用域归并出于 lossless 安全被**有意停用**，文档与 SKILL.md 都已写明，
   但实现整套留在树里且无测试可达。是删、还是留并标注，属产品决策。
7. **`internal/archguard` 三处覆盖偏窄**（只报告，未动，规则 0）：INV-7 的注册表豁免按
   **文件名** `register.go` 判定而非「仅 init 期写入」；INV-3 只匹配字面
   `os.<API>(...)` 选择器，别名 import 与「持有已打开的写句柄」都绕得过；INV-2 只禁
   `[]byte` 返回值与一张前缀表，返回 `string` 的整文档函数不在其中。

### 11.6 裁决后的执行结果

裁决：①去重只合并逐字节副本；②补契约执行形态字段 + 守卫对账；③删除已停用的
CSS 作用域归并实现；④授权收紧 archguard 三处偏窄覆盖。逐条落地如下。

#### 11.6.1 同类缺陷类在全部写入型能力上闭合

§11.5 第 1 条点出的四个能力已全部区域化，做法与 §11.2 一致
（`xhtml.ScanRegions` 判定真实标签 / `<style>` 内容 / `<?xml-stylesheet?>`，
其余字节透传），扫描截断一律转成带文件名与字节偏移的 warning，不再静默半改：

| 能力 | 改前的裸匹配 | 改后 |
|---|---|---|
| `epub.cover.replace` | `refs.go` 的 `subNameQuoteURI` / `subCSSURL` / `subCSSImport` 对整份文件生效，且对**每个 manifest 项**无条件跑 | 标记文件走区域；独立 `.css` 仍全文（整份文件本来就是 CSS） |
| `epub.package.merge` | 同上，且对**每册的每个 manifest 项**无条件跑 | 同上 |
| `epub.typography.optimize` | `typoLinkRe` / `typoHeadEndRe` 全文匹配，`</head>` 无注释/CDATA/`<script>` 排除 | `</head>` 与 `<link>` 只在真实标签上成立；保留「必须是行首非空白」的 Python 语义 |
| `epub.alite.convert` | `headEndRe` 全文匹配；`cardRe` 是裸针（不要求 `<`）；`addClassToTag` 手写扫描无引号感知 | 三处均按区域判定 |

`epub.cover.replace` 另修一处**同类但独立的路径**：`svg.go` 的
`resizeSVGCoverPages` 用裸 `strings.Index(lower, "<svg")` / `"</svg"` / `"<image"`
扫描。它要求字面尖括号，所以转义正文命中不了，但注释 / CDATA / `<script>` 里
**未转义**的示例 SVG 片段仍会被改写——而 `redline text` 不给注释内容设块，
这一类损坏它抓不到。现在三处候选都必须落在真实标签区间上，顺带修掉了原来
「找第一个 `>`」被属性值里的 `>` 截断的问题。

`alite` 的 `cardRe` 值得单独记一笔：它的失效方向是**漏做变换**而不是改坏正文
（版权页正文里原样写出 `class="copyright-card"` → 判为「容器已存在」→ 跳过真正的
结构包裹）。没有文本变化可比，红线结构上抓不到，只能靠人工发现。

#### 11.6.2 新落地的共享工具：`xhtml.TagParts`

区域化过程中，两个能力各自复制了一份逐字节相同的「从标签字节切出名字与属性」
小工具，注释里写着「caps 互不 import，各自维护一份」——那正是本轮要消除的反射。
已收进 `internal/scan/xhtml.TagParts`（层 4，两个 caps 本来就 import 它）。

#### 11.6.3 永久跳过的测试清零：16 → 0

§11.3 只处理了 `internal/redline` 的 9 个；余下 7 个（`cover` 3 / `merge` 1 /
`metadata` 2 / `split` 1）同因同治，已全部改为 Go-native 断言并删掉死掉的
oracle 脚手架（`runPythonHarness` / `pyCanonicalXML` / `compareEntries` /
`normalizePaths` → `replaceAll` → `indexOf` 死链等）。**全仓 `go test ./... -v`
的 SKIP 计数现在是 0。**

新断言里有几条值得点名：
- `cover`：SVG 封面页 `viewBox` / `<image>` 尺寸的缩放数学按 PNG IHDR 手算期望值；
  真书用例跑完 `epub.cover.replace` 后直接用 `redline.CheckText` 断言零发现。
- `merge`：冲突资源改名后引用确实从旧路径翻到新路径，未涉及的 entry 逐字节不变。
- `metadata`：`dcterms:modified` 不被字段写入触碰（此前只有注释声明，无测试）；
  `b.ModifiedNames()` 恰为 `{OPF, mimetype}`，其余 entry 逐字节不变（INV-1）。
- `split`：分段 nav / NCX 的目标（含锚点片段）全部解析到分段内部；
  `validateSegment` 的 metadata/cover/drm 红线块被**直接调用**证明它会开火——
  这个块经由公开 `Run()` 路径当前不可达（封面强制保留 + 加密提前拒绝），
  所以它今天只防内部回归，不防任何用户输入。这一点算设计注记，不是缺陷。

#### 11.6.4 取消语义（ctx）

`internal/extern.Run` 现在接收 `context.Context` 并用 `exec.CommandContext`。
两处超出「加个参数」的改动，都是为了让取消**可观测**：

1. `exec.CommandContext` 被杀时返回的是普通 `*exec.ExitError`（`signal: killed`，
   `ExitCode() == -1`），**不会**自动包上 `context.Canceled`。旧代码把这种情况
   当成干净跑完（`err == nil`）。现在 `Run` 在 `cmd.Run()` 之后检查 `ctx.Err()`
   并显式并入返回的 error，`errors.Is(err, context.Canceled)` 才成立。
2. `cmd.WaitDelay = 5s`。回归测试实测到真实挂起：`sh -c "sleep 5"` 被 SIGKILL
   之后孙进程 `sleep` 仍持有 stdout/stderr 管道，`Wait()` 会阻塞满 5 秒。
   `uv run python …`（fontcoverage 的实际形状，python 是 uv 的子进程）正是这个形状。
   **5s 这个常量与「限时 WaitDelay 而不是杀整个进程组」的取法值得人类复核。**

信封形态：`status: "cancelled"`（此前 `report.StatusCancelled` 与 v2 schema 的
`cancelled` 枚举**从未被赋值**）、一条 `error run.cancelled`、退出码 **1**
（SPEC §8.5 没有取消档；取消属于「没跑完」，映射到失败，但 status 与 finding
说清是取消而不是书有问题），且**不落盘**。注意与红线的区别：红线失败仍然落盘
供人工 diff review（§0 决策 2），取消不落盘。`cmd/epub` 用
`signal.NotifyContext` 接 SIGINT / SIGTERM。

pipeline 不依赖任何 capability 配合：除了 `errors.Is` 判 runner 返回的 error，
还在每个 stage 的**前后**边界自查 `ctx.Err()`，并对 `b.WriteToContext` 的 error 单独分类
（取消正好落在唯一那次写盘上时也算取消）。

**仍然开放**：16 个 capability 里 13 个的 `Run(ctx, …)` 完全不使用 ctx
（`split` / `sourceintake` / `fontcoverage` 已用）。其中 6 个（`content_analyze` /
`image_layout` / `migrate_epub3` / `navaudit` / `structure_normalize` / `styledemo`）
的 ctx 在到达真正的工作函数之前就被丢掉，下一轮需要沿调用链穿进去而不只是加一句检查。
每个包最该插入检查的循环点已逐条列在本轮的工作记录里。

#### 11.6.5 契约补执行形态字段

`contracts/schemas/v1/capability-manifest.schema.json` 新增必填字段
`execution: {input, output}`：

- `input`: `epub`（必须是 EPUB 文件，会被 `book.Open`）｜
  `epub-or-tree`（可缺省或指向目录 = 源树模式）｜
  `source-path`（目录或任意普通文件，永不 `book.Open`）
- `output`: `single`（pipeline 写一次 `--output`）｜
  `multi`（能力自行写 `output_dir` 下多个产物）｜`none`（只读）

运行时改为**读契约**：`run.go` 的 `noBookCap` / `sourceInputCap` /
`multiOutputCap` / `chainNeedsWrite` 都不再查 Go 侧 id 白名单。四个 `registerX`
仍然记录「作者在注册点声明的形态」，由两道对账断言与契约逐条比对：
`internal/pipeline.TestRegistryMatchesContractExecution` 与
`internal/docguard.TestContractsValid`（后者另查自洽：`output == none` 却声明
`requiresWriteAccess: true` 直接报错）。已做负向验证：故意把一条契约的
`execution.output` 改错，两道断言同时红；改回即绿。

顺带修掉那两处契约在说谎的地方——`epub.notes.popup.normalize` 与
`epub.style.demo.maintain` 的 `requiresWriteAccess` 由 `true` 改为 `false`
（二者执行面从不写盘）。行为零变化：`chainNeedsWrite` 本来就靠白名单把它们排除。

**未动**：这两条的 `kind`（分别是 `transformer` 与 `planner`）。`kind` 全仓没有
任何代码消费，纯声明性；`popup.normalize` 实际是校验器，要不要重定这个分类
留给所有者。

#### 11.6.6 删除已停用的 CSS 作用域归并

作用域归并出于 lossless 安全早已停用（`docs/pipeline/css-cleanup-system-fonts.md`
与 SKILL.md 都写明），但整套实现留在树里、不可达、无测试覆盖。已删除 39 个符号
约 775 行（`css_cleanup.go` -591、`pyutil.go` -132、`opfedit.go` -34、
`filemodel.go` -11、`register.go` -7）。本包 `deadcode` 报告从 30 条降到 0。

**对外行为逐字节不变**：`merge_scoped_local_css` 参数继续被接受，传 `true` 继续
只追加那条说明归并已停用的 warning，`scopedLocalStylesheetsMerged` /
`scopeClassesAdded` 恒为 `0`。新增 `TestMergeScopedLocalCSSOnlyWarns` 钉住这条
——删掉实现之后，这条 warning 就是该参数唯一的可观测行为。

一处判断已记录：`scopedSelector` / `selectorListParts` 等约 120 行原本被
`lossless_test.go` 的一个测试直接调用而显示为「可达」，但它们只为停用的归并存在。
连同那个只测它们自己的测试一起删了，而不是留 120 行只为满足一个自证测试。

#### 11.6.7 收紧 archguard 三处偏窄覆盖（规则 0 下的守卫演进）

与 2026-08-30 的 INV-6 补 v2 信封同性质：**经所有者授权、只扩覆盖面、不放宽任何
规则**，且每处都做了负向验证。

**INV-7（`state_test.go`）——豁免从「文件名」改为「写入时机」。**
旧规则：文件名是 `register.go` 就整份豁免。于是把一个运行期才写的缓存 map 放进
任何一个叫 `register.go` 的文件即可完全绕过 INV-7——豁免的是文件名，而规则原文
说的是「仅在 `init()` 期写入」。新规则：声明处初始化永远允许；声明之后的写入必须
发生在 `init()` 里，或发生在一个**只被 `init()` 调用**的函数里（`redline.Register`
就是这个形状），间接层数不限、递归检查、带环保护。检出 `x = …`、`x op= …`、
`x[k] = …`、`x.f = …`、`x++` / `x--`、`&x`。
负向验证三种形态全部命中：① `register.go` 里的运行期缓存；② 间接一层且调用点不在
init（**第一版漏了这个——`x++` 是 `IncDecStmt` 不是 `AssignStmt`，已修**）；
③ 复合赋值。合规对照（只被 init 调用的注册函数）放行。
已知边界如实写在守卫注释里：纯 AST 无类型信息，经接口值/函数值/反射的间接写入、
以及「把包级 map 当参数传给会写它的函数」都检不出来。

**INV-3（`disk_test.go`）——按导入别名解析，而不是假定字面标识符是 `os`。**
旧实现直接比较 `pkg == "os"`，于是 `import osx "os"` 后调用 `osx.WriteFile`
完全绕得过；`io/ioutil` 的 `WriteFile` / `TempFile` / `TempDir` 此前也不在检查范围。
两种形态均已负向验证命中。
仍然检不出「持有从白名单包返回的已打开写句柄」——那需要类型信息。

**INV-2（`serialize_test.go`）——补前缀表。**
硬判据只拦 `[]byte` 返回值，而本包注释又明确允许「读取小片段返回 string」，
于是返回 string 的整文档函数只要不叫表里那些名字就能同时绕过两条判据。
已补 `String` / `Bytes` / `Whole` / `Document` / `Unparse` / `Reserialize`，
负向验证命中。
两条判断记在守卫注释里：**`Build` 不禁**（「从结构化条目生成一份新文档」输入侧
没有原文档，不属于 INV-2 要防的解析→重序列化往返——这是受守卫不变式上的一个
judgement call，值得人类复核）；**「首参是文档、单一 string 返回值」这种结构判据
没有采用**（`scan/css.StripComments` 正是这个形状且是正当用途，加了就要开白名单，
而开白名单本身被规则 0 禁止）。这条缺口靠 AST 关不掉，如实留着。

#### 11.6.8 仍然开放（本轮明确未做）

1. **caps 之间的非逐字节重复**（逐字节相同的那两对已在 §11.6.9 合并）：
   `typography/opfedit.go` ↔ `css_cleanup/opfedit.go`、两份 `pyutil.go`、
   三份 `refs.go`、四份 `pkgio.go` 的 `readPackage`、两份 `xmlmini.go`（~600 行相同）。
   这些已各自漂移，合并要逐处判定哪一份才对。
   新增一条量化线索（§11.6.9 顺手测出）：`cover/pytool.go` 的 15 个函数里
   **11 个**与新建的 `internal/book/pypath` 行为相同（10 个仅差导出名与
   `urlParts` 字段名，`addProp` 只是把 `propText` 内联展开），
   1 个真有差异（`validateArchivePath` 用 `toolErrf` 而非 `fmt.Errorf`），
   3 个是 cover 专有（`attrEscapeFor` / `itemAttr` / `pathJoin`）；
   `metadata/pytool.go` 的 9 个函数里 8 个同族。把这两份迁到 `pypath`
   可再删约 300 行，并让 `pypath` 里现存的两个死函数（`addProp` /
   `containsStr`，为此保留）变活。
   另外本轮区域化在 `cover/refs.go` 与 `merge/refs.go` 里**新增**了两份逐字节相同的
   `rewriteMarkupReferences` 区域分发（与 `structure_normalize` 的第三份同形）——
   这是延后 `refs.go` 去重的直接代价，下一轮一并处理。
2. **13 个 capability 的 ctx** 见 §11.6.4。
3. **`kind` 分类** 见 §11.6.5。
4. **`extern` 的 `WaitDelay = 5s`** 见 §11.6.4。
5. **`typography` 的整文件替换**：`editset.Replace(path, 0, len(data), updated)`。
   不违反 INV-1 的字面规则（INV-1 管未修改 entry 的透传），且全仓其余触碰 XHTML
   的能力都是同一写法；改成基于区间的最小 edit 有复杂度成本而无 INV 收益，
   本轮保留并在调用点写明理由。没有 XHTML 字节 golden 时这个取舍只能按一致性判。

#### 11.6.9 逐字节副本合并（所有者裁决①：只合并逐字节相同的副本）

两对副本各 2 行差异（仅包名/导出名），已合并为单一事实来源：

| 原副本 | 归并到 | 层 |
|---|---|:--:|
| `caps/merge/pytool.go` ↔ `caps/split/pytool.go`（389/389 行） | **新建 `internal/book/pypath`**（402 行） | 5 |
| `caps/merge/navtoc.go` ↔ `caps/split/navtoc.go`（360/360 行） | `internal/scan/opf/navtoc.go`（325 行） | 4 |

`pypath` 必须落在层 5：caps（层 2）与 `scan/opf`（层 4）都要用它，而同层禁止互相
import，能被两者同时合法 import 的最近下层只有 `book`。`layerOf` 用最长前缀匹配，
所以 `internal/book/pypath` **不需要**改 archguard 的 `layer` 映射（规则 0 也不允许改）；
SPEC §1 因此只补了「子包按前缀同属该层」的说明与由此而来的陷阱
（`internal/book` 不能 import `internal/book/pypath`，同层），职责登记加在 §3 表里。

`BuildNav` / `BuildNCX` 落在 `scan/opf` 依赖 §11.6.7 记下的那个 judgement call
（INV-2 不禁 `Build` 前缀）。**这条判断现在有了具体依赖方**：它们返回整份 nav /
NCX 文档文本。判断本身未变（输入是 `[]TocGroup`，没有被解析的原文档，不存在
解析→重序列化往返），但值得所有者在复核 §11.6.7 时一并看这里。

**未能合并的部分（如实记录）**：`spineTocEntries` 与 `parseToc`（共 53 行，
merge/split 两侧逐字节相同）留在各自包内 —— 它们的参数是 `*pkgInfo`，
而两个包的 `pkgInfo` 结构本身已漂移（merge 有 `meta`，split 有 `byPath`），
`scan/opf`（层 4）又不能反向 import caps（层 2）。用接口或投影类型搬过去需要
在两个包各写约 30 行适配器，比省下的 53 行更多。等 `pkgio.go` 的 `readPackage`
进入去重范围时一并处理。

`ValidateArchivePath` 下沉后错误由 `toolErrf` 变成 `fmt.Errorf`，
`errors.Is(err, ErrPackageTool)` 会从真变假。全仓 `ErrPackageTool` 有 4 处声明、
**0 处消费**，因此行为上无人可见；但已在两处调用点重新包成 `toolErrf("%v", err)`
把可判性接回来（文本不变）。顺带记一条待裁决项：`ErrPackageTool` 是 4 个 caps
包各自声明、全仓零消费的死 API，而且能暴露它的路径都被 `failedResult(err.Error())`
压成文本 —— 它文档注释里「errors.Is 可判」这句目前并不成立。

`refs.go` 里那份**裸文本**扫描入口在 merge 侧已死、在 split 侧被 §11.6.10 换掉，
两处均已删除（merge −15 行；split −190 行，含只服务它的
`uriMatch` / `findNameQuoteMatches` / `findURLMatches` / `findImportMatches`
与 `register.go` 里的 `uriAttrNames`）。留在 split 的 `isWordRune` / `wordBoundary` /
`skipPySpace` 仍被 `collectCSSURIsStrict` 与 srcset 解析使用，已保留。

#### 11.6.10 真书回归查出的两处新缺陷（本轮已修）

这两条都是**跑真书才暴露**的，单测全绿时看不见 —— 也是 §11.3 那个教训的复现。

**(a) `epub.package.split` 的资源闭合收集没有区域化（与 §11.2 同类）。**
`split/pkgio.go` 的闭合 BFS 走 `collectRawURIsStrict`，那是裸文本正则式扫描，
不区分 `src="…"` 出现在标签里还是出现在字符数据里。仓库的回归样书本身是一本
**讲 EPUB 的书**，正文里大量示例代码只转义了尖括号：

```
<p>替换：&lt;audio clipBegin="…" src="../Audio/XinJing.mp3"/&gt;</p>
```

这里的 `src="../Audio/XinJing.mp3"` 是读者可见的正文，而 `OEBPS/Audio/XinJing.mp3`
并不在书里。旧扫描把它当真引用，于是 split 对一本能正常打开的书硬拒
（`resource closure: referenced target missing from source`）。
负向验证：同一份文档喂给旧扫描，字符数据、注释、CDATA 里的 3 条幽灵引用全部被误收。

修法沿用同包 `validation.go` 里**一直是对的**那套判据（`opf.ScanSpanTree` +
只看 `node.Attrs` 与 `<style>` 元素内容），新增 `collectMarkupURIsStrict`，
消掉同包内两套判据的分叉。新增两条测试，含真属性照常收集的负向控制。

顺带说明为什么此前没暴露：这条路径要等上游 nav.audit 的 error 变成非阻断诊断
（§9）之后才可达 —— 在那之前 split 更早就因上游 error 失败了。

**(b) 封面红线的消息自相矛盾。**
`coverCheck` 的判据用**映射后**的 before 路径（`MappedPath(o.PathMap, …)`），
消息却打印未映射的原路径，于是带 path-map 时会给出
`cover: cover-image path changed: 'OEBPS/Images/cover.jpg' -> 'OEBPS/Images/cover.jpg'`
——「变了但两边一样」，读起来像守卫抽风。已改为打印映射后的值；无 path-map 时
措辞逐字节不变。新增两条测试（消息断言 + path-map 与真实改名一致时必须放行的负向控制），
并已负向验证：还原旧写法后测试立刻红。

#### 11.6.11 真书回归查出的两处**待所有者裁决**（本轮未改）

**(a) `epub.package.split` 对悬空 `@font-face url()` 硬拒。**
修完 (a) 之后，split 在原始样书上停在**真实**的悬空引用上：
`Stylesheet.css` 有 6 条 `@font-face`，`src` 形如
`local("st"), local("SongTi"), …, url("../Fonts/YaSong.ttf")`，
而那 6 个 `.ttf` 都不在书里。这是中文 EPUB 常见且**有意**的写法（先系统字体、
url 只作兜底），本仓库还专门有 `docs/pipeline/css-cleanup-system-fonts.md` 讲它。
现在 split 因此完全无法处理本仓库自己的回归样书。三个选项：
① 保持硬拒（一个段落缺字体确实是缺陷）；② `src` 列表里存在 `local()` 兜底时
降级为 warning；③ 任何悬空引用都降为 warning。
这是**拒绝语义**的改动，不自行决定。

补齐这 6 个字体（并加 manifest 项、去掉 1 处真实断链 `nav.xhtml → Filename001.xhtml`）
后，split 在 49.5 MB 派生 fixture 上 **exit 0 / complete / redline 全 pass**，
`modifiedEntries` 含 `OEBPS/nav.xhtml` 与 `OEBPS/toc.ncx` —— 即 §11.6.9 合并后的
`BuildNav` / `BuildNCX` / `pypath` 在真内容上跑通并通过了红线闸门。

**(b) `merge` 的 `mappings` 事实是扁平 `from→to`，丢了「哪一本输入」这一维。**
merge 两卷同源书（802 条重命名）时，映射里含
`{'from': 'OEBPS/Images/cover.jpg', 'to': 'OEBPS/Images/vol2_cover.jpg'}`。
但**两本输入都有** `OEBPS/Images/cover.jpg`，只有 vol2 那份被改名。
扁平映射无法表达「同一路径来自输入 2 时搬走了、来自输入 1 时没动」，
redline 于是把 vol2 的映射套到 vol1 的封面上，报出一条假的 `redline.cover`。
（修完 §11.6.10(b) 后这条 finding 变成
`'OEBPS/Images/vol2_cover.jpg' -> 'OEBPS/Images/cover.jpg'`，问题本身可见了。）
这牵动本轮新加的 `epub.package.merge.mappings` 事实形状、`redline.LoadPathMap`
与 `skills/epub-package-operator/SKILL.md` 的措辞，属于契约改动，交所有者裁决。
注意 §11.4 那条 `TestMergeExposesRenamesAsPathMapFact` 之所以绿，
是因为它的 fixture 里封面路径不冲突。

真书其余能力无回归（`--input` 原始样书）：
`nav.audit` / `layout.audit` exit 1（真实审计 findings，redline 0）；
`text.content.analyze` / `structure.normalize` / `migrate.epub3` /
`typography.optimize` / `css.layering.optimize` 全部 exit 0、redline 0。

### 11.7 2026-09-19 快速合入复审修正

在 2026-09-07 补丁重新应用到基线后，补齐了以下会影响 CLI 实跑的边界：

- `execution.output=multi` 与 single 一样属于写出型；split dry-run 现在返回
  `approval-required`，实跑缺 `output_dir` 在 runner 前以 usage / exit 3 拒绝。
- stage runner 正常返回后再次检查 `ctx.Err()`，不消费 context 的最终只读 runner
  也不会在取消后误报 `complete`。
- `epub run ... --json` 的缺 capability、flag parse 和畸形 `KEY=VALUE` 都输出 v2
  usage envelope；`epub redline` 的 usage 与 nextCommands 统一为 flag-first 顺序。
- 5 个 pending 的纯 AI/人工能力不再因为未来的 `execution.output` 形状提前索要
  `--output`；现在均稳定返回 `capability.not-implemented` / exit 1，便于 agent 转入 skill 流程。
- `internal/extern` 对无 deadline 的调用补 30 分钟上限，并把 stdout/stderr 各限制为
  16 MiB；超限返回可由 `errors.Is(err, ErrOutputLimit)` 判断的错误，同时保留输出前缀。
- SPEC §7.1 改为当前互斥状态：16 个迁移 ready + 1 个 source intake planner +
  5 个纯 AI/人工 skill，共 22 个。

`internal/archguard/` 的补丁改动仍属于规则 0 要求的人类审阅边界；本轮没有继续修改。

### 11.8 2026-09-20 Agent 发现、局部试样与重构收尾

本轮沿用 Go 单一 CLI，不恢复旧执行面，不修改 archguard/docguard。当前仍为
17 个 ready + 5 个纯 AI/人工能力；后者不是待迁移的旧实现，不为补齐数量新增空壳。

**Agent 与真实执行面一起收敛：**

- AGENTS 只保留约束、按任务路由和验证边界；CLAUDE 只跳转。19 个 skills 均保持
  固定四段，明确何时调用、只读/写出区别、结果解释和下一步判断；移除重复规则和虚构执行能力。
- 新增 v2 参数目录（v1 manifests 不变）；`capabilities --id … --json` 可发现参数、
  默认值、执行形态。运行前拒绝未知参数、错误类型、重复 KEY，避免拼写错误静默生效。
- `style.demo.maintain catalog=true query=…` 从真实 OPF/spine/XHTML 发现样例，返回
  path/SHA256/stylesheets/classes；不是渲染、不是阅读器验收，不继承旧 artifact 的 pass。
- `typography.optimize scope_paths='["…xhtml"]'` 向选中 spine 页追加内容寻址 CSS，
  保留既有样式和其他章节。局部 preset 必须自包含；省略参数仍保留原整书模式。
  新版本样式不会自动删除旧试样资源，重复应用同版本幂等。
- normalize 的两个阶段、迁移与 typography 的 dry-run 都生成完整内存候选，交由
  pipeline 禁止落盘。后续应用命令保留 scope/preset 等原参数并做 shell 引号保护；
  失败的 dry-run 不再生成默认应用建议。

**复审修复与去重：**

- 所有者已批准 §11.6.11(a)：仅同一 `@font-face src` 存在有效非空 `local()` 时，
  split 可保留缺失 URL 的声明并继续，只写 `split.font-local-fallback` 事件，不发 warning。
  其他资源缺失仍失败；不声称目标阅读器一定存在系统字体，也不在新模板制造悬空引用。
- §11.6.11(b) 的 merge 扁平 `mappings` 只对应第一 `--input`（红线 before）；新增
  `sourceMappings[]` 按零基 inputIndex/input 隔离每本来源（含 identity 映射）。
  消除其他卷同名路径套到首卷封面的假红线，不把首卷红线冒充多来源无损证明。
- normalize 的 CSS 引用改走 lossless token/byte-range 扫描，删除旧裸文本 helper；
  demo 注释里的字体示例不再触发假缺失警告。扫描不确定的本地 CSS 转义会拒绝改写。
  同时修复 CSS 声明型 at-rule 投影遗漏、无引号 URL span 吞右括号两处扫描缺陷。
- cover/metadata 的相同行为路径工具复用 `book/pypath`；保留各包错误哨兵适配及
  不同语义函数，不盲合并已漂移的 `refs/pkgio/xmlmini/opfedit`。
- extern 输出超过单流 16 MiB 时立即取消直接子进程，保留诊断前缀；原 30 分钟兜底
  与 WaitDelay 5 秒保留。normalize/merge/typography 的主要循环增加取消检查；
  不声称所有历史同步解析函数都已获得细粒度取消能力。

**验证（2026-09-20，本地）：**

- `go build ./...`、`go test ./...`、`go vet ./...`、`go test -race ./...` 通过；
  archguard `-v`、docguard、legacy_surface 通过，旧执行面基线仍为零。
- 重新构建模板自身 `dist/epub-style-demo-20260920-105629.epub`，SHA-256
  `435188966fe8166d7486df86832ce7acaef1d94a4b02d64bf544d0139a74fdd2`；demo maintain、
  弹注、nav 与相对前次 demo 的全项 redline 通过。EPUBCheck 未在本地执行，仍由 CI gate 负责。
- 以该 demo 运行 normalize preview/apply：preview exit 2，apply exit 0，39 项改名映射
  一致；候选 SHA `75e1ff84804312393772f41eeaae49f57632b629cf728e23247407542b3c0e22`。
  外部全项 redline（带实跑信封 path-map）、nav、弹注与 XML 检查通过，无假字体警告。
- 局部试样仅选 `OEBPS/Text/01-body.xhtml`，候选 SHA
  `fafc22cf69666c9727fbaaf9ce2d89b794af74ad983d620a9b116a755e9781e7`。
  entry diff 仅该 XHTML、OPF 和新增 6 个独立 CSS；原样式与其余章节字节不变。
  nav、弹注、全项 redline、XML 通过；保留真实的低 class 覆盖率 warning。
- 候选仅作 CLI 回归，尚无本轮浏览器排版/原生阅读器验收，reader matrix 未改。
- 原始《EPub指南》另做 split preview/apply（split_points=0）：exit 2/0，7 条
  local-font 事件，没有新增 split warning，原声明保留。该书上游仍有 2 error/9 warn；
  候选 nav audit 仍检出既有 SVG/MathML manifest properties 缺失、字体与 GIF 风险。
  对原书的全项 redline 非零（重建导航/TOC 分区导致 XHTML 与 spine 差异），
  split 自身的 metadata/cover/DRM 与分区检查通过。仅作为拆分回归，不冒称成书全门禁通过，
  未顺带修订参考书，也未发布候选。

**留待后续而非合并到本轮：** 自动 before/after 视觉样例库、目标阅读器实测、
已漂移 helper 的语义统一。历史提交 `fc61716` 的守卫改动仍需所有者人工复核；
本轮没有通过改守卫放行实现，也未 push 或 merge。

### 11.9 2026-09-20 网页裁决后的内部重构

**所有者已提交的决定：** 本地问题页 `testMode=false` 答案保存于
`work/agent-cli-review-20260920/answers.json`，时间为 `2026-09-20T07:41:54.231Z`。
先做内部重构；后续目标阅读器为 Apple Books、Readest、Kindle Previewer，
样例方向保留几个可定制预设及灵活组合。该答案不等于阅读器实测证据。
所有者已人工接受历史提交 `fc61716` 的守卫改动，关闭 §11.8 中该待复核项；
这不授权继续修改 `internal/archguard/`，本轮未改守卫。

**实现与审查修正：**

- CSS 清理与 typography 的相同 POSIX 路径函数移到 `book/pypath`，OPF manifest
  遍历、单节点删除 edit 与新增 CSS item 片段移到 `scan/opf`。调用方直接使用公共
  内部 helper，移除重复文件与空转发层；CSS/OPF 源文件仍为字节区间编辑。
- 明确区分 raw relative path / percent-quoted URI、固定双引号属性转义 / 自动选引号、
  `NormJoin` 的仅去 fragment / `ResolveRelativePath` 的 URI 解码，防止去重抹平语义。
  typography 的 ID 分配不改变传入集合，保留它与 `pypath.UniqueID` 的副作用差别。
- nav/layout audit、content analysis、image layout 的 context 不再在入口被丢弃；
  传入资源遍历和 XML token 扫描。源文件 HTML/Markdown/plain-text 分段也检查取消。
  CSS 清理在资源循环及提交 editset 前检查；取消不转为书稿错误，不返回部分成功报告。
- 导航检测复用扫描时的原文，删除无效临时对象与重复取数；删除恒真条件及其误导注释。
  架构 SPEC §7.2 改为实测的完整内存 dry-run，明确只禁止磁盘写出，不能跳过内存应用。
- 新测试覆盖路径/转义边界、OPF namespace/顺序/相邻字节，以及真实注册 runner 的
  确定性中途取消；不依赖 sleep 或抢跑时序，不通过修改 golden/守卫掩盖行为差异。

**验证与非回归证据：**

- `go build ./...`、`go test ./...`、`go vet ./...`、`go test -race ./...`、
  archguard `-v`、docguard、legacy_surface、工作区与暂存区 diff 检查通过。
- 与 `dc5cbcb` 构建的 CLI 对照：demo 与原始《EPub指南》各运行 nav、layout、content、
  image、CSS dry-run、typography dry-run，共 12 组，stdout/stderr 和退出码逐字节一致；
  preview 无输出文件。参考书已有的审计失败保留，不冒称修复。
- CSS 清理、局部 preset、整书 preset 的实际输出也与旧 CLI 逐字节相同；SHA-256 分别为
  `0d5ab5028b5cb3f389050066393a9ba180e1fda1815a9c4e7b73053904ad807a`、
  `fafc22cf69666c9727fbaaf9ce2d89b794af74ad983d620a9b116a755e9781e7`、
  `17ea5410f3cd356c81f0eaccce0a712b5d0d2e6bf7a8cd29bf3f73bdd8e85b4c`。
- 模板重新构建为 `templates/epub-style-demo/dist/epub-style-demo-20260920-161248.epub`，
  SHA 仍为 `435188966fe8166d7486df86832ce7acaef1d94a4b02d64bf544d0139a74fdd2`。
  demo、CSS 清理候选、局部候选的 nav / popup / maintain / 全项 redline / XML 通过；
  maintain 的 EPUBCheck-skipped warning 保留，本地不代替 CI EPUBCheck。
- **整书 preset 仅证明非回归，不能当可发布样例**：nav / popup / 全项 redline / XML
  通过，但 maintain 有两个 error：替换后的 `fonts.css` 无正文锁定链，且与原 OPF
  `ibooks:specified-fonts=true` 不一致。这是原整书模式已有问题，本轮内部去重未改变它，
  没有修改 demo validator 或字体元数据来放行。结果未发布。
- 本地对照脚本、信封与候选在忽略目录 `work/agent-cli-review-20260920/evidence/refactor/`；
  复现脚本为该任务目录内 `compare-refactor.mjs` / `verify-refactor-artifacts.mjs`。
  后者如实以非零退出保留整书 preset 的专项校验失败。

**后续边界：** 优先修整书 preset 的自由/锁定模式保护，再制作可定制的 before/after
视觉样例。`refs/pkgio/xmlmini` 等已漂移实现仍须逐项证明等价，不能机械替换；
未覆盖的同步解析/其他 capability 仍有细粒度取消工作，不声称全仓完成。
Apple Books、Readest、Kindle Previewer 本轮均未实测，reader matrix 不变。
上述内部重构可独立审阅/合入；没有新增依赖、书稿改动、外部发布、push 或 merge。

### 11.10 2026-09-20–21 合入前的三项修复

本轮按所有者授权完成审查中的前三项：CSS 引用误改、整书 preset 字体模式保护、
ZIP/辅助输入限额与取消。没有修改架构守卫、依赖或书稿原件。

**实现：**

- cover / merge 使用共享 CSS token 引用扫描与 byte-range edits，仅重写真实
  `url()` / `@import` 的值，保留注释、content 字符串及选择器字符串。无法安全解释的
  本地转义 URI、未闭合 CSS、含实体转义的内嵌 CSS 明确拒绝，不返回部分改写。
  nav audit 同步识别真实 URL，避免把正确保留的生成文本误报为缺失资源。
- 整书 typography 保留既有 `Styles/fonts.css` 的原始字节与 OPF 字体元数据，报告
  `fontMode=free|locked`、`fontModeAction=preserve`、字体层 `action=keep`。
  正文链与元数据冲突、正文链位于其他 CSS 层或涉及导入／动态／内联等无法安全判定的
  模式时拒绝处理；不自动生成新字体链或更改元数据来通过检查。局部追加模式保持原行为。
  这项保护只覆盖受支持的字体层模式，不是完整 CSS 级联或字体覆盖验证器。
- zipfs 默认限制压缩输入 512 MiB、100,000 条目、单条目 256 MiB、声明解压总量
  1 GiB、路径 4096 字节；解压流再次检查实际读取量。每本 book 缓存原文及修改内容
  合计上限 512 MiB；这是内容预算，不是进程峰值内存承诺。普通 ZIP 与 ZIP64 的目录
  声明及实际条目数会在标准库展开目录前检查，避免超限归档先分配大量条目对象。
- 封面输入限 64 MiB，路径映射 JSON 限 16 MiB；preset JSON 限 1 MiB，单 CSS
  限 4 MiB、合计 16 MiB、1–32 个不重复层。普通文件检查集中到 I/O 层，Unix 使用
  非阻塞打开后检查文件描述符，FIFO 不会等待读端／写端。文件哈希、解压、辅助文件读取
  和 ZIP 写出传递取消；失败读取不返回部分内容，成功空内容保持非 nil，避免误作删除。
- book 记住首次真实读取／预算失败，pipeline 在 stage 与 redline 后复核；即使旧能力
  忽略错误，也不能返回部分成功或写出候选。单文件及多产物事务同样在落盘前拒绝。

**产物与回归：**

- `go build ./...`、`go test ./...`、`go vet ./...`、`go test -race ./...`、
  archguard `-v`、docguard、legacy_surface 及工作区／暂存区 diff 检查通过。
  Windows amd64 与 Linux amd64 交叉构建通过；平台运行验证在 macOS 完成。
  回归覆盖 FIFO、限额、伪造 ZIP 大小、确定性中途取消、空内容、读取错误被吞及单／多产物
  不落盘。CLI 实跑的超大 EPUB、超大封面、目录封面、FIFO 封面均 exit 1 且没有输出文件。
- demo 重建为 `templates/epub-style-demo/dist/epub-style-demo-20260920-230159.epub`，
  SHA `435188966fe8166d7486df86832ce7acaef1d94a4b02d64bf544d0139a74fdd2`。
- 修复后的整书 preset SHA
  `d06d5231c4510729a7e47dd4d2d804eddf8f86a42cac6d498dc87808ff95649f`。
  原先的两个 maintain 字体错误消失，原字体层字节不变；保留低 class 覆盖率 warning。
- 局部 preset 与 CSS 清理候选 SHA 仍分别是
  `fafc22cf69666c9727fbaaf9ce2d89b794af74ad983d620a9b116a755e9781e7`、
  `0d5ab5028b5cb3f389050066393a9ba180e1fda1815a9c4e7b73053904ad807a`。
- cover 复现样例中 `content: "url(../Images/old-cover.png)"` 保持原字节，真实封面引用
  正常改名；候选 SHA `f00aeb7334d19cdbc669b25654de6ca6486a47263d731f01f17942af13ad451b`。
- demo、整书 preset、局部 preset、CSS 清理和 cover 五个产物的 nav / popup / maintain /
  全项 redline / XML 检查均通过；cover redline 带实跑信封 path-map。
  maintain 的 `styledemo.epubcheck-skipped` warning 如实保留，EPUBCheck 仍只由 CI 执行。
- 以 `e840756` CLI 对照 demo 与原始《EPub指南》的 11 组未改行为调用：退出码、stdout、
  stderr 完全一致，dry-run 无文件；参考书原有审计失败保留。整书字体保护属本轮预期行为变化，
  不列为旧行为等价样本。
- 本地复现脚本为 `work/agent-cli-review-20260920/verify-priority-fixes.py`，
  报告与候选在同目录 `evidence/priority-fixes/`，均被 Git 忽略。

**合入边界：** 主分支仍为 `8f769f0`，当前分支的提交图谱领先 21、落后 0，
`merge-tree` 无冲突；本轮工作区修改尚未提交。没有该分支的 PR 或 CI 运行记录，
最终提交 SHA 的 PR 检查（含 CI EPUBCheck）仍须通过后再合入。原生阅读器验收未进行，
reader matrix 不变；这一项不冒充已通过，也不把代码修复的可合入结论当作书籍发布验收。
