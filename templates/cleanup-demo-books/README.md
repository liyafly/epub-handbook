# 自造 EPUB 演示样本

本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和 外部 diff 工具（Calibre / VS Code，见 [EPUB diff review](../../docs/pipeline/epub-diff-review.md)），不依赖公版书来源。

`dist/` 是本地生成目录，默认不入 Git。用户下载仓库后可以运行构建脚本生成这些 EPUB，用来查看、验证和做 diff 参考。

## 样本

| slug | 用途 | before / after |
| --- | --- | --- |
| `city-field-notes` | 样式分层、脚注、表格、代码、资源改动 | 文本不变，红线应通过 |
| `paper-garden` | 诗段、Ruby、blockquote、竖排增强 | 文本不变，红线应通过 |
| `loop-auto-fix` | 多轮 loop 正向演示：章节根元素故意漏语言属性 | 审计应检出 `missing-html-lang`（auto_fixable），正文不变 |
| `redline-trap` | 故意改写正文的反例 | 红线应失败 |

## 生成

```sh
bash templates/cleanup-demo-books/build.sh
```

输出在 `templates/cleanup-demo-books/dist/`。如果脚本逻辑变化，需要重新生成并本地验证这些 demo EPUB。

`dist/` 里的 `.epub` 和 `manifest.json` 都是可再生文件；不要提交生成产物，只提交 `src/`、`build.sh`、测试或说明文档的变化。

## 验证

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/city-field-notes-before.epub \
  templates/cleanup-demo-books/dist/city-field-notes-after-clean.epub

epub redline --check all \
  templates/cleanup-demo-books/dist/paper-garden-before.epub \
  templates/cleanup-demo-books/dist/paper-garden-after-clean.epub
```

反例必须失败：

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/redline-trap-before.epub \
  templates/cleanup-demo-books/dist/redline-trap-after-text-changed.epub
```

## Diff 演示

按 [EPUB diff review](../../docs/pipeline/epub-diff-review.md) 用 Calibre Editor 或 VS Code 选 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。

## 多轮 loop 正向演示

```sh
bash templates/cleanup-demo-books/build.sh
epub run epub.package.nav.audit \
  --input templates/cleanup-demo-books/dist/loop-auto-fix-before.epub \
  --json
```

Go CLI 没有多轮自动 loop 命令：报告中应出现 `missing-html-lang` finding（`auto_fixable: true`）。修复按 [清洗流水线](../../docs/pipeline/cleanup-flow.md) 的固定顺序逐能力执行，最后用 `epub redline --check all` 验证正文不变。
