# EPUB 精排能力

> 面向「已有 EPUB -> 可选结构规范化 -> EPUB3 基线 -> AI 精排建议 -> 分步清洗 -> diff review」的执行入口。

审计类能力只读；写出型能力要求显式 `--output`，不原地覆盖输入文件。before 基线复制按 [cleanup-flow.md §0](cleanup-flow.md#0-准备) 人工完成。

新书级项目先按 [一书一 Git 工作区](book-workspace.md) 建立目录。下文命令中的
`work/` 是 `work-epub/<book>/03 制作工作区/.pipeline/` 的流水线内部简写，不是书级顶层目录。

## 能力总览

| 能力 / 命令 | 做什么 | 何时运行 |
| --- | --- | --- |
| 按序清洗序列（见 [cleanup-flow.md](cleanup-flow.md)） | 保留 before 基线、结构审计、结构规范化、EPUB3 迁移、CSS / 排版精排、redline 校验 | 单书清洗的默认顺序 |
| `epub run epub.package.nav.audit` | 检查 ZIP / mimetype / container / OPF / manifest / spine / XML / CSS url / DRM 标记，并给出结构 findings | 拿到一本 EPUB 后第一步 |
| `epub run epub.structure.normalize` | 可选：`mode=normalize` 固定先格式化目录，再按 OPF manifest id 做文件名反混淆；`mode=inspect` 只读检查 | 内部目录散乱或文件名不可读时，在 EPUB3 迁移前运行 |
| `epub run epub.package.migrate.epub3 --dry-run` | 生成 EPUB3 迁移计划（`approval-required`，退出码 2） | 结构审计没有 error 后，先只读审查 |
| `epub run epub.package.migrate.epub3` | 按确认后的计划写出新 EPUB3，报告 before/after SHA-256 和转换明细 | 计划确认后；不原地覆盖输入 |
| `epub run epub.layout.audit` + `epub run epub.text.content.analyze` + `epub run epub.image.layout.optimize` + `epub run epub.font.coverage.analyze` | 精排建议组合：全局事实与阶段建议、文本结构角色、图片版式候选、字体覆盖风险 | EPUB3 基线前后都可跑；建议在迁移后再跑一次 |
| `epub run epub.text.content.analyze` | 只读识别文本结构角色，并给出字体角色与可重排排版建议 | 精排建议后、语义 class 分派前 |
| `epub run epub.font.coverage.analyze` | 只读调用独立字体覆盖 detector，检查 cmap、缺字、链命中和 reader profile 风险 | 字体策略确定前后；EPUB 含嵌入字体或生僻字时 |
| `epub run epub.image.layout.optimize` | 只读扫描正文/封面等真实图片，输出布局候选与风险；排除 noteref 图标控件 | 精排建议之后；有人需要逐图选择时运行 |
| `epub run epub.typography.optimize` | 预览 class coverage，并可写入选定预设的 CSS、OPF 声明和 XHTML link | EPUB3 基线与精排建议确认后，专项清洗前 |
| `epub run epub.css.layering.optimize` | 合并重复 CSS、替换旧字体链；可选把不交叠局部样式归并为一个 body-scoped CSS | EPUB3 基线通过结构审计后 |
| `epub run epub.alite.convert` | 把“单图卷封 + 紧邻版权页”转换为 A-lite contain 背景、原图 fallback 和紧凑版权排版 | 只在合订 EPUB 明确需要时运行 |

## 推荐顺序

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub

epub run epub.package.nav.audit \
  --input work/before/source.epub \
  --json > work/preflight.json
```

如果 `work/preflight.json` 里有 error 级 finding，先修 package 错误，不进入 AI 清洗。需要结构规范化时，先按 [cleanup-flow.md §1.5](cleanup-flow.md#15-可选先格式化再文件名反混淆) 运行 `epub run epub.structure.normalize --dry-run`。然后把 step-0 产物作为 `BASE`；没有 step-0 时回退到原始复制件：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

epub run epub.package.migrate.epub3 \
  --input "$BASE" \
  --output work/after/step-1-epub3.epub \
  --dry-run --json > work/epub3-migration-plan.json

epub run epub.package.migrate.epub3 \
  --input "$BASE" \
  --output work/after/step-1-epub3.epub \
  --json > work/epub3-migration-apply.json
```

迁移后跑红线。新增的 nav 文件可以 allow-list；正文、核心 metadata、spine 和锚点仍要不变：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

epub redline --check text,metadata,spine,cover,anchors \
  --allow-list '*/nav*.xhtml' \
  "$BASE" \
  work/after/step-1-epub3.epub
```

然后生成精排建议：

```sh
epub run epub.layout.audit \
  --input work/after/step-1-epub3.epub \
  --json > work/refinement.json

epub run epub.text.content.analyze \
  --input work/after/step-1-epub3.epub \
  --json > work/content-analysis.json

epub run epub.font.coverage.analyze \
  --input work/after/step-1-epub3.epub \
  --json profile=kindle-pessimistic > work/font-coverage.json

epub run epub.image.layout.optimize \
  --input work/after/step-1-epub3.epub \
  --json > work/image-layout-advice.json
```

`epub.layout.audit` 负责整本书的全局事实与阶段建议；`epub.image.layout.optimize` 只处理图片专项，把问题图变成逐图候选菜单。候选仍由人选择，确认后按 [cleanup-flow.md §8](cleanup-flow.md#8-diff-人工-review) 的记录模板把结果写入 `records/typeset-decisions.jsonl`。

若书型已确定，可在精排建议后先预览风格预设：

```sh
epub run epub.typography.optimize \
  --input work/after/step-1-epub3.epub \
  --output work/after/step-2-literary-cn.epub \
  --dry-run --json preset=literary-cn
```

coverage 低于 30% 时先完成 class 体系迁移；coverage 足够且人工确认后去掉
`--dry-run`。写出产物必须立刻跑 `epub redline --check all <redline-base.epub> work/after/step-2-literary-cn.epub`。
预设视觉效果尚未完成 reader-matrix 实测，不能把结构合规当作阅读器视觉结论。

## AI 应该怎么用

精排建议（`epub.layout.audit` 等报告的 findings 与 `nextCommands`）是决策输入，不是自动执行器。AI 或人类按以下规则分派：

1. `epub.package.nav.audit` 是硬门禁；有 error 就停。
2. `epub3-migration` 优先于弹注、字体和图片精排。
3. `popup-notes` 只允许 dry-run 后执行，注释正文必须保留。识别到 Sigil `noteref_N/footnote_N` 单条 `aside` 结构时，只有全部本地 notes 都能重组为一个 grouped `aside/ol/li` 才写出；图片触发器使用 `sup.note-marker` 的零行高外壳和相对上移，绝不使用全局 `sup img`。随后跑 `epub run epub.notes.popup.normalize` 和完整 `epub redline --check all`。文本 gate 只忽略 noteref/backlink 控件文字，不忽略注释正文。
4. `typography-fonts` 需要 AI 判断：普通正文默认自由，显式角色优先短系统链；内嵌字体只用于标题、题签、生僻字，或用户明确选择且覆盖正文角色全部实际字符的锁定版。
5. `images` 只负责识别格式和版式风险；真实压缩 / 转码交给外部工具，完成后再回到 package/nav audit。
6. 每个写出步骤都生成 `work/after/step-N-*.epub`，立刻跑 `epub redline --check all`。
7. 最终交付前按 [EPUB diff review](epub-diff-review.md) 做五层人工 review。

## 图片转化工具建议

本仓不内置图片压缩器，只推荐外部工具并在 EPUB 层复查路径、manifest、封面和 figure：

| 工具 | 用途 | 报告中的处理 |
| --- | --- | --- |
| [ImageMagick `magick`](https://imagemagick.org/command-line-tools/) | WebP / TIFF / GIF / SVG 等转 JPEG / PNG，必要时 resize / identify | `epub.package.nav.audit` 检测 `magick` 是否在 PATH |
| [oxipng](https://github.com/oxipng/oxipng) | PNG 无损优化 | 检测 PATH；建议用于已经确认视觉质量的 PNG |
| [pngquant](https://pngquant.org/) | PNG 有损量化压缩 | 检测 PATH；必须人工抽样看质量 |
| [jpegoptim](https://github.com/tjko/jpegoptim) | JPEG 优化 / 压缩 | 检测 PATH；必须保留原图备份 |
| [svgo](https://github.com/svg/svgo) | SVG 清理 / 优化 | 检测 PATH；Kindle 主路径仍优先预栅格化风险 SVG |

外部工具只改资源字节。资源改完后必须重新运行：

```sh
epub run epub.package.nav.audit --input work/after/step-N-images.epub --json
epub redline --check all <redline-base.epub> work/after/step-N-images.epub
```

## 输出字段

各能力默认输出统一信封（`status` / `facts` / `findings` / `nextCommands`，见 [SPEC-go-architecture §8.2](../final/SPEC-go-architecture.md)）。迁移期脚手架 `legacy_report=true` 会把 Python oracle 形状的原始报告保留在 `facts.legacyReport`。

`epub.package.nav.audit`：

- `status`: `complete` / `failed`
- `facts.summary`: zip entry、manifest / spine 数量、媒体类型计数等包结构统计
- `findings[]`: package / XML / manifest / CSS url findings
- `nextCommands[]`: 可交给 AI 的后续命令候选

`epub.package.migrate.epub3 --dry-run`：

- `status`: `approval-required`（退出码 2），列出将执行的 OPF / nav 变更
- `findings[]`: 无 NCX、DRM 标记等需要人工判断的警告

`epub.package.migrate.epub3`：

- `capability`: 固定为 `epub.package.migrate.epub3`
- `input` / `output`: 输入与新产物的 SHA-256
- `facts`: 底层迁移明细（nav 条目、XHTML 更新数、弹注转换数等）

`epub.layout.audit`：

- `facts.summary` / `findings[]`: 版本、nav、图片、字体、弹注、Ruby / 竖排等统计与分阶段建议
- `legacy_report=true` 时 `facts.legacyReport` 保留完整 facts（含 `tool_availability`：本机是否有 `magick`、`oxipng`、`pngquant`、`jpegoptim`、`svgo`；EPUBCheck 在 GitHub Actions 中检查）
- `nextCommands[]`: 候选 skills 与后续命令

`epub.text.content.analyze`：

- `facts`: blocks 总数、`review_required` 计数与角色分布
- 完整 blocks 明细（locator、候选结构角色、置信度、证据和字体/排版角色建议）在 `legacy_report=true` 时保留于 `facts.legacyReport`
- 默认不输出正文；需要片段时加 `include_snippets=true`

`epub.font.coverage.analyze`：

- `facts.summary.by_profile_risk`: reader profile 下的 `ok | risk | fail`
- `facts.profile` / `facts.status`: 本次 profile 与总体结论
- `legacy_report=true` 时 `facts.legacyReport` 保留问题字、覆盖位置、原因、出现位置和 `chain_health` / `unresolved` 明细
