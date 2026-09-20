---
name: epub-typography-optimizer
description: 优化中文/CJK EPUB 字体角色与正文节奏，或审查后应用排版 preset。会写样式层但不自动嵌入/子集化字体；英文主书、图片、弹注或海报另走专项 skill。
---

# EPUB 中文字体与正文节奏

## 何时用

处理 CJK 字体链、行距/缩进/长 token 与角色样式。改字体前读 [SPEC §3、§4、§8](../../docs/final/SPEC-实现约束.md)；新增 alias/文件/class 前读 [字体命名规范](../../docs/final/字体别名命名规范.md)。保留既有自由/锁定模式，不把“更好看”自动解释为锁定正文字体。

## 调什么

```sh
epub run epub.typography.optimize --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.typography.optimize --input "before.epub" --output "candidate.epub" --json preset=literary-cn
# 局部试样：精确指定 ZIP 内 spine XHTML 路径；预览后用同样参数去掉 --dry-run
epub run epub.typography.optimize --input "before.epub" --output "sample.epub" --dry-run --json preset=literary-cn 'scope_paths=["OEBPS/Text/chapter.xhtml"]'
epub redline --check all "before.epub" "candidate.epub"
```

preset 为 `literary-cn`（默认）、`academic-cn`、`classical-annotated-cn`；仅自定义库才用 `preset_dir=`。省略 scope_paths 是整书替换预设层；提供非空 JSON 路径数组则只向所选章节追加内容寻址的独立 CSS，保留原链接与共享样式。不支持携带 url()/@import 的局部 preset，不能借此安装资源或自动改正文结构。字体资源/链变化后另跑 `epub run epub.font.coverage.analyze --input "candidate.epub" --json`。

## 返回怎么读

前缀 `epub.typography.optimize.`：`preset/layers/notes`、`coverage`、`stylesheetActions[]`（add/replace/keep）、`xhtmlLinkFiles`、`manifestItemsAddedHrefs`。局部模式另有 `applicationMode=scoped-additive` 与 `scopePaths`。
**coverage 是 preset 类覆盖率，不是字形覆盖率。** `typography.low-coverage` 表示结构与 preset 不匹配。公共返回见 [索引](../README.md)。

## 依据返回怎么判断

- 审查 add/replace 的具体样式表，尤其避免覆盖原书有效设计；只需局部改动时，不为用能力而套整套 preset。先做代表性正文/章首/复杂页样例，确认视觉节奏再推广。
- 自由模式 body/普通 p 不设 font-family；锁定入口为 fonts.css 的 body 规则并同步 OPF 字体 meta/prefix。局部字体不要求全书锁定；保留既有模式及明确书级例外。
- 设计字体/少量补字使用专用角色；局部子集不可拿来锁全书。C1-body 须按 CSS 继承覆盖全部角色用字和标点；保留 generic fallback，不靠重复嵌入字体解决回退。
- 字体声明进 fonts.css，正文节奏进 base.css；注释结构进 notes.css。带 epub namespace 的选择器须正确声明 namespace。
- 自由版与锁定版从同一内容基线派生，允许差异与 modified 时间规则按 SPEC §8；红线和字体覆盖都通过后，仍需大字号/窄屏阅读器验收。
