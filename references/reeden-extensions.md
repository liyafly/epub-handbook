# Reeden 电子书扩展来源说明

核对日期：2026-10-02。来源发布方 / 作者署名：Reeden 官方文档站；未见逐页个人作者署名。扩展总览自标为 **0.1 Draft**，此版本号不代表 Reeden 应用版本。

本仓只保存来源链接与自行撰写的行为摘要、对照分析，没有镜像官网正文、截图或示例素材。所阅页面未提供明确的文档再分发许可；不能把公开访问或产品会员“许可证”当作文档复用授权。完整来源和权利说明维护在 [THIRD_PARTY.md](../THIRD_PARTY.md)。

## 本次核对的一手页面

| 页面 | 本次使用的内容 |
| --- | --- |
| [电子书扩展](https://docs.reeden.app/ebook_spec) | Draft 状态、标准优先与增强思路 |
| [扩展全集](https://docs.reeden.app/ebook_spec_compatibility) | 名称、放置位置、Reeden / 多看兼容别名 |
| [全屏插图](https://docs.reeden.app/ebook_spec_fullscreen) | EPUB3 prefix、spine/itemref、整屏 / 跨页模式与单主图路径 |
| [页面出血](https://docs.reeden.app/ebook_spec_bleed) | 题图局部贴边、双页中缝、局部宽度与顶部位置 |
| [富文本脚注](https://docs.reeden.app/ebook_spec_footnote) | 标准语义、集中式结构与多看别名 |
| [单图预览](https://docs.reeden.app/ebook_spec_image_single) | 单主图与主副标题关联 |
| [夜间反色](https://docs.reeden.app/ebook_spec_dark_mode_invert) | 单色透明字图、实际 img 上的声明与取值 |
| [图片画廊](https://docs.reeden.app/ebook_spec_image_gallery) | gallery / cell 与每帧图题关联 |
| [扩展音频](https://docs.reeden.app/ebook_spec_audio) | 标准 audio/source 与状态图增强 |

另对照 [W3C EPUB 3.3 前缀机制](https://www.w3.org/TR/epub-33/#sec-prefix-attr)，用于区分扩展词汇声明与 EPUB 默认词汇；W3C 文档适用其 [文档许可](https://www.w3.org/copyright/document-license/)。

这些网页可随上游更新；本页日期仅说明查阅时点。阅读器支持声明不构成本仓 fixture 的兼容 pass。所阅扩展页面未说明 `~slim` 图片选择算法，也没有提供 Reeden 自动选择 `~slim` 的证据。

## 仓库内的研究落点

[多看长屏封面与 Reeden 阅读器扩展参考](../docs/pipeline/reference-reader-extensions.md) 保存本地封面结构证据、推断边界、与现有样式的关系和后续实验建议。原始 EPUB、书稿、字体和封面图均不复制进本仓。
