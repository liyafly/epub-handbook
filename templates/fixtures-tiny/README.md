# 微型测试 fixture

> 决策状态（2026-05-27，2026-08-30 随 Go 迁移更新）：本目录只作为手工扩展槽位保留，
> 不作为自动化测试输入。Go 侧自动化测试在 `internal/caps/*` 内即时构造等价 EPUB、
> 在 `testdata/` 保存 golden，避免维护两套同类样本。

为红线校验（`epub redline`）、外部 diff 工具（见
[EPUB diff review](../../docs/pipeline/epub-diff-review.md)）手动验证准备的最小
fixture 目录骨架。

| 名称 | 覆盖的边界 |
| --- | --- |
| `empty-paragraphs/` | XHTML 含空 `<p>` 段 |
| `ruby-only/` | Ruby 注音；文本提取忽略 `<rt>` |
| `mathml-only/` | MathML 命名空间 |
| `multi-lang/` | 中英混排 + `xml:lang` |
| `one-image-one-text/` | 模拟扫描书边界 |
| `nested-tables/` | 表格嵌套 |
| `drm-marker/` | 含 fake `META-INF/encryption.xml` |

目前每个子目录只保留 `.gitkeep`。后续确实需要手工 EPUB 样本时，再在对应目录补 `source/`、`build.sh` 和 `dist/` 忽略规则。

> 长期定位：本目录是**有意保留的手工 fixture 扩展槽位**，不是待补的缺口。
> 自动化回归一律在 Go 测试内即时构造等价 EPUB（parity 测试的 Python oracle
> 形状见各 `internal/caps/*/*_test.go`）。只有当某个边界确实需要手工维护的
> 实体样本时，才在对应子目录补 `source/` + `build.sh`，并把 `dist/` 加入
> `.gitignore`。
