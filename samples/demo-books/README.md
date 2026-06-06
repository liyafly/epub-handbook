# 自造 EPUB 演示样本

本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和 外部 diff 工具（Calibre / VS Code，见 [EPUB diff review](../../docs/pipeline/epub-diff-review.md)），不依赖公版书来源。

`dist/` 是本地生成目录，默认不入 Git。用户下载仓库后可以运行构建脚本生成这些 EPUB，用来查看、验证和做 diff 参考。

## 样本

| slug | 用途 | before / after |
| --- | --- | --- |
| `city-field-notes` | 样式分层、脚注、表格、代码、资源改动 | 文本不变，红线应通过 |
| `paper-garden` | 诗段、Ruby、blockquote、竖排增强 | 文本不变，红线应通过 |
| `loop-auto-fix` | 多轮 loop 正向演示：章节根元素故意漏语言属性 | loop 应自动应用 `add-xml-lang`，正文不变 |
| `redline-trap` | 故意改写正文的反例 | 红线应失败 |

## 生成

```sh
bash samples/demo-books/build.sh
```

输出在 `samples/demo-books/dist/`。如果脚本逻辑变化，需要重新生成并本地验证这些 demo EPUB。

`dist/` 里的 `.epub` 和 `manifest.json` 都是可再生文件；不要提交生成产物，只提交 `src/`、`build.sh`、测试或说明文档的变化。

## 验证

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/city-field-notes-before.epub \
  samples/demo-books/dist/city-field-notes-after-clean.epub \
  --check all

python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/paper-garden-before.epub \
  samples/demo-books/dist/paper-garden-after-clean.epub \
  --check all
```

反例必须失败：

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/redline-trap-before.epub \
  samples/demo-books/dist/redline-trap-after-text-changed.epub \
  --check all
```

## Diff 演示

按 [EPUB diff review](../../docs/pipeline/epub-diff-review.md) 用 Calibre Editor 或 VS Code 选 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。

## 多轮 loop 正向演示

```sh
bash samples/demo-books/build.sh
python3 scripts/epub_cleanup_loop.py \
  samples/demo-books/dist/loop-auto-fix-before.epub \
  --work-dir work/loop-auto-fix \
  --format json
```

报告中应至少有一项 `add-xml-lang` 自动动作；最终产物为 `work/loop-auto-fix/after/cleaned.epub`。
