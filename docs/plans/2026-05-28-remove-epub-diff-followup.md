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

对照 [2026-05-28-remove-epub-diff.md](./2026-05-28-remove-epub-diff.md) 全部 21 个 Task 与自检 checklist，远端 `codex/sync-local-changes-to-cloud` 上的 commit `473485d` 的覆盖情况如下。

### 1.1 已完成（内容性 Task 全部通过）

见主计划 follow-up 审核记录；本次补完仅修复计划文件漏提问题。

### 1.2 未完成（1 项遗漏）

**主计划文件本身没有被提交到远端分支**：

- 本地 `docs/plans/2026-05-28-remove-epub-diff.md` 一直是 untracked 状态。
- 该文件未出现在之前提交的 file list 里，也不在目标远端 tree 中。
- 同时已有 4 处引用指向该不存在文件，形成死链。

## 2. 目标

把主计划文件与本 follow-up 文件都补提到目标分支，消除 4 处死链，并把自检 checklist 跑 clean。

## 3. 文件变更清单

| # | 类型 | 路径 | 动作 |
| --- | --- | --- | --- |
| 1 | 新增 | `docs/plans/2026-05-28-remove-epub-diff.md` | 把本地 untracked 文件 `git add` + commit |
| 2 | 新增 | `docs/plans/2026-05-28-remove-epub-diff-followup.md` | 本文件，一并 commit |
| 3 | 修改（可选） | `docs/plans/README.md` | 在「当前计划」下追加本 follow-up 的一行注册 |
