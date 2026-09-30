# EPUB3 迁移：产物与验收

> 状态：流程文档；用于把旧 EPUB/EPUB2 转为 EPUB3。排版样式由独立 typography capability 选择。
> 执行入口：`epub run epub.package.migrate.epub3`；目录规范化见 cleanup-flow S2。
> 对应 skill：`$epub-cleanup`

执行顺序以 [cleanup-flow.md](cleanup-flow.md) 主线为准：`$W` 是书级工作区的 `03 制作工作区/.pipeline/`，`$CUR` 是最近通过红线的候选。

## 适用范围

适合 EPUB2、缺少 `nav.xhtml` 的旧包，或需修复可识别的 NCX 路径和旧字体名的输入。不处理 OCR 校对、正文改写、图片压缩/转码或字体内嵌。plain/Sigil 旧尾注保持原样；如需转换，必须另行取得正文修改授权。

## 产物

只做迁移时，先生成 dry-run 计划，再显式写出新文件；不得覆盖输入：

```sh
epub run epub.package.migrate.epub3 \
  --input input.epub --output migrated.epub --dry-run --json > migration-plan.json

epub run epub.package.migrate.epub3 \
  --input input.epub --output migrated.epub --json > migration-apply.json
```

cleanup-flow S3 的候选 `$W/after/s3.epub` 包含：

- EPUB3 `package version="3.0"`、`dcterms:modified` 和新建的 `nav.xhtml`；保留 `toc.ncx` 及 `spine toc="ncx"`。
- 根据可识别的输入补充 `properties="svg"` / `properties="mathml"`，修复 manifest、guide、mimetype 和 XHTML 语言声明。
- 输入已有的 `ibooks:specified-fonts` 不会被自动删除；锁定状态需人工复核并把例外记入书级制作说明。
- 以字节范围改写目标结构；保留排版 CSS、XHTML 标签、普通正文和未命中的格式、注释及 mixed-content。
- 保留旧尾注标记、正文字符、注释段落、ID 和资源；只规范化识别出的 Duokan legacy class 与缺失的 footnote `role`，不改写返回字形。

迁移不添加排版样式、不分派文本角色。遇到 `<big>` 等旧标签时保留原标签并提示人工复核。

## CSS 清洗

在 EPUB3 基线上，CSS 能力只做保守修补（分号、装饰行和已知旧字体链）；去重、分层与 scoped merge 已停用。它不嵌入字体，也不改写正文。对重复携带旧样式表、含旧平台字体名的合订 EPUB，可按主线 S5 单项运行：

```sh
epub run epub.css.layering.optimize \
  --input "$CUR" \
  --output "$W/after/s5-<n>.epub" \
  --json > "$W/s5-<n>.json"
```

每次写出后运行 ZIP 检查、nav audit、弹注检查和全项 redline：

```sh
unzip -tqq "$W/after/s5-<n>.epub"
epub run epub.package.nav.audit --input "$W/after/s5-<n>.epub" --json
epub run epub.notes.popup.normalize --input "$W/after/s5-<n>.epub" --json
epub redline --check all [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s5-<n>.epub"
```

人工检查 CSS 引用、OPF manifest 与 ZIP 内容，并在 Calibre Editor 或 VS Code review diff；是否嵌入字体属于独立阶段，需先核对授权。

## 弹注结构

`epub.package.migrate.epub3` 不会把普通 `wN → mN` 尾注或 Sigil 的 `noteref_N → footnote_N` 改写为标准弹注；正文标记、注释段落、ID 和资源保持不变。若需转换旧尾注，必须先取得明确授权并人工编辑，不依赖 redline 为标记或注释变化豁免。

写出后用只读弹注能力检查候选，再按 S6 运行全项 redline。完整步骤见 [cleanup-flow.md](cleanup-flow.md#主线)。

## 验证

迁移候选至少通过 ZIP 检查、结构审计、弹注检查和全项红线：

```sh
unzip -tqq "$W/after/s3.epub"
epub run epub.package.nav.audit --input "$W/after/s3.epub" --json
epub run epub.notes.popup.normalize --input "$W/after/s3.epub" --json
epub redline --check all [--path-map "$W/s2-normalize.json"] \
  "$W/before/source.epub" "$W/after/s3.epub"
```

正文文本 gate 是硬门禁。若转换触发它，能力返回 exit 1，但候选仍会写出；保留候选供 diff review，不更新 `CUR`，也不把它当作可交付结果。人工 diff 与真实阅读器复测按 cleanup-flow S8–S9 执行，并记录 artifact SHA、阅读器版本和现象。

## 脱敏记录

提交到仓库的文档只记录输入 SHA-256、转换计数、输出角色、工具版本和错误/质量问题数量。不要提交原 EPUB、转换后 EPUB/KPF、真实书名/作者/ISBN/ASIN、水印或私有 metadata，也不要提交未脱敏的 Kindle Previewer 临时路径日志。
