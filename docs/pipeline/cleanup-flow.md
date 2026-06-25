# EPUB 清洗流水线

> 状态：流程文档；用于把一本已存在的 EPUB 收拾干净。
> 对应 SPEC：[§10 AI 改动边界](../final/SPEC-实现约束.md)。
> 对应工具：`scripts/epub_cleanup_pipeline.py`（一命令一键）、`scripts/epub_cleanup_loop.py`（多轮自动收敛）、`scripts/epub_preflight_harness.py`、`scripts/epub_structure_tool.py`、`scripts/epub3_migration_harness.py`、`scripts/epub_refinement_harness.py`、`scripts/epub_ai_harness.py`、`scripts/validate_text_invariance.py`、外部 diff 工具（Calibre / VS Code，见 [EPUB diff review](epub-diff-review.md)）。

## 整体流程

```text
0. 准备 -> 1. preflight 健康检查 -> 1.5 可选结构规范化 ->
2. EPUB3 迁移基线 -> 3. 精排建议 harness -> 4. 红线预检 ->
5. 分派清洗 -> 6. 文本校验 -> 7. diff 人工 review ->
8. 用户确认 -> 9. reader-matrix 回写
```

## 一命令入口

需要快速生成 before 基线、转换产物和审计报告时，优先运行：

```sh
python3 scripts/epub_cleanup_pipeline.py \
  /path/to/input.epub \
  --work-dir work/book-a
```

它收口本页可以自动执行的部分：before 复制、preflight、EPUB3 转换、产物 preflight、弹注校验、metadata / DRM / anchors 红线子集校验、独立正文文本 gate、精排建议和 AI findings。人工 diff review、文本角色 class 分派和阅读器实测仍必须继续执行。

默认只落盘 `reports/pipeline.json` 汇总报告，步骤 stdout/stderr 会保留在该 JSON 中。排障或需要逐项归档时加 `--keep-step-reports`，再额外写出 preflight、conversion、popup、redline、refinement 和 findings 分步报告。结构规范化的 dry-run/apply JSON 始终单独保留，因为前者需要 review，后者还要传给 `validate_text_invariance.py --path-map`。

如果需要 §1.5 结构规范化，用同一个入口先跑 `--normalize dry-run`，人工确认报告后在新的工作目录使用 `--normalize apply --approve-normalize`。详细命令见 [oneclick-epub3-converter.md](oneclick-epub3-converter.md)。

### 弹注和语言壳层边界

- 对 Sigil 旧结构 `a#noteref_N -> aside#footnote_N`（外层为 `section[epub:type="footnotes"]`）只有在该 section 内所有 `aside` 都可识别时，才合并为一个 grouped `aside/ol/li`；保留 `noteref_N` / `footnote_N` 与全部注释正文。不能完整识别时停止自动转换并人工 review。
- XHTML 根同时缺 `lang` 和 `xml:lang` 时，只能从 OPF `dc:language` 复制同一值；OPF 也没有语言值时记录问题，不猜测。已有任一属性时补齐另一属性，不覆盖既有值。
- `validate_text_invariance.py` 对 `a[epub:type~=noteref]` 和 `a[epub:type~=backlink]` 的可见控件文字不计入正文（数字触发器、图标、`◎` 是等价表示）；`li.footnote-item` 内的注释正文仍逐字校验。
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
python3 scripts/epub_preflight_harness.py "$EPUB" --format json > work/preflight.json

unzip -t "$EPUB" >/dev/null && echo "zip OK" || { echo "zip broken"; exit 1; }

python3 -c "
import zipfile
with zipfile.ZipFile('$EPUB') as z:
    first = z.infolist()[0]
    assert first.filename == 'mimetype', 'first entry must be mimetype'
    assert first.compress_type == zipfile.ZIP_STORED, 'mimetype must be stored'
    assert z.read('mimetype') == b'application/epub+zip', 'mimetype content invalid'
print('mimetype OK')
"

unzip -p "$EPUB" META-INF/container.xml | head -5 >/dev/null && echo "container.xml OK" || { echo "container.xml missing"; exit 1; }
unzip -p "$EPUB" META-INF/encryption.xml 2>/dev/null
python3 scripts/epub_lint.py "$EPUB"
```

发现 `META-INF/encryption.xml` 时默认停止。若声明目标在 ZIP 中不存在，结构工具可移除该 stale 引用；若已确认只有 EPUB 标准字体混淆，可用下一节的 `inspect` 显式验证。正文、样式、图片或未知算法加密且目标真实存在时仍立即停止。

`work/preflight.json` 里的 `preflight_status` 为 `fail` 时，不进入 EPUB3 迁移或 AI 清洗。

## 1.5 可选：先格式化，再文件名反混淆

如果 EPUB 内部目录散乱，或 manifest href 使用不可读文件名，先运行组合入口 dry-run：

```sh
python3 scripts/epub_structure_tool.py inspect "$EPUB" --report-format json
python3 scripts/epub_structure_tool.py normalize \
  "$EPUB" \
  --output work/after/step-0-normalized.epub \
  --dry-run \
  --report-format json > work/step-0-normalize.dry-run.json
```

`normalize` 固定先执行 `format`，再执行 `deobfuscate-filenames`。确认两个阶段的 `mappings` 和 `warnings` 后移除 `--dry-run`，写出新 EPUB 并保存实际报告：

```sh
python3 scripts/epub_structure_tool.py normalize \
  "$EPUB" \
  --output work/after/step-0-normalized.epub \
  --report-format json > work/step-0-normalize.json
```

立刻把实际报告作为路径映射传给红线 gate：

```sh
python3 scripts/validate_text_invariance.py \
  "$EPUB" \
  work/after/step-0-normalized.epub \
  --check all \
  --path-map work/step-0-normalize.json
```

脚本只依赖 Python 标准库，不提供 DRM 解密。若加密声明目标在 ZIP 中不存在，报告会记录并移除 stale 引用；如果 `inspect` 已确认只有 EPUB 标准字体混淆，在红线命令额外添加 `--allow-font-obfuscation`。若写出了 step-0 产物，后续 EPUB3 迁移基线使用该产物。

结构规范化后再次运行 preflight。如果 CSS 仍引用 ZIP 中不存在的 `.ttf`、`.otf`、`.woff` 或 `.woff2`，harness 会记录 `missing-css-font-fallback` 警告：不要猜测该别名对应哪个嵌入字体，也不要自动删掉声明，保留 `local()` fallback 并人工复核。图片、样式等非字体资源断链仍是阻断错误。

## 2. EPUB3 迁移基线

如果 preflight 发现 `package_version` 不是 `3.0`，或缺少 `properties="nav"` 的 nav item，先生成 EPUB3 基线：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

python3 scripts/epub3_migration_harness.py \
  "$BASE" \
  --write-output work/after/step-1-epub3.epub \
  --format json > work/epub3-migration.json
```

迁移 harness 只做 package 层保守变更：OPF `version="3.0"`、`dcterms:modified`、必要时生成 `nav.xhtml` 并加入 manifest。它保留 `toc.ncx` 和 `spine toc="ncx"`，不改正文 XHTML。

迁移后立刻跑红线：

```sh
BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

python3 scripts/validate_text_invariance.py \
  "$BASE" \
  work/after/step-1-epub3.epub \
  --check text,metadata,spine,cover,anchors \
  --allow-list '*/nav*.xhtml'
```

如果当前基线已经是 EPUB3 且 nav 正常，本步可跳过。后续优先使用 step-0 产物，没有 step-0 才回退到 `work/before/source.epub`。

## 3. 精排建议 harness

对当前基线跑精排建议：

```sh
BASE=work/after/step-1-epub3.epub
test -f "$BASE" || BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub

python3 scripts/epub_refinement_harness.py "$BASE" --format json > work/refinement.json
python3 scripts/epub_ai_harness.py --mode cleanup "$BASE" --format json > work/findings.json
```

`refinement.json` 会按阶段提示是否需要：

- EPUB3 迁移；
- 标准 popup footnote 转换；
- 多字体排版 / 内嵌字体策略；
- 图片格式转换、封面声明、figure 版式；
- Ruby / 竖排专项复查；
- diff review 与红线 gate。

细节见 [refinement-harnesses.md](refinement-harnesses.md)。

## 4. 红线预检

参照 [SPEC §10.1](../final/SPEC-实现约束.md) 列出本次清洗可能触发哪些红线。

## 5. harness 扫描

```sh
BASE=work/after/step-1-epub3.epub
test -f "$BASE" || BASE=work/after/step-0-normalized.epub
test -f "$BASE" || BASE=work/before/source.epub
python3 scripts/epub_ai_harness.py --mode cleanup "$BASE" --format json > work/findings.json
cat work/findings.json | jq .recommended_skills
```

如果已经在 §3 生成 `work/findings.json`，这里直接读取即可。

## 6. 分派清洗

按 `recommended_skills` 顺序逐一执行。每次改动后跑：

```sh
REDLINE_BASE=work/after/step-1-epub3.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/after/step-0-normalized.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/before/source.epub
python3 scripts/validate_text_invariance.py "$REDLINE_BASE" work/after/step-N.epub --check all
```

退出码 1 立即回滚该次 skill 改动。

## 7. 文本校验（自动 gate）

```sh
REDLINE_BASE=work/after/step-1-epub3.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/after/step-0-normalized.epub
test -f "$REDLINE_BASE" || REDLINE_BASE=work/before/source.epub
python3 scripts/validate_text_invariance.py "$REDLINE_BASE" work/after/cleaned.epub --check all
```

退出码必须 0。

## 8. Diff 人工 review

按 [EPUB diff review](epub-diff-review.md) 的两条路径做：

- 主路径（推荐）：Calibre Editor → Tweak Book → File → Compare to another book → 选 `work/after/cleaned.epub`。
- 精细路径：`unzip` 解压两侧到 `work/before-extracted` / `work/after-extracted`，再用 `git diff --no-index` 整树概览 / `code --diff` 逐文件 / `shasum -a 256` 列表对资源层。
- 五层覆盖：结构 / 文本 / 样式 / 资源 / 元数据。文本红线已在 §5 卡过，本步只确认人眼看到的改动与红线放行的清洗范围一致。

这一步只看文件差异，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。

### 记录 review 决策

把值得跨书复用的判断追加到仓库级记录：

```sh
uv run python scripts/epub_decision_log.py add \
  --file records/typeset-decisions.jsonl \
  --scene image-layout \
  --finding lone-image-no-figure \
  --candidates figure.img-left,figure.img-right,figure-fullwidth \
  --chosen figure.img-right \
  --rationale "说明选择理由，不粘贴正文" \
  --scope global \
  --source manual-review
```

只属于当前书的判断写到 `work/<book>/reports/decisions.json`，把同一命令的 `--file` 指向该路径并使用 `--scope book`。两层记录都禁止保存正文文本；完整 schema 和隐私红线见 [`records/README.md`](../../records/README.md)。

## 9. 用户确认

把 diff 摘要、截图或导出 JSON 发给用户。用户确认后，`work/after/cleaned.epub` 作为交付。

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
| 红线触发数 | `validate_text_invariance.py --check all` | 必须 0 |
| 黄线条数 | Calibre Compare 文件树 modified 计数（或 `git diff --no-index --stat` 行数） | 记录 |
| EPUB 包结构错误数（after） | `scripts/epub_lint.py` | 清零，或逐条记录豁免理由 |
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
python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub --format json | jq '.findings[] | select(.kind == "ocr-residual")'
```

如果检测到，建议回到 `epub-source-intake`，重新 OCR 后再清洗。

## 16. 标准 `notes.md` 模板

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
- epub_lint：N error / N warning

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
python3 scripts/validate_text_invariance.py <redline-base.epub> after/cleaned.epub --check all
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
python3 scripts/validate_text_invariance.py \
  templates/cleanup-demo-books/dist/city-field-notes-before.epub \
  templates/cleanup-demo-books/dist/city-field-notes-after-clean.epub \
  --check all

python3 scripts/validate_text_invariance.py \
  templates/cleanup-demo-books/dist/paper-garden-before.epub \
  templates/cleanup-demo-books/dist/paper-garden-after-clean.epub \
  --check all
```

红线反例：

```sh
python3 scripts/validate_text_invariance.py \
  templates/cleanup-demo-books/dist/redline-trap-before.epub \
  templates/cleanup-demo-books/dist/redline-trap-after-text-changed.epub \
  --check all
```

前两条必须通过；反例必须失败。

## 18. 自动循环清洗（`epub_cleanup_loop.py`）

在 `epub_cleanup_pipeline.py` 的一命令一键纪律之上，本命令增加**确定性多轮自动收敛**：入口会先保留不可修改的 before、执行原始输入 preflight，并在只缺 MathML / SVG manifest `properties` 时先做可审计的安全包清单修复；随后复用单次流水线完成 preflight、可选结构规范化和 EPUB3 迁移，再以迁移后的 EPUB3 作为不可变文本基线。每轮由 Planner 产出受白名单约束的 Action，脚本执行后立即跑正文红线 gate 和本仓 lint；任一 gate 失败都会回滚到上一锚点，收敛或触上限自动停机。

**默认零模型：**

```sh
python3 scripts/epub_cleanup_loop.py /path/book.epub --work-dir work/book-a
```

默认 `--planner rules` 不调用任何模型，纯标准库，可离线/气隙运行。结构规范化仍保持显式批准：需要时先用 `--normalize dry-run` 检查报告，再在新的工作目录以 `--normalize apply --approve-normalize` 执行。脚本只会做**确定性可判定**的改动：

- 补充缺失的 `xml:lang`（lane ② 甲，默认开启）
- `epub:type` 加注仅 handshake 模式按 AI 计划执行（rules 模式不臆测语义角色）
- class 值重命名（lane ② 甲，需提供 mapping）
- 为含内联 MathML / SVG 的 XHTML 补齐 OPF manifest `properties`（package lane，只改包清单，不碰正文）

需要结构改写时显式开启：

```sh
python3 scripts/epub_cleanup_loop.py /path/book.epub \
  --work-dir work/book-a \
  --enable-structural
```

`--enable-structural` 为预留开关；结构改写白名单（如 `div.quote → blockquote`）尚未接入 detector，开启后当前不产生结构动作。后续若接入 detector，结构动作仍必须遵守 lane ② 乙' 的正文文本逐字不动红线。

**AI 辅助模式（handshake）：**

```sh
python3 scripts/epub_cleanup_loop.py /path/book.epub \
  --work-dir work/book-a \
  --planner handshake
```

工具在每轮写出 `plan-request.json` 后暂停，等待本地 AI host 填入 `plan.json` 再继续。工具自身从不主动联网；AI 每轮所见与所提全部落盘在 `reports/`，可审计。

**报告三分类：**

清洗结束后输出 Markdown 报告，分三栏：

- ✅ **已自动改**：脚本已执行并通过红线验证的改动
- 💡 **建议你改**：排版优化建议（不进自动循环）
- 👁 **需人工校对 / 实测**：需人工判断或阅读器实测的项

**与 `epub_cleanup_pipeline.py` 的关系：**

`epub_cleanup_pipeline.py` 是单次一键入口（preflight → 迁移 → 精排 → 红线 → 报告）；`epub_cleanup_loop.py` 先复用该入口建立干净 EPUB3 基线，再增加多轮 Planning-Execution-Gate 循环，适合「脏书扔进去，一条命令跑到收敛」。循环收尾会在 OPF metadata 写入 `epub-handbook:cleanup-rounds` 审计标记。两者共享 preflight、红线 gate 和本仓 lint 等组件，不互相替代。

**循环架构要点**：循环、收敛、gate、回滚全在 Python（确定、可测、可 CI）。每轮 Planner 产 Action 计划 → 脚本执行 → 红线 gate 验证。收敛判据：连续 `DRY_LIMIT=2` 轮无新动作或达到 `MAX_ROUNDS=6` 硬上限即停机。AI 全程不碰正文文字，只通过 `handshake` 模式产 JSON 计划（脚本写 `plan-request.json` → 外部 AI host 填 `plan.json` → 脚本读回执行），默认 `rules` 模式零模型调用。

### 18.1 模型与隐私（说明）

本工具的清洗主体是**确定性脚本**，AI 只是辅助——默认 `--planner rules` **完全不调用任何模型**，纯标准库、可离线/气隙运行，稿件不出本机。

⚠️ **风险提示**：出版社及专业制作团队的稿件常涉及机密与版权。把正文交给**云端大模型**存在泄露风险。

✅ **推荐**：确需 AI 辅助判断时，使用**本地部署的大模型**，通过 `--planner handshake` 在本机内完成「`plan-request.json` → `plan.json`」握手；工具自身从不主动联网，AI 每轮所见与所提全部落盘在 `reports/`，可审计。
