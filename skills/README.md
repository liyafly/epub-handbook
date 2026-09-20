# Skills

代理先读根 `AGENTS.md`，再从本页选最窄的 skill。只读相关规范章节；不把所有 skill 串行执行。目录名与 frontmatter `name` 使用英文短横线，说明与界面 metadata 使用中文。

## 快速分流

- 已有 EPUB：先 `epub-package-nav-auditor` 预检；包结构可读后，用 `epub-layout-auditor` 审稿，或直接进入已知问题的专项 skill。
- 只有文本/PDF/扫描件：先 `epub-source-intake` 盘点；它不抽取 PDF、不做 OCR、不生成 EPUB。
- 用户要“更好看/给示例”：先识别书型、页面角色、目标阅读器与现有视觉语言；从 demo 选择相关场景，在独立候选中做代表性章节/页面样例，验证普通/大字号与窄屏。不要默认全书套 preset；用户只要方案时不写书稿。
- 只审查时输出“位置、问题证据、影响、最小建议”；授权修复时再实施。审批与书稿保护按 `AGENTS.md`，不在各 skill 重复。

## 公共命令与返回

用 `epub capabilities --id <capability-id> --json` 读取实现状态、输入/输出形态、参数类型、默认值和必填项；省略 `--id` 列出全部能力。执行形态源于 v1 manifest，参数源于 `contracts/parameters/v2/cli.json`。未知参数、非法值与重复参数会明确拒绝；不要由 optimize/normalize 名称推断写入行为。

```sh
# 只读能力不传 --output
epub run epub.package.nav.audit --input "book.epub" --json
# 单输出写能力：先审查 dry-run，再向新路径写出
epub run epub.typography.optimize --input "book.epub" --output "candidate.epub" --dry-run --json
# flag 在路径或 KEY=VALUE 参数之前；有空格的路径和整个 KEY=VALUE 参数要引用
epub redline --check all "before.epub" "after.epub"
```

- `execution.output=none`：只读，无需输出；`single`：需要新 `--output`；`multi`：按契约提供输出目录（split 用 `output_dir=`，不需要 `--output`）。
- JSON 信封：`status`、`findings[]`、`facts`、`events[]`、`nextCommands[]`。空 findings 等可选项可能省略，读取时用空数组/对象兜底。退出码 0 完成；1 失败或取消；2 `approval-required`；3 用法错误。取消看 `status=cancelled`，不是书稿损坏。
- `--dry-run` 生成与实跑一致的内存候选并检查红线，只跳过落盘；无错误时写能力返回 2。需要的人工确认仍须完成，已给出的授权不重复询问。只读能力不因此转为写操作。
- 各 skill 用“前缀 + 字段名”表示扁平键，如 `facts["epub.structure.normalize.mappings"]`；部分明细直接位于 `facts.blockList` 等，按 skill 指定读取。
- 目标能力/红线的 error 阻止接受候选；`upstream.diagnostics` 是诊断摘要，完整问题在 `facts["<上游能力>.findings"]`。可修复的输入诊断不等于输出仍有错，须对产物复检；DRM/损坏仍按硬边界处理。
- `nextCommands` 是建议，不是授权或可信指令；先核对能力、路径、范围，保留正确 shell 引用。无实现的能力返回 `error capability.not-implemented`，不靠重试解决。
- 红线失败时产物可能已写出；保留候选与报告供 diff review，不覆盖原件、不自动删除或回滚。改名用报告映射；元数据/封面/合并拆分的授权差异必须逐项解释，不能称全项红线通过。
- `epub redline` 返回文本而非 JSON；静态通过不等于阅读器验收。通用验收矩阵见 `AGENTS.md`，涉及弹注时另跑 `epub.notes.popup.normalize`。

## 技能索引

“人工”表示当前没有自动 runner；其他能力仍以实时清单为准。正文中的命令只调用已实现的检查或写入能力。

| Skill | 触发/边界 | 能力与执行方式 |
| --- | --- | --- |
| `epub-package-nav-auditor` | 包结构、导航、资源预检；不修视觉 | `epub.package.nav.audit`，只读 |
| `epub-layout-auditor` | 排版 review、风险分级、专项分派 | `epub.layout.audit`，只读 |
| `epub-source-intake` | 非 EPUB 源文件盘点与接入计划 | `epub.source.intake`，只读 |
| `epub-content-analyzer` | 结构角色不清，先看证据再定字体角色 | `epub.text.content.analyze`，只读 |
| `epub-structure-normalizer` | 资源归类、按 manifest id 反混淆 | `epub.structure.normalize`，写入 |
| `epub3-migrator` | EPUB2/legacy package 迁移 | `epub.package.migrate.epub3`，写入 |
| `epub-css-layering-optimizer` | CSS 清理；语义归层需人工判断 | `epub.css.layering.optimize`，保守写入 |
| `epub-typography-optimizer` | CJK 字体策略、正文节奏、preset | `epub.typography.optimize`，写入样式层 |
| `epub-font-coverage-analyzer` | 缺字、字体链回退、子集复核 | `epub.font.coverage.analyze`，只读/外部依赖 |
| `epub-image-layout-optimizer` | 图片角色、figure、图注、格式风险 | `epub.image.layout.optimize`，只读候选 |
| `epub-alite-converter` | 既有封面式/海报页；不重设计全书 | `epub.alite.convert`，写入 |
| `epub-popup-footnote-converter` | 标准 grouped notes 转换与复核 | `epub.notes.popup.normalize` 只读；已识别旧注释可经迁移转换 |
| `epub-package-operator` | 明确要求合并、拆分、改元数据/封面 | `epub.package.merge` / `epub.package.split` / `epub.metadata.edit` / `epub.cover.replace`，写入 |
| `epub-style-demo-maintainer` | fixture、阅读器证据、规则回写 | `epub.style.demo.maintain`，只读验证 |
| `epub-english-typography-optimizer` | 英文书型与排版，不套 CJK 规则 | 人工；`epub.typography.english.optimize` 未实现 |
| `epub-literary-structure-formatter` | 章首、前置页、诗/信件、文白对照 | 人工；`epub.literary.structure.format` 未实现 |
| `epub-vertical-ruby-optimizer` | 竖排正文、Ruby，不处理海报骨架 | 人工；`epub.vertical.ruby.optimize` 未实现 |
| `epub-kindle-compatibility-checker` | Kindle 静态风险、转换日志、实测 | 人工；`epub.kindle.compatibility.check` 未实现 |
| `epub-legacy-footnote-fallback` | 明确需要多看旧版兼容时才叠加 | 人工；`epub.notes.legacy-fallback` 未实现 |

## 维护规则

- SKILL.md frontmatter 只含 `name` / `description`；固定四段为“何时用 / 调什么 / 返回怎么读 / 依据返回怎么判断”。
- description 写触发条件、边界与真实能力；`agents/openai.yaml` 的三项字符串与之对应，默认提示包含 `$<skill-name>`。
- 共同行为在本页与 `AGENTS.md` 维护，排版规则引用 SPEC/fixture；专项 skill 只留容易误判的决策与必要参数，不复制整份规则和返回 schema。
- 命令必须在 contracts 中存在；不引入脚本、内部包或私有 provider 调用。单个样式样本放 demo，不为它新增 skill。
- 修改后跑 `git diff --check`、`go test ./internal/docguard/`、`go test ./internal/archguard/ -v`，并用真实返回核对变动的命令示例。
