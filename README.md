# epub-handbook

> 第一次接触、还不懂 EPUB？先读 [docs/learn/00-what-is-epub.md](docs/learn/00-what-is-epub.md)，5 分钟搞懂这仓库能帮你做什么。

中文 EPUB 3 制作与 AI 协作工具集。围绕「硬约束 + 自造 demo + 阅读器实测 + 自动化 skill」四件套构建：所有规则都有 demo fixture 兜底，通过阅读器实测确认的兼容性结论会写入 `reader-matrix.yaml` 并标为 `pass` 或 `fail`；尚未完成复测的条目保留为 `warn`，warn 不等于已验证，所有 AI 行为都按写定的 skill 契约执行。

A practical handbook for EPUB authoring, typography, compatibility, and reading-system behavior across Apple Books, Kindle, Readium, and Readest.

适合：

- 制作中文 EPUB 3 的工程师与编辑
- 想用 AI 帮忙清洗已有 epub 的人
- 想给团队约定 epub 制作规范的 maintainer

## 仓库做什么

1. **工程契约层** — [docs/final/](docs/final/)：SPEC、终极手册、HTML / CSS 属性速查表、阅读器兼容性实测矩阵 `reader-matrix.yaml`。这是对外硬约束。
2. **清洗流水线** — [docs/pipeline/](docs/pipeline/)：已有 EPUB 的清洗工作流，含红线 gate `scripts/validate_text_invariance.py`、harness 扫描器、典型脏 EPUB 模式识别。
3. **AI 协作 skills** — [skills/](skills/)：2 个主入口（`epub-layout-auditor` / `epub-source-intake`）+ 13 个专项 skill。可被 Claude Code / Codex 直接调用，也可由人工照 `SKILL.md` 步骤执行。
4. **入门教程** — [docs/learn/](docs/learn/)：第一次接触本仓的人按这里走。
5. **场景指南** — [docs/how-to/](docs/how-to/)：特定排版场景的实操指南（英文小说、文白对照、章首图等）。另有 ⭐ [Kindle 字体渲染深度参考](docs/how-to/kindle-font-rendering-deep-dive.md)——解释 `font-family` 回退链在 Kindle 上为什么会失灵、生僻字变方块的根因与三种应对策略。
6. **双引擎执行层** — Python（`scripts/`）与 Swift（`swift/`）**按 capability 对等并存**：Python 是 AI agent / CLI / 验证基线的首要 provider；Swift 是 native 执行核心，GUI 能执行的大部分能力原生重写。两者通过 `contracts/` + `adapters/` 共享机器契约。GUI（`gui/`）当前 PARKED。字体/图片转换为独立 Python 项目。

## 项目目标与完成标准

本仓的目标不是只收集 EPUB 知识，而是把它做成两个可执行产品：

1. **新人学习排版**：从 demo、教程、SPEC 和 reader-matrix 理解中文 EPUB 3 的可重排排版规则。
2. **已有 EPUB 精致排版工具**：对一本现成 EPUB 做 preflight、EPUB3 迁移、弹注标准化、多字体 / 内嵌字体建议、图片转化建议、AI skill 分派、红线 gate 和 diff review。

一次任务算完成，至少要留下：输入 / 输出 EPUB 路径、preflight 结果、EPUB3 迁移结果或跳过理由、精排建议 JSON、红线命令结果、diff review 结论、阅读器实测结果或跳过理由、需要回写的文档 / skill 清单。这样后来的维护者不用猜改动依据。

## 我要做什么？

| 场景 | 入口 |
| --- | --- |
| 第一次接手，判断怎么走 | 先看 [#项目目标与完成标准](#项目目标与完成标准)，再跑 [#5-分钟跑通](#5-分钟跑通) |
| 从零做一本新书 | [docs/learn/01-first-epub.md#做你自己的最小书](docs/learn/01-first-epub.md#做你自己的最小书) |
| 改造一本现成 EPUB | [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md) |
| 给一本 EPUB 出精排建议 | [docs/pipeline/refinement-harnesses.md](docs/pipeline/refinement-harnesses.md) + `scripts/epub_refinement_harness.py` |
| 合并 / 拆分 EPUB，或编辑封面 / 元数据 | [docs/pipeline/package-operations.md](docs/pipeline/package-operations.md) + `scripts/epub_package_tool.py` |
| 跑自造清洗样本 | [templates/cleanup-demo-books/README.md](templates/cleanup-demo-books/README.md) |
| 做实时样本 demo 实验 | [templates/epub-style-demo/README.md](templates/epub-style-demo/README.md) + [templates/epub-style-demo/SCENE_MATRIX.md](templates/epub-style-demo/SCENE_MATRIX.md) |
| 看制作硬规则 | [docs/final/SPEC-实现约束.md](docs/final/SPEC-实现约束.md) |
| 查 HTML / CSS 属性 | [docs/final/EPUB 3 HTML CSS 属性速查表.md](docs/final/EPUB%203%20HTML%20CSS%20属性速查表.md) |
| 看阅读器兼容性记录 | [docs/final/reader-matrix.yaml](docs/final/reader-matrix.yaml) |
| 对比改前 / 改后 | [docs/pipeline/epub-diff-review.md](docs/pipeline/epub-diff-review.md) |
| 给 AI 接入 | 先读 [AGENTS.md](AGENTS.md)，再按 [skills/README.md](skills/README.md) 选择专项 skill；metadata 在 `skills/*/agents/openai.yaml` |
| 使用 Swift 核心（macOS App 已 PARKED） | [swift/](swift/)；gui/ 当前不投入，执行逻辑向 swift/ 收口 |
| 看场景化指南 | [docs/how-to/](docs/how-to/) |
| 维护与贡献 | [CONTRIBUTING.md](CONTRIBUTING.md) + [AGENTS.md](AGENTS.md) |

## 准备环境

| 必需 | 用途 |
| --- | --- |
| bash / zip / unzip | 打包 / 解压 EPUB |
| uv + Python 3.14 | 红线脚本、harness、validator |
| Xcode 26.5 / Swift 6.3.2 | Swift core、AppKit macOS target 与测试 |
| mise + Tuist 4.200.5 | 以 `gui/Project.swift` 生成 Xcode workspace；生成物不入 Git |
| git | 仓库 + `git diff --no-index` 当 diff 引擎 |

首次 clone 后，用 uv 复现本仓环境：

```sh
# 没装 uv 时，macOS 可先运行：brew install uv
uv sync
uv run python --version
uv run python scripts/validate_skills_basic.py
```

`uv sync` 会按 `.python-version` / `pyproject.toml` 使用 Python 3.14，并在本地创建 `.venv/`。`.venv/` 是本机环境，不入 git。

推荐：

- **Calibre 5+** — 主路径 diff review（macOS / Windows / Linux 均有官方安装包）
- **VS Code** — 精细 diff review
- **Kindle Previewer 3** — Kindle 转换风险预检
- **Apple Books** — macOS / iOS 实测
- **ImageMagick `magick`** — WebP / TIFF / GIF / SVG 等图片转 JPEG / PNG
- **oxipng / pngquant / jpegoptim / svgo** — PNG、JPEG、SVG 外部优化；本仓只检测和复查，不内置压缩器
- **lxml** — 可选，为未来的 OPF/XHTML 解析工具预留；当前所有脚本仅用标准库 `xml.etree`，无需安装即可运行

EPUBCheck 不要求本机安装；GitHub Actions 会安装并作为发布前 gate 运行。

## 5 分钟跑通

```sh
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
```

详细教程见 [docs/learn/01-first-epub.md](docs/learn/01-first-epub.md)。

## 脚本速查

所有 Python 工具都支持独立查看参数：

```sh
python3 scripts/<script>.py --help
```

常用单任务入口：

| 我想做 | 直接入口 | 说明 |
| --- | --- | --- |
| 构建样式 demo EPUB | `bash templates/epub-style-demo/build.sh` | 输出到 `templates/epub-style-demo/dist/` |
| 验证 demo 规则覆盖 | `bash scripts/validate-epub-style-demo.sh --epub <artifact.epub>` | 配合 `validate-popup-notes.sh` 使用 |
| 一键清洗真实 EPUB | `python3 scripts/epub_cleanup_pipeline.py <input.epub> --work-dir work/book-a` | 默认保留 before、after 和单文件审计报告 |
| 多轮自动收敛清洗 | `python3 scripts/epub_cleanup_loop.py <input.epub> --work-dir work/book-a` | 默认离线 rules planner，正文红线失败即回滚 |
| 合并、拆分、封面、元数据 | `python3 scripts/epub_package_tool.py --help` | 子命令见 [docs/pipeline/package-operations.md](docs/pipeline/package-operations.md) |
| 预检 EPUB 结构风险 | `python3 scripts/epub_preflight_harness.py <input.epub>` | 先判断 DRM、加密标记、包结构和候选 skill |
| 输出精排建议 | `python3 scripts/epub_refinement_harness.py <input.epub>` | 只给建议和风险，不直接改正文 |
| 输出逐图布局候选 | `python3 scripts/epub_image_layout_advisor.py <input.epub> --format md` | 只读扫描问题图，给候选菜单和决策记录模板 |
| 预览或应用风格预设 | `python3 scripts/epub_style_preset_tool.py apply <input.epub> --preset literary-cn --output <out.epub> --dry-run` | 先看 class coverage，再写 CSS、OPF 和 stylesheet link |
| 校验改前 / 改后红线 | `python3 scripts/validate_text_invariance.py before.epub after.epub --check all` | 已有 EPUB 清洗的正文安全 gate |
| 枚举或调用已登记 Python provider | `python3 scripts/epub_handbook_cli.py catalog --format json` | Python-only CLI / Agent JSON adapter；仅 allow-list capability 可执行 |
| 原生 Swift 检查 / popup 事务 | `cd swift && swift run epub-handbook-swift --help` | Swift JSON CLI；不调用 Python，GUI 也不通过该 CLI |
| 校验 AI 入口和 skills | `python3 scripts/validate_ai_entrypoints.py` / `python3 scripts/validate_skills_basic.py` | 改维护文档或 skill 后运行 |

## 三条可执行路线

### A. 实时样本 demo 实验

用当前仓库里的 demo 模板验证一条规则是否能落地：

```sh
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
```

然后把 `$EPUB` 放进 Apple Books、Kindle Previewer、Calibre 或目标阅读器看渲染。若你新增场景，同步更新 `templates/epub-style-demo/SCENE_MATRIX.md`，再按 [#实测回写闭环](#实测回写闭环) 回写结论。

### B. 自造清洗样本

先用本仓自造 before / after EPUB 演练清洗流程，确认红线 gate 和 diff review 都能工作：

```sh
bash templates/cleanup-demo-books/build.sh
python3 scripts/validate_text_invariance.py \
  templates/cleanup-demo-books/dist/city-field-notes-before.epub \
  templates/cleanup-demo-books/dist/city-field-notes-after-clean.epub \
  --check all
```

再按 [docs/pipeline/epub-diff-review.md](docs/pipeline/epub-diff-review.md) 对 before / after 做五层 review。`redline-trap` 是故意失败的反例，用来确认正文改写会被挡住。

`templates/cleanup-demo-books/dist/` 是本地生成目录，不随仓库提交；下载后运行上面的 `build.sh` 即可生成 EPUB 和 manifest 作为查看、验证和 diff 参考。

### C. 真实 EPUB 清洗

单书默认入口是一条命令：

```sh
python3 scripts/epub_cleanup_pipeline.py \
  /path/to/input.epub \
  --work-dir work/book-a
```

它生成 before 备份、`after/cleaned.epub` 和默认单文件 `reports/pipeline.json` 审计汇总。排障时可加 `--keep-step-reports` 恢复完整分步报告。结构规范化需要先 dry-run 再显式批准；人工 diff review 和阅读器实测仍然保留。完整说明见 [docs/pipeline/oneclick-epub3-converter.md](docs/pipeline/oneclick-epub3-converter.md)。

对用户给的 EPUB 只走复制件，不改原文件。需要逐步执行 preflight、结构规范化、EPUB3 迁移、精排建议、红线 gate 和人工 review 时，按 [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md) 的完整流程操作。

## 自动循环清洗（确定性多轮，AI 可选）

`scripts/epub_cleanup_loop.py` 在上面单次 pipeline 的纪律之上，加一段**确定性多轮自动收敛**：先复用单次入口建立干净 EPUB3 基线，再每轮由 planner 产出受白名单约束的改写计划，脚本执行后立即跑正文红线 gate，红则回滚；收敛或触上限自动停机，最后产出 `after/cleaned.epub` 和「已自动改 / 建议你改 / 需人工」三分类报告。AI 全程**不碰正文**，只产受白名单约束的 JSON。

```sh
# 默认 rules：零模型、纯标准库、可离线 / 气隙运行，正文一字不改
python3 scripts/epub_cleanup_loop.py /path/to/input.epub --work-dir work/book-a

# 机读三分类报告
python3 scripts/epub_cleanup_loop.py /path/to/input.epub --work-dir work/book-a --format json
```

需要 AI 辅助判断时用**本地模型**走文件握手，稿件不出本机：

```sh
# 1) 先跑一轮：工具在 reports/round-1.plan-request.json 写出请求并暂停
python3 scripts/epub_cleanup_loop.py /path/to/input.epub --work-dir work/book-a --planner handshake
# 2) 让本地 AI host 按请求填出 reports/round-1.plan.json（仅白名单 op）
# 3) 重跑同一命令：工具读回并执行，逐轮推进到收敛
python3 scripts/epub_cleanup_loop.py /path/to/input.epub --work-dir work/book-a --planner handshake
```

默认 `rules` 全程不联网；AI 每轮所见与所提全部落盘在 `work/book-a/reports/` 可审计。与单次 `epub_cleanup_pipeline.py` 的关系、模型与隐私说明见 [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md) §18。


## 精排 harness

| harness | 作用 | 输出 |
| --- | --- | --- |
| `scripts/epub_preflight_harness.py` | 先检查 EPUB 格式 / package 是否可处理 | `preflight_status`、结构 findings、候选 skill |
| `scripts/epub_structure_tool.py` | 可选：先格式化目录，再按 OPF manifest id 做文件名反混淆 | 两阶段 `mappings`、`warnings`、normalized EPUB |
| `scripts/epub3_migration_harness.py` | dry-run 或写出保守 EPUB3 迁移包 | OPF/nav actions、warnings、`written_output` |
| `scripts/epub_refinement_harness.py` | 给现成 EPUB 出精排建议 | 弹注、字体、图片、Ruby/竖排、红线/diff、AI skill 阶段建议 |
| `scripts/epub_image_layout_advisor.py` | 给问题图片生成逐图候选菜单 | 文件、selector、图片路径、候选布局、可追溯风险与决策命令模板 |
| `scripts/epub_style_preset_tool.py` | 预览或应用三种中文书型风格预设 | coverage、CSS add/replace 清单、OPF/link 写入结果 |
| `scripts/epub_css_cleanup.py` | 可选合并重复 CSS、替换旧字体链并收敛互不交叠的局部样式 | 清洗后 EPUB、CSS/字体计数 |
| `scripts/epub_anthology_refinement.py` | 可选把合订书单图卷封改为 A-lite contain + fallback，并优化紧邻版权页 | 精排后 EPUB、卷封/版权页计数 |

这些入口是给“已有 EPUB 精致排版工具”准备的，不替代人工确认。尤其是弹注正文保留、多字体策略、图片有损压缩质量和阅读器效果，仍需要 AI dry-run + 人工 review + reader-matrix 实测。

## 风格预设

`templates/style-presets/` 提供 `literary-cn`、`classical-annotated-cn` 和
`academic-cn` 三种菜单。工具默认先 dry-run，并报告书内 class 被预设 selector
覆盖的比例；低于 30% 时应先走 cleanup pipeline，避免样式静默无效。

```sh
python3 scripts/epub_style_preset_tool.py list
python3 scripts/epub_style_preset_tool.py apply input.epub \
  --preset literary-cn \
  --output styled.epub \
  --dry-run
```

预设只改 CSS、OPF stylesheet 声明和 XHTML `<head>` link，不改正文。结构规则已
自动验证，视觉效果仍需按 `reader-matrix` 闭环实测。完整说明见
[templates/style-presets/README.md](templates/style-presets/README.md)。

## EPUB diff review

完整操作说明已经下沉到 [docs/pipeline/epub-diff-review.md](docs/pipeline/epub-diff-review.md)。先跑文本红线，再用 Calibre Editor 或 VS Code + `unzip` 人工检查结构、文本、样式、资源和元数据五层差异。

## 已有 EPUB 清洗

主流程见 [docs/pipeline/cleanup-flow.md](docs/pipeline/cleanup-flow.md)。要点：

- **红线先跑**：`scripts/validate_text_invariance.py before.epub after.epub --check all` 退出码必须为 0。
- **harness 扫描**：`python3 scripts/epub_ai_harness.py --mode cleanup input.epub` 给出 findings 与推荐 skill 顺序。
- **分派 skill**：按 findings 依次跑专项 skill；每步保留中间 epub 作回滚锚点。
- **人工 review**：按 [docs/pipeline/epub-diff-review.md](docs/pipeline/epub-diff-review.md) 做五层检查。
- **reader-matrix 回写**：实测有变化时按 [CONTRIBUTING.md](CONTRIBUTING.md) 把结果写回 `docs/final/reader-matrix.yaml`。

## AI Skills

[skills/](skills/) 下每个目录是一个可读契约：判断 / 修复 / 验证三段式 `SKILL.md`。

主入口：

- `epub-layout-auditor` — 总审稿、风险分级、分派专项修复。
- `epub-source-intake` — 从 txt / md / PDF / OCR 等源材料起步。

全部 15 个 skill（2 个主入口 + 13 个专项）见 [docs/learn/04-skills.md](docs/learn/04-skills.md) 反向查表。

无 AI 也可用：`SKILL.md` 本身就是 Markdown 步骤说明，人工跟着走即可。

## 实测回写闭环

任何阅读器 / 打包兼容性判断都不允许只靠手册推断，必须：

```text
1. demo 复现（templates/epub-style-demo/ 加最小场景）
   ↓
2. 构建（templates/epub-style-demo/build.sh）
   ↓
3. 阅读器实测（Kindle Previewer / Apple Books / 多看 / KOReader ...）
   ↓
4. 回写（docs/final/reader-matrix.yaml: pass | warn | fail | na）
   ↓
5. 固化规则（docs/final/SPEC-实现约束.md）
   ↓
6. 同步（终极手册、速查表、相关 skills）
```

详见 [AGENTS.md](AGENTS.md) 的「阅读器实测闭环」段。

## 这个仓库不是什么

- 不是零基础排版速成课：会教你做不崩的 EPUB，但不覆盖通用网页 CSS 基础。完全没接触过的人请先读 [docs/learn/00-what-is-epub.md](docs/learn/00-what-is-epub.md)。
- 不是封闭格式（mobi / AZW3）的制作工具。
- 不是 epub.js 阅读器。
- 不是 Kindle 自费出版的运营指南。
- 不是阅读器渲染验证工具 — 本仓 diff 只比文件，不模拟渲染；渲染效果靠 reader-matrix 实测。

## 文档地图

| 层 | 路径 | 角色 |
| --- | --- | --- |
| 入门 | [docs/learn/](docs/learn/) | 第一次接触本仓的人 |
| 工程契约 | [docs/final/](docs/final/) | SPEC、终极手册、速查表、reader-matrix；对外硬约束 |
| 场景指南 | [docs/how-to/](docs/how-to/) | 英文小说 / 文白对照 / 章首图 / 弹注 fallback 等 |
| 批处理流水线 | [docs/pipeline/](docs/pipeline/) | 拿到一本现成 EPUB 后的流程 |
| 治理 | [docs/meta/](docs/meta/) | 架构分工、各桶入口索引、仓库维护说明 |
| 推导 / 实验 | [docs/source/](docs/source/), [docs/experiments/](docs/experiments/) | 早期推导、实测复盘 |

完整索引见 [docs/README.md](docs/README.md)。

## 协作 / 贡献

阅读 [AGENTS.md](AGENTS.md) 了解 AI 协作约定。`CLAUDE.md` 仅作为 Claude Code 兼容入口。本仓所有约束变更都走 demo → reader-matrix → SPEC → 手册 → 速查表 → skills 的实测闭环。

贡献流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可

代码部分 MIT；文档与样本许可参见 [THIRD_PARTY.md](THIRD_PARTY.md)。
