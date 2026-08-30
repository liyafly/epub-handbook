# EPUB 包操作工具

> 状态：流程文档；用于 EPUB 合并、按目录拆分、元数据编辑和封面替换。
> 推荐入口：四个单能力 `epub run` 命令。
> 对应 skill：`$epub-package-operator`。

## 适用范围

package operations 借鉴 `epub-gadget` 中合并 / 拆分、封面和元数据编辑的实用思路，但实现保持本仓约束：

- 由 Go CLI 单一二进制提供，不需要 Python 环境。
- 不原地覆盖输入 EPUB。
- 遇到 `META-INF/encryption.xml` 默认停止，不做 DRM 解密或字体混淆处理。
- 合并和拆分会重新生成 `nav.xhtml`、`toc.ncx`、OPF manifest 和 spine。
- 封面替换会同步更新 OPF cover 声明，并重写 XHTML/CSS 中指向旧封面的本地引用。
- 所有写操作输出 JSON report，便于归档和后续验证。

## 单任务直接运行

每个写操作是独立 capability，要求显式输出：

| 能力 | 推荐入口 |
| --- | --- |
| 合并多本 EPUB | `epub run epub.package.merge --input <a.epub> --output <merged.epub> --json extra_inputs=<b.epub>` |
| 按目录索引拆分 EPUB | `epub run epub.package.split --input <book.epub> --output <dir>/<首册>.epub --json split_points=<indices> output_dir=<dir>` |
| 写入元数据 | `epub run epub.metadata.edit --input <book.epub> --output <out.epub> --json 'metadata_json={"title":"新标题"}'` |
| 替换封面 | `epub run epub.cover.replace --input <book.epub> --output <out.epub> --json cover=<image>` |

每个写操作都要求显式传入 `--output`（split 的分段产物实际写入 `output_dir`，其 `--output` 仅用于通过 CLI 写权限检查，不实际写入），不会直接覆盖原 EPUB。需要正式交付时，按对应小节的验证建议检查输出。

## 合并 EPUB

```sh
epub run epub.package.merge \
  --input volume-01.epub \
  --output merged.epub \
  --json extra_inputs=volume-02.epub \
  'title=合集标题' > merged.report.json
```

合并场景的能力自带 text 红线会把后续册的文件记为新增（error findings，退出码 1），产物仍会写出；是否符合预期以随后的显式 `epub redline` 与人工 diff review 为准。

合并时会：

- 保留第一个 EPUB 的资源路径；
- 后续 EPUB 遇到同名资源时给文件名加 `volN_` 前缀；
- 重写 XHTML、CSS、nav/NCX 中的本地引用；
- 合并 spine 顺序；
- 生成新的 `OEBPS/nav.xhtml` 和 `OEBPS/toc.ncx`。

验证建议：

```sh
unzip -tqq merged.epub
epub run epub.package.nav.audit --input merged.epub --json
epub redline --check drm,anchors volume-01.epub merged.epub
```

合并会改变 spine、metadata 和书籍结构，不适合跑完整 text invariance 对比；至少检查 ZIP、结构审计、DRM 和锚点。

## 列出拆分点

```sh
epub run epub.package.nav.audit --input book.epub --json
```

拆分点来自 EPUB3 nav；没有 nav 时回退到 NCX；仍没有目录时回退到 spine 文件列表。审计报告的 `facts.summary` 给出 manifest / spine 数量等结构事实，`nextCommands` 提示后续命令。

## 拆分 EPUB

```sh
epub run epub.package.split \
  --input book.epub \
  --output split-out/book_01.epub \
  --json split_points=0,12,30 output_dir=split-out > split.report.json
```

该能力会在输出目录非空时停止。`--output` 仅用于通过 CLI 写权限检查，分段产物实际写入 `output_dir`。

`split_points` 使用目录条目的索引。每个索引是一个新分册的开始位置，输出文件命名为 `<原文件名>_01.epub`、`<原文件名>_02.epub`。

拆分时会：

- 按 spine 范围划分正文文件；
- 收集 XHTML 和 CSS 引用到的图片、样式、字体等资源；
- 为每个分册生成独立 OPF、nav 和 NCX；
- 保留所选正文文件的原始字节，不改写正文文本。

验证建议：

```sh
for epub in split-out/*.epub; do
  unzip -tqq "$epub"
  epub run epub.package.nav.audit --input "$epub" --json
done
```

## 读取和写入元数据

读取：暂无独立读取能力，直接解包查看 OPF `<metadata>`，或用 `epub run epub.package.nav.audit --input book.epub --json` 查看包结构事实。写入：

```sh
epub run epub.metadata.edit \
  --input book.epub \
  --output book-metadata.epub \
  --json 'metadata_json={"title":"新标题","author":"作者","language":"zh-CN"}' \
  > metadata-write.report.json
```

`metadata_json` 是内联 JSON 对象文本，键值均为字符串。

支持字段：

- `title`
- `subtitle`
- `author`
- `language`
- `publisher`
- `description`
- `identifier`
- `rights`

元数据写入不改 spine 和正文。能力运行自带的内置 metadata 红线会把预期中的字段变更记为 findings（退出码 1），产物仍会写出；验证建议用显式 redline 排除 metadata：

```sh
epub redline --check text,spine,cover,drm,anchors \
  book.epub book-metadata.epub
```

## 替换封面

```sh
epub run epub.cover.replace \
  --input book.epub \
  --output book-cover.epub \
  --json cover=cover.png > cover.report.json
```

封面替换会：

- 写入 `Images/cover.<ext>`；
- 更新 manifest 中的 cover item；
- 保证 `properties="cover-image"`；
- 更新 legacy `<meta name="cover" content="..."/>`；
- 移除被替换的旧封面文件；
- 重写 XHTML/CSS 中指向旧封面的本地引用。
- 若封面页用 inline SVG 的 `viewBox` 与 `<image width/height>` 固定旧封面像素尺寸，且新封面是可识别尺寸的 PNG/JPEG，同步改为新封面的像素尺寸，避免拉伸或留边。

验证建议：

```sh
epub run epub.package.nav.audit --input book-cover.epub --json
epub redline --check text,metadata,spine,drm,anchors \
  book.epub book-cover.epub
```

`cover` 校验不适合用于封面替换，因为封面本来就是预期变更。

## 当前边界

- 不做图片压缩、WebP 转换或颜色空间检查。
- 不合并远程资源；外链图片仍需先走资源接入或外部下载流程。
- 不保留每个输入 EPUB 的原始 nav/NCX 文件；合并和拆分会生成新的目录文件。
- 不处理真实存在的未知加密资源。
