# AGENTS.md

本文件是本仓库给 AI 协作代理使用的通用主入口，也是 AI 工作约束的唯一维护源。
Codex、Claude Code 以及其他代理开始工作前都必须先读取本文件；平台专用入口只做跳转，不复制规则。

## 启动读取顺序

1. 先阅读本文件，判断任务属于「已有 EPUB 清洗」「源材料接入」「阅读器兼容性实测」「实现约束变更」还是「说明增强」。
2. 已有 EPUB 清洗：继续阅读 `docs/final/SPEC-实现约束.md` §10、`docs/pipeline/cleanup-flow.md` 和 `docs/pipeline/refinement-harnesses.md`。
3. 源材料接入：继续阅读 `skills/epub-source-intake/SKILL.md`，先建立可审计的 source bundle。
4. 阅读器兼容性实测：继续阅读 `templates/epub-style-demo/README.md`、`templates/epub-style-demo/SCENE_MATRIX.md` 和 `docs/final/reader-matrix.yaml`。
5. 只有在任务需要时才读取对应的 `skills/*/SKILL.md`；技能索引和推荐顺序见 `skills/README.md`。
6. 若模型或客户端不会自动发现本文件，提示词必须显式要求先读取根目录 `AGENTS.md`。

## 规范来源优先级

1. `templates/`：可运行样式样本和机器消费源。遇到阅读器显示、打包、转换兼容性问题时，先补 demo fixture 并实测。
2. `docs/final/`：对外约束层。任何改动都视为规范变更，必须由 demo fixture 或明确实测结果支撑。
3. `skills/*/SKILL.md`：自动化行为契约，修改需保持向后兼容。
4. `docs/guides/`：场景化实操指南，不承载计划、流水线或下游架构。
5. `docs/getting-started/`：入门教程。与 `docs/final/` 冲突时，以 `docs/final/` 为准。
6. `docs/pipeline/`：批处理流水线工作流，与 `docs/guides/` 同级。
7. `docs/plans/`：计划、review 和维护说明，不直接驱动行为。
8. `docs/architecture/`：下游或周边架构副本，不属于对外硬约束。
9. `docs/source/`、`docs/experiments/`：推导与实验区，可补充但不应反向覆盖约束层。
10. `templates/cleanup-demo-books/`：自造清洗和 diff 演示样本；生成的 `.epub` 不入 git。
11. 第三方来源记录写入 `THIRD_PARTY.md` 与 `references/`；实体 `.epub` 只在有明确保留理由和许可记录时入 git。

`tools/` 已于 2026-05-28 移除。人工 diff review 使用 Calibre Editor 或 VS Code，见 `docs/pipeline/epub-diff-review.md`。

## 已有 EPUB 固定流程

已有 EPUB 默认遵循「保留 before → preflight → 必要时格式化和文件名反混淆 → EPUB3 迁移 → 精排建议 → 红线校验 → 人工 diff review」：

1. 复制输入文件，保留不可修改的 before 基线。禁止在唯一原件上直接修改。
2. 运行 `python3 scripts/epub_preflight_harness.py <input.epub>`，先判断 DRM、加密标记、文件损坏和结构风险。
3. 如果资源目录混乱、文件名明显混淆或需要稳定 diff，先 dry-run：

   ```sh
   python3 scripts/epub_structure_tool.py normalize input.epub \
     --output normalized.epub \
     --dry-run \
     --report-format json
   ```

4. 人工确认 dry-run 报告中的两个阶段：先格式化资源目录，再按 OPF manifest id 做文件名反混淆。确认后移除 `--dry-run` 写出 normalized EPUB，并保存 JSON 报告。
5. 将 normalized EPUB 作为后续输入。运行迁移 harness、精排 harness 和相关专项 skill。
6. 运行 `python3 scripts/validate_text_invariance.py before.epub after.epub --path-map <normalize-report.json>`，再用 Calibre Editor 或 VS Code 做人工 diff review。
7. 把值得复用的人工判断写入 `records/typeset-decisions.jsonl`；只属于当前书的判断写入 `work/<book>/reports/decisions.json`，两者都禁止保存正文文本。
8. 留下输入、输出、preflight、迁移结果或跳过理由、结构规范化报告、精排建议、红线结果、diff review 结论、阅读器实测或跳过理由、需要回写的文档和 skill 清单。

边界：

- `scripts/epub_structure_tool.py` 只使用 Python 标准库，不提供 DRM 解密。
- 文件名反混淆只处理 EPUB 内部资源路径，依据 OPF manifest id 生成可读文件名，并同步更新引用。
- 默认遇到加密标记即停止。声明目标在 ZIP 中不存在时，工具可移除 stale encryption 引用；只有工具明确识别为标准字体 obfuscation 且任务得到明确授权时，才可按工具说明单独处理。真实存在的未知加密资源不得猜测或绕过。

## 阅读器实测闭环

阅读器问题默认遵循「demo 先行，文档后补」：

1. 在 `templates/epub-style-demo/` 添加或修改最小但真实的 demo 场景，不直接改手册定论。
2. 运行模板 build 脚本，在 `templates/epub-style-demo/dist/` 生成 EPUB。
3. 用目标阅读器或转换器验证，保留错误码、截图、日志摘要、阅读器名称和版本。
4. 立即更新 `docs/final/reader-matrix.yaml`，标记 `pass | warn | fail | na`，写明 artifact、现象、处理动作和待复测项。
5. 只有 demo 和 reader-matrix 支撑结论后，才将规则写入 `docs/final/SPEC-实现约束.md`。
6. SPEC 变更后，同步检查终极手册、速查表和相关 skills，避免分叉。

如果实测结果与手册冲突，以实测 demo 和 reader-matrix 为准，手册必须被修正。

## 关键约束

- 修改 `docs/final/` 硬规则时，同步检查：
  - `docs/final/EPUB 3 终极实践手册.md`
  - `docs/final/EPUB 3 HTML CSS 属性速查表.md`
  - `docs/final/SPEC-实现约束.md`
- 涉及弹注、字体、A-lite、竖排等规范条目时，优先写入 `SPEC-实现约束.md`，再在手册解释。
- `skills/*/SKILL.md` frontmatter 只保留 `name` 和 `description`；字段名不要随意删改。
- `skills/*/agents/openai.yaml` 只使用扁平字符串 metadata，并与对应 `SKILL.md` 用途一致。
- `templates/` 样本应能独立打包，生成产物放在模板自己的 `dist/`。
- Kindle、Apple Books、Thorium、KOReader 等阅读器兼容性问题，不允许只靠手册推断修改。
- demo EPUB 必须覆盖普通正文、中英混排、大字号标题、图片或封面、表格、代码、标准弹注、legacy fallback、A-lite、竖排和字体链。
- 图文环绕主路径使用 `figure.img-left/right`。`float` 和百分比 `width` 放在 `<figure>`，内部 `<img>` 使用 `width:100%; height:auto`。
- `.wavy` 等带样式下划线必须先写基础 `text-decoration: underline;`，再写 `text-decoration-style`。
- 含 MathML 的 XHTML 必须在 OPF manifest 声明 `properties="mathml"`。
- 修改弹注结构后必须运行 `scripts/validate-popup-notes.sh`；构建后优先用 `--epub <artifact>` 复核产物。
- 任何阅读器实测规则必须能追溯到 demo、artifact、阅读器名称和版本、现象与结论。信息不完整时只能记录为待验证假设。
- 新增第三方 EPUB 参考样本时，必须同步更新 `THIRD_PARTY.md`，写清来源、作者、许可和链接。

## 最小验证矩阵

按改动类型运行足够的验证，不要只依赖人工阅读：

| 改动类型 | 至少运行 |
| --- | --- |
| 任意改动 | `git diff --check` |
| AI 入口或维护文档 | `python3 scripts/validate_ai_entrypoints.py` |
| skills | `python3 scripts/validate_skills_basic.py` |
| Python 脚本 | 对应的 `scripts/test_*.py`；必要时 `python3 -m py_compile <files>` |
| 已有 EPUB 清洗 | preflight、结构规范化 dry-run 或跳过理由、`validate_text_invariance.py`、人工 diff review |
| demo、validator 或 `docs/final/` | build demo、`scripts/validate-epub-style-demo.sh --epub <artifact>`、`scripts/validate-popup-notes.sh --epub <artifact>` |
| OPF、nav、NCX | 额外运行 `xmllint --noout ...`；本机没有 `xmllint` 时记录跳过理由 |
| 任意 EPUB 产物 | `python3 scripts/epub_lint.py <artifact>`，error 必须清零或逐条给出豁免理由 |
| demo / starter 构建产物 | 本地跑 demo validator、popup note validator 与 `scripts/epub_lint.py`；EPUBCheck 只在 GitHub Actions 作为 CI gate 运行 |

可选安装 hook 模板：

```sh
scripts/install-hooks.sh
```

## 文档落点

- 对外硬约束写入 `docs/final/`。
- 某类书的实操方式写入 `docs/guides/`。
- 已有 EPUB 的流程、工具和模式写入 `docs/pipeline/`。
- 计划、review 和维护说明写入 `docs/plans/`，不直接驱动行为。
- 推导和实验记录写入 `docs/source/` 或 `docs/experiments/`。
- 排版决策记录写入 `records/`；仓库级文件只保存脱敏、可复用的机器可读判断。
- 新增第三方来源说明写入 `THIRD_PARTY.md`。

历史计划和实验快照只在任务明确要求时修改。不要为了统一措辞重写历史记录。
