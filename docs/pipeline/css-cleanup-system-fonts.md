# CSS 清洗与系统字体链

> 状态：流程文档；用于在 EPUB3 基线上修补旧字体声明并检查局部样式。

本页的 `$CUR` 与 `$W` 沿用 [cleanup-flow.md](cleanup-flow.md) 主线的当前候选和流水线工作目录。

## 适用范围

这一步适合重复携带每册样式表、旧平台字体名较多的合订 EPUB。它不嵌入字体，不改写正文。

## 公共清洗

先保留不可修改的 before，再生成 EPUB3 基线。基线通过结构审计后运行：

```sh
epub run epub.css.layering.optimize \
  --input "$CUR" \
  --output "$W/after/s5-<n>.epub" \
  --json > "$W/s5-<n>.json"
```

CSS 清洗只做保守修补（分号、装饰行、已知旧字体链）；去重、分层、scoped merge 已停用。

## 验证

每次写出后至少运行：

```sh
unzip -tqq "$W/after/s5-<n>.epub"
epub run epub.package.nav.audit --input "$W/after/s5-<n>.epub" --json
epub run epub.notes.popup.normalize --input "$W/after/s5-<n>.epub" --json
epub redline --check all [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s5-<n>.epub"
```

继续核对：

- OPF 和 `nav.xhtml` 能被 `xmllint` 解析；
- CSS link 不断链，OPF manifest 与 ZIP 内 CSS 数量一致；
- 对去重、分层或 scoped merge 的需求单独记录并人工处理；
- 图片、字体等二进制资源没有意外变化；
- 在 Calibre Editor 或 VS Code 做五层 diff review；
- 至少跑一个目标转换器或阅读器侧检查，并记录版本与日志摘要。

## 排版取舍

系统优先版可通过人工 CSS 建立正文、标题和语义角色字体层级。嵌入字体应作为独立第二阶段：先确定哪些角色真正需要设计字体或生僻字补字，再做子集、manifest 和阅读器复测。
