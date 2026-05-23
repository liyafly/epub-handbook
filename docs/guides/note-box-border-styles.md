# 便签、边框与阴影文本框

> 状态：基础规则；用于可重排 EPUB 中的便签、摘录、提示框、资料卡、信件框和装饰边框。英文正文排版见 `english-fiction-layout.md`。

## 主原则

便签和装饰框必须保留真实文本。边框、阴影、SVG 花边和不规则边缘只提供视觉层次，阅读器忽略它们时仍然要能读。

最稳主路径：

- `border`
- `border-left`
- `background`
- `padding`
- `page-break-inside: avoid`

渐进增强：

- `box-shadow`
- `inset box-shadow`
- `outline` / `outline-offset`
- 不对称 `border-radius`
- 装饰性 SVG 边框线稿

不要在通用 Kindle 版本使用 `transform: rotate()` 旋转整块文本框。2026-05-23 用 Kindle Previewer 3.104 实测：`box-shadow`、`inset box-shadow`、`outline-offset` 可完成转换；`transform: rotate()` 会触发 KFX 增强排版内部错误。

## 样式层级

| 样式 | 用途 | 稳定性 |
|---|---|---|
| 方正框 | 普通提示、定义、编辑批注 | 最高 |
| 圆角浅底 | 温和提示、旁注 | 高 |
| 虚线框 | 草稿、待核查、可选说明 | 高 |
| 双线框 | 文献摘录、题签、复古效果 | 高 |
| 左侧竖线 | 引用、译注、非虚构提示 | 最高 |
| 实心投影 | 纸片厚度 | 中；需边框兜底 |
| 内阴影 | 内嵌资料卡 | 中；夜间模式慎用 |
| 斜角感 | 贴纸偏移感 | 中；用边框/圆角/阴影模拟 |
| 专业花边框 | 古典题签、信件框、摘录框 | 中；SVG 必须是装饰 |
| 不规则边缘 | 手贴纸、剪贴感 | 中；不要依赖 clip-path |

## 专业花边框

用户想要的专业花边框，不应把花纹放进框内正文区，也不应让角标漂在边框外。更合适的做法是把上边框和下边框做成小型 SVG 线稿：贝塞尔曲线、双横线、角部卷草和竖线端点直接相连；正文区只保留左右竖线和真实文本。不要用图片框包文字。

```html
<div class="note-box note-corner-ornament">
  <div class="note-ornate-rule note-ornate-rule-top" aria-hidden="true">
    <svg class="note-ornate-svg" viewBox="0 0 1000 120" preserveAspectRatio="none">
      <path class="note-ornate-main" d="M90 82 L90 120 ... M910 82 L910 120 ..."/>
      <path class="note-ornate-line" d="M126 30 L874 30 M126 38 L874 38"/>
    </svg>
  </div>
  <div class="note-corner-frame">
    <p class="note-title">专业花边框</p>
    <p>这里是真实文本。花边属于边框线稿，正文区仍正常重排。</p>
  </div>
  <div class="note-ornate-rule note-ornate-rule-bottom" aria-hidden="true">
    <svg class="note-ornate-svg" viewBox="0 0 1000 120" preserveAspectRatio="none">
      <path class="note-ornate-main" d="M90 0 L90 38 ... M910 0 L910 38 ..."/>
      <path class="note-ornate-line" d="M126 82 L874 82 M126 90 L874 90"/>
    </svg>
  </div>
</div>
```

```css
.note-corner-ornament {
  border: 0;
  padding: .15em 0;
  background: #fffdf8;
  box-shadow: .18em .18em 0 #ded4c2;
}

.note-ornate-rule {
  display: block;
  margin: 0;
  height: 2.55em;
  text-indent: 0;
  line-height: 0;
}

.note-ornate-svg {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.note-ornate-main,
.note-ornate-line {
  fill: none;
  stroke: #4f4539;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.note-ornate-main {
  stroke-width: 1.8;
}

.note-ornate-line {
  stroke-width: 1.15;
}

.note-corner-frame {
  margin: 0 8.9%;
  border-left: 1px solid #6f6254;
  border-right: 1px solid #6f6254;
  padding: .35em .95em .45em;
  background: #fffdf8;
}
```

这个方案把花边做成装饰性 SVG，必须加 `aria-hidden="true"`，避免朗读时读出无意义线稿。SVG 失效时，正文区仍有左右竖线、底色和真实文本；若某个发行目标不接受内联 SVG，应降级为双线框或左侧竖线框。

## 类似手绘不规则边框

可重排 EPUB 不适合用 `clip-path` 或复杂 SVG path 包正文。类似手绘纸片可以用不对称圆角、不同边宽、外轮廓和投影模拟：

```css
.note-handcut {
  border: 2px solid #5f5448;
  border-top-width: 3px;
  border-right-width: 2px;
  border-bottom-width: 3px;
  border-left-width: 2px;
  border-radius: .9em .18em .75em .28em;
  outline: 1px solid #d8ccb9;
  outline-offset: .16em;
  background: #fffdf8;
  box-shadow: .2em .25em 0 #ded4c2;
}
```

这不能做出任意凹凸轮廓，但能在 Kindle/Readest/Apple Books 中保持真实文本和稳定分页。

## Demo 补充建议

现有 demo 已覆盖正文、Ruby/弹注、竖排、海报、列表/表格/代码、fallback、长段混排、文字效果、章首、小说体、MathML、图文环绕、英文小说和便签。后续最值得补的样式不是更多装饰，而是更接近真实书稿的专项页：

- 信件/日记/手写便条：短手写体 + 边框/底色，验证字体 fallback。
- 报纸剪贴/档案卡：窄栏、标题、日期、编号和投影。
- 诗歌/戏剧：行距、悬挂缩进、speaker 和大字号。
- 大合集局部目录：总目录、本卷目录、回本卷目录链接。
- 大字号压力页：多个便签连续出现时的分页、避孤行和边框断页。
