# AGENTS.md

本文件是本仓库给 AI 协作代理使用的通用主入口，也是 AI 工作约束的唯一维护源。
Codex、Claude Code 以及其他代理开始工作前都必须先读取本文件；平台专用入口只做跳转，不复制规则。

普通人或只想做、修一本书的用户请看根 `README.md` 与 `docs/learn/`，无需阅读本文件；本文件面向 AI 代理与专业维护者。

## 启动读取顺序

1. 先阅读本文件，判断任务属于「已有 EPUB 清洗」「源材料接入」「阅读器兼容性实测」「实现约束变更」还是「说明增强」。
2. 已有 EPUB 清洗：继续阅读 `docs/final/SPEC-实现约束.md` §10、`docs/pipeline/cleanup-flow.md` 和 `docs/pipeline/refinement-harnesses.md`。
3. 源材料接入：继续阅读 `skills/epub-source-intake/SKILL.md`，先建立可审计的 source bundle。
4. 阅读器兼容性实测：继续阅读 `templates/epub-style-demo/README.md`、`templates/epub-style-demo/SCENE_MATRIX.md` 和 `docs/final/reader-matrix.yaml`。
5. Go CLI 架构、迁移或删除旧实现：继续阅读 `docs/pipeline/go-cli-rearchitecture.md`，严格按阶段和 parity gate 执行。
6. 只有在任务需要时才读取对应的 `skills/*/SKILL.md`；技能索引和推荐顺序见 `skills/README.md`。
7. 若模型或客户端不会自动发现本文件，提示词必须显式要求先读取根目录 `AGENTS.md`。

## 架构分工

目标架构是 **Go 模块化单体 CLI + 私有字体 provider**。详细目录、契约、阶段、验收和旧代码删除
顺序见 `docs/pipeline/go-cli-rearchitecture.md`。Go 实现尚未完成前，Python/Swift 只作为迁移基线存在：

| 层 | 目标 / 当前状态 | 职责 |
|---|---|---|
| 公开 CLI / agent runtime | **Go**（待按迁移蓝图实现） | 唯一公开命令、capability registry、事务、流水线与多平台发行 |
| 迁移 oracle | **Python**（`scripts/`） | 当前 CLI 与验证基线；逐 capability 达到 Go parity 后收缩，不一次性删除 |
| 字体工具 | **独立 provider**（初始为 Python + FontTools） | 覆盖、子集化和复杂字体处理；随发行包交付，用户不需要安装 Python/`uv` |
| CSS | **Go 规则层 + 纯 Go parser adapter** | lossless token/patch；复杂字体层叠暂由字体 provider，禁止用正则解析复杂 CSS |
| Swift / GUI | **FROZEN，待删除**（`swift/` + `gui/`） | 不再增加 capability；只有达到蓝图的 parity/release gate 后才删除，移动端不在范围内 |
| 机器契约 | `contracts/` + `adapters/` | provider-neutral capability、request/result 与 agent 适配表面 |
| 规范/证据 | `docs/final/` + `templates/` + `reader-matrix.yaml` | policy/evidence 唯一来源 |

禁止向 Swift/GUI 增加新功能，也禁止在 Go parity gate 前批量删除它们。最终 skill 只能调用统一
`epub-handbook` CLI，不得依赖 Go internal package、旧 Python 路径或私有 provider 路径。
在迁移蓝图 Phase 7 完成前，本文件下方已有 EPUB 流程中的 Python 命令仍是当前可执行入口。

## 规范来源优先级（三档）

**第一档 — 硬约束（违反即事故）：**
`templates/` → `docs/final/` + `reader-matrix.yaml` → `skills/*/SKILL.md`
实测 demo 优先于文档推断。遇到冲突以 demo fixture 与 reader-matrix 为准。

**第二档 — 指南（提供方法，不设硬规则）：**
`docs/how-to/`、`docs/learn/`、`docs/pipeline/`
与 `docs/final/` 冲突时以 `docs/final/` 为准。

**第三档 — 参考（不直接驱动行为）：**
`archive/` 与 git 历史。已完成的设计、实施计划、实验和早期推导只作背景补充，
不应反向覆盖约束层。

第三方来源记录写入 `THIRD_PARTY.md` 与 `references/`；实体 `.epub` 只在有明确保留理由和许可记录时入 git。`tools/` 已于 2026-05-28 移除，人工 diff review 使用 Calibre Editor 或 VS Code。

## 已有 EPUB 固定流程

已有 EPUB 默认遵循「保留 before → preflight → 必要时格式化和文件名反混淆 → EPUB3 迁移 → 精排建议 → 红线校验 → 人工 diff review」：

书级项目默认位于 `work-epub/<book>/`，每本书是独立本地 Git 仓库，目录固定为
`01 源文件/`、`02 校对材料/`和 `03 制作工作区/`。详细边界见
`docs/pipeline/book-workspace.md`。手册主仓库忽略 `work-epub/`，禁止把书级 Git 误加为 submodule。

1. 把入选底本保留在 `01 源文件/`，记录 SHA-256；编辑只发生在 `03 制作工作区/epub/` 或新候选 EPUB。禁止在唯一原件上直接修改。
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
7. 把值得跨书复用的人工判断写入 `records/typeset-decisions.jsonl`；只属于当前书的排版结论默认汇总到书根的 `制作说明.md`。只有工具需要机器可读输入时，才在 `02 校对材料/` 按需保留书级决策 artifact。授权正文校订的含文决策必须放在 `02 校对材料/正文校订/`，不得混入仓库级 `records/`。
8. preflight、dry-run、lint 和中间 JSON 放入 `03 制作工作区/.pipeline/` 并默认忽略；在 `制作说明.md` 持久记录输入/输出 SHA、迁移或跳过理由、红线结果、diff review、阅读器实测与需回写项。正在被 gate 引用的 path map 或校订决策不得提前删除。

用户明确授权校订正文时，正文不变 gate 不得被删除、伪造为通过或用宽泛 allow-list 掩盖；应切换到 `docs/final/SPEC-实现约束.md` §10.1.1 与 `docs/pipeline/cleanup-flow.md` §7.1 的授权正文校订分支。该分支必须冻结现版与参考版、记录篇章映射和 SHA、逐项导出结构化审阅决策、拒绝待查/缺失手工文本，并在新候选 EPUB 上继续执行 metadata、spine、锚点、封面、DRM、非文字 DOM / 属性、注释和图片红线；篇名与 nav / NCX 标签同步须另列授权。

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
| 文档、skills、模板或字体规则同步 | `python3 scripts/validate_docs_consistency.py` |
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
- 某类书的实操方式写入 `docs/how-to/`。
- 已有 EPUB 的流程、工具和模式写入 `docs/pipeline/`。
- 当前维护规则和架构总纲写入本文件；书级制作记录写入 `work-epub/<book>/制作说明.md`，任务计划、review 先留在任务或 issue 中。
- 已完成的计划、review、推导和实验记录归档到 `archive/` 或保留在 git 历史中，不直接驱动行为。
- 排版决策记录写入 `records/`；仓库级文件只保存脱敏、可复用的机器可读判断。
- 新增第三方来源说明写入 `THIRD_PARTY.md`。

历史计划和实验快照只在任务明确要求时修改。不要为了统一措辞重写 `archive/` 中的历史记录。
