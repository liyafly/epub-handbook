# 阅读器

[English](readers.en.md) · [指南首页](README.md)

主要照顾三个阅读器：**Apple Books、Kindle、Reeden**。在它们上面坏掉的书不能交付。Readest 是次要目标，只要求基本版正常可读。

## 看书时的字体怎么选

| 你想要 | 阅读器 | 怎么设置 |
| --- | --- | --- |
| 换成自己喜欢的字体 | Reeden、Readest | 用书的自由版，在阅读器里换字体 |
| 不换，看书里的字体 | Apple Books | 字体选“原始字体” |
| 不换，看书里的字体 | Kindle | 字体选“出版商字体”（Publisher Font） |
| 不换，看书里的字体 | 多看 | 字体选“默认字体” |

原因：Apple Books 一换字，书信、标题这些特意设计的字体也会全部被换掉。Reeden 换字后只换正文，这些字体还在。Readest 这一点还没有单独记录。

设成上面的选项后，锁定版的正文显示书里嵌入的字体。自由版没有嵌入正文字体，正文是阅读器的默认字体。制作规则见 [SPEC §8](../final/SPEC-实现约束.md#8-字体链规则)。

## 在哪些设备上看

- **iPhone**：屏幕最窄，配方会走回退写法。验收以它为准。
- **macOS**：屏幕宽，用来看增强效果。

两种都看过才算看完。iPad 不单独测试。

## Apple Books

- Mac 上双击 EPUB 即可导入；发到 iPhone 用 AirDrop 或 iCloud Drive。
- Apple Books 会缓存旧版本。重新导入前，先在书库里删除旧书。
- 每导入一次就多一个副本，所以日常预览用 Reeden 更方便，Apple Books 留到最后验收。

## Kindle

- 用 **Send to Kindle**（App、网页或邮件）发送 EPUB。亚马逊在云端把它转换成 Kindle 格式，**以设备上实际收到的书为准**。
- Kindle Previewer 只是本地的近似预检，结果和云端转换不一定一样。
- Send to Kindle 有文件大小上限，请查看亚马逊当前的说明。
- 每次构建都对每个成品跑 Kindle 静态检查，终端一行列出 error、warn、info 的数量，明细在 `.epub/report.html`。它只提示，不阻断；只有名为 `kindle` 的阅读器版本遇到 error（比如 WebP 图片）时构建才失败，`out/` 不变。`book.json` 里的 `readers` 只是备注，不影响检查。
- 图片用 JPEG 或 PNG。WebP 不能用于 Kindle，SVG 也不可靠，见 [SPEC §5.1](../final/SPEC-实现约束.md#51-图片格式兼容)。
- 只有样式不同时，优先在通用版里写 `@media amzn-kf8`，其他阅读器会跳过它。每次构建的 Kindle 检查会对它报一条警告，这是预期结果。内容也要不同时，用下面的阅读器版本。文白对照不要用这个查询，见 [SPEC §5.6.1](../final/SPEC-实现约束.md#561-文白对照左右兼容)。

## Reeden

- Mac 上可以直接预览：`epub book preview 我的书 --open=Reeden`。
- 在 `html` 或 `:root` 上写 `font-size`，部分阅读器（包括 Reeden）就调不了字号。构建本来就会拦下这种写法，见 [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放)。
- Reeden 有自己的扩展（出血、全屏插图等），见 [Reeden 扩展](../../references/reeden-extensions.md)。这些只放进 Reeden 版本，实测有效后再用，不放进通用版。

## Readest

只要求通用版能正常阅读。Readest 按文件内容识别书，改了书再导入，有时打开的仍是旧版本。看到旧效果时，先确认打开的是新文件，再判断是不是 CSS 的问题。

## 多看

不是主要目标。验收前先把“版式”恢复默认：调过间距或首行缩进后，顶格和行距都可能不对，容易误判成样式问题。没有 CSS 能单独识别多看，所以多看要特殊处理时只能单独出一个版本，例如[书信](../../templates/recipes/letter-paper/recipe.md)的无横线版。

## 一份源，多个阅读器版本

先只做一个通用版 `book.epub`，所有特殊样式都写成"先写回退，再写增强"。大多数书到这里就够了。

某个阅读器在通用版里"能看但难看"时，再给它单独出一个版本。每个版本都是"通用版 + 一层叠加"：

- 叠加层是一个文件夹，目录结构和 `epub/` 一样；
- 里面**只能放 CSS 文件和 OPF**。放了 XHTML、图片、字体或目录文件，构建会直接报错；
- 所以各版本的正文一定相同，构建还会逐字核对一遍。

在 `book.json` 里这样写：

```json
{
  "version": 1,
  "title": "我的书",
  "output": "book.epub",
  "fonts": [],
  "readers": ["apple", "kindle", "reeden"],
  "variants": {
    "kindle": { "overlay": "variants/kindle" }
  }
}
```

然后把 Kindle 要用的样式放进 `我的书/variants/kindle/OEBPS/Styles/reader.css`。构建后会得到 `out/book.epub` 和 `out/book-kindle.epub`。

- `output` 可以自定文件名，默认是 `book-<名字>.epub`。
- 某个版本写 `"fonts": []` 就不嵌字体，把字体交给阅读器；省略则沿用通用版的字体。
- `readers` 只是给人看的备注，工具不会根据它生成文件。只有 `variants` 里写了的才会单独输出。
- 任何一个版本检查失败，`out/` 里所有文件都不会更新。

### reader.css

`epub book new` 建的新书自带一个空的 `Styles/reader.css`，在每一章里最后加载。阅读器版本只要替换这一个文件，就能覆盖前面的规则。

`epub book import` 导入的书也会自动加上（放在书里已有样式表的同一目录）。之前导入的书没有它，运行 `epub book reader-layer 我的书` 补上：它在 OPF 里登记这个文件，并把它作为每一章最后一个样式表链接；有页面改不了就一个文件都不动。

不要把整份样式表复制进覆盖层再改：两份要一起维护，迟早会漏。覆盖层的 `reader.css` 只写这个版本和通用版不同的规则。

### Kindle 上显示不好的装饰页

书名页、章首页这类装饰页如果在 Kindle 上怎么调都不对，可以只给 Kindle 版换成整页图：见配方 [page-image](../../templates/recipes/page-image/recipe.md)。其他阅读器仍用文字版，正文不转图。
