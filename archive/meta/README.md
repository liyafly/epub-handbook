# 历史治理与索引

本页保存搬迁前的治理分桶。当前行为约束以根目录
[AGENTS.md](../../AGENTS.md) 为唯一维护源，当前文档索引见
[docs/README.md](../../docs/README.md)。

本目录另保存 [2026-05-26 流水线决策快照](2026-05-26-pipeline-decisions.md)，
只用于追溯早期取舍，不作为当前流程入口。

Go CLI 重写的历史材料已拆分为[迁移期决策快照](2026-08-30-go-rewrite-decisions.md)
与[复审日志](2026-09-go-rewrite-review-log.md)；当前接手状态仍维护在
[`docs/pipeline/go-rewrite-handoff.md`](../../docs/pipeline/go-rewrite-handoff.md)。

## 各桶入口

| 桶 | 定位 | 入口 |
|---|---|---|
| `docs/final/` | 唯一硬约束 + 权威手册 | [当前目录](../../docs/final/) |
| `docs/learn/` | 纯入门教程 | [当前入口](../../docs/learn/README.md) |
| `docs/how-to/` | 场景实操指南 | [当前入口](../../docs/how-to/README.md) |
| `docs/pipeline/` | 批处理流水线 | [当前入口](../../docs/pipeline/README.md) |
| — | 历史计划与审稿 | 已移除，记录见 git 历史 |
| `archive/experiments/` | 决策痕迹与实测 | [归档入口](../experiments/README.md) |
| `archive/source/` | 早期推导（已清空，历史在 git） | [归档入口](../source/README.md) |
