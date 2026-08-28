# CSS 清洗与系统字体链

> 状态：流程文档；用于在 EPUB3 基线上收口重复 CSS、旧字体声明和互不交叠的局部样式。

本页的 `work/book-a/` 是流水线内部路径简写。新书级项目应将它置于 `work-epub/<book>/03 制作工作区/.pipeline/`，见 [一书一 Git 工作区](book-workspace.md)。

## 适用范围

这一步适合重复携带每册样式表、旧平台字体名较多的合订 EPUB。它不嵌入字体，不改写正文。

## 公共清洗

先保留不可修改的 before，再生成 EPUB3 基线。基线通过 preflight 后运行：

```sh
python3 scripts/epub_css_cleanup.py \
  work/book-a/intermediate/step-1-epub3.epub \
  --output work/book-a/after/final.epub \
  --merge-scoped-local-css \
  --format json > work/book-a/reports/css-cleanup.json
```

公共脚本只做可复用且可验证的变换：

- 合并完全重复 CSS；
- 将结构相同、少量属性不同的样式拆成公共层和 override；
- 将旧式宋体、黑体、楷体声明替换为四段以内的系统优先字体链；
- 同步 XHTML `<link>` 和 OPF CSS manifest；
- 可选把引用页面集合互不重叠的局部 CSS 改写为 `body.css-local-*` 作用域，并归并到一个 `clean-scoped-local.css`。

`--merge-scoped-local-css` 只处理可证明互不交叠、且不是多数页面共用层的局部样式。两个样式只要被同一个 XHTML 同时引用，就跳过这组并在报告中记录 warning，避免改变原有级联顺序。

## 验证

每次写出后至少运行：

```sh
unzip -tqq work/book-a/after/final.epub
python3 scripts/epub_preflight_harness.py \
  work/book-a/after/final.epub \
  --format json
bash scripts/validate-popup-notes.sh \
  --epub work/book-a/after/final.epub
python3 scripts/validate_text_invariance.py \
  work/book-a/intermediate/step-1-epub3.epub \
  work/book-a/after/final.epub \
  --check all
```

继续核对：

- OPF 和 `nav.xhtml` 能被 `xmllint` 解析；
- CSS link 不断链，OPF manifest 与 ZIP 内 CSS 数量一致；
- 归一化后不再存在重复 CSS；
- 图片、字体等二进制资源没有意外变化；
- 在 Calibre Editor 或 VS Code 做五层 diff review；
- 至少跑一个目标转换器或阅读器侧检查，并记录版本与日志摘要。

## 排版取舍

系统优先版适合作为第一阶段交付：正文宋体链、标题黑体链和语义角色字体彼此有层级，包体较小，也便于跨阅读器比较。嵌入字体应作为独立第二阶段：先确定哪些角色真正需要设计字体或生僻字补字，再做子集、manifest 和阅读器复测。
