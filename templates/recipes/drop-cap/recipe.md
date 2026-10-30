# 首字装饰

## 效果

章首保留完整单词，以 `::first-letter` 做轻量首字装饰。下沉浮动是显式选择的增强，不强制整书启用。来源：[SPEC §5.9](../../../docs/final/SPEC-实现约束.md#59-英文小说正文)。

## 标记结构

[sample.xhtml](sample.xhtml) 使用 `p.recipe-opening`，开启下沉再加 `recipe-opening-sunk`；不拆首字 span，不插入空格。英文页面声明 `lang` 与 `xml:lang`。引号开头、数字、连字符、朗读及复制顺序都需检查。

## 增强层

[recipe.css](recipe.css) 默认首段无缩进，首字略放大；可选下沉类在宽于 24em 时给首字 float。阈值及行高是待调起点。特殊装饰字体另放 `fonts.css`，用授权母版与 OPF 声明闭环，不能依赖未打包的字体名。

## 降级行为

忽略伪元素则是完整普通段落；小屏和大字号使宽度条件不满足时保持轻量首字。若目标阅读器的首字浮动挤压文本，移除可选类。不要用固定段高或裁切掩盖问题。

## 阅读器结果

[矩阵 case `18-english-fiction`](../../../docs/final/reader-matrix.yaml) 的 Apple Books、Readest、Kindle Previewer 历史记录均为 warn：默认字号观察可见，更多字号与字体仍待复测。历史浮动 span 与本片段的伪元素路径不同，不能转签。本片段待首次按当前 SHA 实测。
