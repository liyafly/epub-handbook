# 批处理流水线

> 把已有 epub 批量处理（清洗、改造、对比、回写）的工作流文档。

## 核心文档（按读取顺序）

1. [cleanup-flow.md](cleanup-flow.md)：流水线主流程（健康检查 -> 红线 gate -> diff review -> reader-matrix 回写）
2. [cleanup-patterns.md](cleanup-patterns.md)：典型脏 EPUB 模式识别与 skill 推荐顺序
3. [asset-optimization.md](asset-optimization.md)：图片与字体优化（清洗流程 §4 附件）
4. [diff-tool.md](diff-tool.md)：EPUB Diff Web App 使用说明
5. [skills-matrix.md](skills-matrix.md)：14 个 skill 在清洗 / 新书流程中的角色
6. [decisions.md](decisions.md)：Stage 落地决策与偏差记录

## SPEC 对应

清洗流程的硬规则在 [../final/SPEC-实现约束.md §10](../final/SPEC-实现约束.md)。
