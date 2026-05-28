# EPUB 精排 harness

> 面向「已有 EPUB -> EPUB3 基线 -> AI 精排建议 -> 分步清洗 -> diff review」的脚本入口。

这些 harness 默认只读或 dry-run。只有 `epub3_migration_harness.py --write-output` 会写出新 EPUB；它不会原地覆盖输入文件。

## 三个入口

| 脚本 | 做什么 | 何时运行 |
| --- | --- | --- |
| `scripts/epub_preflight_harness.py` | 检查 ZIP / mimetype / container / OPF / manifest / spine / XML / CSS url / DRM 标记，并复用 `epub_ai_harness.py` 的结构 findings | 拿到一本 EPUB 后第一步 |
| `scripts/epub3_migration_harness.py` | dry-run EPUB3 迁移计划；可选写出 `version="3.0"`、`dcterms:modified`、`nav.xhtml` 和 OPF nav item | preflight 没有 error 后 |
| `scripts/epub_refinement_harness.py` | 输出精排建议：EPUB3、弹注、字体链 / 内嵌字体、图片格式、Ruby / 竖排、diff 与红线 gate、候选 skills | EPUB3 基线前后都可跑；建议在迁移后再跑一次 |

## 推荐顺序

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub

python3 scripts/epub_preflight_harness.py \
  work/before/source.epub \
  --format json > work/preflight.json
```

如果 `work/preflight.json` 里有 `error`，先修 package 错误，不进入 AI 清洗。没有 `error` 后，如果 `package_version` 不是 `3.0` 或缺 nav：

```sh
python3 scripts/epub3_migration_harness.py \
  work/before/source.epub \
  --write-output work/after/step-1-epub3.epub \
  --format json > work/epub3-migration.json
```

迁移后跑红线。新增的 nav 文件可以 allow-list；正文、核心 metadata、spine 和锚点仍要不变：

```sh
python3 scripts/validate_text_invariance.py \
  work/before/source.epub \
  work/after/step-1-epub3.epub \
  --check text,metadata,spine,cover,anchors \
  --allow-list '*/nav*.xhtml'
```

然后生成精排建议：

```sh
python3 scripts/epub_refinement_harness.py \
  work/after/step-1-epub3.epub \
  --format json > work/refinement.json
```

## AI 应该怎么用

`epub_refinement_harness.py` 的 `recommendations` 是决策输入，不是自动执行器。AI 或人类按以下规则分派：

1. `preflight` 是硬门禁；有 error 就停。
2. `epub3-migration` 优先于弹注、字体和图片精排。
3. `popup-notes` 只允许 dry-run 后执行，注释正文必须保留。
4. `typography-fonts` 需要 AI 判断：默认系统优先字体链；内嵌字体只用于标题、题签、生僻字或明确的全字符集例外。
5. `images` 只负责识别格式和版式风险；真实压缩 / 转码交给外部工具，完成后再回到 package/nav audit。
6. 每个写出步骤都生成 `work/after/step-N-*.epub`，立刻跑 `validate_text_invariance.py`。
7. 最终交付前按根 README 的 `#epub-diff-review` 做五层人工 review。

## 图片转化工具建议

本仓不内置图片压缩器，只推荐外部工具并在 EPUB 层复查路径、manifest、封面和 figure：

| 工具 | 用途 | harness 中的处理 |
| --- | --- | --- |
| [ImageMagick `magick`](https://imagemagick.org/command-line-tools/) | WebP / TIFF / GIF / SVG 等转 JPEG / PNG，必要时 resize / identify | `epub_refinement_harness.py` 检测 `magick` 是否在 PATH |
| [oxipng](https://github.com/oxipng/oxipng) | PNG 无损优化 | 检测 PATH；建议用于已经确认视觉质量的 PNG |
| [pngquant](https://pngquant.org/) | PNG 有损量化压缩 | 检测 PATH；必须人工抽样看质量 |
| [jpegoptim](https://github.com/tjko/jpegoptim) | JPEG 优化 / 压缩 | 检测 PATH；必须保留原图备份 |
| [svgo](https://github.com/svg/svgo) | SVG 清理 / 优化 | 检测 PATH；Kindle 主路径仍优先预栅格化风险 SVG |

外部工具只改资源字节。资源改完后必须重新运行：

```sh
python3 scripts/epub_preflight_harness.py work/after/step-N-images.epub --format json
python3 scripts/validate_text_invariance.py work/before/source.epub work/after/step-N-images.epub --check all
```

## 输出字段

`epub_preflight_harness.py`：

- `preflight_status`: `pass` / `warn` / `fail`
- `findings`: package / XML / manifest / CSS url findings
- `recommended_skills`: 可交给 AI 的 skill 候选

`epub3_migration_harness.py`：

- `actions`: 会改哪些 OPF / nav 字段
- `warnings`: 无 NCX、DRM 标记等需要人工判断的情况
- `written_output`: 使用 `--write-output` 后的新 EPUB 路径

`epub_refinement_harness.py`：

- `facts`: 版本、nav、图片、字体、弹注、Ruby / 竖排等统计
- `tool_availability`: 本机是否有 `magick`、`oxipng`、`pngquant`、`jpegoptim`、`svgo`、`epubcheck`
- `recommendations`: 分阶段建议与候选 skills
