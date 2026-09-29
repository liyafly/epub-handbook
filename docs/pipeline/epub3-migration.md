# EPUB3 迁移：产物与验收

> 状态：流程文档；用于把一本旧 EPUB/EPUB2 在本地转换为 EPUB3，并生成可审计工作目录。排版样式需通过独立 typography capability 明确选择。
> 执行入口：`epub run epub.package.migrate.epub3`（结构规范化：`epub run epub.structure.normalize`）
> 对应 skill：`$epub-cleanup`

新书级项目先按 [一书一 Git 工作区](book-workspace.md) 建立目录。本页的 `$W` 表示该书 `03 制作工作区/.pipeline/`，`$CUR` 表示当前已通过红线的候选；执行顺序以 cleanup-flow 主线为准。

## 适用范围

适合这类输入：

- EPUB2 或缺少 `nav.xhtml` 的旧包。
- `toc.ncx` 来自 Kindle/MOBI 反解，存在 `src="Text/file.xhtml"#id` 这类坏片段引号。
- 含 plain 或 Sigil 旧尾注、但另有 EPUB3 package/nav/shell 迁移需求的输入；旧尾注会保持原样，转换需另行授权并人工处理。
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
- plain/Sigil 旧尾注和 `[N]` 标记保持原样；不会改写正文标记、合并旧注释或注入默认图标。
- Duokan legacy class 与缺失的 footnote `role` 仍会按真实标签规范化。

迁移保留已有的排版 CSS 和 XHTML 标签，不添加排版样式、不分派文本角色。遇到旧式 `<big>` 标签时会保留原标签，并在报告中给出人工复核提示。

人工 diff review 与阅读器复测见 [cleanup-flow S6–S9](cleanup-flow.md#主线)。

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

`epub.package.migrate.epub3` 不会把普通 `wN → mN` 尾注或 Sigil 的 `noteref_N → footnote_N` 改写成标准弹注；标记、注释段落、ID 和资源保持不变。只有 Duokan legacy class/role 规范化仍属于迁移能力。

如需把旧尾注转换成弹注，先获得明确的正文修改授权，再按正文校订流程人工编辑，或使用经用户授权的外部工具。不要依赖 redline 为旧 `[N]` 标记与新 noteref 配对豁免；redline 会按普通正文块比较，标记或注释文字的删除/变化会被报告。

完成后运行只读的 `epub.notes.popup.normalize` 检查已有结构，再运行全项 redline：

```sh
epub run epub.notes.popup.normalize --input "$W/after/s3.epub" --json
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
