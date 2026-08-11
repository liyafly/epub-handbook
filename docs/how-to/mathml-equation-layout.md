# EPUB 数学公式排版：MathML + HTML/CSS

公式内容用 MathML 表达，公式在页面中的编号、并列关系和说明文字用 HTML/CSS 排布。这样可以保留数学语义，也能绕开不同阅读器对 MathML 表格布局扩展支持不一致的问题。

行内数学也使用同一原则。例如 `x + y = 12`：

```xhtml
<math xmlns="http://www.w3.org/1998/Math/MathML" display="inline">
  <semantics>
    <mrow><mi>x</mi><mo>+</mo><mi>y</mi><mo>=</mo><mn>12</mn></mrow>
    <annotation encoding="application/x-tex">x+y=12</annotation>
  </semantics>
</math>
```

可只给 `math` 设置 `font-family: "STIX Two Math", serif;`。STIX Two Math 适合数学变量、运算符和伸缩构件，但不应替代中文正文的宋体/黑体。

## 1. 公式编号

DOM 先放公式，再放编号。对于一个 EPUB 同时服务 Kindle 与 Readest、并已在目标版本实测的生产书，保守路径可以使用真实 HTML table；它只负责布局，因此声明 `role="presentation"`：

```xhtml
<table class="eq-table" role="presentation">
  <tbody>
    <tr>
      <td class="eq-formula">
        <math xmlns="http://www.w3.org/1998/Math/MathML" display="block">
          <semantics>
            <mrow><mi>E</mi><mo>=</mo><mi>m</mi><msup><mi>c</mi><mn>2</mn></msup></mrow>
            <annotation encoding="application/x-tex">E=mc^2</annotation>
          </semantics>
        </math>
      </td>
      <td class="eq-num" aria-label="公式 1">（1）</td>
    </tr>
  </tbody>
</table>
```

```css
.eq-table,
.eq-table tbody,
.eq-table tr,
.eq-table td { border: 0; }
.eq-table { width: 100%; border-collapse: collapse; }
.eq-table td { padding: 0; vertical-align: middle; }
.eq-formula { width: 100%; text-align: center; }
.eq-num { padding-left: .5em; text-align: right; white-space: nowrap; }
```

通用 demo 还可以保留线性 `div`，再用 `.eq-grid` 把公式居中、编号置右；Grid 是渐进增强，不是该生产书保守路径的前提。

不要把 `mlabeledtr` 当作跨阅读器公式编号主路径。它不属于 MathML Core 的稳定主路径，Chromium/WebKit 系阅读器可能忽略、错位或把编号挤进公式。真实 HTML table 也必须记录实测 artifact、SHA、阅读器版本和字号，不能声称“Kindle 100% 兼容”。

## 2. 方程组与并列推导

一行内有说明文字、方程组和推导结果时，外层用可换行 Flex；数学结构仍留在各自的 `<math>` 内。

```xhtml
<div class="sys-row">
  <span class="sys-label">由</span>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="inline">…</math>
  <span class="sys-label">可得</span>
  <math xmlns="http://www.w3.org/1998/Math/MathML" display="inline">…</math>
</div>
```

```css
.sys-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: .35em .65em;
  text-indent: 0;
}
.sys-row math { max-width: 100%; }
.sys-label { white-space: nowrap; }
```

方程组大括号使用普通可伸展 `<mo>{</mo>`；矩阵/分段内容放在 `mtable`。不要给 `mtable` 写固定 `width`，避免小屏和大字号下被裁切。

## 3. 长公式落在真实数据表中

真实数据表先保留正确的 `thead`、`tbody`、`th scope`、`rowspan` 和 `colspan`，不要为了
躲避分页把跨行分组拆成重复文字。公式仍放在数据单元格内的 MathML 中，再在该表的
作用域内处理宽度与字号。

单书实测可采用以下调试顺序：

1. 固定表格结构和列宽，先复测默认字号与目标大字号；
2. 若只有公式右端截断，只逐级收紧该表内 `math` 的相对字号；
3. 每次只改一个变量，不同时改变 `rowspan`、列宽与公式结构；
4. `max-content`、`overflow:hidden` 或横向滚动包装只有目标阅读器实测有效时才保留。

匿名生产样本 v3.1 曾在 Readest 0.11.20 的字号 26–27 下把表内 MathML 从
`0.88em` 调为 `0.78em` 后消除右端截断；同一精确 artifact 的真实跨行分组也在
Apple Books 8.5 与 Kindle Previewer 3.106 保持稳定。这里可复用的是“保真结构、
固定变量、按目标字号逐级测”的方法，`0.78em` 不是跨书常量。精确 SHA 与范围见
`reader-matrix.yaml` 的 `external-production-v3-1-long-math-table`。仓库内的脱敏最小
复现位于 `templates/epub-style-demo/OEBPS/Text/16-math.xhtml`；它使用独立候选字号，
在目标阅读器复测前仍只是一条 `warn` fixture。

## 4. 包声明与降级

- 每个 `<math>` 都应具有 `xmlns="http://www.w3.org/1998/Math/MathML"`。
- 校对型生产流程中，每个公式使用 `semantics`，并保存非空的 `annotation encoding="application/x-tex"`；annotation 不作为第二份可见正文。
- 含 MathML 的 XHTML 必须在 OPF manifest 声明 `properties="mathml"`。
- HTML 布局的源码顺序必须在线性降级后仍然可读，不能依赖绝对定位。
- 完全不支持 MathML 的发行目标需要文本公式或图片公式 fallback；图片公式应有等价替代文本。
- Grid/Flex/table 公式布局都需在 Kindle App/Previewer、Readest、Apple Books、Thorium 和目标字号组合上分别实测。一个生产 artifact 的通过记录不能外推成全平台通过。

对应最小场景见 `templates/epub-style-demo/OEBPS/Text/16-math.xhtml`。
