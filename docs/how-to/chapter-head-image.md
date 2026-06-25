# 章节头图设置

> 状态：基础规则；用于普通可重排章节开头的小型装饰图、满栏横幅图、栏目图或系列标识。整页章首海报仍走 A-lite；正文图文混排仍走 `media.css`。

## 主原则

从部分书籍排版可以借鉴“轻量头图 + 真实标题 + 可重排正文”的结构。章节头图只负责气氛、系列感或栏目识别，不承载章节标题、正文信息或必须搜索的文字。

小型章标结构：

```html
<header class="chapter-header">
  <figure class="chapter-head-art">
    <img src="../Images/chapter-mark.png" alt=""/>
  </figure>
  <p class="chapter-kicker">第一章</p>
  <h1 class="decorated-chapter-title">章节标题</h1>
  <p class="chapter-subtitle">可选副标题</p>
</header>
```

满栏横幅结构：

```html
<header class="chapter-header">
  <figure class="chapter-head-banner">
    <img src="../Images/chapter-banner.png" alt=""/>
  </figure>
  <p class="chapter-kicker">第一章</p>
  <h1 class="decorated-chapter-title">章节标题</h1>
</header>
```

## 宽度与留白

- 小型章标：默认用保守宽度 `35%` 左右，同时给 `max-width`，优先保证 Kindle / 窄屏不吃掉首屏。
- 渐进增强：空间充足且已复测时，可给少数页面加增强类到 `40%` 左右；这仍是同一本 EPUB 内的类选择，不是生成第二套书。
- 满栏横幅：使用 `width:100%; max-width:100%` 铺满正文内容栏，不再给小章标那样的 `max-width`。
- 高度由源图比例决定；内层 `img` 使用 `height:auto`。如果需要更矮或更高，优先制作相应宽高比的横向源图，而不是在 CSS 里硬写高度。
- EPUB 里的“满屏宽”通常只能稳定做到“满正文内容栏宽”。真正贴到物理屏幕边缘会碰到阅读器页边距和用户排版设置，不作为通用可重排基线。
- 不用 `vh`、absolute positioning 或大段 `margin-top` 伪造固定页。
- 头图和标题之间保持紧凑，正文自然接在标题之后。

## CSS 归属

章节头图属于 `literary.css`，因为它是章首结构的一部分；不要放进 `media.css`。`media.css` 只负责正文中的 figure 环绕、图片网格和公式块。

```css
.chapter-head-art {
  margin: .8em auto .7em;
  text-align: center;
  text-indent: 0;
  page-break-inside: avoid;
}

.chapter-head-art img {
  display: block;
  width: 35%;
  max-width: 7.5em;
  min-width: 4.5em;
  height: auto;
  margin: 0 auto;
}

.chapter-head-banner img {
  display: block;
  width: 100%;
  max-width: 100%;
  height: auto;
  margin: 0 auto;
}
```

## 禁止事项

- 不把章节标题烘焙进头图；标题必须保留为真实 `h1` 文本。
- 不把普通章首改成固定版式页；需要强视觉首屏时改走 A-lite。
- 不在每章重复超大图片，避免包体增长和 Kindle 转换变慢。
- 不用图片宽度的内联属性作为长期主路径；demo 可展示，但生产规则应落 CSS 类。
- 不用同一张竖图强行拉成横幅；横幅高度应该由已裁好的源图比例控制。
- 不让头图替代 nav / NCX 文案。

## 验证

对应 demo fixture：`templates/epub-style-demo/OEBPS/Text/20-chapter-head-image.xhtml`。

验证时至少检查默认字号和大字号：

- 头图不会占满首屏。
- 横幅头图铺满正文栏但不会横向溢出。
- 标题、kicker、副标题仍是可选中文本。
- 正文不会被头图或标题挤到异常远的位置。
- 如需更醒目的章首，只对已复测页面使用增强类，不为同一内容维护两套 EPUB。
