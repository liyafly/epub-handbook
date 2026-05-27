# 典型脏 EPUB 模式目录

> 状态：模式 + 推荐 skill 顺序；用作 `epub-layout-auditor` 决策的具体落地参考。
> 对应 SPEC：[§10 能力清单](../final/SPEC-实现约束.md)。

## 怎么用本目录

1. 跑 `python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub`。
2. 对照本目录的「特征」找匹配模式。
3. 按推荐 skill 顺序执行。
4. 每步后跑 `validate_text_invariance.py` + diff 工具确认。

## 模式 A：网上下载的扫描书

特征：章节几乎全是 `<img>`，文字段落极少，体积主要来自图片。

判断：不属于清洗范畴；OCR 是另一条链路。

推荐：用 `epub-source-intake` 重新 OCR / source 化，再回到本流水线。

## 模式 B：出版社旧版 EPUB 2 -> EPUB 3 升级

特征：只有 `toc.ncx`、缺 `nav.xhtml`、大量内联样式、footnote 没有 `epub:type`。

推荐 skill 顺序：

1. `epub-package-nav-auditor`
2. `epub-css-layering-optimizer`
3. `epub-popup-footnote-converter`
4. `epub-typography-optimizer`
5. `epub-legacy-footnote-fallback`（可选）

## 模式 C：Fan-made / 自制 EPUB

特征：Sigil 风格 id、字体引用断链、章节锚点不稳、manifest properties 缺失。

推荐 skill 顺序：

1. `epub-package-nav-auditor`
2. `epub-typography-optimizer`
3. `epub-css-layering-optimizer`
4. `epub-image-layout-optimizer`（如有图）

## 模式 D：自己旧作品 / 早期模板

特征：早期 class 命名、OPF properties 推断缺失、未使用当前 CSS 分层。

推荐 skill 顺序：

1. `epub-package-nav-auditor`
2. `epub-layout-auditor`
3. `epub-alite-converter`（可选）

## 模式 E：技术书 / 教科书

特征：MathML、表格、代码块、术语表、注脚密集。

推荐 skill 顺序：

1. `epub-package-nav-auditor`
2. `epub-literary-structure-formatter`
3. `epub-popup-footnote-converter`
4. `epub-css-layering-optimizer`
5. `epub-kindle-compatibility-checker`

## 模式 F：合集 / 大部头古籍

特征：章节数量多、nav 层级深、可能含 Ruby、文白对照、多列布局。

推荐 skill 顺序：

1. `epub-package-nav-auditor`
2. `epub-vertical-ruby-optimizer`
3. `epub-literary-structure-formatter`
4. `epub-typography-optimizer`

## 模式 G：英文 / 双语 epub

特征：大段英文正文、首字下沉、译注、插图。

推荐 skill 顺序：

1. `epub-english-typography-optimizer`
2. `epub-typography-optimizer`
3. `epub-popup-footnote-converter`
4. `epub-image-layout-optimizer`

## 模式 H：「看上去没问题」的 epub

特征：epubcheck 通过，各 reader 能开，只有 CSS 分层或命名小问题。

推荐：不必清洗。绿线问题不值得动。

## 通用建议

- 永远不要并行执行多个 skill。
- 遇到红线立即停。
- 每步产出 `after/step-N.epub`，结果不满意可回滚。
