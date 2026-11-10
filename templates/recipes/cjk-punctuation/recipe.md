# 中文标点

## 效果

中文正文的标点更紧凑：行首开引号、括号的半格空白挤掉（`text-spacing-trim`），中文和西文、数字之间自动加一点间距（`text-autospace`），行尾句号逗号可以挂到版心外（`hanging-punctuation`）。

## 阅读器一览

| Apple Books | Kindle | Reeden | 通用版回退 |
| --- | --- | --- | --- |
| 待测 | 待测 | 待测 | 不认 → 完全不变 |

只写好合成样本，没有在任何阅读器里看过。三个属性都很新，各阅读器支持程度不同，同一阅读器可能只认其中一两个。

## 标记结构

见 [sample.xhtml](sample.xhtml)：给要收紧的容器加 `cjk-punct`。不改文字，不在中西文之间手写空格。先给一两章试，确认效果再扩大范围。

## 增强层

[recipe.css](recipe.css) 每个属性各自写在一个 `@supports` 里，只挂在可选类 `.cjk-punct` 上，不直接写到全局 `p`。

## 降级行为

不认这些属性的阅读器完全不变，按普通中文排版显示。已经在源码里手写了中西文空格的书，加 `text-autospace` 后间距可能变成两份，先删掉手写空格（这是改正文，需要授权）或不用这一条。

## 阅读器结果

尚未实测。记录到 [阅读器矩阵](../../../docs/final/reader-matrix.yaml) 的 `recipe-cjk-punctuation`。
