# 流水线决策记录

## 2026-05-26 Stage 落地决策

| # | 问题 | 决策 | 理由 |
| --- | --- | --- | --- |
| Q1 | Stage 顺序 | 严格串行 1 -> 2 -> 3 -> 4 | SPEC §10、diff app 和 demo 互相依赖 |
| Q2 | Vendor 资源是否入 Git | JS/CSS 不入 Git，只保留 license 和抓取脚本 | 保持仓库体积小，依赖可复现 |
| Q3 | AI 改动黄线是否可配置 | 不可配置 | 清洗边界统一由 SPEC §10 管 |
| Q4 | 公版书实体 .epub 是否入 Git | 不入；首轮 demo 改用自造 EPUB | 仓库体积、来源溯源和演示版式都更稳 |
| Q5 | Web app 入口位置 | ~~`tools/epub-diff/`~~ — 已于 2026-05-28 移除（见下） | 自维护性价比低，外部 Calibre / VS Code 是上位替代 |
| Q6 | `samples/fixtures-tiny/` 是否补齐真实 EPUB fixture | 暂不补齐；保留目录骨架和 README 作为未来手工扩展槽位 | 当前自动化测试已在 `scripts/test_validate_text_invariance.py` 内即时构造等价 EPUB，避免维护两套同类测试样本 |

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


## 2026-05-28 移除 tools/ 与丰富根 README

| # | 问题 | 决策 | 理由 |
| --- | --- | --- | --- |
| Q7 | 是否继续自维护 epub-diff web app | 移除整个 `tools/` 目录 | 1) 1344 行手写代码（含手写 SHA-256）维护成本高；2) Calibre Compare to another book 提供字符级 HTML/CSS diff、图片像素 overlay、文件树着色，是上位替代；3) `file://` 模块限制和 vendor 抓取流程违背仓库「双击即用」原则；4) 红线 gate 与本工具完全解耦，移除不影响自动化 |
| Q8 | 替代工作流落地点 | 不创建独立 guide；diff 工作流唯一权威写进根 `README.md` 的 `## EPUB diff review` 段 | 减少跳转层；让根 README 成为单一入口文档 |
| Q9 | 根 README 是否同步丰富 | 是；扩为多段结构化入口（仓库做什么 / 准备环境 / 5 分钟 / diff review / 清洗 / skills / 实测回写 / 文档地图） | 旧 README 仅 50 行场景表，承载不了「单一入口」定位 |
| Q10 | SPEC §10.2 / §10.4 / §10.5 提到的 web app 怎么改 | 字样全部替换为「外部 diff 工具」，并指向 README 锚点 `#epub-diff-review` | `docs/final/` 是硬约束层，必须与实际工具状态对齐 |

执行记录见 [../plans/2026-05-28-remove-epub-diff.md](../plans/2026-05-28-remove-epub-diff.md)。
