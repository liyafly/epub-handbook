---
name: epub-kindle-compatibility-checker
description: 检查 EPUB 的 Kindle/KDP 兼容风险，包括图片格式、封面 metadata、nav + NCX、MathML properties、CSS fallback、文字裁切、表格/代码溢出和转换日志 warning。用于 Kindle 交付前或 Kindle Previewer/App 与其他阅读器表现不一致时。
---

# EPUB Kindle 兼容检查

这个 skill 用于 Kindle 静态风险审核，也用于跟进 Kindle Previewer 或 KDP 转换日志。

## 风险清单

- EPUB 包中 `mimetype` 第一条且不压缩。
- OPF 声明 `nav.xhtml`；Kindle/旧工具链路径同时提供 `toc.ncx` 和 `spine toc="ncx"`。
- 封面使用 JPEG / PNG，并同时具备 `properties="cover-image"` 与旧式 cover metadata。
- 主路径图片避免 WebP。
- SVG 不作为 Kindle 关键封面或图表的唯一路径，除非已有人工验证。
- 含 MathML 的 XHTML 声明 `properties="mathml"`。
- 波浪下划线有普通 underline fallback。
- 图文环绕使用 `figure.img-left` / `figure.img-right`，不直接 float `img`。
- 表格和代码块在大字号下仍可读。
- 正文页避免 `width:100%` 加水平 padding 造成右侧裁切。
- 长 URL、文件路径、Latin identifier 可以换行。

## 工作流

1. 读取改动过的 XHTML/CSS/OPF/nav/NCX 和可用 Kindle 日志。
2. 区分静态风险和已确认的阅读器结果。
3. 对静态风险，使用对应专项 skill 修复 EPUB 源文件。
4. 对 Kindle 日志：
   - 尽量把 warning code 或 message 映射到具体资源/页面。
   - 没有阅读器测试确认时，不把被忽略的 warning 当成无害。
   - 未解决结果在 `reader-matrix.yaml` 中记录为 `warn` 或待验证。
5. 如果新行为会影响当前书以外的项目，通过 `epub-style-demo-maintainer` 新增或更新最小 fixture。

## 常见修复

- WebP warning：交付图片替换为 JPEG / PNG，WebP 只保留为源文件或现代阅读器实验。
- 封面识别缺失：补 `properties="cover-image"` 和 `<meta name="cover" content="..."/>`。
- NCX 缺失：Kindle/legacy 交付包添加 `toc.ncx`、manifest `id="ncx"` 和 `spine toc="ncx"`。
- 波浪线丢失：在 `text-decoration-style` 前保留普通 underline fallback。
- 环绕图过小：float 和百分比宽度移动到 `figure`，在 `25%` 到 `35%` 内微调。
- MathML 页面未声明：添加 manifest `properties="mathml"`，目标阅读器不支持时准备文本或图片 fallback。

## 禁止事项

- 没有 fixture、人工测试或转换日志时，不虚构 Kindle pass/fail。
- 标准 fallback 能解决时，不把 Kindle 私有 hack 作为主路径。
- 不为旧转换器删除 EPUB 3 语义，除非记录取舍。
- 不把私有 CSS 作为关键阅读内容的唯一机制。
- 除非用户要求编辑正文，不通过改 prose 解决布局问题。

## 验证 fixture

- `Text/08-long-mixed-flow.xhtml`：裁切和长 token。
- `Text/09-kindle-risk.xhtml`：Kindle 专项风险合集。
- `Text/16-math.xhtml`：MathML manifest properties。
- `Text/17-image-layout.xhtml`：figure 环绕。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

如果有 Kindle Previewer 日志，最终说明里要列出日志文件名、warning code、资源路径和测试 app/版本。

## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

调用示例：

```sh
# 预览
<skill-invocation> work/before/source.epub > work/dry-run.json

# 审查
cat work/dry-run.json | jq

# 确认后执行
<skill-invocation> --commit work/before/source.epub
```

dry-run 输出格式见 [docs/guides/epub-cleanup-flow.md](../../docs/guides/epub-cleanup-flow.md)。
