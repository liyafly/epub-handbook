# 一书一 Git 工作区

书稿的长期维护以解包后的 EPUB 源目录为准。书级仓库记录每次可读修改；打包、字体子集和检查都在临时工作区完成，成功后只保留一个可覆盖的交付文件。

## 初始化

从手册仓库根目录执行：

```sh
sh templates/book-starter/new-book.sh work-epub/my-book
```

脚本会创建一本书的独立本地 Git 仓库，放入最小 EPUB 骨架、构建脚本、`.gitignore`、`THIRD_PARTY.md` 和 `制作说明.md`。目标目录必须尚不存在，避免覆盖已有书稿。首次写完元数据或正文后提交：

```sh
git -C work-epub/my-book add .
git -C work-epub/my-book commit -m 'chore: establish book source'
```

目录结构：

```text
work-epub/my-book/
├── 01 源文件/                 # 已有 EPUB 的冻结底本；新书可暂为空
├── 02 校对材料/               # 按需放校对资料和授权决策
├── 03 制作工作区/
│   ├── epub/                  # 唯一长期维护的解包 EPUB 源目录
│   │   ├── mimetype
│   │   ├── META-INF/
│   │   ├── OEBPS/
│   │   ├── build.sh
│   │   └── README.md
│   ├── .pipeline/             # 临时打包、报告和候选，Git 忽略
│   └── dist/book.epub         # 最近一次通过验证的产物，Git 忽略并覆盖
├── .gitignore
├── THIRD_PARTY.md
└── 制作说明.md
```

不需要时不要建立空的 `reports/`、`before/`、`after/` 或额外的 EPUB 版本目录。确有多套图片、字体母版或可复用脚本时，再按书内实际需要增加目录。

## 日常修改和构建

直接修改 `03 制作工作区/epub/OEBPS/` 下的 XHTML、CSS、OPF、nav 与 NCX。每次构建都从这个源目录重新打包，所以源目录始终是完整、可编辑、可比较的版本。提交源文件的变化后运行：

```sh
sh '03 制作工作区/epub/build.sh'
```

脚本把中间包和报告放进 `03 制作工作区/.pipeline/`，最后运行导航结构审计和全项 redline；所有检查通过才用同卷重命名覆盖 `03 制作工作区/dist/book.epub`。构建失败时，最近一次通过验证的 `book.epub` 保持原样。不会按时间戳生成新文件，也不会把中间 EPUB 留在书目录里。

`dist/book.epub` 和 `.pipeline/` 默认由新书脚本写进书级 `.gitignore`。Git 主要维护解包源、校对材料、脚本与制作决策；交付 EPUB 是可从某次源提交重建的输出。每次交付后，在 `制作说明.md` 记录源提交、产物 SHA-256 和阅读器实测。若要长期归档具体交付版，把它发布到书级仓库之外的发行位置，并保留对应 SHA，不要把反复构建的二进制历史塞进源文件提交。

## 字体母版与子集

推荐把获准使用的完整 `.ttf` / `.otf` 母版放在解包树 `OEBPS/Fonts/`，并在 OPF manifest 与 CSS 中正常声明。若它在 manifest 中，构建脚本会自动调用 `epub.font.subset`：Go 流水线把完整字体 EPUB 交给独立的 `epub-font` provider，在临时目录生成和验证子集，最后只将变更后的字体 entry 应用到输出候选。解包源里的完整字体不会被覆盖。

因此每次修改正文后，构建都会从完整字体重新计算所需字形；新增加的字不依赖上次子集化产物，不会因为旧子集缺字而无法恢复。完整字体的许可和来源记入 `THIRD_PARTY.md`。带 OpenType MATH 表的字体按 provider 规则保持完整，不做子集。

若字体母版需要放在 EPUB 外部，可在书根创建 `fonts.json`，`master` 路径相对该配置文件；详细 schema 与可变字体选项见 [`epub-font` 文档](../../tools-font/epub-font/README.md)。无字体的书不需要安装字体 provider。需要子集化时，在手册仓库中安装：

```sh
uv tool install --editable tools-font/epub-font
```

provider 缺失或子集检查失败会使这次构建失败，原有 dist 不会被替换。完整 Go CLI 必须包含 `epub.font.subset`；尚未包含该 capability 的旧版 CLI 不能用于带字体书籍的构建。

## 已有 EPUB 的一次性接入

已有 EPUB 仍按 [`cleanup-flow.md`](cleanup-flow.md) 做底本冻结、预检、必要的结构规范化、迁移判断、全项红线和人工 diff review。确认的最终候选解包到 `03 制作工作区/epub/`，把原始 EPUB 放进 `01 源文件/` 并记录 SHA，然后提交书级基线。后续局部修改直接维护解包源并运行本页的单一构建命令；只有再次执行有意的清洗、迁移或授权校订时，才重跑对应的专项审查，不需要每次编辑都重复完整接入流程。

## Git 与第三方材料

- `work-epub/` 被手册主仓忽略。每本书有自己的 `.git`，不添加为 submodule。
- 书级 Git 保留可编辑源文件、构建脚本、必要的审阅材料、`THIRD_PARTY.md` 和 `制作说明.md`。
- `.pipeline/`、`dist/`、缓存与失败临时候选不进 Git。被正文校订或结构 gate 引用的决策文件与路径映射仍须留到 gate 完成。
- 第三方 EPUB、字体和图片进入书级 Git 前，在 `THIRD_PARTY.md` 记录来源、作者、许可和保留理由；许可未核实不得配置公开 remote 或发布原始材料。
- 不在手册主仓执行 `git add -f work-epub/<book>`。
