---
name: epub-special-layout
description: 处理英文排版、文学结构、竖排与 Ruby、多看旧版弹注 fallback；按书型做最小改写，保留正文和可重排结构，兼容性以目标阅读器实测为准。
---

# EPUB 专项排版

## 何时用

公共返回、授权边界与验收说明见[技能索引](../README.md)。

选择组合效果时查[配方目录](../../templates/recipes/README.md)，按证据状态区分历史实测与候选；选用片段放书级 `design.css`，字体用独立 `fonts.css`，标准弹注用 `notes.css`。不自动改套已有书的分层设计。

- 英文排版：英文为主的书；先判断小说/散文、非虚构、诗剧或双语。按 [SPEC §5.9](../../docs/final/SPEC-实现约束.md) 与[英文 fixture](../../templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml) 选样例，不把小说参数强加给诗行、列表或表格。
- 文学结构精排：页面角色已确认，需要语义结构和视觉节奏。只加载命中的指南：[章首图](../../docs/how-to/chapter-head-image.md)、[文白](../../docs/how-to/classical-modern-layout.md)、[文集导航](../../docs/how-to/anthology-navigation.md)；硬边界以 [SPEC](../../docs/final/SPEC-实现约束.md) 为准。
- 竖排与 Ruby：先区分横排内联 Ruby、整页竖排正文与海报叠字（海报走 A-lite）。对照[竖排 fixture](../../templates/epub-style-demo/OEBPS/Text/14-vertical-body.xhtml) 与 [Ruby fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)，兼容判断查 reader matrix。
- 旧版弹注 fallback：目标包含 Duokan legacy 才用。读 [SPEC §1](../../docs/final/SPEC-实现约束.md) 与[兼容 fixture](../../templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml)；非标准结构先跑 `epub.notes.popup.normalize`。

## 调什么

参数以 `epub capabilities --id <id>` 为准；四个能力都写入，先 `--dry-run` 审阅计划，再向新路径写候选，最后 `epub redline --check all`。一次运行出现任何 error finding 时整批零写入。

```sh
epub run epub.typography.english.optimize --input "before.epub" --dry-run --json [lang=en-GB] ['scope_paths=["OEBPS/Text/chapter.xhtml"]']
epub run epub.literary.structure.format --input "before.epub" --dry-run --json \
  'assignments=[{"path":"OEBPS/Text/chapter.xhtml","id":"epigraph","class":"epigraph"}]' 'stylesheet=OEBPS/Styles/literary.css'
epub run epub.vertical.ruby.optimize --input "before.epub" --dry-run --json op=ruby-rp
epub run epub.notes.legacy-fallback --input "before.epub" --dry-run --json
```

- 英文：只补 `html` 根的 `lang` / `xml:lang`，字体链、节奏、断字、首字装饰与插图仍按 SPEC §5.9 逐项判断。省略 scope 且 OPF 主语言与目标语言不同时，不扫描不写入，返回 `english.opf-language-differs-requires-scope`，须明确选择页面。
- 文学结构：角色须人工确认，能力不推断角色；目标是精确元素 `id`，或 `tag` + 文件内零基 `index`；class 只接受 SPEC §7 与文白指南的词表。角色未确认时先用 `epub.text.content.analyze` 与 nav audit 收集证据。
- 竖排/Ruby：`op=ruby-rp`（给无 fallback 的直接 `<rt>` 加全角括号）与 `op=writing-mode-prefix`（补缺失的 WebKit/EPUB 前缀）一次只跑一个。
- 旧版弹注：先跑 `epub.notes.popup.normalize`，其 `standardViolations`（不含 Duokan class 缺失）为 0 才能补 Duokan class；大于 0 时 fallback 拒绝写入。写入后再跑 popup normalize 复检。
- 涉及弹注加 `epub.notes.popup.normalize`，涉及 demo 走 `epub-reader-verify`。

## 返回怎么读

- 英文：`english.already-declared` / `declared-on-body` 表示无改动；`skipped-*` 与 `lang-mismatch`（不猜哪个声明正确）表示该页被保留；`parse-failed`、`unsupported-encoding`、`self-closing-html`、`lang-conflict`（显式 scope 中已有其他主语言）阻止整批写入；显式 scope 下的 `opf-language-differs` 仅提示，能力不改 OPF。
- 文学结构：`plannedEdits`、`skipped`、`assignmentsTotal`；redline 只证明内容和锚点边界，不证明角色选择或视觉正确。
- 竖排/Ruby：`vertical.ruby-empty-rt`、`vertical.ruby-emphasis` 表示该 Ruby 不生成括号 fallback；`rtc`、嵌套、带命名空间前缀或不完整 `rp` 会跳过。静态扫描与红线不验证实际文字方向或 Ruby 行高。
- 旧版弹注：`standardViolations` 不含 Duokan class 缺失；零违反只代表结构合格，不证明多看交互正确。
- 以上共同：静态发现、红线边界、阅读器现象、待验证项分别列出。

## 依据返回怎么判断

### 英文排版

- 声明可靠的 lang/xml:lang，用短 serif 链；未验证断字时左对齐，不强制 justify。小说首段无缩进、后续约 1.2–1.5em，非虚构看层级，诗剧保留行与 speaker。
- 真实文本优先；首字装饰参考 [drop-cap 配方](../../templates/recipes/drop-cap/recipe.md)，旧 span/drop cap 须复核朗读、复制和大字号。嵌入字体要有设计/覆盖/平台理由。
- 普通英文整段误成斜体时，先查实际载荷、阅读器字体覆盖和同族 `@font-face` 映射；确认误匹配后可试独立斜体别名并显式绑定 `em`。见 [Reeden 单书修复](../../docs/pipeline/reader-adaptation-lessons-2026-10.md#2-reeden英文正文为什么全变成斜体后来怎样改对)；不全局清除斜体，不把此绕过自动套给其他书。
- 居中插图为默认，需要环绕才转 `epub.image.layout.optimize`。保留原文、引号、拼写、诗行与锚点。选章首和连续正文做候选，覆盖窄屏与大字号；目标含 Kindle、Readest、Apple Books 时分别实测。

### 文学结构精排

- `literary.class-not-allowed`：从 SPEC §7 或文白指南确认词表。`path-not-in-spine`、`target-not-found`、`target-ambiguous`、`target-forbidden`：核对精确 spine 路径与 id 或 tag/index，不扩大到相似元素。`unsafe-attribute`、`unsupported-encoding`、`parse-failed`、`no-head`：文件不能安全编辑，先修复再重跑。`stylesheet-not-in-manifest`：改用 manifest 中的 CSS；`already-has-class`、`stylesheet-already-linked` 是已满足项。
- 先以普通章首、连续正文、诗/信件或文白复杂页建立一致样例，再推广可复用类；用字号、间距、对齐和少量装饰形成层次，不重新设计原书。
- 章首保留真实 h1，装饰图用 figure；版权页保留真文本和信息顺序，不猜补书目事实；不把普通章首自动转 A-lite。
- 文白先按源序上下，宽屏增强见 [parallel-pair 配方](../../templates/recipes/parallel-pair/recipe.md)，不用 table/flex/grid 承载正文对照。
- 组件样式归 literary.css；不改变对话标点、诗行、译文或 mixed-content 空白。新兼容结论走 demo 实测，场景见[场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md)。

### 竖排与 Ruby

- `vertical.scope-not-in-spine` / `scope-not-css-manifest-item`：`scope_paths` 改为精确路径。`unsupported-encoding`、`xhtml-parse-failed`、`css-parse-failed`：输入无法安全扫描，先修复，不接受部分计划。
- `vertical.ruby-has-rp` 表示 fallback 已齐；`partial-rp`、`complex`、`prefixed`、`empty-rt`、`emphasis` 表示跳过，按源结构判断，不强行补括号。
- `vertical.writing-mode-unsupported-value`：仅支持 `vertical-rl`、`vertical-lr`、`horizontal-tb` 且不带 `!important`；`vertical.prefix-conflict`：先人工解决标准与厂商前缀值冲突。
- 整页正文用 body.page-vrl 与 .vrl-section，`text-orientation: mixed`，样式归 vertical.css，不混用 poster shell。Ruby 只保留一份注音；`text-combine-upright` 仅用于短数字并经目标阅读器验证；不用图片、固定页高或 absolute 替代真实文字。
- 新规则用最小 fixture 实测后写入 matrix，不把未测效果标为 pass。

### 旧版弹注 fallback

- `notes-fallback.upstream-not-clean`：先独立修复标准弹注错误（看 `standardViolations`）再重跑。`no-notes` 是 info，无需写入。
- `notes-fallback.no-grouped-list`：输入已是标准 footnotes 结构，多看 fallback 尚未定义（id 放 li 还是内嵌 aside 待实测）；不改结构，向用户说明并记录。
- `notes-fallback.unsupported-encoding`、`notes-fallback.parse-failed`、`notes-fallback.multiple-lists`、`notes-fallback.noteref-without-icon`、`notes-fallback.content-class-on-li`、`notes-fallback.unsafe-attribute`：先人工修复对应文件结构或属性，再重跑 popup normalize 与 fallback。
- `notes-fallback.upstream-coverage-mismatch`：上游 noteref 数与 fallback 扫描数不一致；核对 spine 项和 popup 计数，不接受部分 class 变更。
- 弹注按书二选一并写入制作说明：必须严格 EPUB 3 → 标准结构；其他书 → 多看兼容回退。同一本书不混用。
- 标准结构：`aside[epub:type=footnotes] > div.footnote-list > aside.footnote-item[epub:type=footnote][role=doc-footnote]`，noteref 指向单条 aside；容器不标 footnote。
- 多看 fallback 暂用旧 grouped 结构：标准属性/中性类保留；anchor 加 duokan-footnote 且内含图标；ol.footnote-list 加 duokan-footnote-content；li.footnote-item 仅加 duokan-footnote-item，不把 content 类放 li。
- 同文件一个注释容器，noteref 指向唯一单条注释，◎ backlink 返回原 trigger；不得复制可见 note list、display:none 隐藏正文或用 JS。
- 保留现有图标 src/alt；缺图标时由制作方提供资源并同步 manifest。样式并入活动 notes.css。多 note 页面逐个点击确认只打开对应条目；注释文字不得借兼容修改。EPUB2 外壳只按[兼容指南](../../docs/how-to/epub2-popup-note-compatibility.md)记录目标版本实测，不标为严格 EPUB2 标准。
