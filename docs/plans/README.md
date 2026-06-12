# 计划与审稿

> 本目录是计划 / review / 维护记录，**面向贡献者**。第一次接触本仓的新人请从 [docs/getting-started/](../getting-started/) 开始，不需要读这里。

这个目录放：

- **扩展计划**：当前在做或将来要做的多阶段重构 / 新功能。
- **审稿 / review**：对某个阶段的实测对照。
- **维护说明**：仓库自身工作流的说明（skills 维护、模板更新）。

本目录文档**不直接驱动行为**；规则的最终来源仍然是 `docs/final/`。

## 当前计划

- `2026-06-12-lint-and-quickstart-plan.md`：epub-lint v0 + book-starter 快速上手执行计划
- `demo-scene-expansion-plan.md`：demo 模板场景扩展与未测场景补测跟踪
- `skills-and-templates.md`：skills 维护方式与模板目录约定

## 近期复核优先级

1. `docs/final/reader-matrix.yaml` 中 `07-font-family-order` 的正文字体锁定 / 自由模式复测：先确认 `ibooks:specified-fonts` 是否影响 Apple Books 字体切换，再回修 SPEC §8。
2. 两个 Kindle warn 复测：`00-cover-metadata` 与 `09-kindle-risk` 需用当前构建产物重新导入 Kindle Previewer，避免旧 artifact 结论继续滞留。
3. `17-image-layout` 的 Kindle / Readest figure 百分比宽度复测：确认已改为 `figure.img-left/right` 主路径后，是否仍存在图片偏小或大字号回归。

## 已归档

落地完成且无续做的计划，按需移到 `plans/archive/`。

- `archive/handbook-expansion-plan.md`：三层手册 + 清洗流水线 + diff 工具的 4 Stage 历史计划（2026-05-26）
- `archive/handbook-expansion-review.md`：上面计划的落地审稿（2026-05-27）
- `archive/2026-05-28-remove-epub-diff.md`：移除整个 `tools/` 目录、把 diff workflow 写进根 README、丰富 README 的 review + 执行计划（2026-05-28）
- `archive/2026-05-28-remove-epub-diff-followup.md`：上一条计划落地后补提缺失的主计划文件、修复 4 处死链（2026-05-28）
- `archive/2026-05-28-readme-tools-followup-review.md`：复查并落地 README 断链、空 `tools/` 本地残留和 follow-up 正文去重（2026-05-28）
- `archive/2026-06-03-audit-remediation-plan.md`：仓库审计发现的修复清单与 v0.2.0 收尾依据（2026-06-03）
- `archive/2026-06-10-repo-improvement-execution-plan.md`：文档治理、公共模块、排版决策、图片候选、风格预设与 v0.2.0 发版执行记录（2026-06-10）
- `archive/css-layering-plan.md`：`Styles/` 八层 CSS 骨架计划；核心分层已由 demo 与 SPEC §7 吸收，保留作历史推导（2026-06-12 归档）
- `archive/fonts-css-expansion-plan.md`：`fonts.css` 系统字体优先策略；核心条款已由 SPEC §8 和 demo validator 接管，旧的无条件 `ibooks:specified-fonts` 清单不再作为现行规则（2026-06-12 归档）
- `archive/2026-06-12-body-font-mode-review-fixes.md`：正文字体模式 review 修复清单；15 项已执行，后续只保留 reader-matrix 复测项（2026-06-12）
