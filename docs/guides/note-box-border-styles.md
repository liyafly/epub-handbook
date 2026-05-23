# 便签、边框与阴影文本框

> 状态：基础规则；用于可重排 EPUB 中的便签、摘录、提示框、资料卡、信件框和装饰边框。英文正文排版见 `english-fiction-layout.md`。

## 主原则

便签和装饰框必须保留真实文本。边框、阴影和不规则边缘只提供视觉层次，阅读器忽略它们时仍然要能读。

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
| 不规则边缘 | 手贴纸、剪贴感 | 中；不要依赖 clip-path |

生产推荐顺序是：普通方框或左侧竖线优先；需要题签感时用双线框；需要纸片层次时再加阴影；手贴纸感用不对称圆角、外轮廓和投影模拟。不要把复杂花边作为通用 EPUB 的默认边框。

## SVG 花边实验

`19-border-shadow-notes.xhtml` 中的 SVG 花边样例只用于验证“简单内联 SVG path 能否作为装饰边线被 Readest、Apple Books 和 Kindle Previewer 转换链路接受”。它不是生产推荐边框。

实验结论：

- 可行：小型内联 SVG、`path`、贝塞尔曲线、双横线和 `aria-hidden="true"` 可用于装饰性边线。
- 必须保留真实文本：SVG 只能画边框或角部花纹，不能承载正文。
- 必须可降级：SVG 失效时，正文区还要有底色、左右竖线或普通边框可读。
- 不作为默认推荐：SVG 花边维护成本高，细节依赖阅读器缩放、主题和抗锯齿；普通书稿优先用双线框、左侧竖线或浅底投影。
- 仅在强设计需求中考虑：例如古典题签、请柬式引文、少量扉页题记。使用前必须在目标阅读器和大字号下复测。

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
