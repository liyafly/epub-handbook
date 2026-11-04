# 手写最小 EPUB

本页从构建仓库自带的演示书开始，再手写一本只有一章的 EPUB，用来理解每个文件的作用。只想做书的话不需要读，直接用 `epub book new` 或 `epub book import`，见 [快速开始](../guide/quick-start.md)。

## 你需要

- 一台装了 `zip` 命令的电脑（macOS / Linux 自带；Windows 用 Git Bash 或 WSL）。
- Go 1.27 或更新版本（`go version` 检查；下载见 go.dev/dl）。
- Calibre 5+ 或 VS Code（用来做 EPUB diff review）。

## 步骤

### 1. 克隆仓库

```sh
git clone https://github.com/liyafly/epub-handbook.git
cd epub-handbook
```

### 2. 构建 CLI

```sh
go build -o /tmp/epub ./cmd/epub
/tmp/epub capabilities
```

下文示例直接写 `epub`；如果没有将 CLI 放进 PATH，可把 `epub` 换成 `/tmp/epub`，
也可以使用 `go run ./cmd/epub`。

### 3. 构建示例 EPUB

```sh
bash templates/epub-style-demo/build.sh
```

输出固定覆盖一个 `.epub` 路径：

```text
templates/epub-style-demo/dist/epub-style-demo.epub
```

### 4. 跑校验

```sh
EPUB="templates/epub-style-demo/dist/epub-style-demo.epub"
epub run epub.style.demo.maintain --input "$EPUB" --json
epub run epub.notes.popup.normalize --input "$EPUB" --json
```

退出码 0 = 通过。

### 5. 改一段文字，重新构建，再用外部 diff 工具看看改了什么

```sh
COMPARE_DIR=$(mktemp -d)
BEFORE="$COMPARE_DIR/before.epub"
AFTER="$COMPARE_DIR/after.epub"
cp templates/epub-style-demo/dist/epub-style-demo.epub "$BEFORE"

# 编辑 templates/epub-style-demo/OEBPS/Text/01-body.xhtml，改一行文字
git diff -- templates/epub-style-demo/OEBPS/Text/01-body.xhtml
bash templates/epub-style-demo/build.sh
cp templates/epub-style-demo/dist/epub-style-demo.epub "$AFTER"
```

`$BEFORE` 是修改前的副本，`$AFTER` 是本次构建的副本；模板 `dist/` 仍只保留固定产物，不累积版本。按 [EPUB diff review](../pipeline/epub-diff-review.md) 用 Calibre Editor 比较这两个文件，或在 VS Code 中解包后比较文件树。审阅完成后删除 `$COMPARE_DIR`，不要把临时 EPUB 放进模板或书级 Git。

### 6. 用阅读器打开

- **macOS**：双击文件，用 Apple Books 打开。
- **iOS**：通过 iCloud Drive 或 AirDrop 发到手机。
- **Kindle**：拖入 Kindle Previewer 3。
- **其他 EPUB 阅读器**：例如 Readest 或 Thorium；记录阅读器名称、版本和实测现象。

## 手写你自己的最小书

上面的 demo 故意包含很多场景，用来测试兼容性。它不是新书的空白模板。第一次做自己的书时，可以复制 demo，再裁成只剩一章的最小骨架。

先认识后面会反复出现的五个词：[OPF](../guide/glossary.md#opf) 是项目文件，
[manifest](../guide/glossary.md#manifest) 列资源，[spine](../guide/glossary.md#spine) 定阅读顺序，
[nav.xhtml](../guide/glossary.md#navxhtml) 是 EPUB 3 目录，[NCX](../guide/glossary.md#ncx) 是 EPUB 2
兼容导航。

### 1. 复制并裁掉演示场景

```sh
mkdir -p work-epub
cp -R templates/epub-style-demo work-epub/my-first-book
rm -rf work-epub/my-first-book/dist
find work-epub/my-first-book/OEBPS/Text -type f ! -name '01-body.xhtml' -delete
find work-epub/my-first-book/OEBPS/Styles -type f ! -name 'base.css' -delete
rm -rf work-epub/my-first-book/OEBPS/Images
```

保留后的结构应为：

```text
work-epub/my-first-book/
├── mimetype
├── META-INF/container.xml
└── OEBPS/
    ├── package.opf
    ├── nav.xhtml
    ├── toc.ncx
    ├── Styles/base.css
    └── Text/01-body.xhtml
```

demo 自带的 `GALLERY.md`、`README.md`、`SCENE_MATRIX.md`、`build.sh`、`retired/` 不参与打包，可删除。

`META-INF/container.xml` 不需要改。它仍然指向 `OEBPS/package.opf`。

### 2. 把 OPF 改成最小清单

用下面内容替换 `work-epub/my-first-book/OEBPS/package.opf`。原 demo 中除 `nav`、`ncx`、`css-base`、`body` 之外的 manifest item 都要删除；spine 中只保留 `body`。

```xml
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:replace-with-your-id</dc:identifier>
    <dc:title>我的第一本 EPUB</dc:title>
    <dc:creator>你的名字</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-06-01T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css-base" href="Styles/base.css" media-type="text/css"/>
    <item id="body" href="Text/01-body.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="body"/>
  </spine>
</package>
```

### 3. 把目录改成只剩一章

用下面内容替换 `work-epub/my-first-book/OEBPS/nav.xhtml`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN">
  <head><title>目录</title></head>
  <body>
    <nav epub:type="toc" id="toc">
      <h1>目录</h1>
      <ol><li><a href="Text/01-body.xhtml">第一章</a></li></ol>
    </nav>
  </body>
</html>
```

用下面内容替换 `work-epub/my-first-book/OEBPS/toc.ncx`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:replace-with-your-id"/></head>
  <docTitle><text>我的第一本 EPUB</text></docTitle>
  <navMap>
    <navPoint id="navpoint-1" playOrder="1">
      <navLabel><text>第一章</text></navLabel>
      <content src="Text/01-body.xhtml"/>
    </navPoint>
  </navMap>
</ncx>
```

### 4. 写第一章和基础样式

用下面内容替换 `work-epub/my-first-book/OEBPS/Text/01-body.xhtml`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
  <head>
    <title>第一章</title>
    <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  </head>
  <body>
    <h1>第一章</h1>
    <p>这是我的第一段正文。</p>
  </body>
</html>
```

用下面内容替换 `work-epub/my-first-book/OEBPS/Styles/base.css`：

```css
body {
  margin: 0 5%;
  line-height: 1.7;
}

p {
  margin: 0;
  text-indent: 2em;
}
```

### 5. 校验并打包

```sh
xmllint --noout \
  work-epub/my-first-book/OEBPS/package.opf \
  work-epub/my-first-book/OEBPS/nav.xhtml \
  work-epub/my-first-book/OEBPS/toc.ncx

cd work-epub/my-first-book
zip -X0 ../my-first-book.epub mimetype
zip -Xr9D ../my-first-book.epub META-INF OEBPS
cd ../..
epub run epub.package.nav.audit --input work-epub/my-first-book.epub --json
```

没有 error 级 finding 后，再用 Apple Books 或目标阅读器打开 `work-epub/my-first-book.epub`。

想继续理解内部结构和版本兼容，见 [结构与兼容](epub-structure-and-compat.md)。
对外硬规则在 [docs/final/](../final/)，AI 协作能力在 [skills/](../../skills/)。
