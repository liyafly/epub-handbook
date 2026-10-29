---
name: epub-reader-verify
description: 审核 Kindle 静态风险、维护可复现版式 demo 并闭合阅读器证据；区分静态扫描、转换器结果与目标阅读器实测，不自动升级兼容结论。
---

# EPUB 阅读器验证

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### Kindle 兼容检查

Kindle 交付前，或 Previewer/App 与其他阅读器不同。规则按 [SPEC](../../docs/final/SPEC-实现约束.md) §5–§5.11 和 [reader matrix](../../docs/final/reader-matrix.yaml) 核对，不从记忆推广到所有 Kindle 版本。

### 版式 demo 与证据

新增示例、修改 fixture 或验证阅读器行为时。先读 [README](../../templates/epub-style-demo/README.md)、[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 与 [reader matrix](../../docs/final/reader-matrix.yaml)，选相关场景，不复制全部规范到本 skill。

## 调什么

### Kindle 兼容检查

`epub.kindle.compatibility.check` 是只读静态检查；输出不代表 Kindle Previewer、App 或设备验收。运行检查并审阅返回：

```sh
# 1. 扫描并读取 JSON 信封
epub run epub.kindle.compatibility.check --input "book.epub" --json
# 2. 阅读 findings[] 与 facts["epub.kindle.compatibility.check.counts"]
```

该能力只读，不生成 `plannedEdits` 或 EPUB 候选。按 finding ID 选择对应专项并遵守授权；若另有已授权修复候选，再对该候选运行全项 redline。实测前确认本机转换器版本、实际输出与日志路径，按 [demo README](../../templates/epub-style-demo/README.md) 选择验证场景。

### 版式 demo 与证据

按模板 README 的构建流程生成独立产物，再验证：

```sh
# 先找真实场景，再读取返回的 XHTML/CSS；省略 query 可列全目录
epub run epub.style.demo.maintain --json catalog=true query=chapter
epub run epub.style.demo.maintain --input "templates/epub-style-demo" --json
epub run epub.style.demo.maintain --input "demo.epub" --json
epub run epub.notes.popup.normalize --input "demo.epub" --json
epub run epub.package.nav.audit --input "demo.epub" --json
```

maintain 均只读，无 `--output`。catalog=true 从真实 spine 生成场景，query 匹配标题/路径/id/class；目录模式不运行验证器，不得把发现成功称为验证通过。改动基线比较与 XML 检查按根 AGENTS；构建步骤不由 maintain 代办。

## 返回怎么读

### Kindle 兼容检查

读取 `facts["epub.kindle.compatibility.check.checks"]` 的检查顺序、`counts` 的逐项命中数、`cssFilesScanned` / `xhtmlFilesScanned` 的扫描范围和 `staticOnly=true`。这是只读 validator，不返回 `plannedEdits` 或 `skipped`。finding 前缀为 `kindle.`；error 会令状态 `failed`，warn/info 不会单独阻断。转换日志另记文件、错误码、资源路径、工具/版本、产物 SHA。转换成功、Previewer 展示、App/设备验收是三种不同证据。

### 版式 demo 与证据

前缀 `epub.style.demo.maintain.`：验证看 mode=source-tree/artifact、errors；目录看 mode=catalog、scenes[] 的 path/SHA256/stylesheets/classes。previewStatus=not-rendered、readerStatus=not-verified 不继承旧实测结论。`error styledemo` 是结构违反；`styledemo.epubcheck-skipped` 说明未跑 EPUBCheck，CI 单独验收。

## 依据返回怎么判断

### Kindle 兼容检查

- `kindle.ncx-missing`：补齐 NCX manifest item 或 spine `toc`；`kindle.cover-not-raster`：按目标格式核对封面资源是否走 JPEG/PNG 主路径。cover-image 与 legacy `name=cover` 元数据由 nav.audit 报告。
- `kindle.image-webp` 是 error，改用 JPEG/PNG 并重查；`kindle.image-gif` 要逐资源复核并人工确认帧数；`kindle.image-svg` 检查目标格式的 raster fallback。TIFF 等非核心图片类型的 manifest fallback 由 nav.audit 统一报告。
- `kindle.css-transform-rotate`：移除通用 EPUB 便签旋转；`kindle.css-styled-underline`：先声明基础 underline 再声明增强样式。
- `kindle.css-amzn-media-query`：移除 Kindle 专用媒体查询；`kindle.css-img-direct-float`：将 float 放在 wrapping figure；`kindle.css-unicode-range`：核对字体分配在目标 Kindle 格式中的实测结果。
- `kindle.css-parse-failed`：修复或人工检查对应样式表后重跑。spine XHTML 的解析与 MathML properties 检查由 nav.audit 报告。转换日志中的 warning 映射到具体资源再判断，不默认无害；没有实测不虚构 pass/fail。设备不可用时列待验项，不把静态修复当成验收完成。
- 改动交最窄专项 skill，保留 EPUB3 语义和正文；新兼容规则走 demo → 实测 → matrix → SPEC，不把私有 CSS 当关键内容唯一路径。

### 版式 demo 与证据

- 示例先展示一个明确设计问题：页面角色、基线、目标变化、降级方式；保留足够真实文本与复杂内容，不能用短占位文假装证明分页/环绕。
- “好看”候选复用现有字体角色/CSS 层，正文、章首、复杂页保持一致节奏；普通/大字号、窄屏及目标阅读器分别检查，不把浏览器截图当通用 EPUB 结论。
- 最小 fixture → 构建 → 静态校验 → 实际阅读器/转换器验证 → matrix 记录 artifact/SHA、版本、现象与证据 → 有依据再更新 SPEC、手册、速查表、skills。
- 原有场景覆盖不能为新示例删减；未实测写待验证，不自动置 pass。书级特例留书级记录，不把单本审美偏好升级成全仓硬规则。
- 导入前记录候选与测试副本 SHA、唯一 identifier 差异；导入后核对实际载荷或原生条目/URL。Readest 0.12.16 曾因 partial-MD5 相同复用旧内容，只换 UUID 不保证隔离；不匹配时先解决身份，再判断样式成败，保留受限旧图。操作细节见 [10 月复盘 §4](../../docs/pipeline/reader-adaptation-lessons-2026-10.md#4-readest先排除旧内容再判断-css-是否有效)。
- 图片裁切先看源图是否完整，再在同窗口/栏数、两个相关字号下连续检查目标、相邻图及下一章；静态 `break-inside:avoid` 和一张目标图通过不证明分页修复。总览/图像索引与全尺寸细标签验收分别计数。
- 共性问题先用小样确认机制，再列出并检查全部受影响实例；图片尺寸/公式高度改变后，把同段正文、图注和表格分页一起回归。代表抽样不能替代全部变更点验收，缺口按位置继续登记。
- 长注分别记录打开、滚到底、关闭、返回触发位置；只打开或只看到 ◎ 不算完整交互通过。主题样式要记录换章/切主题/重开后的表现，含 dark 选择器不等于已验夜间。
- 测试前记录书库条目、位置、字号、字体覆盖、主题、窗口与栏数；结束后逐项核对恢复并保留证据。进度/统计、测试书库等不可完全还原的影响如实记录；中断与工具崩溃记待验，不补写 pass。
