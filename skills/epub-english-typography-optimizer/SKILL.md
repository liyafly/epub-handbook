---
name: epub-english-typography-optimizer
description: 为英文小说、散文、非虚构或诗剧制定并人工实施可重排排版，处理语言、serif 链、段落和断字；不套 CJK 规则，当前无自动排版 runner。
---

# EPUB 英文排版

## 何时用

英文为主的书籍；先判断小说/散文、非虚构、诗剧或双语的结构区别。按 [SPEC §5.9](../../docs/final/SPEC-实现约束.md) 与 [英文 fixture](../../templates/epub-style-demo/OEBPS/Text/18-english-fiction.xhtml) 选样例，不把小说参数强加给诗行、列表或表格。

## 调什么

`epub.typography.english.optimize` 当前未实现；按用户授权人工修改 XHTML/CSS，不反复调用占位能力。检查入口：

```sh
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

涉及弹注时加 popup validator，涉及 demo 时走 demo skill；公共返回见 [索引](../README.md)。

## 返回怎么读

layout findings 只证明静态扫描结果；红线证明所选内容边界。把静态发现、实际阅读器现象、尚未验证项分别列出，不把人工实施描述成自动能力已落地。

## 依据返回怎么判断

- 声明可靠的 lang/xml:lang，用短 serif 链；未验证断字时左对齐，不强制 justify。小说首段无缩进、后续约 1.2–1.5em、少段距；非虚构看层级，诗剧保留行与 speaker。
- 真实文本优先；首字装饰用 ::first-letter，旧 span/drop cap 必须复核朗读、复制和大字号。嵌入字体要有设计/覆盖/平台理由，不为英文排版默认嵌字。
- 居中插图为默认；需要环绕才转 image skill。保留原文、引号、拼写、诗行与锚点。
- 选章首和连续正文做候选比较，覆盖窄屏与大字号；交付目标包含 Kindle、Readest、Apple Books 时分别实测，缺项明确标注。
