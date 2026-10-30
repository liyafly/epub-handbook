# 着重号

## 效果

保持真实文字与 `<em>` 语义，以原生着重号增强。候选默认保留下划线作为不支持着重号时的可见提示，不用字面圆点插入正文。横排与竖排是两个验收场景。

## 标记结构

[sample.xhtml](sample.xhtml) 使用 `em.recipe-emphasis`，文字与标点照原文保留；不要将每字改为 Ruby 来模拟圆点，以免改变朗读与复制。已有书若有字面标注，须作为独立正文变更审阅。

## 增强层

[recipe.css](recipe.css) 同时给出标准和 EPUB 的 `under right`，WebKit 前缀只写单值 `under`。支持声明的引擎可取消下划线；`@supports` 只说明解析能力，不等于真实显示通过。竖排位置、换行、标点跳过与行距见 [demo 24 / 25](../../epub-style-demo/OEBPS/Text/24-text-emphasis.xhtml) 比较探针。

## 降级行为

忽略着重号与条件增强时保留下划线；忽略全部 CSS 时仍有完整 `<em>` 文字。若引擎解析支持但不绘制，删除取消下划线的增强块，或保留双重提示，按目标实测决定。

## 阅读器结果

Readest 0.12.16 的横排 18px / 26px 下着重号可见、换行无重叠，按当前测试 SHA 记 pass。Kindle Previewer 4.0.3 Song 8 同时显示下划线和圆点，正文可读，条件取消下划线未生效，记 warn。竖排、夜间和 Apple Books 当前包仍待测。 详见 [当前产物、SHA 与截图日志](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。不把合成样本范围外推到其他书。
