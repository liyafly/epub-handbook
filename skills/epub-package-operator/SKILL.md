---
name: epub-package-operator
description: 对 EPUB 执行单个明确的包操作：合并多本、按目录索引拆分、修改书名作者等元数据、替换封面。始终保留原文件并写出新 EPUB 与可审计 JSON 报告。
---

# EPUB 包操作

## 何时用

- 需要合并两本或多本 EPUB、按 TOC 索引拆分一本书、修改书名/作者等元数据、或替换封面，且必须保留原文件并输出新 EPUB 与可审计报告时。只执行用户明确选择的一个写操作；只检查 OPF/nav/NCX 时改用 `epub-package-nav-auditor`，不要把包结构审计和写操作混成自动修复。
- 操作语义：
  - 合并：至少两个输入（`--input` 为主输入，其余经 `extra_inputs=`），产出单一新 EPUB；可用 `title=` 重设合并后标题。资源冲突时按序改名并在报告的 `renamedResources` 中可追溯。
  - 拆分：切分点是当前书 TOC 目标（nav/NCX 条目；无目录时退化为 spine 顺序）的下标，`split_points` 取值范围 0 到目标数-1，越界会被拒绝；每段一个 `<stem>_<NN>.epub` 写入 `output_dir`，输出目录必须为空，非空会被拒绝。
  - 元数据：`metadata_json` 是内联 JSON 对象文本（不是文件路径），只接受字符串字段：`title`、`subtitle`、`author`、`language`、`publisher`、`description`、`identifier`、`rights`。
  - 封面：`cover=` 指向本地图片（.jpg/.jpeg/.png/.svg/.webp/.gif），同步 OPF cover metadata、`cover-image` properties 和本地引用；封面文件不存在时拒绝。
- 边界与保护：
  - 始终写出新文件：`--output` 不能与 `--input` 相同（CLI 拒绝），也不要指向已存在的产物。
  - 不在包操作中重写正文、改字体、转换图片或绕过 DRM；真实加密资源、损坏 ZIP 或无效 OPF 时停止（先跑预检）。
  - 不把多个包操作叠成一次调用；需要多个操作时逐个执行并各自校验。

## 调什么

先对输入做预检（只读）：

```sh
epub run epub.package.nav.audit --input <书> --json
```

再执行用户选择的单个操作（写型能力，`--output` 必填且指向新文件）：

```sh
# 合并（extra_inputs 逗号分隔第二本及以后）
epub run epub.package.merge --input <第一本> --output <merged.epub> --json extra_inputs=<第二本>[,<第三本>...] [title=<新标题>]

# 拆分（split_points 为 TOC 目标下标；output_dir 必须为空目录）
epub run epub.package.split --input <书> --output <占位新书> --json output_dir=<空目录> split_points=0,8

# 修改元数据（metadata_json 为内联 JSON 文本）
epub run epub.metadata.edit --input <书> --output <新书> --json metadata_json='{"title":"新书名"}'

# 替换封面
epub run epub.cover.replace --input <书> --output <新书> --json cover=<cover.png>
```

拆分的 `--output` 是 CLI 用法检查要求，实际段产物由 `output_dir` 承载。需要旧报告形状明细（OperationReport：inputs/outputs/segments_created/renamed_resources 等）时给 run 命令加 `legacy_report=true`。需要 DRM 或字体混淆口径的两文件比对时：

```sh
epub redline --check all <before.epub> <after.epub>
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（缺 `--output`、输出与输入相同、KEY=VALUE 非法等）。
- facts 键前缀为各能力 id（`epub.package.merge.` / `epub.package.split.` / `epub.metadata.edit.` / `epub.cover.replace.`）：
  - merge：`inputs`（全部输入）、`output`、`opf`、`mergedItems`、`renamedResources`、`warnings`。
  - split：`outputDir`、`outputs`（逐段产物路径）、`segmentsCreated`、`opf`。
  - metadata：`output`、`opf`、`fieldsUpdated`。
  - cover：`output`、`opf`、`coverPath`（包内新封面路径）。
- findings：
  - `error package.refused`：操作被拒绝（合并输入不足、split point 越界、输出目录非空、封面文件缺失、加密资源等），`title` 是原因，输出不落盘。
  - `warn merge.warning`：合并时的资源改名、metadata 冲突等提示。
  - run 内置红线失败时出现 `error redline.<check>`（text/metadata/spine/anchors/cover/drm）。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（旧 OperationReport 形状）。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- `status == complete` 且无 `error` → 核对 facts 与预期一致：merge 看 `inputs` 数量与 `renamedResources` 是否可接受；split 看 `segmentsCreated`、`outputs` 与段边界；metadata 看 `fieldsUpdated`；cover 看 `coverPath` 与尺寸。
- 合并或拆分改变了 package/spine → 人工确认报告中的输入、输出、段数和重命名资源，再用 Calibre Editor 或 VS Code 抽查 OPF/nav/NCX；需要时对产物重跑 `epub.package.nav.audit`。
- `error package.refused` → 按 `title` 修正前提（补输入、换空输出目录、修正 split_points、确认封面文件存在），不删除或绕过保护；提示加密时停止。
- `findings` 出现 `error redline.*` → 停止：输出保留供人工 diff review，先修源再重跑；不允许用宽泛 allow-list 掩盖。
- `status == approval-required` → 停下来问人；每个操作完成后在书级 `制作说明.md` 记录输入/输出 SHA 与理由。
