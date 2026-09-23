# EPUB 精排能力

> 面向「已有 EPUB -> 可选结构规范化 -> EPUB3 基线 -> AI 精排建议 -> 分步清洗 -> diff review」的执行入口。

审计类能力只读；单输出写能力要求显式 `--output`，多输出 split 用 `output_dir=`，均不原地覆盖输入。before 基线复制按 [cleanup-flow.md §0](cleanup-flow.md#0-准备) 人工完成。

新书级项目先按 [一书一 Git 工作区](book-workspace.md) 建立目录。下文命令中的
`work/` 是 `work-epub/<book>/03 制作工作区/.pipeline/` 的流水线内部简写，不是书级顶层目录。

## 能力总览

| 能力 / 命令 | 做什么 | 何时运行 |
| --- | --- | --- |
| 按序清洗序列（见 [cleanup-flow.md](cleanup-flow.md)） | 保留 before 基线、结构审计、结构规范化、EPUB3 迁移、CSS / 排版精排、redline 校验 | 单书清洗的默认顺序 |
| `epub run epub.package.nav.audit` | 检查 ZIP / mimetype / container / OPF / manifest / spine / XML / CSS url / DRM 标记，并给出结构 findings | 拿到一本 EPUB 后第一步 |
| `epub run epub.structure.normalize` | 可选：先格式化目录，再按 OPF manifest id 反混淆；inspect 非 dry-run 会写未修改副本 | 内部目录散乱或文件名不可读时，在 EPUB3 迁移前运行 |
| `epub run epub.package.migrate.epub3 --dry-run` | 生成 EPUB3 迁移计划，仍需检查具体 findings | 排除 DRM/损坏阻断后，先审查计划 |
| `epub run epub.package.migrate.epub3` | 按确认后的计划写出新 EPUB3，报告 before/after SHA-256 和转换明细 | 计划确认后；不原地覆盖输入 |
| `epub run epub.layout.audit` + `epub run epub.text.content.analyze` + `epub run epub.image.layout.optimize` + `epub run epub.font.coverage.analyze` | 精排建议组合：全局事实与阶段建议、文本结构角色、图片版式候选、字体覆盖风险 | EPUB3 基线前后都可跑；建议在迁移后再跑一次 |
| `epub run epub.text.content.analyze` | 只读识别文本结构角色，并给出字体角色与可重排排版建议 | 精排建议后、语义 class 分派前 |
| `epub run epub.font.coverage.analyze` | 只读调用独立字体覆盖 detector，检查 cmap、缺字、链命中和 reader profile 风险 | 字体策略确定前后；EPUB 含嵌入字体或生僻字时 |
| `epub run epub.image.layout.optimize` | 只读扫描正文/封面等真实图片，输出布局候选与风险；排除 noteref 图标控件 | 精排建议之后；有人需要逐图选择时运行 |
| `epub run epub.typography.optimize` | 预览 class coverage，并可写入选定预设的 CSS、OPF 声明和 XHTML link | EPUB3 基线与精排建议确认后，专项清洗前 |
| `epub run epub.css.layering.optimize` | 保守修补分号、装饰行与已知旧字体链；自动去重、语义分层、scoped merge 均停用 | 审查具体 CSS 修改范围后 |
| `epub run epub.alite.convert` | 把“单图卷封 + 紧邻版权页”转换为 A-lite contain 背景、原图 fallback 和紧凑版权排版 | 只在合订 EPUB 明确需要时运行 |

## 推荐顺序

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub

epub run epub.package.nav.audit \
  --input work/before/source.epub \
  --json > work/preflight.json
```

先区分 DRM/未知加密、无法读取的 package 等阻断，与可由迁移修复的版本/导航诊断；后者可进入对应修复，但必须复检产物。需要结构规范化时，先按 [cleanup-flow.md §1.5](cleanup-flow.md#15-可选先格式化再文件名反混淆) 审查 dry-run。把 step-0 产物作为 `BASE`；没有 step-0 时回退到原始复制件：

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

coverage 是 preset 类覆盖率，低于 30% 时先核对是否适合该预设，不为达标强行重命名全书 class。先审查样式替换与代表性页面，再决定应用或局部人工调整。写出产物必须立刻跑 `epub redline --check all <redline-base.epub> work/after/step-2-literary-cn.epub`。
预设视觉效果尚未完成 reader-matrix 实测，不能把结构合规当作阅读器视觉结论。

## AI 应该怎么用

精排建议（`epub.layout.audit` 等报告的 findings 与 `nextCommands`）是决策输入，不是自动执行器。AI 或人类按以下规则分派：

1. 先预检并分类处理诊断；DRM/损坏是阻断，其他问题按修复能力边界判断，输出须复检。
2. 需要 EPUB3 迁移才进入迁移，不把局部精排扩成全书转换。
3. popup normalize 只是校验器。已识别旧注释可随授权迁移转换，否则人工处理；目标结构与图标样式以 [SPEC §1](../final/SPEC-实现约束.md) 和 fixture 为准。模糊 Sigil 分组不部分转换；注释正文保留，修改后跑 popup validator 和全项 redline。
4. `typography-fonts` 需要 AI 判断：普通正文默认自由，显式角色优先短系统链；内嵌字体只用于标题、题签、生僻字，或用户明确选择且覆盖正文角色全部实际字符的锁定版。
5. `images` 只负责识别格式和版式风险；真实压缩 / 转码交给外部工具，完成后再回到 package/nav audit。
6. 每个写出步骤都生成 `work/after/step-N-*.epub`，立刻跑 `epub redline --check all`。
7. 最终交付前按 [EPUB diff review](epub-diff-review.md) 做五层人工 review。

## 图片转化工具建议

本仓不内置图片压缩器，只推荐外部工具并在 EPUB 层复查路径、manifest、封面和 figure。
**这些工具都不由 CLI 探测或调用**（layout audit 的 `facts["epub.layout.audit.toolAvailability"]` 目前只探测 `epubcheck`，nav audit 则是 `facts["epub.package.nav.audit.toolAvailability"]`），
需要自己确认已安装、自己运行，然后回到 EPUB 层复核：

| 工具 | 用途 | 人工注意事项 |
| --- | --- | --- |
| [ImageMagick `magick`](https://imagemagick.org/command-line-tools/) | WebP / TIFF / GIF / SVG 等转 JPEG / PNG，必要时 resize / identify | 转换后回到 `epub.package.nav.audit` 复核格式、manifest 与封面 |
| [oxipng](https://github.com/oxipng/oxipng) | PNG 无损优化 | 建议用于已经确认视觉质量的 PNG |
| [pngquant](https://pngquant.org/) | PNG 有损量化压缩 | 必须人工抽样看质量 |
| [jpegoptim](https://github.com/tjko/jpegoptim) | JPEG 优化 / 压缩 | 必须保留原图备份 |
| [svgo](https://github.com/svg/svgo) | SVG 清理 / 优化 | Kindle 主路径仍优先预栅格化风险 SVG |

外部工具只改资源字节。资源改完后必须重新运行：

```sh
epub run epub.package.nav.audit --input work/after/step-N-images.epub --json
epub redline --check all <redline-base.epub> work/after/step-N-images.epub
```

## 输出字段

统一信封、可选字段、退出码、上游诊断与建议命令的解释只在 [skills 公共命令与返回](../../skills/README.md#公共命令与返回) 维护。能力专属 facts 见该索引中的对应 SKILL.md；所有能力 facts 均以能力 id 为前缀，不另造报告形状。

特别注意：typography 的 coverage 是 class 覆盖，不是字体 cmap；字体检测器失败不提供覆盖结论；layout audit 当前是结构/风险审查而非实际渲染器；dry-run 状态必须结合 findings，不能仅靠“没有输出文件”判定通过。
