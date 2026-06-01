# EPUB 结构剖析

EPUB 是一个有固定入口规则的 ZIP 包。先看一个典型结构：

```text
book.epub
├── mimetype
├── META-INF/
│   └── container.xml
└── OEBPS/
    ├── package.opf
    ├── nav.xhtml
    ├── toc.ncx
    ├── Text/*.xhtml
    ├── Styles/*.css
    ├── Images/*
    └── Fonts/*
```

`OEBPS/`、`Text/`、`Styles/`、`Images/` 和 `Fonts/` 是常见约定，不是标准强制目录名。但从 `mimetype` 到 `container.xml` 到 `package.opf` 这条路径是强制的——每个阅读器都从这三个文件开始认识一本书。

把它分层看：

| 层 | 负责什么 | 关键约束 |
| --- | --- | --- |
| ZIP 容器层 | 让 `.epub` 能被阅读器识别与解包 | `mimetype` 必须是第一个 entry、不压缩 |
| 入口层 | 告诉阅读器 OPF 在哪里 | `META-INF/container.xml` 只有一个 rootfile 指向 |
| 包描述层 | 登记 metadata、资源和阅读顺序 | `package.opf` 的 manifest + spine |
| 内容层 | 读者真正看到的文字、样式和资源 | XHTML + CSS + 图片 + 字体 |

下面逐层展开技术细节。EPUB2 与 EPUB3 的版本差异和兼容策略见 [08-epub2-epub3-compatibility.md](08-epub2-epub3-compatibility.md)。

## 1. ZIP 容器层：mimetype 为什么必须第一个且不压缩

`mimetype` 文件内容固定为 `application/epub+zip`（准确写法是 `application/epub+zip`，没有空格，没有换行）。

两个硬约束：

1. **必须是 ZIP 的第一个 entry。** 阅读器在解压前先按偏移量读到它，判断 "这是一个 EPUB 而不是普通 zip"。如果 mimetype 排在后面，部分阅读器会直接拒绝打开。
2. **必须 STORED（不压缩）。** 阅读器从 ZIP 中央目录或固定偏移直接读 mimetype 原始字节，不经过解压步骤。如果压缩了，阅读器拿到的是一段 deflate 数据而不是 `application/epub+zip` 字符串，校验失败。

用 `zip` 命令打包时：

```sh
# -X0：不压缩（store），去掉额外文件属性
zip -X0 book.epub mimetype
# -Xr9D：其余文件按最大压缩，不保留目录条目
zip -Xr9D book.epub META-INF OEBPS
```

诊断一个已有 EPUB 的 ZIP 结构：

```sh
# 看 entry 顺序和压缩方式：stored=不压缩，deflated=压缩
unzip -lv book.epub | head -20
```

输出第一行应看到 `mimetype`、`Stored`、大小 20 字节。

> 溯源：EPUB 3.3 spec §2.2 OCF；本仓 demo build 脚本 `templates/epub-style-demo/build.sh`。

## 2. 加载链：阅读器打开 EPUB 的 5 步

```
1. 读 mimetype → 确认是 application/epub+zip
2. 解析 META-INF/container.xml → 找到 OPF 路径
3. 解析 OPF → 建立 manifest 资源清单
4. 按 spine 顺序加载 XHTML 正文
5. 对每个 XHTML 解析 CSS、图片、字体引用并渲染
```

任何一步失败都会导致书打不开或显示异常。比如 `container.xml` 里 OPF 路径写错了 → 阅读器不知道有哪些文件 → 整本书不可读；manifest 漏声明了某张图 → 该图在所有阅读器里都不显示。

> 溯源：AGENTS.md §已有 EPUB 固定流程；`templates/epub-style-demo/META-INF/container.xml`。

### 2.1 container.xml：只有一个职责

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

可以有多个 `<rootfile>`，但 EPUB 阅读器只认 `media-type="application/oebps-package+xml"` 的那一个。`full-path` 是相对于 EPUB 根目录的路径。

## 3. OPF：三件事，每件都有坑

OPF 承担三个职责：metadata（描述书）、manifest（登记资源）、spine（阅读顺序）。

### 3.1 metadata：不止是书名和作者

```xml
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:identifier id="bookid">urn:uuid:xxx-xxx-xxx</dc:identifier>
  <dc:title>书名</dc:title>
  <dc:creator>作者</dc:creator>
  <dc:language>zh-CN</dc:language>
  <dc:date>2026-01-01</dc:date>
  <meta property="dcterms:modified">2026-01-01T00:00:00Z</meta>
</metadata>
```

关键细节：

- **`dc:identifier` 必须唯一。** `<package unique-identifier="bookid">` 中的 `bookid` 指向 `<dc:identifier id="bookid">`。Kindle 的 NCX `dtb:uid` 也要与此一致。
- **`dc:language` 影响阅读器的断字和字体选择。** 中英混排时值应为 `zh-CN`（主要语言），具体段落的语言切换用 XHTML 的 `xml:lang`。
- **`dcterms:modified` 是 EPUB 3 必备。** EPUB 2 没有这项。缺了它 epubcheck 会报 error。
- **`meta` 的 `refines` 机制：** 可以给某个 `dc:creator` 细化角色，如 `<meta refines="#creator01" property="role" scheme="marc:relators">aut</meta>`，表示作者（author）。
- **`meta property="ibooks:specified-fonts">true</meta>`：** Apple Books 专用，告诉系统"书里有嵌入字体"。需在 `<package>` 声明 `prefix="ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks-vocabulary-1.0/"`。

> 溯源：SPEC §3、§5；reader-matrix case 00-cover-metadata；demo `package.opf`。

### 3.2 manifest：资源的身份证

```xml
<manifest>
  <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  <item id="c1" href="Text/ch01.xhtml" media-type="application/xhtml+xml"/>
  <item id="css-base" href="Styles/base.css" media-type="text/css"/>
  <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  <item id="math-page" href="Text/math.xhtml" media-type="application/xhtml+xml" properties="mathml"/>
</manifest>
```

每个 `<item>` 的 `id` 在整个 OPF 内必须唯一。`spine` 和内部引用都通过 `id` 定位资源。

**`properties` 属性详解**（EPUB 3 才有，EPUB 2 不支持）：

| properties 值 | 含义 | 缺失后果 |
| --- | --- | --- |
| `nav` | 这是 EPUB 3 的 navigation document | 阅读器找不到目录；epubcheck 报 fatal |
| `cover-image` | 这是封面图（必须是 raster：JPEG/PNG） | Kindle 不识别封面；Apple Books 可能不显示 |
| `mathml` | 此 XHTML 包含 MathML | 阅读器可能不渲染公式 |
| `svg` | 此 XHTML 包含 SVG 内容 | 阅读器可能跳过 SVG 元素 |
| `scripted` | 此 XHTML 包含脚本 | 保守阅读器可能拒绝执行 |
| `remote-resources` | 此资源引用网络 URL | 离线阅读器可能阻止加载 |
| `scripted-mathml` | MathML + 脚本的组合声明 | 组合降级 |

**`media-type` 常见值**：

| media-type | 内容 |
| --- | --- |
| `application/xhtml+xml` | XHTML 正文、nav |
| `text/css` | 样式表 |
| `image/jpeg` | JPEG 图片 |
| `image/png` | PNG 图片 |
| `image/svg+xml` | SVG 图片 |
| `font/ttf` | TrueType 字体 |
| `font/opentype` | OpenType 字体 |
| `font/woff` | WOFF 字体 |
| `font/woff2` | WOFF2 字体 |
| `application/x-dtbncx+xml` | NCX 导航文件 |
| `application/mathml+xml` | 独立 MathML 文件 |

**常见故障**：
- manifest 漏声明某个文件 → 阅读器忽略它，即使文件存在于 ZIP 中
- `id` 重复 → OPF 解析失败
- `properties="cover-image"` 没写 → Kindle 不显示封面（见 reader-matrix case 00-cover-metadata）

> 溯源：SPEC §5、§5.1；reader-matrix case 00-cover-metadata、16-math。

### 3.3 spine：阅读顺序，不是文件列表

```xml
<spine toc="ncx" page-progression-direction="ltr">
  <itemref idref="c1"/>
  <itemref idref="c2"/>
  <itemref idref="c3" linear="no"/>
</spine>
```

**关键属性**：

- **`toc="ncx"`：** 指向 manifest 中 NCX 的 id，供 EPUB 2 和 Kindle 兼容。EPUB 3 主导航是 nav.xhtml，但保留 NCX 可以覆盖更多平台。
- **`page-progression-direction`：** `ltr`（从左到右）或 `rtl`（从右到左）。竖排中文书建议 `rtl`，影响阅读器的翻页方向和页面进度的视觉呈现。
- **`linear="no"`：** 该章节在默认阅读流之外（如附录、答案页、版权页）。读者仍可通过目录跳转访问，但不会顺序翻到。
- **`itemref` 的顺序即阅读顺序。** 清洗已有 EPUB 时不允许擅自重排，这是本仓的红线之一。

> 溯源：SPEC §10；reader-matrix case 21-classical-modern。

## 4. 导航双轨：nav.xhtml 与 toc.ncx 的分工

| 文件 | 谁在用 | 是不是必须 | 读者是否看到 |
| --- | --- | --- | --- |
| `nav.xhtml` | EPUB 3 阅读系统 | EPUB 3 必须 | 可以隐藏（`epub:type="toc"` 标记后阅读器不展示正文），也可以做成可见目录页 |
| `toc.ncx` | EPUB 2 阅读系统、Kindle、旧工具链 | 面向 Kindle 时建议保留 | 通常不会直接看到 |
| 正文目录页 | 读者 | 不强制 | 会——读者可以翻到并点击 |

EPUB 3 的推荐写法是：保留 `nav.xhtml`（机器导航）+ `toc.ncx`（legacy fallback）+ spine 的 `toc="ncx"`。

NCX 的 `dtb:uid` 必须与 OPF 的 `dc:identifier` 一致，否则 Kindle 可能认为这不是同一本书。

> 溯源：SPEC §5；08-epub2-epub3-compatibility.md §2.3；reader-matrix case 09-kindle-risk。

## 5. CSS 8 层分层方案：为什么这样拆

本仓 demo 模板使用 8 层 CSS（`templates/epub-style-demo/OEBPS/Styles/`）：

```
fonts.css → base.css → notes.css → effects.css → literary.css → media.css → vertical.css → poster.css
```

**设计逻辑**：

- **加载顺序即优先级。** 同选择器冲突时后加载的覆盖前加载的。`base.css` 定义正文默认值，`notes.css` 覆盖弹注样式，`poster.css` 最后加载可以覆盖所有前面的规则。
- **每层职责单一，可以按需取舍。** 一本书如果只包含普通中文正文，只需要 `base.css`；有弹注时加 `notes.css`；有竖排时再加 `vertical.css`。
- **新人最小起点就一个 `base.css`。** 从 demo 裁剪最小书时，删除不需要的 CSS 文件，同步在 OPF manifest 去掉对应 `<item>`。

每层负责什么：

| 文件 | 职责 | 什么时候需要 |
| --- | --- | --- |
| `fonts.css` | `@font-face` 声明、字体链定义 | 有嵌入字体 |
| `base.css` | 正文 body/p/h1-h6 基础样式、中英混排 | 所有书 |
| `notes.css` | 弹注结构、回跳链接、隐藏/显示逻辑 | 有脚注/弹注 |
| `effects.css` | 首字下沉、波浪线、着重号、文字效果 | 有文学效果 |
| `literary.css` | 章首页、题记、文白对照、诗段 | 有文学结构 |
| `media.css` | 图片、图注、封面、图文环绕 | 有图片 |
| `vertical.css` | 竖排正文 `writing-mode` | 有竖排 |
| `poster.css` | A-lite 海报页 | 有 A-lite 页 |

如果同时加载 `fonts.css` 和 `base.css`，`fonts.css` 的 `@font-face` 会先被注册，等 `base.css` 里引用 `font-family` 时字体已经可用。

本仓的 `city-field-notes` 自造清洗样本使用了简化的 4 文件方案（`base / media / notes / tables.css`），按书的复杂度选，两套都合法。新书从模板起步时以 8 层为参考。

> 溯源：SPEC §7；`templates/epub-style-demo/OEBPS/Styles/`；05-case-study.md §案例 1。

## 6. 字体嵌入：四条检查链

### 6.1 文件到包

字体文件必须存在于 EPUB 内并在 OPF manifest 登记：

```xml
<item id="font-title" href="Fonts/NotoSerifSC-Regular.otf" media-type="font/opentype"/>
```

### 6.2 CSS 声明

```css
@font-face {
  font-family: "MySerif";
  src: url(../Fonts/NotoSerifSC-Regular.otf) format("opentype");
  font-weight: normal;
  font-style: normal;
}

h1 { font-family: "MySerif", serif; }
```

`src` 的 `url()` 路径是**相对于 CSS 文件的位置**，不是相对于 XHTML。这意味着如果 CSS 在 `OEBPS/Styles/base.css`，字体在 `OEBPS/Fonts/font.otf`，路径应写 `../Fonts/font.otf`。

### 6.3 字体回退链

`font-family` 的值应是列表，最后一个必须是通用族：

```css
h1 { font-family: "MySerif", "Noto Serif SC", serif; }
```

当嵌入字体加载失败（文件缺失、阅读器不支持），浏览器/阅读器沿列表回退。`serif`（衬线）、`sans-serif`（无衬线）、`monospace`（等宽）是 CSS 通用族，总是可用。

### 6.4 font obfuscation

EPUB 标准提供两种字体混淆方式，用于嵌入商业授权字体时阻止直接提取：

- **IDPF 算法**：对字体文件前 1040 字节做 XOR。OPF manifest 的字体 item 标记 `<item ... properties="font-obfuscation">`，并需要 `encryption.xml`（`META-INF/encryption.xml`）记录混淆算法。
- **Adobe 算法**：已不推荐。

如果 `scripts/epub_preflight_harness.py` 报告 `encryption.xml` 存在但有字体 item 未被混淆引用，这可能表示合法字体保护或残留的旧 DRM 片段，需逐一判断。

> 溯源：SPEC §3；reader-matrix case 07-font-family-order；demo `fonts.css`。

## 7. XHTML 命名空间：声明什么、不声明会怎样

每个正文 XHTML 的根元素至少需要：

```xml
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
```

| 声明 | 作用 | 缺失后果 |
| --- | --- | --- |
| `xmlns="http://www.w3.org/1999/xhtml"` | 声明元素属于 XHTML | 阅读器可能不把元素当作 HTML 解析；XML 解析器可能报命名空间错误 |
| `xmlns:epub="http://www.idpf.org/2007/ops"` | 允许写 `epub:type` 属性 | `epub:type="noteref"` 等语义被当作无意义的自定义属性，弹注功能失效 |
| `xml:lang="zh-CN"` | 声明该文档的语言 | 影响阅读器的断字、字体选择、语音合成；中英混排时段落级切换用 `<p xml:lang="en">` |

**`xmlns:epub` 不等于 EPUB 3 升级。** 它只是声明 XML 命名空间前缀，既不改 OPF version，也不生成 nav.xhtml。EPUB 2 加了这个声明仍是 EPUB 2，只是多了一个阅读器可能识别也可能忽略的语义提示。

> 溯源：SPEC §1；08-epub2-epub3-compatibility.md §5.2–5.4。

## 8. 拿到一本陌生 EPUB 的诊断流程

快速摸底三连：

```sh
# 1. 看 ZIP 结构：mimetype 是否在第一？目录散乱程度？
unzip -l book.epub | sed -n '1,80p'

# 2. 找到 OPF 在哪
unzip -p book.epub META-INF/container.xml

# 3. 结构化诊断：DRM、加密、结构风险
python3 scripts/epub_preflight_harness.py book.epub --format json
```

`preflight_harness.py` 会输出 `preflight_status`（`pass` / `warn` / `fail`）、加密标记、manifest 完整性、缺失文件等。看到 `fail` 时不要继续往下走清洗流水线，先修结构。

> 溯源：AGENTS.md §已有 EPUB 固定流程；`scripts/epub_preflight_harness.py`。

## 9. 清洗时最需要保护什么（红线）

已有 EPUB 清洗时，以下内容不允许修改（本仓红线）：

- 正文文本内容（`validate_text_invariance.py` 比对零容忍）
- 核心 metadata（dc:identifier、dc:title、dc:creator、dc:language）
- spine 顺序
- 章节内部锚点 id
- 封面资源（图片文件和 manifest 声明）

> 溯源：SPEC §10；`scripts/validate_text_invariance.py`；`docs/pipeline/cleanup-flow.md`。

## 下一步

- 继续 [03-readers.md](03-readers.md) 选择目标阅读器
- 或者跳到 [08-epub2-epub3-compatibility.md](08-epub2-epub3-compatibility.md) 深入了解版本差异与兼容策略
- 需要术语定义时查 [glossary.md](glossary.md)
