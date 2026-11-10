# 纵中横

## 效果

竖排正文里，两三位的数字（年份后两位、月日、回数）横着排进一个字格，像手写竖排那样。来源：[SPEC §5.12](../../../docs/final/SPEC-实现约束.md#512-竖排纵中横)。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 待测 | 待测 | 待测 | 纵中横 → 数字侧躺 |

只写好合成样本，没有在任何阅读器里看过。

## 标记结构

见 [sample.xhtml](sample.xhtml)：把要横排的数字包进 `<span class="tcy">`，不改文字，不插空格。只包两三位数字；四位以上横排会挤成看不清的一格，保持侧躺或改用汉字数字。

## 增强层

[recipe.css](recipe.css) 同时写 `-webkit-text-combine: horizontal`（旧 WebKit）、`-epub-text-combine: horizontal`（EPUB 3.0 前缀）和标准的 `text-combine-upright: all`，标准写法放最后。`.tcy-vertical` 只是样例里的竖排容器，书里已有竖排容器（如 demo 的 `page-vrl`）时只复制 `.tcy`。

## 降级行为

不认这些属性时，数字按竖排的默认方式侧躺，仍按原顺序可读。横排页面里 `.tcy` 不起作用。

## 阅读器结果

尚未实测。记录到 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-tate-chu-yoko`。
