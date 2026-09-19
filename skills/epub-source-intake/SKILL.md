---
name: epub-source-intake
description: 从非 EPUB 源材料建立 EPUB 制作入口，包括纯文本、Markdown、HTML、PDF、扫描件 OCR 结果和图片素材的盘点、角色分类、风险标记、结构化、校对与后续排版验证。用于用户还没有 EPUB，需要先对源目录做可审计的只读盘点并生成 source bundle 计划，再进入 EPUB 排版优化流程时。
---

# EPUB 源材料接入

## 何时用

- 用户没有现成 EPUB，而是提供文本、Markdown、HTML、PDF、扫描件或图片素材目录。目标不是直接做最终精排，而是先做可审计的只读盘点，生成可校对、可验证、可继续排版的 EPUB source bundle。
- 输入判断：已有 `.epub` 时不用本 skill，先用 `epub-package-nav-auditor`（本能力遇到 `.epub` 只会给出 info 并指向它）；`.txt`/`.md` 走文本结构化（章节、段落、注释和空行语义）；`.html`/`.xhtml` 走 HTML 清理（语义标签、资源路径、XML 合法性）；born-digital PDF 优先用外部工具抽取文本和图片再人工抽样校对；扫描 PDF/图片先 OCR，把 OCR 结果当不可信 source；多源目录先跑本能力建立文件角色清单。
- PDF 处理边界（仓库所有者决策）：本仓不实现 PDF 解析、OCR、图片压缩或版面识别引擎，本能力只把 PDF 标记为 `pdf-out-of-scope`。可调用用户环境已有工具，但必须在来源记录中登记工具名和版本。born-digital PDF 也可能有断行、页眉页脚、连字符、阅读顺序和多栏问题，不要默认抽取结果正确；扫描 PDF 必须标记为 OCR 风险输入；PDF 中的页码、脚注编号、图题和表题要单独检查，不要只看首章。
- 图片压缩边界：图片压缩不属于本项目实现范围。本能力只标记图片格式风险（WebP、TIFF、BMP、AVIF、HEIC、CMYK JPEG），OPF manifest/封面声明/figure 包装/图注的检查交给下游 `epub-package-nav-auditor` 与 `epub-image-layout-optimizer`；建议外部压缩转码后回到本项目验证。
- 禁止事项：不在没有抽样校对的情况下把 PDF/OCR 文本当最终正文；不自动改写作者文字修复抽取错误；不丢弃脚注、边注、图注、表格标题或公式；不把 PDF 页面截图当可重排正文主路径；不把图片压缩结果直接纳入规则结论（除非有阅读器验证）；不在本 skill 决定最终视觉样式——这里只负责接入、盘点、结构化和校验入口。

## 调什么

```sh
epub run epub.source.intake --input <源文件目录或单个源文件> --json
```

- `--input` 必填，可以是目录或任意单个**普通**文件（不要求是 EPUB）；缺失、不存在，或指向 FIFO / 设备 / socket 这类非普通文件（如 `/dev/zero`、`/dev/null`）→ 退出码 3。不需要 `--output`：本能力是只读 planner，不写任何文件；`--dry-run` 与正常运行结果完全相同（包括 `nextCommands`），不会返回 `approval-required`，也不会建议 `--output`。
- 可选参数 `max_files=<n>`（默认 5000）：必须是**正整数**，`abc` / `0` / `-1` 一律是用法错误 → 退出码 3（不会静默回落到默认值）。文件数超过上限时给出 `error intake.too-many-files` 并停止遍历。
- 遍历规则：按路径字典序、跳过以 `.` 开头的隐藏文件与目录（含 `.DS_Store`、`.git`）、不跟随符号链接（只记录为 `unknown` 角色并给 warn）。**例外**：`--input` 本身是符号链接目录时（iCloud / Dropbox 镜像、软链的 `01 源文件/`）会解析一次再遍历，否则整棵树会被误判为空；报告里仍显示你给出的路径。目录读不出（权限）时只丢那一棵子树并给 warn，其余条目照常盘点。每个文件流式计算 SHA-256，不修改任何输入。
- 盘点完成后按人工流程继续：按书级项目约定建工作区（`01 源文件/`、`02 校对材料/`、`03 制作工作区/`，见 [docs/pipeline/book-workspace.md](../../docs/pipeline/book-workspace.md)），把入选源文件原样保留在 `01 源文件/`，`facts` 中每个文件的 `sha256` 即来源记录的 SHA-256；在 `制作说明.md` 写来源记录：文件角色、抽取工具和参数、需要人工确认的页码/脚注/表格/公式/图片。形成 source tree 或 `.epub` 后再交给 `epub-package-nav-auditor` 与排版专项 skill。

## 返回怎么读

- `status`：`complete` 表示盘点完成（可含 warn/info；权限导致的局部缺口也是 `complete` + warn + blocker，见 `intake.unreadable-*`）；`failed` 表示目录为空（`intake.empty`）或超过文件上限（`intake.too-many-files`），退出码 1；退出码 3 为用法错误（`--input` 缺失/不存在/非普通文件，`max_files` 非正整数）。本能力从不返回 `approval-required`。
- `findings[]`（id 稳定）：
  - `error intake.empty`：输入目录没有非隐藏文件。
  - `error intake.too-many-files`：超过 `max_files`，盘点不完整。
  - `warn intake.pdf-out-of-scope`：每个 PDF 一条；本仓不解析 PDF/OCR。
  - `warn intake.image-format-risk`：每个非 EPUB 核心媒体类型图片（WebP/TIFF/BMP/AVIF/HEIC）或 SOF 声明 4 分量的 CMYK/YCCK JPEG 一条。
  - `warn intake.encoding`：文本/HTML 文件不是合法 UTF-8。
  - `warn intake.nested-archive`：`.zip`/`.rar`/`.7z` 需先解包。
  - `warn intake.symlink`：符号链接未跟随。
  - `warn intake.unreadable-file`：文件无法读取，未计算 SHA-256；该条目带 `risks: ["unreadable"]`，其 `size: 0` 与空 `sha256` 表示"没读到"，不是"文件为空"。
  - `warn intake.unreadable-dir`：目录无法列出（权限），整棵子树未纳入盘点；`location` 是该目录的相对路径。其余条目照常盘点、照常有 SHA-256，但本次盘点不完整。
  - `info intake.already-epub`：输入里已有 `.epub`，`detail` 末尾就是 `nextCommands` 里对应的那条命令，逐字相同、含 `--json`、路径已按 POSIX shell 引用，可直接复制执行。
  - `info intake.summary`：文件数、总字节数与角色计数。
- `facts`（键带前缀 `epub.source.intake.`）：
  - `sourcePath`（绝对路径）、`inputKind`（`directory | file`）、`fileCount`、`totalBytes`、`roleCounts`（角色 → 数量）。
  - `files[]`：`{path（相对路径，正斜杠）, role, size, sha256, risks[]}`。`role` 取值：`text | html | image | pdf | epub | font | css | audio | video | archive | document | unknown`；`risks` 取值：`encoding-not-utf8 | utf8-bom | crlf | image-format-risk | jpeg-cmyk | pdf-out-of-scope | already-epub | nested-archive | symlink | unreadable`。`roleCounts` / `fileCount` 包含 `unreadable` 条目（它们确实在目录里），但 `blockers` 推断会跳过它们——读不出的 `.txt` 不算"已有可结构化正文"。
  - `workspacePlan`：`01 源文件/`、`02 校对材料/`、`03 制作工作区/`、`制作说明.md` 各应放什么的静态建议。
  - `plan`：合 `contracts/schemas/v1/execution-plan.schema.json` 的下游链建议（`artifact` + `steps`：nav-audit → structure-normalize（需批准）→ migrate-epub3（需批准）→ layout-audit + `blockers`）。`blockers` 同时在顶层重复一份，便于直接读取。
- `nextCommands[]`：仅当输入里已有 `.epub` 时，对每个（最多 5 个）给出 `epub run epub.package.nav.audit --input <file> --json`；`<file>` 是宿主绝对路径，含空格或括号时已用单引号包好，可直接粘进 shell。纯源材料输入时为空（`--dry-run` 也一样为空），下一步是人工抽取与结构化。

## 依据返回怎么判断

- `status == failed` 且 `intake.empty` → 先把入选源材料放入输入目录（或 `01 源文件/`）再重跑；`intake.too-many-files` → 缩小输入目录或提高 `max_files`，盘点不完整时不要基于 `facts.files` 写来源记录。
- `blockers` 非空 → 逐条清除后再进入下游链：PDF 项需外部抽取 + 抽样校对并记录工具名与版本（扫描件另标 OCR 风险）；非 UTF-8 项先转码为 UTF-8（无 BOM），转码前后各记一次 SHA-256；图片格式项外部转码为 JPEG/PNG/GIF/SVG（CMYK 转 sRGB）；嵌套压缩包解包后重跑本能力；"N 个文件/目录无法读取"项说明本次盘点不完整（见 `intake.unreadable-file` / `intake.unreadable-dir` 的 `location`），修好权限重跑后再写来源记录；"尚无 EPUB 或可结构化文本"项说明必须先人工建立 source tree。
- `files[].risks` 含 `utf8-bom` / `crlf` → 结构化时统一为无 BOM、LF；这是 warn 以下的提示，不阻断，但要写入来源记录。
- `intake.already-epub` → 不走源材料接入，直接**逐字执行** `nextCommands` 中的 `epub run epub.package.nav.audit --input <file> --json`（不要自己重拼路径），再按 `AGENTS.md`「已有 EPUB 固定流程」进行。
- `roleCounts` 只有 `image`/`font`/`unknown` 而无 `text`/`html`/`document`/`epub` → 没有可结构化正文，停下来向用户确认正文来源（外部 OCR/抽取或补文本）。
- 抽取与结构化的质量判据（人工）：章节切分清楚、保留原始顺序；正文、标题、注释、图注、表格、公式和页码残留有明确分类；抽取日志记录工具、参数、抽样页和已知风险；XHTML 保持真实文本，不把可编辑正文转成图片；图片素材保留原始文件与派生交付文件的对应关系。抽样校对必须覆盖：第一页/目录页；每个章节边界；至少一个脚注密集页；至少一个表格或公式页；至少一个图片页。
- 形成 `.epub` 后按 `plan.steps` 顺序进入排版链路：`epub.package.nav.audit` → `epub.structure.normalize`（dry-run 先看再批准）→ `epub.package.migrate.epub3`（需批准）→ `epub.layout.audit`；专项分流：字体/正文 → `epub-typography-optimizer`；图片 → `epub-image-layout-optimizer`；注释 → `epub-popup-footnote-converter`；竖排/Ruby → `epub-vertical-ruby-optimizer`；Kindle → `epub-kindle-compatibility-checker`；英文排版 → `epub-english-typography-optimizer`。外部图片压缩完成后以 `epub redline --check all <before> <after>` 确认改动范围。
