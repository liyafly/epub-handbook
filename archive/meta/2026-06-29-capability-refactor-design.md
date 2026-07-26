# EPUB 能力细分与 Harness/Skill 具体化设计

> 状态：已批准，进入实施。
> 范围：测试 fixture、`epub_lib`、package operations、EPUB3 migration、AI router、skills/contracts/adapters。

## 目标

在不破坏现有 CLI、Python import 和清洗流程的前提下，把过大的脚本拆成职责清晰的能力单元，并让 AI 能通过具体 harness 和 skill 找到真实可执行入口。

本轮完成此前重构建议的第 2–4 项：

1. 合并测试中的 EPUB ZIP/fixture 重复代码；
2. 将重复的 ZIP 路径、URI、XML 与归档基础能力收口到 `scripts/epub_lib.py`；
3. 拆分 `epub_package_tool.py`、`epub3_oneclick_converter.py` 和 `epub_ai_harness.py`，保留原入口作为稳定 façade。

## 不变式

- 不重命名或删除现有公开脚本。
- 现有子命令、参数、JSON 字段、退出码和 import API 保持兼容。
- 不在重构中修改正文、标点、资源内容或阅读器规则。
- 写操作仍必须显式指定新输出路径，禁止覆盖唯一输入。
- 未知加密资源继续拒绝处理。
- Python 与 Swift 继续按 capability 并存；GUI 仍为 PARKED。
- 新 skill 的 frontmatter 仅含 `name` 和 `description`，每个 skill 单独完成失败测试、实现与验证。

## 架构

### 1. 测试 fixture 层

新增 `scripts/test_support/epub_fixture.py`，只负责测试归档构造：

- `EpubFixture` 累积文本和二进制成员；
- `write()` 保证 `mimetype` 首项且默认不压缩；
- 可显式构造错误压缩、重复成员、缺失 container 等反例；
- 不提供业务级“万能书模板”，各测试仍自行声明与断言相关的 OPF/XHTML。

这样减少 ZIP 写入样板，同时不隐藏测试意图。

### 2. 共享 EPUB 基础层

`scripts/epub_lib.py` 新增纯基础函数：

- ZIP member 路径规范化与越界拒绝；
- 本地/外部 URI 判断、相对路径解析和 URI 引用；
- 安全归档读取，检测重复 member；
- 通用 XML 直接子节点查找。

`epub_package_tool.py` 与 `epub_structure_tool.py` 通过薄包装把 `EpubLibError` 转成各自已有错误类型，维持错误信息和调用方契约。

### 3. Package operations

新增 `scripts/epub_package/` 包：

- `models.py`：package、manifest、spine、metadata、TOC 与 report 数据模型；
- `package_io.py`：OPF/container 读取、资源映射和写出；
- `references.py`：XHTML/CSS/srcset 引用重写；
- `navigation.py`：nav/NCX 解析与生成；
- `merge.py`、`split.py`、`metadata.py`、`cover.py`：每个写操作一个模块。

`scripts/epub_package_tool.py` 只保留兼容 re-export、argparse 和 JSON 输出。

新增四个明确 harness：

- `epub_package_merge_harness.py`
- `epub_package_split_harness.py`
- `epub_metadata_edit_harness.py`
- `epub_cover_replace_harness.py`

每个 harness 只暴露一个操作、要求显式输出、输出结构化 JSON，并由一个 `epub-package-operator` skill 路由。使用一个 skill 而不是四个重叠 skill，避免发现列表碎片化。

### 4. EPUB3 migration

新增 `scripts/epub3_conversion/` 包：

- `models.py`：`ConversionReport`；
- `package.py`：metadata、manifest、cover、guide 和 spine；
- `navigation.py`：NCX 读取与 nav 生成；
- `xhtml.py`：XHTML shell/format、CSS 注入；
- `notes.py`：plain/Sigil/Duokan note 迁移；
- `converter.py`：按固定顺序协调各 pass。

`epub3_oneclick_converter.py` 保持原 CLI 与函数 re-export。新增 `epub3_migration_apply_harness.py`，与现有只读 `epub3_migration_harness.py` 形成 plan/apply 对。新增 `epub3-migrator` skill，明确 preflight、dry-run、apply、红线与产物验证顺序。

### 5. AI router

新增 `scripts/epub_ai/` 包：

- `model.py`：`EpubModel` 和 source/tree/EPUB model builder；
- `detectors.py`：detector registry 和确定性 findings；
- `report.py`：finding/report 数据结构与渲染；
- `routing.py`：按输入种类和 mode 生成 skills/commands。

`epub_ai_harness.py` 继续作为唯一公共路由入口，并 re-export 当前测试和第三方调用会使用的 `detector`、`DETECTORS`、`collect_actionable_findings` 等符号。

## Capability 与 Skill

新增 capability manifest：

- `epub.package.merge`
- `epub.package.split`
- `epub.metadata.edit`
- `epub.cover.replace`
- `epub.package.migrate.epub3`

写操作 capability 保持 `requiresWriteAccess=true`，不直接加入只接受单一 artifact 的通用 provider allow-list；它们通过各自 harness 执行。公开 entrypoint inventory 仍登记真实脚本和 capability，避免“有契约、无入口”。

新增两个 skill：

- `epub-package-operator`：合并、拆分、元数据和封面写操作；
- `epub3-migrator`：EPUB2/legacy 到 EPUB3 的 plan/apply 流程。

已有 `epub-package-nav-auditor` 只负责只读审计，不与写操作 skill 合并。

## 错误处理

- 公共 façade 捕获内部错误时继续输出现有错误类型和退出码。
- harness 参数不足时由 argparse 返回 2；业务拒绝写出时返回 1。
- 输出已经存在时仍拒绝覆盖，除非旧 CLI 原本明确支持覆盖选项。
- package split/merge/metadata/cover 的输入、输出和附属文件都写入 JSON 报告。
- 拆分过程中若发现现有测试未覆盖的行为，先添加 characterization test，再迁移代码。

## 测试

- 先为测试 fixture、共享 helper、每个 façade re-export、每个 harness 和 skill/index 写失败测试。
- 每迁移一个模块，运行对应现有测试，确保输出和异常兼容。
- skills 按 `writing-skills` 要求逐个验证，不批量写完后再补测试。
- 最终运行所有 `scripts/test_*.py`、coverage detector pytest、Swift tests、demo build/validators、popup validator、EPUB lint、contracts/adapters/skills/docs validator 和 `git diff --check`。

## 暂不处理

- 不把所有 `scripts/` 一次性变成新的安装包。
- 不恢复 GUI，也不让 GUI 依赖 Python。
- 不把 capability manifest、provider catalog 和 inventory 强制合成一个生成源；先保持现有验证器治理。
- 不提交或推送，除非用户另行要求。
