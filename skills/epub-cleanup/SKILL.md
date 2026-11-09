---
name: epub-cleanup
description: 清洗已有 EPUB 的目录、版本、CSS、中文排版、字体子集和标准弹注。按清洗 runbook 做已授权的局部变更，保留原件，先预检和 dry-run，再审查红线差异。
---

# EPUB 清洗

## 何时用

公共返回、授权边界与验收说明见[技能索引](../README.md)；预检与写入保护见根 AGENTS。

- 目录结构规范化：只处理内部资源路径，先 format 保留文件名并归类目录，再 deobfuscate 按 manifest id 改名；正文和字体字节不变。
- EPUB3 迁移：预检确认需要版本/结构迁移时；已符合 EPUB3 且仅有局部排版问题则不重复迁移。范围是 package metadata/properties、nav（保留 NCX）、XHTML shell 与已识别注释，不应用排版样式、不嵌入新字体。遇到 `<big>` 保留原标签。
- CSS 分层与清理：CLI 只修补缺失分号、装饰分隔行与已知旧字体链；新增样式、语义拆层和冲突级联需人工判断。分层契约见 [SPEC §7](../../docs/final/SPEC-实现约束.md)，便签边框见[专门指南](../../docs/how-to/note-box-border-styles.md)。
- 中文字体与正文节奏：CJK 字体链、行距/缩进/长 token 与角色样式。改字体前读 [SPEC §3、§4、§8](../../docs/final/SPEC-实现约束.md)，新增 alias/文件/class 前读[字体命名规范](../../docs/final/字体别名命名规范.md)。保留既有自由/锁定模式，不把“更好看”解释为锁定正文字体。
- 完整字体重新子集化：完整字体母版在共享字体库、以书级配置锁定 SHA；子集化的输入是注入全量母版后的源 EPUB，不能把上次子集当母版。
- 标准弹注：转换、迁移后复核或弹注失联。完整读 [SPEC §1](../../docs/final/SPEC-实现约束.md)，参照[标准 fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)；多看旧版兼容另走 `epub-special-layout`。

## 调什么

参数以 `epub capabilities --id <id>` 为准。写入类先 `--dry-run` 审查，再向新路径实跑，最后 `epub redline --check all`。

```sh
epub run epub.structure.normalize --input "before.epub" --output "normalized.epub" --dry-run --json
epub redline --check all --path-map normalize-envelope.json "before.epub" "normalized.epub"
epub run epub.package.migrate.epub3 --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.css.layering.optimize --input "before.epub" --output "candidate.epub" --dry-run --json
epub run epub.font.subset --input full-font-source.epub --output subset-candidate.epub --json [font_config=fonts.json]
epub run epub.notes.popup.normalize --input "book.epub" --json
```

- 批量：`epub clean <路径> --out <目录> [--steps normalize,migrate,css] [--approve]`。默认只审计出计划；`--steps` 只接受按序的 `normalize,migrate,css`，不编排排版；`--approve` 在末次审计和全项红线通过后才写出候选。字段与限制见[清洗 runbook](../../docs/pipeline/cleanup-flow.md)。
- normalize 实跑时把信封存为文件供红线 `--path-map`。`mode=inspect` 仍属单输出调用，非 dry-run 会写未修改副本。仅在明确授权且确认标准字体混淆时加 `allow_font_obfuscation=true`（红线加 `--allow-font-obfuscation`），不能用于未知加密。
- `epub.package.migrate.epub3` 迁移后依次跑 `epub.notes.popup.normalize`、nav audit、全项红线。
- `epub.notes.popup.normalize` 只读校验，不转换 plain/Sigil 旧尾注；转换旧尾注须先取得正文修改授权，走正文校订流程。
- `epub.font.subset` 调独立 `epub-font` provider；缺失、目标字体混淆、母版不匹配或验证失败时停止且不写候选。`fonts.json` 顶层 `"usage": "css"` 仅在样式表简单且明确要按角色缩小字符集时设置；不确定时删除，用全书模式。构建流程见[工作区指南](../../docs/pipeline/book-workspace.md)。
- preset 是新书起点：用 `epub book new <dir> --from-preset <name>` 复制进书级 `design.css`，不注入已有书。已含 `epub-preset-*.css` 的书，该 CSS 已属本书，直接编辑；已有书的排版增强走配方（[templates/recipes](../../templates/recipes/README.md)）与书级设计。字体资源/链变化后另跑 `epub.font.coverage.analyze`。

## 返回怎么读

前缀均为能力 id + `.`。

- `epub.structure.normalize.mappings[]` 是后续红线的路径映射；把完整 normalize 信封作为 `--path-map`，不要手抄。
- `epub.package.migrate.epub3`：输入/输出 SHA 在信封 input/output，不在 facts。
- `epub.css.layering.optimize`：`semanticFactoringDisabled=true`、`scopedMergeDisabled=true`、`duplicateDeduplication=disabled` 是当前策略；旧拆分/去重计数键保留但为 0，不代表功能可用。
- `epub.font.subset`：`changedFonts` 是已替换字节的字体路径，`providerReport` 含输入/输出 SHA 与逐字体统计。`font-subset.usage-fallback` 表示 CSS 角色范围被保守扩大；`font-subset.not-in-master` 是缺字警告，确认 fallback 能覆盖；`font-subset.provider-missing` 是 provider 缺失错误；`font-subset.stale-input` 须先对未修改的源 EPUB 子集化再做其他改书步骤。
- `epub.notes.popup.normalize`：`violations` 与 `error popupnotes` 的 title/location 给出文件和问题。迁移报告的 `duokanNotesNormalized/warnings` 只表示处理结果，不证明所有原注释合规。

## 依据返回怎么判断

### 目录结构规范化

- dry-run 在内存完成两个阶段并检查真实候选；审查映射/冲突/警告及红线后实跑，产物再用同次实跑信封作 path-map 复核。预览出现红线 error 不能按“尚未应用”忽略。
- `markup scan stopped at byte offset N` 表示后续引用未改写：先修源，不能接受候选。
- `structure.non-utf8-text`：按 finding 的 `location` 人工转码为 UTF-8 并重新冻结 S0 底本后再运行，不接受部分改写。
- 缺失字体 URL 不猜文件、不删声明，保留 `local()` fallback；非字体资源断链须修复。stale encryption 只移除目标已不存在的引用。
- 真实未知加密停止；保守重写失败不一概推断是 DRM，读具体 events/findings。红线失败保留候选供 diff。

### EPUB3 迁移

- 审查 warnings、注释识别范围和样式/导航计划；模糊注释保留转人工，不部分猜转，转换计数不等于原注释总数。
- 红线失败时，元数据版本等预期迁移变化与意外正文/顺序变化分开解释，不用宽泛豁免掩盖。人工 diff OPF/nav/NCX、封面、spine、注释及 CSS；迁移成功不是阅读器通过。

### CSS 分层与清理

- 核对具体声明改动，字体链变化不能误伤既有嵌入角色；不以“CSS 数量越少”为成功标准。CSS 逐字节相同，独立 manifest id、refines/fallback 或 @import 也可能语义不同，不自动删副本。
- 归层：字体/角色绑定进 fonts；通用正文进 base；notes/effects/literary/media/vertical/poster 按组件职责。引用集合、相对 URL 或级联不清时保留，不得整文档重序列化；EPUB/WebKit 前缀与基础 fallback 不因浏览器正常而删。
- 正文不变不证明 CSS 外观不变，通过红线后仍需 diff 和相关页面视觉检查。
- nav audit 的 `css-root-font-size` error：从 `html` / `:root` / `*`（含 reset 选择器列表）删除 `font-size` / `font`，其余属性保留，列表里的 `body` 可一并删。`css-body-font-size` info 不单独处理；`css-fixed-font-size` warn 改相对 `em` / `%`，元素内无文字时可记录理由保留（SPEC §5.5.1）。改后重跑审计与全项红线，并在目标阅读器放大两级复查。
- `css-page-color-lock` warn：`html` / `:root` / `*` / 普通 `body` 写了页面级 `color` / `background`，会在夜间与护眼主题下削弱对比度（SPEC §7）。须人工判断：该色是否有设计意图、改为继承后是否可读；确需保留的记录理由。不批量删除，A-lite 的 `poster-*` modifier 已豁免。

### 中文字体与正文节奏

- 只需局部改动时，不为用能力而套整套预设层；先做代表性正文/章首/复杂页样例，确认节奏再推广。审查 add/replace 的具体样式表，避免覆盖原书有效设计。
- 自由模式 body/普通 p 不设 font-family；锁定入口为 fonts.css 的 body 规则并同步 OPF 字体 meta/prefix。局部字体不要求全书锁定，保留既有模式及书级例外。
- 设计字体/少量补字用专用角色；锁定正文须按 CSS 继承覆盖全部角色用字和标点，链尾保留 generic，不靠重复嵌入字体解决回退。
- 字体声明进 fonts.css，正文节奏进 base.css，注释结构进 notes.css；带 epub namespace 的选择器须声明 namespace。
- 自由版与锁定版从同一内容基线派生，差异规则按 SPEC §8；红线和覆盖都通过后，仍需大字号/窄屏阅读器验收。

### 完整字体重新子集化

- 审查 `changedFonts` 与 `providerReport`：替换路径、输入/输出 SHA、每字体 action；无变化时确认 MATH 字体是否按策略保留。成功后做全项 redline。
- 启用 `usage: "css"` 时逐字体确认 `usage.mode`，任何 `usage-fallback` 须看 detail，接受较大字集后才继续。静态验证不能代替阅读器实测。

### 标准弹注

- 同一 XHTML 中：图片 noteref anchor（epub:type、role、唯一 id）→ 单条 `aside.footnote-item[epub:type=footnote][role=doc-footnote]` → ◎ backlink 返回 trigger；每文件最多一个 aside[epub:type=footnotes] 容器（不带 doc-footnote），内有 div.footnote-list。声明 epub namespace。
- 保留已有图标 src/alt；新建图片 noteref 由用户提供资源并同步 manifest，迁移器不注入默认图标。图标规则限定作用域，不影响普通 sup。
- 不把整组注释标成 epub:type=footnote；不跨文件、不 display:none、不复制第二份注释、不改注释文字。
- `popupnotes.legacy-container` warn：旧 grouped 结构没有挂多看 class。按书选择路径：必须严格 EPUB 3 时改为 footnotes 容器 + 逐条 aside；其他书补齐 duokan-* 走多看兼容回退。两种都保留原 id、href 与回跳。
- 人工转换旧 Sigil 逐条 aside 时，先逐条核对 noteref 与 aside，再收进同一 footnotes 容器；有无法匹配的 aside 或附加内容时不做部分合并，尽量保留原 id。
- 零违反还须对照原 noteref/note 数量、正文与每个触发目标，避免“没扫到”误作正确。红线只允许表示控件差异，注释正文必须保持；多条 note 的弹窗范围需目标阅读器实测。
