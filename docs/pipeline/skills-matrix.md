# Skills × 流程步骤映射表

> 当前 skills 在「清洗流水线」与「新书制作」中的角色。
>
> `epub run epub.layout.audit` 返回的 findings 与 `nextCommands` 是候选清单，不是自动执行顺序。实际清洗顺序必须由操作者按 findings、[cleanup-patterns.md](cleanup-patterns.md) 和 SPEC §10 决定。
>
> 各 SKILL.md 中的 Dry-run / 写出约定是 AI 行为契约：调用方应先给出预期改动和红线风险，再经确认调用 `epub run <capability-id>`。

## 总表

| Skill | 清洗 | 新书 | 用在哪一步 | 类型 |
| --- | --- | --- | --- | --- |
| `epub-layout-auditor` | yes | yes | 清洗 §2 分派；新书 review 前 | 审稿 |
| `epub-content-analyzer` | yes | yes | 清洗 §3 文本角色建议；新书结构复核 | 只读分析 |
| `epub-source-intake` | no | yes | 新书：txt/md/PDF/OCR -> source | 接入 |
| `epub-structure-normalizer` | maybe | no | 清洗 §1.5：先格式化，再文件名反混淆 | 结构清洗 |
| `epub3-migrator` | yes | no | preflight 后建立 EPUB3 基线 | 迁移 |
| `epub-css-layering-optimizer` | yes | yes | 清洗 §4 黄线；新书 finalize | 清洗 / 制作 |
| `epub-popup-footnote-converter` | yes | yes | 清洗 §4 黄线；新书弹注 | 清洗 / 制作 |
| `epub-legacy-footnote-fallback` | yes | yes | 清洗 §4；新书做多看兼容 | 清洗 / 制作 |
| `epub-typography-optimizer` | yes | yes | 清洗 §4；新书排版细化 | 清洗 / 制作 |
| `epub-font-coverage-analyzer` | yes | yes | 字体策略前后检查 cmap、缺字与回退 | 只读分析 |
| `epub-english-typography-optimizer` | yes | yes | 清洗 §4（双语 epub）；新书英文体 | 清洗 / 制作 |
| `epub-image-layout-optimizer` | yes | yes | 清洗 §4；新书图文 | 清洗 / 制作 |
| `epub-vertical-ruby-optimizer` | yes | yes | 清洗 §4（古籍 / 日文）；新书竖排 | 清洗 / 制作 |
| `epub-literary-structure-formatter` | yes | yes | 清洗 §4；新书文白 / 章首 | 清洗 / 制作 |
| `epub-kindle-compatibility-checker` | yes | yes | 清洗 §4；新书 Kindle 专项 | 清洗 / 制作 |
| `epub-alite-converter` | maybe | yes | 清洗按场景；新书 A-lite | 制作 |
| `epub-package-nav-auditor` | yes | yes | 清洗 §4；新书 OPF/nav 校验 | 清洗 / 制作 |
| `epub-package-operator` | maybe | yes | 明确选择合并、拆分、元数据或封面操作时 | 写入操作 |
| `epub-style-demo-maintainer` | no | no | 本仓 fixture 维护 | 仓库内部 |

## 清洗流水线中 skill 的典型顺序

以旧版 EPUB 2 升级为例：

1. `epub-structure-normalizer`（目录散乱或文件名不可读时）
2. `epub-layout-auditor`
3. `epub-package-nav-auditor`
4. `epub-css-layering-optimizer`
5. `epub-popup-footnote-converter`
6. `epub-typography-optimizer`
7. `epub-legacy-footnote-fallback`（可选）
8. `epub-kindle-compatibility-checker`

每个 skill 执行前先做 dry-run 审查；每次实际写盘后立刻跑 `epub redline --check all`。
