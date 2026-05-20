# CLAUDE.md

本文件给 AI 协作代理（Claude Code/Codex 等）提供仓库工作约束。

## 修改优先级

1. `templates/`：可运行样式样本和机器消费源。遇到阅读器显示、打包、转换兼容性问题时，先补 demo fixture 并实测。
2. `docs/final/`：对外约束层，任何改动都应视为规范变更；必须由 demo fixture 或明确实测结果支撑。
3. `skills/*/SKILL.md`：自动化行为契约，修改需保持向后兼容。
4. `docs/guides/`：维护说明和工作流建议，不承载下游架构。
5. `docs/source/`、`docs/experiments/`：推导与实验区，可自由补充但不应反向覆盖约束层。

## 关键约束

- 若你修改了 `docs/final/` 的硬规则，请同步检查：
  - `docs/final/EPUB 3 终极实践手册.md`
  - `docs/final/EPUB 3 HTML CSS 属性速查表.md`
  - `docs/final/SPEC-实现约束.md`
- 涉及弹注、字体、A-lite、竖排等规范条目时，优先写入 `SPEC-实现约束.md`，再在手册里解释。
- `skills/*/SKILL.md` 的 frontmatter 字段名不要改（可补字段值，不要随意删改 key）。
- `templates/` 下的样本应能独立打包，生成产物放在模板自己的 `dist/` 目录。
- Kindle/Apple Books/Thorium/KOReader 等阅读器兼容性问题，不允许只靠手册推断修改；必须先更新或新增 demo EPUB 场景，打包验证，再回写 `docs/final/reader-matrix.yaml` 和最终文档。
- demo EPUB 必须覆盖结构多样性：普通正文、中英混排、大字号标题、图片/封面、表格、代码、标准弹注、legacy fallback、A-lite、竖排、字体链。
- 任何从阅读器实测得出的规则，必须能追溯到具体 demo 文件、构建产物、阅读器名称/版本、现象描述和处理结论；缺少这些信息时，只能记录为待验证假设，不能写成最终约束。
- 每次新增、修复或推翻一个兼容性判断，都必须实时更新 `docs/final/reader-matrix.yaml`。若该判断会影响制作规则，必须继续更新 SPEC、终极手册、速查表；若会影响自动化行为，再同步更新相关 skill。
- 任何新增第三方 EPUB 参考样本，必须同步更新 `THIRD_PARTY.md`（来源/作者/许可/链接）。

## 实测回写闭环

阅读器问题的默认工作方式是「demo 先行，文档后补」：

1. 复现：先在 `templates/epub-style-demo/` 添加或修改最小但真实的 demo 场景，不直接改手册定论。
2. 构建：运行模板自己的 build 脚本，生成带时间戳的 EPUB 到 `templates/epub-style-demo/dist/`。
3. 验证：用目标阅读器或转换器验证，保留错误码、截图、日志摘要、阅读器名称与版本。
4. 记录：立即更新 `docs/final/reader-matrix.yaml`，标记 `pass | warn | fail | na`，并写明 artifact、现象、处理动作和待复测项。
5. 固化：只有当 demo 与 reader-matrix 支撑结论后，才把规则写入 `docs/final/SPEC-实现约束.md`。
6. 同步：SPEC 变更后，同步更新终极手册、速查表和相关 skills，避免文档、样本、自动化契约分叉。

如果实测结果与手册冲突，以实测 demo 和 reader-matrix 为准，手册必须被修正。

## 建议流程

1. 先判断是「阅读器/打包实测问题」「实现约束变更」还是「说明增强」。
2. 阅读器/打包实测问题：先改 `templates/` 或新增 fixture，build EPUB，记录阅读器版本、日志、现象与结果。
3. 实测结果稳定后，再按 `reader-matrix.yaml` → `SPEC-实现约束.md` → 终极手册 → 速查表 → skills 的顺序同步规则。
4. 说明增强：仅改 `docs/source/` 或 `docs/experiments/` 也可。
5. 变更后执行最小自检（打包、结构 grep、XML/ZIP 校验、链接、标题、路径、术语一致性）。
