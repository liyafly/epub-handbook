# 资源优化：图片与字体

> 状态：操作指南；用现有工具，不写新脚本。
> 对应清洗步骤：[cleanup-flow.md](cleanup-flow.md) §4。
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
| 字体子集化 | `pyftsubset` / fonttools | `python3 -m pip install fonttools brotli zopfli` |
| 字形扫描 | `glyphhanger` | `npm install -g glyphhanger` |

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
python3 -c "
import xml.etree.ElementTree as ET
root = ET.parse('OEBPS/package.opf').getroot()
ns = {'opf': 'http://www.idpf.org/2007/opf'}
for item in root.findall('.//opf:item', ns):
    if 'cover-image' in (item.get('properties') or ''):
        print('COVER:', item.get('href'))
"
```

这个文件路径必须从所有批量处理命令里排除。

## 4. 字体处理

中文字体单个常有 5-15MB；一本书实际用字远少于完整字体。子集化后体积通常降到 5-15%。

### 4.1 glyphhanger

```sh
npm install -g glyphhanger
glyphhanger OEBPS/Text/*.xhtml \
  --formats=woff2 \
  --subset=OEBPS/Fonts/NotoSerifSC.otf
```

### 4.2 pyftsubset

```sh
pyftsubset OEBPS/Fonts/NotoSerifSC.otf \
  --text-file=/tmp/used-chars.txt \
  --output-file=OEBPS/Fonts/NotoSerifSC-subset.woff2 \
  --flavor=woff2 \
  --no-hinting \
  --desubroutinize
```

子集化后同步 CSS `@font-face` 和 OPF manifest，再确认没有旧字体引用。

### 4.3 WOFF2 vs WOFF vs OTF/TTF

| 格式 | 大小 | 推荐 |
| --- | --- | --- |
| WOFF2 | 最小 | 主路径 |
| WOFF | 中等 | 旧 reader fallback |
| OTF / TTF | 最大 | 仅必要时保留 |
| SVG fonts | 废弃 | 不用 |

## 5. 在清洗流水线中的位置

- §1 健康检查：列出 WebP、大图、大字体作为黄线候选。
- §4 清洗执行：由 `epub-image-layout-optimizer` 和 `epub-typography-optimizer` 调用本指南命令。
- §5 红线校验：`validate_text_invariance.py --check all` 必须退出 0。
- §6 Diff 报告：资源层显示文件 hash、大小和格式变化。

## 6. 验证清单

```sh
! find OEBPS -name "*.webp" -o -name "*.avif" | grep .
grep -rn "\.webp" OEBPS/ || true
grep -E "media-type=\"image/webp\"" OEBPS/package.opf && echo "WebP MIME still in OPF" || echo "OPF clean"
which epubcheck >/dev/null && epubcheck . || echo "epubcheck not installed; skip"
```

## 7. 不做的

- 不做 AI 增强 / 超分辨率。
- 不做色彩管理 / ICC profile 转换。
- 不内置 binary 工具。
- 不自动跑破坏性命令；skill 必须先 dry-run。

## 8. 参考资料

- [oxipng](https://github.com/shssoichiro/oxipng)
- [mozjpeg](https://github.com/mozilla/mozjpeg)
- [Google WebP tools](https://developers.google.com/speed/webp/docs/precompiled)
- [fonttools / pyftsubset](https://github.com/fonttools/fonttools)
- [glyphhanger](https://github.com/zachleat/glyphhanger)
- [EPUB 3 Core Media Types](https://www.w3.org/publishing/epub32/epub-spec.html#sec-cmt-supported)
