# 术语表

按字母顺序。每条一句话定义 + 在本仓哪里有更详细说明。

## A

- **A-lite**：本仓自定义的「轻量增强 EPUB」配置组合。详见 [SPEC §3](../final/SPEC-实现约束.md)。
- **AZW3**：Amazon 私有 EPUB 衍生格式，KF8 时代的容器。

## C

- **container.xml**：`META-INF/container.xml`，告诉阅读器 OPF 位置；EPUB 必备文件。

## D

- **DRM**：内容加密 / 授权限制。本仓清洗流水线拒绝处理有 DRM 的 epub。

## E

- **epub:type**：EPUB 3 语义注解属性，如 `epub:type="footnote"`。
- **epubcheck**：W3C 官方 EPUB 校验器。
- **Enhanced Typesetting**：Amazon 的增强排版路径，KFX 转换后启用。

## F

- **fixed-layout**：固定版式 EPUB；本仓主路径不做 FXL。
- **footnote / 弹注**：Kindle / Apple Books 在脚注处弹出 tooltip 的能力。

## K

- **KFX**：Kindle 当前主流容器。
- **KDP**：Kindle Direct Publishing。

## M

- **manifest**：OPF 中列出所有 epub 内文件的清单。
- **MathML**：数学公式标记语言。

## N

- **NCX**：EPUB 2 时代的导航文件。
- **nav.xhtml**：EPUB 3 的目录文件。

## O

- **OPF**：EPUB 的项目文件，包含 metadata、manifest、spine。
- **OEBPS**：EPUB 内放正文的常用目录名。

## P

- **properties**：OPF manifest item 上的属性，如 `cover-image`、`svg`、`mathml`。
- **PG**：Project Gutenberg，主流公版书来源；当前首轮 demo 不依赖 PG。

## R

- **reflowable**：可重排版式。
- **Ruby / 注音**：CJK 标注，`<ruby>` 配合 `<rt>` 给汉字加注音。
- **reader-matrix**：本仓 `docs/final/reader-matrix.yaml`。

## S

- **Sigil**：开源 EPUB 编辑器。
- **spine**：OPF 中声明 EPUB 的阅读顺序。
- **SubtleCrypto**：浏览器 Web Crypto API；diff 工具用它做 SHA-256。

## T

- **Tauri**：Rust + 系统 webview 的桌面 app 框架；本仓 v1 不做。

## V

- **Validator**：校验器。本仓有 demo validator、popup validator 和 `validate_text_invariance.py`。

## X

- **XHTML**：EPUB 正文文件格式；语法上是 XML。
- **xml:lang**：标记元素语言。

## Z

- **zip.js**：本仓 web app 用的浏览器 ZIP 解析库。
