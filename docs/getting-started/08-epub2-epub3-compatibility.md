# EPUB 结构、EPUB2 / EPUB3 与渐进兼容

> 状态：入门说明；用于判断一个 EPUB 包里有哪些文件、EPUB2 和 EPUB3 的差异，以及旧阅读器兼容版应该怎样保留 fallback。
>
> 本页只解释结构与兼容策略。对外硬约束仍以 [SPEC-实现约束.md](../final/SPEC-实现约束.md) 和 [reader-matrix.yaml](../final/reader-matrix.yaml) 为准。

## 1. EPUB 不是一个 HTML 文件

EPUB 是一个有固定入口规则的 ZIP 容器。正文、样式、图片、字体和目录都放在包内，再由 OPF 统一登记。

常见目录如下：

```text
book.epub
├── mimetype
├── META-INF/
│   └── container.xml
└── OEBPS/
    ├── package.opf
    ├── nav.xhtml
    ├── toc.ncx
    ├── Text/
    │   ├── chapter-01.xhtml
    │   └── chapter-02.xhtml
    ├── Styles/
    │   ├── base.css
    │   └── notes.css
    ├── Images/
    │   └── cover.jpg
    └── Fonts/
        └── title-font.ttf
```

只有三个位置是入口规则的一部分：

1. 根目录 `mimetype`：必须是 ZIP 第一个 entry，内容固定为 `application/epub+zip`，且不压缩。
2. `META-INF/container.xml`：告诉阅读器 OPF 在哪里。
3. OPF package document：登记 metadata、manifest 和 spine。

`OEBPS/`、`Text/`、`Styles/`、`Images/` 和 `Fonts/` 都是常用约定，不是强制目录名。已有 EPUB 清洗时可以规范化目录，但必须同步改 OPF 与全部引用。

## 2. OPF、manifest、spine 和导航各管什么

### 2.1 `container.xml`

最小写法：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0"
  xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf"
      media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
```

### 2.2 OPF package document

OPF 至少承担三件事：

| 区域 | 作用 | 常见内容 |
| --- | --- | --- |
| `metadata` | 描述书 | 标题、作者、语言、identifier、修改时间 |
| `manifest` | 登记包内资源 | XHTML、CSS、图片、字体、nav、NCX |
| `spine` | 指定默认阅读顺序 | 按顺序引用 manifest item id |

`manifest` 是资源清单，不是阅读顺序；`spine` 才是阅读顺序。清洗已有书时，不要因为文件名难看就擅自重排 `spine`。

### 2.3 导航

- EPUB2 主路径：`toc.ncx`。
- EPUB3 主路径：一个 XHTML navigation document，通常命名为 `nav.xhtml`，内部至少有 `<nav epub:type="toc">`。
- 面向 Kindle 或旧阅读器的 EPUB3：保留 `nav.xhtml`，同时保留 `toc.ncx` 和 `spine toc="ncx"`。

本仓 demo 使用第三种写法：以 EPUB3 为基线，保留 NCX 作为 legacy fallback。

## 3. XHTML 不是 EPUB3 独占能力

EPUB2 已经可以使用 XHTML。EPUB2 的 OPS 规范允许 XHTML content document；EPUB3 继续使用 XHTML content document，并将其建立在 HTML 语义之上。

这两个结论要分开理解：

1. 文件扩展名写成 `.xhtml`，并不代表这本书已经是 EPUB3。
2. 能被某个阅读器显示，也不代表这个 XHTML 对目标 EPUB 版本严格有效。

判断版本时先看 OPF：

```xml
<!-- EPUB2 -->
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">

<!-- EPUB3；EPUB 3.3 的 package version 仍写 3.0 -->
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
```

EPUB2 正文通常按 XHTML 1.1 约束编写。EPUB3 可以使用更丰富的 HTML 语义，例如 `section`、`aside`、`nav`、`ruby` 和 MathML，但仍要保持 XML-valid。

## 4. EPUB2 与 EPUB3 的核心差异

| 维度 | EPUB2 | EPUB3 |
| --- | --- | --- |
| OPF `version` | `2.0` | `3.0` |
| 逻辑目录 | `toc.ncx` | XHTML navigation document |
| XHTML 能力 | XHTML 已可用，但语义集合较旧 | XHTML + 更丰富 HTML 语义 |
| CSS | 以 EPUB2 / OPS CSS 能力为基线 | 更接近现代 Web CSS，并有阅读系统支持要求 |
| Ruby、MathML、媒体 | 依赖私有实现或 fallback | 有标准化路径，但仍需阅读器实测 |
| 弹注语义 | 双向锚点 fallback | `epub:type="noteref"` + `epub:type="footnote"` |
| legacy 兼容 | 原生路径 | 可保留 NCX 和基础 CSS 作为 fallback |

EPUB3 并不意味着所有现代 CSS 在每个平台都一样工作。阅读器仍可能忽略属性、覆盖字体或在转换时降级。推荐写法始终是“基础样式可读，增强样式可丢失”。

## 5. 怎样在 EPUB2 里做渐进增强

### 5.1 先保住 EPUB2 基线

旧阅读器兼容版先使用简单结构：

```html
<p>
  正文
  <a id="note-ref-1" href="#note-1"><sup>[1]</sup></a>
</p>

<div class="footnotes">
  <p id="note-1">
    <a href="#note-ref-1">[1]</a>
    注释正文。
  </p>
</div>
```

这个结构最重要的是：

- 正文到注释有链接；
- 注释到正文有回跳；
- 注释正文仍是真实文本；
- 即使没有弹窗，读者也能完成阅读。

CSS 也采用同样原则：

```css
.wavy {
  text-decoration: underline;
  text-decoration-style: wavy;
}
```

旧引擎不认识 `text-decoration-style` 时，仍应保留基础下划线。

### 5.2 不要把 EPUB3 语义冒充成严格 EPUB2

下面是 EPUB3 标准弹注结构：

```html
<p>
  正文
  <a epub:type="noteref" role="doc-noteref" href="#note-1">[1]</a>
</p>

<aside epub:type="footnote" role="doc-footnote" id="note-1">
  <p><a epub:type="backlink" role="doc-backlink" href="#note-ref-1">◎</a>注释正文。</p>
</aside>
```

`aside` 和 `epub:type` 属于 EPUB3 语义路径。部分阅读器可能在 EPUB2 外壳里容忍或识别它们，但不能据此把这种混搭称为严格 EPUB2。

更稳妥的交付方式是：

1. 主发行包使用 EPUB3：写标准弹注、`nav.xhtml` 和 EPUB3 metadata。
2. 同一 EPUB3 包保留双向链接、基础 CSS、`toc.ncx` 和 `spine toc="ncx"`，让不识别弹窗的阅读器退化为跳转。
3. 如果目标平台明确只接受 EPUB2，再单独导出 EPUB2 fallback：保留双向链接和普通注释块，不依赖 EPUB3 语义。

### 5.3 CSS 可以增强，但不能把正文押在增强上

适合渐进增强：

- `text-decoration-style: wavy`
- `writing-mode` 与 `-webkit-writing-mode`
- `hanging-punctuation`
- 阴影、圆角和视觉边框
- 宽屏 `@media (min-width: ...)` 内的短文本 float 对照

不适合当通用主路径：

- `display:flex`、`grid` 或绝对定位承载正文
- 依赖固定屏幕宽高的正文
- 把注释、标题或正文烤进图片
- 只在单个阅读器验证过的私有 CSS

## 6. 平台兼容策略

下面的“建议”不是跨版本永久结论。具体书稿仍要按 [reader-matrix.yaml](../final/reader-matrix.yaml) 留下 reader version、artifact 和实测结果。

| 平台 | 可依赖的基础层 | 增强层如何处理 | 本仓建议 |
| --- | --- | --- | --- |
| Apple Books / iBooks | EPUB3 XHTML、CSS、`nav.xhtml`、双向链接 | Apple 官方说明 EPUB3 可用 `epub:type="noteref"` 与 `epub:type="footnote"` 触发弹注 | EPUB3 主包优先在 Apple Books 复测；重新导入前删除旧书，避免缓存 |
| Kindle / KDP | EPUB 输入、双向脚注链接、NCX、基础 CSS | 部分设备把脚注显示为弹窗；KDP 转换和 Enhanced Typesetting 仍会降级或忽略部分 CSS | 保留 NCX；必须跑 Kindle Previewer 3 和质量检查 |
| Readest | 可重排 EPUB、真实文本、基础 CSS | 阅读器允许用户调整字体、主题与版式，书内样式要允许被覆盖 | 记录 Readest 版本；重点测中文字体链、竖排、图片和大字号 |
| Readium 系阅读器 | Readium CSS 面向可重排 EPUB2 / EPUB3；Thorium 基于 Readium Desktop toolkit | 阅读系统会在作者样式与用户设置之间做平衡 | 用 Thorium 做桌面重排对照；不要把“Readium 支持”直接等同于所有下游 App 都通过 |
| KOReader | 官方支持可重排 EPUB，并允许用户覆盖字体、行距和样式 | 自定义引擎与电子墨水设备环境会放大 CSS 差异 | 使用保守 CSS；重点测目录、字体覆盖、图片、竖排降级和注释回跳 |

术语提醒：

- `iBooks` 是 Apple Books 的旧名称。
- Readium 是工具链与阅读系统生态，不是单一终端 App。
- Thorium 是本仓用于实测的 Readium 系桌面阅读器之一。
- KOReader 不是浏览器壳；不能只凭 WebKit / Chromium 经验推断效果。

## 7. 推荐发行组合

### 7.1 通用可重排书

只维护一个 EPUB3 主包：

- OPF `version="3.0"`；
- `nav.xhtml`；
- 保留 `toc.ncx` 与 `spine toc="ncx"`；
- XHTML 保持 XML-valid；
- CSS 先写基础值，再写增强值；
- 弹注同时保留 EPUB3 语义和双向链接；
- 图片优先 JPEG / PNG；
- 字体只在授权明确时嵌入。

### 7.2 必须照顾旧 EPUB2 阅读器

维护两个构建产物：

1. EPUB3 主包：标准语义 + legacy fallback。
2. EPUB2 fallback：NCX + XHTML 1.1 安全结构 + 双向链接注释 + 基础 CSS。

不要长期维护一个“既不是严格 EPUB2，也不是完整 EPUB3”的混搭包。短期兼容实验可以做，但要在 reader matrix 中明确记录目标阅读器和版本。

## 8. 最小检查清单

结构检查：

```sh
unzip -t book.epub
python3 scripts/epub_preflight_harness.py book.epub --format json
```

弹注检查：

```sh
bash scripts/validate-popup-notes.sh --epub book.epub
```

阅读器复测：

1. Apple Books：导入 EPUB3 主包。
2. Kindle Previewer 3：完成转换和质量检查。
3. Thorium：看重排、目录和用户样式覆盖。
4. Readest：看中文字体链、图片和大字号。
5. KOReader：在目标设备或模拟环境复测保守降级。

## 9. 规范与平台资料

- [W3C EPUB 3.3](https://www.w3.org/publishing/epub3/)
- [W3C EPUB 3 Overview](https://w3c.github.io/epub-specs/epub33/overview/)
- [IDPF OPF 2.0.1](https://idpf.org/epub/20/spec/OPF_2.0.1_draft.htm)
- [Apple Books Asset Guide: EPUB 3 Structure Overview](https://help.apple.com/itc/booksassetguide/en.lproj/itccdf8e5ab3.html)
- [Apple Books Asset Guide: Pop-up Footnotes](https://help.apple.com/itc/booksassetguide/en.lproj/itccf8ecf5c8.html)
- [KDP: Navigation Guidelines](https://kdp.amazon.com/en_US/help/topic/GY3AD8C6C6GAG42N)
- [KDP: Hyperlink Guidelines](https://kdp.amazon.com/en_US/help/topic/GQ6JQ7FM6C72HE4X)
- [KDP: Kindle Previewer](https://kdp.amazon.com/help/topic/G202131170)
- [Readium CSS](https://readium.org/css/)
- [Thorium Reader](https://github.com/edrlab/thorium-reader)
- [Readest documentation](https://readest.com/docs/getting-started)
- [KOReader](https://github.com/koreader/koreader)
