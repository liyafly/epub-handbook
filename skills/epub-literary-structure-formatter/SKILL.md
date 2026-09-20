---
name: epub-literary-structure-formatter
description: 人工精排 EPUB 章首、前置页、题记、对话、诗、信件和文白对照，保持正文与可重排结构。用于文学页面示例和局部润色，不自动转海报或改导航。
---

# EPUB 文学结构精排

## 何时用

页面角色已确认，需要语义结构和视觉节奏。章首图读 [chapter-head-image](../../docs/how-to/chapter-head-image.md)，文白读 [classical-modern-layout](../../docs/how-to/classical-modern-layout.md)，文集导航读 [anthology-navigation](../../docs/how-to/anthology-navigation.md)，只加载任务命中的指南；硬边界仍以 [SPEC](../../docs/final/SPEC-实现约束.md) 为准。

## 调什么

`epub.literary.structure.format` 当前未实现；人工读取目标 XHTML 和 literary.css，按授权最小改写。角色模糊时先分析：

```sh
epub run epub.text.content.analyze --input "before.epub" --json
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

## 返回怎么读

blockList 的 evidence/confidence 用于角色复核，不是自动套 class 的许可；layout 是静态风险，redline 不能证明美观。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 先以普通章首、连续正文、诗/信件或文白复杂页建立一致样例，再推广可复用类；通过字号、间距、对齐和少量装饰形成层次，不重新设计原书。
- 章首保留真实 h1，装饰图用 figure；版权页保留真文本和信息顺序，不猜补书目事实；不将普通章首自动转换 A-lite。
- 文白先按源序上下；短组宽屏可增强浮动，长组/窄屏允许分页，不用 table/flex/grid 承载正文对照。具体比例和类名复用指南，不另造体系。
- 组件样式归 literary.css；不改变对话标点、诗行、译文或 mixed-content 空白。nav 文案同步属于另行明确的范围。
- 用 [场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 找 frontmatter/chapter-head/classical-modern 对照；新兼容结论走 demo 实测，不仅靠浏览器效果。
