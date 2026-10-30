# 原译对照

## 效果

基础状态按原文、译文上下排列；仅短组在宽屏增强为双栏。来源：[SPEC §5.6.1](../../../docs/final/SPEC-实现约束.md#561-文白对照左右兼容)。

## 标记结构

[sample.xhtml](sample.xhtml) 的 `.parallel-pair` 直接包含 `.classical-text` 与 `.modern-text`，随后是空 `.parallel-clear`。短组显式加 `.parallel-float-pair`；长组使用 `.parallel-stack-pair`，不包固定页高容器。

## 增强层

[recipe.css](recipe.css) 只在 `min-width:40em` 开双 float，起点 38%/58%，总宽留至少 4% 沟槽。近似等长可选 `.parallel-ratio-balanced`，原文较长可选 `.parallel-ratio-source-wide`。长组默认允许分页，短组才尝试避免内部切页。

## 降级行为

小屏、忽略媒体查询或浮动时按源序上下。Kindle、大字号或风险组直接保留 stack；不以 table、overflow:hidden 或整组强制不分页兜底。比例不会自动判断篇幅，需书级选择。

## 阅读器结果

[矩阵 case `21-classical-modern`](../../../docs/final/reader-matrix.yaml) 保留历史观察与字号/夜间复测项；转换成功不等于视觉通过。本片段尚未按当前 SHA 实测，Kindle 字号与分页复测继续保留，不能将 SPEC 的结构约定写成全阅读器 pass。
