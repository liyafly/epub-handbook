---
name: epub-popup-footnote-converter
description: 把 EPUB 普通注释、尾注、旧式注释或纯文本注释标记规范化为项目标准 popup footnote 结构：图片注释图标触发、同文件 grouped note body、◎ 回跳，并保留注释内容。结构规范化由 EPUB3 迁移能力内置完成，本 skill 提供目标结构与校验判据。
---

# EPUB Popup Footnote 转换

## 何时用

- 需要把普通脚注、尾注、Sigil 旧结构、旧多看注释或纯文本 noteref 标记转换为项目最终 popup footnote 模式时。规范化动作由 `epub.package.migrate.epub3` 内置完成（默认转换经识别的 plain/Sigil/Duokan 注释结构）；本 skill 定义目标结构，并负责迁移后校验与 AI 补修。
- 权威结构源是 `docs/final/SPEC-实现约束.md` §1；本 skill 不得与该节分叉。目标结构：
  - 任何使用 `epub:type` 的 XHTML 根 `<html>` 声明 `xmlns:epub="http://www.idpf.org/2007/ops"`（已有则保留，不重复添加）。
  - noteref 是带 `epub:type="noteref"` 和 `role="doc-noteref"` 的 `<a>`，内容是图片图标；已有本地图标资源时保留原 `img src`，只有从纯文本标记生成图片触发器且 EPUB 还没有可用图标时，才使用本 skill 的 `assets/note.png`（复制进 `Images/` 并补 manifest）。
  - 每个 XHTML 文件最多一个 grouped note body：`<aside epub:type="footnote" role="doc-footnote">`；该文件所有 notes 放进 `ol.footnote-list`；每条 note target 是带目标 `id` 的 `li.footnote-item`；noteref `href` 指向 `li.footnote-item` id，不指向独立 per-note aside；回跳符号是 `◎`（`epub:type="backlink"`、`role="doc-backlink"`）。
  - noteref、target `li` 和包含它的 aside 位于同一 XHTML 文件；注释正文精确保留；私有 note 机制不能作为主路径。
- noteref 模式：

  ```html
  <sup class="note-marker">
    <a id="note-1"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  ```

  grouped body 模式：

  ```html
  <aside epub:type="footnote" role="doc-footnote">
    <div><hr class="footnote-line xian"/></div>
    <ol class="footnote-list">
      <li class="footnote-item" id="footnote-1">
        <p class="footnote">
          <a class="footnote-back"
             epub:type="backlink"
             role="doc-backlink"
             href="#note-1">◎</a>
          注释内容。
        </p>
      </li>
    </ol>
  </aside>
  ```

- 弹注 CSS 写进 `Styles/notes.css`（本仓库 layered demo 契约），不写进 `poster.css`；`@font-face` 和字体工具类属于 `Styles/fonts.css`。要点：`sup.note-marker > .noteref-icon` 零行高、相对上移（`top:-0.14em`、`text-decoration:none`），图标 `img` 高 `0.72em`；`.footnote-line` 用 `border-top: 1px solid #777`；`.footnote-list`/`.footnote-item` 去 list-style；`.footnote-back` 去 text-decoration。
- 禁止事项：
  - 除非没有图标资源且用户同意，不把图片图标替换为纯文本；已有图片图标不得无差别替换为默认 `Images/note.png`。
  - 不使用无作用域的 `sup img` 或裸 `sup` 图标规则；普通文字上标保持原样。
  - 不对 footnote body 使用 `display:none`；不把 notes 移到另一个 XHTML 文件；同一文件多条 notes 时必须一个 aside + `ol/li` 分组，不输出每条一个 aside。
  - 不改写注释正文；不把 `duokan-wavyline`、多看专属 notes 或 JS 作为主机制。
  - 需要多看旧版兼容时，先完成本转换，再应用 `epub-legacy-footnote-fallback`。

## 调什么

规范化（写型能力，先 dry-run 审查再实跑）：

```sh
# 1) dry-run：只扫描并报告，不写输出
epub run epub.package.migrate.epub3 --input <书> --output <新书> --dry-run --json

# 2) 人工确认后实跑；只做弹注规范化时关掉基础排版
epub run epub.package.migrate.epub3 --input <书> --output <新书> --json no_typography=true
```

默认同时转换弹注与基础排版；`no_popup_notes=true` / `no_typography=true` 仅在用户明确要求时使用。

迁移完成后对产物跑弹注结构校验（只读）：

```sh
epub run epub.notes.popup.normalize --input <产物> --json
```

再做正文红线与人工 diff review：

```sh
epub redline --check all <before.epub> <after.epub>
```

需要旧报告形状明细（逐文件 ERROR 行等）时给上述 run 命令加 `legacy_report=true`。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（参数非法、文件不存在、缺 `--output`）。
- `epub.package.migrate.epub3` 的 facts 键前缀 `epub.package.migrate.epub3.`：`plainNotesConverted`（plain/纯文本标记转换数）、`duokanNotesNormalized`（Duokan 结构归一数）、`navEntries`、`xhtmlFilesUpdated`、`stylesheetLinksAdded`、`manifestItemsAdded`、`manifestItemsUpdated`、`metadataUpdates`、`typographyRoles`、`warnings`，以及开关回显 `popupNotes` / `typography`。findings：`warn migrate.warning`（迁移期保留的模糊结构等）。
- `epub.notes.popup.normalize` 的 facts 键前缀 `epub.notes.popup.normalize.`：`noterefs`（noteref 数）、`text_files`（XHTML 文件数）、`violations`（结构违反数）。`violations > 0` 时 `findings` 逐条出现 `error popupnotes`，`title` 是具体文件与问题（缺 id、href 非同文件片段、缺 `noteref-icon`/`footnote-item`、目标缺失、backlink 缺失或错指、Duokan fallback 类不全、图标未声明或文件缺失等）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- `status == approval-required`（dry-run，退出码 2）→ 逐条 review facts 的 `warnings` 与计划改动后实跑，不要跳过审查。
- 校验 `violations == 0` 且无 `popupnotes` findings → 弹注结构合格。出现 `error popupnotes` → 按 `title` 定位文件修复后重跑，不得绕过。
- 迁移保留的模糊结构（`warn migrate.warning`）：人工 review 对应文件，按本 skill 的目标结构手工补转；Sigil 旧结构（`section[epub:type="footnotes"]` 内多个 `aside#footnote_N` + 正文 `a#noteref_N`）只有全部 aside 都能匹配时才自动合并为一个 grouped `aside/ol/li`，有任何无法识别的 aside 或附加内容时不做部分转换，改为人工 review。尽量保留已有 note id，只有缺失或冲突时才规范化。
- 图标纪律：每个 noteref 图标都必须在 OPF manifest 声明、media-type 为 image、文件存在于包内；原 noteref 已含图片时保留原 `img src` 和 `alt`。
- 红线约束：`epub redline` 只将 noteref/backlink 的数字、图标和 `◎` 当作表示控件；所有 `li.footnote-item` 的注释正文必须逐字相同。红线未通过 → 回滚或修复后重跑。
- 多条 note 的人工验证：同一 XHTML 内多条 notes 时，每个 trigger 只能打开它指向的 `li` 内容；标准 EPUB 路径必须能通过 href → target id 精确解析。
- fixture 参考：`templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml`（标准弹注样例）；demo 构建与验证由 `epub-style-demo-maintainer` 处理。
- `status == failed` 或 `findings` 出现 `error redline.*` → 停下来修源或回滚，产物保留供人工 diff review；需要多看旧版兼容时再进入 `epub-legacy-footnote-fallback`。
