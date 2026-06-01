# 合集分卷 A-lite 与精简交付设计

## 背景

本轮在已有 EPUB 清洗结果上继续精排，不重跑或推翻既有 EPUB3、弹注、系统字体链和 CSS 去重逻辑。

目标书属于多册合订 EPUB。当前可审计事实：

- 已有最终基线为 `work-epub/css-clean-system-fonts/after/final.epub`。
- 包内有 16 个分卷海报页。每页是独立 XHTML，只含一张竖版 raster 海报图。
- 每个分卷海报页后紧跟一个版权信息页，共 16 个版权页。
- 海报图的文字与出版社标识接近上下边缘。使用 `cover` 可能裁切文字；强制 `100% 100%` 会改变宽高比。
- 当前工作目录保留了中间 EPUB、分步报告、Kindle KPF 和一次性补标脚本。它们适合调试，不适合作为最终交付。

## 目标

1. 保持既有清洗结果和正文文本不变。
2. 将分卷海报页转成独立 A-lite 全页：整页显示、前后强制分页、内部不分页。
3. 让海报背景在不裁切、不变形的前提下尽可能铺满页面。
4. 优化版权页层级：去掉方块列表感，改为克制的信息页排版。
5. 继续收敛 CSS 文件：将局部 stylesheet 在作用域隔离后合并，避免文件分散和跨册串色。
6. 把可复用经验固化为公共脚本、测试和流水线文档。
7. 默认生成精简报告；最终本地交付目录只保留核心 EPUB、JSON、TXT 和 MD。

## 方案比较

### 方案 A：`contain` 全屏背景，保留图片 fallback

为分卷页生成 A-lite 外壳，背景使用 `background-size: contain`、`background-position: center center`、`background-repeat: no-repeat`。原始 `<img>` 保留为 fallback，但在 A-lite 样式生效时隐藏。

优点：

- 完整显示海报，不裁切文字与出版社标识。
- 保持图片宽高比，不拉伸。
- CSS 失效时仍能显示原始图片，不退化为空白页。

缺点：

- 屏幕比例与海报比例不一致时会有少量留白。

### 方案 B：`cover` 全屏背景

背景使用 `background-size: cover`。

优点：

- 视觉上始终铺满屏幕。

缺点：

- 会裁切海报边缘。目标图的上下区域包含有效信息，不适合采用。

### 方案 C：强制 `100% 100%`

背景宽高都强制为 `100%`。

优点：

- 不留白，也不裁切。

缺点：

- 会拉伸变形，破坏原始设计。

## 选定方案

采用方案 A。`contain` 是本书分卷海报的生产规则。`cover` 继续作为 demo 中的全幅对照，不作为本轮目标书主路径。

## 组件设计

### 1. 合集精排转换器

新增 `scripts/epub_anthology_refinement.py`，只处理可证明的合集结构：

- 只扫描 OPF manifest 中的 XHTML。
- 只将“独立 XHTML、正文仅有一张 raster 图、标题为封面、后继 spine item 为版权信息页”的页面识别为分卷海报候选。
- 只将紧跟候选海报页、标题和正文结构符合版权信息页模式的页面识别为版权候选。
- 默认 dry-run，只输出 JSON 计划。
- 只有显式传入 `--write-output` 才写出新 EPUB。
- 写出时新增 `Styles/anthology-refinement.css`，并同步 OPF manifest 与 XHTML link。
- 保持 spine 顺序、nav、NCX、锚点、图片资源字节和正文文本不变。

海报 XHTML 目标结构沿用仓库既有 A-lite 类名：

```html
<body class="fullpage poster-bg poster-bg-volume-001">
  <section class="fullframe" epub:type="chapter">
    <img class="poster-fallback" src="../Images/volume-001.jpeg" alt=""/>
  </section>
</body>
```

生成 CSS 的公共骨架：

```css
@page {
  margin: 0;
  padding: 0;
}

html,
body.fullpage {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body.fullpage {
  font-size: 16px;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;
  box-sizing: border-box;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-page-break-inside: avoid;
  break-before: page;
  break-after: page;
  break-inside: avoid;
  overflow: hidden;
}

body.poster-bg {
  background-repeat: no-repeat;
  background-position: center center;
  background-size: contain;
}

.fullframe {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
  overflow: visible;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
  break-inside: avoid;
}

.poster-fallback {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0 auto;
}

@supports (background-size: contain) {
  .poster-fallback {
    display: none;
  }
}
```

每个海报页再生成一个只负责 `background-image` 的 modifier。默认保留原始 `<img>`；确认支持 `background-size: contain` 的阅读器通过 `@supports` 隐藏原图。CSS 完全失效或不识别 `@supports` 时，原始图片仍可见，不退化为空白页。

### 2. 版权页排版

版权页保持原始文字和链接不变，只添加页面级类并覆盖列表视觉：

- `<body class="anthology-copyright-page">`
- 标题继续使用现有 `.cp`
- `ul.list` 保留原有语义，但改成无项目符号、无默认左缩进的信息列表
- 每条 `li.i` 使用更紧凑的行距与轻量分隔
- 页面宽度使用相对单位和 `max-width`，不使用固定页高

版权页只做可重排 CSS 润色，不做 A-lite，不强制整页锁定。

### 3. 低风险附加优化

本轮只纳入与目标页面直接相关的低风险改进：

- 分卷海报页不再继承普通正文的段落缩进和页边距。
- 版权列表去掉方块项目符号和过宽默认缩进。
- 海报 fallback 图片保持 `max-width: 100%; height: auto`。
- 将局部 CSS 在页面作用域隔离后合并，减少碎片化文件。

不改正文段落、章节标题、注释、图片资源、nav 文案或 spine 顺序。

### 4. CSS 二次收敛

扩展 `scripts/epub_css_cleanup.py`，新增显式参数 `--merge-scoped-local-css`。它在既有去重逻辑之后处理仍然分散的局部 stylesheet：

- 只合并被至少一个 XHTML 引用、但不是全书通用层的 stylesheet。
- 排除 `clean-shared-*`、`epub3-enhancements.css`、多数页面共用层和包含不支持规则的 stylesheet。
- 每个被合并的局部 stylesheet 分配稳定作用域类，例如 `css-local-01`。
- 给原本引用该 stylesheet 的 XHTML `<body>` 添加对应作用域类。
- 将 selector 改写为 `body.css-local-01 ...` 后合入 `Styles/clean-scoped-local.css`。
- 将页面原有局部 stylesheet link 替换为 `clean-scoped-local.css`。
- 同步删除 OPF manifest 中不再使用的局部 CSS item。
- 如果同一个页面同时引用多个待合并局部 stylesheet，跳过这些重叠 stylesheet 并报告，避免改变 cascade 顺序。

目标书当前 12 个 CSS 中，可收敛的局部文件为 7 个 `clean-override-*` 和 2 个目录样式表。二次收敛后保留：

```text
Styles/clean-shared-01.css
Styles/clean-scoped-local.css
Styles/epub3-enhancements.css
<existing-book-local-stylesheet>
Styles/anthology-refinement.css
```

目标书最终 CSS 文件数从 `12` 收敛到 `5`。其中 `anthology-refinement.css` 由合集精排转换器新增，职责是 A-lite 分卷页与版权页，不并入正文基础层。

### 5. Demo 与阅读器记录

在 `templates/epub-style-demo/` 增加 raster 海报 `contain` 对照 fixture。它与既有 `poster-bg-fullbleed` 的 `cover` fixture 并存：

- `cover`：验证允许裁切的全幅背景。
- `contain`：验证文字不能裁切的分卷海报。

同步更新：

- `templates/epub-style-demo/README.md`
- `templates/epub-style-demo/SCENE_MATRIX.md`
- `templates/epub-style-demo/OEBPS/package.opf`
- `templates/epub-style-demo/OEBPS/nav.xhtml`
- `templates/epub-style-demo/OEBPS/toc.ncx`
- `docs/final/reader-matrix.yaml`

新 fixture 在真实阅读器复测前标记为 `warn`，不虚构 `pass`。目标书至少运行 Kindle Previewer CLI；如 GUI 窗口可被 Computer Use 读取，再补视觉抽样。GUI 不可读时记录跳过理由。只有拿到 demo 和 reader-matrix 证据后，才同步修改 `docs/final/SPEC-实现约束.md`、终极手册、Markdown 速查表及其派生 HTML。

### 6. 精简流水线报告

修改 `scripts/epub_cleanup_pipeline.py`：

- 默认只持久化 `reports/pipeline.json`。
- `pipeline.json` 内嵌各步骤的结构化 JSON 结果和文本校验摘要。
- `--keep-step-reports` 显式保留旧式分步报告，供调试使用。
- `normalize dry-run` 仍单独保留 `reports/normalize-dry-run.json`，因为它需要人工审核。
- `normalize apply` 的 path map 可在运行期临时落盘；成功后默认收口进 `pipeline.json`。

最终目标书建立新的精简交付目录，只保留：

```text
delivery/
├── final.epub
├── summary.json
├── notes.md
└── reader-check.txt
```

旧工作目录作为历史调试证据保留，不原地删除；新的 `delivery/` 才是交付入口。

## 数据流

```text
既有 final.epub
  -> CSS scoped consolidation
  -> 合集转换器 dry-run JSON
  -> 人工确认候选页计数和路径
  -> 合集转换器写出 refined EPUB
  -> preflight / popup / redline / XML / ZIP 校验
  -> Kindle Previewer CLI
  -> 精简 delivery 目录
  -> 回写 pipeline 文档、demo、reader-matrix 和相关 skill
```

## 错误处理

- 未找到成对的海报页和版权页：dry-run 报告并停止写出。
- 候选海报不是 JPEG/PNG raster：跳过并报告，不猜测转换。
- 版权页含无法识别的复杂结构：跳过并报告，不重写。
- OPF manifest、spine、资源引用或 XML 解析失败：立即停止。
- 任一红线校验失败：不交付该 EPUB。
- Kindle Previewer 转换失败：保留日志摘要，停止将结果标为完成。

## 验证

实现阶段至少运行：

```sh
python3 scripts/test_epub_anthology_refinement.py
python3 scripts/test_epub_cleanup_pipeline.py
python3 -m py_compile scripts/epub_anthology_refinement.py scripts/epub_cleanup_pipeline.py
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
python3 scripts/validate_ai_entrypoints.py
python3 scripts/validate_skills_basic.py
git diff --check
```

目标书写出后至少运行：

```sh
unzip -tqq delivery/final.epub
python3 scripts/epub_preflight_harness.py delivery/final.epub --format json
bash scripts/validate-popup-notes.sh --epub delivery/final.epub
python3 scripts/validate_text_invariance.py \
  work-epub/css-clean-system-fonts/after/final.epub \
  delivery/final.epub \
  --check all
```

还需检查：

- 识别并改写 16 个分卷海报页。
- 识别并润色 16 个版权信息页。
- CSS 文件数从 12 收敛到 5。
- 图片资源字节未变化。
- spine 顺序未变化。
- `delivery/` 只含四个约定文件。
- Kindle Previewer CLI 转换成功，记录错误数和质量问题数量。

## 范围外

- 不嵌入字体。
- 不压缩、裁切或替换海报图片。
- 不修改正文、注释正文、元数据、nav 文案或 spine 顺序。
- 不把目标书专用视觉规则并入公共转换器。
- 不删除旧工作目录中的历史调试证据。
