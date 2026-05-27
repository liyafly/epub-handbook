# 流水线决策记录

## 2026-05-26 Stage 落地决策

| # | 问题 | 决策 | 理由 |
| --- | --- | --- | --- |
| Q1 | Stage 顺序 | 严格串行 1 -> 2 -> 3 -> 4 | SPEC §10、diff app 和 demo 互相依赖 |
| Q2 | Vendor 资源是否入 Git | JS/CSS 不入 Git，只保留 license 和抓取脚本 | 保持仓库体积小，依赖可复现 |
| Q3 | AI 改动黄线是否可配置 | 不可配置 | 清洗边界统一由 SPEC §10 管 |
| Q4 | 公版书实体 .epub 是否入 Git | 不入；首轮 demo 改用自造 EPUB | 仓库体积、来源溯源和演示版式都更稳 |
| Q5 | Web app 入口位置 | `tools/epub-diff/` | 与 fixture、docs 分离，作为用户工具 |

## 计划偏差记录

- `lxml` 是推荐依赖，但当前核心红线脚本使用标准库实现，避免环境未安装时无法跑测试。
- 计划文档写 Project Gutenberg #25196 为《唐诗三百首》；实测该编号是《百家姓》。
- 2026-05-27 用户确认首轮不做公版书 demo。Stage 4 主样本改为 `samples/demo-books/` 自造 EPUB；`samples/third-party/` 仅保留为未来第三方样本占位。

## 2026-05-27 文档重组

按 `docs/plans/handbook-expansion-review.md` §9 重组：

- `docs/guides/` 拆为 guides（场景）、plans（计划）和 pipeline（流程）。
- `docs/final/` 收回 `epub-pro` 架构副本到 `docs/architecture/`。
- `docs/architecture/` 承接原 `docs/reference/` 的下游参考定位。
- `docs/final/fixtures.md` 删除。
- `docs/experiments/EPUB 3 章节扉页与竖排实战 · 补充 05.md` 归位到 `docs/source/`。
- `docs/README.md` 增加新文档分类决策树。
