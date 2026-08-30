# EPUB 清洗与 EPUB3 转换

> 状态：流程文档；用于把一本旧 EPUB/EPUB2 在本地转换为 EPUB3，生成可审计工作目录，并套用项目的弹注与 CJK 文学排版基础层。
> 执行入口：`epub run epub.package.migrate.epub3`（结构规范化：`epub run epub.structure.normalize`）
> 对应 skill：`$epub3-migrator`

新书级项目先按 [一书一 Git 工作区](book-workspace.md) 建立目录。本页为保留流水线内部结构的可读性，仍以 `work/book-a/` 表示流水线工作目录；新项目实际应指向 `work-epub/<book>/03 制作工作区/.pipeline/`。

## 适用范围

适合这类输入：

- EPUB2 或缺少 `nav.xhtml` 的旧包。
- `toc.ncx` 来自 Kindle/MOBI 反解，存在 `src="Text/file.xhtml"#id` 这类坏片段引号。
- 注释是同文件 `wN -> mN` 普通尾注，或 Sigil 的 `noteref_N -> footnote_N` 单条 `aside` 尾注。
- CSS 里正文使用不存在的 `cnepub` 或过旧的宋/黑/楷字体名。

不处理：

- OCR 校对。
- 正文改写。
- 图片压缩或转码。
- 字体内嵌。迁移只写多字体使用规则，不打包字体。

## 只做 EPUB3 迁移

不需要完整清洗工作目录时，先用 dry-run 生成只读计划，再显式写出新文件：

```sh
epub run epub.package.migrate.epub3 \
  --input input.epub \
  --output migrated.epub \
  --dry-run --json > migration-plan.json

epub run epub.package.migrate.epub3 \
  --input input.epub \
  --output migrated.epub \
  --json > migration-apply.json
```

正式执行不原地覆盖输入 EPUB（`--output` 不得与 `--input` 相同），并在报告中保留 before/after SHA-256 与底层转换明细。

## 按序执行

用一个新的脱敏工作目录承接真实文件，不把真实文件名写进提交记录。Go CLI 没有一键流水线命令，按序执行：

```sh
mkdir -p work/book-a/before work/book-a/after work/book-a/reports
cp /path/to/input.epub work/book-a/before/source.epub

epub run epub.package.nav.audit \
  --input work/book-a/before/source.epub \
  --json > work/book-a/reports/preflight.json

epub run epub.package.migrate.epub3 \
  --input work/book-a/before/source.epub \
  --output work/book-a/after/cleaned.epub \
  --json > work/book-a/reports/migrate.json

epub run epub.notes.popup.normalize \
  --input work/book-a/after/cleaned.epub \
  --json > work/book-a/reports/popup.json

epub redline --check metadata,drm,anchors \
  --allow-list '*/nav.xhtml' \
  work/book-a/before/source.epub work/book-a/after/cleaned.epub

epub run epub.layout.audit \
  --input work/book-a/after/cleaned.epub \
  --json > work/book-a/reports/findings.json
```

按序完成：

1. 复制输入为 `work/book-a/before/source.epub`，保留不可修改基线。
2. 跑前置结构审计；有 error 级 finding 立即停止。
3. 调用 EPUB3 迁移能力（含底层转换器全部阶段）。
4. 跑弹注结构校验、红线子集 gate 和正文文本 gate。
5. 生成精排建议和审计 findings。
6. 各报告以 `--json` 统一信封写入 `reports/`，逐项归档。

输出 EPUB 位于 `work/book-a/after/cleaned.epub`，包含：

- EPUB3 `package version="3.0"`。
- `dcterms:modified`。
- `ibooks:specified-fonts`（检测到直接 `body` 字体规则或既有 `body-font-locked` 页时添加；若输入已存在但未检测到锁定则保留并提示人工复核，默认 lint 会报 L-F05，只有书级历史例外才显式豁免，见 `docs/final/SPEC-实现约束.md` §8）。
- 新建 `nav.xhtml`，保留 `toc.ncx` 和 `spine toc="ncx"`。
- 修正 `mimetype` 为 zip 第一项且 stored。
- 修正 `guide` 中可自动识别的坏相对路径。
- XHTML 根缺 `lang` 和 `xml:lang` 时，从 OPF `dc:language` 补入两者；不覆盖已有值，也不猜测缺失的 OPF 语言。
- 发生 XHTML 重写时，对 XML-valid 页面使用两空格缩进的多行输出；不压缩为单行，也不改写 mixed-content 中的正文文字。无法 XML 解析的遗留页面保留原格式并继续由其他校验报告问题。
- 新增 `Styles/epub3-enhancements.css`；该 CSS 使用 `[epub|type]` 选择器，因此在首个普通规则前声明标准 `@namespace epub`（若未来加入 `@charset` / `@import`，namespace 紧随其后并保持早于 `@font-face`）。
- 仅在纯文本/数字上标注释标记需要图标化时新增 `Images/note.png`；已有图片 noteref 保留原图标。
- 图片 noteref 的 `sup` 使用 `class="note-marker"`；其零行高外壳与相对上移图标只作用于脚注，避免 `sup img` 撑高正文行距。
- 普通尾注转为同文件 grouped popup footnote。

流水线不会替代人工 diff review 和真实阅读器复测。审计报告的 `nextCommands` 会把它们列为剩余步骤。

## 可选结构规范化

内部目录混乱、文件名明显混淆或需要稳定 diff 时，先生成 dry-run 报告：

```sh
epub run epub.structure.normalize \
  --input /path/to/input.epub \
  --output work/book-a-normalize-review/after/step-0-normalized.epub \
  --dry-run --json > work/book-a-normalize-review/reports/normalize-dry-run.json
```

检查报告的两个阶段后，在新的输出路径显式实跑：

```sh
epub run epub.structure.normalize \
  --input /path/to/input.epub \
  --output work/book-a-normalized/after/step-0-normalized.epub \
  --json > work/book-a-normalized/reports/normalize.json
```

实跑不接受隐式确认：必须先 review `--dry-run` 报告。每次运行使用新的输出路径，避免覆盖 before 基线和旧报告。

## 字体策略

脚本注入的覆盖层不嵌入字体，普通正文默认保持自由模式：`body` 只接收行高、对齐等排版属性，不写 `font-family`。显式角色使用以下系统链：

- `.type-body`：`"Songti SC", "SimSun", "Noto Serif CJK SC", serif`
- 标题：`"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`
- 楷体类、引用、注释：`"Kaiti SC", "STKaiti", "KaiTi", serif`

覆盖层还提供 `type-body`、`type-title`、`type-subtitle`、`type-quote`、`type-note`、`type-emphasis` 和 `type-meta` 角色类。后续如需内嵌字体，只替换或补充显式类；只含少数字符的局部补字子集不挂到 `body`。

角色拆分和本地文学 EPUB 的脱敏分析见 [reference-font-role-patterns.md](reference-font-role-patterns.md)。

## 可选 CSS 去重与局部样式合并

合订 EPUB 如果重复携带每册 CSS，可在 EPUB3 基线通过结构审计后运行：

```sh
epub run epub.css.layering.optimize \
  --input work/book-a/intermediate/step-1-epub3.epub \
  --output work/book-a/after/final.epub \
  --json merge_scoped_local_css=true > work/book-a/reports/css-cleanup.json
```

清洗器会：

- 合并完全重复的 CSS；
- 把结构相同但少量属性不同的样式拆成公共层与小型 override；
- 将旧式 `cnepub`、`SimSun`、`SimHei`、`STKaiti` 声明替换为短系统字体链；
- 同步更新 XHTML `<link>` 和 OPF CSS manifest；
- 可选把引用页面集合互不重叠的局部样式归并为 `clean-scoped-local.css`，规则改写为 `body.css-local-*` 作用域；引用集合有交叠时跳过并报告。

这是该能力的保守边界。完整决策和复用步骤见 [css-cleanup-system-fonts.md](css-cleanup-system-fonts.md)。

清洗前后必须运行完整红线 gate：

```sh
epub redline --check all \
  work/book-a/intermediate/step-1-epub3.epub \
  work/book-a/after/final.epub
```

## 可选合集卷封与版权页精排

既有合订 EPUB 如果每卷以“单图封面 + 紧邻版权信息页”开头，可在 CSS 清洗后单独运行：

```sh
epub run epub.alite.convert \
  --input work/book-a/after/final.epub \
  --output work/book-a/after/final-anthology.epub \
  --json expect_volumes=<N> > work/book-a/reports/anthology-refinement.json
```

该能力把单图卷封转换为 A-lite `contain` 背景并保留 `<img class="poster-fallback">`，避免裁图或空白页；版权信息页只增加紧凑排版容器和 class，不改书名、作者、ISBN 或链接文字。`expect_volumes` 用来阻止漏识别时继续交付。

完成验证后，面向交付方新建精简目录，不复制中间包和转换器日志：

```text
delivery/
├── final.epub
├── summary.json
├── notes.md
└── reader-check.txt
```

## 弹注结构

普通尾注：

```html
<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a>
...
<p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文。</p>
```

会转为：

```html
<sup class="note-marker">
  <a id="w1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#m1">
    <img alt="注" src="../Images/note.png"/>
  </a>
</sup>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>
  <ol class="footnote-list">
    <li class="footnote-item" id="m1">
      <p class="footnote">
        <a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#w1">◎</a>注释正文。
      </p>
    </li>
  </ol>
</aside>
```

如果原 noteref 已经是图片触发器，转换器只整理 note body 为同文件 grouped `aside/ol/li`，保留原 `img src` 和 OPF 资源；不会无差别替换为默认 `Images/note.png`。默认图标只用于纯文本或数字上标标记。

图标基线规则只使用 `sup.note-marker`、其直接 noteref 和 `img` 子元素；普通文字上标不应用零行高或相对位移。

Sigil 的旧式 `section[epub:type="footnotes"]` 若包含多条 `aside#footnote_N`，且正文引用为 `a#noteref_N`，转换器会保留原 ID、合并为一个 grouped `aside/ol/li`，并逐条保留注释正文。若 section 内出现无法完整识别的内容，转换器不做部分合并，交由人工 review。

完整文本 gate 不把 noteref 的数字、图标或 backlink 的 `◎` 当作正文，但仍逐字比较所有注释正文：

```sh
epub redline --check all \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub
```

## 验证

```sh
unzip -tqq work/book-a/after/cleaned.epub
epub run epub.package.nav.audit --input work/book-a/after/cleaned.epub --json
epub run epub.notes.popup.normalize --input work/book-a/after/cleaned.epub --json
epub redline --check metadata,drm,anchors \
  --allow-list '*/nav.xhtml' \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub

epub redline --check text \
  --allow-list '*/nav.xhtml' --allow-list '*/toc.ncx' \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub
```

正文文本 gate 是硬门禁。若转换触发它，流水线立即停止，不把该产物当作可交付结果。

Kindle Previewer 可选：

```sh
mkdir -p work/book-a/after/kindle-preview-output
'/Applications/Kindle Previewer 3.app/Contents/MacOS/Kindle Previewer 3' \
  work/book-a/after/cleaned.epub \
  -convert -qualitychecks \
  -output work/book-a/after/kindle-preview-output \
  -locale zh
```

通过标准：

- Summary log：增强排版状态为支持。
- 转码状态为成功。
- 错误数为 `0`。
- 质量问题数量为 `0`。

## 脱敏记录

提交到仓库的文档只记录：

- 输入 SHA-256。
- 转换计数，例如 nav 条目数、弹注数量、CSS 链接数量。
- 输出文件角色，例如 `work/after/cleaned.epub`。
- 工具版本和错误/质量问题数量。

不要提交：

- 原 EPUB。
- 转换后 EPUB/KPF。
- 真实书名、作者、ISBN、ASIN、水印、私有 metadata。
- Kindle Previewer 生成的完整临时路径日志，除非已替换成本地占位路径。

## 底层变换器入口

旧 `epub3_oneclick_converter.py` 兼容入口已收口：`epub run epub.package.migrate.epub3` 是唯一执行入口。需要直接触发底层转换时（上层仍须先完成 before 备份、结构审计和审计记录），运行：

```sh
epub run epub.package.migrate.epub3 \
  --input work/before/source.epub \
  --output work/after/cleaned.epub \
  --json > work/after/cleaned.report.json
```

实现按 package、navigation、XHTML、notes 与转换编排拆为该能力的内部阶段。
