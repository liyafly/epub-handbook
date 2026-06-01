# EPUB 是什么

如果你第一次接触电子书制作，先从这里开始。你不需要先懂排版术语，也不需要先读完整手册。

## 一句话解释

EPUB 是一个装着 HTML、CSS、图片和目录的 zip 包。电子书阅读器打开它，再根据屏幕大小和你的字号设置重新排版文字。

这也是它和 PDF 最大的区别：PDF 更像印好的纸张，EPUB 更像一个可以自动换行的网站。

## 为什么同一本书在不同阅读器里会不一样

Apple Books、Kindle、多看和其他阅读器对样式的支持不完全相同。同一本书可能在一个阅读器里正常，在另一个阅读器里出现目录失效、注释打不开、图片溢出或字号变大后页面挤坏的问题。

这个仓库要解决的是：怎样做一本在常见阅读器里都不容易崩的中文 EPUB，以及怎样把别人做坏的 EPUB 清洗干净。

## 这个仓库帮你做两件事

1. **从零学着排一本书**：先跟着 [01-first-epub.md](01-first-epub.md) 构建示例，再裁出你自己的最小书。
2. **清洗一本现成 EPUB**：按 [清洗流水线](../pipeline/cleanup-flow.md) 保留原件、检查风险、生成清洗结果，再人工对比改前改后。

## 你不需要先会什么

- 不需要先背 EPUB 规范。
- 不需要先懂所有 CSS 属性。
- 会在命令行复制粘贴命令即可。
- 遇到不懂的词，先查 [术语表](glossary.md)。

## 先看一眼正常效果

下面是同一个 demo 在 Apple Books 8.5 中的真实渲染。正文应能随着窗口和字号变化保持可读。

![Apple Books 中的普通中文正文](images/apple-books-body.png)

弹注图标点开前：

![Apple Books 中尚未点开的弹注](images/apple-books-popup-before.png)

弹注图标点开后，注释应直接在阅读页内出现：

![Apple Books 中已经点开的弹注](images/apple-books-popup-after.png)

截图的 demo 版本、阅读器版本和场景记录见 [images/README.md](images/README.md)。

## 下一步

继续阅读 [01-first-epub.md](01-first-epub.md)，5 分钟构建第一本示例 EPUB。
