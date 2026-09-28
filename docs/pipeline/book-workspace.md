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

直接修改 `03 制作工作区/epub/OEBPS/` 下的 XHTML、CSS、OPF、nav 与 NCX。`build.sh` 要求 `mimetype`、`META-INF/` 和 `OEBPS/` 同时存在，只打包 `META-INF/` 与 `OEBPS/`，字体扫描也只检查 `OEBPS/`。每次构建都从这个源目录重新打包，所以源目录始终是完整、可编辑、可比较的版本。提交源文件的变化后运行：

```sh
sh '03 制作工作区/epub/build.sh'
```

脚本把中间包和报告放进 `03 制作工作区/.pipeline/`，最后运行导航结构审计和全项 redline；所有检查通过才用同卷重命名覆盖 `03 制作工作区/dist/book.epub`。每次构建先清除上次 gate 报告，当前报告使用固定文件名保存，重复构建不会累计报告；终端只显示通过/警告摘要，失败时会打印对应报告。构建失败时，最近一次通过验证的 `book.epub` 保持原样。不会按时间戳生成新文件，也不会把中间 EPUB 留在书目录里。

`dist/book.epub` 和 `.pipeline/` 默认由新书脚本写进书级 `.gitignore`。Git 主要维护解包源、校对材料、脚本与制作决策；交付 EPUB 是可从某次源提交重建的输出。打包前会在临时目录复制源树、统一时间戳并按固定路径顺序归档；相同源提交和相同 `zip` 版本可复现相同 SHA-256，不同 `zip` 版本不承诺字节级一致。每次交付后，在 `制作说明.md` 记录源提交、产物 SHA-256 和阅读器实测。若要长期归档具体交付版，把它发布到书级仓库之外的发行位置，并保留对应 SHA，不要把反复构建的二进制历史塞进源文件提交。

## 字体母版与子集

推荐把获准使用的完整 `.ttf` / `.otf` 母版放在解包树 `OEBPS/Fonts/`，即 OPF manifest 声明的目标路径，并在 OPF 与 CSS 中正常声明。`build.sh` 发现 OEBPS 下任意字体后缀文件或书根 `fonts.json` 时都会调用 `epub.font.subset`；能力只对子集化 OPF manifest 中登记的字体。若触发构建但没有 manifest 字体目标，会以 `font-subset.no-fonts` 失败；应把每个随书字体都登记到 OPF，避免留下未处理的字体文件。Go 流水线把含完整字体的源 EPUB 交给独立的 `epub-font` provider，在临时目录生成和验证子集，最后只将变更后的字体 entry 应用到输出候选。随后 `build.sh` 调用独立的 `epub-font check --against FULL.epub`，按 NEW 的字符清单逐一核对同路径字体的缺字、空字形和 IVS/SVS 序列损失；只有 FULL 原本可用、子集后变得不可用的项才判为回归。解包源里的完整字体不会被覆盖。

因此每次修改正文后，构建都会从完整字体重新计算所需字形；新增加的字不依赖上次子集化产物，不会因为旧子集缺字而无法恢复。完整字体的许可和来源记入 `THIRD_PARTY.md`。自动发现模式会将带 OpenType MATH 表的字体按原字节保留，并在 provider 报告中标记 `action=preserve`、`reason=math-table` 与输入/输出 SHA；普通字体继续子集化。配置文件要显式保留数学字体时使用 `action: "preserve"`，不能把 MATH 字体交给 subset。加密/混淆、损坏或不支持的字体仍会失败。交付 EPUB 必须由书内 `build.sh` 构建，或由 `epub.font.subset` 生成候选后通过规定检查；禁止直接把解包源树打包交付。

若字体母版按包外模式维护（而不是放在书级解包源树），可在书根创建 `fonts.json`，用 `master` 指定母版位置，路径相对该配置文件；详细 schema 与可变字体选项见 [`epub-font` 文档](../../tools-font/epub-font/README.md)。无字体的书不需要安装字体 provider。需要子集化时，在手册仓库中安装：

```sh
uv tool install --editable tools-font/epub-font
```

provider 缺失或子集检查失败会使这次构建失败，原有 dist 不会被替换。完整 Go CLI 必须包含 `epub.font.subset`；尚未包含该 capability 的旧版 CLI 不能用于带字体书籍的构建。

## 已有 EPUB 的一次性接入

已有 EPUB 仍按 [`cleanup-flow.md`](cleanup-flow.md) 做底本冻结、预检、必要的结构规范化、迁移判断、全项红线和人工 diff review。解包前先核对 `META-INF/container.xml` 指向的 OPF 路径和 ZIP 目录：`EPUB/`、`EPUB/OPS/` 或根目录 OPF 等非 `OEBPS/` 布局，不能直接使用本页的 starter `build.sh`；它会报告缺少 `OEBPS/`。`epub.structure.normalize` 的路径规范化也不承诺把所有源树转换成 OEBPS。遇到非 OEBPS 书籍时，保留并验证该书现有可用的构建路径，不要为了适配模板脚本未经审查就重命名目录。只有确认最终候选符合本页 OEBPS 骨架且构建验证通过后，才把它解包到 `03 制作工作区/epub/`，把原始 EPUB 放进 `01 源文件/` 并记录 SHA，然后提交书级基线。后续局部修改直接维护解包源并运行已验证的构建命令；只有再次执行有意的清洗、迁移或授权校订时，才重跑对应的专项审查，不需要每次编辑都重复完整接入流程。

对任意合法 EPUB 源树进行通用打包仍是后续功能；本页没有实现新的解包器或通用打包入口。

## Git 与第三方材料

- `work-epub/` 被手册主仓忽略。每本书有自己的 `.git`，不添加为 submodule。
- 书级 Git 保留可编辑源文件、构建脚本、必要的审阅材料、`THIRD_PARTY.md` 和 `制作说明.md`。
- `.pipeline/`、`dist/`、缓存与失败临时候选不进 Git。被正文校订或结构 gate 引用的决策文件与路径映射仍须留到 gate 完成。
- 第三方 EPUB、字体和图片进入书级 Git 前，在 `THIRD_PARTY.md` 记录来源、作者、许可和保留理由；许可未核实不得配置公开 remote 或发布原始材料。
- 不在手册主仓执行 `git add -f work-epub/<book>`。
