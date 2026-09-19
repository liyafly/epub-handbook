# AGENTS.md

本文件是本仓库给 AI 协作代理使用的通用主入口，也是 AI 工作约束的唯一维护源。
Codex、Claude Code 以及其他代理开始工作前都必须先读取本文件；平台专用入口只做跳转，不复制规则。

普通人或只想做、修一本书的用户请看根 `README.md` 与 `docs/learn/`，无需阅读本文件；本文件面向 AI 代理与专业维护者。

## 启动读取顺序

1. 先阅读本文件，判断任务属于「已有 EPUB 清洗」「源材料接入」「阅读器兼容性实测」「实现约束变更」还是「说明增强」。
2. **Go 实现 / CLI 命令面 / SKILL.md 改写 / 删除旧实现：先完整读 [`SPEC-go-architecture.md`](docs/final/SPEC-go-architecture.md)（第一档架构硬约束，由 `internal/archguard/` 守卫强制）与 [`SPEC-go-modern-guidelines.md`](docs/final/SPEC-go-modern-guidelines.md) §2（先检测 `go.mod` 再取适用规则），再读 `docs/pipeline/go-rewrite-handoff.md` 了解当前状态。** `docs/pipeline/go-cli-rearchitecture.md` 只是 SPEC 落地前的背景蓝图，冲突时不得覆盖 SPEC。
3. 已有 EPUB 清洗：`docs/final/SPEC-实现约束.md` §10、`docs/pipeline/cleanup-flow.md`、`docs/pipeline/refinement-harnesses.md`。
4. 源材料接入：`skills/epub-source-intake/SKILL.md`，先建立可审计的 source bundle。
5. 阅读器兼容性实测：`templates/epub-style-demo/README.md`、`templates/epub-style-demo/SCENE_MATRIX.md`、`docs/final/reader-matrix.yaml`。
6. 只有在任务需要时才读取对应的 `skills/*/SKILL.md`；技能索引和推荐顺序见 `skills/README.md`。
7. 若模型或客户端不会自动发现本文件，提示词必须显式要求先读取根目录 `AGENTS.md`。

## 架构分工

架构是 **Go 单一 CLI + 私有字体 provider**。依赖方向、十条不变式、任务模板和迁移映射
以 `docs/final/SPEC-go-architecture.md` 为唯一硬约束；迁移历史见 `docs/pipeline/go-rewrite-handoff.md`。

| 层 | 状态 | 职责 |
|---|---|---|
| 公开 CLI / agent runtime | **Go**（`cmd/epub` + `internal/`，已落地） | 唯一公开命令、capability registry、流水线与统一 JSON 信封 |
| 字体工具 | **独立 provider**（Python + FontTools，`tools-font/`） | 覆盖、子集化与复杂字体处理；不打包进发行包，由 `internal/extern` 调起。安装与缺失时的降级行为见 `tools-font/README.md` |
| CSS | **Go scan/editset 规则层** | 只产出 lossless byte-range edit；禁止整文档序列化，禁止用正则解析复杂 CSS |
| 机器契约 | `contracts/` | capability、request/result 与 redline 事实来源 |
| 规范/证据 | `docs/final/` + `templates/` + `reader-matrix.yaml` | policy/evidence 唯一来源 |

公开命令面是 **harness 中立**的：CLI 不含任何模型 key、endpoint 或厂商适配层，
能力面由 `epub capabilities` 自描述，接入方（skill / MCP / 其它 Agent 工具）自己决定怎么调。
旧执行面（Python `scripts/`、Swift/GUI、`adapters/` provider 适配层、旧 `tools/`）均已删除，
迁移史见 `docs/pipeline/go-rewrite-handoff.md`。架构规则由 `internal/archguard/` 的守卫测试强制。硬约束：

- **禁止修改 `internal/archguard/`**。守卫失败时修改实现；若确信守卫有误，停下来交由人类审阅。
- 不得向文档新增旧执行面引用；`tools/parity/legacy-refs.txt` 棘轮已归零，应保持为零。
- `skills/` 下不得出现 `.py` / `.sh`；SKILL.md 只能调用 `epub run <capability-id>`，不得依赖 Go internal package 或私有 provider 路径。
- 新增 capability 必须走 SPEC §6.1 任务模板并通过 §5.2 parity gate。

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

第三方来源记录写入 `THIRD_PARTY.md` 与 `references/`；实体 `.epub` 只在有明确保留理由和许可记录时入 git。人工 diff review 使用 Calibre Editor 或 VS Code。

## 已有 EPUB 固定流程

已有 EPUB 默认遵循「保留 before → preflight → 必要时格式化和文件名反混淆 → EPUB3 迁移 → 精排建议 → 红线校验 → 人工 diff review」：

书级项目默认位于 `work-epub/<book>/`，每本书是独立本地 Git 仓库，目录固定为
`01 源文件/`、`02 校对材料/`和 `03 制作工作区/`。详细边界见
`docs/pipeline/book-workspace.md`。手册主仓库忽略 `work-epub/`，禁止把书级 Git 误加为 submodule。

1. 把入选底本保留在 `01 源文件/`，记录 SHA-256；编辑只发生在 `03 制作工作区/epub/` 或新候选 EPUB。禁止在唯一原件上直接修改。
2. 运行 `epub run epub.package.nav.audit --input <input.epub> --json`，先判断 DRM、加密标记、文件损坏和结构风险。
3. 如果资源目录混乱、文件名明显混淆或需要稳定 diff，先 dry-run：

   ```sh
   epub run epub.structure.normalize --input input.epub \
     --output normalized.epub --dry-run --json
   ```

4. 人工确认 dry-run 报告中的两个阶段：先格式化资源目录，再按 OPF manifest id 做文件名反混淆。确认后移除 `--dry-run` 写出 normalized EPUB，并原样保存 `--json` 信封（其 `facts["epub.structure.normalize.mappings"]` 即改名映射）。
5. 将 normalized EPUB 作为后续输入。按序运行 `epub.package.migrate.epub3`、精排能力（`epub.layout.audit` / `epub.text.content.analyze` / `epub.image.layout.optimize` / `epub.font.coverage.analyze` / `epub.typography.optimize` 等）和相关专项 skill。
6. 运行 `epub redline --check all --path-map <normalize 信封.json> before.epub after.epub`（信封可直接作为路径映射，见 `docs/pipeline/cleanup-flow.md` §1.5），再用 Calibre Editor 或 VS Code 做人工 diff review。
7. 把值得跨书复用的人工判断写入 `records/typeset-decisions.jsonl`；只属于当前书的排版结论默认汇总到书根的 `制作说明.md`。只有工具需要机器可读输入时，才在 `02 校对材料/` 按需保留书级决策 artifact。授权正文校订的含文决策必须放在 `02 校对材料/正文校订/`，不得混入仓库级 `records/`。
8. preflight、dry-run、lint 和中间 JSON 放入 `03 制作工作区/.pipeline/` 并默认忽略；在 `制作说明.md` 持久记录输入/输出 SHA、迁移或跳过理由、红线结果、diff review、阅读器实测与需回写项。正在被 gate 引用的 path map 或校订决策不得提前删除。

用户明确授权校订正文时，**正文不变 gate 不得被删除、伪造为通过或用宽泛 allow-list 掩盖**；应切换到 `docs/final/SPEC-实现约束.md` §10.1.1 与 `docs/pipeline/cleanup-flow.md` §7.1 的授权正文校订分支，该分支的逐条要求以那两处为准。

边界：

- `epub run epub.structure.normalize` 不提供 DRM 解密。
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
- **具体排版条目一律查 `docs/final/SPEC-实现约束.md`，本文件不复制**（图文环绕、带样式下划线、
  MathML `properties`、弹注 class 词汇、字体归属等）。本文件只登记维护规则与流程。
- 任何阅读器实测规则必须能追溯到 demo、artifact、阅读器名称和版本、现象与结论。信息不完整时只能记录为待验证假设。
- 新增第三方 EPUB 参考样本时，必须同步更新 `THIRD_PARTY.md`，写清来源、作者、许可和链接。

## 最小验证矩阵

按改动类型运行足够的验证，不要只依赖人工阅读：

| 改动类型 | 至少运行 |
| --- | --- |
| 任意改动 | `git diff --check` |
| Go 代码 | `go build ./...` 与 `go test ./...` |
| 架构相关改动（依赖方向、capability、SKILL.md、文档执行面） | `go test ./internal/archguard/ -v` |
| 已有 EPUB 清洗 | `epub run epub.package.nav.audit`、结构规范化 dry-run 或跳过理由、`epub redline --check all`、人工 diff review |
| demo、validator 或 `docs/final/` | build demo、`epub run epub.style.demo.maintain --input <artifact> --json`、`epub run epub.notes.popup.normalize --input <artifact> --json` |
| OPF、nav、NCX | 额外运行 `xmllint --noout ...`；本机没有 `xmllint` 时记录跳过理由 |
| 任意 EPUB 产物 | `epub run epub.package.nav.audit --input <artifact> --json` 与 `epub redline --check all`；error 必须清零或逐条给出豁免理由；EPUBCheck 只在 GitHub Actions 作为 CI gate 运行 |

可选安装 hook 模板（本仓 hook 不调外部脚本，只调 Go 守卫与 CLI）：

```sh
cp hooks/pre-commit.epub-handbook .git/hooks/pre-commit
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
