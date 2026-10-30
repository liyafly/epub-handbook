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

Readest 0.12.16 的 26px 桌面配置中短组双栏、长组上下并自然跨页；Kindle Previewer 4.0.3 的 Song 4 / 8 均按源序上下，8 号字续页完整。这些配置按各自测试 SHA 记 pass。Apple Books 当前包捕获的阅读区灰色空白，无法验收视觉效果，记 warn；窄屏、其他字体与昼夜组合仍待测。 详见 [当前产物、SHA 与截图日志](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。不把合成样本范围外推到其他书。

Apple Books 补验见 [独立日志](../reader-evidence/observations-20261030-backing.json)。
