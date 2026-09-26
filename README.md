# epub-handbook

中文 EPUB 3 制作、清洗与兼容性工具集。

如果你只是想做一本书、修一本现成 EPUB，或排查一个具体问题，从下面三条路里选一条即可。

CLI 统一入口是 `epub`（仓库内以 `go run ./cmd/epub` 运行，或 `go build -o epub ./cmd/epub` 后直接使用）。
`epub capabilities --json` 列出全部能力；加 `--id <capability-id>` 可只看该能力的参数、默认值和执行形态。
`epub run ... --json` 返回统一 JSON 信封（capabilities 返回数组，redline 返回文本），退出码 0/1/2/3
（0 成功；1 失败或存在 error 级发现；2 需人工批准；3 用法错误）。

可从 [GitHub Releases](https://github.com/liyafly/epub-handbook/releases/latest) 下载 0.4.1 CLI：Linux amd64、Windows amd64、macOS arm64 和 macOS amd64。发布附件含 SHA256 校验和与安装说明；二进制内嵌 contracts、schemas 和 style presets，可在仓库目录之外运行。字体覆盖和书籍构建中的字体子集化由可选 provider 提供，安装方式见对应版本的发行说明。

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
# 改 OEBPS/package.opf 和 OEBPS/Text/01-chapter.xhtml
(cd ~/my-book && sh build.sh)
# 保持在本仓根目录运行 CLI；输入应选定单个构建产物
go run ./cmd/epub run epub.package.nav.audit --input ~/my-book/dist/*.epub --json
```

详细步骤、手写最小 EPUB 的原理路径都在 [做一本书](docs/learn/做一本书.md)。

### 修一本现成 EPUB

单项能力仍可逐步预览、审查并执行：

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

批量检查可用 `epub clean`。默认只做审计并生成逐书计划，不自动迁移或套用样式；结构步骤用 `--steps` 明确选择。若选择 `typography`，还必须明确 `--preset` 和 `--scope all` 或一个/多个 EPUB 内 spine XHTML 路径。只有步骤、末次审计和全项红线均通过时，`--approve` 才写出候选；失败候选只有再加 `--retain-review-candidate` 才会以 `.review-only.epub` 保存。机器调用可加 `--json` 获取批次信封。

```sh
epub clean "$BOOKS" --out "$W/clean-preview" --json
epub clean "$BOOKS" --out "$W/clean-candidates" --steps normalize,migrate --approve
epub clean "$BOOKS" --out "$W/typography-preview" --steps typography \
  --preset literary-cn --scope all
```

### 查一个具体问题

先去 [新手必读](docs/learn/) 按症状直达。常见问题集中在
[FAQ](docs/learn/07-faq.md)，核心术语集中在 [术语表](docs/learn/glossary.md)。

---

## AI / 专业维护者入口

先读 [AGENTS.md](AGENTS.md)。它是 AI 协作约束的唯一维护源，也是专业层的总路由。

想找版式示例，先搜索真实 demo；想试改已有 EPUB，先做局部候选：

```sh
go run ./cmd/epub run epub.style.demo.maintain --json catalog=true query=chapter
go run ./cmd/epub capabilities --id epub.typography.optimize --json
go run ./cmd/epub run epub.typography.optimize --input before.epub --output sample.epub --dry-run --json 'scope_paths=["OEBPS/Text/chapter.xhtml"]'
```

`scope_paths` 应替换成实际 spine XHTML 路径。局部模式追加独立 CSS，保留原样式与其他章节；
不传此参数是整书预设应用，保留已有字体层及自由／锁定模式；模式冲突会拒绝处理，
详见[预设说明](templates/style-presets/README.md)。发现示例、预览候选和真实阅读器视觉验收是三个不同阶段。

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
provider 适配层已按迁移计划删除。字体能力由 `tools-font/coverage-detector/` 下独立的
Python + FontTools 项目提供，不打包进发行包：需要本机安装 `uv`（在该目录执行一次 `uv sync`），
缺少 `uv` 时 `epub.font.coverage.analyze` 会以带明确提示的 failed 结果显式失败；
其余 capability 只需要 Go 二进制。

EPUB 输入默认限制为：压缩文件 512 MiB、100,000 个 ZIP 条目、单条目解压后
256 MiB、声明解压总量 1 GiB、条目路径 4096 字节。实际解压流也检查单条目限额；
每个打开的 book 缓存原文与修改内容合计最多 512 MiB（不是整个进程的峰值内存承诺）。
替换封面限 64 MiB，redline 路径映射 JSON 限 16 MiB；这些输入须是普通文件。
超限会明确报错，CLI 暂无放宽限额参数。

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
