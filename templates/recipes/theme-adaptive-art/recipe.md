# 主题插图

## 效果

先让透明插图在任何主题都可见：组件内白色衬片为保守路径。确认是单色线稿后，可选 `theme-art-line` 在夜间反相。彩色图、照片及带纸纹图片不反相。

## 标记结构

[sample.xhtml](sample.xhtml) 用 `figure.theme-art`；单色线稿才加 `theme-art-line`。`art.svg` 是独立合成黑色线稿，仅检验机制，透明画布不含纸纹。实际位图保留原尺寸与比例；转换前后的 RGB、alpha 和白色合成结果用 [epub-art](../../../tools-art/README.md) 审计，封闭白区与保护蒙版另行确认。

## 增强层

[recipe.css](recipe.css) 将主题查询和滤镜支持查询同时满足时的反相限定在单色组件，增强时才移除局部衬片。不设置 html/body 页面颜色，不按像素白度自动反相所有图。阅读器主题未必映射到系统 `prefers-color-scheme`，因此仍须手动切换阅读器主题观察。

## 降级行为

忽略查询或滤镜时保持原图与白色局部衬片；它可能有白框，但仍可辨认。不可用 opacity 隐去整图或使纸张边缘“看起来消失”。需无白框的交付必须逐目标实测，不能仅看浏览器。

## 阅读器结果

Readest 0.12.16 浅色 26px / 深色 18px 中两幅线稿可见，深色仍保留黑线白色衬片，可选反相未触发，记 warn。Kindle Previewer 4.0.3 白底可显示两图，但转换走非增强排版，夜间未测，记 warn。Apple Books 当前包和实际书图的夜间边缘仍待验证。 详见 [当前产物、SHA 与截图日志](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。不把合成样本范围外推到其他书。
