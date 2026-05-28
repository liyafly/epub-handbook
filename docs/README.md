# 文档目录

本目录按读者任务分层。`docs/final/` 是稳定约束层；`docs/plans/` 和 `docs/source/` 只保留规划、推导与历史材料。

## 入门层

- [getting-started/](getting-started/)：第一次接触本仓时的阅读路径
  - [01-first-epub.md](getting-started/01-first-epub.md)
  - [02-anatomy.md](getting-started/02-anatomy.md)
  - [03-readers.md](getting-started/03-readers.md)
  - [04-skills.md](getting-started/04-skills.md)
  - [05-case-study.md](getting-started/05-case-study.md)
  - [06-test-your-own.md](getting-started/06-test-your-own.md)
  - [07-faq.md](getting-started/07-faq.md)
  - [glossary.md](getting-started/glossary.md)

## 工程契约层

- [final/SPEC-实现约束.md](final/SPEC-实现约束.md)：实现硬规则
- [final/EPUB 3 终极实践手册.md](final/EPUB 3 终极实践手册.md)：对外手册
- [final/EPUB 3 HTML CSS 属性速查表.md](final/EPUB 3 HTML CSS 属性速查表.md)：属性速查表
- [final/reader-matrix.yaml](final/reader-matrix.yaml)：阅读器兼容性实测矩阵

## 场景指南

- [guides/README.md](guides/README.md)：专题指南索引
- [guides/english-fiction-layout.md](guides/english-fiction-layout.md)：英文小说排版
- [guides/classical-modern-layout.md](guides/classical-modern-layout.md)：文白对照 / 原译对照
- [guides/chapter-head-image.md](guides/chapter-head-image.md)：章首图
- [guides/anthology-navigation.md](guides/anthology-navigation.md)：合集导航
- [guides/note-box-border-styles.md](guides/note-box-border-styles.md)：便签框与边框
- [guides/duokan-footnote-fallback-fix.md](guides/duokan-footnote-fallback-fix.md)：多看 fallback

## 批处理流水线

- [pipeline/README.md](pipeline/README.md)：流水线主入口
- [pipeline/cleanup-flow.md](pipeline/cleanup-flow.md)：清洗流程
- [pipeline/refinement-harnesses.md](pipeline/refinement-harnesses.md)：预检、EPUB3 迁移与精排建议 harness
- [pipeline/cleanup-patterns.md](pipeline/cleanup-patterns.md)：典型脏 EPUB 模式
- [pipeline/asset-optimization.md](pipeline/asset-optimization.md)：图片与字体优化
- EPUB diff review：见 [根 README #epub-diff-review](../README.md#epub-diff-review)（Calibre / VS Code）
- [pipeline/skills-matrix.md](pipeline/skills-matrix.md)：skill 角色矩阵
- [pipeline/decisions.md](pipeline/decisions.md)：决策记录

## 计划与架构参考

- [plans/README.md](plans/README.md)：计划、review 与仓库维护说明
- [architecture/README.md](architecture/README.md)：下游 / 周边架构副本

## 样本与工具

- 自造清洗 / diff demo：[../samples/demo-books/](../samples/demo-books/)
- 第三方样本占位：[../samples/third-party/](../samples/third-party/)
- EPUB diff review 工作流：[../README.md#epub-diff-review](../README.md#epub-diff-review)（Calibre / VS Code）

## 推导与实验

- [source/](source/)：早期推导稿和补充材料
- [experiments/](experiments/)：实测、复盘和实验快照

## 新文档放哪

```text
文档是给谁看的、什么角色？
|
|- 第一次接触本仓的人 / AI -> docs/getting-started/
|
|- 对外硬约束（违反等于事故）-> docs/final/
|  仅放 SPEC、终极手册、速查表、reader-matrix
|
|- 某类书的实操指南（英文小说、文白对照、章首图等）-> docs/guides/
|
|- 拿到一本现成 EPUB 后的流程 / 工具 / 模式 -> docs/pipeline/
|
|- 历史、在做或未来扩展计划；阶段 review；维护说明 -> docs/plans/
|
|- 下游 / 周边项目架构副本 -> docs/architecture/
|
|- 早期推导稿、补充材料 -> docs/source/
|
`- 某次实测 / 复盘 / 实验快照 -> docs/experiments/
```

强约束：

- `docs/final/` 只放对外硬约束；新增前必须能被 `CLAUDE.md` 的优先级规则解释。
- `docs/guides/` 只放场景指南；包含 plan、review、flow、pattern、tool 身份的文档应放到 `docs/plans/` 或 `docs/pipeline/`。
- `docs/plans/` 不直接驱动行为；落地规则必须回写到 `docs/final/`、模板、脚本或 skills。
- `docs/experiments/` 文件名必须带日期或 commit hash。
