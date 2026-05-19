# Skills 与模板维护建议

> 状态：维护指南，不改变 `epub-pro` 技术架构文档。

## Review 结论

当前仓库已经有清晰的三层：`docs/final/` 负责人读规范，`skills/` 负责半自动转换行为，`docs/source/` 与 `docs/experiments/` 负责溯源。需要补上的不是新的架构逻辑，而是两个可执行层：

1. skills 的维护边界：每个 skill 应说明触发场景、固定目标、禁止事项和验证清单。
2. 可运行模板：每个重要样式需要有最小 EPUB 样本，能快速打包到阅读器里实测。

这次新增 `templates/epub-style-demo/`，用于验证中文正文、列表、表格、Ruby、弹注、局部竖排和 A-lite 海报页。后续如果加入新的规则，优先补一个模板样本，再决定是否升级成 skill。

## 怎么做 Skills

Skill 不应写成完整程序，也不应把下游引擎实现细节放进来。它更适合写成稳定的转换契约：

- `description` 只写触发条件和目标能力，保持短句。
- `Fixed Target` 写最终结构，不写多套备选方案。
- `Workflow` 写读什么、改什么、保留什么、怎么验证。
- `Guardrails` 写禁止事项，尤其是会破坏 EPUB 兼容性的捷径。
- `Validation` 指向可运行模板或检查项，避免只停留在文本规则。

新增或修改 skill 时，建议同步检查：

- 这个能力是否已经写进 `docs/final/EPUB 3 终极实践手册.md`。
- HTML 速查表是否有对应条目和预览。
- `templates/epub-style-demo/` 是否有最小样本能验证。
- 是否需要新增独立模板，而不是把所有场景塞进一个 skill。

## 当前手册建议

不建议把下游实现细节继续加进手册。当前手册最有价值的定位是：告诉制作者“该写什么结构、该避免什么写法、为什么兼容”。

建议补充方向：

- 在速查 HTML 中把每条规则变成可视预览，方便像字体样张一样比较效果。
- 在模板目录维护可运行样本，作为阅读器实测入口。
- 在 `skills/README.md` 维护 skill 清单和验证方式。
- 保持 `docs/final/epub-pro 技术架构 v1.md` 不随日常样式实验变化。

## 模板目录约定

模板目录使用下面的结构：

```text
templates/
  README.md
  epub-style-demo/
    README.md
    build.sh
    mimetype
    META-INF/container.xml
    OEBPS/
      package.opf
      nav.xhtml
      Styles/base.css
      Text/*.xhtml
      Images/*.svg
```

模板应满足：

- 可以用系统自带 `zip` 打包，不依赖 npm、Python 包或下游实现仓。
- 生成的 EPUB 是最小样本，不追求完整书籍体验。
- 每个 XHTML 页面只验证一组相关样式，便于定位阅读器差异。
- 不内置第三方字体和版权图片。
- 生成产物放在模板自己的 `dist/` 下，默认不进入版本管理。
