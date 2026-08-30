# EPUB 清洗流水线

> 状态：流程文档；用于把一本已存在的 EPUB 收拾干净。
> 对应 SPEC：[§10 AI 改动边界](../final/SPEC-实现约束.md)。
> 对应工具：Go CLI `epub`（`epub run <capability-id>`、`epub capabilities`、`epub redline`，契约见 [SPEC-go-architecture §8](../final/SPEC-go-architecture.md)）、外部 diff 工具（Calibre / VS Code，见 [EPUB diff review](epub-diff-review.md)）。

## 整体流程

```text
0. 准备 -> 1. preflight 健康检查 -> 1.5 可选结构规范化 ->
2. EPUB3 迁移基线 -> 3. 精排建议 harness -> 4. 红线预检 ->
5. 分派清洗 -> 6. 文本校验 -> 7. diff 人工 review ->
8. 用户确认 -> 9. reader-matrix 回写
```

## 书级工作区

新项目先按 [一书一 Git 工作区](book-workspace.md) 建立 `work-epub/<book>/`。本流水线的
`before/after/reports` 是工具内部目录，统一放在书级工作区的忽略区：

```sh
BOOK_ROOT='work-epub/book-a'
PIPELINE_WORK="$BOOK_ROOT/03 制作工作区/.pipeline"
```

下文为了突出流水线内部结构，仍用 `work/...` 作简写；在书级项目中应将这个
`work` 理解为 `$PIPELINE_WORK`，不再新建仓库根级 `work/<book>/`。

## 按序能力入口

需要快速生成 before 基线、转换产物和审计报告时，Go CLI 没有一键流水线命令，按固定顺序逐能力执行，人工 review 在每步之间：

```sh
BOOK=work/before/source.epub   # 先按 §0 保留不可修改基线

# 1. 健康检查 / 结构审计
epub run epub.package.nav.audit --input "$BOOK" --json > work/preflight.json

# 2. 可选结构规范化（先 dry-run，确认后实跑）
epub run epub.structure.normalize \
  --input "$BOOK" \
  --output work/after/step-0-normalized.epub \
  --dry-run --json > work/step-0-normalize.dry-run.json

# 3. EPUB3 迁移
epub run epub.package.migrate.epub3 \
  --input "$BOOK" \
  --output work/after/step-1-epub3.epub \
  --json > work/epub3-migration-apply.json

# 4. CSS 清洗与排版精排（按需）
epub run epub.css.layering.optimize \
  --input work/after/step-1-epub3.epub \
  --output work/after/step-2-css.epub \
  --json > work/css-cleanup.json
epub run epub.typography.optimize \
  --input work/after/step-2-css.epub \
  --output work/after/cleaned.epub \
  --json > work/typography.json

# 5. 红线校验
epub redline --check all "$BOOK" work/after/cleaned.epub
```

它收口本页可以自动执行的部分：结构审计、EPUB3 转换、弹注校验（`epub run epub.notes.popup.normalize`）、metadata / DRM / anchors 红线校验、精排建议、文本结构角色分析和 findings。人工 diff review、文本角色 class 写入和阅读器实测仍必须继续执行。

每个能力的 `--json` 统一信封报告单独落盘归档。写出型能力自带内置红线 gate：预期中的变更（如 metadata 编辑、合并新增分册文件）会以 error findings 记录并把退出码置 1，产物仍会写出供 review；结论以随后的显式 `epub redline` 与人工 diff review 为准。结构规范化的 dry-run/apply JSON 始终单独保留，因为前者需要 review，后者的 `mappings` 还要提取后传给 `epub redline --path-map`（见 §1.5）。

如果需要 §1.5 结构规范化，先跑上面的 `--dry-run`，人工确认报告后去掉 `--dry-run` 实跑。详细命令见 [oneclick-epub3-converter.md](oneclick-epub3-converter.md)。

### 弹注和语言壳层边界

- 对 Sigil 旧结构 `a#noteref_N -> aside#footnote_N`（外层为 `section[epub:type="footnotes"]`）只有在该 section 内所有 `aside` 都可识别时，才合并为一个 grouped `aside/ol/li`；保留 `noteref_N` / `footnote_N` 与全部注释正文。不能完整识别时停止自动转换并人工 review。
- XHTML 根同时缺 `lang` 和 `xml:lang` 时，只能从 OPF `dc:language` 复制同一值；OPF 也没有语言值时记录问题，不猜测。已有任一属性时补齐另一属性，不覆盖既有值。
- `epub redline` 对 `a[epub:type~=noteref]` 和 `a[epub:type~=backlink]` 的可见控件文字不计入正文（数字触发器、图标、`◎` 是等价表示）；`li.footnote-item` 内的注释正文仍逐字校验。
- 图片 noteref 的上标外壳使用 `sup.note-marker`：`line-height:0` 限制图标不撑高正文行框，内部图标用相对上移实现略高基线。不得用无作用域的 `sup img`，也不得影响普通文字上标。
- 用户明确要求删除页面时，先建立精确删除白名单，并同步删除 ZIP、manifest、spine、nav、NCX 引用；文本 gate 只 allow-list 被授权删除的 XHTML 和重新生成的 nav，不能借此跳过其他正文页。
- 所有重打包输出排除任意路径中的 `.DS_Store`；清洗输出不得携带 Finder 元数据。

## 0. 准备

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub
```

不要原地覆盖原始 epub。

## 1. 健康检查

任何清洗前必须通过的最低门槛。任一项失败立即停止。

```sh
EPUB=work/before/source.epub
epub run epub.package.nav.audit --input "$EPUB" --json > work/preflight.json

unzip -t "$EPUB" >/dev/null && echo "zip OK" || { echo "zip broken"; exit 1; }
unzip -p "$EPUB" mimetype   # 应输出 application/epub+zip
```

mimetype 必须是首个 entry、STORED 存储、内容精确为 `application/epub+zip`；
构建产物的完整容器校验由 `epub run epub.style.demo.maintain --input <产物> --json`
与 CI 的 EPUBCheck 覆盖。
unzip -p "$EPUB" META-INF/container.xml | head -5 >/dev/null && echo "container.xml OK" || { echo "container.xml missing"; exit 1; }
unzip -p "$EPUB" META-INF/encryption.xml 2>/dev/null
```

产物结构检查由 `epub run epub.package.nav.audit` 与 `epub redline`（正文不变）组合覆盖；EPUBCheck 只在 GitHub Actions 作为 CI gate 运行。

默认结构检查要求 `ibooks:specified-fonts=true` 与直接 `body` 正文字体锁定成对出现；既有书的 `body-font-locked` class 仅作为兼容输入识别。只有既有书因历史兼容或用户明确要求保留自由正文 meta、且书级报告已经写明理由时，才在书级报告中记录该豁免；豁免不是新书模板入口。

发现 `META-INF/encryption.xml` 时默认停止。若声明目标在 ZIP 中不存在，结构工具可移除该 stale 引用；若已确认只有 EPUB 标准字体混淆，可用下一节的 `mode=inspect` 显式验证。正文、样式、图片或未知算法加密且目标真实存在时仍立即停止。

`work/preflight.json` 里的 `status` 为 `failed` 时，不进入 EPUB3 迁移或 AI 清洗。

## 1.5 可选：先格式化，再文件名反混淆

如果 EPUB 内部目录散乱，或 manifest href 使用不可读文件名，先运行只读检查与组合 dry-run：

```sh
epub run epub.structure.normalize --input "$EPUB" --json mode=inspect
epub run epub.structure.normalize \
  --input "$EPUB" \
  --output work/after/step-0-normalized.epub \
  --dry-run --json > work/step-0-normalize.dry-run.json
```

`mode=normalize` 固定先执行 `format`，再执行 `deobfuscate-filenames`。确认两个阶段的 `mappings` 和 `warnings` 后去掉 `--dry-run`，写出新 EPUB 并保存实际报告：

```sh
epub run epub.structure.normalize \
  --input "$EPUB" \
  --output work/after/step-0-normalized.epub \
  --json > work/step-0-normalize.json
```

立刻把实际报告中的改名映射提取出来，作为路径映射传给红线 gate：

```sh
jq '{mappings: .facts["epub.structure.normalize.mappings"]}' \
  work/step-0-normalize.json > work/step-0-mappings.json

epub redline --check all \
  --path-map work/step-0-mappings.json \
  "$EPUB" \
  work/after/step-0-normalized.epub
```

该能力不提供 DRM 解密。若加密声明目标在 ZIP 中不存在，报告会记录并移除 stale 引用；如果 `mode=inspect` 已确认只有 EPUB 标准字体混淆，在红线命令额外添加 `--allow-font-obfuscation`。若写出了 step-0 产物，后续 EPUB3 迁移基线使用该产物。

结构规范化后再次运行结构审计（`epub run epub.package.nav.audit`）。如果 CSS 仍引用 ZIP 中不存在的 `.ttf`、`.otf`、`.woff` 或 `.woff2`，审计会记录 `missing-css-font-fallback` 警告：不要猜测该别名对应哪个嵌入字体，也不要自动删掉声明，保留 `local()` fallback 并人工复核。图片、样式等非字体资源断链仍是阻断错误。

## 2. EPUB3 迁移基线

如果 preflight 发现 `package_version` 不是 `3.0`，或缺少 `properties="nav"` 的 nav item，先生成 EPUB3 基线：

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

dry-run 先列出 package/nav 变更（status 为 `approval-required`，退出码 2）；确认后正式执行，处理 package、XHTML shell、已识别弹注和可选基础排版。它保留 `toc.ncx` 与 `spine toc="ncx"`，不改正文文字；实际计数以 apply 报告 `facts` 中的迁移计数为准。

迁移后立刻跑红线：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

epub redline --check text,metadata,spine,cover,anchors \
  --allow-list '*/nav*.xhtml' \
  "$BASE" \
  work/after/step-1-epub3.epub
```

如果当前基线已经是 EPUB3 且 nav 正常，本步可跳过。后续优先使用 step-0 产物，没有 step-0 才回退到 `work/before/source.epub`。

## 3. 精排建议

对当前基线跑精排建议组合（原精排 harness 已展开为下列单能力命令）：

```sh
BASE=work/after/step-1-epub3.epub
test -f "$BASE" || BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

epub run epub.layout.audit --input "$BASE" --json > work/refinement.json
epub run epub.text.content.analyze --input "$BASE" --json > work/content-analysis.json
epub run epub.image.layout.optimize --input "$BASE" --json > work/image-layout-advice.json
epub run epub.font.coverage.analyze --input "$BASE" --json profile=kindle-pessimistic > work/font-coverage.json
```

各项报告会按阶段提示是否需要：

- EPUB3 迁移；
- 标准 popup footnote 转换；
- 多字体排版 / 内嵌字体策略；
- 图片格式转换、封面声明、figure 版式；
- Ruby / 竖排专项复查；
- diff review 与红线 gate。

细节见 [refinement-harnesses.md](refinement-harnesses.md)。

## 4. 红线预检

参照 [SPEC §10.1](../final/SPEC-实现约束.md) 列出本次清洗可能触发哪些红线。

## 5. 审计扫描

```sh
BASE=work/after/step-1-epub3.epub
test -f "$BASE" || BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub
epub run epub.layout.audit --input "$BASE" --json > work/findings.json
jq .nextCommands work/findings.json
```

如果已经在 §3 生成 `work/findings.json`（同一 `epub.layout.audit` 报告），这里直接读取即可。

## 6. 分派清洗

按 `nextCommands` 与推荐 skill 顺序逐一执行。每次改动后跑：

```sh
REDLINE_BASE=work/after/step-1-epub3.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/after/step-0-normalized.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/before/source.epub
epub redline --check all "$REDLINE_BASE" work/after/step-N.epub
```

退出码 1 立即回滚该次 skill 改动。

## 7. 文本校验（自动 gate）

```sh
REDLINE_BASE=work/after/step-1-epub3.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/after/step-0-normalized.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/before/source.epub
epub redline --check all "$REDLINE_BASE" work/after/cleaned.epub
```

退出码必须 0。

## 7.1 授权正文校订（仅用户明确授权）

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
# 若 EDITORIAL_BASE 早于结构规范化，追加：--path-map work/step-0-mappings.json
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

## 8. Diff 人工 review

按 [EPUB diff review](epub-diff-review.md) 的两条路径做：

- 主路径（推荐）：Calibre Editor → Tweak Book → File → Compare to another book → 选 `work/after/cleaned.epub`。
- 精细路径：`unzip` 解压两侧到 `work/before-extracted` / `work/after-extracted`，再用 `git diff --no-index` 整树概览 / `code --diff` 逐文件 / `shasum -a 256` 列表对资源层。
- 五层覆盖：结构 / 文本 / 样式 / 资源 / 元数据。普通清洗的文本红线已在 §7 卡过；授权正文校订则同时核对决策 JSON 和两份正文 diff。人眼看到的改动必须与放行范围一致。

这一步只看文件差异，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。

### 记录 review 决策

把值得跨书复用的判断手工追加到仓库级记录 `records/typeset-decisions.jsonl`（每行一个 JSON 对象）：

```json
{"id":"dec-NNNN","date":"<DATE>","source":"manual-review","scene":"image-layout","finding":"lone-image-no-figure","candidates":["figure.img-left","figure.img-right","figure-fullwidth"],"chosen":"figure.img-right","rationale":"说明选择理由，不粘贴正文","scope":"global","reusable":true}
```

只属于当前书的判断默认写入书根 `制作说明.md`。只有后续工具需要机器可读输入时，才把同一条记录写入 `02 校对材料/排版决策.jsonl` 并把 `scope` 标为 `book`。两层记录都禁止保存正文文本；完整 schema 和隐私红线见 [`records/README.md`](../../records/README.md)。

## 9. 用户确认

把 diff 摘要、截图或导出 JSON 发给用户。授权正文校订还要一并给出决策统计和“候选 → 参考版”剩余差异摘要。用户确认后，`work/after/cleaned.epub` 或明确命名的 editorial candidate 作为交付。

## 10. reader-matrix 回写

如果清洗涉及阅读器兼容性变更，在 `docs/final/reader-matrix.yaml` 增条目，初始 status 一律 `warn`，直到有真实阅读器实测。

## 11. 批量模式

一次处理多本 epub 时，每本一个工作目录：

```sh
ls /path/to/books/*.epub | parallel -j 4 ./clean-one.sh {}
```

失败的写入 `failed.log`，跑完后重试：

```sh
cat failed.log | parallel -j 4 ./clean-one.sh {}
```

单批次建议不超过 50 本，方便人工 review。

## 12. 回滚剧本

每步产出带编号的中间 epub：

```text
work/after/
├── step-1-css-layering.epub
├── step-2-popup-footnote.epub
└── cleaned.epub
```

回滚到 step-K：

```sh
cp work/after/step-K-*.epub work/after/cleaned.epub
```

中间 epub 是回滚锚点，不要直接修改。

## 13. 可信度评估

| 指标 | 来源 | 期望 |
| --- | --- | --- |
| 红线触发数 | `epub redline --check all` | 必须 0 |
| 黄线条数 | Calibre Compare 文件树 modified 计数（或 `git diff --no-index --stat` 行数） | 记录 |
| EPUB 包结构错误数（after） | `epub run epub.package.nav.audit` + `epub redline`（EPUBCheck 在 GitHub Actions 作为 CI gate） | 清零，或逐条记录豁免理由 |
| 阅读器兼容性回归 | reader-matrix 复测 | 不变差 |

结论：

- 红线 0 + 必须 review 项 0 + 本地 lint 清零 -> 自动通过。
- 红线 0 + 有必须 review 项 -> 人工 review。
- 红线 > 0 -> 重做。

## 14. 错误恢复

每完成一步写入 `work/state.json`：

```json
{
  "input_sha256": "abc123",
  "completed_steps": [
    {"skill": "epub-css-layering-optimizer", "output": "after/step-1-css-layering.epub"}
  ],
  "next_step": "epub-typography-optimizer"
}
```

恢复时从 `next_step` 继续，跳过已完成步骤。

## 15. OCR-style 脏 epub 识别

特征：

- 章节几乎全是 `<img>` 引用，少量散乱文本。
- 文本有大量 OCR 噪点。
- 文件名常带 `scan` / `ocr` / `_p001`。

判定：

```sh
epub run epub.layout.audit --input work/before/source.epub --json \
  | jq '.findings[] | select(.detail == "ocr-residual")'
```

如果检测到，建议回到 `epub-source-intake`，重新 OCR 后再清洗。

## 16. 书级 `制作说明.md` 清洗记录模板

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

## 17. 自造 demo

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

## 18. 多轮收敛清洗

Go CLI 没有一键流水线或多轮自动收敛命令。需要多轮处理脏书时，按 §「按序能力入口」的顺序逐能力执行，人工 review 在每步之间，直到收敛：

```sh
# 每轮按序执行；输入取上一轮产物，before 基线始终不变
epub run epub.package.nav.audit --input "$BASE" --json
epub run epub.structure.normalize \
  --input "$BASE" \
  --output work/after/step-0-normalized.epub \
  --dry-run --json   # 需要结构规范化时先 dry-run，确认后去掉 --dry-run 实跑
epub run epub.package.migrate.epub3 \
  --input "$BASE" \
  --output work/after/step-1-epub3.epub \
  --json
epub run epub.css.layering.optimize \
  --input work/after/step-1-epub3.epub \
  --output work/after/step-2-css.epub \
  --json
epub run epub.typography.optimize \
  --input work/after/step-2-css.epub \
  --output work/after/cleaned.epub \
  --json
epub redline --check all work/before/source.epub work/after/cleaned.epub
```

纪律与原自动循环一致：

- 先保留不可修改的 before 基线，再开始任何写出步骤。
- 结构规范化保持显式批准：先 `--dry-run` 检查报告，确认后再实跑。
- 每个写出步骤之后立刻跑 `epub redline`；任一 gate 失败回滚到上一锚点（见 §12）。
- 每轮只做**确定性可判定**的改动；语义角色、结构改写等判断交给人工或显式授权的 AI 辅助，AI 全程不碰正文文字，只读报告、给建议。
- 每轮结束把结果分三栏汇总：✅ 已自动改（能力已执行且显式红线通过）、💡 建议你改（报告中的 warn / info findings 与 `nextCommands`）、👁 需人工校对 / 实测（`approval-required` 与人工 review 项）。

收敛判据：连续两轮无新动作，或达到人工设定的轮次上限即停。收尾在书根 `制作说明.md` 记录轮次与每轮红线结果。

### 18.1 模型与隐私（说明）

清洗主体是**确定性能力命令**，AI 只是辅助——AI 不直接执行写出步骤，默认流程**完全不调用任何模型**，可离线/气隙运行，稿件不出本机。

⚠️ **风险提示**：出版社及专业制作团队的稿件常涉及机密与版权。把正文交给**云端大模型**存在泄露风险。

✅ **推荐**：确需 AI 辅助判断时，使用**本地部署的大模型**，让 AI 只读取本地报告并给出建议；执行仍由人逐条确认，稿件不出本机。
