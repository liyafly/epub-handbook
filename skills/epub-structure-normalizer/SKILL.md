---
name: epub-structure-normalizer
description: 整理已有 EPUB 的资源目录，并按固定顺序先格式化、再按 OPF manifest id 还原被混淆的资源文件名，同步重写 OPF、XHTML、CSS、NCX、SVG、SMIL 和标准字体混淆 URI。用于 EPUB 内部路径散乱、文件名不可读、需要先做结构规范化再精排，或用户明确要求文件名反混淆时；不用于 DRM 解密。
---

# EPUB 结构格式化与文件名反混淆

## 何时用

- EPUB 内部路径散乱、文件名不可读、需要稳定 diff，或用户明确要求文件名反混淆。
- 固定语义：`normalize` = 先 `format`（把 manifest 资源归类到 OPF 同级的 `Text/`、`Styles/`、`Images/`、`Fonts/`、`Audio/`、`Video/`、`Misc/`，`ncx` 留在 OPF 同级，保留原文件名），再 `deobfuscate`（按 OPF manifest `id` 生成可读文件名）。两阶段同步重写 OPF、XHTML、CSS、NCX、SVG、SMIL 中的本地链接。
- 边界：始终写出新文件，不原地覆盖输入；不改写作者正文，不做 EPUB3 迁移，不替代 `epub-package-nav-auditor`。
- 加密边界（不可放宽）：
  - 不提供 DRM 解密；遇到正文、样式、图片或未知算法加密时立即停止，不猜测、不绕过。
  - 声明目标在 ZIP 中不存在的 stale encryption 引用会被移除并给出 warning。
  - EPUB 标准字体混淆允许：移动字体时同步更新 `META-INF/encryption.xml` 的 URI，不修改字体字节；只有任务得到明确授权时才处理。
  - CSS 引用不存在的字体文件时，不猜测它对应哪个嵌入字体，也不删除声明；保留 `local()` fallback，警告交人工复核。CSS 中非字体资源断链是阻断错误，必须修复后才能进入后续清洗。

## 调什么

双阶段是固定流程：先 dry-run 人工 review，再实跑。

```sh
# 1) dry-run：全局 --dry-run 只扫描不写输出
epub run epub.structure.normalize --input <原始 EPUB> --output <normalized.epub> --dry-run --json

# 2) 人工 review facts 里的 mappings 后实跑，并保存信封（内含 normalize JSON 报告）
epub run epub.structure.normalize --input <原始 EPUB> --output <normalized.epub> --json legacy_report=true > work/normalize-envelope.json

# 3) 提取 normalize JSON 报告（含改名映射），立刻作为红线 gate 的路径映射
jq -r '.facts["epub.structure.normalize.legacyReport"]' work/normalize-envelope.json > work/normalize-report.json

# 4) 两文件红线比对（flag 必须写在两个路径之前）；inspect 确认只含标准字体混淆时加 --allow-font-obfuscation
epub redline --check all --path-map work/normalize-report.json <before.epub> <normalized.epub>
```

检查加密边界（只读体检）：

```sh
epub run epub.structure.normalize --input <书> --output <临时副本> --json mode=inspect
```

`mode=inspect` 不修改书；当前 CLI 用法检查要求同时给 `--output`，inspect 模式下输出只是输入的未修改副本。

单步排障（只在定位故障时使用，先 dry-run）：`mode=format`、`mode=deobfuscate`。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required（dry-run review）；3 用法错误（参数非法、文件不存在、缺 `--output`）。
- facts 键前缀 `epub.structure.normalize.`：
  - `mappings[]`（`from`/`to` 改名映射）、`warnings[]`、`stages[]`（每阶段的 `operation`、`mappings`、`warnings`、`removed_stale_encryption_resources` 等）、`movedResources`、`renamedResources`、`rewrittenFiles`、`dryRun`。
  - `mode=inspect` 时：`manifestResources`、`fontObfuscationResources`、`removedStaleEncryptionResources`、`opf`。
- findings：
  - `warn structure.warning`：warnings 转化（不安全/缺失的本地引用未改动、stale encryption 移除等）。
  - `error redline.<check>`：run 内置红线门禁（text/metadata/spine/anchors/cover/drm）。
  - `error capability.run-failed`：保守重写失败（典型原因是遇到不支持的加密），事件里可见 `EPUB cannot be rewritten conservatively`。
- `epub redline` 输出是逐行文本（不是统一信封）：`All requested red-line checks passed.` 表示通过，其余行列出违反项与退出码。

## 依据返回怎么判断

- dry-run `status == approval-required`（退出码 2）→ 逐条 review `facts` 里的 `mappings` 与 `warnings`，确认两阶段改动合理后移除 `--dry-run` 实跑。不要在未 review `mappings` 的情况下批量覆盖输出。
- 已知行为：当 `mappings` 含 XHTML 改名时，dry-run 信封会出现指向改名文件的 `redline.text` / `redline.anchors` error findings。这是 dry-run 中间态（改名尚未应用到书态）的表现，不作为阻断；review 对象是 `mappings`/`warnings`，红线结论以实跑后的内置门禁与两文件 `epub redline --path-map` 为准。
- 实跑 `status == failed` 且事件提示保守重写失败 → 存在 DRM 或非字体加密：停止，不解密、不猜测、不绕过；只有工具明确识别为标准字体混淆且任务得到明确授权时才可继续。
- `findings` 出现 `redline.*` error → 立即停：输出文件保留供人工 diff review，先修源再重跑；不允许用宽泛 allow-list 掩盖。
- `structure.warning` → 逐条人工复核；CSS 字体断链按「不猜测、不删声明、保留 `local()` fallback」处理。
- 红线通过 → 用 Calibre Editor 或 VS Code 做人工 diff review，再进入 EPUB3 迁移（`epub3-migrator`）与 OPF/nav 审核（`epub-package-nav-auditor`）、排版专项 skill；不要把结构规范化与 EPUB3 package 迁移混成一步。
- 行为目标参考 [cnwxi/epub_tool](https://github.com/cnwxi/epub_tool) 的 `reformat` 与文件名 `decrypt` 工作流；第三方说明见 `THIRD_PARTY.md`。
