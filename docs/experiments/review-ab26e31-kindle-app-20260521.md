# Review: ab26e31 + Kindle App 2026-05-21 follow-up

范围：commit `ab26e31` 后的 epub-style-demo 兼容性修补。  
来源：用户 2026-05-21 Kindle App 实测反馈。  
状态：demo 已按本轮结论修改并构建；reader-matrix 的 pass/fail 与最终 SPEC 仍需人工 Kindle App 复测后回写。

## 已落 demo 修改

- `.wavy` 从 `text-decoration: underline wavy` 拆为基础 underline + `text-decoration-style: wavy`，避免 Kindle/KF8 丢弃整条 shorthand。用户复测显示 Kindle App 至少显示普通下划线，wavy 不显示。
- 图片环绕主路径改为 `figure.img-left/right { float; width: 30%; }`，并把正式建议收敛为 `25%–35%` 百分比宽度区间；去掉 direct img 直挂对照，用户反馈 direct img 在部分阅读器显示偏小。
- `17-image-layout.xhtml` 增加长正文阈值、短段反例与大字号 figure 回归测试。
- `.demo-system-first` 字体链补 `SimSun`。
- 新增 `03b-poster-fullbleed.xhtml`，并在 OPF / NCX / nav / 场景矩阵中接入。
- `poster.css` 新增 `body.poster-bg-fullbleed`，用 `background-size: cover` 做全幅海报对照。

## 待人工复测

构建产物：`templates/epub-style-demo/dist/epub-style-demo-20260521-230603.epub`

必须记录阅读器名、版本、artifact、现象描述与 `status: pass | warn | fail | na`：

- `10-text-effects`：Kindle App 中 `.wavy` 拆行后是否至少显示 underline。
- `17-image-layout`：默认字号下 figure 浮动 + CSS 百分比宽度是否环绕；demo 默认 `30%`，正式书稿建议在 `25%–35%` 内按阅读器实测微调。
- `17-image-layout` 大字号回归测试：放大字号后 figure 是否仍环绕。
- `03-alite-poster`：80% 海报背景是否保持既有基线。
- `03b-poster-fullbleed`：`background-size: cover` 是否满铺。

## Kindle conversion log 记录

`templates/epub-style-demo/dist/epub-style-demo-20260521-232619-conversionLog.csv` 显示：

- `W14012`：不支持媒体文件格式。
- `W14015`：`OEBPS/Images/complex-webp.webp` 图像无效，且没有通过电子书创建。

结论：Kindle 主路径不得使用 WebP。WebP 只能作为现代阅读器增强实验；面向 Kindle 的构建产物必须预转换为 JPEG / PNG。复杂 SVG 未出现在该 conversion log 中，说明转换阶段未报 SVG 错，但仍需 Kindle App 视觉确认。该 WebP / SVG 临时 demo 已从 `epub-style-demo` 中移除，只保留本结论。

## 检索结论

- Apple Books Asset Guide 5.3.1: Apple Books 支持 EPUB 3.3，并在可访问性章节把 MathML 描述为 EPUB 3 中表示数学公式的 XML 标记语言。见 [Apple Books Asset Guide](https://help.apple.com/itc/booksassetguide/en.lproj/static.html)。
- Kindle 官方 [Image Guidelines - Reflowable](https://kdp.amazon.com/en_US/help/topic/G75V4YX5X8GRGXWV) 说明带 caption 的图片使用 `figure`；用户 Kindle App 实测确认 figure 浮动也能环绕，因此 demo 以 figure 为主路径。
- [Kindle Publishing Guidelines PDF](https://kindlegen.s3.amazonaws.com/AmazonKindlePublishingGuidelines.pdf) 支持表列出 `float` 与 `width` 支持；本轮主路径使用百分比宽度，建议先在 `25%–35%` 内调整，避免 fixed px 在不同屏幕和阅读器中显得过大或过小。
- Kindle Publishing Guidelines 对 `text-decoration` 的支持值只列 `overline` / `underline`，没有 `text-decoration-style`；Kindle App 显示普通下划线是预期 fallback。
- KDP [Text Guidelines - Reflowable](https://kdp.amazon.com/en_US/help/topic/GH4DRT75GWWAGBTU) 的 MathML Support 说明 Enhanced Typesetting 支持 MathML，并列出本轮 demo 覆盖的标签集合。
- W3C [EPUB 3.3](https://www.w3.org/TR/epub-33/) 作为 Apple Books 与项目最终文档的 EPUB 标准基线；含 MathML 的内容文档需要在 OPF manifest 上通过 `properties="mathml"` 标记。

## 文档同步门槛

人工复测完成后，按 `reader-matrix.yaml` -> `SPEC-实现约束.md` -> `EPUB 3 终极实践手册.md` -> `EPUB 3 HTML CSS 属性速查表.md` 的顺序同步。缺少 Kindle App 版本号或 artifact 时，不把 Q1/Q3/Q4 写成最终约束。
