# 清洗记录：自动清洗正向演示

`loop-auto-fix-before.epub` 是多轮清洗 loop 的正向演示输入。章节 `OEBPS/Text/chapter.xhtml` 的根元素故意漏写 `xml:lang` / `lang`；正文保持完整且不需要人工判断。

运行：

```sh
bash templates/cleanup-demo-books/build.sh
python3 scripts/epub_cleanup_loop.py \
  templates/cleanup-demo-books/dist/loop-auto-fix-before.epub \
  --work-dir work/loop-auto-fix \
  --format json
```

预期：至少一轮 `applied` 含 `add-xml-lang`，最终产物写入 `work/loop-auto-fix/after/cleaned.epub`，并通过逐轮文本红线。
