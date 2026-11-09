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

- 横线画在行内 `span` 上，逐行跟随文字，所以跨页、换字体、调行距都不会错位。段落可以拆页。
- 横线只到文字栏两侧，不到纸边。文字两端对齐，让每行线一样长；大字号下字距会拉开。
- 横线用 CSS 渐变，不用 `border`：多看不支持渐变，不画线，而 `border` 在多看里显示不正常。
- `p` 上要同时写 `overflow: hidden` 和 `overflow: clip`。只写 `hidden` 时 Chrome 不拆长段，会裁掉内容。
- 顶格写 `0em`。阅读器的自定义版式会盖过书内样式：多看调过“版式”里的间距或首行缩进后，顶格会失效。验收时用默认版式。

## Kindle 回退

同一个包覆盖所有阅读器，不需要 Kindle 变体。[recipe.css](recipe.css) 末尾的 `@media amzn-kf8` 在 Kindle 去掉横线，纸面、文字、缩进和落款位置都保留。其他阅读器不认识这个媒体类型，会跳过它；如果某个阅读器不跳过，结果也只是没有横线。

## 验证状态

候选配方。见[阅读器矩阵](../../../docs/final/reader-matrix.yaml)的 `recipe-letter-paper`。用户已确认 `border` 版在多看以外的观感；改成渐变后，Apple Books、Readest、Reeden、多看和 Kindle 都要再看一次。大字号、窄屏和昼夜主题没有记录。
