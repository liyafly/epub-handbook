---
name: epub-image-layout-optimizer
description: 优化 EPUB 图片版式、figure 环绕、图注、栅格格式、封面声明与阅读器兼容性。用于图片过小、裁切、Kindle 空白、图文环绕差、缺图注，或封面/海报图片处理需要 EPUB 安全规则时。
---

# EPUB 图片版式优化

这个 skill 用于可重排 EPUB 正文中的图片。封面式全页海报布局使用 `epub-alite-converter`。

## 固定目标

生产图片版式应使用：

- JPEG / PNG 作为主要交付格式。
- `figure` 包裹浮动图片和图注。
- 百分比宽度挂在 `figure` 上，不直接 float `img`。
- wrapped figure 内部使用 `img { width: 100%; height: auto; }`。
- 封面图在 OPF 中完整声明。
- 周围有足够正文来证明环绕行为。

## Figure 模式

左/右环绕：

```html
<figure class="img-left">
  <img src="../Images/example.png" alt="图示说明"/>
  <figcaption>图注文字。</figcaption>
</figure>
```

```css
.img-left {
  float: left;
  width: 30%;
  margin: 0.2em 1em 0.6em 0;
}

.img-left img,
.img-right img {
  display: block;
  width: 100%;
  height: auto;
}
```

## 工作流

1. 读取目标 XHTML、`media.css`、`base.css`、OPF manifest 和图片资源。
2. 分类图片：
   - 封面图。
   - 正文内联图。
   - 浮动 figure。
   - 通栏 figure。
   - 图标 / 注释标记。
   - 海报背景。
   - 公式或图表 fallback。
3. 把 direct floated `img` 转成 `figure.img-left` 或 `figure.img-right`。
4. figure 宽度从 `30%` 起步；正式默认保持在 `25%` 到 `35%`，除非阅读器测试证明需要调整。
5. 保留图注和 alt。只有图片角色明确时才补 alt。
6. 图片环绕规则写进 `media.css`；通用 `figure/img` 默认写进 `base.css`。
7. 新增或删除图片时同步 OPF。
8. 面向 Kindle 时，用 JPEG / PNG 替换 WebP 主路径；必要时预栅格化风险 SVG。

## 封面规则

包封面应满足：

- manifest 中封面图片 item 带 `properties="cover-image"`。
- metadata 同步提供 `<meta name="cover" content="..."/>`，兼容 Kindle。
- Kindle 交付优先使用 JPEG / PNG 封面资源。
- Kindle 生产包不依赖 SVG-only 封面声明。

## 图片压缩边界

图片压缩、色彩空间转换和有损质量参数不在本仓实现。本 skill 只负责：

- 判断图片是否适合 EPUB/Kindle 主路径。
- 检查 OPF manifest、封面声明、figure 包装、图注和 alt。
- 在外部压缩/转码后复查路径、格式和阅读器风险。

如果用户要求压缩图片，先说明需要外部工具完成压缩，再把压缩后的资源带回本 skill 做 EPUB 结构与版式校验。

## 禁止事项

- 不在可重排主路径固定图片高度。
- 不依赖 `aspect-ratio` 作为 EPUB 图片环绕主路径。
- Kindle 主路径不使用 `em` 宽度控制 figure float。
- 不为了视觉整齐删除图注或 alt。
- 除非用户明确选择，不用截图替代真实文本。
- 不用短段落直接判定 float 失败；需要长正文 fixture，或标记为阈值反例。

## 验证 fixture

- `Text/01-body.xhtml`：基础 figure。
- `Text/03b-poster-fullbleed.xhtml`：全幅海报对照。
- `Text/09-kindle-risk.xhtml`：Kindle 图片风险页。
- `Text/17-image-layout.xhtml`：figure 环绕和大字号回归。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```
