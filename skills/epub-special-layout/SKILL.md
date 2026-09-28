---
name: epub-special-layout
description: 处理英文排版、文学结构、竖排与 Ruby、多看旧版弹注 fallback；按书型做最小改写，保留正文和可重排结构，兼容性以目标阅读器实测为准。
---

# EPUB 专项排版

## 何时用

本入口按任务合并相关技能。所有能力的公共返回、授权边界与验收说明统一见[技能索引](../README.md)。

### 英文排版

英文为主的书籍；先判断小说/散文、非虚构、诗剧或双语的结构区别。按 [SPEC §5.9](../../docs/final/SPEC-实现约束.md) 与 [英文 fixture](../../templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml) 选样例，不把小说参数强加给诗行、列表或表格。

### 文学结构精排

页面角色已确认，需要语义结构和视觉节奏。章首图读 [chapter-head-image](../../docs/how-to/chapter-head-image.md)，文白读 [classical-modern-layout](../../docs/how-to/classical-modern-layout.md)，文集导航读 [anthology-navigation](../../docs/how-to/anthology-navigation.md)，只加载任务命中的指南；硬边界仍以 [SPEC](../../docs/final/SPEC-实现约束.md) 为准。

### 竖排与 Ruby

先区分横排内联 Ruby、整页竖排正文与海报叠字；海报走 A-lite。对照 [竖排 fixture](../../templates/epub-style-demo/OEBPS/Text/14-vertical-body.xhtml) 与 [Ruby fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)，兼容判断查 reader matrix。

### 旧版弹注 fallback

目标包含 Duokan legacy 才使用。先读 [SPEC §1](../../docs/final/SPEC-实现约束.md) 与 [兼容 fixture](../../templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml)；非标准结构先交 popup skill。

## 调什么

### 英文排版

先用 `epub.typography.english.optimize` dry-run 检查 spine XHTML 的语言声明，再审阅计划并写出新候选。该能力只补齐 `html` 根节点的 `lang` / `xml:lang`；字体链、段落节奏、对齐、断字、首字装饰与插图仍按 SPEC §5.9 逐项判断，不由此能力修改。

```sh
epub run epub.typography.english.optimize --input "before.epub" --dry-run --json
epub run epub.typography.english.optimize --input "before.epub" --output "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

可显式传 `lang=en-GB`，或传 `scope_paths='["OEBPS/Text/chapter.xhtml"]'` 限定 spine XHTML；路径须精确匹配 spine 项。省略 scope 且 OPF `dc:language` 主语言与目标语言不同时，能力不扫描页面也不写入，返回 `english.opf-language-differs-requires-scope`；只有明确选择页面后才继续。OPF 语言匹配时，无语言页面若没有 Unicode 字母文本或 CJK 比例达到阈值会跳过。显式 scope 中已有其他主语言则报 error，且整本零编辑。审阅 `plannedEdits[{path,action,value}]`、`skipped[{path,reason}]`、`filesScanned` 与 `editCount`；`lang-mismatch` 表示不猜测两种声明中的哪一个正确，显式 scope 下的 `opf-language-differs` 仅提示，能力不改 OPF。

涉及弹注时加 popup validator，涉及 demo 时走 demo skill；

### 文学结构精排

角色已经人工确认后，用 `epub.literary.structure.format` 按显式清单追加 class。能力只接受 SPEC §7 与文白指南列出的词表，不会推断页面角色；目标必须精确指定为元素 `id`，或 `tag` 加该文件内同名元素的零基 `index`。可选 `stylesheet` 必须是 manifest 已有 CSS。先 dry-run 审阅计划，再写出新候选：

```sh
epub run epub.literary.structure.format --input "before.epub" --dry-run --json \
  'assignments=[{"path":"OEBPS/Text/chapter.xhtml","id":"epigraph","class":"epigraph"}]' \
  'stylesheet=OEBPS/Styles/literary.css'
epub run epub.literary.structure.format --input "before.epub" --output "candidate.epub" --json \
  'assignments=[{"path":"OEBPS/Text/chapter.xhtml","tag":"blockquote","index":0,"class":"epigraph"}]'
epub redline --check all "before.epub" "candidate.epub"
```

角色尚未确认时，先用 `epub.text.content.analyze` 与 `epub.layout.audit` 收集证据，并由人复核后再填写清单。一次运行出现任何 error finding 时整批零写入；既有 class 与已链接 stylesheet 都是 no-op。

### 竖排与 Ruby

用 `epub.vertical.ruby.optimize` 做单一机械修补：`op=ruby-rp` 为没有 fallback 的直接 `<rt>` 添加 `<rp>` 括号；`op=writing-mode-prefix` 为 manifest CSS 中精确的标准 `writing-mode` 补齐缺失的 WebKit/EPUB 前缀。一次只运行一个 op；可用 `scope_paths` 指定 spine XHTML 或 manifest CSS 路径。`rp_open` / `rp_close` 默认是全角括号，也可设置为一个安全字符。先 dry-run 审阅计划，再向新路径写候选：

```sh
epub run epub.vertical.ruby.optimize --input "before.epub" --dry-run --json op=ruby-rp
epub run epub.vertical.ruby.optimize --input "before.epub" --output "candidate.epub" --json op=ruby-rp
epub redline --check all "before.epub" "candidate.epub"
epub run epub.vertical.ruby.optimize --input "before.epub" --dry-run --json op=writing-mode-prefix
```

Ruby 包含 `rtc`、嵌套 Ruby、带命名空间前缀或已有不完整 `rp` 时会跳过并报告；自闭合 `rt`、空白且无子元素的 `rt` 报 `vertical.ruby-empty-rt`，只有 `●○◎△▽・﹅﹆` 着重号符号的 `rt` 报 `vertical.ruby-emphasis`，都不会补括号。标准 writing-mode 值不受支持、厂商前缀冲突或 CSS 无法解析时也不会猜测。一次运行发现任何 error 时整批零写入。涉及弹注另跑 popup validator。

### 旧版弹注 fallback

先用 popup validator 检查标准 grouped footnote；`standardViolations` 排除 Duokan legacy class 缺失项。若它为 0，legacy fallback 可补齐部分缺失的 Duokan class；若大于 0，fallback 会拒绝写入。写入能力会自动运行同一上游校验。审阅 dry-run 计划后，写到新候选：

```sh
epub run epub.notes.popup.normalize --input "before.epub" --json
epub run epub.notes.legacy-fallback --input "before.epub" --dry-run --json
epub run epub.notes.legacy-fallback --input "before.epub" --output "candidate.epub" --json
epub run epub.notes.popup.normalize --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

只处理精确的 spine XHTML 子集时，可在 dry-run 与实跑命令中加 `scope_paths='["OEBPS/Text/chapter.xhtml"]'`；每个路径必须属于 spine XHTML。检查 `facts["epub.notes.legacy-fallback.plannedEdits"]`、`editCount`、`filesScanned` 和 `skipped`。标准 popup 违规、目标路径不在 spine、结构不支持或属性不能安全编辑时，finding 为 error 且本能力零写入；只有 Duokan class 缺失时可补齐。范围内没有 noteref 时返回 `notes-fallback.no-notes` info，不修改文件。

## 返回怎么读

### 英文排版

`english.already-declared` 与 `english.declared-on-body` 表示没有改动；`english.skipped-other-lang`、`english.skipped-cjk-text`、`english.skipped-no-text` 与 `english.lang-mismatch` 表示该页被保留。`english.opf-language-differs-requires-scope` 表示 OPF 主语言与目标语言不同，未提供 scope 时整本未扫描且未写入；显式范围出现 `english.lang-conflict` 时全书不应用计划中的修改。显式 scope 下的 `english.opf-language-differs` 仅提示。layout findings 只证明静态扫描结果；红线证明所选内容边界。把静态发现、实际阅读器现象、尚未验证项分别列出。

### 文学结构精排

检查 `facts["epub.literary.structure.format.plannedEdits"]`、`skipped`、`assignmentsTotal` 与 `editCount`。`literary.target-ambiguous`、`literary.target-forbidden` 等 error 表示整批未修改；redline 检查内容和锚点边界，不证明角色选择或视觉效果正确。

### 竖排与 Ruby

审阅 `facts["epub.vertical.ruby.optimize.op"]`、`plannedEdits`、`skipped`、`filesScanned` 和 `editCount`；`vertical.ruby-empty-rt` 与 `vertical.ruby-emphasis` 表示该 Ruby 不会生成括号 fallback。静态扫描/红线不验证排版引擎的实际文字方向或 Ruby 行高；必须分别报告源码结构、内容边界与目标阅读器结果。

### 旧版弹注 fallback

前缀 `epub.notes.popup.normalize.`：`noterefs/text_files/violations/standardViolations`；`standardViolations` 不含 Duokan legacy class 缺失。`error popupnotes` 的 title/location 指向问题。零违反只代表结构合格，不证明旧多看交互正确。fallback 的 `plannedEdits` 列出 path/tag/id/addClass，`skipped` 说明未选文件、无注释文件或已有目标 class；标准错误或其它 fallback error 会清空计划且不应用 class 编辑。

## 依据返回怎么判断

### 英文排版

- 声明可靠的 lang/xml:lang，用短 serif 链；未验证断字时左对齐，不强制 justify。小说首段无缩进、后续约 1.2–1.5em、少段距；非虚构看层级，诗剧保留行与 speaker。
- 真实文本优先；首字装饰用 ::first-letter，旧 span/drop cap 必须复核朗读、复制和大字号。嵌入字体要有设计/覆盖/平台理由，不为英文排版默认嵌字。
- 居中插图为默认；需要环绕才转 image skill。保留原文、引号、拼写、诗行与锚点。
- 选章首和连续正文做候选比较，覆盖窄屏与大字号；交付目标包含 Kindle、Readest、Apple Books 时分别实测，缺项明确标注。

### 文学结构精排

- 先以普通章首、连续正文、诗/信件或文白复杂页建立一致样例，再推广可复用类；通过字号、间距、对齐和少量装饰形成层次，不重新设计原书。
- 章首保留真实 h1，装饰图用 figure；版权页保留真文本和信息顺序，不猜补书目事实；不将普通章首自动转换 A-lite。
- 文白先按源序上下；短组宽屏可增强浮动，长组/窄屏允许分页，不用 table/flex/grid 承载正文对照。具体比例和类名复用指南，不另造体系。
- 组件样式归 literary.css；不改变对话标点、诗行、译文或 mixed-content 空白。nav 文案同步属于另行明确的范围。
- 用 [场景矩阵](../../templates/epub-style-demo/SCENE_MATRIX.md) 找 frontmatter/chapter-head/classical-modern 对照；新兼容结论走 demo 实测，不仅靠浏览器效果。

### 竖排与 Ruby

- 整页正文用 body.page-vrl 与 .vrl-section，vertical-rl 带标准/WebKit/EPUB 前缀；text-orientation: mixed，不强制所有 Latin 直立。样式归 vertical.css，不能混用 poster shell。
- Ruby 保留一份注音：`<ruby>漢<rp>（</rp><rt>かん</rt><rp>）</rp></ruby>`。已有有效 rt 不重复复制；inline Ruby 和 .has-ruby 行距兜底归 base.css。
- text-combine-upright 只用于短数字/标记并经目标阅读器验证；不用图片、固定页高或 absolute positioning 替代真实文字。
- 对照普通/大字号、混排和分页检查裁切；新规则用最小 fixture 实测后写入 matrix，不把未测效果标为 pass。

### 旧版弹注 fallback

- 标准属性/中性类保留；anchor 加 duokan-footnote 且内含图标；ol.footnote-list 加 duokan-footnote-content；li.footnote-item 仅加 duokan-footnote-item，不把 content 类放 li。
- 同文件一个 aside/ol，noteref 指向唯一 li，◎ backlink 返回原 trigger；不得复制可见 note list、display:none 隐藏正文或用 JS。
- 保留现有图标 src/alt，缺少才复用项目资源并同步 manifest。样式并入活动 notes.css，分隔线只留一套，不影响普通上标。
- 多 note 页面逐个点击确认只打开对应 li；红线失败保留候选分析，注释文字不得借兼容修改。EPUB2 外壳只按 [兼容指南](../../docs/how-to/epub2-popup-note-compatibility.md) 记录目标版本实测，不能标为严格 EPUB2 标准。
