# EPUB 风格预设

风格预设把常见中文书型的排版选择收敛为三个菜单项。每个预设只提供
`Styles/` CSS、OPF stylesheet 声明和 XHTML `<head>` link，不改正文文本或结构。

## 使用前提

预设选择器面向本仓 class 体系，例如 `.font-st`、`.chapter-head`、
`.parallel-float-pair`。先运行 `apply --dry-run` 查看 `coverage`；低于 30% 表示
原书尚未迁入该体系，应先走 cleanup pipeline。低 coverage 不是错误，但直接应用
很可能静默无效。

```sh
python3 scripts/epub_style_preset_tool.py list
python3 scripts/epub_style_preset_tool.py show literary-cn
python3 scripts/epub_style_preset_tool.py apply input.epub \
  --preset literary-cn \
  --output output.epub \
  --dry-run
```

预设遵守 `docs/final/SPEC-实现约束.md` §7 的 CSS 分层和加载顺序。新增预设时：

1. 建立 `<name>/preset.json`、`README.md` 和 `Styles/`。
2. `preset.json` 的 `layers` 按 `fonts.css`、`base.css`、其余场景层排序。
3. 单 CSS 文件不超过 400 行；不得在页面级设置颜色或背景。
4. 运行 `scripts/test_epub_style_preset_tool.py`、红线 gate 和弹注 validator。

> **实测状态**：这些预设的结构合规由自动化 validator 保证；视觉效果尚未经
> `reader-matrix` 实测，应用后应标记为待实测，直到按阅读器实测闭环回写。
