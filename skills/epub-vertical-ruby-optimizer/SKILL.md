---
name: epub-vertical-ruby-optimizer
description: 优化 EPUB 竖排、Ruby 注音、中西文混排方向和非海报竖排正文页。用于竖排文字横倒、裁切、Ruby 间距异常、或页面需要阅读器安全 vertical-rl CSS 但不能变成固定版式时。
---

# EPUB 竖排与 Ruby 优化

## 何时用

- 竖排正文页和 Ruby 行为优化；全页海报/A-lite 叠加文本用 `epub-alite-converter`。
- 固定目标：竖排正文页保持可重排——`body.page-vrl` 标记页面；`.vrl-section` 承载竖排 writing context；`writing-mode: vertical-rl` 带 EPUB/WebKit 前缀 fallback；`text-orientation` 明确处理混排文字方向；Ruby 保留语义化 `ruby`、`rt`、`rp`；不使用 absolute positioning、viewport sizing 或 fixed-layout package metadata。
- 禁止事项：不把竖排正文转成图片；普通竖排正文页不用 fixed layout；不删除 EPUB/WebKit writing-mode 前缀 fallback；不混用 poster shell 类和 `body.page-vrl`；mixed orientation 更可读时不强制所有 Latin 字母直立；除非明确制作 fallback，不把 `rt` 文本复制成额外可见正文。

## 调什么

本 skill 是 AI 分析与手工精排类 skill：读目标 XHTML 和已加载 CSS 层，判断页面类型（横排正文中的 inline Ruby、整页竖排正文、海报式 A-lite 叠加）后落地 CSS。改书后必须跑校验组合：

```sh
epub run epub.notes.popup.normalize --input <产物> --json    # 涉及弹注时
epub run epub.style.demo.maintain --input <demo 产物> --json # 涉及 demo 模板时
epub redline --check all <before.epub> <after.epub>          # 每次改书后
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- `epub run epub.notes.popup.normalize` 的 facts：`noterefs`、`text_files`、`violations`；violations 对应 `error popupnotes` findings。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过。

## 依据返回怎么判断

- CSS 模式：`body.page-vrl` 与 `.vrl-section` 都用三套 `writing-mode: vertical-rl`（标准 + `-webkit-` + `-epub-`）前缀；`.vrl-section` 另加 `text-orientation: mixed` 三套前缀。
- Ruby 保持语义（`<ruby>漢<rt>かん</rt><rp>（</rp><rt>かん</rt><rp>）</rp></ruby>`）；源文件已有有效 `rt` 时保留注音文本，不额外复制 fallback 可见文字。
- 分层归位：inline Ruby 默认样式与 `.has-ruby` 行距兜底放 `base.css`；竖排正文页用 `body.page-vrl` 和 `vertical.css` 中的 `.vrl-section`；只有新增 XHTML fixture 或移动页面文件时才更新 OPF/nav（交给 `epub-package-nav-auditor`）。
- `text-combine-upright` 只用于短数字或标记，并在确认阅读器支持后使用。
- `findings` 出现 `error`（含 `popupnotes`、`redline.*`）→ 回滚或修复后重跑；`status == approval-required` → 停下来问人。
- fixture 参考：`Text/02-ruby-note.xhtml`（inline Ruby + notes）、`Text/10-text-effects.xhtml`（Ruby + 文字效果）、`Text/14-vertical-body.xhtml`（非海报竖排正文）、`Text/03-vertical-alite.xhtml`（A-lite 对照，用 `epub-alite-converter`）；新规则先落 demo，实测后回写 `docs/final/reader-matrix.yaml`。
