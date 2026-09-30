# EPUB 清洗 runbook

> 本文是已有 EPUB 清洗的唯一步骤来源。AGENTS.md「已有 EPUB 流程」是它的摘要。
> 规则来源：SPEC-实现约束 §10。

## 变量（每本书先设一次）

书级工作区不存在时先 `git init "$BOOK_ROOT"`，并在书级 `.gitignore` 写入 `03 制作工作区/.pipeline/` 与 `03 制作工作区/dist/`。

```sh
BOOK_ROOT='work-epub/<book>'
W="$BOOK_ROOT/03 制作工作区/.pipeline"
SOURCE_EPUB='<原 EPUB 路径>'
SOURCE_NAME='<原文件名.epub>'
mkdir -p "$BOOK_ROOT/01 源文件" "$W/before" "$W/after"
CUR="$W/before/source.epub"     # CUR 永远指向"最新的、已通过红线的候选"
```

## 主线

| 步 | 命令 | 产物 | 通过条件 | 失败时 |
| --- | --- | --- | --- | --- |
| S0 冻结 | `cp "$SOURCE_EPUB" "$BOOK_ROOT/01 源文件/$SOURCE_NAME" && cp "$SOURCE_EPUB" "$W/before/source.epub" && shasum -a 256 "$BOOK_ROOT/01 源文件/$SOURCE_NAME" "$W/before/source.epub"` | `01 源文件/$SOURCE_NAME`、`source.epub` | 两个 SHA 一致，并写进 `制作说明.md` | — |
| S1 预检 | `epub run epub.package.nav.audit --input "$CUR" --json > "$W/s1-audit.json"` | s1-audit.json | 无 DRM/损坏类 error | DRM、未知加密、ZIP 损坏 → **停止**，报告用户 |
| S2 规范化（可选） | ① dry-run：`epub run epub.structure.normalize --input "$CUR" --output "$W/after/s2.epub" --dry-run --json > "$W/s2-dry.json"`；② 审映射；③ 同一命令去掉 `--dry-run` 实跑并把信封保留到 `"$W/s2-normalize.json"`；④ `CUR="$W/after/s2.epub"` | s2.epub、s2-normalize.json | 映射逐条看过；实跑 exit 0 | 不需要就在制作说明写跳过理由 |
| S3 EPUB3 迁移（EPUB2 或缺 nav 时） | 先 `--dry-run`，再 `epub run epub.package.migrate.epub3 --input "$CUR" --output "$W/after/s3.epub" --json > "$W/s3.json"`；`CUR="$W/after/s3.epub"` | s3.epub | exit 0，S6 通过 | 读 findings，不覆盖重试 |
| S4 审计（只读） | `epub.package.nav.audit`、`epub.text.content.analyze`、`epub.image.layout.optimize`、`epub.font.coverage.analyze` 各跑一次 `--input "$CUR" --json` | s4-*.json | 只生成报告 | 字体 provider 缺失 → 记"无字体覆盖结论"，继续 |
| S5 修改（每次只做一项） | 按 `cleanup-patterns.md` 判定模式，再在 [skills/README 技能索引](../../skills/README.md#技能索引)选择该 skill 下一个 capability；dry-run 后写出 `$W/after/s5-<n>.epub` | s5-n.epub | 紧接着跑 S6 并通过，才 `CUR=` 它 | 丢弃该候选，CUR 不变 |
| S5f 字体（可选） | `epub run epub.font.subset --input "$CUR" --output "$W/after/s5-font.epub" --json [font_config=fonts.json]` | s5-font.epub 与 capability envelope；provider 临时报告位于系统临时目录，命令结束时删除 | provider 用独立字符收集器比较完整源字体与子集候选，确认没有新增缺字、空字形或 IVS/SVS 损失；exit 0 后检查通过的 manifest 字体 entry 写入候选；全项 redline 再确认非字体内容不变 | 失败不写出候选 EPUB、不应用字体编辑；检查 `font-subset.check-failed` finding、完整字体源和 `THIRD_PARTY.md` 许可记录 |
| S6 红线（**每次写出后都跑**） | `epub redline --check all [--path-map "$W/s2-normalize.json"] "$W/before/source.epub" <新候选>` | 终端输出 | exit 0 | 不删 gate、不放宽 allow-list；丢弃该候选 |
| S7 复检 | 对 `$CUR` 再跑 S1 的 nav.audit | json | error 为 0，或逐条写明授权/豁免 | 回到 S5 |
| S8 人工 diff | 按 `epub-diff-review.md` 在 Calibre / VS Code 看 before vs `$CUR` | 记录 | 每处差异都在授权范围 | 回到 S5 |
| S9 交付 | 按附录 E 模板写 `制作说明.md`；涉及阅读器兼容时更新 `reader-matrix.yaml`（未实测记 `warn`/待验证） | 制作说明.md | 用户确认 | — |

写出能力的 dry-run 成功时，信封为 `status=planned`、退出码 0；候选只在内存中，不会创建 `--output` 文件。出现 error finding 时仍为 `failed` / exit 1。只读能力运行 dry-run 仍为 `complete` / exit 0。审阅 planned 的 facts 和红线后，再用同一参数去掉 `--dry-run` 写出新候选。

`--path-map` 只在 S2 实际改过文件名时加。

失败候选保留供分析但不交付；恢复时从 `CUR` 指向的最近通过红线的候选继续，不覆盖原输入或既有锚点。

## 附录 A 授权正文校订（仅用户明确授权）

普通清洗仍以主线 S6 的正文不变 gate 为默认。用户明确要求按参考版校订字词、标点或空格时，切换到 [SPEC §10.1.1](../final/SPEC-实现约束.md)，不要删除 text gate，也不要用宽泛 allow-list 把差异伪装成不变。

### 冻结输入与比较范围

1. 保留现版与参考版不可修改副本，记录每个 EPUB 的 SHA-256。
2. 建立篇章映射：篇名、现版 XHTML、参考 EPUB、参考 XHTML、版本或来源说明。
3. 明确连续正文提取范围。篇名、小标题、篇末日期、noteref、注释正文、图片和图注是否参与比较必须逐项写清；排除的注释与图片后续单独做签名校验。
4. 参考版只提供候选文字，不直接整章覆盖。即使用户选择“整篇采用”，也必须把该选择展开为该篇全部差异项的 `adopt_reference` 决策，并导出、校验同一份逐项 artifact；不能绕过稳定 id、片段复核和总数校验。

### 静态审阅页与决策 JSON

差异很多、用户不适合手写清单时，优先在本地生成静态 HTML：逐项显示篇章、差异类型、精确 locator、现版/参考片段和上下文，并支持以下状态：

| 状态 | 含义 | 应用条件 |
| --- | --- | --- |
| `adopt_reference` | 采用参考版片段 | 可直接应用 |
| `keep_current` | 保留现版片段 | 可直接应用 |
| `manual` | 使用人工填写的最终片段 | `manual_text` 非空 |
| `pending` | 待查 | 禁止应用 |

导出的 JSON 至少包含 schema version、差异源报告 SHA-256、现版/参考 artifact 身份、item count、稳定 id、篇章、两侧片段和最终决策。应用前必须满足 `pending=0`、`undecided=0`、`manual_missing=0`。含正文片段的 Markdown、HTML 与 JSON 只留在书级 `02 校对材料/正文校订/`，不得复制进 `records/` 或提交为手册仓库级样本。

### 防止审阅结果过期

应用器必须重新计算或逐项核对差异：源报告 SHA、item id、篇章、现版片段、参考片段和总数任一不符就停止。不能只凭相同文件名假设 JSON 仍适用于当前 EPUB。

### 写出与验证

只生成新候选，不覆盖现版或参考版。正文变化已获授权，因此 `--check text` 与 `--check all` 会如实失败，不能声称“全量红线通过”。非文本红线必须以生成差异和决策 artifact 时冻结的现版为 `EDITORIAL_BASE`；冻结后的现版路径如下。若它早于结构规范化，再传入对应映射：

```sh
EDITORIAL_BASE="$CUR"  # 生成差异时冻结的现版
epub redline --check metadata,spine,cover,drm,anchors \
  "$EDITORIAL_BASE" \
  "$W/after/editorial-candidate.epub"
# 若 EDITORIAL_BASE 早于结构规范化，追加：--path-map "$W/s2-normalize.json"
```

同时必须证明：

- 结构变换的往返一致或字节幂等只能证明转换器可逆，不能证明往返前没有丢内容；
  必须另与变换前冻结版本比较可见字符或目标节点签名，并证明增减恰好落在已授权决策内；
- 最终连续正文逐字等于决策 JSON 合并结果；
- 只允许决策 locator 指向的文字节点变化；目标 XHTML 的非文字 DOM / 属性签名保持不变，包括 tag 序列、`id/class/epub:type/href/src/alt/lang`、`em/strong`、ruby / rt 与 pagebreak；
- noteref、注释正文、注释目标、图片 `src/alt` 和其他排除结构保持不变；
- 若篇名变化和目录同步均已获授权，对应 nav.xhtml / toc.ncx 标签可进入成员白名单，但标签必须等于最终篇名，链接目标和导航顺序必须不变；未授权时导航文件不得随正文改动；
- 输出“现版 → 候选”与“候选 → 参考版”两份 unified diff，后者明确展示保留现版或手工修正的例外；
- 结构审计（`epub run epub.package.nav.audit`）、ZIP 完整性、弹注校验（`epub run epub.notes.popup.normalize`）和产物结构检查通过；EPUBCheck 在 GitHub Actions 作为 CI gate 运行。

同时交付正文自由版与锁定版时，两版使用同一份决策 JSON，并断言目标正文完全一致；字体相关成员之外的差异按 SPEC §8 白名单复核。

## 附录 B 批量处理

一次处理多本 EPUB 时，每本书建立独立书级工作区，并按本文主线执行。批量处理不免除单书预检、每次写出后的 S6 红线和人工 diff review。单批次建议不超过 50 本，便于逐本复核。

### 批量预览与候选输出

`epub clean` 可对单本 EPUB 或目录中的 `.epub` 文件按统一步骤运行；目录递归扫描、忽略非 EPUB 文件并按路径顺序串行处理。默认不执行变换，只审计并生成逐书 `planned` 报告。使用 `--steps` 明确选择 `normalize,migrate,css` 中的步骤，顺序必须保持不变。取消后，已开始的书会保留逐书报告；尚未开始的输入路径仅列在批次信封的 `facts["epub.clean.notStarted"]` 中，不会生成逐书报告。

```sh
# 默认只审计并生成计划；只写每本书的 JSON 汇总，不变换书稿。
epub clean "$CUR" --out "$W/clean-preview"

# 明确选择结构变换；审查后才加 --approve 写出。
epub clean "$CUR" --out "$W/normalize-preview" --steps normalize
epub clean "$CUR" --out "$W/normalize-approved" --steps normalize --approve

# JSON 模式只把批次信封写到 stdout；日志和错误写到 stderr。
epub clean "$BOOKS" --out "$W/clean-preview" --json
```

也可以用 Bash 循环为每本输入建立隔离的报告目录。变量 `BOOKS` 应指向输入目录，`W` 指向该批次的临时工作区；输入路径含空格时仍会作为单个参数传入。

```bash
BOOKS='/path/to/input-books'
W='/path/to/batch-work'
EPUB_BIN=${EPUB_BIN:-epub}
mkdir -p "$W"
while IFS= read -r -d '' source; do
  relative=${source#"$BOOKS"/}
  book_out="$W/batch-preview/${relative%.*}"
  mkdir -p "$book_out"
  "$EPUB_BIN" clean "$source" --out "$book_out" --json > "$book_out/batch.json" || {
    exit_code=$?
    printf '%s exit=%s\n' "$source" "$exit_code" >> "$W/failures.txt"
  }
done < <(find "$BOOKS" -type f -iname '*.epub' -print0)
```

步骤候选在同一个 Book session 的内存态中串联；成功步骤产生 `step:<name>` 状态，失败步骤的 fork 不进入后续阶段。各步摘要用 `inputState`、`outputState` 和 `changedEntries` 表示状态流转与 entry 差异，失败步骤的 `outputState` 仍是其输入状态。中间态不是 ZIP 文件，因此不生成中间 ZIP SHA；逐书 envelope 的 `input.sha256` 是原始输入 SHA，`output.sha256` 只在最终候选实际写出后记录。未批准预演用 `facts["epub.clean.previewState"]` 标出当前内存态，不提供伪造的 preview ZIP SHA。随后对最终状态重新运行 nav audit 和全项红线。每本已开始处理的书写一个 `<书名>.clean.json` 信封；批次 `--json` 还会在 stdout 返回逐书状态、路径、findings，以及取消时的 `facts["epub.clean.notStarted"]` 路径列表。若运行了 normalize，逐书报告的 `facts["epub.clean.normalize.mappings"]` 可直接作为后续 `epub redline --path-map` 的映射信封。未批准时状态为 `planned`；批准并通过时为 `complete`。成功退出码为 0；阻断、步骤失败或末次审计/红线失败会标为 `failed` 并返回 1。

带 `--approve` 时，最后一个成功步骤产生的候选只有在末次审计和全项红线通过后才写入输入书籍的相对目录下同名 EPUB；步骤中间态不写入磁盘。若步骤或检查失败，候选不写出。`facts["epub.clean.candidateSteps"]` 按执行顺序列出实际进入候选的成功变换步骤，不包含失败步骤的 fork。报告 facts 中 `pipeline.artifactDisposition` 和 `pipeline.blockers` 说明候选资格与阻断项：`planned` 表示未发布的审计计划或内存预演，`approved` 表示通过 gate 并已写出 EPUB；报告写入失败另由 `clean.report-write-failed` finding 标记；`withheld` 表示候选被阻断且未留存，`none` 表示没有候选。已有报告或候选路径不会覆盖；目录输入的 `--out` 必须在输入目录之外。flags-first 写法可用 `epub clean --out DIR [其他 flags] INPUT`，把输入放在 flags 后面。

`epub clean` 不替代 S8 人工 diff review、S9 制作说明或真实阅读器验收。只有报告与候选对应的 SHA、finding 和 diff 都审过，且相关阅读器实测完成后，才记录书级结论。

### 模型与隐私说明

清洗主体是**确定性能力命令**，AI 只是辅助——AI 不直接执行写出步骤，默认流程**完全不调用任何模型**，可离线/气隙运行，稿件不出本机。

⚠️ **风险提示**：出版社及专业制作团队的稿件常涉及机密与版权。把正文交给**云端大模型**存在泄露风险。

✅ **推荐**：确需 AI 辅助判断时，使用**本地部署的大模型**，让 AI 只读取本地报告并给出建议；执行仍由人逐条确认，稿件不出本机。

## 附录 C 自造 demo 自检

自造 demo 的构建与验证见 [cleanup-demo-books README](../../templates/cleanup-demo-books/README.md)；运行其中的 build.sh 需要 Python 3。

## 附录 E 制作说明模板

````md
# 清洗记录：<书名>

> 日期：<DATE>
> 输入 SHA-256：<sha>
> 输出 SHA-256：<sha>

## 0. 健康检查

- zip：OK
- mimetype：OK
- container.xml：OK
- DRM：无
- 包结构检查：N error / N warning

## 1. 审计结果

- ...

## 2. 模式判定

匹配模式：模式 B。

### 2.1 迁移或跳过理由

- 状态：migrated | skipped
- 理由：<说明迁移内容，或跳过迁移的依据>

## 3. 清洗步骤

### S5-<n>: <capability-id>

- dry-run 输出：`s5-<n>-dry.json`
- 文本红线：pass
- 中间产物：`$W/after/s5-<n>.epub`

## 4. 完整红线校验

```sh
epub redline --check all [--path-map "$W/s2-normalize.json"] "$W/before/source.epub" "$W/after/s5-<n>.epub"
```

## 5. Diff 概览

- 结构：unchanged
- 文本：identical
- 样式：N selector 改动
- 资源：N add / delete / modified
- 元数据：core unchanged

## 6. 阅读器实测

| 阅读器名称 | 版本 | 产物 SHA-256 | 状态 | 现象 / 证据 |
| --- | --- | --- | --- | --- |
| <名称> | <版本> | <SHA-256> | pass / warn / fail / na | <现象、截图或日志路径> |

## 7. 待办

- <待复测项或后续事项；没有则写“无”>

````
