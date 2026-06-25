# 文档目录

本目录按读者任务分层。`docs/final/` 是稳定约束层，其余桶围绕它组织。

## 先读

### 入门层

- [learn/](learn/)：第一次接触本仓时的阅读路径
  - [00-what-is-epub.md](learn/00-what-is-epub.md)
  - [01-first-epub.md](learn/01-first-epub.md)
  - [02-anatomy.md](learn/02-anatomy.md)
  - [03-readers.md](learn/03-readers.md)
  - [04-skills.md](learn/04-skills.md)
  - [05-case-study.md](learn/05-case-study.md)
  - [06-test-your-own.md](learn/06-test-your-own.md)
  - [07-faq.md](learn/07-faq.md)
  - [08-epub2-epub3-compatibility.md](learn/08-epub2-epub3-compatibility.md)
  - [09-make-a-book.md](learn/09-make-a-book.md)
  - [glossary.md](learn/glossary.md)

### 工程契约层

- [final/SPEC-实现约束.md](final/SPEC-实现约束.md)：实现硬规则
- [final/字体别名命名规范.md](final/字体别名命名规范.md)：字体 alias、文件名与 class 的短别名规范
- [final/EPUB 3 终极实践手册.md](final/EPUB 3 终极实践手册.md)：对外手册
- [final/EPUB 3 HTML CSS 属性速查表.md](final/EPUB 3 HTML CSS 属性速查表.md)：属性速查表
- [final/reader-matrix.yaml](final/reader-matrix.yaml)：阅读器兼容性实测矩阵

### 场景指南

- [how-to/](how-to/)：针对特定排版场景的实操指南
  - [kindle-font-rendering-deep-dive.md](how-to/kindle-font-rendering-deep-dive.md) ⭐ Kindle 字体渲染深度参考
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

## 按需 / 贡献者

### 治理

- [meta/](meta/)：仓库治理入口，指向 AGENTS.md 分工表与各桶索引

### 推导与实验

- [source/](source/)：早期推导稿（已清空，历史在 git）
- [experiments/](experiments/)：实测、复盘和实验快照

### 模板与工具

- 自造清洗 / diff demo：[../templates/cleanup-demo-books/](../templates/cleanup-demo-books/)
- 第三方来源记录：[../THIRD_PARTY.md](../THIRD_PARTY.md) 与 [../references/](../references/)

## 新文档放哪

```text
文档是给谁看的、什么角色？
|
|- 第一次接触本仓的人 / AI -> docs/learn/
|
|- 对外硬约束（违反等于事故）-> docs/final/
|
|- 某类书的实操指南 -> docs/how-to/
|
|- 已有 EPUB 的清洗流程 / 工具 / 模式 -> docs/pipeline/
|
|- 治理、索引、架构分工 -> docs/meta/
|
|- 实测复盘、实验快照 -> docs/experiments/
|
`- 早期推导稿（历史在 git）-> docs/source/
```

强约束：

- `docs/final/` 只放对外硬约束；新增前必须能被 `AGENTS.md` 的规范来源优先级解释。
- `docs/how-to/` 只放场景指南，不承载计划、流水线或架构。
- `docs/pipeline/` 只放流程、工具和模式文档。
- `docs/experiments/` 文件名必须带日期或 commit hash。
