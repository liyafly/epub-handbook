---
name: epub-vertical-ruby-optimizer
description: 优化 EPUB 竖排、Ruby 注音、中西文混排方向和非海报竖排正文页。用于竖排文字横倒、裁切、Ruby 间距异常、或页面需要阅读器安全 vertical-rl CSS 但不能变成固定版式时。
---

# EPUB 竖排与 Ruby 优化

这个 skill 用于竖排正文页和 Ruby 行为。全页海报/A-lite 叠加文本使用 `epub-alite-converter`。

## 固定目标

竖排正文页保持可重排：

- `body.page-vrl` 标记页面。
- `.vrl-section` 承载竖排 writing context。
- `writing-mode: vertical-rl` 带 EPUB/WebKit 前缀 fallback。
- `text-orientation` 明确处理混排文字方向。
- Ruby 保留语义化 `ruby`、`rt`、`rp`。
- 不使用 absolute positioning、viewport sizing 或 fixed-layout package metadata。

## CSS 模式

```css
body.page-vrl {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
}

.vrl-section {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
}
```

Ruby 保持语义：

```html
<ruby>漢<rt>かん</rt><rp>（</rp><rt>かん</rt><rp>）</rp></ruby>
```

源文件已经有有效 `rt` 时，保留注音文本，不额外复制 fallback 可见文字。

## 工作流

1. 读取目标 XHTML 和已加载 CSS 层。
2. 判断页面类型：
   - 横排正文中的 inline Ruby。
   - 整页竖排正文。
   - 海报式 A-lite 叠加。
3. inline Ruby 默认样式与 `.has-ruby` 行距兜底放 `base.css`。
4. 竖排正文页使用 `body.page-vrl` 和 `vertical.css` 中的 `.vrl-section`。
5. 保留所有真实文本、注音文本和阅读顺序。
6. `text-combine-upright` 只用于短数字或标记，并在确认阅读器支持后使用。
7. 只有新增 XHTML fixture 或移动页面文件时才更新 OPF/nav。

## 禁止事项

- 不把竖排正文转成图片。
- 普通竖排正文页不使用 fixed layout。
- 不删除 EPUB/WebKit writing-mode 前缀 fallback。
- 不混用 poster shell 类和 `body.page-vrl`。
- mixed orientation 更可读时，不强制所有 Latin 字母直立。
- 除非明确制作 fallback，不把 `rt` 文本复制成额外可见正文。

## 验证 fixture

- `Text/02-ruby-note.xhtml`：inline Ruby + notes。
- `Text/10-text-effects.xhtml`：Ruby + 文字效果。
- `Text/14-vertical-body.xhtml`：非海报竖排正文。
- `Text/03-vertical-alite.xhtml`：A-lite 对照；这个页面用 A-lite skill。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

调用示例：

```sh
# 预览
<skill-invocation> work/before/source.epub > work/dry-run.json

# 审查
cat work/dry-run.json | jq

# 确认后执行
<skill-invocation> --commit work/before/source.epub
```

dry-run 输出格式见 [docs/pipeline/cleanup-flow.md](../../docs/pipeline/cleanup-flow.md)。
