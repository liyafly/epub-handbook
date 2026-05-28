# EPUB 清洗流水线

> 状态：流程文档；用于把一本已存在的 EPUB 收拾干净。
> 对应 SPEC：[§10 AI 改动边界](../final/SPEC-实现约束.md)。
> 对应工具：`scripts/epub_ai_harness.py`、`scripts/validate_text_invariance.py`、外部 diff 工具（Calibre / VS Code，见 [../../README.md#epub-diff-review](../../README.md#epub-diff-review)）。

## 整体流程

```text
0. 准备 -> 1. 健康检查 -> 2. 红线预检 -> 3. harness 扫描 ->
4. 分派清洗 -> 5. 文本校验 -> 6. diff 人工 review ->
7. 用户确认 -> 8. reader-matrix 回写
```

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
which epubcheck >/dev/null && epubcheck "$EPUB" || echo "epubcheck not installed; skip"
```

输出 `META-INF/encryption.xml` 即有 DRM，立即停止。

## 2. 红线预检

参照 [SPEC §10.1](../final/SPEC-实现约束.md) 列出本次清洗可能触发哪些红线。

## 3. harness 扫描

```sh
python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub --format json > work/findings.json
cat work/findings.json | jq .recommended_skills
```

## 4. 分派清洗

按 `recommended_skills` 顺序逐一执行。每次改动后跑：

```sh
python3 scripts/validate_text_invariance.py work/before/source.epub work/after/step-N.epub --check all
```

退出码 1 立即回滚该次 skill 改动。

## 5. 文本校验（自动 gate）

```sh
python3 scripts/validate_text_invariance.py work/before/source.epub work/after/cleaned.epub --check all
```

退出码必须 0。

## 6. Diff 人工 review

按 [../../README.md#epub-diff-review](../../README.md#epub-diff-review) 的两条路径做：

- 主路径（推荐）：Calibre Editor → Tweak Book → File → Compare to another book → 选 `work/after/cleaned.epub`。
- 精细路径：`unzip` 解压两侧到 `work/before-extracted` / `work/after-extracted`，再用 `git diff --no-index` 整树概览 / `code --diff` 逐文件 / `shasum -a 256` 列表对资源层。
- 五层覆盖：结构 / 文本 / 样式 / 资源 / 元数据。文本红线已在 §5 卡过，本步只确认人眼看到的改动与红线放行的清洗范围一致。

这一步只看文件差异，不是阅读器效果验收。阅读器效果通过 reader-matrix 单独覆盖。

## 7. 用户确认

把 diff 摘要、截图或导出 JSON 发给用户。用户确认后，`work/after/cleaned.epub` 作为交付。

## 8. reader-matrix 回写

如果清洗涉及阅读器兼容性变更，在 `docs/final/reader-matrix.yaml` 增条目，初始 status 一律 `warn`，直到有真实阅读器实测。

## 9. 批量模式

一次处理多本 epub 时，每本一个工作目录：

```sh
ls /path/to/books/*.epub | parallel -j 4 ./clean-one.sh {}
```

失败的写入 `failed.log`，跑完后重试：

```sh
cat failed.log | parallel -j 4 ./clean-one.sh {}
```

单批次建议不超过 50 本，方便人工 review。

## 10. 回滚剧本

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

## 11. 可信度评估

| 指标 | 来源 | 期望 |
| --- | --- | --- |
| 红线触发数 | `validate_text_invariance.py --check all` | 必须 0 |
| 黄线条数 | Calibre Compare 文件树 modified 计数（或 `git diff --no-index --stat` 行数） | 记录 |
| epubcheck error 数（after） | `epubcheck` | 不多于 before |
| 阅读器兼容性回归 | reader-matrix 复测 | 不变差 |

结论：

- 红线 0 + 必须 review 项 0 + epubcheck 不变差 -> 自动通过。
- 红线 0 + 有必须 review 项 -> 人工 review。
- 红线 > 0 -> 重做。

## 12. 错误恢复

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

## 13. OCR-style 脏 epub 识别

特征：

- 章节几乎全是 `<img>` 引用，少量散乱文本。
- 文本有大量 OCR 噪点。
- 文件名常带 `scan` / `ocr` / `_p001`。

判定：

```sh
python3 scripts/epub_ai_harness.py --mode cleanup work/before/source.epub --format json | jq '.findings[] | select(.kind == "ocr-residual")'
```

如果检测到，建议回到 `epub-source-intake`，重新 OCR 后再清洗。

## 14. 标准 `notes.md` 模板

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
- epubcheck：N error / N warning

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
python3 scripts/validate_text_invariance.py before/source.epub after/cleaned.epub --check all
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

## 15. 自造 demo

首轮端到端演示不依赖公版书。先生成仓库自造样本：

```sh
bash samples/demo-books/build.sh
```

合法清洗对：

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/city-field-notes-before.epub \
  samples/demo-books/dist/city-field-notes-after-clean.epub \
  --check all

python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/paper-garden-before.epub \
  samples/demo-books/dist/paper-garden-after-clean.epub \
  --check all
```

红线反例：

```sh
python3 scripts/validate_text_invariance.py \
  samples/demo-books/dist/redline-trap-before.epub \
  samples/demo-books/dist/redline-trap-after-text-changed.epub \
  --check all
```

前两条必须通过；反例必须失败。
