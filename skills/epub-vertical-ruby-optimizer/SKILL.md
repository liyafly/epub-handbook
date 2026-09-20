---
name: epub-vertical-ruby-optimizer
description: 人工修复 EPUB 竖排正文、Ruby 注音与中西文方向，保持可重排及前缀 fallback。用于横倒、裁切或注音异常；不处理 A-lite 海报骨架，当前无自动 runner。
---

# EPUB 竖排与 Ruby

## 何时用

先区分横排内联 Ruby、整页竖排正文与海报叠字；海报走 A-lite。对照 [竖排 fixture](../../templates/epub-style-demo/OEBPS/Text/14-vertical-body.xhtml) 与 [Ruby fixture](../../templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml)，兼容判断查 reader matrix。

## 调什么

`epub.vertical.ruby.optimize` 当前未实现；按授权人工调整后：

```sh
epub run epub.layout.audit --input "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

若涉及弹注另跑 popup validator；公共返回和 demo 验证见 [索引](../README.md)。

## 返回怎么读

静态扫描/红线不验证排版引擎的实际文字方向或 Ruby 行高；必须分别报告源码结构、内容边界与目标阅读器结果。

## 依据返回怎么判断

- 整页正文用 body.page-vrl 与 .vrl-section，vertical-rl 带标准/WebKit/EPUB 前缀；text-orientation: mixed，不强制所有 Latin 直立。样式归 vertical.css，不能混用 poster shell。
- Ruby 保留一份注音：`<ruby>漢<rp>（</rp><rt>かん</rt><rp>）</rp></ruby>`。已有有效 rt 不重复复制；inline Ruby 和 .has-ruby 行距兜底归 base.css。
- text-combine-upright 只用于短数字/标记并经目标阅读器验证；不用图片、固定页高或 absolute positioning 替代真实文字。
- 对照普通/大字号、混排和分页检查裁切；新规则用最小 fixture 实测后写入 matrix，不把未测效果标为 pass。
