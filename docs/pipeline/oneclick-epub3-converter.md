# 一命令 EPUB 清洗与 EPUB3 转换

> 状态：流程文档；用于把一本旧 EPUB/EPUB2 在本地转换为 EPUB3，生成可审计工作目录，并套用项目的弹注与 CJK 文学排版基础层。
> 流水线入口：`scripts/epub_cleanup_pipeline.py`
> 底层变换器：`scripts/epub3_oneclick_converter.py`

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
- 字体内嵌。脚本只写多字体使用规则，不打包字体。

## 一条命令

用一个新的脱敏工作目录承接真实文件，不把真实文件名写进提交记录：

```sh
python3 scripts/epub_cleanup_pipeline.py \
  /path/to/input.epub \
  --work-dir work/book-a
```

入口会自动完成：

1. 复制输入为 `work/book-a/before/source.epub`，保留不可修改基线。
2. 跑前置 preflight；有阻断错误立即停止。
3. 调用底层 EPUB3 转换器。
4. 跑产物 preflight、弹注 validator、红线子集 gate 和独立正文文本 gate。
5. 生成精排建议和 AI findings。
6. 写入 `work/book-a/reports/pipeline.json`。

默认只落盘这一份汇总 JSON。需要逐项排障归档时加 `--keep-step-reports`，再额外保留 preflight、conversion、popup、redline、refinement 和 findings 文件。结构规范化报告不受此开关影响，始终单独保留。

输出 EPUB 位于 `work/book-a/after/cleaned.epub`，包含：

- EPUB3 `package version="3.0"`。
- `dcterms:modified`。
- `ibooks:specified-fonts`（仅当检测到 `body-font-locked` 页时添加；若输入已存在但未检测到锁定页则保留并提示人工复核，见 `docs/final/SPEC-实现约束.md` §8）。
- 新建 `nav.xhtml`，保留 `toc.ncx` 和 `spine toc="ncx"`。
- 修正 `mimetype` 为 zip 第一项且 stored。
- 修正 `guide` 中可自动识别的坏相对路径。
- XHTML 根缺 `lang` 和 `xml:lang` 时，从 OPF `dc:language` 补入两者；不覆盖已有值，也不猜测缺失的 OPF 语言。
- 发生 XHTML 重写时，对 XML-valid 页面使用两空格缩进的多行输出；不压缩为单行，也不改写 mixed-content 中的正文文字。无法 XML 解析的遗留页面保留原格式并继续由其他校验报告问题。
- 新增 `Styles/epub3-enhancements.css`。
- 仅在纯文本/数字上标注释标记需要图标化时新增 `Images/note.png`；已有图片 noteref 保留原图标。
- 图片 noteref 的 `sup` 使用 `class="note-marker"`；其零行高外壳与相对上移图标只作用于脚注，避免 `sup img` 撑高正文行距。
- 普通尾注转为同文件 grouped popup footnote。

流水线不会替代人工 diff review 和真实阅读器复测。`reports/pipeline.json` 会把它们列为剩余步骤。

## 可选结构规范化

内部目录混乱、文件名明显混淆或需要稳定 diff 时，先用同一个入口生成 dry-run 报告：

```sh
python3 scripts/epub_cleanup_pipeline.py \
  /path/to/input.epub \
  --work-dir work/book-a-normalize-review \
  --normalize dry-run
```

检查 `work/book-a-normalize-review/reports/normalize-dry-run.json` 的两个阶段后，在新的工作目录显式批准：

```sh
python3 scripts/epub_cleanup_pipeline.py \
  /path/to/input.epub \
  --work-dir work/book-a-normalized \
  --normalize apply \
  --approve-normalize
```

`apply` 不接受隐式确认。每次运行使用新的工作目录，避免覆盖 before 基线和旧报告。

## 字体策略

脚本注入的覆盖层遵循系统优先，不嵌入字体：

- 正文：`"Songti SC", "SimSun", "Noto Serif CJK SC", serif`
- 标题：`"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`
- 楷体类、引用、注释：`"Kaiti SC", "STKaiti", "KaiTi", serif`

覆盖层还提供 `type-body`、`type-title`、`type-subtitle`、`type-quote`、`type-note`、`type-emphasis` 和 `type-meta` 角色类。后续如需内嵌字体，只替换或补充显式类，不把子集字体挂到 `body`。

角色拆分和本地文学 EPUB 的脱敏分析见 [reference-font-role-patterns.md](reference-font-role-patterns.md)。

## 可选 CSS 去重与局部样式合并

合订 EPUB 如果重复携带每册 CSS，可在 EPUB3 基线通过 preflight 后运行：

```sh
python3 scripts/epub_css_cleanup.py \
  work/book-a/intermediate/step-1-epub3.epub \
  --output work/book-a/after/final.epub \
  --merge-scoped-local-css \
  --format json > work/book-a/reports/css-cleanup.json
```

清洗器会：

- 合并完全重复的 CSS；
- 把结构相同但少量属性不同的样式拆成公共层与小型 override；
- 将旧式 `cnepub`、`SimSun`、`SimHei`、`STKaiti` 声明替换为短系统字体链；
- 同步更新 XHTML `<link>` 和 OPF CSS manifest；
- 可选把引用页面集合互不重叠的局部样式归并为 `clean-scoped-local.css`，规则改写为 `body.css-local-*` 作用域；引用集合有交叠时跳过并报告。

这是公共脚本的保守边界。完整决策和复用步骤见 [css-cleanup-system-fonts.md](css-cleanup-system-fonts.md)。

清洗前后必须运行完整红线 gate：

```sh
python3 scripts/validate_text_invariance.py \
  work/book-a/intermediate/step-1-epub3.epub \
  work/book-a/after/final.epub \
  --check all
```

## 可选合集卷封与版权页精排

既有合订 EPUB 如果每卷以“单图封面 + 紧邻版权信息页”开头，可在 CSS 清洗后单独运行：

```sh
python3 scripts/epub_anthology_refinement.py \
  work/book-a/after/final.epub \
  --output work/book-a/after/final-anthology.epub \
  --expect-volumes <N> \
  --format json > work/book-a/reports/anthology-refinement.json
```

脚本把单图卷封转换为 A-lite `contain` 背景并保留 `<img class="poster-fallback">`，避免裁图或空白页；版权信息页只增加紧凑排版容器和 class，不改书名、作者、ISBN 或链接文字。`--expect-volumes` 用来阻止漏识别时继续交付。

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
python3 scripts/validate_text_invariance.py \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub \
  --check all
```

## 验证

```sh
unzip -tqq work/book-a/after/cleaned.epub
python3 scripts/epub_preflight_harness.py work/book-a/after/cleaned.epub --format json
bash scripts/validate-popup-notes.sh --epub work/book-a/after/cleaned.epub
python3 scripts/validate_text_invariance.py \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub \
  --check metadata,drm,anchors \
  --allow-list '*/nav.xhtml'

python3 scripts/validate_text_invariance.py \
  work/book-a/before/source.epub \
  work/book-a/after/cleaned.epub \
  --check text \
  --allow-list '*/nav.xhtml' \
  --allow-list '*/toc.ncx'
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

## 只调用底层变换器

只有在上层已经完成 before 备份、preflight 和审计记录时，才直接调用底层脚本：

```sh
python3 scripts/epub3_oneclick_converter.py \
  work/before/source.epub \
  --output work/after/cleaned.epub \
  --format json > work/after/cleaned.report.json
```
