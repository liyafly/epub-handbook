# 清洗记录：红线变更反例

> 类型：自造反例 demo
> 生成命令：`bash samples/demo-books/build.sh`

## 目标

故意制造一对不合格 before / after，用来演示 `validate_text_invariance.py` 如何拦住正文改写。

## 覆盖点

- 样式变化。
- 正文句子被改写。
- 元数据、spine 和封面保持不变。

## 验证

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/redline-trap-before.epub \
  samples/demo-books/dist/redline-trap-after-text-changed.epub \
  --check all
```

预期：退出码 1，并报告 `text: modified ...`。

## 用法

这对文件只用于红线反例和 diff 文本层演示，不可作为合法清洗交付样本。
