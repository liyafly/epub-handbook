---
name: epub3-migrator
description: 把 EPUB2、Kindle/MOBI 回转或其他 legacy EPUB 迁移到 EPUB3 基线：package/nav、XHTML shell、弹注和基础排版，同时保留正文与原文件。用于需要 EPUB3 迁移且必须可审计、可回滚时。
---

# EPUB3 迁移

## 何时用

- 输入是 EPUB2、Kindle/MOBI 回转或其他 legacy EPUB，需要迁移 package/nav、XHTML shell、弹注和基础排版到 EPUB3，且必须保留正文和原文件时。
- 流程固定为「预检 → dry-run 审查 → 写出 → 红线 → 产物校验 → 人工 diff review」。迁移只建立 EPUB3 基线，不替代后续文学结构、字体覆盖或阅读器实测。
- 迁移内容：package version、`dcterms:modified`、manifest media/properties；保留 NCX 并生成 EPUB3 `nav.xhtml`；XHTML5 shell、语言属性和可维护格式；经识别的 plain/Sigil/Duokan 注释结构；可选基础排版 stylesheet（不嵌入新字体）。
- 边界：
  - 不覆盖输入或已有输出；`--output` 指向新文件，不能与 `--input` 相同。
  - 不解密、不改正文、不自动替换字符；DRM、未知加密、损坏 container/OPF 时停止。
  - 只有已识别的注释结构可以迁移；模糊结构保留并在 `warnings` 中报告，转人工。
  - 迁移报告不是阅读器兼容性实测结论。

## 调什么

```sh
# 1) 预检（只读）：DRM、未知加密、损坏 container/OPF 时停止
epub run epub.package.nav.audit --input <书> --json

# 2) dry-run 审查：只扫描并报告，不写输出
epub run epub.package.migrate.epub3 --input <书> --output <新书> --dry-run --json

# 3) 确认后写出新文件
epub run epub.package.migrate.epub3 --input <书> --output <新书> --json

# 4) 产物校验：弹注结构 + 正文红线
epub run epub.notes.popup.normalize --input <新书> --json
epub redline --check all <书> <新书>
```

可选 KEY=VALUE：`no_popup_notes=true`、`no_typography=true`——只在用户明确要求跳过弹注或基础排版时使用（默认两者都做）。需要旧报告形状明细（conversion report 逐项）时给 run 命令加 `legacy_report=true`。单元与回归验证由 `go test` 承担，不在本 skill 展开。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（参数非法、文件不存在、缺 `--output`、输出与输入相同）。
- facts 键前缀 `epub.package.migrate.epub3.`：
  - `packageVersionBefore`、`opf`：迁移前 package 版本与 OPF 路径。
  - `navEntries`、`xhtmlFilesUpdated`、`stylesheetLinksAdded`：nav/-shell/样式表改动量。
  - `plainNotesConverted`、`duokanNotesNormalized`：弹注转换量。
  - `manifestItemsAdded`、`manifestItemsUpdated`、`metadataUpdates`、`typographyRoles`：manifest 与 metadata 改动量。
  - `warnings`：迁移期保留的模糊结构等警告。
  - `popupNotes` / `typography`：两个开关是否生效（回显）。
- findings：`warn migrate.warning` 对应 `warnings` 逐条；run 内置红线门禁失败时出现 `error redline.<check>`（text/metadata/spine/anchors/cover/drm）。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（底层 conversion report，含 before/after SHA-256）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- dry-run `status == approval-required`（退出码 2）→ 逐条 review facts（尤其 `warnings`、`manifestItemsAdded`、`plainNotesConverted`）后实跑；不要跳过审查直接写输出。
- `status == complete` 且红线通过 → 进入人工检查：OPF/nav/NCX、封面、spine 和弹注 diff 用 Calibre Editor 或 VS Code 人工 review；需要字体、图片或结构精排时再分派 `epub-typography-optimizer`、`epub-image-layout-optimizer`、`epub-literary-structure-formatter` 等专项 skill。
- `warnings` 非空或 `warn migrate.warning` → 逐条人工复核：模糊注释结构按 `epub-popup-footnote-converter` 的目标结构手工处理，不做部分转换。
- `findings` 出现 `error redline.*` → 停止：输出文件保留供人工 diff review，先修源再重跑；不允许用宽泛 allow-list 掩盖。
- `status == failed` → 检查事件里的失败原因（缺 `--output`、输出已存在、保守重写失败等）；提示加密时停止，不解密、不猜测、不绕过。
- 迁移完成后重跑 `epub run epub.package.nav.audit --input <新书> --json` 确认无 EPUB2/结构 error，再记入书级 `制作说明.md`。
