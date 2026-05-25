# 文白对照与双文本排版

> 状态：基础规则；用于文言/白话对照、原文/译文对照、双语短篇和带出处的古籍条目。超大合集导航见 `anthology-navigation.md`。

## 主原则

文白对照的最小单元是“条目”，不是段落表格。每个条目应保留真实标题、稳定 `id`、原文段落、白话段落和出处/注记。正文顺序推荐为标题 → 出处 → 原文 → 白话 → 回本页目录。

原文和译文可以做左右对照，但主路径必须仍是可重排文本：先按源序写成上下相邻，再用 `float` 把每组原文/译文增强为左右两栏。阅读器不支持 `float`、屏幕太窄或字号过大时，应自然退回上下显示。不要用 `table`、`display:flex`、`grid` 或固定页面尺寸承载正文；这些路径在 Kindle 转换、辅助功能和大字号下都更容易失败。

## 推荐结构

```html
<nav epub:type="toc" id="classical-modern-toc" class="classical-modern-local-toc">
  <h2>本页条目</h2>
  <ol>
    <li><a href="#entry-01">条目标题</a></li>
  </ol>
</nav>

<section class="parallel-entry" id="entry-01">
  <h2 class="parallel-entry-title">条目标题</h2>
  <p class="parallel-source">出处：示例出处。</p>

  <section class="parallel-pair parallel-float-pair">
    <div class="parallel-col parallel-col-classical">
      <p class="parallel-label">原文</p>
      <p class="classical-text book-song">文言原文。</p>
    </div>
    <div class="parallel-col parallel-col-modern">
      <p class="parallel-label">白话</p>
      <p class="modern-text book-kai">白话译文。</p>
    </div>
    <div class="parallel-clear" aria-hidden="true"></div>
  </section>

  <p class="parallel-return">
    <a href="#classical-modern-toc" title="回本页条目" aria-label="回本页条目">(•)</a>
  </p>
</section>
```

最小 CSS：

```css
.parallel-pair {
  clear: both;
}

.parallel-col-classical,
.parallel-col-modern {
  width: auto;
}

.parallel-float-pair .parallel-col-classical {
  float: left;
  width: 37%;
  margin-right: 5%;
}

.parallel-float-pair .parallel-col-modern {
  overflow: hidden;
  width: auto;
}

.parallel-clear {
  clear: both;
  height: 0;
  font-size: 0;
  line-height: 0;
}
```

如果每组两侧都只有一个段落，也可以直接把 `float` 写在原文段落上，让译文段落保持普通块并用 `overflow:hidden` 形成右侧块；多段原文、多段译文或需要标签时，推荐用 `.parallel-col` 包裹每侧文本。默认 `.parallel-col-*` 必须保持全宽 block，只有加 `.parallel-float-pair` 的组才进入左右增强，避免阅读器不支持或挤不下时出现“半宽上下堆叠”。当前推荐起点是左侧原文 `float:left; width:37%; margin-right:5%`，白话列不再 float，而是以普通 block 占据右侧剩余宽度；这比两列同时 float 更不容易在 Kindle 分页器里把第二列挤到下一块。不要依赖 `::after` clearfix 作为唯一清除方式，KF8 对伪元素选择器支持不稳；显式 `.parallel-clear` 更容易在 Kindle 路径中保留。

有些 Kindle 专用 AZW3/MOBI 成品会用 `table-layout: fixed` 和左右 `td` 做英汉/文白对照；这能解释为什么 Kindle 里可以见到成功的左右对照。但对 EPUB 源文件和 KDP 上传路径，不把 table 作为推荐主路径：Amazon 质量规则长期把非表格正文塞进 table 视为风险，且大字号、辅助技术和窄屏更容易变差。只有明确只交付 Kindle 专用 AZW3、并且已经在目标设备逐页验收时，才把 table 当作专用例外。

## CSS 归属

文白对照属于文学结构，样式放在 `literary.css`。字体链不要在 `literary.css` 里重复声明；原文和译文通过 `fonts.css` 的 `.book-song` / `.book-kai` 等工具类组合。

推荐职责：

- `.parallel-entry`：条目间距和分隔线。
- `.parallel-entry-title`：条目标题。
- `.parallel-source`：出处、卷次、校注来源。
- `.parallel-pair`：一组原文/译文对照，默认全宽上下。
- `.parallel-float-pair`：启用 float 左右增强。
- `.parallel-col-classical` / `.parallel-col-modern`：左右列；默认全宽，增强态下原文列 `float:left` 并保留约 5% 余量，白话列作为普通块占右侧剩余宽度。
- `.parallel-clear`：显式清除浮动，避免依赖伪元素。
- `.parallel-label`：原文/白话标签。
- `.classical-text`：文言段落节奏。
- `.modern-text`：白话段落节奏。
- `.parallel-return`：回本页条目链接。

## 大部头做法

- 按卷、篇、部类或作品集拆 XHTML，不把整本压进一个文件。
- 主 nav / NCX 可以覆盖卷级入口；检索型古籍可进一步覆盖条目级锚点。
- 每卷内保留局部目录，条目末尾可用 `(•)` 回本卷或本页目录。
- 条目 id 应由 source 稳定生成；临时工具 id 只适合一次性样本。
- 原文、白话、注记都保留为可选中文本，不用图片承载。
- Kindle 目标版本不要把左右对照挂在 `@media (orientation: landscape)` 或 `display:flex` 上；应把 `float` 作为基础增强，让不支持时自动按源序上下排列。

## 验证清单

- 默认字号下，支持 `float` 的阅读器可显示左右对照。
- 大字号或窄屏下，原文与白话仍能连续阅读，允许退回上下显示。
- 不横向滚动，不依赖 table、flex、grid 或固定版式。
- nav / NCX / 局部目录链接都能定位到条目。
- 原文、白话和出处角色清晰，但不依赖颜色表达唯一信息。
- 原文和译文字体通过 `fonts.css` 工具类组合，默认不新增嵌入字体。

对应 demo fixture：`templates/epub-style-demo/OEBPS/Text/21-classical-modern.xhtml`。
