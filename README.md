# epub-handbook

中文 EPUB 3 制作与 AI 协作工具集。

适合的人：制作中文 EPUB 3 的工程师、想用 AI 帮忙清洗已有 epub 的编辑、想给团队约定 epub 制作规范的 maintainer。

## 我要做什么？

| 我的场景 | 入口 |
| --- | --- |
| 从零做一本新书 | [docs/getting-started/](docs/getting-started/) -> `templates/epub-style-demo/` |
| 改造一本现成 EPUB | [docs/pipeline/](docs/pipeline/) -> `scripts/validate_text_invariance.py` |
| 想看改前 / 改后差异 | [tools/epub-diff/index.html](tools/epub-diff/index.html) -> 浏览器打开 |
| 想看制作规范 | [docs/final/](docs/final/) |
| 给 AI 接入 | [skills/](skills/) + `agents/openai.yaml` |

## 5 分钟跑通

```sh
bash templates/epub-style-demo/build.sh
bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<新建的 .epub>
```

详细教程见 [docs/getting-started/01-first-epub.md](docs/getting-started/01-first-epub.md)。

## 看改前 / 改后差异

把 `tools/epub-diff/index.html` 用浏览器打开（双击或拖入 Chrome / Safari / Firefox），选两个 epub 文件，点 Compare。详细见 [docs/pipeline/diff-tool.md](docs/pipeline/diff-tool.md)。

## 这个仓库不是什么

- 不是初级排版课。
- 不是封闭格式（mobi/AZW3）的制作工具。
- 不是 epub.js 阅读器。
- 不是 Kindle 自费出版的运营指南。
- 不是用来检验 epub 在某阅读器里「显示效果」的工具（diff 工具只比文件，不模拟渲染）。

## 整体目录

详见 [docs/README.md](docs/README.md)。

## 协作

阅读 [CLAUDE.md](CLAUDE.md) 了解 AI 协作约定。本仓所有约束变更都走 demo -> reader-matrix -> SPEC -> 手册 -> 速查表 -> skills 的实测闭环。

贡献流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可

代码部分 MIT；文档与样本许可参见 [THIRD_PARTY.md](THIRD_PARTY.md)。
