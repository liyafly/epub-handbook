---
name: epub-legacy-footnote-fallback
description: 在项目标准 EPUB 3 grouped footnote 结构上叠加多看旧版 popup note 兼容 fallback。用于必须兼容 Duokan legacy readers，且仍需满足 SPEC §1 fallback 约束时。
---

# EPUB 旧版弹注 Fallback

## 何时用

- 只有目标 EPUB 必须保留多看旧版 popup-note 兼容性时才使用；它是兼容层，不是项目默认 note 模式——普通项目输出用 `epub-popup-footnote-converter`。
- 权威结构源是 `docs/final/SPEC-实现约束.md` §1；本 skill 不得与该节分叉，不得单方面新增或改名同族 class。
- 主形态保留项目标准结构：XHTML 根声明 `xmlns:epub="http://www.idpf.org/2007/ops"`；noteref 是带 `epub:type="noteref"` 和 `role="doc-noteref"` 的 `<a>`，`href` 指向同一 XHTML 文件内的 note `li`；每个 XHTML 文件只有一个 grouped note body（`<aside epub:type="footnote" role="doc-footnote">`）；本地 notes 放 `ol.footnote-list`；note target 是 `li.footnote-item`；backlink 是 `◎`。
- 禁止事项：不删除 `footnote-list`、`footnote-item` 等中性类；不只保留 `duokan-*` 类；不复制第二份可见 note list；不把 `duokan-footnote-content` 放在单个 `li` 上（旧多看兼容验证的是 grouped `ol` 上的类）；不使用 JavaScript 或 `display:none` note body；不在多看 fallback 范围外添加阅读器私有 note 属性。

## 调什么

本 skill 是 AI 手工改写类 skill：在标准结构上叠加 legacy hooks，不创建第二份 note body。改写后必须校验：

```sh
epub run epub.notes.popup.normalize --input <产物> --json
epub redline --check all <before.epub> <after.epub>
```

涉及 demo 模板改动时再加：

```sh
epub run epub.style.demo.maintain --input <demo 产物> --json # 能力迁移中：findings 出现 warn capability.not-implemented 表示校验逻辑尚未执行
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`（noteref 数）、`text_files`（XHTML 文件数）、`violations`（结构违反数）；violations > 0 时 `findings` 出现 `error popupnotes`，`detail` 指明具体文件与问题。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过。

## 依据返回怎么判断

- 叠加的 legacy hooks（缺一不可，且不得替换标准属性）：noteref anchor 增加 `duokan-footnote`；grouped `ol.footnote-list` 增加 `duokan-footnote-content`；每个 note `li` 增加 `duokan-footnote-item`；noteref anchor 内放 note icon 图片。
- 改写流程判据：从已符合（或可先转换为）标准 `aside > ol.footnote-list > li.footnote-item` 模式的文件开始；尽量保留 id，noteref id 与 note target id 在当前 XHTML 内唯一；给 noteref anchor 加 `duokan-footnote` 时不删除 `epub:type`、`role`、`id`、`href`；已有本地图标时保留原 `img src`，只有缺少图片热区且需要新增 legacy fallback 资源时才用 `../Images/note.png`，并确保该图标已在 OPF manifest 声明且文件存在；所有 href/backlink target 必须解析到同一 XHTML 文件内。
- 结构校验返回 `violations: 0` 且无 `popupnotes` findings → 结构合格；出现 `error popupnotes` → 按 detail 修复后重跑，不得绕过。
- 多条 note 的人工验证：同一 XHTML 文件内有多条 notes 时，每个 trigger 只能打开它指向的 `li` 内容；标准 EPUB 路径必须能通过 href → target id 精确解析。
- CSS：EPUB 还没有样式化这些类时，把 `.noteref-icon, a.duokan-footnote { text-decoration: none; }`、`.noteref-icon img, a.duokan-footnote img { width:auto; height:1em; vertical-align: baseline; }`、`ol.duokan-footnote-content { list-style-type:none; padding:0; margin:0; }`、`.footnote-item.duokan-footnote-item { list-style-type:none; }` 合并进活动 note CSS；源文件没有 `.footnote-line` 时可加可见分隔线——要么用 `footnote-line` 规则，要么给 `.duokan-footnote-content` 加 border，同一路径不要两者都用。
- fixture 参考：`templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml`（单条兼容样例）；历史多条样例在 `templates/epub-style-demo/retired/06-multi-legacy-note-fallback.xhtml`（不进入默认 demo 构建）。
- `status == approval-required` → 停下来问人；红线未通过 → 回滚或修复后重跑，再做人工 diff review。
