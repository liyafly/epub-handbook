# 清洗记录：纸上花园观察录

> 类型：自造 demo
> 生成命令：`bash samples/demo-books/build.sh`

## 目标

演示诗段、Ruby、blockquote 和宽屏竖排增强的清洗结果。after 只调整 CSS 和非封面图片资源，不改正文。

## 覆盖点

- 诗段与换行。
- Ruby。
- blockquote。
- 列表。
- 宽屏 `writing-mode: vertical-rl` 增强。
- 非封面 SVG 资源改动。

## 验证

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/paper-garden-before.epub \
  samples/demo-books/dist/paper-garden-after-clean.epub \
  --check all
```

预期：退出码 0。

## Diff 概览

- 结构：manifest/CSS 资源变化，spine 不变。
- 文本：一致。
- 样式：诗段和竖排增强变化。
- 资源：章节插图颜色变化；封面不变。
- 元数据：核心字段不变。
