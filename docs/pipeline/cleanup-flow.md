# EPUB 清洗 runbook

> 本文是已有 EPUB 清洗的唯一步骤来源。AGENTS.md「已有 EPUB 流程」是它的摘要。
> 规则来源：SPEC-实现约束 §10。

## 变量（每本书先设一次）

```sh
BOOK_ROOT='work-epub/<book>'
W="$BOOK_ROOT/03 制作工作区/.pipeline"
mkdir -p "$W/before" "$W/after"
CUR="$W/before/source.epub"     # CUR 永远指向"最新的、已通过红线的候选"
```

## 主线

| 步 | 命令 | 产物 | 通过条件 | 失败时 |
| --- | --- | --- | --- | --- |
| S0 冻结 | `cp <源.epub> "$W/before/source.epub" && shasum -a 256 "$W/before/source.epub"` | source.epub | SHA 写进 `制作说明.md` | — |
| S1 预检 | `epub run epub.package.nav.audit --input "$CUR" --json > "$W/s1-audit.json"` | s1-audit.json | 无 DRM/损坏类 error | DRM、未知加密、ZIP 损坏 → **停止**，报告用户 |
| S2 规范化（可选） | ① 加 `--dry-run` 跑 `epub run epub.structure.normalize --input "$CUR" --output "$W/after/s2.epub" --json > "$W/s2-dry.json"`；② 审映射；③ 去掉 `--dry-run` 实跑，输出存 `$W/s2-normalize.json`；④ `CUR="$W/after/s2.epub"` | s2.epub、s2-normalize.json | 映射逐条看过；实跑 exit 0 | 不需要就在制作说明写跳过理由 |
| S3 EPUB3 迁移（EPUB2 或缺 nav 时） | 先 `--dry-run`，再 `epub run epub.package.migrate.epub3 --input "$CUR" --output "$W/after/s3.epub" --json > "$W/s3.json"`；`CUR="$W/after/s3.epub"` | s3.epub | exit 0，S6 通过 | 读 findings，不覆盖重试 |
| S4 审计（只读） | `epub.layout.audit`、`epub.text.content.analyze`、`epub.image.layout.optimize`、`epub.font.coverage.analyze` 各跑一次 `--input "$CUR" --json` | s4-*.json | 只生成报告 | 字体 provider 缺失 → 记"无字体覆盖结论"，继续 |
| S5 修改（每次只做一项） | 模型读 S4 报告和书的实际文件，按 `cleanup-patterns.md` 选一个能力，dry-run 后写出 `$W/after/s5-<n>.epub` | s5-n.epub | 紧接着跑 S6 并通过，才 `CUR=` 它 | 丢弃该候选，CUR 不变 |
| S5f 字体（可选） | `epub-font subset "$CUR" --out "$W/after/s5-font.epub" [--config fonts.json]`，再 `epub-font check "$W/after/s5-font.epub"` | s5-font.epub、.font-report.json | subset exit 0；check 的 missing 只含"本来就靠后备字体"的字 | exit 1 = 检查失败未写出；exit 2 = 输入/配置问题 |
| S6 红线（**每次写出后都跑**） | `epub redline --check all [--path-map "$W/s2-normalize.json"] "$W/before/source.epub" <新候选>` | 终端输出 | exit 0 | 不删 gate、不放宽 allow-list；丢弃该候选 |
| S7 复检 | 对 `$CUR` 再跑 S1 的 nav.audit | json | error 为 0，或逐条写明授权/豁免 | 回到 S5 |
| S8 人工 diff | 按 `epub-diff-review.md` 在 Calibre / VS Code 看 before vs `$CUR` | 记录 | 每处差异都在授权范围 | 回到 S5 |
| S9 交付 | 按附录 E 模板写 `制作说明.md`；涉及阅读器兼容时更新 `reader-matrix.yaml`（未实测记 `warn`/待验证） | 制作说明.md | 用户确认 | — |

写出能力的 dry-run 成功时，信封为 `status=planned`、退出码 0；候选只在内存中，不会创建 `--output` 文件。出现 error finding 时仍为 `failed` / exit 1。只读能力运行 dry-run 仍为 `complete` / exit 0。审阅 planned 的 facts 和红线后，再用同一参数去掉 `--dry-run` 写出新候选。

`--path-map` 只在 S2 实际改过文件名时加。

## 附录 A 授权正文校订（仅用户明确授权）

普通清洗仍以 §7 的正文不变 gate 为默认。用户明确要求按参考版校订字词、标点或空格时，切换到 [SPEC §10.1.1](../final/SPEC-实现约束.md)，不要删除 text gate，也不要用宽泛 allow-list 把差异伪装成不变。

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

只生成新候选，不覆盖现版或参考版。正文变化已获授权，因此 `--check text` 与 `--check all` 会如实失败，不能声称“全量红线通过”。非文本红线必须以生成差异和决策 artifact 时冻结的现版为 `EDITORIAL_BASE`；它通常就是前文的 `REDLINE_BASE`，若从结构规范化前的源文件比较，则继续传入对应 `--path-map`：

```sh
EDITORIAL_BASE="$REDLINE_BASE"  # 必须与差异报告中的现版 artifact 身份一致
epub redline --check metadata,spine,cover,drm,anchors \
  "$EDITORIAL_BASE" \
  work/after/editorial-candidate.epub
# 若 EDITORIAL_BASE 早于结构规范化，追加：--path-map work/step-0-normalize.json
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

### 模型与隐私说明

清洗主体是**确定性能力命令**，AI 只是辅助——AI 不直接执行写出步骤，默认流程**完全不调用任何模型**，可离线/气隙运行，稿件不出本机。

⚠️ **风险提示**：出版社及专业制作团队的稿件常涉及机密与版权。把正文交给**云端大模型**存在泄露风险。

✅ **推荐**：确需 AI 辅助判断时，使用**本地部署的大模型**，让 AI 只读取本地报告并给出建议；执行仍由人逐条确认，稿件不出本机。

## 附录 C 自造 demo 自检

首轮端到端演示不依赖公版书。先生成仓库自造样本：

```sh
bash templates/cleanup-demo-books/build.sh
```

合法清洗对：

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/city-field-notes-before.epub \
  templates/cleanup-demo-books/dist/city-field-notes-after-clean.epub

epub redline --check all \
  templates/cleanup-demo-books/dist/paper-garden-before.epub \
  templates/cleanup-demo-books/dist/paper-garden-after-clean.epub
```

红线反例：

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/redline-trap-before.epub \
  templates/cleanup-demo-books/dist/redline-trap-after-text-changed.epub
```

前两条必须通过；反例必须失败。

## 附录 D 命令速查

### 能力总览

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

## 1. harness findings

- ...

## 2. 模式判定

匹配模式：模式 B。

## 3. 清洗步骤

### Step 1: <skill name>

- dry-run 输出：`step-1.dry-run.json`
- 文本红线：pass
- 中间产物：`after/step-1.epub`

## 4. 完整红线校验

```sh
epub redline --check all <redline-base.epub> after/cleaned.epub
```

## 5. Diff 概览

- 结构：unchanged
- 文本：identical
- 样式：N selector 改动
- 资源：N add / delete / modified
- 元数据：core unchanged

## 6. 可信度评估

- 红线触发数：0
- 结论：自动通过
````

## 附录 F 回滚与错误恢复

每个成功写出的中间 EPUB 都作为回滚锚点，文件名统一使用 s*.epub：

```text
work/after/
├── s3-epub3.epub
├── s5-1-css-layering.epub
├── s5-2-popup-footnote.epub
└── s9-cleaned.epub
```

回滚时从上一个已通过红线的候选生成新文件，不覆盖原始输入或已有锚点：

```sh
cp work/after/s5-1-css-layering.epub work/after/s9-restored.epub
```

失败的候选保留供分析，但不更新 CUR。恢复时从 CUR 指向的上一个成功候选继续，重新运行失败步骤并再次通过红线后再更新 CUR。流水线状态以 CUR、报告和制作说明中的 SHA-256 为准。
