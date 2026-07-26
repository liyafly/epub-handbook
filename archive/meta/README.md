# 治理与索引

本目录是仓库治理的入口索引。行为约束以根目录 [AGENTS.md](../../AGENTS.md) 为唯一维护源。

## 各桶入口

| 桶 | 定位 | 入口 |
|---|---|---|
| `docs/final/` | 唯一硬约束 + 权威手册 | [README](../final/) |
| `docs/learn/` | 纯入门教程 | [README](../learn/README.md) |
| `docs/how-to/` | 场景实操指南 | [README](../how-to/README.md) |
| `docs/pipeline/` | 批处理流水线 | [README](../pipeline/README.md) |
| — | 历史计划与审稿 | 已移除，记录见 git 历史 |
| `docs/experiments/` | 决策痕迹与实测 | [README](../experiments/README.md) |
| `docs/source/` | 早期推导（已清空，历史在 git） | [README](../source/README.md) |

## 架构分工

按 [AGENTS.md](../../AGENTS.md) 的分工表：

- **Python**（`scripts/` + `skills/`）：AI agent provider，CLI 与验证基线
- **Swift**（`swift/`）：执行核心，native 首要 provider
- **字体 / 图片转换**：独立 Python 项目（`uv` 管理），Swift/GUI 通过子进程调用 CLI
- **GUI**（`gui/`）：PARKED，当前非焦点
- **契约**（`contracts/` + `adapters/`）：Python/Swift 按 capability 并存的契约与 agent 适配表面
