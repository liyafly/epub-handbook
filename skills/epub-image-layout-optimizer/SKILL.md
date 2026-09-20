---
name: epub-image-layout-optimizer
description: 只读分析 EPUB 图片角色、figure 环绕、图注与格式风险并给出候选；授权后人工修复。不压缩图片、不自动分配左右浮动，整页海报另用 A-lite。
---

# EPUB 图片版式优化

## 何时用

图片过小、裁切、环绕或图注异常时。改前读 [SPEC §5.1、§5.6、§5.10.1](../../docs/final/SPEC-实现约束.md)；章首图转 literary skill，全页封面式布局转 A-lite，换封面文件用 package operator。

## 调什么

```sh
epub run epub.image.layout.optimize --input "book.epub" --json
```

这是只读扫描，无 `--output`；压缩、色彩转换和转码使用外部工具，完成后再复查资源与版式。

## 返回怎么读

`facts.imageFindings[]` 给出 file、selector、image、scene、finding、candidates；`facts.warningList` 是扫描缺口。前缀 `epub.image.layout.optimize.` 的 `findings/warnings` 是计数。
候选类别包括 lone-image-no-figure、caption-detached、float-width-risk、missing-alt、chapter-head-image-candidate、fullpage-image-alite-candidate。计数不等于错误数，也不是批量改写清单；公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 先确认正文图、封面、图标、公式、章首或背景角色。noteref 图片是交互控件，扫描已排除，不把它包装成 figure。
- 图文关系未确认时不自动加左右浮动。需要环绕时用 `figure.img-left/right` 承载 float 与百分比宽度，内图 `width:100%; height:auto`；25%–35% 只是实测起点。
- 保留图注、alt、文字和资源；不为整齐裁掉内容。JPEG/PNG 为生产主路径，Kindle 风险 SVG/WebP 先确认转码范围。
- 有扫描 warning 则结论不完整；手工修改后跑全项红线和 package audit，并用足够长的周围正文验证普通/大字号及窄屏，短段落不足以证明环绕失败。
