# 维护者文档索引

普通用户看 [使用指南](guide/README.md)。AI 与维护者先读根目录 `AGENTS.md`，本页只做索引和 CLI 事实。

## CLI 事实

- 唯一入口是 [`cmd/epub`](../cmd/epub)。仓库内用 `go run ./cmd/epub`，或 `go build -o epub ./cmd/epub`。架构规则由 `internal/archguard/` 的守卫测试强制。旧的 Python 主程序与 Swift/GUI 执行层已删除；`templates/cleanup-demo-books/build_demo_epubs.py` 只生成自造清洗样本，不属于用户执行面。
- `epub capabilities --json` 列出全部能力；加 `--id <capability-id>` 只看该能力的参数、默认值和执行形态。
- `epub run ... --json` 与 `epub redline --json` 返回统一 JSON 信封；`epub capabilities --json` 返回能力数组。
- 退出码：0 成功；1 失败或有 error 级发现；2 需人工批准；3 用法错误。`epub book` 不返回 2。
- 发行二进制内嵌 contracts、schemas 和预设，可在仓库外运行。`epub.style.demo.maintain catalog=true` 可离线用内嵌目录；`--input <book.epub>` 模式需要仓库 checkout，或设 `EPUB_HANDBOOK_ROOT` 指向仓库根。发行说明模板见 [release/](release/README.md)。
- 字体 provider `epub-font`（[`tools-font/`](../tools-font/README.md)）独立安装，不进发行包，由 `internal/extern` 经 PATH 调用。`epub.font.coverage.analyze` 用 `epub-font coverage`，`epub.font.subset` 用 `epub-font subset`。无字体的书不需要；含字体的构建缺 provider 或检查失败即失败，保留原有成品。
- 找版式示例：`epub run epub.style.demo.maintain catalog=true query=chapter`；`collection=presets` 搜预设。发现、候选生成和真实阅读器验收是三个阶段。

### 输入限额

EPUB 输入默认上限：压缩文件 512 MiB、100,000 个 ZIP 条目、单条目解压后 256 MiB、声明解压总量 1 GiB、条目路径 4096 字节。实际解压流也检查单条目限额。每个打开的 book 缓存原文与修改内容合计最多 512 MiB（不是整个进程的峰值内存承诺）。替换封面限 64 MiB，redline 路径映射 JSON 限 16 MiB；这些输入须是普通文件。超限明确报错，暂无放宽参数。

### 书级工作流

`epub book new|import|preview|build|fonts|clean|accept-dist` 是主路径，跨平台，自动识别新旧布局，见 [书级工作区](pipeline/book-workspace.md)。`templates/book-starter/` 下的 sh 脚本是旧路径，只为既有书保留。

## 工程契约（`final/`）

- [SPEC-实现约束](final/SPEC-实现约束.md)：排版与清洗硬规则
- [SPEC-go-architecture](final/SPEC-go-architecture.md)：Go 架构硬约束；动 Go、CLI、skills 执行面或旧实现时必读
- [SPEC-go-modern-guidelines](final/SPEC-go-modern-guidelines.md)：Go 1.27 编程指南；服从架构、EPUB 安全、wire schema 与无损编辑约束
- [字体别名命名规范](final/字体别名命名规范.md)
- [EPUB 3 终极实践手册](final/EPUB%203%20终极实践手册.md)、[属性速查表](final/EPUB%203%20HTML%20CSS%20属性速查表.md)
- [reader-matrix.yaml](final/reader-matrix.yaml)：阅读器实测矩阵

## Go 维护交接

- [go-rewrite-handoff](pipeline/go-rewrite-handoff.md)：当前状态、已裁决事项与验证入口

## 场景指南（`how-to/`）

- [结构与兼容](how-to/epub-structure-and-compat.md)：ZIP、OPF、导航、CSS 分层、字体链、EPUB 2/3 差异
- [手写最小 EPUB](how-to/handwrite-minimal-epub.md)：从 demo 构建到手写一本只有一章的书
- [kindle-font-rendering-deep-dive](how-to/kindle-font-rendering-deep-dive.md)
- [english-fiction-layout](how-to/english-fiction-layout.md)
- [classical-modern-layout](how-to/classical-modern-layout.md)
- [chapter-head-image](how-to/chapter-head-image.md)
- [figure-layout](how-to/figure-layout.md)
- [shape-wrap](how-to/shape-wrap.md)
- [mathml-equation-layout](how-to/mathml-equation-layout.md)
- [anthology-navigation](how-to/anthology-navigation.md)
- [note-box-border-styles](how-to/note-box-border-styles.md)
- [epub2-popup-note-compatibility](how-to/epub2-popup-note-compatibility.md)
- [duokan-footnote-fallback-fix](how-to/duokan-footnote-fallback-fix.md)

## 流程与工具（`pipeline/`）

- [book-workspace](pipeline/book-workspace.md)：书级工作区
- [cleanup-flow](pipeline/cleanup-flow.md)：已有 EPUB 清洗 runbook
- [cleanup-case-study](pipeline/cleanup-case-study.md)：自造样本的清洗案例
- [cleanup-patterns](pipeline/cleanup-patterns.md)
- [epub3-migration](pipeline/epub3-migration.md)
- [asset-optimization](pipeline/asset-optimization.md)
- [package-operations](pipeline/package-operations.md)
- [epub-diff-review](pipeline/epub-diff-review.md)
- [reference-font-role-patterns](pipeline/reference-font-role-patterns.md)
- [reference-reader-extensions](pipeline/reference-reader-extensions.md)：多看长屏封面与 Reeden 扩展
- [reader-adaptation-lessons-2026-10](pipeline/reader-adaptation-lessons-2026-10.md)：Reeden、Readest 与 Kindle 适配复盘
- [EPUB排版问题复盘与验收清单](pipeline/EPUB排版问题复盘与验收清单.md)

## 模板、技能与历史

- [skills/](../skills/README.md)：AI 技能索引；[learn/04-skills.md](learn/04-skills.md) 是面向使用者的速查
- [templates/](../templates/README.md)：demo、starter、配方与预设
- [cleanup-demo-books](../templates/cleanup-demo-books/)：自造清洗与 diff 样本
- 第三方来源：[THIRD_PARTY.md](../THIRD_PARTY.md) 与 [references/](../references/README.md)
- 已完成的设计、计划、实验和早期推导：[archive/](../archive/README.md)

## 新文档放哪

| 内容 | 位置 |
| --- | --- |
| 普通用户怎么用（中英双语） | `docs/guide/` |
| 对外硬约束，违反等于事故 | `docs/final/` |
| 某类书或某个场景的实操 | `docs/how-to/` |
| 流程、工具、模式、迁移交接 | `docs/pipeline/` |
| AI 与维护规则 | `AGENTS.md` |
| 已完成的计划、review、实验 | `archive/` 或 git 历史 |

- `docs/final/` 新增内容必须能用 `AGENTS.md` 的规范来源优先级解释。
- `docs/how-to/` 不放计划、流水线或架构。
- 历史记录不反向覆盖当前约束；要重新启用，先在当前任务中验证，再写进对应活跃文档。
