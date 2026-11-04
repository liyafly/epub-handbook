# 阅读器

[English](readers.en.md) · [指南首页](README.md)

主要照顾三个阅读器：**Apple Books、Kindle、Reeden**。在它们上面坏掉的书不能交付。Readest 是次要目标，只要求基本版正常可读。

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
- 图片用 JPEG 或 PNG。WebP 不能用于 Kindle，SVG 也不可靠，见 [SPEC §5.1](../final/SPEC-实现约束.md#51-图片格式兼容)。
- 不要用 `@media amzn-kf8` 之类的 Kindle 专用查询。需要给 Kindle 不同样式时，用下面的阅读器版本。

## Reeden

- Mac 上可以直接预览：`epub book preview 我的书 --open=Reeden`。
- 在 `html` 或 `:root` 上写 `font-size`，部分阅读器（包括 Reeden）就调不了字号。构建本来就会拦下这种写法，见 [SPEC §5.5.1](../final/SPEC-实现约束.md#551-阅读器字号缩放)。
- Reeden 有自己的扩展（出血、全屏插图等），见 [Reeden 扩展](../../references/reeden-extensions.md)。这些只放进 Reeden 版本，实测有效后再用，不放进通用版。

## Readest

只要求通用版能正常阅读。Readest 按文件内容识别书，改了书再导入，有时打开的仍是旧版本。看到旧效果时，先确认打开的是新文件，再判断是不是 CSS 的问题。

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

导入的书没有这个文件。想用阅读器版本的话，在 `epub/` 里自己加一个空的 `reader.css`：在 OPF 的 manifest 里登记它，再在每个 XHTML 里把它作为最后一个样式表链接。
