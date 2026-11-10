---
name: epub-reader-verify
description: 审核 Kindle 静态风险、维护可复现版式 demo 并闭合阅读器证据；区分静态扫描、转换器结果与目标阅读器实测，不自动升级兼容结论。
---

# EPUB 阅读器验证

## 何时用

公共返回、授权边界与验收说明见[技能索引](../README.md)。

- Kindle 兼容检查：Kindle 交付前，或 Previewer/App 与其他阅读器表现不同。规则按 [SPEC](../../docs/final/SPEC-实现约束.md) §5–§5.11 和 [reader matrix](../../docs/final/reader-matrix.yaml) 核对，不从记忆推广到所有 Kindle 版本。
- 版式 demo 与证据：新增示例、修改 fixture 或验证阅读器行为时。先读 [README](../../templates/epub-style-demo/README.md)、[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 与 reader matrix，选相关场景。

## 调什么

```sh
epub run epub.kindle.compatibility.check --input "book.epub" --json
epub run epub.style.demo.maintain --json catalog=true query=chapter
epub run epub.style.demo.maintain --input "templates/epub-style-demo" --json
epub run epub.style.demo.maintain --input "demo.epub" --json
```

- 两个能力都只读，无 `--output`；`kindle.compatibility.check` 是静态检查，不代表 Previewer、App 或设备验收。
- maintain 的 `catalog=true` 从真实 spine 生成场景目录，不运行验证器，不得把发现成功称为验证通过；构建步骤不由它代办。demo 另跑 `epub.notes.popup.normalize` 与 nav audit。
- 实测前确认本机转换器版本、实际输出与日志路径。

## 返回怎么读

- Kindle 检查：`counts` 是逐项命中数，`staticOnly=true`；finding 前缀 `kindle.`，error 令状态 `failed`，warn/info 不单独阻断。转换成功、Previewer 展示、App/设备验收是三种不同证据，转换日志另记文件、错误码、资源路径、工具/版本、产物 SHA。
- demo 维护：`mode=source-tree/artifact` 是验证，`mode=catalog` 是目录；`previewStatus=not-rendered`、`readerStatus=not-verified` 不继承旧实测结论。`error styledemo` 是结构违反；`styledemo.epubcheck-skipped` 表示未跑 EPUBCheck，由 CI 单独验收。

## 依据返回怎么判断

### Kindle 兼容检查

- `kindle.ncx-missing`：补齐 NCX manifest item 或 spine `toc`；`kindle.cover-not-raster`：封面走 JPEG/PNG 主路径。cover-image 与 legacy `name=cover` 由 nav.audit 报告。
- `kindle.image-webp` 是 error，改 JPEG/PNG；`kindle.image-gif` 逐资源复核帧数；`kindle.image-svg` 检查 raster fallback。
- `kindle.css-transform-rotate`：移除通用 EPUB 便签旋转；`kindle.css-styled-underline`：先声明基础 underline 再声明增强样式；`kindle.css-amzn-media-query`：移除 Kindle 专用媒体查询；`kindle.css-img-direct-float`：float 放在 wrapping figure；`kindle.css-unicode-range`：核对字体分配在目标格式的实测。
- `kindle.css-float-clear-stack`（info，依据 reader-matrix `slice-float-seam`，用户观察无截图）：同一规则既浮动又 clear 时，Kindle 不把它叠到上一个浮动下面；需要无缝拼接改用整图或 page-image 配方。`kindle.css-emphasis-underline`（warn，依据 `recipe-text-emphasis` 截图与日志）：Kindle 同时显示着重号和下划线，`@supports` 取消下划线不生效；可读，按书决定是否接受。
- `kindle.css-parse-failed`：修复样式表后重跑。转换日志里的 warning 映射到具体资源再判断，不默认无害；没有实测不虚构 pass/fail，设备不可用时列待验项。
- 改动交最窄专项 skill，保留 EPUB3 语义和正文；新兼容规则走 demo → 实测 → matrix → SPEC，不把私有 CSS 当关键内容唯一路径。

### 版式 demo 与证据

- 示例展示一个明确设计问题：页面角色、基线、目标变化、降级方式；用足够真实的文本与复杂内容，不用短占位文证明分页/环绕。普通/大字号、窄屏及目标阅读器分别检查，浏览器截图不是通用 EPUB 结论。
- 最小 fixture → 构建 → 静态校验 → 阅读器/转换器验证 → matrix 记录 artifact/SHA、版本、现象与证据 → 有依据再更新 SPEC、手册、速查表、skills。
- 原有场景覆盖不为新示例删减；未实测写待验证，不自动置 pass。书级特例留书级记录，不把单本审美偏好升级成全仓硬规则。
- 导入前记录候选与测试副本 SHA、唯一 identifier 差异；导入后核对实际载荷。Readest 0.12.16 曾因 partial-MD5 相同复用旧内容，只换 UUID 不保证隔离；先解决身份，再判断样式成败。见 [10 月复盘 §4](../../docs/pipeline/reader-adaptation-lessons-2026-10.md#4-readest先排除旧内容再判断-css-是否有效)。
- 图片裁切先看源图是否完整，再在同窗口/栏数、两个相关字号下连续检查目标、相邻图及下一章；静态 `break-inside:avoid` 和一张目标图通过不证明分页修复。
- 共性问题先小样确认机制，再检查全部受影响实例；代表抽样不能替代全部变更点验收，缺口按位置继续登记。
- 长注分别记录打开、滚到底、关闭、返回触发位置；只打开或只看到 ◎ 不算完整交互通过。主题样式记录换章/切主题/重开后的表现，含 dark 选择器不等于已验夜间。
- 验收按三态记录：增强生效 / 退回可读 / 坏掉，只有坏掉阻断（SPEC §0）。
- 测试前记录书库条目、位置、字号、字体覆盖、主题、窗口与栏数；结束后逐项核对恢复。不可完全还原的影响如实记录；中断与工具崩溃记待验，不补写 pass。
