# 无边框信纸

浅色纸面、无边框，横线逐行跟随文字。长信和短信用同一种写法。版式：

```text
亲爱的妈妈：                      顶格
□□您好！                          空两格
□□正文……                         空两格

□□祝您                            空两格
身体健康，万事如意！              顶格

                  您的儿子：小明  右对齐
                  2026年11月9日   右对齐
```

□ 只表示缩进位置。缩进和对齐全部由 CSS 完成，不在正文里写空格或方框，也不预先拆行。

## 接入

按 [sample.xhtml](sample.xhtml) 写结构。每个自然段一个 `p`，文字包在 `<span class="letter-rule">` 里：

| 元素或类 | 用途 | 效果 |
| --- | --- | --- |
| `letter-salutation` | 称呼 | 顶格 |
| 无类名 | 问候、正文 | 空两格 |
| `letter-closing` | 祝语引导 | 空两格，不与下一行分页 |
| `letter-wish` | 祝愿 | 顶格 |
| `letter-signoff` | 包住署名和日期 | 不拆页 |
| `letter-signature`、`letter-date` | 署名、日期 | 右对齐 |
| `letter-flush` | 其他要顶格的行 | 顶格 |
| `letter-right` | 其他要右对齐的行 | 右对齐 |
| `<div class="letter-gap"></div>` | 空一行 | 一条空白横线 |

`epub book recipes letter-paper --add <书目录>` 只追加 [recipe.css](recipe.css)，不需要图片。

## 字体

书信字体固定，不随正文换字。在 `fonts.css` 中按 [fonts.example.css](fonts.example.css) 绑定专用别名 `tszt-letter`，链只写 `"tszt-letter", serif`。

以后换字体只改 `fonts.json` 里这个别名指向的字体，CSS 不动。换字体后看一眼线和文字的距离，不合适就调 `.letter-rule` 的下内边距。公共样本不带字体，不算字形验收。

## 规则和限制

- 横线是行内 `span` 的下边框，逐行跟随文字，所以跨页、换字体、调行距都不会错位。段落可以拆页。
- 横线只到文字栏两侧，不到纸边。文字两端对齐，让每行线一样长；大字号下字距会拉开。
- `p` 上要同时写 `overflow: hidden` 和 `overflow: clip`。只写 `hidden` 时 Chrome 不拆长段，会裁掉内容。
- 顶格写 `0em`。阅读器的自定义版式会盖过书内样式：多看调过“版式”里的间距或首行缩进后，顶格会失效。验收时用默认版式。
- 不要把横线改成渐变。渐变版在 Readest 上段首字消失、一行只剩一两个字，原因没有查明。

## 阅读器回退

| 阅读器 | 包 | 效果 |
| --- | --- | --- |
| Apple Books、Reeden、Readest | 通用包 | 有横线 |
| Kindle | 通用包，靠 [recipe.css](recipe.css) 末尾的 `@media amzn-kf8` | 无横线，其余不变 |
| 多看 | 多看变体，叠加 [reader-duokan.css](reader-duokan.css) | 无横线，其余不变 |

多看没有 CSS 能单独识别，所以要单独出一个包。在书的 `book.json` 里登记变体，覆盖层放这份样式，并作为样式表最后加载的 `reader.css`；做法见[书级工作区](../../../docs/pipeline/book-workspace.md)的“变体”。

## 坑点

都实测踩过，改这份配方前先看：

| 做法 | 结果 |
| --- | --- |
| 横线铺在整封信或段落的背景上，段落拆页 | WebKit 里续页横线穿过文字（Apple Books、Readest 真机确认）。图片和渐变都一样，错位与画法无关 |
| 块元素加 `box-decoration-break: clone` 修跨页 | 只有 Chrome 生效，WebKit 不生效 |
| 段落不拆页来保住对齐 | 页底大片留白；超过一页的段落仍然错位 |
| 行内延伸线用 `clone` | WebKit 把 padding 算进每一行，一行只剩一个字 |
| 背景用 PNG 瓦片 | 通用包在 Kindle Previewer 上出现横穿文字的灰条 |
| 行内横线改成渐变，让多看不画线 | 多看确实不画了，但 Readest 0.12.20 段首字消失、一行一两个字。加 `slice`、加 `margin !important` 都无效，原因未查明。本地注入 `span { margin: 0 !important }` 能复现同样现象，修复却对真机无效，所以这个复现不能当作原因 |
| `p` 只写 `overflow: hidden` | Chrome 不拆长段，内容被裁掉 |
| 署名和日期固定缩进 12 字 | iPhone 默认字号下放不下，落款折行，所以改为右对齐 |
| 带着多看的自定义版式验收 | 不顶格、线距不准都是自己的间距设置造成的，曾被误判成样式问题 |

工具上：Kindle Previewer 主程序加 `-help` 会拉起界面卡住，命令行用同目录的 `KindlePreviewer4CLI`，`--output` 的目录要先建好。

## 验证状态

候选配方，尚未在两本书上验证。2026-11-09 用户确认：通用包在 Apple Books、Readest、Kindle 正常，多看变体正常；Reeden 的结果未单独记录，版本和截图也未记录。大字号、窄屏和昼夜主题没有记录。见[阅读器矩阵](../../../docs/final/reader-matrix.yaml)的 `recipe-letter-paper`。
