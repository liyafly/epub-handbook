# Go 重写交接

> 状态快照：**设计完成，W0 未开工** · 2026-08-26 · 工具链 Go 1.27
>
> 规范来源是 [`docs/final/SPEC-go-architecture.md`](../final/SPEC-go-architecture.md)（第一档硬约束，651 行）。
> 本文件是**状态与决策记录**，不是约束。两者冲突时以 SPEC 为准。

| 指标 | 当前值 |
|---|---|
| SPEC 行数 | 651 |
| 不变式条数 | 10 |
| 守卫测试 | 14（11 通过 / 3 bootstrap 跳过） |
| 棘轮待清引用 | 149 处 / 46 个文件 |
| capability 总数 | 22（16 有 Python 实现，6 个纯 AI skill） |

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
执行链路上无下游依赖，可立即删除。

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
Go 侧只实现 B 的行为，A 连同 `test_epub3_migration_harness.py` 一并删除。

---

## 2. 核心设计：架构编码成可执行守卫

这份方案的前提是：**后续修改要能交给能力较低的模型执行且不偏移架构。**

弱模型不会因为读懂了设计说明就不跑偏，只会因为改歪了编译不过、测试红才不跑偏。
所以重心不是架构图，是 `internal/archguard/` 里的 14 个守卫测试。

配套的是 **规则 0：禁止修改 archguard**——守卫红了意味着改动违反了架构，不是守卫写错了。
错误反应是改守卫、加白名单、注释掉断言、打 `t.Skip`。

### 2.1 十条不变式

| | 规则 | 堵住的跑偏 | 状态 |
|---|---|---|---|
| INV-1 | 未修改 entry 必须 `zip.Writer.Copy` 透传 | 退回整包读写，800MB I/O 回来 | 待 `zipfs` |
| INV-2 | `scan/*` 只产 `[]Edit`，禁导出整文档序列化 | DOM 往返静默破坏正文不变 | 已验证 |
| INV-3 | 一次运行只写一次盘 | 中间态落盘，退回旧架构 | 已验证 |
| INV-4 | `caps` 禁 `os/exec` | 外部依赖散落各处 | 已验证 |
| INV-5 | 契约里每条红线必须有注册校验器 | 声明了红线却没实现 | 已验证 |
| INV-6 | 报告必须过 schema | 输出格式漂移 | 待 golden |
| INV-7 | 禁包级可变 `var` | 弱模型最爱的「加个全局传状态」 | 已验证 |
| INV-8 | `skills/` 不得有 `.py` / `.sh` | 执行面重新分散 | 已验证 |
| INV-9 | SKILL.md 只能引用真实存在的 capability id | 改名后文档失效，AI 照着跑就炸 | 已验证 |
| INV-10 | 旧执行面引用棘轮，只删不增 | 迁移中途有人图省事再写一条 | 已验证 |

### 2.2 守卫已验证会开火

空仓库上全绿证明不了任何事。因此造了 9 处故意违规逐条验证——
向上 import、caps 互相 import、cmd 不薄、`os.Create`、`os/exec`、`archive/zip` 外泄、
整文档序列化、包级全局变量、未登记的新包——**全部命中，且定位到文件与行号**。

红线闭包单独验证：只注册 3/6 条时，精确点名缺失的 `anchors` / `cover` / `drm`，
并列出每条由哪些契约声明。`register.go` 白名单也确认生效——注册表放行，
换个文件名同样的变量就被抓。

### 2.3 守卫的真正牙齿

文档里那句「禁止修改 archguard」对弱模型只是软约束——它完全可以删掉断言让测试变绿。
真正的强制来自三处仓库配置，已一并落地：

| 文件 | 作用 |
|---|---|
| `.github/workflows/archguard.yml` | 独立必过 job，故意不与其它测试合并；触碰 archguard 时额外告警 |
| `.github/CODEOWNERS` | 守卫、SPEC、棘轮基线、`contracts/` 均需人类审阅 |
| `.github/pull_request_template.md` | 四项架构自检 + 棘轮进度栏 |

---

## 3. 棘轮：迁移进度的唯一硬指标

旧执行面引用**只许减少，不许新增**。基线在 `tools/parity/legacy-refs.txt`，
每迁移一个 capability 就从中删掉对应行。

```
149 处待清 · 46 个文件 · 目标 0
├── 142 处  SKILL.md 与文档 markdown
└──   7 处  hooks/pre-commit.epub-handbook
```

**棘轮归零前不得删除 `scripts/`。**

### 3.1 曾经漏掉的第三执行面

`scripts/` 的引用不止在 SKILL.md 里。git hook 直接调用 8 个脚本，
而它不是 markdown——早期版本的棘轮扫不到它。现已纳入覆盖。

第三处是 `adapters/python/*.v1.json` 的 provider catalog，它不走棘轮，
由 W4 的 CLI `capabilities` 子命令取代。

> **新增执行面时先问：棘轮扫得到吗？**

棘轮的扫描面：根目录的 `AGENTS.md` / `README.md` / `CONTRIBUTING.md` / `CLAUDE.md`，
加上 `skills/`、`docs/`、`templates/`、`hooks/`。
`CHANGELOG.md` 故意排除（历史记录不该被改写），
`docs/final/SPEC-go-architecture.md` 精确路径豁免（迁移文档必然要点名旧执行面）。

---

## 4. 推进路线

W0 是唯一必须由高能力模型完成的波次。W1 之后每个单元都被任务模板和守卫夹住，
适合派发给较低能力模型逐个推进——改错了会立刻红。

| 波次 | 内容 | 完成判据 |
|---|---|---|
| W0 | `zipfs` + `book` + `editset` + INV-1 行为测试 | 49MB 样本书透传 I/O 实测，验证「800MB → 几 MB」成立 |
| W1 | 六条红线校验器 + `report` | INV-5 闭包成立 |
| W2 | `scan/{xhtml,css,opf}` + audit / lint / content_analyze | parity P1+P2 全绿 |
| W3 | structure_normalize / css_cleanup / migrate_epub3 + 四个 package 操作 | parity 三级全绿 |
| W4 | `pipeline` + `cmd/epub` + 统一返回信封 | 端到端 parity 全绿 |
| W5 | 19 个 SKILL.md + 41 个文档；两个 shell 校验器变子命令 | 棘轮归零，**此时才允许删 `scripts/`** |

### 4.1 终态仓库形态

删除有严格顺序，不可交换。

| 目录 | 体积 | 去留 | 时机与依据 |
|---|---|---|---|
| `swift/` | 303M | 删 | **立即**。执行链路无下游依赖，是构建占盘问题的最大单笔 |
| `gui/` | 68K | 删 | 立即，随 swift。已 PARKED 且只依赖 swift |
| `scripts/` | 1.3M | 删 | W5 棘轮归零后。尚有一处引用就删，即是断链 |
| `adapters/` | 24K | 删 | W4 后。被 `epub capabilities` 取代 |
| `.venv` / `uv.lock` / `pyproject.toml` / `.python-version` | 21M | 删 | scripts 删除后。`tools-font/` 有独立 uv 项目，不受影响 |
| `tools/parity/` | — | 删 | W5 完成后。迁移期脚手架，功成即撤 |
| `docs/` `skills/` | 1.5M | 留 | 文档层。skills 只剩 SKILL.md |
| `contracts/` `templates/` | 860K | 留 | 契约与 demo fixture，第一档硬约束的证据来源 |
| `references/` | 49M | 留 | 样本 EPUB，测试与 parity 的输入 |
| `records/` `archive/` | 152K | 留 | 排版决策记录与第三档参考 |
| `tools-font/coverage-detector/` | 45M | 留 | Python + fonttools，**明确不迁**。任何语言都摆脱不了它 |
| `cmd/` `internal/` `.github/` `hooks/` | — | 留 | Go 实现与 CI（hook 内容改为调 `epub` CLI） |

---

## 5. 两处反直觉结论

### 5.1 报告格式的政策反转过一次

早先结论是「legacy 报告格式必须逐字节保持，不要顺手规整」，理由是下游解析会碎。
但下游只有两类：互相调用的 Python 脚本，和告诉 AI 该跑什么的 SKILL.md——
**二者全部重写后，这个理由不成立了**。

于是 `epub_lint.py` 的数组顶层、`validate_text_invariance.py` 的纯文本输出，
从「要保护的契约」变成「要清掉的包袱」，统一到 SPEC §8.2 的单一信封。

代价是 parity gate 没法再逐字节比对。对策是迁移期加一个 `--legacy-report`
临时脚手架，让比对保持满强度，随 `scripts/` 一起删掉。
**这是唯一被批准的临时脚手架。**

仍然不许动的是**退出码语义**——pre-commit hook 依赖它，信封换了但 0/非 0 的含义必须一致。

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

## 6. 接手第一步

1. **读 SPEC 的 §0–§6**
   [`docs/final/SPEC-go-architecture.md`](../final/SPEC-go-architecture.md)。
   §0 是规则 0 与文档用法，§1–§5 是必须遵守的规则，§6 是照抄式任务模板。
   改文档则读 §7.3 与 §8。

2. **跑一次守卫，确认基线**
   `go test ./internal/archguard/ -v`，当前应为 11 通过 / 3 bootstrap 跳过。
   三处跳过会随对应包的创建自动失效，无法再跳。

3. **删除 `swift/` 与 `gui/`**
   无前置依赖，可立即执行，收回 303MB。这是唯一不需要等任何 gate 的清理动作。

4. **开工 W0**
   建 `zipfs` 时必须同时写 `TestRawPassthrough`——archguard 断言了它的存在，删不掉。
   拿 `references/` 里 49MB 的样本书验证透传 I/O。

### 6.1 尚未处理

- `hooks/pre-commit.epub-handbook` 仍调用 8 个 Python 脚本。已纳入棘轮，
  但改写要等 CLI 有对应子命令，属于 W5 范围。
- `.github/CODEOWNERS` 里用的是 `@liyafly`，从 git remote 推断。
  若 GitHub handle 不同需手工修正。
- `contracts/schemas/v2/envelope.schema.json` 尚未创建，属于 W4 范围。

---

## 7. 本次已交付的文件

| 文件 | 说明 |
|---|---|
| `docs/final/SPEC-go-architecture.md` | 架构 SPEC，651 行，第一档硬约束 |
| `docs/pipeline/go-rewrite-handoff.md` | 本文件 |
| `internal/archguard/` | 11 个文件，14 个守卫测试 |
| `go.mod` | module `github.com/liyafly/epub-handbook`，go 1.27 |
| `tools/parity/legacy-refs.txt` | 棘轮基线，149 条 |
| `.github/workflows/archguard.yml` | 独立必过 CI job |
| `.github/CODEOWNERS` | 人类审阅归属 |
| `.github/pull_request_template.md` | 架构自检清单 |
| `AGENTS.md` | 已加 SPEC 指针与迁移期状态表（**已修改，非新增**） |
