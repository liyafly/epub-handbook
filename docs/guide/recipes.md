# 配方

[English](recipes.en.md) · [指南首页](README.md)

配方是一段可以复制的 CSS 加一个最小样例，做一种特殊效果。每个配方都先写回退，再写增强：阅读器不支持增强时，书仍然好读。

配方在 [`templates/recipes/`](../../templates/recipes/README.md)，每个目录里有：

- `recipe.md`：效果、标记写法、回退方式和实测结果；
- `recipe.css`：要复制的样式；
- `sample.xhtml`：最小样例，照着它改 XHTML 的结构和类名。

用法：把 `recipe.css` 里需要的部分复制到书的 `design.css`（导入的书复制到它自己的样式表末尾），按 `sample.xhtml` 给正文加类名，然后预览。

## 各阅读器表现

| 配方 | Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- | --- |
| [轮廓环绕](../../templates/recipes/shape-wrap/recipe.md) | 首个展开页沿轮廓环绕（续页未看） | 退回矩形环绕 | 待测 | 轮廓 → 矩形 → 窄屏块图 |
| [首字装饰](../../templates/recipes/drop-cap/recipe.md) | 阅读区空白，原因未查明 | 轻量首字和下沉首字都正常 | 待测 | 首字装饰 → 普通段落 |
| [着重号](../../templates/recipes/text-emphasis/recipe.md) | 阅读区灰色，无法判断 | 下划线和圆点同时出现，可读 | 待测 | 原生着重号 → 下划线 |
| [主题插图](../../templates/recipes/theme-adaptive-art/recipe.md) | 深色主题下白底图可见 | 白底图可见，夜间待测 | 待测 | 图片自带白底 |
| [原译对照](../../templates/recipes/parallel-pair/recipe.md) | 阅读区灰色，无法判断 | 按原文、译文上下排 | 待测 | 宽屏双栏 → 上下排 |

这张表的证据边界：

- 结果来自合成样例，不来自真实的书；
- Apple Books 是 macOS 上的 9.0 版；Kindle 是 Kindle Previewer 4.0.3，不是 Send to Kindle 送到设备的结果；
- iPhone、Reeden、Send to Kindle 都还没测。"待测"就是接下来要补的实测清单。

每个格子的详细记录、截图和 Readest 结果，见对应的 `recipe.md` 和 [阅读器矩阵](../final/reader-matrix.yaml)。

## 书型预设

[预设](../../templates/style-presets/README.md) 是另一种起点：一整套适合某类书的样式。只在建新书时用一次（`epub book new 我的书 --from-preset literary-cn`），复制进来以后就归这本书自己改。
