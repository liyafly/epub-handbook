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

`epub.typography.english.optimize` 当前未实现；按用户授权人工修改 XHTML/CSS，不反复调用占位能力。检查入口：

```sh
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

涉及弹注时加 popup validator，涉及 demo 时走 demo skill；

### 文学结构精排

`epub.literary.structure.format` 当前未实现；人工读取目标 XHTML 和 literary.css，按授权最小改写。角色模糊时先分析：

```sh
epub run epub.text.content.analyze --input "before.epub" --json
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

### 竖排与 Ruby

`epub.vertical.ruby.optimize` 当前未实现；按授权人工调整后：

```sh
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

若涉及弹注另跑 popup validator；

### 旧版弹注 fallback

`epub.notes.legacy-fallback` 当前未实现；人工叠加 hooks 后只读验证：

```sh
epub run epub.notes.popup.normalize --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

## 返回怎么读

### 英文排版

layout findings 只证明静态扫描结果；红线证明所选内容边界。把静态发现、实际阅读器现象、尚未验证项分别列出，不把人工实施描述成自动能力已落地。

### 文学结构精排

blockList 的 evidence/confidence 用于角色复核，不是自动套 class 的许可；layout 是静态风险，redline 不能证明美观。

### 竖排与 Ruby

静态扫描/红线不验证排版引擎的实际文字方向或 Ruby 行高；必须分别报告源码结构、内容边界与目标阅读器结果。

### 旧版弹注 fallback

前缀 `epub.notes.popup.normalize.`：`noterefs/text_files/violations`；`error popupnotes` 的 title/location 指向问题。零违反只代表结构合格，不证明旧多看交互正确。

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
