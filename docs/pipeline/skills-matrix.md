# Skills × 流程步骤映射表

> 14 个现有 skill 在「清洗流水线」与「新书制作」中的角色。
>
> `scripts/epub_ai_harness.py --mode cleanup` 输出的 `recommended_skills` 是候选清单，不是自动执行顺序。实际清洗顺序必须由操作者按 findings、[cleanup-patterns.md](cleanup-patterns.md) 和 SPEC §10 决定。
>
> 各 SKILL.md 中的 Dry-run / `--commit` 约定是 AI 行为契约：调用方应先给出预期改动和红线风险，再经确认执行写盘。本仓没有提供统一的 `<skill-invocation>` CLI，也不会自动强制 `--commit` flag。

## 总表

| Skill | 清洗 | 新书 | 用在哪一步 | 类型 |
| --- | --- | --- | --- | --- |
| `epub-layout-auditor` | yes | yes | 清洗 §2 分派；新书 review 前 | 审稿 |
| `epub-source-intake` | no | yes | 新书：txt/md/PDF/OCR -> source | 接入 |
| `epub-css-layering-optimizer` | yes | yes | 清洗 §4 黄线；新书 finalize | 清洗 / 制作 |
| `epub-popup-footnote-converter` | yes | yes | 清洗 §4 黄线；新书弹注 | 清洗 / 制作 |
| `epub-legacy-footnote-fallback` | yes | yes | 清洗 §4；新书做多看兼容 | 清洗 / 制作 |
| `epub-typography-optimizer` | yes | yes | 清洗 §4；新书排版细化 | 清洗 / 制作 |
| `epub-english-typography-optimizer` | yes | yes | 清洗 §4（双语 epub）；新书英文体 | 清洗 / 制作 |
| `epub-image-layout-optimizer` | yes | yes | 清洗 §4；新书图文 | 清洗 / 制作 |
| `epub-vertical-ruby-optimizer` | yes | yes | 清洗 §4（古籍 / 日文）；新书竖排 | 清洗 / 制作 |
| `epub-literary-structure-formatter` | yes | yes | 清洗 §4；新书文白 / 章首 | 清洗 / 制作 |
| `epub-kindle-compatibility-checker` | yes | yes | 清洗 §4；新书 Kindle 专项 | 清洗 / 制作 |
| `epub-alite-converter` | maybe | yes | 清洗按场景；新书 A-lite | 制作 |
| `epub-package-nav-auditor` | yes | yes | 清洗 §4；新书 OPF/nav 校验 | 清洗 / 制作 |
| `epub-style-demo-maintainer` | no | no | 本仓 fixture 维护 | 仓库内部 |

## 清洗流水线中 skill 的典型顺序

以旧版 EPUB 2 升级为例：

1. `epub-layout-auditor`
2. `epub-package-nav-auditor`
3. `epub-css-layering-optimizer`
4. `epub-popup-footnote-converter`
5. `epub-typography-optimizer`
6. `epub-legacy-footnote-fallback`（可选）
7. `epub-kindle-compatibility-checker`

每个 skill 执行前先做 dry-run 审查；每次实际写盘后立刻跑 `validate_text_invariance.py`。
