# EPUB 结构剖析

EPUB 是一个有固定入口规则的 zip 包。最小结构通常长这样：

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

`OEBPS/`、`Text/`、`Styles/`、`Images/` 和 `Fonts/` 是常见约定，不是标准强制目录名。阅读器先从 `META-INF/container.xml` 找到 OPF，再由 OPF 找到资源。

## 必备入口

- `mimetype`：必须是 zip 的第一个 entry，内容固定为 `application/epub+zip`，并且不能压缩。
- `META-INF/container.xml`：告诉阅读器 OPF 在哪里。
- `OEBPS/package.opf`：全书的项目文件，包含 metadata、manifest、spine。

## OPF 三件事

- `metadata`：标题、作者、语言、identifier、封面声明等。
- `manifest`：列出 EPUB 内所有文件；没列出来的资源可能被阅读器忽略。
- `spine`：阅读顺序；清洗已有 EPUB 时不允许擅自重排。

## 导航

- `nav.xhtml`：EPUB 3 标准目录，必须有 `<nav epub:type="toc">`。
- `toc.ncx`：EPUB 2 / Kindle 兼容目录；本仓面向 Kindle 的 demo 保留它。
- 正文目录页：读者可以翻到的一章，不等同于 `nav.xhtml` 或 `toc.ncx`。

## 正文与资源

- `OEBPS/Text/*.xhtml`：章节正文，必须是合法 XML。
- `OEBPS/Styles/*.css`：样式。当前 demo 按 `fonts/base/notes/effects/literary/media/vertical/poster.css` 分层。
- `OEBPS/Images/*`：图片。Kindle 主路径优先 JPEG / PNG。
- `OEBPS/Fonts/*`：嵌入字体。只嵌有授权或自由协议的字体，并在 OPF manifest 声明。

## 清洗时最需要保护什么

已有 EPUB 的正文文本、核心 metadata、spine 顺序、章节锚点和封面资源是红线。具体见 [SPEC §10](../final/SPEC-实现约束.md)。

继续阅读 [EPUB 结构、EPUB2 / EPUB3 与渐进兼容](08-epub2-epub3-compatibility.md)，了解两个版本的 OPF、XHTML 头、导航和弹注兼容模式。
