# loop-package-properties

这是 `epub_cleanup_loop.py` 的正向清洗样本。正文 XHTML 同时包含内联 SVG 与 MathML，输入 OPF 故意遗漏对应 manifest `properties`。

```sh
bash samples/demo-books/build.sh
rm -rf work/loop-package-properties
python3 scripts/epub_cleanup_loop.py \
  samples/demo-books/dist/loop-package-properties-before.epub \
  --work-dir work/loop-package-properties \
  --format json
```

预期结果：第一轮自动执行 `add-manifest-properties`，给正文 manifest item 增加 `svg mathml`；随后两轮无动作后以 `stopped_by: dry` 收敛；`cleaned.epub` 的 OPF 带 `epub-handbook:cleanup-rounds` 审计 meta。
