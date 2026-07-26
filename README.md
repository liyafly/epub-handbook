# epub-handbook

中文 EPUB 3 制作、清洗与兼容性工具集。

如果你只是想做一本书、修一本现成 EPUB，或排查一个具体问题，从下面三条路里选一条即可。

## 我想……

| 目标 | 最短入口 |
| --- | --- |
| 做一本新书 | [做一本书](docs/learn/做一本书.md) |
| 修 / 清洗一本现成 EPUB | [运行一键清洗](#修一本现成-epub) |
| 查目录、弹注、字体、图片等问题 | [新手必读的症状直达表](docs/learn/README.md#3-带着问题直接查) |

### 做一本书

直接复制现成骨架：

```sh
cp -r templates/book-starter ~/my-book
cd ~/my-book
# 改 OEBPS/package.opf 和 OEBPS/Text/01-chapter.xhtml
sh build.sh
python3 <仓库路径>/scripts/epub_lint.py dist/*.epub
```

详细步骤、手写最小 EPUB 的原理路径都在 [做一本书](docs/learn/做一本书.md)。

### 修一本现成 EPUB

一条命令会保留 before 基线、检查风险，并把结果和报告写进独立工作目录：

```sh
python3 scripts/epub_cleanup_pipeline.py /path/to/input.epub --work-dir work/book-a
```

不要在唯一原件上直接修改。需要人工批准的结构规范化、正文红线和 diff review
仍会保留；完整说明见 [清洗流程](docs/pipeline/cleanup-flow.md)。

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
| Python / AI provider 与校验基线 | [scripts/README.md](scripts/README.md) |
| 原生执行核心 | [swift/](swift/) |
| 机器契约与适配表面 | `contracts/`、`adapters/` |
| 阅读器最小实测样本 | [templates/epub-style-demo/](templates/epub-style-demo/) |
| 历史设计、实验与推导 | [archive/](archive/) 与 git 历史 |

Python 和 Swift 按 capability 并存：Python 是 AI agent、CLI 与验证基线的首要
provider；Swift 是 native 执行核心。`gui/` 当前 PARKED，不投入、不作依赖。
字体和图片转换工具位于 `tools-font/` 及其独立 Python 项目。

完整文档索引见 [docs/README.md](docs/README.md)，Python 脚本按受众和职责的索引见
[scripts/README.md](scripts/README.md)。

## 维护验证

按改动类型执行 [AGENTS.md](AGENTS.md) 的最小验证矩阵。文档与入口变更至少运行：

```sh
python3 scripts/validate_docs_consistency.py
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
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
