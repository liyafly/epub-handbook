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

`epub.kindle.compatibility.check` 是只读静态检查；输出不代表 Kindle Previewer、App 或设备验收。按以下顺序采集、审阅和复查：

```sh
# 1. 生成只读检查报告
epub run epub.kindle.compatibility.check --input "book.epub" --dry-run --json > "kindle-plan.json"
# 2. 审阅所有 Kindle finding 及计数
python3 -c 'import json; r=json.load(open("kindle-plan.json")); print(json.dumps({"findings":r.get("findings",[]),"counts":r.get("facts",{}).get("epub.kindle.compatibility.check.counts",{})}, ensure_ascii=False, indent=2))'
# 3. 对确认后的输入重新运行只读扫描；本能力不写出 EPUB
epub run epub.kindle.compatibility.check --input "book.epub" --json > "kindle-final.json"
# 4. 若另有已授权修复候选，验证内容边界
epub redline --check all "book.epub" "candidate.epub"
```

前两次扫描均为只读，不会生成 `plannedEdits`；审阅时按 finding ID 选择对应专项并遵守授权。第 4 步只适用于已存在的候选，不把静态报告当成修改许可。实测前确认本机转换器版本、实际输出与日志路径，按 [demo README](../../templates/epub-style-demo/README.md) 选择验证场景。

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

读取 `facts.epub.kindle.compatibility.check.checks` 的检查顺序、`counts` 的逐项命中数、`cssFilesScanned` / `xhtmlFilesScanned` 的扫描范围和 `staticOnly=true`。这是只读 validator，不返回 `plannedEdits` 或 `skipped`。finding 前缀为 `kindle.`；error 会令状态 `failed`，warn/info 不会单独阻断。转换日志另记文件、错误码、资源路径、工具/版本、产物 SHA。转换成功、Previewer 展示、App/设备验收是三种不同证据。

### 版式 demo 与证据

前缀 `epub.style.demo.maintain.`：验证看 mode=source-tree/artifact、errors；目录看 mode=catalog、scenes[] 的 path/SHA256/stylesheets/classes。previewStatus=not-rendered、readerStatus=not-verified 不继承旧实测结论。`error styledemo` 是结构违反；`styledemo.epubcheck-skipped` 说明未跑 EPUBCheck，CI 单独验收。

## 依据返回怎么判断

### Kindle 兼容检查

- `kindle.ncx-missing`：补齐 NCX manifest item 或 spine `toc`；`kindle.cover-image-missing`、`kindle.cover-meta-missing`、`kindle.cover-not-raster`：按封面规范核对 cover-image、name=cover 元数据和 JPEG/PNG 主路径。
- `kindle.image-webp` 是 error，改用 JPEG/PNG 并重查；`kindle.image-tiff`、`kindle.image-gif` 要逐资源复核，GIF 另人工确认帧数；`kindle.image-svg` 检查目标格式的 raster fallback。
- `kindle.mathml-properties-missing`：为含 MathML 的 spine item 补 `properties="mathml"`；`kindle.css-transform-rotate`：移除通用 EPUB 便签旋转；`kindle.css-styled-underline`：先声明基础 underline 再声明增强样式。
- `kindle.css-amzn-media-query`：移除 Kindle 专用媒体查询；`kindle.css-img-direct-float`：将 float 放在 wrapping figure；`kindle.css-unicode-range`：核对字体分配在目标 Kindle 格式中的实测结果。
- `kindle.css-parse-failed` / `kindle.xhtml-parse-failed`：修复或人工检查对应文件后重跑。转换日志中的 warning 映射到具体资源再判断，不默认无害；没有实测不虚构 pass/fail。设备不可用时列待验项，不把静态修复当成验收完成。
- 改动交最窄专项 skill，保留 EPUB3 语义和正文；新兼容规则走 demo → 实测 → matrix → SPEC，不把私有 CSS 当关键内容唯一路径。

### 版式 demo 与证据

- 示例先展示一个明确设计问题：页面角色、基线、目标变化、降级方式；保留足够真实文本与复杂内容，不能用短占位文假装证明分页/环绕。
- “好看”候选复用现有字体角色/CSS 层，正文、章首、复杂页保持一致节奏；普通/大字号、窄屏及目标阅读器分别检查，不把浏览器截图当通用 EPUB 结论。
- 最小 fixture → 构建 → 静态校验 → 实际阅读器/转换器验证 → matrix 记录 artifact/SHA、版本、现象与证据 → 有依据再更新 SPEC、手册、速查表、skills。
- 原有场景覆盖不能为新示例删减；未实测写待验证，不自动置 pass。书级特例留书级记录，不把单本审美偏好升级成全仓硬规则。
