# 执行记录

> 按 `README.md` 规则 8：**只记** `skipped(...)`、`blocked`、`deviated` 三类，每个任务 ID 一条；完成的任务把验证写进提交信息的 `Verified:` 段，不在这里重复。
> 不要删除或改写已有记录。全部做完后，把「收尾检查」的输出原样贴到本文件末尾。

<!-- 模板：
### <任务 ID>
- 状态：skipped(待裁决 Qn) | skipped(被 Sxx 覆盖) | blocked | deviated
- 原因：
- 证据：
  ```
  $ <命令>
  <最后几行输出与 exit code>
  ```
- 下一步：
-->

### H7
- 状态：blocked
- 原因：Q15=C 要保留第三方 EPUB，但根 `AGENTS.md` 要求已跟踪实体 EPUB 同时有保留理由和许可记录；H7 明确说明许可记录缺失，并禁止修改 AGENTS.md。当前证据不足以同时满足两项规则。
- 证据：`work/review-20260929/00-process.md` 的 H7 明确写有 “No license or permission record ... on file”；根 `AGENTS.md` 的第三方来源与实体 EPUB 维护规则要求许可记录。
- 下一步：提供权利方许可/授权记录，或由所有者另行决定是否修订仓库规则；再补齐 H7。

### H8
- 状态：skipped(目标 stash 不匹配；远端 tag 已一致)
- 原因：任务只授权清理原 `stash@{0}`（`b804550` / `WIP on main: 18a9a96`），当前 `stash@{0}` 是另一项用户工作；不删除。v0.4.1 的本地与远端 peeled commit 已相同，无需强制 fetch。
- 证据：`git stash list --format='%gd %H %s'` 输出 `stash@{0} e6dea8e7ff61fb509b3269ef135107424bdbda67 WIP on codex/go-cli-architecture-docs: 402ee73 新增横排章首页兼容样例`；`git rev-parse 'v0.4.1^{}'` 与 `git ls-remote origin 'refs/tags/v0.4.1^{}'` 均为 `1bb1cc3a05b25b8d5e2c57f35ef4c32290597386`。
- 下一步：若要清理 stash，先由所有者确认这个当前 stash 是否可删除；否则保留。

### C12
- 状态：skipped(被 S13 覆盖)
- 原因：裁决表规定 S13 删除 notes-fallback 的 `scope_paths`，C12 对该参数做的平行修复随之失去适用对象。
- 证据：`grep -nE '^\| Q21 ' work/review-20260929/06-decisions.md` -> Q21 仅保留第 6 项 `mode=inspect`。
- 下一步：无；按计划由 S13 删除该参数及其行为。

### S21
- 状态：skipped(Q27=B)
- 原因：所有者裁决暂不新增 `epub pack` 命令。
- 证据：`grep -nE '^\| Q27 ' work/review-20260929/06-decisions.md` -> Q27=B。
- 下一步：若重新考虑 `epub pack`，另开任务并重新评估其范围。

### R15
- 状态：blocked
- 原因：workflow pin 已提交并推送；验收明确要求由用户在 GitHub UI 从 `main` 手动 dispatch，以确认发布 job 被 tag 条件跳过且日志无 Node20 annotation。此项需要用户操作。
- 证据：本地 `actionlint`、SHA/tag 对照与输入名差分均通过；workflow 的 `publish-release` 仍由 `startsWith(github.ref, 'refs/tags/')` 条件保护。
- 下一步：用户在 GitHub UI 手动运行 Release CLI workflow，确认 verify 与 native smoke 全绿、publish-release 被跳过且无 Node20 annotation。

### R21
- 状态：deviated
- 原因：R14 已先修复默认 demo CSS，因此任务包给出的旧构建 SHA `0b2c8812…` 已不适合作为当前基准；当前源码 SHA 为 `f38f8de8…`。此外，基线 starter 脚本实际只有一条 `.DS_Store -delete`，不是文中说的两条；该条现已删除，因为 archive find 已排除所有 dotfile。
- 证据：过滤前无 sidecar SHA=`f38f8de8a054feebc129aa3c21eedf00891f8a841b38de4d09c20a4d93fed99c`、有 `.x.swp` SHA=`bb7281fdf129052a38114209137e356e930a084ac72c867e9e0201246e879447` 且 ZIP 含 1 项；过滤后无 sidecar / 有 sidecar / 当前源树构建 SHA 均为 `f38f8de8a054feebc129aa3c21eedf00891f8a841b38de4d09c20a4d93fed99c`，`swp_entries=0`。`git show 726ce18:templates/book-starter/build.sh | rg -n 'DS_Store -delete'` 仅命中一次。
- 下一步：无；当前同源基准下噪声文件不影响产物，R14 的 CSS 修复仍保留。

### S01 + G35
- 状态：deviated
- 原因：任务包给出的递归 `grep` 未排除根规则明确忽略的 `work-epub/` 与 `.git`，扫描到了用户书级工作区中的历史流水线报告和 Git fsmonitor socket；没有修改这些用户数据，改用只查 tracked source 的 `git grep` 验证。
- 证据：
  ```
  $ git grep -n -E 'epub\.layout\.audit|layoutAudit' -- . ':!archive/**' ':!CHANGELOG.md' ':!work/**'
  tracked_source_grep_exit=1 (1 means no matches)
  $ git diff --check
  exit 0
  ```
- 下一步：无；tracked source 的旧 capability 引用已清零。

### G30
- 状态：skipped(被 S06 覆盖)
- 原因：Q22=A 已删除 redline 的 legacy noteref 配对例外；G30 的收窄实现不再有适用对象。
- 证据：`grep -nE '^\| Q22 ' work/review-20260929/06-decisions.md` -> Q22=A；S06 删除 `ExtractTextBlocksWithLegacyNoterefPairing`，恢复普通 `ExtractTextBlocks`。
- 下一步：无；旧式 `[N]` 标记按普通正文块接受 redline 比较。

### G33
- 状态：skipped(被 S07 覆盖)
- 原因：Q19=B 删除 `epub clean` 的 typography 步骤及其范围选择入口；G33 所针对的 `--scope` 错映射路径已不存在。
- 证据：`grep -nE '^\| Q19 ' work/review-20260929/06-decisions.md` -> Q19=B；S07 验收检索确认 `resolveCleanScope` 与 `--preset` 已从实现、docs、skills 中删除。
- 下一步：无；排版能力改为逐本通过公开 capability 调用。

### S15
- 状态：deviated
- 原因：按 §0 运行既有字体 smoke 时，build 未发现 `.pipeline/build.*/src/OEBPS/Fonts/` 下的字体，因为 `find ... ! -path '*/.*/*'` 把隐藏的 `.pipeline` 祖先也当作源树内隐藏目录，导致字体子集流程被跳过。为使任务要求的 smoke 实际覆盖字体流程，将发现步骤改成相对源树路径并 prune 源树内的隐藏目录；未更改测试断言或降低构建检查。
- 证据：原路径过滤在 `.pipeline/build.123/src/OEBPS/Fonts/full.ttf` 上输出 0，按源树相对路径运行 prune 版本输出 1；修复后的 smoke 与三层验证结果记录在 S15 提交的 `Verified:` 段。
- 下一步：S15 与 F10 的 smoke 均使用修复后的 build 字体发现流程复验。

### X1
- 状态：deviated
- 原因：提交 `9ce8710` 在修复 legacy EPUB 验收时还改变了两个行为：生成的 nav 不再追加到 spine（避免把导航文档加入阅读顺序）；重写 `mimetype` 时不再写 ZIP `Modified` 时间，避免产生 OCF 禁止的 ZIP extra field。
- 证据：`git show 9ce8710 -- internal/caps/migrate_epub3/migrate_epub3.go internal/zipfs/zipfs.go`；`internal/zipfs/passthrough_test.go::TestRewrittenMimetypeHasNoExtraFields`；W3C EPUB 3.3 OCF §4.3.3 与 EPUBCheck `PKG_005` 规定/报告 mimetype extra field 不允许。
- 下一步：无；G32 已将 legacy 样本 normalize 映射空集写入说明和 CI 断言。

### R16
- 状态：deviated
- 原因：移入 PR 的 release smoke 首次运行发现任务包基线中的 capability 数量断言仍为 23；当前 CLI 因 S01 已删除 `epub.layout.audit`，实际注册 22 个 capability。为让 smoke 校验当前公开面且使 PR gate 可通过，将断言更新为 22。
- 证据：首次 outside-checkout smoke 失败并输出 `AssertionError: 22`；更新后输出 `release smoke passed: 0.4.6-dev darwin/arm64 (outside checkout)`，exit 0。GitHub workflow 固定运行目标为 linux/amd64。
- 后续 CI：GitHub run `36589506586` 的 Build EPUB Demo 在 Linux/amd64 成功，日志含 `release smoke passed: 0.4.6-dev linux/amd64 (outside checkout)`。

### D25
- 状态：skipped(被 S18 覆盖)
- 原因：D25 所指 README 清洗代码块已在 S18 中获批删除；当前 README 保留指向 cleanup-flow 的单行入口，不恢复已删命令块。D25 其余构建路径修正已完成。
- 证据：`git show c84129de -- README.md` 显示 S18 将「修一本现成 EPUB」改为 runbook 链接；当前该节没有旧命令块。
- 下一步：无。

### 收尾检查

```
go test ok
guards ok
```
