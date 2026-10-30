# 异形轮廓环绕

先选矩形回退也可读的 figure 宽度，再增加轮廓。小图、图注、短正文和大字号分别检查，不整书默认套用。硬规则见 [SPEC §5.6.2](../final/SPEC-实现约束.md#562-异形环绕渐进增强)，历史样本与证据见 [shape-wrap-demo](../../templates/shape-wrap-demo/README.md)。

```html
<figure class="wrap wrap-left outline-l">
  <img src="../Images/outline.png" alt="示意轮廓"/>
</figure>
<p>足够长的连续正文……</p>
```

```css
/* L0：无增强时仍保留普通块图。 */
figure.wrap { display: block; width: 30%; padding: 0; border: 0; margin: 1em auto; }
figure.wrap img { display: block; width: 100%; height: auto; }
/* L1：矩形回退保留文字一侧的普通间距。 */
figure.wrap-left { float: left; margin: 0 .65em 0 0; }
figure.wrap-right { float: right; margin: 0 0 0 .65em; }
/* L2：坐标只适用于此图，显式参考 content-box。 */
figure.outline-l {
  shape-outside: polygon(0% 0%, 27% 0%, 27% 72%, 100% 72%, 100% 100%, 0% 100%) content-box;
  shape-margin: .45em;
}
/* L3：窄屏护栏；20em 是起点，须按图/字号复测。 */
@media (max-width: 20em) {
  figure.wrap { float: none; margin: 1em auto; shape-outside: none; }
}
```

图像必须与 polygon 共享同一内容盒比例；右浮动需把图片及 x 坐标一起镜像。figure 不放图注，不能增加会改变参考盒的 padding/border。若不用显式 `content-box`，裸 polygon 默认采用 margin-box，须重新算坐标；不能把本例的点直接搬过去。`shape-margin` 的有效扩张受 float margin-box 裁剪，普通侧边 margin 同时保住矩形回退间距。[CSS Shapes 的盒模型与参考盒规则](https://www.w3.org/TR/css-shapes-1/#relation-to-box-model-and-float-behavior)只解释坐标，不证明任何阅读器支持。

不使用 `@supports` 改宽度，也不把 PNG 的 alpha `url()` 当可靠主路径。历史 alpha 试验在 Apple Books 退回矩形；显式 polygon 的历史 v3 在记录版本的 Apple Books、Readest 普通字号中通过，Kindle Previewer 为矩形回退。历史包不包含本例新加的 20em 护栏，因此不能把历史通过转签给整套新 CSS。实际使用后仍需普通/大字号、窄屏及主题检查，实体设备未测就保留待验证。

某次生产排版为匹配参考版而主动改回矩形，这是书级设计决定，不是轮廓失效证据。具体书名、内容定位和原始恢复材料继续留在书级记录，不写入公共配方。
