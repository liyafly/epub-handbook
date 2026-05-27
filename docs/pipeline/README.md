# 批处理流水线

把已有 epub 批量处理（清洗、改造、对比、回写）的工作流文档。

## 工作流概览

```text
intake -> analyze -> cleanup -> diff -> accept
  |         |          |          |        |
  |         |          |          |        reader-matrix + 用户确认
  |         |          |          tools/epub-diff/index.html
  |         |          validate_text_invariance.py
  |         epub-layout-auditor 推荐 skills
  scripts/epub_ai_harness.py --mode cleanup
```

## 每一步对应的工具

| 步骤 | 工具 | 文档 |
| --- | --- | --- |
| intake | `scripts/epub_ai_harness.py --mode cleanup <epub>` | [epub-cleanup-flow.md](../guides/epub-cleanup-flow.md) |
| analyze | `epub-layout-auditor` skill | `skills/epub-layout-auditor/SKILL.md` |
| cleanup | 各专项 skill（按 §10 红/黄/绿规则） | [SPEC §10](../final/SPEC-实现约束.md) |
| 红线 gate（CI / 自动化） | `scripts/validate_text_invariance.py` | [SPEC §10.5](../final/SPEC-实现约束.md) |
| diff（人工 review） | `tools/epub-diff/index.html` 浏览器打开 | [epub-diff-tool.md](../guides/epub-diff-tool.md) |
| accept | reader-matrix 回写 + 用户确认 | `docs/final/reader-matrix.yaml` |

## 我现在该看什么

- 第一次接触清洗流水线：[../guides/epub-cleanup-flow.md](../guides/epub-cleanup-flow.md)
- AI 改动边界：[../final/SPEC-实现约束.md §10](../final/SPEC-实现约束.md)
- Diff 工具使用：[../guides/epub-diff-tool.md](../guides/epub-diff-tool.md)
- 自造案例：[../getting-started/05-case-study.md](../getting-started/05-case-study.md)

## 决策记录

按 [decisions.md](decisions.md) 记录每个 Stage 落地前回答的必答题。
