---
name: epub-kindle-compatibility-checker
description: 人工审核 Kindle/KDP 静态风险、转换日志和实际阅读表现。用于 Kindle 交付或跨阅读器差异；当前无自动兼容 checker，静态通过不等于设备验收。
---

# EPUB Kindle 兼容检查

## 何时用

Kindle 交付前，或 Previewer/App 与其他阅读器不同。规则按 [SPEC](../../docs/final/SPEC-实现约束.md) §5–§5.11 和 [reader matrix](../../docs/final/reader-matrix.yaml) 核对，不从记忆推广到所有 Kindle 版本。

## 调什么

`epub.kindle.compatibility.check` 当前未实现；使用可用静态检查，再读实际转换日志：

```sh
epub run epub.package.nav.audit --input "book.epub" --json
epub run epub.layout.audit --input "book.epub" --json
```

有授权修复后按根 AGENTS 跑红线；涉及注释加 popup validator。实测前确认本机转换器版本、实际输出与日志路径，按 [demo README](../../templates/epub-style-demo/README.md) 选择验证场景。

## 返回怎么读

JSON 只代表静态发现，见 [公共语义](../README.md)。转换日志另记文件、错误码、资源路径、工具/版本、产物 SHA。转换成功、Previewer 展示、App/设备验收是三种不同证据。

## 依据返回怎么判断

- 包结构：nav+NCX、spine toc、封面 cover-image/兼容 metadata、MathML properties。
- 资源/布局：JPEG/PNG 主路径，风险 SVG 需 fallback；figure 承载 float/% 宽度；带样式下划线有普通 underline；长 token、表格、代码和大字号不能溢出。
- 日志 warning 映射具体资源再判断，不默认无害；没有实测不虚构 pass/fail。设备不可用时列待验项，不把静态修复当成验收完成。
- 改动交最窄专项 skill，保留 EPUB3 语义和正文；新兼容规则走 demo → 实测 → matrix → SPEC，不把私有 CSS 当关键内容唯一路径。
