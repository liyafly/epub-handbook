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

当前合成样本在 Readest 0.12.16（18px）和 Kindle Previewer 4.0.3（Bookerly 8）看到轻量首字与下沉首字，按各自测试 SHA 记 pass。Apple Books 9.0 的 AX 有完整正文，但阅读区捕获为空白，重启后仍相同，记 warn，未定位为 CSS 问题。 详见 [当前产物、SHA 与截图日志](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。不把合成样本范围外推到其他书。
