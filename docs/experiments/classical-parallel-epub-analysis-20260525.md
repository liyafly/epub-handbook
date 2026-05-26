# 大部头文白对照 EPUB 结构分析

> 样本：本地大部头文白对照 EPUB（真实书名和路径不入库）  
> 日期：2026-05-25  
> 方法：只读解析 OPF / CSS / nav / XHTML 结构；不摘录正文内容。

## 概览

- EPUB 3.0，OPF 位于 `OEBPS/content.opf`，不是 demo 常见的 `package.opf`。
- zip entries：524。
- OPF manifest：521 items；spine：507 items。
- XHTML：507 个，其中正文卷 `chapter001.xhtml` 到 `chapter500.xhtml` 共 500 个。
- CSS：4 个。
- 图片：1 张封面图；正文几乎全是可选中文本。
- 字体：8 个 manifest item，zip 内也有 8 个字体文件。
- 导航：EPUB 3 nav + `toc.ncx` 都存在；封面有 `properties="cover-image"`。

本仓 harness 修正后会报一个真实问题：

```text
CSS url() target missing: Styles/Stylesheetcss.css -> ../Fonts/<missing-fallback-font>.otf
```

也就是说，主 CSS 里启用了某个补字字体的 `@font-face`，但该字体既不在 zip 内，也不在 OPF manifest 内。这里不记录真实字体文件名。

## 可吸收点

1. **超大古籍按卷拆 XHTML 是对的。**  
   500 卷分别拆成 500 个正文文件，平均每卷约 19.8 个条目 section，单文件压力可控。这个模式比整本一个 XHTML 更适合编辑、定位、阅读器恢复进度和错误二分。

2. **条目级目录可以服务检索型阅读，但必须自动生成。**  
   nav 约 7622 个链接，覆盖卷级入口和条目级锚点。对工具书、笔记小说、古籍合集很有价值；但这种目录不能手工维护，必须由章节标题或结构化 source 自动生成，并同步 NCX。

3. **文白对照不必用 table。**  
   样本用重复 section 承载每个条目，内部用不同段落类区分原文、白话译文和出处/注记。这个方向值得吸收：文白、双语、原注/译注都应保持真实段落和自然重排，不用表格或固定双栏作为主路径。

4. **古籍字体策略要区分“主字体”和“补字字体”。**  
   样本嵌入了正文、楷体、仿宋、标题字体，以及多个很小的罕用字/Unicode plane 补字字体。这说明古籍项目确实会遇到“全书可读性 + 少量生僻字”的组合；但多段补字字体越多，越容易出现 CSS 引用与 OPF/zip 不一致的问题。生产规则仍应优先使用单个全字符集字体或明确的 `.rare` 类，并让工具检查每个 `@font-face src` 是否真实存在。

5. **只有封面图的纯文本大部头很稳。**  
   正文没有把文本烘焙成图片，也没有内容图片依赖。对文言/白话对照类书，优先级应该是结构、锚点、字体覆盖和目录，不是视觉装饰。

6. **文白左右对照不能依赖 flex + orientation，也不能默认先 float。**
   样本的原文/译文每组已经有 `section.paracol` 包裹，两侧段落类也清楚；但左右布局主要写在 `@media (orientation: landscape)` 内，并使用 `display:flex`。这能服务 Apple Books / Readest 这类现代 EPUB 阅读器，却不足以作为 Kindle 主路径。Kindle 兼容版本应把基础结构保持为源序上下，然后只在足够宽的 `min-width` 媒体查询内用百分比宽度的 `float:left/right` 作为增强；media 或 float 不生效时自然保持上下显示。

7. **Kindle 专用 AZW3 的成功样本使用 table。**
   另一个本地英汉对照 AZW3 样本经 Calibre 只读展开后，正文主体是一个 `table`，每个段落对照是一行，左右 `td` 分别承载英文和中文；CSS 使用 `display: table`、`table-layout: fixed`、`width:100%`，单元格为 `50%`。这解释了为什么 Kindle 成品中能看到左右对照。但这不是通用 EPUB 主路径：KDP 质量规则不鼓励用表格承载非表格正文，且大字号/窄屏/辅助功能风险更高。本仓只把它记录为 Kindle 专用成品格式的参考，不写成 EPUB 推荐规则。

## 需要警惕的点

- 主 CSS 存在缺失字体引用，说明仅检查 OPF manifest href 不够；还必须扫描 CSS `url()`。
- OPF 文件名不固定，validator 不应硬编码 `OEBPS/package.opf`。
- CSS 中有 Print/PDF 和阅读器导航遗留样式，进入生产模板前应清理到明确分层。
- 条目锚点类似 `sigil_toc_id_N`，对生成流程可接受，但长期维护更推荐语义化稳定 id。
- 主 nav 极深。对参考书可接受；对普通小说/散文会过度，应按书籍类型决定目录粒度。

## 已吸收到本仓

- `scripts/epub_ai_harness.py`：把 `.otf/.ttf` 与 `application/vnd.ms-opentype` 正确计入字体；扫描 CSS `url()`，发现缺失资源或未进 OPF manifest 时报错。
- `scripts/test_epub_ai_harness.py`：新增缺失 CSS 字体 URL 的 smoke test。
- `scripts/validate_popup_notes.py`：通过 `META-INF/container.xml` 定位真实 OPF；无弹注的外部 EPUB 不再因为缺少 demo 专用 `Images/note.png` 失败。
- `docs/guides/anthology-navigation.md`：补充超大古籍/工具书的条目级目录规则。
- `docs/guides/fonts-css-expansion-plan.md`：补充 active CSS `url()` / `@font-face src` 必须同时存在于 zip 与 OPF manifest 的检查项。
- `templates/epub-style-demo/OEBPS/Text/21-classical-modern.xhtml`：文白 demo 改为样本式 `.parallel-pair > .classical-text + .modern-text`；默认上下，短组只在 `min-width:40em` 宽屏 media 内 float 增强，长组保留上下显示并允许分页。
- `docs/guides/classical-modern-layout.md`：把“不要双栏”修正为“不要 table/flex/grid/固定版式；允许 float 渐进增强”。

## 2026-05-26 Kindle float 复测

- demo 构建产物：`templates/epub-style-demo/dist/epub-style-demo-20260526-080944.epub`。
- Kindle Previewer 3.104.0 转换结果：Enhanced Typesetting 支持，0 errors，0 quality issues。
- GUI 可视结果：Kindle 电子书阅读器 profile、字号 4 下，`21-classical-modern` 的 `.parallel-pair` 原文/白话显示为左右两栏。
- 外部大部头样本生成了本地测试副本，输出位置不入库：同目录 `*.kindle-float-test.epub`。该副本把 `display:flex` + `orientation` 改为 `.paracol .bodyContent-1` / `.bodyContent-1-kaiti` 百分比 float，并移除缺失字体 URL。`epub_ai_harness.py` 未发现即时结构问题；Kindle Previewer 3.104.0 转换成功，Enhanced Typesetting 支持，0 errors，0 quality issues。
- 外部大部头样本的 KPF GUI 加载耗时过长，超过数分钟仍未暴露窗口；本轮不把实际书 GUI 视觉结果写成 pass，只记录转换通过和 demo GUI 通过。

## 2026-05-26 晚间 Kindle GUI 修正

- Kindle Previewer 3.104.0、Kindle E-reader profile、字号 4 下，短组默认 float 会出现半宽列、回目录链接和下一条目挤在同一页的问题；长组继承 `page-break-inside: avoid` 还会制造大空白页。
- 修正规则：`.parallel-pair` 默认上下并允许正常分页；`.parallel-float-pair` 只保护短组不拆页；真正的左右列移入 `@media (min-width: 40em)`。Kindle 电子墨水、小屏和大字号优先上下，不再强求左右。
- 回目录链接取消按钮边框，避免 Kindle 焦点框或分页残留抢占版面。

## 暂不吸收

- 不吸收“默认正文链里串多个嵌入补字字体”的做法。它适合特定古籍样本，但和本仓默认系统字体优先策略冲突；后续若要支持古籍专用构建，应作为 `forceAll` / `.rare` / 专项补字策略单独建模。
- 不把 7000+ 条主 nav 作为所有合集默认规则。它只适合检索型大部头；普通合集仍优先卷级 nav + 局部目录。
