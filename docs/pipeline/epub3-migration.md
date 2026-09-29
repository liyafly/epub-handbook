# EPUB3 迁移：产物与验收

> 状态：流程文档；用于把一本旧 EPUB/EPUB2 在本地转换为 EPUB3，并生成可审计工作目录。排版样式需通过独立 typography capability 明确选择。
> 执行入口：`epub run epub.package.migrate.epub3`（结构规范化：`epub run epub.structure.normalize`）
> 对应 skill：`$epub-cleanup`

新书级项目先按 [一书一 Git 工作区](book-workspace.md) 建立目录。本页的 `$W` 表示该书 `03 制作工作区/.pipeline/`，`$CUR` 表示当前已通过红线的候选；执行顺序以 cleanup-flow 主线为准。

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

完整步骤见 [cleanup-flow.md](cleanup-flow.md) S3。

cleanup-flow 主线中的 S3 产物（`$W/after/s3.epub`）包含：

- EPUB3 `package version="3.0"`。
- `dcterms:modified`。
- `ibooks:specified-fonts`（检测到直接 `body` 字体规则或既有 `body-font-locked` 页时添加；若输入已存在但未检测到锁定则保留，迁移报告 `metadataUpdates` 会记为 `kept existing` 并提示人工复核。CLI 没有独立 lint；豁免理由写入书级 `制作说明.md`，见 `docs/final/SPEC-实现约束.md` §8）。
- 新建 `nav.xhtml`，保留 `toc.ncx` 和 `spine toc="ncx"`。
- 修正 `mimetype` 为 zip 第一项且 stored。
- 修正 `guide` 中可自动识别的坏相对路径。
- XHTML 根缺 `lang` 和 `xml:lang` 时，从 OPF `dc:language` 补入两者；不覆盖已有值，也不猜测缺失的 OPF 语言。
- XHTML 按目标字节区间更新；未命中的格式、注释和 mixed-content 正文文字保持原样。
- 仅在纯文本/数字上标注释标记需要图标化时新增 `Images/note.png`；已有图片 noteref 保留原图标。
- 图片 noteref 的 `sup` 使用 `class="note-marker"`；其零行高外壳与相对上移图标只作用于脚注，避免 `sup img` 撑高正文行距。
- 普通尾注转为同文件 grouped popup footnote。

迁移保留已有的排版 CSS 和 XHTML 标签，不添加排版样式、不分派文本角色。遇到旧式 `<big>` 标签时会保留原标签，并在报告中给出人工复核提示。

流水线不会替代人工 diff review 和真实阅读器复测。审计报告的 `nextCommands` 会把它们列为剩余步骤。

## 可选结构规范化

结构规范化属于 cleanup-flow 主线 S2；执行与映射审查见 [cleanup-flow.md 主线 S2](cleanup-flow.md#主线)。

## 字体策略

EPUB3 迁移不注入 CSS，也不更改字体链。需要显式排版时，使用 `epub.typography.optimize` 并选择预设与作用范围：

```sh
epub run epub.typography.optimize --input "migrated.epub" --output "typography-candidate.epub" --dry-run --json preset=literary-cn
```

审查 dry-run 后，去掉 `--dry-run` 并保持其余选项相同，再运行红线和阅读器检查。预设的角色映射与字体链见 [reference-font-role-patterns.md](reference-font-role-patterns.md)。

若计划内嵌字体，先核对授权；局部补字子集不要挂到 `body`。

## CSS 清洗

CSS 清洗只做保守修补（分号、装饰行、已知旧字体链）；去重、分层、scoped merge 已停用。

能力边界和人工处理建议见 [css-cleanup-system-fonts.md](css-cleanup-system-fonts.md)。

清洗前后必须运行完整红线 gate：

```sh
epub redline --check all \
  [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s3.epub"
```

## 可选合集卷封与版权页精排

既有合订 EPUB 如果每卷以“单图封面 + 紧邻版权信息页”开头，可在 CSS 清洗后单独运行：

```sh
epub run epub.alite.convert \
  --input "$CUR" \
  --output "$W/after/s5-<n>.epub" \
  --json expect_volumes=<N> > "$W/s5-<n>.json"
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
  [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s3.epub"
```

## 验证

```sh
unzip -tqq "$W/after/s3.epub"
epub run epub.package.nav.audit --input "$W/after/s3.epub" --json
epub run epub.notes.popup.normalize --input "$W/after/s3.epub" --json
epub redline --check all [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s3.epub"
```

正文文本 gate 是硬门禁。若转换触发它，流水线立即停止，不把该产物当作可交付结果。

Kindle Previewer 可选：

```sh
mkdir -p "$W/after/kindle-preview-output"
'/Applications/Kindle Previewer 3.app/Contents/MacOS/Kindle Previewer 3' \
  "$W/after/s3.epub" \
  -convert -qualitychecks \
  -output "$W/after/kindle-preview-output" \
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
- 输出文件角色，例如 `$W/after/s3.epub`。
- 工具版本和错误/质量问题数量。

不要提交：

- 原 EPUB。
- 转换后 EPUB/KPF。
- 真实书名、作者、ISBN、ASIN、水印、私有 metadata。
- Kindle Previewer 生成的完整临时路径日志，除非已替换成本地占位路径。

## 底层变换器入口

迁移能力的执行、前置条件与输出位置见 [cleanup-flow.md 主线 S3](cleanup-flow.md#主线)。
