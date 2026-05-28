# Follow-up：`codex/sync-local-changes-to-cloud` 补完计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 `superpowers:executing-plans` 逐 Task 执行。每个 step 都用 `- [ ]` checkbox 标记，按顺序勾完再进下一个 Task。
>
> **作者**：Claude（Opus 4.7）
>
> **日期**：2026-05-28
>
> **针对分支**：`origin/codex/sync-local-changes-to-cloud`（commit `473485d`）
>
> **关联主计划**：[2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)

## 1. Review 结论

对照 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md) 全部 21 个 Task 与自检 checklist，远端 `codex/sync-local-changes-to-cloud` 上的 commit `473485d` 的覆盖情况如下：

### 1.1 已完成（内容性 Task 全部通过）

| Task | 计划内容 | 远端落地 |
| --- | --- | --- |
| Task 2 | `git rm -r tools` | ✅ 19 个 tracked 文件全部删除（含 `index.html`、`app.js`、`parsers/*`、`layers/*`、`render/*`、`util/*`、`assets/*`、`scripts/fetch-vendor.sh`） |
| Task 3 | `git rm docs/pipeline/diff-tool.md` | ✅ 已删除 |
| Task 4 | 重写 README（180–230 行、≥10 个 `## `、Calibre ≥5、`git diff --no-index` ≥3、`epub-diff-review` ≥2） | ✅ 215 行 / 12 个 `## ` / Calibre×9 / `git diff --no-index`×7 / `epub-diff-review`×2 |
| Task 5 | `docs/README.md` 改链接 | ✅ |
| Task 6 | `docs/pipeline/cleanup-flow.md` §6 + §11 黄线指标 | ✅ |
| Task 7 | `docs/pipeline/decisions.md` Q5 划线 + 新增 Q7/Q8/Q9/Q10 | ✅ |
| Task 8 | `01-first-epub.md` / `04-skills.md` / `05-case-study.md` | ✅ |
| Task 9 | `06-test-your-own.md` §3 | ✅ |
| Task 10 | `07-faq.md` Node.js + Diff 工具段 | ✅ |
| Task 11 | `docs/getting-started/README.md` | ✅ |
| Task 12 | `docs/final/SPEC-实现约束.md` §10.2 / §10.4 / §10.5 | ✅ lines 221 / 245 / 263 都已替换为「外部 diff 工具」+ README 锚点 |
| Task 13 | `samples/demo-books/README.md` | ✅ |
| Task 14 | `samples/fixtures-tiny/README.md` | ✅ |
| Task 15 | `skills/epub-layout-auditor/SKILL.md` 第 14 行 | ✅ |
| Task 16 | `THIRD_PARTY.md` 删 zip.js | ✅ |
| Task 17 | `CLAUDE.md` 优先级第 9 条 + 关键约束最后一条 | ✅（删除线 + 替换措辞） |
| Task 18 | 历史 plan banner（`handbook-expansion-plan.md` Stage 3、`handbook-expansion-review.md` 顶部） | ✅ |
| Task 19 | `docs/plans/README.md` 注册本计划 | ✅ |
| Task 20 | 全仓 grep `tools/epub-diff` / `epub_diff` 残留 | ✅ 除允许的历史档外 clean |
| Task 21 | PR / 推送 | ✅ 已推到 `origin/codex/sync-local-changes-to-cloud`（分支名与计划中 `chore/remove-tools-and-enrich-readme` 不同，但不影响交付） |

### 1.2 未完成（1 项遗漏）

**主计划文件本身没有被提交到远端分支**：

- 本地 `docs/plans/2026-05-28-remove-epub-diff.md` 一直是 untracked 状态（`git status --porcelain` 显示 `?? docs/plans/2026-05-28-remove-epub-diff.md`）。
- 该文件未出现在 commit `473485d` 的 file list 里，也不在 `origin/codex/sync-local-changes-to-cloud` 的 tree 中。
- 与此同时，远端分支已有 **4 处**引用指向这个不存在的文件，全部成为死链：

| 引用位置 | 引用片段 |
| --- | --- |
| `docs/plans/README.md` | `` - `2026-05-28-remove-epub-diff.md`：移除整个 `tools/` 目录…执行计划 `` |
| `docs/plans/handbook-expansion-plan.md`（Stage 3 banner） | `执行记录见 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)` |
| `docs/plans/handbook-expansion-review.md`（顶部 banner） | `见根 README 的 #epub-diff-review 段与 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md)` |
| `docs/pipeline/decisions.md`（2026-05-28 决策段末尾） | `执行记录见 [../plans/2026-05-28-remove-epub-diff.md](../plans/2026-05-28-remove-epub-diff.md)` |

主计划 §3 文件变更清单的 #22 行写「`docs/plans/2026-05-28-remove-epub-diff.md` — 本文件（已存在）」，前提是执行前这份计划已被 commit。实际执行时它仅是 untracked 文件，因此被漏掉。

## 2. 目标

把主计划文件与本 follow-up 文件都补提到 `codex/sync-local-changes-to-cloud`（或开新分支 rebase），消除 4 处死链，并把自检 checklist 跑 clean。

## 3. 文件变更清单

| # | 类型 | 路径 | 动作 |
| --- | --- | --- | --- |
| 1 | 新增 | `docs/plans/2026-05-28-remove-epub-diff.md` | 把本地 untracked 文件 `git add` + commit |
| 2 | 新增 | `docs/plans/2026-05-28-remove-epub-diff-followup.md` | 本文件，一并 commit |
| 3 | 修改（可选） | `docs/plans/README.md` | 在「当前计划」下追加本 follow-up 的一行注册 |

## Task 1：基线确认

**Files:** 仅 git 操作。

- [ ] **Step 1：确认当前位置**

```sh
cd /path/to/epub-handbook
git status --porcelain
git branch --show-current
```
Expected: `?? docs/plans/2026-05-28-remove-epub-diff.md` 与 `?? docs/plans/2026-05-28-remove-epub-diff-followup.md` 至少之一在输出中；当前分支应是 `codex/handbook-expansion-pipeline-diff` 或后续操作分支。

- [ ] **Step 2：确认远端已无 plan 文件**

```sh
git fetch origin codex/sync-local-changes-to-cloud
git ls-tree origin/codex/sync-local-changes-to-cloud docs/plans/ | grep "2026-05-28" || echo "missing on remote"
```
Expected: `missing on remote`。如果输出含 `2026-05-28-remove-epub-diff.md` 行，说明上游已被别人补上，跳到 Task 4 验证即可。

- [ ] **Step 3：决定操作分支**

二选一：

- **方案 A（推荐）**：直接在 `codex/sync-local-changes-to-cloud` 上加一个补丁 commit。
  ```sh
  git checkout codex/sync-local-changes-to-cloud
  git pull --ff-only origin codex/sync-local-changes-to-cloud
  ```
- **方案 B**：开新分支 `chore/restore-plan-doc` 走 PR。
  ```sh
  git checkout -b chore/restore-plan-doc origin/codex/sync-local-changes-to-cloud
  ```

如果上游有保护分支策略，必须用 B；否则 A 简洁。

## Task 2：提交主计划文件

**Files:**
- Add: `docs/plans/2026-05-28-remove-epub-diff.md`

- [ ] **Step 1：核对文件存在且内容完整**

```sh
test -f docs/plans/2026-05-28-remove-epub-diff.md && echo "exists" || echo "MISSING"
wc -l docs/plans/2026-05-28-remove-epub-diff.md
grep -c "^## Task " docs/plans/2026-05-28-remove-epub-diff.md
```
Expected: `exists`；行数约 1400+；`grep -c "^## Task "` 输出 ≥ 21。

- [ ] **Step 2：把 4 处死链对照 plan 文件路径**

```sh
grep -nF "2026-05-28-remove-epub-diff.md" \
  docs/plans/README.md \
  docs/plans/handbook-expansion-plan.md \
  docs/plans/handbook-expansion-review.md \
  docs/pipeline/decisions.md
```
Expected: 4 处命中，路径相对正确（`./` 或 `../plans/`）。

- [ ] **Step 3：`git add` 并 commit**

```sh
git add docs/plans/2026-05-28-remove-epub-diff.md
git commit -m "docs(plans): restore 2026-05-28 remove-epub-diff plan file

The execution plan that drove the 'remove tools/' commit (473485d) was
never committed — it existed only as an untracked file locally. Four
documents on this branch already link to it (plans/README.md,
handbook-expansion-plan.md, handbook-expansion-review.md,
pipeline/decisions.md), all of which became dead links once the branch
was pushed. Restoring the file fixes those links and preserves the
review + per-task execution log."
```

## Task 3：提交本 follow-up 文件

**Files:**
- Add: `docs/plans/2026-05-28-remove-epub-diff-followup.md`

- [ ] **Step 1：核对文件存在**

```sh
test -f docs/plans/2026-05-28-remove-epub-diff-followup.md && echo "exists" || echo "MISSING"
```

- [ ] **Step 2：commit**

```sh
git add docs/plans/2026-05-28-remove-epub-diff-followup.md
git commit -m "docs(plans): add follow-up plan covering missing plan-file commit"
```

## Task 4（可选）：在 `docs/plans/README.md` 注册本 follow-up

跳过亦可，下面这步只是让索引完整。

**Files:**
- Modify: `docs/plans/README.md`

- [ ] **Step 1：在主计划注册行下方追加**

把：
```markdown
- `2026-05-28-remove-epub-diff.md`：移除整个 `tools/` 目录、把 diff workflow 写进根 README、丰富 README 的 review + 执行计划
```
改成（追加一行）：
```markdown
- `2026-05-28-remove-epub-diff.md`：移除整个 `tools/` 目录、把 diff workflow 写进根 README、丰富 README 的 review + 执行计划
- `2026-05-28-remove-epub-diff-followup.md`：上一条计划落地后补提缺失的主计划文件、修复 4 处死链
```

- [ ] **Step 2：commit**

```sh
git add docs/plans/README.md
git commit -m "docs(plans): register 2026-05-28 remove-epub-diff follow-up"
```

## Task 5：链接校验

**Files:** 仅校验，无修改。

- [ ] **Step 1：4 处引用现在都能解析**

```sh
for f in docs/plans/README.md docs/plans/handbook-expansion-plan.md docs/plans/handbook-expansion-review.md docs/pipeline/decisions.md; do
  if grep -qF "2026-05-28-remove-epub-diff.md" "$f"; then
    echo "[ref] $f"
  fi
done
test -f docs/plans/2026-05-28-remove-epub-diff.md && echo "[target] exists"
```
Expected: 4 行 `[ref] ...` + 1 行 `[target] exists`。

- [ ] **Step 2：再扫一遍主计划自检 checklist 的两个 grep（确认 follow-up 没引入新残留）**

```sh
grep -RIn "tools/epub-diff\|epub_diff" \
  --exclude-dir=.git \
  --exclude="2026-05-28-remove-epub-diff.md" \
  --exclude="2026-05-28-remove-epub-diff-followup.md" \
  --exclude="handbook-expansion-plan.md" \
  --exclude="handbook-expansion-review.md" \
  --exclude="decisions.md" \
  --exclude="CLAUDE.md" \
  . || echo "clean"
```
Expected: `clean`。

## Task 6：推送

**Files:** 无文件改动；纯 git。

- [ ] **Step 1：推送到远端**

方案 A（直接补到 `codex/sync-local-changes-to-cloud`）：
```sh
git push origin codex/sync-local-changes-to-cloud
```

方案 B（新分支走 PR）：
```sh
git push -u origin chore/restore-plan-doc
gh pr create --base main --head chore/restore-plan-doc \
  --title "docs(plans): restore missing 2026-05-28 plan file" \
  --body "Restores docs/plans/2026-05-28-remove-epub-diff.md, which was the execution plan for commit 473485d but was never committed (untracked locally). Four documents on this branch already link to it; this PR fixes the dead links."
```

- [ ] **Step 2：远端再校验一次**

```sh
git fetch origin
git ls-tree origin/codex/sync-local-changes-to-cloud docs/plans/ | grep 2026-05-28
```
Expected: 两行（主计划 + follow-up）。

## 自检 Checklist

- [ ] `docs/plans/2026-05-28-remove-epub-diff.md` 已成为 tracked 文件并 push 到远端。
- [ ] `docs/plans/2026-05-28-remove-epub-diff-followup.md`（本文件）已 push 到远端。
- [ ] 4 处引用（plans/README、handbook-expansion-plan、handbook-expansion-review、pipeline/decisions）的链接 hover 不再是死链。
- [ ] 全仓 grep `tools/epub-diff` / `epub_diff` 仅在允许的历史档出现（本计划、follow-up 自身、handbook-expansion-plan 正文、handbook-expansion-review 正文、decisions.md 删除线条目、CLAUDE.md 删除线条目）。

## 风险与回滚

| 风险 | 触发 | 回滚 |
| --- | --- | --- |
| 上游已被别人补上同名 plan 文件，内容不一致 | Task 1 Step 2 检测到 plan 文件已在远端 | 跳过 Task 2，把本地 untracked 文件与远端 diff 比较；以远端为基线，必要时合并 |
| `codex/sync-local-changes-to-cloud` 已被合并到 main 后才发现死链 | 这次补完滞后 | 在 main 上开 hotfix 分支补提 plan 文件，PR 走 main |
| 计划文件正文有过时表述（例如分支名 `chore/remove-tools-and-enrich-readme`） | review 时发现 | 计划文件作为**历史快照**保留；不在本 follow-up 内修改正文，仅恢复缺失文件 |
