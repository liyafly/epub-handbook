# 一书一 Git 工作区

> 状态：流程文档；用于管理一本已有 EPUB 的源文件、校对材料、可编辑包和产物。

## 核心约定

每本书在 `work-epub/<book>/` 拥有独立本地 Git 仓库。手册主仓库忽略整个
`work-epub/`，因此书级仓库不会被误收入主仓库，也不作为 submodule。

初始只建立三个顶层目录：

```text
work-epub/<book>/
├── 01 源文件/
├── 02 校对材料/
├── 03 制作工作区/
│   ├── epub/
│   ├── images/
│   ├── fonts/
│   ├── scripts/
│   └── dist/
├── .gitignore
├── THIRD_PARTY.md
└── 制作说明.md
```

不为未发生的阶段预先建立 `reports/`、`before/`、`after/` 或多层空目录。

## 目录边界

- `01 源文件/`：只放入选定制作底本和必要的源素材。记录 SHA-256，不原地修改。
- `02 校对材料/`：放异版 EPUB、参考文本和图片。按需才建立子目录。授权正文校订时，在这里新建 `正文校订/`，保留必需的审阅页、决策 JSON 和 diff。
- `03 制作工作区/epub/`：唯一的可编辑 EPUB 解包树。
- `images/`：图片优化的工作副本；原图仍受 Git 和底本 SHA 保护。
- `fonts/`：字体源文件、子集化输入和授权说明。无授权记录的字体不进入产物。
- `scripts/`：仅保留本书可复现的构建和资源处理脚本。
- `dist/`：只放可交付产物；中间 EPUB 不放这里。

## 过程数据与审计

流水线仍可以使用自己的 `before/after/reports` 内部结构，但统一放在：

```text
03 制作工作区/.pipeline/
```

该目录默认被书级 Git 忽略，可以容纳 preflight、dry-run、lint、精排建议和中间
JSON。结束时把需要长期保留的事实汇总到 `制作说明.md`：

- 输入与输出 SHA-256；
- 每步改了什么、为什么；
- preflight、lint、红线、diff review 和阅读器实测结论；
- 用户对黄线或正文变更的授权。

两类文件不能当作普通缓存删除：正在被 `epub redline --path-map`
引用的路径映射，以及授权正文校订的决策 artifact。

## 流水线用法

清洗的中间产物（dry-run 报告、红线输出、normalize 映射等）统一放 `03 制作工作区/.pipeline/`。可执行命令集中在 [`cleanup-flow.md`](cleanup-flow.md)，不在各场景文档重复；全部通过 `epub run <capability-id>` 与 `epub redline` 执行。

工具仍会在该忽略区内产生 `before/`、`after/` 和 `reports/`；这些是流水线内部路径，不是新的书级顶层目录。经确认的最终 EPUB 移入 `03 制作工作区/dist/`。

## Git 约定

```sh
git -C 'work-epub/book-a' init -b main
git -C 'work-epub/book-a' add .
git -C 'work-epub/book-a' commit -m 'chore: establish book workspace baseline'
```

- 默认只建立本地仓库，不自动添加 remote。
- 不在手册主仓库执行 `git add -f work-epub/<book>`。
- 第三方 EPUB、字体和图片入书级 Git 前，在该书的 `THIRD_PARTY.md` 记录来源、作者、许可和保留理由；许可未核实时不得配置公开远程或发布原始材料。
- 提交可编辑解包树、构建脚本、制作说明和需要保留的产物；不提交 `.pipeline/`、缓存或临时文件。
