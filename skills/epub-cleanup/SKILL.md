---
name: epub-cleanup
description: 清洗已有 EPUB 的目录、版本、CSS、中文排版和标准弹注。按清洗 runbook 做已授权的局部变更，保留原件，先预检和 dry-run，再审查红线差异。
---

# EPUB 清洗

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### 目录结构规范化

只处理内部资源路径：先 format 保留文件名并归类目录，再 deobfuscate 按 manifest id 改名。正文和字体字节不变；包迁移另走 migrator。预检与写入保护见根 AGENTS。

### EPUB3 迁移

预检确认需要版本/结构迁移时；已经符合 EPUB3 且仅有局部排版问题，不重复迁移。范围包含 package metadata/properties、nav（保留 NCX）、XHTML shell、已识别注释与可选基础排版，不嵌入新字体。

### CSS 分层与清理

区分两件事：CLI 修补缺失分号、装饰分隔行与已知旧字体链；新增样式、语义拆层和冲突级联需人工判断。先读 [SPEC §7](../../docs/final/SPEC-实现约束.md) 的完整分层契约；便签边框按需读 [专门指南](../../docs/how-to/note-box-border-styles.md)。

### 中文字体与正文节奏

处理 CJK 字体链、行距/缩进/长 token 与角色样式。改字体前读 [SPEC §3、§4、§8](../../docs/final/SPEC-实现约束.md)；新增 alias/文件/class 前读 [字体命名规范](../../docs/final/字体别名命名规范.md)。保留既有自由/锁定模式，不把“更好看”自动解释为锁定正文字体。

### 标准弹注

标准注释转换、迁移后复核或弹注失联。先完整读 [SPEC §1](../../docs/final/SPEC-实现约束.md)，参照 [标准 fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)；多看旧版额外兼容另走 legacy skill。

## 调什么

### 多本书按序预览或写出

当用户要对一本或多本已有 EPUB 执行同一组确定性清洗步骤时，可用 `epub clean` 汇总每本书的审计、SHA 链、步骤 findings 和红线结果：

```sh
# 默认全链预览，只写每本书的 .clean.json 汇总
epub clean "before.epub" --out "clean-preview"

# 审查汇总后，显式写出最终候选
epub clean "books/" --out "clean-approved" --approve --jobs 2
```

默认步骤为 `normalize,migrate,css,typography`。`--steps` 可按该顺序选择其中若干步；`--jobs` 必须大于 0。目录输入递归查找 EPUB，输出目录须在输入目录外；已有产物不会覆盖。失败候选和报告要结合人工 diff review 分析，不得更新成最新通过版本。完整状态、产物命名与限制见[清洗 runbook](../../docs/pipeline/cleanup-flow.md)。

### 目录结构规范化

```sh
epub run epub.structure.normalize --input "before.epub" --output "normalized.epub" --dry-run --json
# 人工审查两阶段映射后实跑；报告目录须已存在
epub run epub.structure.normalize --input "before.epub" --output "normalized.epub" --json > normalize-envelope.json
epub redline --check all --path-map normalize-envelope.json "before.epub" "normalized.epub"
```

排障才用 `mode=format|deobfuscate|inspect`。`mode=inspect` 仍属单输出调用，非 dry-run 会写未修改副本，不称“无输出只读”。明确授权且确认标准字体混淆时，运行加 `allow_font_obfuscation=true`，红线加 `--allow-font-obfuscation`；不能用于未知加密。

### EPUB3 迁移

```sh
epub run epub.package.nav.audit --input "before.epub" --json
epub run epub.package.migrate.epub3 --input "before.epub" --output "candidate.epub" --dry-run --json
# 审查后，保持与 dry-run 相同的选项
epub run epub.package.migrate.epub3 --input "before.epub" --output "candidate.epub" --json
epub run epub.notes.popup.normalize --input "candidate.epub" --json
epub run epub.package.nav.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

按用户明确范围选择 `no_popup_notes=true` / `no_typography=true`，不要用默认值悄悄扩展为未授权排版。输出必须是新路径。

### CSS 分层与清理

```sh
epub run epub.css.layering.optimize --input "before.epub" --output "candidate.epub" --dry-run --json
# 审查后
epub run epub.css.layering.optimize --input "before.epub" --output "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

不要请求 `merge_scoped_local_css=true`：该语义归并已因 lossless 安全停用，只产生 warning，不会改 link/body class。

### 中文字体与正文节奏

```sh
epub run epub.typography.optimize --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.typography.optimize --input "before.epub" --output "candidate.epub" --json preset=literary-cn
# 局部试样：精确指定 ZIP 内 spine XHTML 路径；预览后用同样参数去掉 --dry-run
epub run epub.typography.optimize --input "before.epub" --output "sample.epub" --dry-run --json preset=literary-cn 'scope_paths=["OEBPS/Text/chapter.xhtml"]'
epub redline --check all "before.epub" "candidate.epub"
```

preset 为 `literary-cn`（默认）、`academic-cn`、`classical-annotated-cn`；仅自定义库才用 `preset_dir=`。省略 scope_paths 是整书替换预设层，但逐字节保留已有 `Styles/fonts.css` 及 OPF 字体元数据；正文链须已位于该字体层且与元数据一致，模式冲突或无法安全识别的导入、动态、内联正文链会拒绝处理。提供非空 JSON 路径数组则只向所选章节追加内容寻址的独立 CSS，保留原链接与共享样式。不支持携带 url()/@import 的局部 preset，不能借此安装资源或自动改正文结构。字体资源/链变化后另跑 `epub run epub.font.coverage.analyze --input "candidate.epub" --json`。

### 标准弹注

```sh
# 名字含 normalize，但当前只校验，不改书
epub run epub.notes.popup.normalize --input "book.epub" --json
```

已授权 EPUB3 迁移时，用 migrator 转换经识别的 plain/Sigil/Duokan 结构；它还会改 package/nav/shell，并非“仅弹注”写工具。仅授权弹注时人工修改对应资源，不借此扩大到全书迁移。修改后再跑本命令和全项 redline。

## 返回怎么读

### 目录结构规范化

前缀 `epub.structure.normalize.`：`mappings[]`（from/to）、`warnings[]`、`movedResources`、`renamedResources`、`rewrittenFiles`、`fontObfuscationResources`、`removedStaleEncryptionResources`；默认双阶段另有 `stages[]`。保存完整信封作为 path-map，不手抄映射。

### EPUB3 迁移

前缀 `epub.package.migrate.epub3.`：`packageVersionBefore`、`navEntries`、`xhtmlFilesUpdated`、manifest/metadata 改动计数、`plainNotesConverted`、`duokanNotesNormalized`、`warnings` 与开关回显。
输入/输出 SHA 在信封 input/output，不在 facts。

### CSS 分层与清理

前缀 `epub.css.layering.optimize.`：`cssFilesBefore/After`、`fontDeclarationsRewritten`、`warnings`；`semanticFactoringDisabled=true`、`scopedMergeDisabled=true`、`duplicateDeduplication=disabled` 是当前策略。旧的拆分/去重计数键保留但为 0，不代表功能可用。

### 中文字体与正文节奏

前缀 `epub.typography.optimize.`：`preset/layers/notes`、`coverage`、`stylesheetActions[]`（add/replace/keep）、`xhtmlLinkFiles`、`manifestItemsAddedHrefs`。整书模式另有 `fontMode=free|locked` 与 `fontModeAction=preserve`，保留字体层的 action 为 `keep`。局部模式另有 `applicationMode=scoped-additive` 与 `scopePaths`。
**coverage 是 preset 类覆盖率，不是字形覆盖率。** `typography.low-coverage` 表示结构与 preset 不匹配。

### 标准弹注

前缀 `epub.notes.popup.normalize.`：`noterefs`、`text_files`、`violations`；`error popupnotes` 的 title/location 给出文件和问题。
迁移报告的 `plainNotesConverted/duokanNotesNormalized/warnings` 只表示识别/处理结果，不证明所有原注释都被识别。

## 依据返回怎么判断

### 目录结构规范化

- dry-run 在内存完成两个阶段并检查真实候选，mappings 与候选一致；不写文件。审查映射/冲突/警告及红线后实跑，产物再用同次实跑信封作 path-map 复核。预览出现红线 error 不能按“尚未应用”忽略。
- `markup scan stopped at byte offset N` 表示后续引用未改写：先修源，不能直接接受候选。其他断链逐项检查。
- 缺失字体 URL 不猜文件、不删声明，保留 `local()` fallback；非字体资源断链须修复。stale encryption 只移除目标已不存在的引用。
- 真实未知加密停止；保守重写失败不一概推断是 DRM，读取具体 events/findings。实跑红线失败保留候选供 diff，不覆盖原件。通过后人工检查路径/链接，再判断是否需要 EPUB3 迁移。

### EPUB3 迁移

- 审查 warnings、注释识别范围和样式/导航计划；模糊注释保留转人工，不部分猜转。不要把转换计数等同于原注释总数。
- DRM/未知加密停止；保守重写失败读取具体事件，不绕过保护或覆盖输出重试。
- 实跑红线失败保留候选，逐项分析；元数据版本等预期迁移变化与意外正文/顺序变化分开解释，不能用宽泛豁免掩盖。
- 产物预检与注释验证后，人工 diff OPF/nav/NCX、封面、spine、注释及 CSS；需要精排才分派专项 skill，迁移成功不是阅读器通过。

### CSS 分层与清理

- 核对具体声明改动，字体链变化不能误伤既有嵌入角色；不以“CSS 数量越少”作为成功标准。即使 CSS 逐字节相同，独立 manifest id、refines/fallback 或 @import 也可能赋予不同语义，不自动删除副本。
- 人工归层：字体/角色绑定进 fonts；通用正文进 base；notes/effects/literary/media/vertical/poster 按组件职责。XHTML 先依赖后覆盖，manifest 只保留实际资源。
- 局部 CSS 的引用集合、相对 URL 或级联关系不清时保留，不猜测可合并；不得整文档重序列化。EPUB/WebKit 前缀与基础 fallback 不因浏览器正常而删掉。
- 红线失败保留候选，定位原因；通过后还需 diff 和相关页面的视觉检查，正文不变并不证明 CSS 外观不变。

### 中文字体与正文节奏

- 审查 add/replace 的具体样式表，尤其避免覆盖原书有效设计；只需局部改动时，不为用能力而套整套 preset。先做代表性正文/章首/复杂页样例，确认视觉节奏再推广。
- 自由模式 body/普通 p 不设 font-family；锁定入口为 fonts.css 的 body 规则并同步 OPF 字体 meta/prefix。局部字体不要求全书锁定；保留既有模式及明确书级例外。
- 设计字体/少量补字使用专用角色；局部子集不可拿来锁全书。C1-body 须按 CSS 继承覆盖全部角色用字和标点；保留 generic fallback，不靠重复嵌入字体解决回退。
- 字体声明进 fonts.css，正文节奏进 base.css；注释结构进 notes.css。带 epub namespace 的选择器须正确声明 namespace。
- 自由版与锁定版从同一内容基线派生，允许差异与 modified 时间规则按 SPEC §8；红线和字体覆盖都通过后，仍需大字号/窄屏阅读器验收。

### 标准弹注

- 同一 XHTML 中：图片 noteref anchor（epub:type、role、唯一 id）→ li.footnote-item 的 id → ◎ backlink 返回 trigger；每文件最多一个 aside[epub:type=footnote]，内有 ol.footnote-list。声明 epub namespace。
- 保留已有图标 src/alt；仅缺图标时使用 [note.png](assets/note.png) 并补 manifest。结构/视觉写 notes.css，字体声明写 fonts.css，图标规则限定作用域，不影响普通 sup。
- 不逐条创建 aside，不搬到跨文件 note body，不 display:none、不复制第二份注释、不改注释文字。
- 模糊结构先逐条配对；Sigil 分组内有无法匹配的 aside 或附加内容时不做部分合并。尽量保留原 id。
- 零违反还须对照原 noteref/note 数量、正文与每个触发目标，避免“没有被扫描到”误作正确。红线只允许表示控件差异，注释正文必须保持；多条 note 的弹窗范围需目标阅读器实测。
