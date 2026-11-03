# 图片环绕与并排插图

> 状态：基础规则；用于正文中的矩形图文环绕和多幅并排插图。异形轮廓环绕见 `shape-wrap.md`；章首头图见 `chapter-head-image.md`。硬规则以 [SPEC §5.6、§5.10.1](../final/SPEC-实现约束.md) 为准。

## 矩形图文环绕

图文环绕使用 `<figure>` 浮动作为主路径。Kindle App 实测 figure 也能环绕；关键是 figure 有明确百分比宽度，并且前后正文足够长，让阅读器有足够行数展示绕排。direct `img` 直挂 float 不作为主路径，避免部分阅读器图片显示过小。

```html
<figure class="img-left">
  <img src="../Images/poster.png" alt="左浮动"/>
  <figcaption>图片说明。</figcaption>
</figure>
<p>
  正文从图片右侧环绕，段落要足够长才能观察绕排。
</p>
```

```css
figure.img-left {
  float: left;
  width: 30%;
  margin: 0 1em 0.6em 0;
  text-align: center;
}

figure.img-right {
  float: right;
  width: 30%;
  margin: 0 0 0.6em 1em;
  text-align: center;
}

figure.img-left img,
figure.img-right img {
  width: 100%;
  height: auto;
}
```

宽度的默认起点是 `25%–35%`（样例取 `30%`），取值理由见 [shape-wrap 配方](../../templates/recipes/shape-wrap/recipe.md)。这不是 EPUB 标准常量，而是兼顾 Kindle App 绕排阈值与 Readest 图片可读性的保守起点：宽度越大图片越清楚但剩余文本列越窄；越小环绕越稳但图片可能偏小。`50%` 在某些设备上也能成功，是因为它仍是百分比宽度，且当时的屏幕与字号还给正文留下足够列宽；但它更接近阈值，窄屏或大字号下更容易被阅读器改排到图片下方。正式书稿按目标阅读器、屏幕宽度、用户字号和正文长度实测微调。

- 不用 `em` 做 Kindle 主路径：`em` 随用户字号放大，浮动盒和剩余列宽一起变化；百分比绑定页面宽度，大字号下更稳。
- 图片高度不固定：内层 `img` 用 `height:auto`；`aspect-ratio` 不作主路径（旧阅读器支持不稳，且 figure 还要容纳 caption 的自然高度）。
- 短段落无法证明环绕失败，测试时让正文至少有数行能贴住浮动图片。

## 不同宽高比插图并排

原书若把一张窄高图和一张宽图放在同一行，并要求图形垂直协调、图题底部对齐，不需要用 HTML table。保留两张独立 `figure`，在每张图里增加一个等高的图像区：

```xhtml
<div class="figure-pair">
  <figure class="figure figure-narrow">
    <div class="figure-stage"><img src="figure-a.png" alt="……"/></div>
    <figcaption>图 A</figcaption>
  </figure>
  <figure class="figure figure-wide">
    <div class="figure-stage"><img src="figure-b.png" alt="……"/></div>
    <figcaption>图 B</figcaption>
  </figure>
</div>
```

```css
.figure-pair { display:flex; align-items:stretch; gap:1em; }
.figure-pair .figure { display:flex; flex-direction:column; margin:0; }
.figure-stage {
  display:flex;
  flex:1 1 auto;
  align-items:center;
  justify-content:center;
}
.figure-stage img { display:block; width:100%; height:auto; }
```

图像区负责垂直居中，纵向 `figure` 让图题自然落在共同底线；窄屏下把外层改为 `display:block`。`table` 只能作为某个目标阅读器无法正确渲染 Flex、且已经实测的专用回退，并应声明 `role="presentation"`；非表格内容默认仍使用 `figure`。
