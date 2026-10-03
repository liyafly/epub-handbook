# 一书一 Git 工作区

书稿的长期维护以解包后的 EPUB 源目录为准。书级仓库记录每次可读修改；打包、字体子集和检查都在临时工作区完成，成功后只保留一个可覆盖的交付文件。本页的 `book-starter` 和 `build.sh` 使用固定的 `OEBPS/` 布局，只覆盖该模板的打包范围。

## 初始化

从手册仓库根目录执行：

```sh
sh templates/book-starter/new-book.sh work-epub/my-book
```

脚本会创建一本书的独立本地 Git 仓库，放入书根 `README.md`、最小 EPUB 骨架、构建脚本、`.gitignore`、`THIRD_PARTY.md` 和 `制作说明.md`。书根 README 是给后续维护者的日常操作说明；解包目录只放 EPUB 源文件和构建脚本。目标目录必须尚不存在，避免覆盖已有书稿。首次写完元数据或正文后提交：

```sh
git -C work-epub/my-book add .
git -C work-epub/my-book commit -m 'chore: establish book source'
```

目录结构：

```text
work-epub/my-book/
├── README.md                  # 书级日常修改、构建与字体说明
├── 01 源文件/                 # 已有 EPUB 的冻结底本；新书可暂为空
├── 02 校对材料/               # 按需放校对资料和授权决策
├── 03 制作工作区/
│   ├── epub/                  # 唯一长期维护的解包 EPUB 源目录
│   │   ├── mimetype
│   │   ├── META-INF/
│   │   ├── OEBPS/
│   │   └── build.sh
│   ├── .pipeline/             # 临时打包、报告和候选，Git 忽略
│   └── dist/book.epub         # 最近一次通过验证的产物，Git 忽略并覆盖
├── .gitignore
├── THIRD_PARTY.md
└── 制作说明.md
```

不需要时不要建立空的 `reports/`、`before/`、`after/` 或额外的 EPUB 版本目录。确有多套图片、字体母版或可复用脚本时，再按书内实际需要增加目录。

## 日常修改和构建

直接修改 `03 制作工作区/epub/OEBPS/` 下的 XHTML、CSS、OPF、nav 与 NCX。`build.sh` 要求 `mimetype`、`META-INF/` 和 `OEBPS/` 同时存在，只打包 `META-INF/` 与 `OEBPS/`；字体扫描也会检查这两个打包目录中的已知字体扩展名。每次构建都从这个源目录重新打包，所以源目录始终是完整、可编辑、可比较的版本。提交源文件的变化后运行：

```sh
sh '03 制作工作区/epub/build.sh'
```

脚本把中间包和报告放进 `03 制作工作区/.pipeline/`，最后运行导航结构审计和全项 redline；所有 gate 通过才用同卷重命名覆盖 `03 制作工作区/dist/book.epub`。构建内 FULL/FINAL redline 只检查构建过程中的字体子集化等变换。若已有 `dist/book.epub`，构建还会把它与新候选跑一次全项 redline，并将差异摘要打印到终端；该对比只提示作者检查正文等版本变化，不是正文不变 gate。

为防止人工编辑过的成品被后续构建覆盖，脚本在 `.pipeline/dist-sha256` 保存最近一次成功构建的交付件 SHA-256。重建前必须确认当前 `dist/book.epub` 与收据匹配；收据缺失、摘要不匹配、文件在构建过程中被改动或被替换时，脚本会停止并保留现有成品。所有源文件变更仍应提交到书级 Git；dist 只是交付输出，不是维护源。

若需用 Sigil 等工具直接修复交付 EPUB，先复制留档，对照改前版本运行 `epub redline --check all` 并人工检查差异；再将需要的变化并回 `03 制作工作区/epub/`，运行导航审计、全项 redline并审阅源码 diff。确认源树已含所需修复后，才把当前交付件 SHA 写入收据，允许下一次构建：macOS 用 `shasum -a 256 '03 制作工作区/dist/book.epub' | awk '{print $1}' > '03 制作工作区/.pipeline/dist-sha256'`，Linux 可改用 `sha256sum`。若 `.pipeline/` 清理导致收据遗失，而 dist 仍存在，应先重新审查并对齐源再恢复收据；不得将缺少收据视作可安全覆盖。构建失败时，最近一次成品保持原样。不会按时间戳生成新文件，也不会把中间 EPUB 留在书目录里。

将编辑器修复并回源时，同时记录它属于正文、导航、元数据、资源还是 XML/CSS 序列化变化，并说明哪些是必须长期保留的行为。需要长期保留的转换行为应写入本书的生成脚本；能够稳定断言的结果则加入书级验证器，避免下一次重建或换编辑器保存时丢失。若编辑器只改缩进、空元素形式或生成器标记，应单独审阅，不要把批量字节变化误认为书稿内容已核验。`制作说明.md` 记录最终采纳的修复与对应验证。

例如，《圣经的故事》保存 Sigil 2.8.1 后，为全部 XHTML 加入 `<!DOCTYPE html>`、移除了冗余 `<meta charset>`，并更新 OPF 修改时间、加入 Sigil 版本标记及调整 `container.xml` 缩进。经结构化对比确认正文和导航语义未变后，书级 `tools/prepare-book.py` 固定 DOCTYPE 与不输出冗余 charset 的行为，`tools/verify-book.py` 对全部 XHTML 逐文件断言这两条规则。它们是该书接受的编辑器修复，不自动成为所有书籍的 EPUB 规则。

维护 starter 构建保护时，在手册仓库根运行 `sh templates/book-starter/test-build-dist-guard.sh`；回归覆盖首次构建、源更新重建、人工修改后拒绝覆盖，以及收据缺失时拒绝覆盖。

`dist/book.epub` 和 `.pipeline/` 默认由新书脚本写进书级 `.gitignore`。Git 主要维护解包源、校对材料、脚本与制作决策；交付 EPUB 是可从某次源提交重建的输出。打包前会在临时目录复制源树、统一时间戳并按固定路径顺序归档；相同源提交、平台以及 `zip -v` 显示的相同 zip 构建可复现相同 SHA-256，跨平台或 zip 构建不同不承诺字节级一致。macOS 系统 zip 不会为非 ASCII 条目名设置 UTF-8 标志，EPUB 条目名请使用 ASCII。每次交付后，在 `制作说明.md` 记录源提交、产物 SHA-256 和阅读器实测。若要长期归档具体交付版，把它发布到书级仓库之外的发行位置，并保留对应 SHA，不要把反复构建的二进制历史塞进源文件提交。

## 字体母版与子集

推荐把获准使用的完整 `.ttf` / `.otf` 字体放在解包树 `OEBPS/Fonts/`，即 OPF manifest 声明的目标路径，并在 OPF 与 CSS 中正常声明。`build.sh` 扫描 `META-INF/` 与 `OEBPS/` 下任何已知字体后缀文件（不论它是否已登记在 OPF manifest）以及书根 `fonts.json`；发现任一项都会调用 `epub.font.subset`。能力会读取 EPUB entry 的前四字节，按 SFNT、TTC、WOFF 或 WOFF2 魔数检查全部 manifest 与未登记资源；未登记字体报告 `font-subset.unmanifested-font`，错误声明为非字体媒体类型的字体报告 `font-subset.disguised-font`，这两类资源都不能进入交付产物。若扫描触发构建但没有 manifest 字体目标，会以 `font-subset.no-fonts` 失败。一个残余限制是：若无 `fonts.json`、无已知字体扩展名的书把字体改名后藏入资源树，书级 `build.sh` 无法据此触发 provider；先将其改回常用字体扩展名，或配置 fonts.json。Go 流水线把含完整字体的源 EPUB 交给独立的 `epub-font` provider，在临时目录生成和验证子集，最后只将变更后的字体 entry 应用到输出候选。provider 还会用独立字符收集器按完整源字体与候选字体比较缺字、空字形和 IVS/SVS 序列；仅对完整源字体可用、子集后退化的项目判为回归。解包源里的完整字体不会被覆盖。

因此每次修改正文后，构建都会从解包源中的完整字体重新计算所需字形；新增加的字不依赖上次子集化产物，不会因为旧子集缺字而无法恢复。完整字体的许可和来源记入 `THIRD_PARTY.md`。带 OpenType MATH 表的字体无论是否列在配置中都会按原字节保留，provider 报告记录 `reason=math-table` 与输入/输出 SHA；普通字体继续子集化。旧配置里的 `action: "preserve"` 暂时作为弃用 no-op 接受并提示，之后应删除该字段。可变字体配置只接受 `variation.mode: "instance"`。加密/混淆、损坏或不支持的字体仍会失败。交付 EPUB 必须由书内 `build.sh` 构建，或由 `epub.font.subset` 生成候选后通过规定检查；禁止直接把解包源树打包交付。

无字体的书不需要安装字体 provider。需要子集化时，在手册仓库中安装：

```sh
uv tool install --editable tools-font/epub-font
```

provider 缺失或子集检查失败会使这次构建失败，原有 dist 不会被替换。完整 Go CLI 必须包含 `epub.font.subset`；尚未包含该 capability 的旧版 CLI 不能用于带字体书籍的构建。

## 升级既有书

书级仓库会保留创建时复制的 `build.sh`，升级手册仓库不会自动更新这些脚本。epub-font 2.0.0 已删除 `epub-font check --against`；若旧脚本仍调用它，构建会以迁移提示退出。检查并删除该调用，或审阅后将脚本更新到当前 `templates/book-starter/build.sh`。dist SHA 收据保护同样只在书内脚本更新后生效；更新既有书时，先检查本书构建逻辑，再同步并验证模板。当前 Go 的 `epub.font.subset` 会在应用候选前检查独立覆盖回归。

已安装的 provider 升级到 2.0.0：

```sh
uv tool install --editable --reinstall tools-font/epub-font
```

provider 2.0.0 使用 schemaVersion 2。旧 schema 会以 `font-subset.provider-outdated` 失败，且不会应用候选。旧 `fonts.json` 中的 `action: "preserve"` 在 2.x 暂时接受并提示弃用；请在 3.0.0 前移除。`tools-font/coverage-detector/` 已删除，字体覆盖分析统一经 PATH 上的 `epub-font coverage` 执行。

## 已有 EPUB 的一次性接入

已有 EPUB 仍按 [`cleanup-flow.md`](cleanup-flow.md) 做底本冻结、预检、必要的结构规范化、迁移判断、全项红线和人工 diff review。S0 时即把原始 EPUB 放进 `01 源文件/` 并记录 SHA。解包前先核对 `META-INF/container.xml` 指向的 OPF 路径和 ZIP 目录：`EPUB/`、`EPUB/OPS/` 或根目录 OPF 等非 `OEBPS/` 布局，不能直接使用本页的 starter `build.sh`；它会报告缺少 `OEBPS/`。`epub.structure.normalize` 的路径规范化也不承诺把所有源树转换成 OEBPS。遇到非 OEBPS 书籍时，保留并验证该书现有可用的构建路径，不要为了适配模板脚本未经审查就重命名目录。只有确认最终候选符合本页 OEBPS 骨架且构建验证通过后，才把它解包到 `03 制作工作区/epub/` 并提交书级基线。后续局部修改直接维护解包源并运行已验证的构建命令；只有再次执行有意的清洗、迁移或授权校订时，才重跑对应的专项审查，不需要每次编辑都重复完整接入流程。

对任意合法 EPUB 源树进行通用打包仍是后续功能；本页没有实现新的解包器或通用打包入口。

## Git 与第三方材料

- `work-epub/` 被手册主仓忽略。每本书有自己的 `.git`，不添加为 submodule。
- 书级 Git 保留可编辑源文件、构建脚本、必要的审阅材料、`THIRD_PARTY.md` 和 `制作说明.md`。
- `.pipeline/`、`dist/`、缓存与失败临时候选不进 Git。被正文校订或结构 gate 引用的决策文件与路径映射仍须留到 gate 完成。
- 第三方 EPUB、字体和图片进入书级 Git 前，在 `THIRD_PARTY.md` 记录来源、作者、许可和保留理由；许可未核实不得配置公开 remote 或发布原始材料。
- 不在手册主仓执行 `git add -f work-epub/<book>`。
