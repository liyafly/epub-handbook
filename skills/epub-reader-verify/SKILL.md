---
name: epub-reader-verify
description: 审核 Kindle 静态风险、维护可复现版式 demo 并闭合阅读器证据；区分静态扫描、转换器结果与目标阅读器实测，不自动升级兼容结论。
---

# EPUB 阅读器验证

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### Kindle 兼容检查（原 epub-kindle-compatibility-checker）

Kindle 交付前，或 Previewer/App 与其他阅读器不同。规则按 [SPEC](../../docs/final/SPEC-实现约束.md) §5–§5.11 和 [reader matrix](../../docs/final/reader-matrix.yaml) 核对，不从记忆推广到所有 Kindle 版本。

### 版式 demo 与证据（原 epub-style-demo-maintainer）

新增示例、修改 fixture 或验证阅读器行为时。先读 [README](../../templates/epub-style-demo/README.md)、[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 与 [reader matrix](../../docs/final/reader-matrix.yaml)，选相关场景，不复制全部规范到本 skill。

## 调什么

### Kindle 兼容检查（原 epub-kindle-compatibility-checker）

`epub.kindle.compatibility.check` 当前未实现；使用可用静态检查，再读实际转换日志：

```sh
epub run epub.package.nav.audit --input "book.epub" --json
epub run epub.layout.audit --input "book.epub" --json
```

有授权修复后按根 AGENTS 跑红线；涉及注释加 popup validator。实测前确认本机转换器版本、实际输出与日志路径，按 [demo README](../../templates/epub-style-demo/README.md) 选择验证场景。

### 版式 demo 与证据（原 epub-style-demo-maintainer）

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

### Kindle 兼容检查（原 epub-kindle-compatibility-checker）

JSON 只代表静态发现，见 [公共语义](../README.md)。转换日志另记文件、错误码、资源路径、工具/版本、产物 SHA。转换成功、Previewer 展示、App/设备验收是三种不同证据。

### 版式 demo 与证据（原 epub-style-demo-maintainer）

前缀 `epub.style.demo.maintain.`：验证看 mode=source-tree/artifact、errors；目录看 mode=catalog、scenes[] 的 path/SHA256/stylesheets/classes。previewStatus=not-rendered、readerStatus=not-verified 不继承旧实测结论。`error styledemo` 是结构违反；`styledemo.epubcheck-skipped` 说明未跑 EPUBCheck，CI 单独验收。

## 依据返回怎么判断

### Kindle 兼容检查（原 epub-kindle-compatibility-checker）

- 包结构：nav+NCX、spine toc、封面 cover-image/兼容 metadata、MathML properties。
- 资源/布局：JPEG/PNG 主路径，风险 SVG 需 fallback；figure 承载 float/% 宽度；带样式下划线有普通 underline；长 token、表格、代码和大字号不能溢出。
- 日志 warning 映射具体资源再判断，不默认无害；没有实测不虚构 pass/fail。设备不可用时列待验项，不把静态修复当成验收完成。
- 改动交最窄专项 skill，保留 EPUB3 语义和正文；新兼容规则走 demo → 实测 → matrix → SPEC，不把私有 CSS 当关键内容唯一路径。

### 版式 demo 与证据（原 epub-style-demo-maintainer）

- 示例先展示一个明确设计问题：页面角色、基线、目标变化、降级方式；保留足够真实文本与复杂内容，不能用短占位文假装证明分页/环绕。
- “好看”候选复用现有字体角色/CSS 层，正文、章首、复杂页保持一致节奏；普通/大字号、窄屏及目标阅读器分别检查，不把浏览器截图当通用 EPUB 结论。
- 最小 fixture → 构建 → 静态校验 → 实际阅读器/转换器验证 → matrix 记录 artifact/SHA、版本、现象与证据 → 有依据再更新 SPEC、手册、速查表、skills。
- 原有场景覆盖不能为新示例删减；未实测写待验证，不自动置 pass。书级特例留书级记录，不把单本审美偏好升级成全仓硬规则。
