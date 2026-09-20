---
name: epub-style-demo-maintainer
description: 发现和维护可复现 EPUB 版式示例、兼容 fixture 与阅读器证据闭环。maintain 可搜索真实场景或只读验证，不构建 EPUB、不自动更新 matrix。
---

# EPUB 版式 Demo 与证据

## 何时用

新增示例、修改 fixture 或验证阅读器行为时。先读 [README](../../templates/epub-style-demo/README.md)、[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 与 [reader matrix](../../docs/final/reader-matrix.yaml)，选相关场景，不复制全部规范到本 skill。

## 调什么

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

前缀 `epub.style.demo.maintain.`：验证看 mode=source-tree/artifact、errors；目录看 mode=catalog、scenes[] 的 path/SHA256/stylesheets/classes。previewStatus=not-rendered、readerStatus=not-verified 不继承旧实测结论。`error styledemo` 是结构违反；`styledemo.epubcheck-skipped` 说明未跑 EPUBCheck，CI 单独验收。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- 示例先展示一个明确设计问题：页面角色、基线、目标变化、降级方式；保留足够真实文本与复杂内容，不能用短占位文假装证明分页/环绕。
- “好看”候选复用现有字体角色/CSS 层，正文、章首、复杂页保持一致节奏；普通/大字号、窄屏及目标阅读器分别检查，不把浏览器截图当通用 EPUB 结论。
- 最小 fixture → 构建 → 静态校验 → 实际阅读器/转换器验证 → matrix 记录 artifact/SHA、版本、现象与证据 → 有依据再更新 SPEC、手册、速查表、skills。
- 原有场景覆盖不能为新示例删减；未实测写待验证，不自动置 pass。书级特例留书级记录，不把单本审美偏好升级成全仓硬规则。
