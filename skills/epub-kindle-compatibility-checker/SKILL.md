---
name: epub-kindle-compatibility-checker
description: 检查 EPUB 的 Kindle/KDP 兼容风险，包括图片格式、封面 metadata、nav + NCX、MathML properties、CSS fallback、文字裁切、表格/代码溢出和转换日志 warning。用于 Kindle 交付前或 Kindle Previewer/App 与其他阅读器表现不一致时。
---

# EPUB Kindle 兼容检查

## 何时用

- Kindle 交付前的静态风险审核，以及跟进 Kindle Previewer 或 KDP 转换日志。
- 静态风险清单（逐项核对）：包内 `mimetype` 第一条且不压缩；OPF 声明 `nav.xhtml`，Kindle/旧工具链路径同时提供 `toc.ncx` 和 `spine toc="ncx"`；封面用 JPEG/PNG 并同时具备 `properties="cover-image"` 与旧式 cover metadata；主路径图片避免 WebP；SVG 不作为 Kindle 关键封面或图表的唯一路径（除非已有人工验证）；含 MathML 的 XHTML 声明 `properties="mathml"`；波浪下划线先写普通 underline fallback；图文环绕用 `figure.img-left` / `figure.img-right`，不直接 float `img`；表格和代码块在大字号下仍可读；正文页避免 `width:100%` 加水平 padding 造成右侧裁切；长 URL、文件路径、Latin identifier 可以换行。
- 禁止事项：没有 fixture、人工测试或转换日志时不虚构 Kindle pass/fail；标准 fallback 能解决时不用 Kindle 私有 hack 作主路径；不为旧转换器删除 EPUB 3 语义（除非记录取舍）；不把私有 CSS 作为关键阅读内容的唯一机制；除非用户要求编辑正文，不通过改 prose 解决布局问题。

## 调什么

本 skill 是 AI 分析与手工精排类 skill：读改动过的 XHTML/CSS/OPF/nav/NCX 和可用 Kindle 日志，区分静态风险与已确认的阅读器结果，再按上述清单审核。改书后必须跑校验组合：

```sh
epub run epub.notes.popup.normalize --input <产物> --json    # 涉及弹注时
epub run epub.style.demo.maintain --input <demo 产物> --json # 涉及 demo 模板时
epub redline --check all <before.epub> <after.epub>          # 每次改书后
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`（noteref 数）、`text_files`（XHTML 文件数）、`violations`（结构违反数），violations 对应 `error popupnotes` findings。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过。
- Kindle 转换日志没有统一信封：整理日志文件名、warning code、资源路径与测试 app/版本，作为人工结论的一部分。

## 依据返回怎么判断

- 静态风险 → 用对应专项 skill 修复 EPUB 源文件后重跑校验组合。
- Kindle 日志 → 尽量把 warning code 或 message 映射到具体资源/页面；没有阅读器测试确认时，不把被忽略的 warning 当成无害；未解决结果在 `docs/final/reader-matrix.yaml` 记录为 `warn` 或待验证。
- 常见修复：WebP warning → 交付图片换 JPEG/PNG，WebP 只留作源文件或现代阅读器实验；封面识别缺失 → 补 `properties="cover-image"` 和 `<meta name="cover" content="..."/>`；NCX 缺失 → Kindle/legacy 交付包加 `toc.ncx`、manifest `id="ncx"`、`spine toc="ncx"`；波浪线丢失 → 在 `text-decoration-style` 前保留普通 underline fallback；环绕图过小 → float 和百分比宽度移到 `figure`，在 `25%` 到 `35%` 内微调；MathML 页面未声明 → 加 manifest `properties="mathml"`，目标阅读器不支持时准备文本或图片 fallback。
- 新行为会影响当前书以外的项目 → 通过 `epub-style-demo-maintainer` 先新增或更新最小 fixture（`Text/08-long-mixed-flow.xhtml` 裁切与长 token、`Text/09-kindle-risk.xhtml` Kindle 专项风险、`Text/16-math.xhtml` MathML properties、`Text/17-image-layout.xhtml` figure 环绕），实测后回写 `docs/final/reader-matrix.yaml`，再沉淀到 SPEC 与手册。
- `status == approval-required` → 停下来问人；`findings` 出现 `error` 或红线未通过 → 回滚或修复后重跑。
