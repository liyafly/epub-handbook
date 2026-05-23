# Skills 与模板维护建议

> 状态：维护指南，不改变 `epub-pro` 技术架构文档。

## Review 结论

当前仓库保持三层职责：

- `docs/final/` 负责最终规则、读者可读解释和执行约束。
- `skills/` 负责半自动排版/转换行为，是给 Codex/Claude Code 使用的操作契约。
- `templates/epub-style-demo/` 负责最小可运行 fixture，用来验证规则在阅读器里的表现。

这次 skills 从少量转换契约扩展成一组完整排版工作流：先用 `scripts/epub_ai_harness.py` 判断输入类型；已有 EPUB 用 `epub-layout-auditor` 做总审稿；只有文本/PDF/HTML/OCR 结果时，用 `epub-source-intake` 建立可校对 source bundle；之后再按问题类型切到 CSS、中文字体、英文排版、文学结构、图文、竖排、Kindle、OPF/nav、弹注或 A-lite 专项 skill。

## 怎么做 Skills

Skill 不应写成完整程序，也不应把下游引擎实现细节放进来。它适合写成稳定的转换契约：

- `description` 写触发条件和目标能力，保持短句但覆盖关键词。
- `Fixed Target` / `固定目标` 写最终结构，不写多套备选方案。
- `Workflow` / `工作流` 写读什么、改什么、保留什么、怎么验证。
- `Guardrails` / `禁止事项` 写会破坏 EPUB 兼容性的捷径。
- `Validation` / `验证` 指向可运行模板或检查命令。
- 技能正文使用中文；目录名与 `name` 使用英文短横线。

新增或修改 skill 时，建议同步检查：

- 这个能力是否已经写进 `docs/final/EPUB 3 终极实践手册.md`。
- `docs/final/SPEC-实现约束.md` 是否有可执行规则。
- HTML 速查表是否有对应条目和预览。
- `templates/epub-style-demo/` 是否有最小样本能验证。
- `skills/README.md` 是否需要更新技能清单和运行方式。

## 使用顺序

1. 输入判断：`scripts/epub_ai_harness.py <path>` 输出输入类型、结构问题、推荐 skills 和下一步命令。
2. 源材料接入：没有 EPUB 时，使用 `epub-source-intake` 处理文本、Markdown、HTML、PDF 或 OCR 结果。
3. 总审稿：已有 EPUB 或 source tree 时，`epub-layout-auditor` 先看 diff、页面类型和风险优先级。
4. 专项修复：
   - CSS 层级：`epub-css-layering-optimizer`
   - 中文字体/正文：`epub-typography-optimizer`
   - 英文书籍排版：`epub-english-typography-optimizer`
   - 文学结构：`epub-literary-structure-formatter`
   - 图片：`epub-image-layout-optimizer`
   - 竖排/Ruby：`epub-vertical-ruby-optimizer`
   - Kindle：`epub-kindle-compatibility-checker`
   - OPF/nav：`epub-package-nav-auditor`
   - 弹注：`epub-popup-footnote-converter`，必要时追加 `epub-legacy-footnote-fallback`
   - A-lite：`epub-alite-converter`
5. fixture 与规则同步：`epub-style-demo-maintainer`。
6. 运行 build 和 validator，再决定是否更新最终手册、速查表或 reader matrix。

## Harness 与 Hook

`scripts/epub_ai_harness.py` 是给 AI 和人工共用的入口脚本。它只用 Python 标准库，能检查：

- `.epub` 的 zip/mimetype/container/OPF/nav/NCX/cover/MathML/图片格式风险。
- EPUB source tree 的 OPF manifest、spine、资源引用和推荐 skills。
- 文本/PDF/HTML/图片目录的 source-intake 路由。

输出 Markdown：

```sh
scripts/epub_ai_harness.py <path>
```

输出 JSON，便于 AI/harness 消费：

```sh
scripts/epub_ai_harness.py <path> --format json
```

`hooks/pre-commit.epub-handbook` 是可选 git hook 模板，不默认安装。需要时执行：

```sh
cp hooks/pre-commit.epub-handbook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

图片压缩、PDF 解析和 OCR 不在本仓实现。对应 workflow 只记录风险、工具边界和校验入口，真正压缩/抽取由外部工具完成。

## 运行命令

```sh
scripts/epub_ai_harness.py <path>
scripts/validate_skills_basic.py
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
git diff --check
```

`xmllint` 不可用时，Python validator 仍然要跑；它会解析 XML 并检查 manifest、链接、MathML、图片环绕等项目约束。

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
      toc.ncx
      Styles/base.css
      Text/*.xhtml
      Images/*.png
```

模板应满足：

- 可以用系统自带 `zip` 打包，不依赖 npm、Python 包或下游实现仓。
- 生成的 EPUB 是最小样本，不追求完整书籍体验。
- 每个 XHTML 页面只验证一组相关样式，便于定位阅读器差异。
- 不内置第三方字体和版权图片。
- 生成产物放在模板自己的 `dist/` 下，默认不进入版本管理。
