# OpenType 特性

## 效果

英文正文用嵌入字体自带的旧式数字（`onum`）、小型大写（`smcp`）和连字（`liga`、`dlig`）。来源：[SPEC §8](../../../docs/final/SPEC-实现约束.md#opentype-特性)。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 待测 | 待测 | 待测 | 特性字形 → 普通字形 |

公共合成包不带字体，样例只能看到普通字形，效果只能在书级用真字体验证。

## 标记结构

见 [sample.xhtml](sample.xhtml)：段落或行内加 `ot-oldstyle`、`ot-smallcaps`、`ot-ligatures`。小型大写写小写字母（`bbc`），不要先写成大写，否则会变成普通大写。

## 增强层

[recipe.css](recipe.css) 每个类同时写 `font-variant-*` 和对应的 `font-feature-settings`，后者给只认底层写法的阅读器。`font-feature-settings` 不会叠加：一个元素同时用两个类时，后写的会盖掉前面的；需要组合就另写一个类，把几个特性写进同一条 `font-feature-settings`。

字体参照 [fonts.example.css](fonts.example.css)：放授权字体、在 OPF 声明、子集化并检查覆盖。子集化已保留全部 OpenType 特性（provider 的 `layout_features = ["*"]`），不用改 provider。

## 降级行为

字体没有该特性、或阅读器不认时，显示普通字形，文字不变。例外：字体没有 `smcp` 时，部分阅读器会把 `font-variant-caps: small-caps` 合成为缩小的大写，看起来比真小型大写细。不想要合成效果就删掉这一行，只留 `font-feature-settings`。

## 阅读器结果

尚未实测。记录到 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-opentype-features`。
