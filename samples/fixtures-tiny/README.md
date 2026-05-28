# 微型测试 fixture

为 `scripts/validate_text_invariance.py`、`tools/epub-diff/` 手动验证和后续脚本测试准备的最小 fixture 目录。

| 名称 | 覆盖的边界 |
| --- | --- |
| `empty-paragraphs/` | XHTML 含空 `<p>` 段 |
| `ruby-only/` | Ruby 注音；文本提取忽略 `<rt>` |
| `mathml-only/` | MathML 命名空间 |
| `multi-lang/` | 中英混排 + `xml:lang` |
| `one-image-one-text/` | 模拟扫描书边界 |
| `nested-tables/` | 表格嵌套 |
| `drm-marker/` | 含 fake `META-INF/encryption.xml` |

目前自动化测试在 `scripts/test_validate_text_invariance.py` 内生成等价 EPUB；本目录保留可手工扩展的 fixture 位置。
