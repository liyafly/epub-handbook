# EPUB 结构、EPUB2 / EPUB3 与渐进兼容

> 状态：入门说明；用于判断一个 EPUB 包里有哪些文件、EPUB2 和 EPUB3 的差异，以及旧阅读器兼容版应该怎样保留 fallback。
>
> 本页只解释结构与兼容策略。对外硬约束仍以 [SPEC-实现约束.md](../final/SPEC-实现约束.md) 和 [reader-matrix.yaml](../final/reader-matrix.yaml) 为准。

## 0. 先记住三个判断

第一次拆 EPUB 时，先把三个问题分开：

1. **这是什么版本的包？** 看 OPF 根元素的 `version`，不是看文件扩展名。
2. **正文文件能写什么？** EPUB2 和 EPUB3 都可以使用 XHTML；但两代标准允许的元素、属性和导航方式不同。
3. **阅读器最终会怎样显示？** 标准合规、阅读器容错和平台私有兼容是三件不同的事。某个阅读器能弹窗，不等于所有阅读器都会弹，也不等于 EPUB2 校验器会接受这份混合写法。

本页先讲结构，再讲版本，最后讲“EPUB2 外壳中尝试 EPUB3 弹注语义”的兼容模式。需要直接照着改文件时，看 [EPUB2 外壳中的 popup note 兼容写法](../guides/epub2-popup-note-compatibility.md)。

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

把它想成四层：

| 层 | 负责什么 | 典型文件 |
| --- | --- | --- |
| ZIP 容器层 | 让 `.epub` 可以被阅读器解包 | `mimetype` |
| 入口层 | 告诉阅读器 OPF 在哪里 | `META-INF/container.xml` |
| 包描述层 | 登记书籍 metadata、资源和阅读顺序 | `package.opf` 或 `content.opf` |
| 内容层 | 真正给读者看的正文、目录、样式和资源 | `Text/*.xhtml`、`Styles/*.css`、`Images/*` |

只有三个位置是入口规则的一部分：

1. 根目录 `mimetype`：必须是 ZIP 第一个 entry，内容固定为 `application/epub+zip`，且不压缩。
2. `META-INF/container.xml`：告诉阅读器 OPF 在哪里。
3. OPF package document：登记 metadata、manifest 和 spine。

`OEBPS/`、`Text/`、`Styles/`、`Images/` 和 `Fonts/` 都是常用约定，不是强制目录名。已有 EPUB 清洗时可以规范化目录，但必须同步改 OPF 与全部引用。

拿到一本陌生 EPUB，可以先运行：

```sh
unzip -l book.epub | sed -n '1,80p'
unzip -p book.epub META-INF/container.xml
python3 scripts/epub_preflight_harness.py book.epub --format json
```

不要直接解包后改几个文件再压回去。重新打包时，`mimetype` 的顺序和压缩方式很容易被破坏。

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

一个简化的 EPUB3 OPF 如下：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf"
         version="3.0"
         unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:example</dc:identifier>
    <dc:title>示例书</dc:title>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-05-31T00:00:00Z</meta>
  </metadata>

  <manifest>
    <item id="nav" href="nav.xhtml"
          media-type="application/xhtml+xml"
          properties="nav"/>
    <item id="ncx" href="toc.ncx"
          media-type="application/x-dtbncx+xml"/>
    <item id="c1" href="Text/chapter-01.xhtml"
          media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/base.css"
          media-type="text/css"/>
  </manifest>

  <spine toc="ncx">
    <itemref idref="c1"/>
  </spine>
</package>
```

一个简化的 EPUB2 OPF 则通常是：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf"
         version="2.0"
         unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:example</dc:identifier>
    <dc:title>示例书</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>

  <manifest>
    <item id="ncx" href="toc.ncx"
          media-type="application/x-dtbncx+xml"/>
    <item id="c1" href="Text/chapter-01.xhtml"
          media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/base.css"
          media-type="text/css"/>
  </manifest>

  <spine toc="ncx">
    <itemref idref="c1"/>
  </spine>
</package>
```

两段最直观的差异是：

- EPUB2 写 `version="2.0"`，以 `toc.ncx` 为导航主路径。
- EPUB3 写 `version="3.0"`，必须登记带 `properties="nav"` 的 XHTML navigation document。
- EPUB3 仍可以保留 NCX，给 Kindle 和旧工具链做 fallback。
- EPUB3 常见 `dcterms:modified`；EPUB2 没有这项 EPUB3 必备 metadata。

### 2.3 导航

- EPUB2 主路径：`toc.ncx`。
- EPUB3 主路径：一个 XHTML navigation document，通常命名为 `nav.xhtml`，内部至少有 `<nav epub:type="toc">`。
- 面向 Kindle 或旧阅读器的 EPUB3：保留 `nav.xhtml`，同时保留 `toc.ncx` 和 `spine toc="ncx"`。

本仓 demo 使用第三种写法：以 EPUB3 为基线，保留 NCX 作为 legacy fallback。

初学者容易把“正文目录页”和“机器导航文件”混在一起：

| 名称 | 读者是否会看到 | 作用 |
| --- | --- | --- |
| 正文目录页 | 通常会 | 书内一章，读者可以翻到并点击 |
| `toc.ncx` | 通常不会直接看到 | EPUB2 和旧工具链读取的机器目录 |
| `nav.xhtml` | 可以隐藏，也可以做成可见页面 | EPUB3 阅读系统读取的机器导航 |

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

### 3.1 EPUB2 的最小正文头

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml"
      xml:lang="zh-CN">
<head>
  <title>第一章</title>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
</head>
<body>
  <h1>第一章</h1>
  <p>正文。</p>
</body>
</html>
```

### 3.2 EPUB3 的最小正文头

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>第一章</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>第一章</h1>
    <p>正文。</p>
  </section>
</body>
</html>
```

这里有两个不同的 namespace：

| 声明 | 作用 |
| --- | --- |
| `xmlns="http://www.w3.org/1999/xhtml"` | 说明正文元素属于 XHTML |
| `xmlns:epub="http://www.idpf.org/2007/ops"` | 声明 `epub:` 前缀，允许写 `epub:type` |

`xmlns:epub` 不是某种脚本，也不会自动升级 OPF。它只是在 XML 中声明 `epub:` 前缀代表哪个命名空间。

## 4. EPUB2 与 EPUB3 的核心差异

| 维度 | EPUB2 | EPUB3 |
| --- | --- | --- |
| OPF `version` | `2.0` | `3.0` |
| 逻辑目录 | `toc.ncx` | XHTML navigation document |
| XHTML 能力 | XHTML 已可用，但语义集合较旧 | XHTML + 更丰富 HTML 语义 |
| CSS | 以 EPUB2 / OPS CSS 能力为基线 | 更接近现代 Web CSS，并有阅读系统支持要求 |
| Ruby、MathML、媒体 | 依赖私有实现或 fallback | 有标准化路径，但仍需阅读器实测 |
| 弹注语义 | 双向锚点 fallback | `epub:type="noteref"` + `epub:type="footnote"` |
| 内容语义 | 结构表达能力较弱 | 可用 `epub:type` 标记 chapter、bodymatter、footnote 等 |
| metadata | OPF2 metadata 模型 | OPF3 metadata + refinement；要求 `dcterms:modified` |
| manifest 特征声明 | 较少 | `properties="nav"`、`cover-image`、`mathml`、`svg` 等 |
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

### 5.2 你记得的“头文件”是什么

历史 EPUB 制作实践里，常见一种兼容写法：保留 OPF `version="2.0"`，但在需要弹注的 XHTML 根元素上增加 EPUB namespace：

```xml
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
```

然后给正文链接和注释目标增加 EPUB3 弹注语义：

```html
<p>
  正文
  <a id="note-ref-1"
     epub:type="noteref"
     href="#note-1">[1]</a>
</p>

<aside id="note-1" epub:type="footnote">
  <p><a href="#note-ref-1">◎</a>注释正文。</p>
</aside>
```

这正是 `_epub_reference/epub-guide/OEBPS/Text/Chapter10-6.xhtml` 记录的历史经验。Apple Books 的 EPUB3 官方弹注说明也要求：使用 `epub:type` 时，必须在 `<html>` 上声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。

但要注意三件事：

1. 加 namespace **不会**把 OPF `version="2.0"` 升级成 EPUB3。
2. EPUB2 的 OPS XHTML 基线来自 XHTML modules；HTML5 的 `<aside>` 不属于严格 EPUB2 主路径。
3. 阅读器是否在 EPUB2 外壳里识别这套语义，属于目标阅读器兼容行为，必须实测。

因此，本仓把它称为 **EPUB2 外壳中的 popup note 兼容模式**，不称为严格 EPUB2 标准弹注。

### 5.3 两种 EPUB2 兼容变体

| 变体 | 注释目标 | 优点 | 代价 |
| --- | --- | --- | --- |
| 保守跳转版 | `<div id="note-1">` 或 `<p id="note-1">` | 更接近 EPUB2 可理解元素；所有阅读器至少可跳转 | 注释通常仍显示在正文流里；弹窗识别依赖阅读器 |
| 混合弹窗版 | `<aside id="note-1" epub:type="footnote">` | 支持该兼容行为的阅读器可隐藏正文注释并弹窗 | 不是严格 EPUB2 XHTML；必须跑校验和目标阅读器实测 |

无论选哪种，都必须保留：

- 正文到注释的 `href`；
- 注释到正文的回跳；
- 真实注释文本；
- 不支持弹窗时仍可完成阅读的 fallback。

完整可复制模板见 [EPUB2 外壳中的 popup note 兼容写法](../guides/epub2-popup-note-compatibility.md)。

### 5.4 不要把 EPUB3 语义冒充成严格 EPUB2

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

### 5.5 CSS 可以增强，但不能把正文押在增强上

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
- 不要把 Apple Books 官方 EPUB3 弹注说明外推成“所有 EPUB2 包、所有平台都能弹注”。EPUB2 外壳兼容模式必须单列 reader version 和 artifact。

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

不要长期维护一个“既不是严格 EPUB2，也不是完整 EPUB3”的混搭包。短期兼容实验可以做，但要在 reader matrix 中明确记录目标阅读器、版本、artifact 和是否通过 epubcheck。

## 8. 从 EPUB2 升级到 EPUB3 时改什么

最小迁移清单：

1. OPF `version="2.0"` 改为 `version="3.0"`。
2. 补齐 EPUB3 metadata，特别是 `dcterms:modified`。
3. 新建 XHTML navigation document，在 manifest 标记 `properties="nav"`。
4. 保留原 `toc.ncx` 作为 legacy fallback，继续在 spine 写 `toc="ncx"`。
5. XHTML 根元素需要使用 `epub:type` 时，补 `xmlns:epub="http://www.idpf.org/2007/ops"`。
6. 把普通脚注改成双向链接保底的标准弹注结构。
7. 含 MathML 的 XHTML 在 OPF manifest 声明 `properties="mathml"`。
8. 运行 preflight、validator、epubcheck 和目标阅读器实测。

已有书不要手工批量替换。优先走：

```sh
python3 scripts/epub_cleanup_pipeline.py input.epub --work-dir work
```

## 9. 新人最容易踩的坑

| 误区 | 正确理解 |
| --- | --- |
| 文件叫 `.xhtml`，所以一定是 EPUB3 | EPUB2 也用 XHTML；版本看 OPF |
| 文件夹必须叫 `OEBPS/Text/Styles` | 这些只是约定；入口与引用正确才是关键 |
| manifest 顺序就是阅读顺序 | 阅读顺序看 spine |
| 有 `nav.xhtml` 就可以删 NCX | EPUB3 标准主路径是 nav；面向 Kindle 和旧工具链时保留 NCX |
| 加 `xmlns:epub` 就完成 EPUB3 升级 | 它只声明 XML 前缀；OPF、nav、metadata 仍要迁移 |
| Apple Books 能弹窗，所以所有平台都能弹 | 平台能力不同；必须保留跳转 fallback 并实测 |
| CSS 在浏览器有效，阅读器也一定有效 | EPUB 阅读器有自己的 CSS 子集和用户样式覆盖 |

## 10. 最小检查清单

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

## 11. 规范与平台资料

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

## 12. EPUB 标准演进与平台分化史

下面从时间线角度解释"为什么同一本书在不同阅读器会不一样"——不是看图说话，而是理解每个平台当初做了什么选择、这些选择今天以什么形式影响你做的 EPUB。

### 12.1 时间线

```
2007  IDPF 发布 EPUB 2.0
      → OPF + NCX + XHTML 确立为基本架构
      → Sony Reader、早期 Nook 等设备采用
2010  苹果发布 iBooks (后改名 Apple Books)
      → iPad 发布，EPUB 从电子墨水走向全彩触屏
      → 苹果选择拥抱 EPUB 标准（而不是自建格式），推动 EPUB 3 的诞生
2011  IDPF 发布 EPUB 3.0
      → nav.xhtml、epub:type 语义、MathML、Ruby、CSS 3 子集加入
      → 苹果是 EPUB 3 的第一批支持者
2011  Amazon 发布 Kindle Format 8 (KF8 / AZW3)
      → 基于 EPUB 3 的容器，但做了一套 Amazon 私有扩展
      → Kindle 继续走自建生态路线，不原生支持 EPUB 3
2014  EPUB 3.0.1 维护修订
2015  Amazon 推出 Enhanced Typesetting（增强排版引擎）和 KFX 格式
      → 排版能力提升，但 CSS 子集更保守（不支持 float、部分 text-decoration 等降级为普通样式）
2017  IDPF 与 W3C 合并
      → EPUB 标准从出版业主导转向 Web 标准主导
2023  W3C 发布 EPUB 3.3 推荐标准
      → 清理过时引用，更贴近现代 Web 标准
      → 但阅读器实现更新远比标准慢，大量存量设备仍跑旧引擎
```

### 12.2 三条路线

三条路线一直并行至今：

**路线一：Apple Books — 标准路线的标杆**

苹果从 2010 年起就是 EPUB 标准的积极推动者。Apple Books（原 iBooks）的渲染引擎基于 WebKit，因此对 CSS 3 的支持在阅读器中最为完整：`text-decoration-style`、`writing-mode`、`box-shadow`、`hanging-punctuation` 等都能正常渲染。但它有两个特点值得注意：
- iBooks 时代引入的 `ibooks:specified-fonts` 等私有 metadata 至今仍被 Apple Books 识别——这是苹果在标准之外加的"厂商前缀"。
- Apple Books 的强缓存机制曾导致无数制作者以为"改了没生效"。

**路线二：Amazon Kindle — 自建生态，输入 EPUB 但输出不是**

Amazon 从未让自己的设备原生支持 EPUB。作者上传 EPUB，由 Kindle Previewer 内部转换为 KFX（或转 MOBI / KF8），再分发到 Kindle 设备。这意味着：
- 你写的 CSS 不直接送达读者，而是经过一轮转换。
- Amazon 的转换器有自己的 CSS 子集：不支持的属性被静默丢弃或降级。
- 导航方面，Kindle 主要依赖 NCX，不依赖 `nav.xhtml`。
- Enhanced Typesetting（KFX 的排版引擎）改善了连字符、字距和换页，但进一步收窄了支持的 CSS 范围。

这是 Kindle 兼容性问题比其他平台更频繁的根本原因。

**路线三：中文阅读器 — 起步晚、需求特殊、兼容机制被滥用**

多看阅读、微信读书等中文平台面临特有的中文排版需求：竖排、Ruby 注音、弹注、中英混排字距。它们在 EPUB 2 基础上叠加了私有兼容机制（如多看的 `duokan-footnote` class），在 EPUB 2 外壳里做 EPUB 3 才有的功能。

这产生了大量"既不严格 EPUB 2 也不完整 EPUB 3"的混搭包，客观上增加了中文 EPUB 生态的碎片化。本仓的立场是：主包做标准 EPUB 3 + legacy fallback，多看兼容作为增强层单独叠加，不混在标准路径里。

### 12.3 开源生态

EPUB 的开源工具链主要围绕 Readium 项目展开：

- **Readium SDK / Readium Desktop**：EPUB 3 渲染和解析的开源实现，Thorium Reader 基于它构建。
- **KOReader**：面向电子墨水设备（Kobo、Kindle 越狱等）的开源阅读器，有自己的渲染引擎，支持 EPUB 但不走 Readium 路线。CSS 策略更保守，适合作为"最低兼容"对照。
- **Readest**：较新的跨平台阅读器（Tauri + 系统 WebView），对中文 EPUB 体验做了专门优化。

> 溯源：reader-matrix.yaml；本仓 demo 的 reader-matrix 以 Apple Books / Kindle Previewer / Thorium / Readest / KOReader 为实测基线。

本地参考材料 `_epub_reference/epub-guide/OEBPS/Text/Chapter10-6.xhtml` 记录了历史兼容经验，但 `_epub_reference/` 不进入 git，也不替代标准文档和 reader matrix 实测。
