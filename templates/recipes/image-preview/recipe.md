# 地图预览

## 效果

地图、关系图这类要放大看的单图：在多看和 Reeden 里，点图进入大图预览，题注跟着显示；其他阅读器是一张普通整宽图，题注是图下的普通文字。另有一个可选做法：在 spine 上标记，把这一页做成整屏图页。思路来自一本多看模板书的地图页。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 待测 | 待测 | 待测 | 点图预览 → 普通整宽图 + 图下题注 |

只在 Chrome 中预检过；Reeden 的预览效果还没测。

**多看注意**：一本生产书在多看 5.9.1 里套用这个结构后，整张图不显示（该书没用本配方的 `recipe.css`，配方原样未测）。在多看上使用前先实测；只想让图注居中时，不要加 `duokan-` 类名，用普通 `div` + `p` 即可。见[阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-image-preview`。

## 标记结构

见 [sample.xhtml](sample.xhtml)：

```html
<div class="duokan-image-single">
  <img src="../Images/map.jpg" alt="写清图上有什么"/>
  <p class="duokan-image-maintitle">主题注</p>
  <p class="duokan-image-subtitle">副题注</p>
</div>
```

- 一个容器只放一张图。放多张时，题注只关联第一张。
- 用 `div` + `p`，这是多看和 Reeden 文档给出的结构。`figure` / `figcaption` 能否识别没有证据，先不用。
- 用 `duokan-` 类名，多看和 Reeden 都认；Reeden 自己的 `reeden-image-single` 多看不认。类名在严格 EPUB3 里也合法。
- 主、副题注都可以省略。`alt` 必须写清图的内容，因为图上的字无法检索。

## 增强层

[recipe.css](recipe.css) 只给普通显示用：图宽 100%，题注居中、缩小，整块避免分页。点图预览由阅读器根据类名实现，不需要 CSS。

## 可选：整屏图页

地图单独占一页、不带题注时，可以在 OPF 的 spine 上标记整屏显示。页面里只放这一张图，前面不要有正文：

```html
<body><p><img src="../Images/map.jpg" alt="……"/></p></body>
```

- **非严格 EPUB3 的书**（与弹注走多看路线的书一致）：`<itemref idref="map" properties="duokan-page-fullscreen"/>`。
- **严格 EPUB3 的书**：在 `package` 上声明 `prefix="reeden: https://reeden.app/epub/vocab/#"`，写 `properties="reeden:page-fullscreen"`。不带前缀的 `duokan-` 名称在 EPUB3 校验里不合法；多看不认 `reeden:` 名称。

| 值 | 按 Reeden 文档的含义 |
| --- | --- |
| `page-fullscreen` | 整图铺满一屏，留白小于 5% 时裁满 |
| `page-fullscreen-spread` | 显示完整整图，小图保持原尺寸 |
| `page-fullscreen-spread-left` / `-right` | 只显示横幅大图的左半 / 右半 |

参考书的地图页只标了 `spread-left`，没有配对的 `-right` 页，实际显示效果没测过，先不跟。左右两页配对的做法，在不认标记的阅读器里会把同一张图显示两次，代价要单书权衡。

不认这些标记的阅读器会显示一张普通整宽图。

## 降级行为

- 不认类名：普通整宽图 + 图下两行文字，内容不丢。
- 横幅大图在竖屏上会显得小，这正是要点图预览的原因；不认预览的阅读器里，读者只能靠阅读器自带的看图功能。
- 图片保持原色，夜间模式下不反色。

## 阅读器结果

未实测。先在多看、Reeden 上确认点图预览和题注，再在 Apple Books、Kindle Previewer 上确认回退，登记到 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-image-preview`。整屏图页另记。
