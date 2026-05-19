# SPEC-实现约束（执行层）

> 本文件只记录**硬约束**，用于执行层实现与回归测试的硬约束清单。描述性解释请看《EPUB 3 终极实践手册》。

## 1) 弹注（Footnote Popup）

- `a[epub:type="noteref"]` 与对应 `aside[epub:type="footnote"]` 必须位于**同一 XHTML 文件**。
- 每个章节文件最多一个注释容器：`<aside epub:type="footnote" id="footnotes">`。
- 多条注释必须使用：`ol.footnote-list > li.footnote-item`。
- 每条注释必须可回跳，默认回跳符号 `◎`（U+25CE）。
- 字体子集化时，`◎`（U+25CE）必须包含在 codepoints 中；若正文主字体无该字形，回退为 `↑`（U+2191）或图片 backlink。

## 2) A-lite 页面约束

- 仅允许 reflowable EPUB；v1 不支持 FXL。
- 禁用 `position:absolute` 进行全页覆盖排版。
- 禁用 `vh` / `vw` 作为核心版式依赖。
- 海报页统一使用 A-lite 结构（容器 + 图片 + 文本层）并保持内容语义可访问。

## 3) 字体与 OPF

- 若启用书内字体嵌入，OPF 必须声明：`<meta property="ibooks:specified-fonts">true</meta>`（或等效 iBooks 指定字体开关写法）。
- 标题字体来源仅允许：书内嵌入字体 + 通用族回退（serif/sans-serif/monospace 等）。
- 字体策略必须与 `fontspec` 三态一致：`auto | forceAll | none`。

## 4) 子集策略算法（执行层对齐）

`auto` 模式下，子集字符集合 =
1. 全书 XHTML 实际用字
2. 角色映射要求字符（body / heading / quote / rare）
3. 用户 `extraCodepoints`
4. 硬约束字符（如 `◎`）

附加规则：
- 当角色字体本身即为人工子集（rare 专用字库），可按角色策略显式 `none`，避免重复裁切。

## 5) 结构化产物要求

- 输出包必须满足 EPUB `mimetype` 首条且 STORED（无压缩）规则。
- OPF 元数据、manifest/spine 的排序与稳定性必须可复现（便于 golden fixture diff）。
- 生成物应回写构建元数据：子集器名称/版本、字形统计、构建时间。
