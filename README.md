# epub-handbook

中文 EPUB 3 制作、清洗与兼容性工具集。

如果你只是想做一本书、修一本现成 EPUB，或排查一个具体问题，从下面三条路里选一条即可。

CLI 统一入口是 `epub`（仓库内以 `go run ./cmd/epub` 运行，或 `go build -o epub ./cmd/epub` 后直接使用）。
`epub capabilities --json` 列出全部能力；所有命令返回统一 JSON 信封，退出码 0/1/2/3
（0 成功；1 失败或存在 error 级发现；2 需人工批准；3 用法错误）。

## 我想……

| 目标 | 最短入口 |
| --- | --- |
| 做一本新书 | [做一本书](docs/learn/做一本书.md) |
| 修 / 清洗一本现成 EPUB | [运行清洗流程](#修一本现成-epub) |
| 查目录、弹注、字体、图片等问题 | [新手必读的症状直达表](docs/learn/README.md#3-带着问题直接查) |

### 做一本书

直接复制现成骨架：

```sh
cp -r templates/book-starter ~/my-book
cd ~/my-book
# 改 OEBPS/package.opf 和 OEBPS/Text/01-chapter.xhtml
sh build.sh
go run ./cmd/epub run epub.package.nav.audit --input dist/*.epub --json
```

详细步骤、手写最小 EPUB 的原理路径都在 [做一本书](docs/learn/做一本书.md)。

### 修一本现成 EPUB

清洗是按序的单能力执行，每步之间保留人工 review（没有一键流水线）：

```sh
go run ./cmd/epub run epub.package.nav.audit --input input.epub --json
go run ./cmd/epub run epub.structure.normalize --input input.epub --output normalized.epub --dry-run --json
# 人工 review dry-run 报告后去掉 --dry-run 实跑
go run ./cmd/epub run epub.package.migrate.epub3 --input normalized.epub --output migrated.epub --dry-run --json
# 同样 review 后实跑，再按需 css.layering.optimize / typography.optimize
go run ./cmd/epub redline --check all input.epub migrated.epub
```

不要在唯一原件上直接修改；需要人工批准的结构规范化、正文红线和 diff review
仍会保留。完整说明见 [清洗流程](docs/pipeline/cleanup-flow.md)。

### 查一个具体问题

先去 [新手必读](docs/learn/) 按症状直达。常见问题集中在
[FAQ](docs/learn/07-faq.md)，核心术语集中在 [术语表](docs/learn/glossary.md)。

---

## AI / 专业维护者入口

先读 [AGENTS.md](AGENTS.md)。它是 AI 协作约束的唯一维护源，也是专业层的总路由。

| 能力 | 位置 |
| --- | --- |
| 对外硬约束与阅读器证据 | [docs/final/](docs/final/) |
| 已有 EPUB 流水线 | [docs/pipeline/](docs/pipeline/) |
| 场景化排版指南 | [docs/how-to/](docs/how-to/) |
| AI 能力契约与反向查表 | [docs/learn/04-skills.md](docs/learn/04-skills.md) |
| Go CLI 实现与架构守卫 | [cmd/epub](cmd/epub/) 与 [internal/](internal/) |
| 机器契约 | [contracts/](contracts/) |
| 阅读器最小实测样本 | [templates/epub-style-demo/](templates/epub-style-demo/) |
| 历史设计、实验与推导 | [archive/](archive/) 与 git 历史 |

架构是面向 Windows、macOS、Linux 的 Go 单一 CLI（`cmd/epub` + `internal/`），
架构规则由 `internal/archguard/` 的守卫测试强制；旧的 Python 脚本、Swift/GUI 实现和
provider 适配层已按迁移计划删除。字体能力继续使用随发行包交付的独立 provider，
用户不需要自行安装 Python 工具链。

完整文档索引见 [docs/README.md](docs/README.md)。

每本书使用 `work-epub/<book>/` 独立工作区，并在该目录自行初始化 Git；
手册主仓库仍忽略整个 `work-epub/`，不把书级仓库当成 submodule。统一目录和过程文件约定见
[一书一 Git 工作区](docs/pipeline/book-workspace.md)。

## 维护验证

按改动类型执行 [AGENTS.md](AGENTS.md) 的最小验证矩阵。Go 代码与入口变更至少运行：

```sh
go test ./...
go test ./internal/archguard/
git diff --check
```

阅读器兼容性结论必须先有 demo、artifact、阅读器名称和版本及实测现象，再写入
`docs/final/reader-matrix.yaml`；不能只根据手册推断。

## 范围

本仓聚焦可重排 EPUB 3：

- 不制作 mobi / AZW3 等封闭格式；
- 不实现 epub.js 阅读器；
- 不替代 Kindle 自费出版运营工具；
- 不模拟阅读器渲染，视觉结论来自真实阅读器实测。

## 协作与许可

贡献流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。第三方材料的来源、作者、许可和链接
记录在 [THIRD_PARTY.md](THIRD_PARTY.md)。

代码部分使用 MIT 许可；文档与样本许可见 `THIRD_PARTY.md`。
