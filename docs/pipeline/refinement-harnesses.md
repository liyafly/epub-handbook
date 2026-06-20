# EPUB 精排 harness

> 面向「已有 EPUB -> 可选结构规范化 -> EPUB3 基线 -> AI 精排建议 -> 分步清洗 -> diff review」的脚本入口。

底层 harness 默认只读或 dry-run。`epub_cleanup_pipeline.py` 会复制 before 基线后调用写出步骤；所有写出步骤都不会原地覆盖输入文件。

## 九个入口

| 脚本 | 做什么 | 何时运行 |
| --- | --- | --- |
| `scripts/epub_cleanup_pipeline.py` | 一命令生成 before 基线、EPUB3 清洗产物、validator 结果、精排建议和 AI findings 审计 bundle | 单书清洗的默认入口 |
| `scripts/epub_preflight_harness.py` | 检查 ZIP / mimetype / container / OPF / manifest / spine / XML / CSS url / DRM 标记，并复用 `epub_ai_harness.py` 的结构 findings | 拿到一本 EPUB 后第一步 |
| `scripts/epub_structure_tool.py` | 可选：`normalize` 固定先格式化目录，再按 OPF manifest id 做文件名反混淆 | 内部目录散乱或文件名不可读时，在 EPUB3 迁移前运行 |
| `scripts/epub3_migration_harness.py` | dry-run EPUB3 迁移计划；可选写出 `version="3.0"`、`dcterms:modified`、`nav.xhtml` 和 OPF nav item | preflight 没有 error 后 |
| `scripts/epub_refinement_harness.py` | 输出精排建议：EPUB3、弹注、字体链 / 内嵌字体、图片格式、Ruby / 竖排、diff 与红线 gate、候选 skills | EPUB3 基线前后都可跑；建议在迁移后再跑一次 |
<<<<<<< ours
| `scripts/epub_image_layout_advisor.py` | 只读扫描正文/封面等真实图片，输出 2–3 个布局候选、出处与决策记录命令模板；排除 noteref 图标控件 | refinement 之后；有人需要逐图选择时运行 |
| `scripts/epub_style_preset_tool.py` | 预览 class coverage，并可写入选定预设的 CSS、OPF 声明和 XHTML link | EPUB3 基线与 refinement 建议确认后，专项清洗前 |
>>>>>>> theirs
| `scripts/epub_css_cleanup.py` | 合并重复 CSS、替换旧字体链；可选把不交叠局部样式归并为一个 body-scoped CSS | EPUB3 基线通过 preflight 后 |
| `scripts/epub_anthology_refinement.py` | 把“单图卷封 + 紧邻版权页”转换为 A-lite contain 背景、原图 fallback 和紧凑版权排版 | 只在合订 EPUB 明确需要时运行 |

## 推荐顺序

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub

python3 scripts/epub_preflight_harness.py \
  work/before/source.epub \
  --format json > work/preflight.json
```

如果 `work/preflight.json` 里有 `error`，先修 package 错误，不进入 AI 清洗。需要结构规范化时，先按 [cleanup-flow.md §1.5](cleanup-flow.md#15-可选先格式化再文件名反混淆) 运行 `epub_structure_tool.py normalize`。然后把 step-0 产物作为 `BASE`；没有 step-0 时回退到原始复制件：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

python3 scripts/epub3_migration_harness.py \
  "$BASE" \
  --write-output work/after/step-1-epub3.epub \
  --format json > work/epub3-migration.json
```

迁移后跑红线。新增的 nav 文件可以 allow-list；正文、核心 metadata、spine 和锚点仍要不变：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

python3 scripts/validate_text_invariance.py \
  "$BASE" \
  work/after/step-1-epub3.epub \
  --check text,metadata,spine,cover,anchors \
  --allow-list '*/nav*.xhtml'
```

然后生成精排建议：

```sh
python3 scripts/epub_refinement_harness.py \
  work/after/step-1-epub3.epub \
  --format json > work/refinement.json

python3 scripts/epub_image_layout_advisor.py \
  work/after/step-1-epub3.epub \
  --format md \
  --report work/image-layout-advice.md
```

`epub_refinement_harness.py` 负责整本书的全局事实与阶段建议；`epub_image_layout_advisor.py` 只处理图片专项，把问题图变成逐图候选菜单。候选仍由人选择，确认后用报告末尾的 `epub_decision_log.py add` 模板把结果写入决策记录。

若书型已确定，可在 refinement 后先预览风格预设：

```sh
python3 scripts/epub_style_preset_tool.py apply \
  work/after/step-1-epub3.epub \
  --preset literary-cn \
  --output work/after/step-2-literary-cn.epub \
  --dry-run
```

coverage 低于 30% 时先完成 class 体系迁移；coverage 足够且人工确认后移除
`--dry-run`。写出产物必须立刻跑 `validate_text_invariance.py --check all`。
预设视觉效果尚未完成 reader-matrix 实测，不能把结构合规当作阅读器视觉结论。

## AI 应该怎么用

`epub_refinement_harness.py` 的 `recommendations` 是决策输入，不是自动执行器。AI 或人类按以下规则分派：

1. `preflight` 是硬门禁；有 error 就停。
2. `epub3-migration` 优先于弹注、字体和图片精排。
3. `popup-notes` 只允许 dry-run 后执行，注释正文必须保留。识别到 Sigil `noteref_N/footnote_N` 单条 `aside` 结构时，只有全部本地 notes 都能重组为一个 grouped `aside/ol/li` 才写出；图片触发器使用 `sup.note-marker` 的零行高外壳和相对上移，绝不使用全局 `sup img`。随后跑 `validate-popup-notes.sh` 和完整 `validate_text_invariance.py --check all`。文本 gate 只忽略 noteref/backlink 控件文字，不忽略注释正文。
3. `popup-notes` 只允许 dry-run 后执行，注释正文必须保留。识别到 Sigil `noteref_N/footnote_N` 单条 `aside` 结构时，只有全部本地 notes 都能重组为一个 grouped `aside/ol/li` 才写出；随后跑 `validate-popup-notes.sh` 和完整 `validate_text_invariance.py --check all`。文本 gate 只忽略 noteref/backlink 控件文字，不忽略注释正文。
4. `typography-fonts` 需要 AI 判断：默认系统优先字体链；内嵌字体只用于标题、题签、生僻字或明确的全字符集例外。
5. `images` 只负责识别格式和版式风险；真实压缩 / 转码交给外部工具，完成后再回到 package/nav audit。
6. 每个写出步骤都生成 `work/after/step-N-*.epub`，立刻跑 `validate_text_invariance.py`。
7. 最终交付前按 [EPUB diff review](epub-diff-review.md) 做五层人工 review。

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
python3 scripts/validate_text_invariance.py <redline-base.epub> work/after/step-N-images.epub --check all
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
- `tool_availability`: 本机是否有 `magick`、`oxipng`、`pngquant`、`jpegoptim`、`svgo`；EPUBCheck 在 GitHub Actions 中检查
- `recommendations`: 分阶段建议与候选 skills
