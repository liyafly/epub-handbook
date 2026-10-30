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

[矩阵 case `24-text-emphasis` / `25-text-emphasis-vertical`](../../../docs/final/reader-matrix.yaml) 均列为待测。本片段是候选；没有把来源书的 CSS 当作阅读器通过证据，需分别补横排、竖排、大字号与夜间模式。
