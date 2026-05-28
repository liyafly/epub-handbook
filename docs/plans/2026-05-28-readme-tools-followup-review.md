# Review + 执行计划：README / tools 清理 follow-up

> 日期：2026-05-28
>
> 范围：复查「移除 `tools/` + 丰富根 README + diff review 迁移到 Calibre / VS Code」这一轮改动在当前仓库里的剩余收尾项。

## 1. Review 结论

这轮主体工作已经完成：

- Git 层面已没有 tracked `tools/` 文件：`git ls-files tools` 输出为空。
- 根 `README.md` 已扩展为单一入口文档：当前 215 行、12 个 `##`、`Calibre` 出现 9 次、`git diff --no-index` 出现 7 次、`epub-diff-review` 锚点出现 2 次。
- `docs/plans/2026-05-28-remove-epub-diff.md` 与 follow-up 计划文件都已成为 tracked 文件。
- 基础脚本健康检查通过：
  - `python3 scripts/validate_skills_basic.py`
  - `python3 scripts/test_validate_text_invariance.py`
  - `python3 scripts/test_epub_ai_harness.py`

仍需补完的不是主体功能，而是仓库清洁度与文档链接一致性。

## 2. 未完成项

### P0：活跃文档还有 3 个真实断链

读者会实际点击到这些链接，不属于历史快照：

| 文件 | 当前问题 | 建议处理 |
| --- | --- | --- |
| `README.md` | `agents/openai.yaml` 指向仓库根，但根目录没有该文件 | 改为指向 `skills/README.md`，并在文字里说明 metadata 位于 `skills/*/agents/openai.yaml` |
| `docs/README.md` | 仍列出已删除的 `pipeline/diff-tool.md` | 改为「EPUB diff review：根 README #epub-diff-review」或直接删除该条 |
| `docs/pipeline/README.md` | 仍列出已删除的 `diff-tool.md` | 同步改成根 README 锚点，不再叫 Web App 使用说明 |

本次断链扫描命令：

```sh
python3 - <<'PY'
from pathlib import Path
import re, urllib.parse
root = Path('.')
mds = [p for p in root.rglob('*.md') if '.git' not in p.parts and p.parts[0] not in {'_epub_reference'}]
link_re = re.compile(r'(?<!\!)\[[^\]]*\]\(([^)]+)\)')
bad = []
for p in mds:
    text = p.read_text(encoding='utf-8', errors='ignore')
    for m in link_re.finditer(text):
        raw = m.group(1).strip()
        if not raw or raw.startswith(('#', 'http://', 'https://', 'mailto:')):
            continue
        if raw.startswith('<') and raw.endswith('>'):
            raw = raw[1:-1]
        target = urllib.parse.unquote(raw.split('#', 1)[0])
        if not target:
            continue
        t = (p.parent / target).resolve()
        if not t.exists():
            line = text.count('\n', 0, m.start()) + 1
            bad.append((p, line, raw))
for p, line, raw in bad:
    print(f'{p}:{line} {raw}')
print('TOTAL', len(bad))
PY
```

注意：`docs/plans/2026-05-28-remove-epub-diff.md`、`docs/plans/handbook-expansion-plan.md`、`docs/plans/handbook-expansion-review.md` 内的旧 `tools/epub-diff` 链接大多是历史快照正文，不建议作为 P0 直接改写；这些文件顶部已有 banner 说明历史内容已作废。

### P1：本机仍有空 `tools/` 目录

当前状态：

```text
tools
tools/.DS_Store
```

它没有进入 Git，因为 `.DS_Store` 被 `.gitignore` 忽略；所以远端不会保留这个目录。但如果目标是「当前工作区也干净」，应删除本地空目录。

### P1：follow-up 计划文件正文重复

`docs/plans/2026-05-28-remove-epub-diff-followup.md` 在第一份正文结束后，又从第二个 `# Follow-up...` 重新开始了一遍；第二个标题当前还贴在上一行表格末尾。这个文件已经 tracked，但重复内容会误导后续 review，也会让计划目录显得不干净。

建议只保留第一份完整正文，删除重复的第二份；同时可以顺手去掉不适用于本仓的外部路径示例，例如 `/Users/yafeili/Developer/epub-handbook`。

### P2：`docs/plans/README.md` 归档段落此前自相矛盾

本次落地执行文档时已顺手处理：原先「已归档」段落写了「暂无」，下面又列出 2 个已归档计划；现在已删除「暂无」句，并把本文件登记到「当前计划」。

### P2：历史计划断链需要建立审查口径

历史计划里保留旧路径是合理的，但一旦开始做 Markdown link check，需要明确排除规则，否则每次都会报一批已知历史断链。

建议口径：

- 活跃入口文档必须无断链：`README.md`、`docs/README.md`、`docs/pipeline/README.md`、`docs/getting-started/`、`docs/final/`、`skills/`、`samples/`。
- 历史快照文档允许保留旧链接，但文件顶部必须有「历史快照 / 已作废」说明。
- 如果未来新增 `scripts/check_markdown_links.py`，默认排除 `docs/plans/handbook-expansion-plan.md`、`docs/plans/handbook-expansion-review.md` 和 `docs/plans/2026-05-28-remove-epub-diff.md` 正文中的历史链接。

## 3. 执行计划

### Task 1：修复活跃断链

修改：

- `README.md`
- `docs/README.md`
- `docs/pipeline/README.md`

建议改法：

```diff
- | 给 AI 接入 | [skills/](skills/) + [`agents/openai.yaml`](agents/openai.yaml) |
+ | 给 AI 接入 | [skills/README.md](skills/README.md)；各 skill 的 UI metadata 在 `skills/*/agents/openai.yaml` |
```

```diff
- [pipeline/diff-tool.md](pipeline/diff-tool.md)：EPUB Diff 工具
+ EPUB diff review：见 [根 README #epub-diff-review](../README.md#epub-diff-review)（Calibre / VS Code）
```

`docs/pipeline/README.md` 中也把 `diff-tool.md` 一行替换为根 README 锚点。

验收：

```sh
rg -n "diff-tool\.md|tools/epub-diff|epub_diff|\\(agents/openai\\.yaml\\)" README.md docs/ skills/ samples/ CLAUDE.md THIRD_PARTY.md \
  -g '!docs/plans/2026-05-28-remove-epub-diff.md' \
  -g '!docs/plans/2026-05-28-remove-epub-diff-followup.md' \
  -g '!docs/plans/2026-05-28-readme-tools-followup-review.md' \
  -g '!docs/plans/handbook-expansion-plan.md' \
  -g '!docs/plans/handbook-expansion-review.md'
```

预期：活跃文档里不再出现 `diff-tool.md` 与根路径 `(agents/openai.yaml)`；`docs/pipeline/decisions.md` 里的删除线历史记录可保留。

### Task 2：删除本地空 `tools/`

确认只有忽略文件：

```sh
find tools -maxdepth 2 -print
git ls-files tools
```

如果 `git ls-files tools` 为空，执行：

```sh
rm -rf tools
```

验收：

```sh
test -d tools && echo "STILL EXISTS" || echo "removed"
git status --short
```

预期：`tools/` 不存在；`git status` 不因为这一步产生 tracked diff。

### Task 3：清理重复的 follow-up 计划

修改：

- `docs/plans/2026-05-28-remove-epub-diff-followup.md`

动作：

- 删除第二份重复正文，从第二个 `# Follow-up` 开始到文件末尾。
- 保留第一份 review 结论、任务列表、自检 checklist 和风险表。
- 可选：把旧机器路径替换为当前仓库路径，或删除路径示例。

验收：

```sh
grep -o '# Follow-up' docs/plans/2026-05-28-remove-epub-diff-followup.md | wc -l
wc -l docs/plans/2026-05-28-remove-epub-diff-followup.md
```

预期：标题计数为 `1`；行数显著低于当前 525 行。

### Task 4：确认 plans 索引归档段

修改：

- `docs/plans/README.md`

本文件创建时已完成以下动作：

- 删除「暂无。落地完成且无续做的计划，按需移到 `plans/archive/`。」里的「暂无」判断。
- 保留 `2026-05-28-remove-epub-diff.md` 与 `2026-05-28-remove-epub-diff-followup.md` 两条归档记录。
- 把本文件加入「当前计划」，等执行完成后再移到「已归档」。

### Task 5：最终验证

运行：

```sh
python3 scripts/validate_skills_basic.py
python3 scripts/test_validate_text_invariance.py
python3 scripts/test_epub_ai_harness.py

git ls-files tools
test -d tools && echo "STILL EXISTS" || echo "removed"

rg -n "diff-tool\.md|tools/epub-diff|epub_diff|\\(agents/openai\\.yaml\\)" README.md docs/ skills/ samples/ CLAUDE.md THIRD_PARTY.md \
  -g '!docs/plans/2026-05-28-remove-epub-diff.md' \
  -g '!docs/plans/2026-05-28-remove-epub-diff-followup.md' \
  -g '!docs/plans/2026-05-28-readme-tools-followup-review.md' \
  -g '!docs/plans/handbook-expansion-plan.md' \
  -g '!docs/plans/handbook-expansion-review.md'
```

验收标准：

- 3 个 Python 检查通过。
- `git ls-files tools` 为空。
- `tools/` 本地目录不存在。
- 活跃文档无 `diff-tool.md` 和根路径 `(agents/openai.yaml)` 断链。
- `docs/plans/2026-05-28-remove-epub-diff-followup.md` 不再重复。

## 4. 建议提交拆分

1. `docs: fix active README links after tools removal`
   - 修 `README.md`、`docs/README.md`、`docs/pipeline/README.md`
2. `docs(plans): clean duplicated remove-epub-diff follow-up`
   - 修 follow-up 计划和 `docs/plans/README.md`

本地删除空 `tools/` 不会进入 commit。
