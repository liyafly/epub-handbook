# Changelog

## v0.4.0 - 2026-09-26

### Added

- **`epub version` 与 CLI 发行包**：新增 plain / JSON 版本信息（version、commit、构建时间、Go 版本和目标平台）；为 Linux amd64、Windows amd64、macOS arm64 和 macOS amd64 增加原生构建、脱仓 smoke、SHA256 校验和及使用说明。smoke 覆盖版本、能力发现和内嵌 typography preset，不代表阅读器验收。

### Changed

- **阅读器矩阵证据状态**：缺少精确 artifact SHA 或阅读器版本的历史 pass 保留为历史观察并改标 warn，不再用于证明当前产物兼容。

### Fixed

- **`epub.package.migrate.epub3` OPF 更新**：改用字节区间编辑，只改迁移命中的属性、metadata 与新建 manifest/spine 节点；保留非目标 OPF 原文，并对无法安全改写的文本形态明确拒绝。
- **`epub.package.migrate.epub3` XHTML 更新**：shell 属性、doctype、charset、stylesheet link 与弹注标记改为局部字节编辑，并要求真实标签边界匹配；删除无条件整页格式化，保留非目标标签、注释、PI、CDATA、属性顺序与正文空白。
- **`epub clean` 多步执行**：每本书复用一个打开的源 archive，在内存 Book 状态间串联步骤并只在最终批准时写一次；步骤失败会丢弃该步 fork。逐步报告改为状态 ID 与 `changedEntries`，不再构造中间 EPUB 或填写中间 ZIP SHA。
- **`epub.literary.structure.format`**：新增按显式 JSON 清单定位 spine XHTML 元素并追加白名单 class 的能力；支持多 class 合并、可选 manifest CSS 链接、歧义拒绝与整批 error 零写入。
- **`epub clean`**：默认改为仅审计并生成计划；结构步骤须由 `--steps` 明确选择，typography 还须给出 `--preset` 与 `--scope`。`--approve` 仅在步骤、末次审计与全项红线通过后写出；失败候选默认不保留，显式 `--retain-review-candidate` 才另存为 `.review-only.epub`。批次新增 `--json` v2 汇总。
- **`epub.typography.english.optimize`**：新增只补齐 spine XHTML 根节点 `lang` / `xml:lang` 的确定性写入能力；按主语言、body 声明和 CJK 比例跳过歧义页面，不改 CSS 与 OPF metadata。
- **`epub.vertical.ruby.optimize`**：新增 Ruby `<rp>` 后备括号与标准 `writing-mode` 厂商前缀两种确定性修补，支持精确范围、冲突跳过、整批 error 零写入和 dry-run 计划。
- **`epub.notes.legacy-fallback`**：新增多看旧版弹注 class fallback；要求标准弹注校验干净，支持精确 spine XHTML 范围与 dry-run 计划，任何 error finding 都不会应用部分编辑。
- **`epub.kindle.compatibility.check`**：新增只读 Kindle 静态风险 validator，覆盖 NCX/封面/图片、MathML、manifest CSS 与 spine XHTML；报告固定顺序的 finding 与计数，不代表目标阅读器验收。
- **字体工具 CLI**：`tools-font/subset-demo` 更名为 `tools-font/epub-font`，提供 `epub-font subset` / `epub-font check`；修复 `check` 将 CSS 非渲染字符串当作必需字符的误报。
- **引用安全与 EPUB 输入限额**（`390e27f`）：XHTML / CSS / SVG 引用改写限定在真实语法区域；ZIP 元数据、归档条目数、单条与总解压量及辅助输入加入上限与取消检查。
- **移除 EPUBCheck 拒绝的 CSS 方向规则**（`62c5b37`）：从 demo 的 `chapter-compat.css` 和 `poster.css` 删除 `direction: ltr`。

- **快速合入复审收口（2026-09-19）**：多输出 `split` 正确归类为写出型，dry-run
  返回 `approval-required`，实跑缺 `output_dir` 返回 usage / exit 3；pipeline 在 runner
  返回后复查 context，避免最终只读 stage 在取消后误报完成；`epub run --json` 的缺 ID、
  flag parse 与畸形 `KEY=VALUE` 均输出 v2 usage envelope；redline usage 与 nextCommands
  统一为 flag-first；5 个 pending 人工能力不再提前索要 `--output`，统一返回
  `capability.not-implemented`；外部 provider 增加 30 分钟默认时限与 stdout/stderr
  各 16 MiB 上限，超限以 `ErrOutputLimit` 可判定。
- **`epub.package.split` 的资源闭合把正文当引用（真书上导致硬拒）**：闭合 BFS 走的
  `collectRawURIsStrict` 是裸文本正则扫描，不区分 `src="…"` 出现在标签里还是出现在
  字符数据里。回归样书本身是一本讲 EPUB 的书，正文示例只转义尖括号
  （`<p>替换：&lt;audio src="../Audio/XinJing.mp3"/&gt;</p>`），于是那条读者可见的
  正文被当成真引用，而该音频文件并不在书里 —— split 对一本能正常打开的书报
  `resource closure: referenced target missing from source` 并硬拒。
  改用同包 `validation.go` 里一直正确的判据（`opf.ScanSpanTree` + 只看
  `node.Attrs` 与 `<style>` 元素内容），新增 `collectMarkupURIsStrict`，
  消掉同包内两套判据的分叉；裸扫描入口连同只服务它的 190 行辅助一并删除
  （merge 侧那份已死，同时删）。负向验证：旧扫描在同一份文档上误收字符数据 /
  注释 / CDATA 里的 3 条幽灵引用。
- **封面红线的消息自相矛盾**：`coverCheck` 判据用**映射后**的 before 路径，消息却打印
  未映射的原路径，于是带 `--path-map` 时输出
  `cover-image path changed: 'X' -> 'X'`（真书 merge 实测到）。现在打印映射后的值；
  无 path-map 时措辞逐字节不变。
- **同类「全文裸匹配改写字符数据」缺陷在全部写入型能力上闭合**：`epub.cover.replace`
  与 `epub.package.merge`（`refs.go` 的 `subNameQuoteURI` / `subCSSURL` /
  `subCSSImport` 此前对**每个 manifest 项**无条件全文匹配）、
  `epub.typography.optimize`（`</head>` / `<link>` 无注释/CDATA/`<script>` 排除）、
  `epub.alite.convert`（`headEndRe` 全文匹配；裸针 `cardRe` 会把正文里写出的
  `class="copyright-card"` 误判为「容器已存在」而**跳过真正的结构包裹**——
  这一类是漏做变换，没有文本变化可比，红线抓不到）。做法与 `structure_normalize`
  一致：只在 `xhtml.ScanRegions` 认定的真实标签 / `<style>` 内容 /
  `<?xml-stylesheet?>` 上改写，扫描截断转成带文件名与字节偏移的 warning。
  独立 `.css` 文件仍全文改写（整份文件本来就是 CSS）。
- **`epub.cover.replace` 的内联 SVG 缩放同样区域化**：`svg.go` 用裸
  `strings.Index(lower, "<svg")` / `"</svg"` / `"<image"` 扫描，注释 / CDATA /
  `<script>` 里**未转义**的示例 SVG 片段会被改写，而 `redline text` 不给注释内容
  设块、抓不到。顺带修掉「找第一个 `>`」被属性值里的 `>` 截断的问题。
- **永久跳过的测试清零：16 → 0**。除 `internal/redline` 的 9 个（见下），
  `cover` 3 / `merge` 1 / `metadata` 2 / `split` 1 同因同治，已全部改为 Go-native
  断言并删除死掉的 oracle 脚手架。新覆盖包括：SVG 封面 `viewBox` 缩放数学、
  merge 冲突改名后的引用翻转、`metadata` 不触碰 `dcterms:modified` 且非 OPF entry
  逐字节不变、split 分段 nav/NCX 目标全部解析到段内、以及直接调用
  `validateSegment` 证明其 metadata/cover/drm 红线块会开火。
- **取消（ctx）真正可用**：`internal/extern.Run` 改为接收 `context.Context` +
  `exec.CommandContext`。两处必要的额外处理：`exec.CommandContext` 被杀时返回的是
  普通 `*exec.ExitError`（`signal: killed`），**不会**自动包上 `context.Canceled`，
  旧代码把它当成干净跑完；以及 `cmd.WaitDelay = 5s`——回归测试实测到真实挂起
  （被 SIGKILL 后孙进程仍持有管道，`Wait()` 阻塞满时长），而 `uv run python …`
  正是这个形状。信封新增 `status: "cancelled"`（`report.StatusCancelled` 与 v2
  schema 的枚举此前**从未被赋值**）+ 一条 `error run.cancelled` + 退出码 1，
  且**不落盘**（与「红线失败仍落盘供人工 review」相区别）。`cmd/epub` 接
  SIGINT / SIGTERM。
- **`text` 红线漏掉行内元素里的正文（迁移回归，最严重的一条）**：
  `redline.ExtractTextBlocks` 只把字符数据写进**最内层**打开元素的缓冲，子元素闭合时
  没有并回父帧，于是 `<p><em>…</em></p>`、`<p><a>…</a></p>`、`<li><a>章名</a></li>`
  这类文本完全不进块哈希——整段被行内元素包裹时该段甚至不产出块，可以被整段删除而
  红线零 findings。oracle（`scripts/validate_text_invariance.py` 的
  `element_text_without_ignored`）是 `node.text` → 递归子元素 → `node.tail` 的完整
  递归收集，只在 `IGNORED_TEXT_TAGS` 与 note-control 锚点剪枝并保留其 tail；Go 侧现已
  对齐。样本书《EPub指南》Chapter12-2 的块数由 558 升到 578，`structure.normalize` /
  `typography.optimize` / `css.layering.optimize` 实跑仍为 0 findings（只扩覆盖面，
  未引入误报）。回归：`internal/redline/textblocks_test.go`（14 种取文形状 + 3 种
  端到端改动）。
- **`epub.package.migrate.epub3` 改写作者正文**（交接文档 §10 记为"红线能拦住但尚未修"）：
  `normalizeDuokanNotes` 的三条 `class="duokan-…"` 字面替换针不含尖括号，对全文生效，
  于是讲解多看注释写法的正文（`&lt;li class="duokan-footnote-item"…&gt;`，尖括号转义、
  属性是裸字节）被当成标记改掉。样本书实跑 8 条 `error redline.text`，能力在该书上不可用。
  现按 `structure_normalize` 已验证的方案区域化：class 改名只发生在真实标签字节内
  （`xhtml.ScanRegions` 的 tag 区域），区域扫描截断时给出带偏移的 warning 而不静默半改。
  实跑由 `failed` / 退出码 1 / 8 findings 变为 `complete` / 退出码 0 / 0 findings
  （`duokanNotesNormalized` 15 → 7，减少的 8 次正是那 8 处正文误改）。
- **用法错误不再吞掉信封**：`epub run … --json` 在退出码 3 上曾输出零字节，`--json` 的
  调用方必须另写一条"stdout 不是 JSON"的分支，原因只能从 stderr 自由文本里猜。现在
  用法错误照常给出合 v2 schema 的信封（`status: failed` + 单条
  `error usage.invalid-argument`，退出码仍是 3）。同时 `split_points` / `expect_volumes`
  的解析失败改走 `pipeline.UsageError`（退出码 3），不再与 `max_files` 一个退 3 一个退 1。
- **改名映射出得了信封**：`epub.package.merge` / `epub.cover.replace` 的改名此前只走内部
  `Result.Renames`（喂 pipeline 的红线 path map），信封里只有计数，`epub redline --path-map`
  在合并或换封面之后没有映射可用；`skills/epub-package-ops/SKILL.md` 却让 agent 去读
  信封里并不存在的 `renames` 字段。现在两者都给出与 `epub.structure.normalize` 同形状的
  `facts` 键 `mappings`（`{from,to}` 数组，空时为 `[]`），SKILL.md 同步改为实际键名。
- **`epub.font.coverage.analyze` 的错误归因**：`extern.Run` 的 error 被丢弃，进程起不来时
  （含 `ErrToolMissing`）会用零值 `CmdResult` 报出 `exit code 0`，暗示工具跑完且干净退出，
  把真正原因藏起来。现在直接以"could not be started: %v"失败。
- **数组形状的 facts 不再序列化成 `null`**：`epub.package.merge` 的 `warnings` / `inputs`
  （`append([]string(nil), …)` 在源切片为空时返回 nil）、`epub.source.intake` 的 `files`
  （空目录即 `intake.empty` 这条一等错误路径）、pipeline 的 `modified_entries`。
- **守卫的牙齿**（SPEC §5.1.1）：`archguard.yml` 的"守卫是否被改动"检测在浅克隆下
  `git diff <base> HEAD` 必然失败、警告永不触发，现在 `fetch-depth: 0` 并在 base commit
  取不到时显式报错；检测范围与 `CODEOWNERS` 一并覆盖 SPEC §5.1 同样声明为"需人类审阅"的
  `internal/legacy_surface/`、`internal/docguard/` 与 INV-10 棘轮基线。

### Changed

- **Skills 按任务从 19 个合并为 6 个入口**：

  | 新 skill | 合并来源 |
  | --- | --- |
  | `epub-audit` | `epub-package-nav-auditor`、`epub-layout-auditor`、`epub-content-analyzer`、`epub-image-layout-optimizer`、`epub-font-coverage-analyzer` |
  | `epub-cleanup` | `epub-structure-normalizer`、`epub3-migrator`、`epub-css-layering-optimizer`、`epub-typography-optimizer`、`epub-popup-footnote-converter` |
  | `epub-package-ops` | `epub-package-operator`、`epub-alite-converter` |
  | `epub-source-intake` | 保持不变 |
  | `epub-special-layout` | `epub-english-typography-optimizer`、`epub-literary-structure-formatter`、`epub-vertical-ruby-optimizer`、`epub-legacy-footnote-fallback` |
  | `epub-reader-verify` | `epub-kindle-compatibility-checker`、`epub-style-demo-maintainer` |

- **共享字节编辑 helper 并传播扫描取消**（`77235e9`）：公共路径与 OPF edit helpers 供能力复用；将 context 传入目录遍历、XML token 扫描和 CSS 清理阶段。

- **删除契约的 `adapters` 字段**（22 份契约 + schema 的 `required`/`properties` + docguard 校验 +
  `pipeline.Contract.Adapters`）。该字段枚举 `openai` / `claude` / `mcp` / `cli`，把「哪家 harness
  能调这条能力」写进了机器契约；而 CLI 侧**从不读它**（`Contract.Adapters` 声明后零消费）。
  本工具链要作为 skill 接入任意 harness，厂商名单不该是契约的一部分 —— 能力面由
  `epub capabilities` 自描述，接入方自行决定怎么调。`skills/*/agents/openai.yaml` 保留不动。
  `additionalProperties: false` 使该字段现在被**主动拒绝**（负向验证：加回去 schema 与字段两道
  校验同时红），不只是"不再必填"。
- **AGENTS.md 精简 6%**（12840 → 12062 字符）：合并重复 3 次的迁移史与重复的 SPEC 阅读顺序；
  4 条排版细则（图文环绕 / 带样式下划线 / MathML `properties` / 弹注复核）换成一条指向
  `SPEC-实现约束.md` 的指针 —— 它们在 SPEC 与手册里本来就都有，而 AGENTS.md 自己就写着
  「这类条目优先写入 SPEC-实现约束.md」，属自我违规；授权正文校订分支只保留禁令原文
  （最高安全属性，逐字不动），逐条要求交回被引的两份文档。
  字体 provider 的安装与降级说明移入 `tools-font/README.md`（原先 AGENTS.md 表格里塞了整套说明，
  而那份 README 反而没有；顺带修掉它开头「与 `scripts/`、`swift/` 并存」的过期表述）。

- **逐字节相同的 caps 副本合并为单一事实来源**（所有者裁决：本轮只合并逐字节副本）。
  `caps/{merge,split}/pytool.go`（389/389 行，2 行差异）归入**新建**
  `internal/book/pypath`（层 5）；`caps/{merge,split}/navtoc.go`（360/360 行）归入
  `internal/scan/opf`，`BuildNav` / `BuildNCX` 导出并返回 `string`。
  层 5 是唯一合法落点：caps（层 2）与 `scan/opf`（层 4）都要用，而同层禁止互相 import。
  `layerOf` 用最长前缀匹配，因此**不需要**改 archguard 的 `layer` 映射（规则 0 也不允许）；
  SPEC §1 只补了「子包按前缀同属该层」及由此而来的同层陷阱说明，职责登记进 §3。
  `spineTocEntries` / `parseToc`（53 行）未能合并 —— 参数是各自包私有且已漂移的
  `*pkgInfo`，搬过去要写的适配器比省下的代码更多，如实记在交接文档里。
  `ValidateArchivePath` 下沉使 `errors.Is(err, ErrPackageTool)` 会从真变假；
  全仓该哨兵 4 处声明、0 处消费，行为上无人可见，仍在两处调用点重新包回以保住可判性。

- **契约新增必填字段 `execution: {input, output}`**（`capability-manifest.schema.json`）：
  描述 pipeline 怎么调这条能力（`input`: `epub` / `epub-or-tree` / `source-path`；
  `output`: `single` / `multi` / `none`）。此前执行形态**只**以
  `internal/pipeline/register.go` 里的四张 id 白名单存在，契约完全不提，于是
  「`contracts/` 是机器契约唯一事实来源」对执行形态并不成立——而且已经分叉：
  `epub.notes.popup.normalize` 契约写需写权限却注册为只读，
  `epub.style.demo.maintain` 契约写 planner + 需写权限却永不写盘。现在运行时读
  契约（`noBookCap` / `sourceInputCap` / `multiOutputCap` / `chainNeedsWrite`），
  注册点与契约的一致性由 `pipeline.TestRegistryMatchesContractExecution` 与
  `docguard.TestContractsValid` 两道对账断言保证（后者另查自洽：`output == none`
  却声明 `requiresWriteAccess: true` 直接报错）。上述两条契约的
  `requiresWriteAccess` 已改为 `false`，行为零变化。
- **删除已停用的 CSS 作用域归并实现**：约 775 行、39 个符号，`deadcode` 在
  `css_cleanup` 的报告从 30 条降到 0。对外行为逐字节不变——
  `merge_scoped_local_css` 参数继续被接受，传 `true` 继续只追加那条说明归并已
  停用的 warning，`scopedLocalStylesheetsMerged` / `scopeClassesAdded` 恒为 `0`；
  新增 `TestMergeScopedLocalCSSOnlyWarns` 钉住这条（删掉实现之后，这条 warning
  就是该参数唯一的可观测行为）。
- **收紧 archguard 三处偏窄覆盖**（规则 0 下的守卫演进，经仓库所有者授权，
  只扩覆盖面不放宽规则，每处均做负向验证）：
  INV-7 的注册表豁免从「文件名是 `register.go`」改为行为判定「仅在 `init()` 期
  写入，或在只被 `init()` 调用的函数里写入」（递归、带环保护；检出
  `x = …` / `x op= …` / `x[k] = …` / `x.f = …` / `x++` / `&x`）；
  INV-3 改为按导入别名解析，`import osx "os"` 与 `io/ioutil` 的写 API 不再绕得过；
  INV-2 补 `String` / `Bytes` / `Whole` / `Document` / `Unparse` / `Reserialize`
  前缀。三处的已知边界与两条刻意不加的判据都写在守卫注释里，没有当成已关闭。
- **`xhtml.TagParts`**：区域化过程中两个 caps 各自复制了一份逐字节相同的
  「从标签字节切出名字与属性」小工具（注释里写着「caps 互不 import，各自维护
  一份」），已收进 `internal/scan/xhtml`。
- **标记区域扫描下沉到 `internal/scan/xhtml`（SPEC §1 第 4 层）**：原
  `internal/caps/structure_normalize/markup_regions.go` 是通用、无能力耦合的扫描器，
  却因 `caps` 之间禁止 import 而无法复用（交接文档 §10 已记下这一点）。现移为
  `internal/scan/xhtml/regions.go` 并导出 `ScanRegions` / `Region` / `RegionTag` /
  `RegionStyle` / `RegionStylesheetPI` / `ScanComplete`（只返回区间，不返回文档字节，
  INV-2 不受影响），`structure_normalize` 与 `migrate_epub3` 共用同一份；`internal/scan/xhtml`
  此前**零消费者**。

### Added

- **Agent 安全预览与场景发现**（`4ae7e43`）：demo catalog 支持 `catalog=true` / `query=` 场景查询；typography 增加 `scope_paths` 局部试样。

- **`epub.source.intake` Go 实现（最小盘点 planner）**：`epub run epub.source.intake --input <目录或文件> --json`
  对非 EPUB 源材料做只读盘点——按扩展名分角色（text/html/image/pdf/epub/font/css/audio/video/
  archive/document/unknown）、流式 SHA-256、UTF-8/BOM/CRLF、非 EPUB 核心图片格式与 CMYK JPEG
  头部探测；findings `intake.*`（`empty` / `too-many-files` 为 error，其余 warn/info）；facts
  给出 `files[]`、`roleCounts`、`workspacePlan` 与合 `execution-plan.schema.json` 的 `plan` /
  `blockers`；输入里已有 `.epub` 时 `nextCommands` 指向 `epub.package.nav.audit`。按仓库所有者
  决策，PDF 解析、OCR 与图片转码明确不在范围内，只作标记。pipeline 新增 `registerSourceInput`
  注册类（`--input` 可为目录或任意常规文件，永不 `book.Open`，不写输出，dry-run 保持 `complete`）；
  `epub capabilities` 现在 17 / 22 ready。golden：`testdata/envelope/source-intake-directory.report.json`
  （fixture `testdata/sourceintake/`）；`pending-capability` golden 改为以 B 类
  `epub.notes.legacy-fallback` 捕获。
  健壮性边界：`--input` 为符号链接目录时先解析根再遍历（树内符号链接仍只记录不跟随）；
  非常规文件（FIFO、设备文件）在 pipeline 计算信封 SHA-256 之前就以退出码 3 拒绝，不再挂死；
  不可读子目录记 `warn intake.unreadable-dir` 并跳过该子树，已盘点结果照常给出，不可读文件
  带 `unreadable` risk 且不参与 blocker 推断；`max_files` 非正整数以退出码 3 拒绝（经新增的
  `pipeline.UsageError` 通道），不再静默回退默认值；命令里的路径按需 shell 引号化，
  `intake.already-epub` 的 detail 与 `nextCommands` 用同一条命令文本。

### Removed

- **`--legacy-report` 迁移脚手架**（SPEC-go-architecture §5.2 唯一被批准的临时脚手架，
  触发条件——Python 脚本删除——已于 2026-08-29 满足）：删除 `epub run --legacy-report`
  flag、`legacy_report=true` 参数、pipeline 透传与所有 capability 的
  `facts.legacyReport`。曾只在 legacy 报告中出现的数据提升为正式 camelCase `facts`
  键（见各 SKILL.md 与 `docs/pipeline/refinement-harnesses.md`）。
  `epub redline --path-map` 现在直接接受 `epub run epub.structure.normalize --json`
  保存的信封（读取 `*.mappings` facts），旧的 `{stages:[{mappings}]}` / `{mappings}`
  形状继续兼容；不再需要 jq 提取 legacy 报告。行为收紧：`--path-map` 认不出任何
  mappings 数组时（失败 normalize 的信封、`facts` 为空、`*.mappings` 不是数组、顶层
  数组等）现在以退出码 3 报输入错误，不再静默当成空映射继续比对——后者会产生满屏
  「文件缺失 / 新增文件」的假红线。未改名的成功运行输出 `"mappings": []`，仍然合法。

### Fixed

- **requires 链上游不再阻断目标能力**（回归自 ff30a1a）：`epub run <id>` 执行完整
  requires 链时，上游 stage（如 `epub.package.nav.audit`）的 `status: failed` 或 error
  finding 曾直接中止链条，导致 `epub.structure.normalize` / `epub.package.migrate.epub3` /
  `epub.font.coverage.analyze` 等在任何带审计 error 的真书上都返回 `failed` / 退出码 1 /
  `output: null`。现在上游是非阻断诊断：结果落入 `facts["<id>.status"]` /
  `facts["<id>.findings"]`，信封只追加一条 `info upstream.diagnostics`，事件为
  `<id>:completed`；runner Go error 与未实现能力仍然阻断。
- **红线失败不再吞掉输出**（回归自 ff30a1a）：写出型能力的红线 error / 校验器错误仍把
  状态降为 `failed`、退出码 1，但唯一一次落盘照常执行、信封 `output.path` / `sha256`
  照常给出，供人工 diff review。此前 `epub.metadata.edit` / `epub.package.merge` /
  `epub.cover.replace` / `epub.package.migrate.epub3` 因自身契约红线必被预期变更触发而
  永远无法产出文件。语义记录见 `docs/pipeline/go-rewrite-handoff.md` §0 决策 1–2 与 §9。
- **`epub.structure.normalize` 不再改写正文里形似属性的文字**：引用重写器此前对整份
  XHTML/NCX/SVG/SMIL 字节做 `src|href|…="…"` 与 `url()`/`@import` 正则替换，会命中字符
  数据中被实体转义的代码示例（如 `&lt;text src="../Text/Chapter2-2.xhtml#\2"/&gt;`、
  `src=" ../Images/note.png"`），改写作者正文并触发 `redline.text`（Python 版同缺陷）。
  现在先做一次前向区域扫描，URI 属性与内联 style 只在真实标签内部改写，CSS 重写只作用于
  `<style>` 元素内容与 `.css` 文件；注释、CDATA、PI、DOCTYPE、`<script>` 内容与正文逐字节
  保留（仍是字节区间替换，不构树、不重序列化）。样本书 normalize 现在红线零发现。
- **只读能力的 `--dry-run` 不再建议 `--output`**：`nextCommands` 此前在任何 dry-run 下都
  给出 `epub run <id> --input <reviewed-input> --output <out.epub>`，对不写盘的能力
  （`epub.source.intake` / `epub.style.demo.maintain` / `epub.notes.popup.normalize`）
  是错误指引——这些能力根本不接受 `--output`。现在链上无写入需求时 dry-run 不再给出该建议；
  写出型与多产物能力的 dry-run 文案不变。

- **`epub.structure.normalize` 区域扫描的三处收尾**：`<?xml-stylesheet …?>` 的 `href`
  重新参与改写（区域化扫描初版整段跳过全部 PI，SVG/XHTML 里这条合法样式表引用改名后会
  静默断链，`anchors` 红线只校验 id 存活、发现不了）；扫描器遇到 DOCTYPE 内部子集
  （`<!DOCTYPE html [ <!-- don't --> ]>` 的撇号曾被当成属性引号扫到 EOF）、HTML 空注释
  `<!-->` 与标签内不配对引号时不再静默放弃文档剩余部分——前两者正确跳过，最后一种仍然
  停止但会给出带文件名与字节偏移的 `structure.warning`；`url()`/`@import` 只在 `style=""`
  属性值里改写，`title` / `alt` 等读者可见文本不再被当成 CSS。样本书输出字节不变。

## v0.3.0 - 2026-08-30

Go 单一 CLI 重写落地（SPEC-go-architecture W0–W5），并完成一轮迁移后 review 收尾。

### Added

- **`cmd/epub` + `internal/`**：唯一公开 CLI（`epub run <id>` / `epub capabilities` /
  `epub redline`），统一 JSON 信封（schemaVersion 2）、退出码 0/1/2/3、契约驱动的
  requires 校验。16 个 A 类 capability 全部注册并通过 parity gate。
- **`internal/archguard/`**：十条不变式的可执行守卫（INV-1 至 INV-10），配套独立
  CI job、CODEOWNERS 与 PR 模板自检。
- **`contracts/schemas/v2/envelope.schema.json`**：统一信封的 JSON Schema；
  INV-6 守卫按 golden 的 schemaVersion 分发校验（v1 迁移期形状保留）。
  golden 报告含真实 CLI 捕获（源树校验、pending 拒绝、nav audit findings）。

### Changed

- `skills/`（19 个 SKILL.md）全部改写为 `epub run <capability-id>` 形态；
  `docs/` 与根文档的旧执行面引用清零（棘轮 149 → 0）。
- pre-commit hook 与 CI 改调 Go 守卫与 epub CLI；EPUBCheck 保留为 CI gate。
- **pending 能力语义**：契约存在但无 Go 实现的能力（5 个纯 AI/人工 skill +
  `epub.source.intake`）现在返回 `status: failed`、退出码 1、`error
  capability.not-implemented`，不再伪装成 `complete` / 退出码 0。
- `epub redline` 补 `--path-map`；`--legacy-report` 迁移期脚手架保留（拆除为
  独立后续任务）。

### Removed

- 旧执行面整体删除：`scripts/`（75 py + 3 sh）、`swift/`（303M）、`gui/`、
  `adapters/`、`tools/parity/`、Python 环境文件（`.venv` / `uv.lock` /
  `pyproject.toml` / `.python-version` / `mise.toml`）。
- `tools-font/coverage-detector/` 明确不迁，保持 Python 独立项目，经
  `internal/extern` 调用。

## v0.2.10 - 2026-08-15

本版本聚焦于仓库入口、维护治理和阅读器实测证据的整理，让新用户更容易开始做书，
也让 AI 与专业维护者能够沿着统一规则复现 EPUB 处理流程。

### Changed

- **仓库分层与文档精简**：根 README 收敛为“做书 / 修书 / 查问题”三条普通人入口，
  AI 与专业维护统一路由到 `AGENTS.md` 和 `scripts/README.md`。
- `docs/learn/` 从线性 00–09 教程收敛为唯一入口、做书页、进阶结构兼容页和按需查页面。
- 已完成的 `docs/meta/`、`docs/experiments/`、`docs/source/` 迁入 `archive/`；
  未跟踪的 `docs/superpowers/` 退出仓库工作树。
- 归档历史流水线决策，修正归档索引、贡献指南与 CI 中残留的旧目录入口。
- 将匿名技术书 v3.1 的长 MathML 表格与图片宽度实测分层回写到脱敏 demo、reader
  matrix、场景指南、手册和排版决策记录；精确 pass 只绑定已验证 artifact SHA，
  新 demo 保持 `warn` 待复测。
- 将匿名插图型英文合集的人工验收结论回填到英文排版指南、终极手册与排版决策记录；
  保留正文自由与展示角色分层方法，不把单书数值或缺少版本信息的反馈写成阅读器 `pass`。
- 将匿名英文单本小说两项选择性反馈回写：Apple Books 8.5 章题居中现象及其 CSS 级联解释，
  以及 Reeden 不支持 `::first-letter` 的待验证假设；只落通用方法与待复测结论，不把缺少
  artifact 或版本信息的反馈提升为全局决策或 reader matrix `pass`。
- `gui/` 保持 PARKED，`references/` 保持现状，Python 与 Swift 继续按 capability 并存。

## v0.2.6 - 2026-06-25

> ⚠️ 字体工具仍在完善中

### Added

- `tools-font/` 字体工具目录：
  - `font-preview.html`：单文件离线字体预览工具（拖入 .ttf/.otf，多字体对比，手写 SFNT name 表解析器）
  - `font-coverage-viewer.html`：字体覆盖报告查看器（拖入 JSON + EPUB/字体，生僻字清单、内嵌字体对比、修复建议）
  - `coverage-detector/`：EPUB 字体覆盖检测器 Python CLI（uv + fontTools/tinycss2/lxml，8 模块 2,800+ 行）
- `docs/how-to/kindle-font-rendering-deep-dive.md`：Kindle 字体渲染深度参考
- `docs/meta/`：治理索引桶

### Changed

- **仓库整合**：docs/ 从 84 文件 ~29k 行收敛到 47 文件 ~8k 行
- 目录重排：`getting-started/` → `learn/`，`guides/` → `how-to/`
- 删除 `plans/`、`archive/`、`architecture/`（历史留 git）
- `AGENTS.md` 新增架构分工表，优先级从 8 级压到 3 档
- `README.md` 双引擎执行层体现 Python/Swift parity
- `gui/README.md` 顶部 PARKED 标注
- `docs/superpowers/` 加入 .gitignore

### Fixed

- `kindle-pessimistic` 画像修正：`only-non-embedded` 从 fail → risk（系统字体实际可用）
- resolver：`@font-face` 仅 local() 的不标 embedded
- harvester：`<ruby>` 标签中的 Ext B 生僻字采集
- `requirements.txt` 删除（统一走 uv）

## v0.2.3 - 2026-06-12

### Added

- `scripts/epub_lint.py`：通用 SPEC 规则机检（v0 共 10 条规则，覆盖 §1/§2/§3/§5.7/§5.8/§8），可对任意 EPUB 运行；配套回归测试。
- `templates/book-starter/`：最小成书骨架（标题页 + 一章 + nav + NCX，预装 literary-cn preset，自由模式），新增入门页 09 讲解十分钟出书路径。

### Changed

- AGENTS.md 最小验证矩阵纳入 epub-lint 与 epubcheck 运行政策；skills/README 推荐顺序同步。
- demo-scene-expansion-plan 标注旧 `ibooks:specified-fonts` 口径已被 SPEC §8 取代；demo validator 的 Java 探测对 macOS 占位 java 免疫。

## v0.2.2 - 2026-06-12

### Fixed

- 统一 `ibooks:specified-fonts` 条件规则：修正 SPEC §3、手册 §一 / §4.2、demo fonts.css 注释、typography skill、入门教程中残留的「始终保留」旧表述。
- `epub3_oneclick_converter.py` 不再无条件注入 `ibooks:specified-fonts=true`，改为检测 `body-font-locked` 后按需添加，并补充自由 / 锁定两个回归用例。
- SPEC §8 补全嵌入字体分支（未实测，暂按保守口径添加，待 Apple Books 实测后修订，见 reader-matrix 待测条目），明确正文字体模式为全书级决策；demo 演示书的混合页面口径写入 demo README。
- demo SCENE_MATRIX / README、三个 style preset README 同步新规则；`.body-font-locked` 并入宋体选择器组；reader-matrix 将字体模式行为登记为待实测假设。

## v0.2.1 - 2026-06-10

### Changed

- **Body font is now free by default.** `base.css` no longer sets `font-family` on `body`, letting reader font settings take effect. This is the more reader-friendly behavior seen in well-made Chinese EPUBs.
- `ibooks:specified-fonts` is now conditional: only set to `true` when the publisher opts into font locking via `body.body-font-locked`.

### Added

- `.body-font-locked` utility class in all `fonts.css` presets. Add it to `<body>` to lock the text font to the cross-platform system chain and prevent reader font switching.
- Demo page `07-font-family-order.xhtml` now uses `body-font-locked` to demonstrate the locked mode in action.

### Updated

- SPEC §8 documents the free/locked body font distinction.
- EPUB 3 handbook §三, quick-reference cheatsheet §4.1, and the typography-optimizer skill all reflect the new free-by-default behavior.

## v0.2.0 - 2026-06-10

### Highlights

- Add reusable typesetting decision records with validated JSONL add/list/match commands.
- Add a read-only image layout advisor with traceable candidates, Markdown decision templates, and cleanup-pipeline integration.
- Add literary, classical annotated, and academic Chinese style presets with class coverage analysis and redline-safe application.
- Extract shared standard-library EPUB package helpers into `scripts/epub_lib.py`.
- Archive obsolete plans, tighten popup-note rule drift checks, and clarify the newcomer reading path.

### Fixes

- Align text-invariance checks with NFC Unicode normalization.
- Replace duplicate EPUB3 nav manifest items with one generated nav item.
- Reject zip-slip member paths in popup-note EPUB validation.
- Report detector/read failures to stderr instead of silently dropping them.
- Rewrite `srcset` URLs during structure normalization.

### Hardening

- Make one-click EPUB writes atomic and keep NCX updates transactional.
- Clean failed cleanup-pipeline outputs before reruns.
- Keep cleanup-loop state and `epubcheck_ok` report schema consistent.
- Expand skill contract validation to all 15 skills.

### Tests and CI

- Add regression tests for preflight, EPUB3 migration, refinement, popup-note validation, text invariance, detector failures, and `srcset` rewriting.
- Add Markdown lint and demo-books EPUBCheck gates to CI.
- Run every `scripts/test_*.py` test in CI and trigger it for style-preset changes.
- Document local hook vs CI coverage and docs/final quick-reference HTML sync expectations.

## v0.1.0 - 2026-06-01

Initial public release of the EPUB handbook and cleanup toolkit.

### Highlights

- Documents practical EPUB authoring, typography, compatibility, and reading-system behavior across Apple Books, Kindle, Readium, and Readest.
- Provides EPUB preflight, structure normalization, EPUB3 migration, popup-note validation, refinement recommendations, and redline text-invariance checks.
- Adds an optional CSS cleanup tool for repeated stylesheets and system-first CJK font chains.
- Keeps book-specific cleanup artifacts local and documents the boundary between reusable automation and per-title editorial review.
