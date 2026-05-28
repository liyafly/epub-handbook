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

## 正文与资源

- `OEBPS/Text/*.xhtml`：章节正文，必须是合法 XML。
- `OEBPS/Styles/*.css`：样式。当前 demo 按 `fonts/base/notes/effects/literary/media/vertical/poster.css` 分层。
- `OEBPS/Images/*`：图片。Kindle 主路径优先 JPEG / PNG。
- `OEBPS/Fonts/*`：嵌入字体。只嵌有授权或自由协议的字体，并在 OPF manifest 声明。

## 清洗时最需要保护什么

已有 EPUB 的正文文本、核心 metadata、spine 顺序、章节锚点和封面资源是红线。具体见 [SPEC §10](../final/SPEC-实现约束.md)。
