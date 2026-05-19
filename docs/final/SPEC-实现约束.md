# SPEC-实现约束（执行层）

> 本文件只记录**实现约束**，用于执行层实现与回归测试的约束清单。描述性解释请看《EPUB 3 终极实践手册》。

## 1) 弹注（Footnote Popup）

- `a[epub:type="noteref"]` 与对应 `aside[epub:type="footnote"]` 必须位于**同一 XHTML 文件**。
- 每个章节文件最多一个注释容器：`<aside epub:type="footnote" role="doc-footnote">`。
- `a[epub:type="noteref"]` 必须具有唯一 `id`，供注释回跳定位。
- 多条注释必须使用：`ol.footnote-list > li.footnote-item`。
- 每条注释必须可回跳，默认回跳符号 `◎`（U+25CE）。

## 2) A-lite 页面约束

- 仅允许 reflowable EPUB；v1 不支持 FXL。
- A-lite 页面 CSS 禁用 `position: absolute`。
- A-lite 页面 CSS 禁用 `vh` / `vw` 单位。
- 海报页 `<body>` 必须带 `class="fullpage"`，外层必须是 `<section class="fullframe" epub:type="chapter">`。
- 所有可见叠加文本必须为真实文本节点；不允许将全部可见文字仅以图片承载。

## 3) 字体与 OPF

- 若启用书内字体嵌入，OPF `<package>` 必须在 `prefix` 声明 ibooks 命名空间，并包含：`<meta property="ibooks:specified-fonts">true</meta>`。
- 标题字体来源仅允许：书内嵌入字体 + 通用族回退（serif/sans-serif/monospace 等）。
- 字体策略必须与 `fontspec` 三态一致：`auto | forceAll | none`。

## 4) 子集策略算法（执行层对齐）

`auto` 模式下，子集字符集合 =
1. 全书 XHTML 实际用字
2. 角色映射要求字符（body / heading / quote / rare）
3. 用户 `extraCodepoints`
4. 实现显式声明的额外字符；默认回跳符号 `◎` 走阅读器或系统 fallback，不强制进入各角色字体子集

附加规则：
- 当角色字体本身即为人工子集（rare 专用字库），可按角色策略显式 `none`，避免重复裁切。

## 5) 结构化产物要求

- 输出包必须满足 EPUB `mimetype` 首条且 STORED（无压缩）规则。
- OPF 元数据、manifest/spine 的排序与稳定性必须可复现（便于 golden fixture diff）。
- 生成物应回写构建元数据：子集器名称/版本、字形统计、构建时间。


## 6) Fixture 命名索引（M5 对齐）

- `01-basic-cjk`
- `02-footnotes`
- `03-fontspec-no-subset`
- `04-fontspec-subset`
- `05-vertical-cjk`
