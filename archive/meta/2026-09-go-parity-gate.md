# Go parity gate 与迁移映射（历史归档）

> 本文保留 `SPEC-go-architecture.md` 原 §5.2–§5.3 与 §7 的迁移期材料。D5 完成后，新能力验收以现行 SPEC §5.2 的 golden 测试与全项 redline 为准。

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
正式输出走 §8.2 的新信封。

- 好处：P2 保持**逐字节**强度，不降级成"语义等价"这种模糊判据
- 移除触发条件：对应 Python 脚本删除时，同步删掉该 capability 的 `--legacy-report`
- 这是**唯一被批准的临时脚手架**。不要用同样理由再引入第二个
- **状态（2026-09-04）：已拆除。** Python 脚本删除后，`--legacy-report` / `legacy_report=true` /
  `facts.legacyReport` 全部移除；`epub redline --path-map` 直接读取 `--json` 信封中的
  `*.mappings` facts，不再依赖 legacy 报告形状。
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

## §7 Python → Go 迁移映射

### 7.1 迁移单元总览

`contracts/capabilities/v1/` 共 **22 个** capability。迁移完成后的当前执行状态如下；
旧 adapter catalog 覆盖的 3 条只是历史上 16 个迁移能力的子集，不再作为与 A/B
并列、可相加的第三类：

| 类别 | 数量 | 处理方式 |
|---|---|---|
| A. 已迁移并 ready 的 Go 能力 | 16 | 已完成 §6.1 模板与 §5.2 parity gate |
| B. 新增的最小 source intake planner | 1 | `epub.source.intake`，只读盘点，不含 PDF/OCR/转码 |
| C. 纯 AI / 人工 skill，无专属实现 | 5 | 不建 `caps/` 包；由 ready 的通用校验能力提供机械检查 |

C 类（不建 `caps/` 包）：`epub.kindle.compatibility.check`、`epub.literary.structure.format`、
`epub.notes.legacy-fallback`、`epub.typography.english.optimize`、
`epub.vertical.ruby.optimize`。

> 这 5 个没有专属实现，但**不等于没有执行需求**：所需的通用机械校验已经由
> `epub.style.demo.maintain` 与 `epub.notes.popup.normalize` 两个 ready Go 能力承接；
> 具体判断仍按对应 SKILL.md 的人工/AI 流程执行。

> 历史背景：旧 `adapters/python/provider-catalog.v1.json` 只登记过 3 条，而
> `public-entrypoints.v1.json` 覆盖 16 条；这是 Go 重写必须消除的路由分裂。
> 当前 17 个 ready 能力已经统一走 Go pipeline 与 v2 envelope，旧 adapters 执行面已删除。

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
| `epub_ai_harness.py` / `epub_package_tool.py` / 旧 EPUB3 迁移门面 | 17/68/19 | **删除** | 三个都是 "Backward-compatible CLI façade"，零逻辑 |
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
> | pipeline 调用 | 无 | `epub_cleanup_pipeline.py` → EPUB3 迁移门面 |
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
> 扫描与 `b.Apply(edits)` 都在内存中完成，后续阶段和红线检查同一份候选；
> 不能跳过内存应用，否则多阶段改名、迁移与局部样式的预览会失真。
> `--dry-run` 是**全局 flag，不是某个 capability 的特性**：`pipeline`
> 统一禁止最终落盘，报告保留完整变更摘要与计划路径；多产物 capability
> 同样不得在 dry-run 创建目录或写出产物。预览失败不生成默认应用建议。

### 7.3 报告格式：从「逐字节保持」改为「统一信封」

> **本节的结论在 2026-08-26 反转过一次，理解这个反转很重要。**

早先的结论是「legacy 报告格式必须逐字节保持，不要顺手规整」，理由是下游解析会碎。
但下游只有两类：**互相调用的 Python 脚本**，和**告诉 AI 该跑什么的 SKILL.md**。
既然二者全部重写，这个理由就不成立了 —— 那两个「陷阱」不是要保护的契约，
是要清掉的历史包袱。

**现行结论：Go 版统一到 §8.2 的单一信封。**

被统一掉的两个畸形格式：

| 旧形态 | 出处 | 归到信封的哪里 |
|---|---|---|
| JSON 顶层是**数组** | `epub_lint.py` | `findings[]` |
| **纯文本行 + 退出码**，无 JSON | `validate_text_invariance.py` 与 7 个 `validate_*.py` | `findings[]` + `status` + 退出码 |

**代价与对策**：这样一来 parity gate 的 P2 就没法再逐字节比对了。
对策见 §5.2 —— 迁移期加一个 `--legacy-report` 临时脚手架，让 P2 保持满强度，
迁移结束时随 `scripts/` 一起删掉。（状态：2026-09-04 已删除。）

**仍然不许动的**：退出码语义。`epub_text_gate.py` 之外还有 pre-commit hook
依赖退出码，信封换了但 0/非 0 的含义必须一致（细则见 §8.5）。

### 7.4 迁移波次

| 波次 | 内容 | 完成判据 |
|---|---|---|
| W0 | `zipfs` + `book` + `editset` + `archguard` 全绿 | INV-1 行为测试通过；49MB 样本书透传 I/O 实测 |
| W1 | `redline` 六条 + `report` | INV-5 闭包成立；陷阱 2 的纯文本格式逐字节一致 |
| W2 | `scan/{xhtml,css,opf}` + 3 个只读 capability（audit / lint / content_analyze） | parity P1+P2 全绿 |
| W3 | 写入型 capability（structure_normalize / css_cleanup / migrate_epub3 / 4 个 package 操作） | parity 三级全绿 |
| W4 | `pipeline` 编排 + `cmd/epub` + §8.2 信封 | 端到端 parity 全绿 |
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
