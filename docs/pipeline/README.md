# 批处理流水线

> 把已有 epub 批量处理（清洗、改造、对比、回写）的工作流文档。

## 核心文档（按读取顺序）

1. [cleanup-flow.md](cleanup-flow.md)：流水线主流程（preflight -> 可选结构规范化 -> EPUB3 迁移 -> 精排建议 -> 红线 gate -> diff review -> reader-matrix 回写）
2. [oneclick-epub3-converter.md](oneclick-epub3-converter.md)：已有 EPUB 一命令审计、旧 EPUB/EPUB2 转 EPUB3、弹注和 CJK 角色排版覆盖层
3. [refinement-harnesses.md](refinement-harnesses.md)：预检、EPUB3 迁移与精排建议 harness
4. [cleanup-patterns.md](cleanup-patterns.md)：典型脏 EPUB 模式识别与 skill 推荐顺序
5. [asset-optimization.md](asset-optimization.md)：图片与字体优化（精排建议的资源附件）
6. EPUB diff review：见 [根 README #epub-diff-review](../../README.md#epub-diff-review)（Calibre / VS Code）
7. [skills-matrix.md](skills-matrix.md)：15 个 skill 在清洗 / 新书流程中的角色
8. [decisions.md](decisions.md)：Stage 落地决策与偏差记录
9. [reference-font-role-patterns.md](reference-font-role-patterns.md)：本地文学 EPUB 的脱敏字体角色分析

## SPEC 对应

清洗流程的硬规则在 [../final/SPEC-实现约束.md §10](../final/SPEC-实现约束.md)。
