# 主题插图

## 效果

先保证插图可辨认：保守样本使用资源内部的白色实底。Apple Books 深色主题可能覆盖 CSS 白底，不能仅凭 `background-color` 保证透明黑线可见。确认是单色线稿后，可选 `theme-art-line` 在夜间反相。彩色图、照片及带纸纹图片不反相。

## 标记结构

[sample.xhtml](sample.xhtml) 用 `figure.theme-art`；保守图 `art-backed.svg` 在 SVG 内绘制白色矩形。单色线稿才加 `theme-art-line`，其 `art.svg` 保留透明画布。两图均为合成材料，仅检验机制。实际位图保留原尺寸与比例；转换前后的 RGB、alpha 和白色合成结果用 [epub-art](../../../tools-art/README.md) 审计，封闭白区与保护蒙版另行确认；制作实底副本也须保留原图。

## 增强层

[recipe.css](recipe.css) 将主题查询和滤镜支持查询同时满足时的反相限定在单色组件，增强时才移除局部衬片。不设置 html/body 页面颜色，不按像素白度自动反相所有图。阅读器主题未必映射到系统 `prefers-color-scheme`，因此仍须手动切换阅读器主题观察。

## 降级行为

保守图忽略查询或滤镜时仍有资源内白底，白框是预期降级。可选透明图的 CSS 衬片依赖阅读器，不能替代保守图。不可用 opacity 隐去整图或使纸张边缘“看起来消失”。需无白框的交付必须逐目标实测，不能仅看浏览器。

## 阅读器结果

Apple Books 9.0 (6655) 深色主题中，修复前保守图黑线落在深色页上，记 fail；资源内增加白底后，保守图黑线白框、可选图白线均可见，只对新 SHA 的深色两页记 pass。Readest 0.12.16 新包浅色/深色 18px 两图可见，可选反相未触发，记 warn。Kindle Previewer 4.0.3 新包 Song4 白底两图可见，夜间未测，记 warn。详见 [修复前后 SHA 与截图](../reader-evidence/observations-20261030-backing.json)、[早期观察](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。实际书图边缘与其他配置仍待验证。
