# EPUB 包操作工具

> 状态：流程文档；用于 EPUB 合并、按目录拆分、元数据编辑和封面替换。
> 对应工具：`scripts/epub_package_tool.py`。

## 适用范围

`scripts/epub_package_tool.py` 借鉴 `epub-gadget` 中合并 / 拆分、封面和元数据编辑的实用思路，但实现保持本仓约束：

- 只使用 Python 标准库。
- 不原地覆盖输入 EPUB。
- 遇到 `META-INF/encryption.xml` 默认停止，不做 DRM 解密或字体混淆处理。
- 合并和拆分会重新生成 `nav.xhtml`、`toc.ncx`、OPF manifest 和 spine。
- 封面替换会同步更新 OPF cover 声明，并重写 XHTML/CSS 中指向旧封面的本地引用。
- 所有写操作输出 JSON report，便于归档和后续验证。

## 合并 EPUB

```sh
python3 scripts/epub_package_tool.py merge \
  volume-01.epub volume-02.epub \
  --output merged.epub \
  --title "合集标题" > merged.report.json
```

合并时会：

- 保留第一个 EPUB 的资源路径；
- 后续 EPUB 遇到同名资源时给文件名加 `volN_` 前缀；
- 重写 XHTML、CSS、nav/NCX 中的本地引用；
- 合并 spine 顺序；
- 生成新的 `OEBPS/nav.xhtml` 和 `OEBPS/toc.ncx`。

验证建议：

```sh
unzip -tqq merged.epub
python3 scripts/epub_preflight_harness.py merged.epub --format json
python3 scripts/validate_text_invariance.py volume-01.epub merged.epub --check drm,anchors
```

合并会改变 spine、metadata 和书籍结构，不适合跑完整 text invariance 对比；至少检查 ZIP、preflight、DRM 和锚点。

## 列出拆分点

```sh
python3 scripts/epub_package_tool.py split-targets book.epub > split-targets.json
```

拆分点来自 EPUB3 nav；没有 nav 时回退到 NCX；仍没有目录时回退到 spine 文件列表。返回项包含 `title`、`href` 和 `level`。

## 拆分 EPUB

```sh
python3 scripts/epub_package_tool.py split \
  book.epub \
  --output-dir split-out \
  --split-points 0,12,30 > split.report.json
```

`--split-points` 使用 `split-targets` 返回数组的索引。每个索引是一个新分册的开始位置，输出文件命名为 `<原文件名>_01.epub`、`<原文件名>_02.epub`。

拆分时会：

- 按 spine 范围划分正文文件；
- 收集 XHTML 和 CSS 引用到的图片、样式、字体等资源；
- 为每个分册生成独立 OPF、nav 和 NCX；
- 保留所选正文文件的原始字节，不改写正文文本。

验证建议：

```sh
for epub in split-out/*.epub; do
  unzip -tqq "$epub"
  python3 scripts/epub_preflight_harness.py "$epub" --format json
done
```

## 读取和写入元数据

读取：

```sh
python3 scripts/epub_package_tool.py metadata-read book.epub > metadata.json
```

写入：

```sh
python3 scripts/epub_package_tool.py metadata-write \
  book.epub \
  --output book-metadata.epub \
  --metadata-json '{"title":"新标题","author":"作者","language":"zh-CN"}' \
  > metadata-write.report.json
```

支持字段：

- `title`
- `subtitle`
- `author`
- `language`
- `publisher`
- `description`
- `identifier`
- `rights`

元数据写入不改 spine 和正文。验证建议：

```sh
python3 scripts/validate_text_invariance.py \
  book.epub book-metadata.epub \
  --check text,spine,cover,drm,anchors
```

## 替换封面

```sh
python3 scripts/epub_package_tool.py replace-cover \
  book.epub \
  --output book-cover.epub \
  --cover cover.png > cover.report.json
```

封面替换会：

- 写入 `Images/cover.<ext>`；
- 更新 manifest 中的 cover item；
- 保证 `properties="cover-image"`；
- 更新 legacy `<meta name="cover" content="..."/>`；
- 移除被替换的旧封面文件；
- 重写 XHTML/CSS 中指向旧封面的本地引用。

验证建议：

```sh
python3 scripts/epub_preflight_harness.py book-cover.epub --format json
python3 scripts/validate_text_invariance.py \
  book.epub book-cover.epub \
  --check text,metadata,spine,drm,anchors
```

`cover` 校验不适合用于封面替换，因为封面本来就是预期变更。

## 当前边界

- 不做图片压缩、WebP 转换或颜色空间检查。
- 不合并远程资源；外链图片仍需先走资源接入或外部下载流程。
- 不保留每个输入 EPUB 的原始 nav/NCX 文件；合并和拆分会生成新的目录文件。
- 不处理真实存在的未知加密资源。
