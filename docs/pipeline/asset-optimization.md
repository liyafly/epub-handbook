# 资源优化：图片与字体

> 状态：操作指南；用现有工具，不写新脚本。
> 对应清洗步骤：[cleanup-flow.md](cleanup-flow.md) 的精排建议与分派清洗阶段。
> 对应 SPEC：[§10.2 黄线](../final/SPEC-实现约束.md) + [§10.1 红线](../final/SPEC-实现约束.md)。

## 1. 适用范围

做什么：

- 把 WebP / AVIF 等非 EPUB core media type 转出到 PNG / JPEG。
- 对 PNG 做无损压缩。
- 对 JPEG 做无损优化。
- 字体子集化。

不做什么：

- 不动 `properties="cover-image"`。
- 不裁剪、翻转、加水印、改色调。
- 不引入新打包脚本。
- 不强制 AVIF。

## 2. 工具栈

| 用途 | 工具 | 安装 |
| --- | --- | --- |
| PNG 无损压缩 | `oxipng` | `brew install oxipng` |
| JPEG 无损优化 | `jpegtran` / mozjpeg | `brew install mozjpeg` |
| WebP 编解码 | `dwebp` / `cwebp` | `brew install webp` |
| 通用转换 | ImageMagick `magick` | `brew install imagemagick` |
| 字体子集化与全量校验 | `epub-font` | `uv tool install --editable tools-font/epub-font` |

## 3. 图片处理

EPUB 3 core media types 包含 JPEG / PNG / GIF / SVG。WebP / AVIF 不是 Kindle 主路径，清洗时转成 PNG / JPEG。

### 3.1 WebP -> PNG / JPEG

```sh
dwebp input.webp -o /tmp/input.png
magick /tmp/input.png -quality 85 -interlace Plane -strip output.jpg
```

有透明通道时保留 PNG：

```sh
dwebp input.webp -o output.png
oxipng -o max --strip safe output.png
```

转换后必须同步 OPF manifest、XHTML `src` 和 CSS `url()` 引用。

### 3.2 PNG 无损压缩

```sh
find OEBPS/Images -name "*.png" -exec oxipng -o max --strip safe {} +
```

不要用 `--strip all`，避免破坏 colorspace 信息。

### 3.3 JPEG 无损优化

```sh
find OEBPS/Images \( -name "*.jpg" -o -name "*.jpeg" \) | while read f; do
  jpegtran -copy none -optimize -progressive "$f" > "$f.opt" && mv "$f.opt" "$f"
done
```

`jpegtran` 不改像素，只优化编码和元数据。

### 3.4 GIF / SVG

- 静态 GIF 可转 PNG 后跑 `oxipng`。
- 动态 GIF 保留。
- SVG 保留；如需 minify，用 `svgo` 单独评估。

### 3.5 红线：cover-image 不动

```sh
unzip -p book.epub OEBPS/package.opf | grep 'cover-image'
```

定位声明行后，该 href 指向的文件必须从所有批量处理命令里排除；
构建产物可用 `epub run epub.package.nav.audit --input book.epub --json` 复核封面声明。

## 4. 字体处理

中文字体单个常有 5-15MB；一本书实际用字远少于完整字体。子集化后体积通常降到 5-15%。

### 4.1 字体子集化与全量校验

```sh
epub run epub.font.subset --input full-font-source.epub --output subset-candidate.epub --json
epub redline --check all full-font-source.epub subset-candidate.epub
```

推荐由正式 capability `epub.font.subset` 调用独立 `epub-font` provider。完整字体保留在书级 Git 的解包源中；每次都从完整母版生成候选子集，不能把上次输出当作新母版。能力只替换现有 manifest 字体 entry，不改 OPF / CSS / XHTML；provider 的逐字体核验成功后，pipeline 再运行红线并写候选。可变字体需在书根 `fonts.json` 中显式设置 `variation.mode`。书级固定产物与失败保留语义见[工作区指南](book-workspace.md)。

需要单独检查既有产物或排查字体配置时，仍可直接使用 `epub-font check`；字体 provider 的 CLI 选项见 [`epub-font` 文档](../../tools-font/epub-font/README.md)。完成静态检查后再做目标阅读器实测。

### 4.2 WOFF2 vs WOFF vs OTF/TTF

| 格式 | 大小 | 推荐 |
| --- | --- | --- |
| WOFF2 | 最小 | 主路径 |
| WOFF | 中等 | 旧 reader fallback |
| OTF / TTF | 最大 | 仅必要时保留 |
| SVG fonts | 废弃 | 不用 |

## 5. 在清洗流水线中的位置（编号对应 [cleanup-flow.md](cleanup-flow.md)）

- §1 健康检查：列出 WebP、大图、大字体作为黄线候选。
- §6 分派清洗：由 `epub-audit` 和 `epub-cleanup` 调用本指南命令。
- §7 文本校验：每个写出步骤后 `epub redline --check all` 必须退出 0。
- §8 Diff 人工 review：资源层显示文件 hash、大小和格式变化。

## 6. 验证清单

```sh
! find OEBPS -name "*.webp" -o -name "*.avif" | grep .
grep -rn "\.webp" OEBPS/ || true
grep -E "media-type=\"image/webp\"" OEBPS/package.opf && echo "WebP MIME still in OPF" || echo "OPF clean"
epub run epub.package.nav.audit --input <artifact.epub> --json
```

### 3.6 图片转化工具建议

本仓不内置图片压缩器，只推荐外部工具并在 EPUB 层复查路径、manifest、封面和 figure。
这些工具不由 CLI 探测或调用；需要自行确认已安装、运行后回到 EPUB 层复核：

| 工具 | 用途 | 人工注意事项 |
| --- | --- | --- |
| [ImageMagick `magick`](https://imagemagick.org/command-line-tools/) | WebP / TIFF / GIF / SVG 等转 JPEG / PNG，必要时 resize / identify | 转换后回到 `epub.package.nav.audit` 复核格式、manifest 与封面 |
| [oxipng](https://github.com/oxipng/oxipng) | PNG 无损优化 | 用于已经确认视觉质量的 PNG |
| [pngquant](https://pngquant.org/) | PNG 有损量化压缩 | 必须人工抽样看质量 |
| [jpegoptim](https://github.com/tjko/jpegoptim) | JPEG 优化 / 压缩 | 必须保留原图备份 |
| [svgo](https://github.com/svg/svgo) | SVG 清理 / 优化 | Kindle 主路径仍优先预栅格化风险 SVG |

外部工具只改资源字节。资源改完后必须重新运行：

```sh
epub run epub.package.nav.audit --input work/after/step-N-images.epub --json
epub redline --check all <redline-base.epub> work/after/step-N-images.epub
```

产物结构检查由 nav.audit 与 `epub redline`（正文不变）组合覆盖；EPUBCheck 在 GitHub Actions 作为 CI gate 运行。

## 7. 不做的

- 不做 AI 增强 / 超分辨率。
- 不做色彩管理 / ICC profile 转换。
- 不内置 binary 工具。
- 不自动跑破坏性命令；skill 必须先 dry-run。

## 8. 参考资料

- [oxipng](https://github.com/shssoichiro/oxipng)
- [mozjpeg](https://github.com/mozilla/mozjpeg)
- [Google WebP tools](https://developers.google.com/speed/webp/docs/precompiled)
- [`epub-font` 使用说明](../../tools-font/epub-font/README.md)
- [fontTools](https://github.com/fonttools/fonttools)
- [EPUB 3 Core Media Types](https://www.w3.org/publishing/epub32/epub-spec.html#sec-cmt-supported)
