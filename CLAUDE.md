# CLAUDE.md

本文件给 AI 协作代理（Claude Code/Codex 等）提供仓库工作约束。

## 修改优先级

1. `docs/final/`：对外约束层，任何改动都应视为规范变更。
2. `templates/`（若存在）：机器消费源，优先级高于文档中的示例代码块。
3. `skills/*/SKILL.md`：自动化行为契约，修改需保持向后兼容。
4. `docs/source/`、`docs/experiments/`：推导与实验区，可自由补充但不应反向覆盖约束层。

## 关键约束

- 若你修改了 `docs/final/` 的硬规则，请同步检查：
  - `docs/final/EPUB 3 终极实践手册.md`
  - `docs/final/EPUB 3 HTML CSS 属性速查表.md`
  - `docs/final/SPEC-实现约束.md`
- 涉及弹注、字体、A-lite、竖排等规范条目时，优先写入 `SPEC-实现约束.md`，再在手册里解释。
- `skills/*/SKILL.md` 的 frontmatter 字段名不要改（可补字段值，不要随意删改 key）。
- 任何新增第三方 EPUB 参考样本，必须同步更新 `THIRD_PARTY.md`（来源/作者/许可/链接）。

## 建议流程

1. 先判断是「实现约束变更」还是「说明增强」。
2. 实现约束变更：先改 `SPEC-实现约束.md`，再改终极手册/速查表。
3. 说明增强：仅改 `docs/source/` 或 `docs/experiments/` 也可。
4. 变更后执行最小自检（链接、标题、路径、术语一致性）。
