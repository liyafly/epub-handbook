# 自造 EPUB 演示样本

本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和 `tools/epub-diff/`，不依赖公版书来源。

`dist/` 下的自造 `.epub` 和 `manifest.json` 可以入 Git，方便用户不构建也能直接打开 diff 工具演示。

## 样本

| slug | 用途 | before / after |
| --- | --- | --- |
| `city-field-notes` | 样式分层、脚注、表格、代码、资源改动 | 文本不变，红线应通过 |
| `paper-garden` | 诗段、Ruby、blockquote、竖排增强 | 文本不变，红线应通过 |
| `redline-trap` | 故意改写正文的反例 | 红线应失败 |

## 生成

```sh
bash samples/demo-books/build.sh
```

输出在 `samples/demo-books/dist/`。如果脚本逻辑变化，需要重新生成并提交这些 demo EPUB。

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

## Diff 工具演示

打开 `tools/epub-diff/index.html`，选择任意 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。
