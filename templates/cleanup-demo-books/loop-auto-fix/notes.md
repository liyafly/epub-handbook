# 清洗记录：自动清洗正向演示

`loop-auto-fix-before.epub` 是多轮清洗 loop 的正向演示输入。章节 `OEBPS/Text/chapter.xhtml` 的根元素故意漏写 `xml:lang` / `lang`；正文保持完整且不需要人工判断。

运行：

```sh
bash templates/cleanup-demo-books/build.sh
epub run epub.package.nav.audit \
  --input templates/cleanup-demo-books/dist/loop-auto-fix-before.epub \
  --json
```

预期：findings 中每章一条 `missing-html-lang`（`auto_fixable: true`），提示 `<html>` 根元素缺 `lang` / `xml:lang`。Go CLI 没有多轮自动 loop 命令；修复按 [清洗流水线](../../../docs/pipeline/cleanup-flow.md) 的固定顺序逐能力执行，最后用 `epub redline --check all` 验证正文不变。
