# 文档目录

本页是维护者和机器使用的完整索引。第一次接触 EPUB 的读者只需从
[learn/README.md](learn/README.md) 开始；AI 与专业维护者先读根目录 `AGENTS.md`。

## 普通人入口

- [learn/README.md](learn/README.md)：唯一新手入口和症状直达表
- [learn/做一本书.md](learn/做一本书.md)：`book-starter` 快速路径与手写原理
- [learn/进阶-结构与兼容.md](learn/进阶-结构与兼容.md)：结构、版本和平台兼容（进阶）
- [learn/03-readers.md](learn/03-readers.md)：阅读器与测试范围
- [learn/04-skills.md](learn/04-skills.md)：AI skills 反向查表
- [learn/05-case-study.md](learn/05-case-study.md)：清洗案例
- [learn/06-test-your-own.md](learn/06-test-your-own.md)：测试自己的 EPUB
- [learn/07-faq.md](learn/07-faq.md)：常见问题
- [learn/glossary.md](learn/glossary.md)：术语表

## AI / 专业维护者

### 工程契约

- [final/SPEC-实现约束.md](final/SPEC-实现约束.md)：实现硬规则
- [final/字体别名命名规范.md](final/字体别名命名规范.md)：字体 alias、文件名与 class 的短别名规范
- [final/EPUB 3 终极实践手册.md](final/EPUB%203%20终极实践手册.md)：对外手册
- [final/EPUB 3 HTML CSS 属性速查表.md](final/EPUB%203%20HTML%20CSS%20属性速查表.md)：属性速查表
- [final/reader-matrix.yaml](final/reader-matrix.yaml)：阅读器兼容性实测矩阵

### 场景指南

- [how-to/](how-to/)：特定排版场景的实操指南
  - [kindle-font-rendering-deep-dive.md](how-to/kindle-font-rendering-deep-dive.md)
  - [english-fiction-layout.md](how-to/english-fiction-layout.md)
  - [classical-modern-layout.md](how-to/classical-modern-layout.md)
  - [chapter-head-image.md](how-to/chapter-head-image.md)
  - [anthology-navigation.md](how-to/anthology-navigation.md)
  - [note-box-border-styles.md](how-to/note-box-border-styles.md)
  - [epub2-popup-note-compatibility.md](how-to/epub2-popup-note-compatibility.md)
  - [duokan-footnote-fallback-fix.md](how-to/duokan-footnote-fallback-fix.md)

### 批处理流水线

- [pipeline/](pipeline/)：已有 EPUB 的清洗流程与工具
  - [cleanup-flow.md](pipeline/cleanup-flow.md)
  - [cleanup-patterns.md](pipeline/cleanup-patterns.md)
  - [refinement-harnesses.md](pipeline/refinement-harnesses.md)
  - [package-operations.md](pipeline/package-operations.md)
  - [epub-diff-review.md](pipeline/epub-diff-review.md)
  - [skills-matrix.md](pipeline/skills-matrix.md)
  - [decisions.md](pipeline/decisions.md)

### 模板、工具与历史

- Python 工具入口：[../scripts/README.md](../scripts/README.md)
- 自造清洗 / diff demo：[../templates/cleanup-demo-books/](../templates/cleanup-demo-books/)
- 第三方来源记录：[../THIRD_PARTY.md](../THIRD_PARTY.md) 与 [../references/](../references/)
- 已完成的设计、实施计划、实验和早期推导：[../archive/](../archive/)

## 新文档放哪

```text
文档给谁看、承担什么角色？
|
|- 第一次接触本仓的人 -> docs/learn/
|
|- 对外硬约束（违反等于事故）-> docs/final/
|
|- 某类书的实操指南 -> docs/how-to/
|
|- 已有 EPUB 的清洗流程 / 工具 / 模式 -> docs/pipeline/
|
|- AI / 专业维护规则与架构总纲 -> AGENTS.md
|
`- 已完成的计划、review、实验、早期推导 -> archive/ 或 git 历史
```

强约束：

- `docs/final/` 只放对外硬约束；新增前必须能被 `AGENTS.md` 的规范来源优先级解释。
- `docs/how-to/` 只放场景指南，不承载计划、流水线或架构。
- `docs/pipeline/` 只放流程、工具和模式文档。
- 历史记录不反向覆盖当前约束；需要重新激活时先在当前任务中验证，再进入对应活跃文档。
