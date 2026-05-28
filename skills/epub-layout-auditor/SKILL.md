---
name: epub-layout-auditor
description: 审核 EPUB、XHTML、CSS、OPF、nav 和模板改动的排版质量、阅读器兼容性与项目规则覆盖。用于 EPUB 排版优化前后、格式 review、对比 develop 或 fixture、识别缺失技能、分派到专项 EPUB skills、输出优先级修复计划。
---

# EPUB 排版审稿

把这个 skill 作为 EPUB 排版工作的第一轮入口：先判断改了什么、风险在哪里、该由哪个专项 skill 处理。

## 审核流程

1. 确定范围：
   - 在本仓库工作时，先与用户指定的基线分支对比。
   - 若用户场景是「改造已有 EPUB」（而非「从零做新书」），本 skill 是入口；按 [SPEC §10](../../docs/final/SPEC-实现约束.md) 的红 / 黄 / 绿规则决定改动范围；每次改动跑 `scripts/validate_text_invariance.py` 红线 gate，红线触发立即回滚。人工 review 通过外部 diff 工具（Calibre Editor / VS Code，见 [根 README #epub-diff-review](../../README.md#epub-diff-review)）完成。
   - 典型脏 epub 模式对照 [docs/pipeline/cleanup-patterns.md](../../docs/pipeline/cleanup-patterns.md)。
   - 读取改动过的 XHTML、CSS、OPF、nav、NCX、图片/字体资源和相关文档。
   - 区分用户是只要审稿，还是要审稿后直接修复。
2. 给受影响页面分类：
   - 普通可重排正文。
   - 英文小说 / 英文非虚构正文。
   - 注释 / popup footnote。
   - 前置页 / 章首 / 小说文学结构。
   - 图片混排 / 封面 / 全幅海报。
   - 竖排正文 / Ruby / 中西文混排方向。
   - A-lite 海报页。
   - MathML / 代码 / 表格密集页。
   - Kindle 专项风险页。
3. 对照本地规则：
   - `docs/final/SPEC-实现约束.md`
   - `docs/final/EPUB 3 终极实践手册.md`
   - `docs/final/EPUB 3 HTML CSS 属性速查表.md`
   - `templates/epub-style-demo/SCENE_MATRIX.md`
   - `docs/final/reader-matrix.yaml`
4. 按优先级输出发现：
   - P0：EPUB 无效、manifest 链接断裂、页面不可读、缺关键资源。
   - P1：阅读器兼容回归、Kindle 转换风险、注释目标不匹配。
   - P2：版式脆弱、CSS 分层漂移、字体链欠整理、fallback 弱。
   - P3：润色、命名一致性、重复规则、文档未同步。
5. 如果用户要修复，切到最窄的专项 skill，保持改动范围可控。

## 分派规则

- `epub-css-layering-optimizer`：CSS 文件边界、加载顺序、规则归位。
- `epub-typography-optimizer`：中文正文节奏、字体链、嵌入字体策略。
- `epub-english-typography-optimizer`：英文书籍类型判断、英文段落节奏、serif 链、hyphenation 和大字号回归。
- `epub-literary-structure-formatter`：章首、前置页、对话、诗、信件。
- `epub-image-layout-optimizer`：figure 环绕、图注、图片格式、封面资源。
- `epub-vertical-ruby-optimizer`：竖排正文、Ruby、文字方向。
- `epub-package-nav-auditor`：OPF manifest/spine、nav、NCX、metadata、资源引用。
- `epub-kindle-compatibility-checker`：Kindle/KDP 静态风险与转换日志跟进。
- `epub-popup-footnote-converter`：标准 grouped popup notes。
- `epub-legacy-footnote-fallback`：在标准弹注上叠加多看旧版 fallback。
- `epub-alite-converter`：可重排全页海报 / 封面式 A-lite 页面。
- `epub-style-demo-maintainer`：fixture、reader matrix 与最终规则维护。

## 审稿清单

- XHTML 能按 XML 解析，链接只指向存在的本地资源。
- 每个被引用的 CSS 都在 OPF manifest 声明，并至少被一个 XHTML 使用。
- spine 的每个 idref 都能找到 manifest item，阅读顺序符合 nav/NCX 预期。
- 注释与 noteref 位于同一 XHTML，且每个文件只使用一个 grouped aside。
- 普通正文页不同时使用 `width:100%` 与左右 padding。
- 图片环绕使用 `figure.img-left` / `figure.img-right`，不直接 float `img`。
- Kindle 主路径避免 WebP 和未经验证的 SVG-only 封面路径。
- 含 MathML 的 XHTML 在 OPF 中声明 `properties="mathml"`。
- CSS 改动留在正确层级文件中。
- 阅读器结论必须能追溯到 fixture、matrix、转换日志或人工测试记录。

## 禁止事项

- 不改写正文、不重排章节、不替换图片、不重新设计整本书，除非用户明确要求。
- 不虚构阅读器 pass/fail；未复测时写 `warn` 或待验证说明。
- 局部 XHTML/CSS 能解决时，不做大范围重构。
- 不把可重排内容转成固定版式来绕过排版问题。
- 没有 fixture 或 matrix 记录时，不把兼容例外直接写进最终手册。

## 验证

按改动范围运行相关子集：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
git diff --check
```
