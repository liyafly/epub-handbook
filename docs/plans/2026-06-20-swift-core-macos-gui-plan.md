# Swift 核心库与 macOS GUI 实施计划

> 状态：S0 已完成；S1 的 archive / container / OPF / package inspection 基座与 S2 的 AppKit 只读垂直切片已实现。S3 的原生全量红线、Sigil popup normalize、默认图标/OPF 写入、语言壳层补齐、transaction JSON CLI 与 Python 双跑基线已实现。原生 CSS cleanup 已完成纯 Swift scanner、plan、archive/OPF/XHTML 写回和 CLI 双跑，但尚未达到 GUI 可用或 `swift-primary` 门槛；GUI apply 流程仍未实现，且不在本轮范围内。
>
> 前置：先完成 [三层项目重构计划](2026-06-20-project-three-layer-refactor-plan.md) 的 R0、R1；Swift 只消费 contract，绝不实现或调用 Python skill / harness。

## 目标与边界

本计划只负责用 Swift 实现 agent-neutral core、原生 transaction / gate 与 AppKit-first macOS 产品表面。它不删除 Python，不重写现有 skill / harness，也不把 GUI 做成 Python subprocess 前端。

首个可交付闭环是：

```text
选择 EPUB → Swift preflight → InspectionReport → ExecutionPlan
→ 用户确认 → Swift popup footnote normalize → text invariance
→ 新 output EPUB + RunReport
```

能力必须先在 CLI / fixture / Python 双跑中通过，再进入 GUI。iOS 只在 macOS 闭环稳定后使用同一个 Swift package 接入。

## 工具链与版本

| 项目 | 选择 | 规则 |
|---|---|---|
| 编译器 | Xcode 26.5 / Apple Swift 6.3.2 | 采用当前本机最新正式 Swift；Swift language mode 6。 |
| 核心与 CLI | SwiftPM | `swift-tools-version: 6.3`；业务逻辑和测试只在 package 内。 |
| Xcode 工程 | Tuist 4.200.5 | 以 `Project.swift` 为 source of truth；生成 workspace 不提交。 |
| 工具版本 | mise | 在 `mise.toml` 固定 Tuist；升级工具必须独立验证。 |
| macOS UI | AppKit-first + 少量 SwiftUI hosting | 不使用纯 SwiftUI App。 |
| iOS UI（后续） | UIKit 文件流 + SwiftUI 功能页 | 不复制 macOS 的文件路径语义。 |

部署目标：macOS 15；iOS target 创建时为 iOS 18。SDK 仍使用 Xcode 26.5 提供的 macOS / iOS 26.5 SDK；高版本 API 以 feature availability 分支处理。

### 2026-06-20 已实施基线

- `swift/` 是 Swift tools 6.3 / language mode 6 的 package，已实现 `EPUBContracts`、`EPUBRuntime`、`EPUBArchive`、`EPUBPackage`、`EPUBInspection`、`EPUBValidation`、`EPUBStructuredTransforms`、`EPUBCLI` 与 `epub-handbook-swift` executable；SwiftPM unit tests 覆盖 report/plan、provider policy、安全 archive path、archive rewrite、transaction rollback、container/OPF、text / anchors / metadata / spine / cover / DRM、popup 结构 / 图标资源和 Swift CLI transaction。
- 读取 XML 固定结构直接使用 Foundation `XMLParser`；可接受规范化 XHTML 写回的 transform 使用 SwiftSoup `2.13.5` `parseXML(...)`。`PopupFootnoteArchiveNormalizer` 保留既有图片图标；对完整 Sigil `noteref_N/footnote_N` 结构可生成 grouped aside，对文本标记只在同一 native transaction 内注入 `Images/note.png` 与 OPF manifest item，并从 OPF `dc:language` 补齐缺失的 XHTML `lang` / `xml:lang`。它不调用 Python。`scripts/test_swift_python_parity.py` 独立运行 Python redline / popup validator 与 Swift CLI，覆盖 pass/fail、metadata、DRM 与 popup artifact。
- `gui/Project.swift` + `Tuist.swift` 已生成 `HandbookMac` 和 `HandbookMacTests`。初始窗口是 AppKit，使用 sandbox security-scoped file access；它只调用 Swift `PackageInspector` 并显示 `InspectionReport`，不调用 Python 或读取 `SKILL.md`。

## Swift Package 结构

```text
swift/
├── Package.swift
├── Package.resolved
├── Sources/
│   ├── EPUBContracts/       # 对应 contracts/ 的 Swift Codable 值类型
│   ├── EPUBArchive/         # ZIP、mimetype、path safety
│   ├── EPUBPackage/         # container、OPF、manifest、spine、nav/NCX
│   ├── EPUBInspection/      # 只读 preflight facts
│   ├── EPUBValidation/      # text/anchors/metadata/spine/cover/DRM/popup validators
│   ├── EPUBStructuredTransforms/ # SwiftSoup XML-mode XHTML 结构化变换
│   ├── EPUBStylesheets/     # 无损 CSS scanner、cleanup plan、archive/OPF/XHTML 写回
│   ├── EPUBRuntime/         # registry、workspace、gate、transaction、events
│   ├── EPUBCLI/             # native CLI service / transaction boundary
│   └── EPUBHandbookSwiftCLI/# epub-handbook-swift JSON executable
├── Tests/
│   ├── EPUBContractsTests/
│   ├── EPUBArchiveTests/
│   ├── EPUBPackageTests/
│   ├── EPUBInspectionTests/
│   ├── EPUBValidationTests/
│   ├── EPUBStructuredTransformsTests/
│   ├── EPUBRuntimeTests/
│   ├── EPUBCLITests/
│   └── Fixtures/
└── README.md
```

### 模块依赖

```mermaid
flowchart LR
    C[EPUBContracts] --> A[EPUBArchive]
    C --> P[EPUBPackage]
    A --> P
    P --> I[EPUBInspection]
    P --> V[EPUBValidation]
    A --> S[EPUBStylesheets]
    P --> S
    V --> T[EPUBStructuredTransforms]
    C --> R[EPUBRuntime]
    I --> CLI[EPUBHandbookCLI]
    V --> CLI
    T --> CLI
    S --> CLI
    R --> CLI
```

| Module | 责任 | 禁止依赖 |
|---|---|---|
| `EPUBContracts` | `ArtifactReference`、`Finding`、report、plan、manifest、error、event。 | ZIP/XML/UI、Python、prompt。 |
| `EPUBArchive` | 安全 ZIP 读取 / 写入、entry order、`mimetype` stored / first、路径逃逸阻断。 | agent、AppKit、业务排版判断。 |
| `EPUBPackage` | container、OPF、manifest、spine、nav、NCX、相对路径。 | 自动纠错正文或 UI。 |
| `EPUBInspection` | preflight 与事实扫描。 | 写入 EPUB。 |
| `EPUBValidation` | 正文不变性、锚点、metadata、spine、cover、加密标记。 | 修改输入以掩盖错误。 |
| `EPUBStructuredTransforms` | SwiftSoup XML-mode 的白名单 XHTML DOM 变换和 path map。 | 未经批准的语义改写、任意 HTML 编辑器。 |
| `EPUBStylesheets` | CSS lossless scanner、保守 declaration cleanup、stylesheet graph、OPF/XHTML link 写回。 | cascade / renderer、外部 CSS bridge、Python。 |
| `EPUBRuntime` | registry、workspace、gate、transaction、取消、event stream。 | 具体 EPUB 规则、shell 路径、UI。 |
| `EPUBCLI` / `EPUBHandbookSwiftCLI` | 参数、JSON result、exit code 与 native transaction orchestration。 | 业务算法复制、Python bridge。 |

基础包使用 `ZIPFoundation`；`EPUBStructuredTransforms` / popup validator 额外固定 `SwiftSoup 2.13.5`，只用于明确授权的 XHTML DOM 结构化变换与结构校验。CLI 只使用小型原生参数解析，避免再引入 command-parser dependency。使用 `Foundation`、`FoundationXML`、`Codable`、Swift Testing；不使用 PythonKit、CocoaPods、Carthage 或大型 DI / reactive framework。

2026-06-20 决定：可接受 XHTML 规范化重写，不要求 source-level 最小 diff。因此 SwiftSoup 可调用 `parseXML(...)` 与 DOM serialization；但每次 apply 仍必须写入新的 output artifact，并保留 before、红线校验、EPUB lint 与人工 diff review。`EPUBArchive`、`EPUBPackage` 的 ZIP / OPF 读取仍直接使用 ZIPFoundation / Foundation `XMLParser`，不依赖 SwiftSoup。

## Swift 与 Python 的关系

Swift 只消费项目 contract、fixture 与 validator 定义。它不读取 `SKILL.md`，不实现 / 调用 harness，也不调用 `scripts/*.py`。Python skill / harness 仅服务现有 CLI 与 AI Agent。

| 场景 | 允许的 provider | 原因 |
|---|---|---|
| Swift 单元、Swift CLI integration | Swift only | 原生 core 不依赖 Python entrypoint。 |
| 外部 parity test | 独立 test runner 调度 Python 与 Swift JSON artifact | 只比较标准化输出；不将 Python bridge 链接进 Swift package。 |
| macOS App 正常运行 | Swift provider only | 避免 Sandbox、环境版本与取消语义问题。 |
| iOS App | Swift provider only | iOS 不存在可依赖的 Python runtime。 |
| 未迁移 capability | Python CLI / agent 继续可用；GUI 显示 unavailable。 | 不伪装支持，也不分裂行为。 |

Python adapter 的 request / result JSON 文件属于 Python CLI / Agent 层。Swift 只读写统一 report / plan JSON；不会传递 Swift object、GUI object 或 Python object。

## 首批 Swift capability

| Capability ID | Swift provider | Python 对照 | 进入 GUI 的门槛 |
|---|---|---|---|
| `epub.package.nav.audit` | `PackageInspector` | `epub_preflight_harness.py` / `epub_ai_harness.py`。 | blocker、finding severity、exit code 对齐。 |
| `epub.text.invariance` | 原生 `TextInvarianceValidator` | `validate_text_invariance.py`。 | text、anchors、metadata、spine、cover 对照通过。 |
| `epub.notes.popup.normalize` | 原生 `PopupFootnoteArchiveNormalizer` + `PopupFootnoteValidator` | popup skill、converter、`validate-popup-notes.sh`。 | 保留既有图标、redline、popup validator 与 EPUB lint 全通过。 |
| `epub.css.layering.optimize` | `CSSCleanupArchiveTransformer` + `CSSCleanupValidator` | `epub_css_cleanup.py`。 | 同 fixture 双跑、EPUB lint、三次 CI 与人工 diff review 后才考虑 GUI。 |
| `epub.structure.normalize` | 后续。 | `epub_structure_tool.py`。 | 先只有 dry-run / path map；apply 后置。 |
| `epub.package.merge-split` | 后续。 | `epub_package_tool.py`。 | 不在首个 GUI 范围。 |

首个写入 capability 选择 popup footnote，是因为它有明确结构约束、现有 demo、validator 和“保留已有本地图标”的回归要求，能证明 transaction / gate 设计是否正确。

## macOS GUI 设计

### Tuist 工程

```text
gui/
├── Project.swift
├── Tuist.swift
├── Config/
│   ├── Debug.xcconfig
│   └── Release.xcconfig
├── Targets/
│   ├── HandbookMac/
│   │   ├── App/
│   │   ├── Windowing/
│   │   ├── Navigation/
│   │   ├── Features/
│   │   │   ├── Preflight/
│   │   │   ├── Cleanup/
│   │   │   ├── Report/
│   │   │   └── Settings/
│   │   ├── Hosting/
│   │   └── Resources/
│   ├── HandbookMacTests/
│   └── HandbookMacUITests/
├── README.md
└── .gitignore
```

`HandbookMac` 以 local Swift package dependency 引用 `../swift`。Tuist manifest、xcconfig、source 和测试入 Git；生成的 `.xcworkspace` / `.xcodeproj`、DerivedData、`.build` 不入 Git。

```sh
mise install
cd gui
tuist generate
open *.xcworkspace
```

### AppKit-first 的边界

| 区域 | 技术 | 原因 |
|---|---|---|
| 生命周期、菜单、快捷键、窗口 | AppKit `NSApplicationDelegate` / window controller | macOS 原生多窗口与 command 行为。 |
| 项目导航、任务队列、复杂表格、inspector | `NSSplitViewController`、`NSOutlineView`、`NSTableView` | 适合 EPUB 文件、分阶段 plan 和审计记录。 |
| 导入、导出、最近项目 | `NSOpenPanel`、`NSSavePanel`、security-scoped bookmark | 支持 Sandbox、用户授权与持久访问。 |
| 报告详情、设置、简短表单、空状态 | SwiftUI + `NSHostingController` | 局部复用高，不接管 AppKit navigation。 |
| 长任务进度、取消、错误 | AppKit feature controller 订阅 `RunEvent` | 保留明确的取消 / rollback / retry 状态。 |

View 只消费 feature view model；view model 直接调用原生 `EPUBInspection`、`EPUBValidation`、`EPUBStructuredTransforms` 与 `EPUBRuntime` transaction API。任何 Python skill / harness、XML 规则实现、ZIP 写入细节或 prompt 生成都不出现在 UI target。

### Sandbox 与输出纪律

- UI 负责选取并开始访问 security-scoped URL；Swift core 只接收已授权的 `URL`。
- 输入永远只读，写入必须输出到用户选定位置或 App work directory 的新 artifact。
- 任何 apply 都先将原始输入复制为 `before/source.epub`；`Transaction` 在 redline 或 popup validator 失败时删除 staged output。
- 默认不在 GUI 内运行外部 Python、Java、字体子集或图片压缩工具；将来 capability probe 可报告它们，但不能默默降级。

## 分阶段实施计划

### S0 — 接入项目 contract

- 等待项目重构 R0、R1 完成；读取 capability manifest、schema 和 Python baseline。
- 在 `swift/` 创建 package、test fixture 目录和 `EPUBContracts`。
- 对每一 Swift `Codable` 类型做 JSON round-trip 与 schema compatibility test。

**完成标准：** Swift 能读写项目级 contract；没有业务逻辑落在 GUI 或 `scripts/` 的新分支里。

### S1 — archive / package / inspection

- 实现 archive path 安全、ZIP entry 规则、container、OPF、manifest、spine、nav / NCX。
- 实现 `PackageInspector` 的只读模式与结构化 `InspectionReport`。
- 提供 `epub-handbook-swift inspect <input> --format json`、`plan <input> --format json`。
- 运行 Python 与 Swift baseline 对比。

**完成标准：** fixture 的 blocker、severity、关键 package facts 和 exit code 对齐；没有写入输入。

### S2 — Tuist 与 AppKit 只读垂直切片

- 创建 `mise.toml`、Tuist project、`HandbookMac`、unit test 和 UI test target。
- 完成 AppKit 主窗口、文件选择、Sandbox entitlement、security scope 生命周期和 report 展示。
- GUI 调用 Swift `PackageInspector`，不调用 Python、skill 或 harness。

**完成标准：** 用户可选择 demo EPUB，查看 report，取消任务或文件授权失败时得到结构化错误；输入文件字节不变。

### S3 — redline 与 popup footnote capability

- 已实现原生 `TextInvarianceValidator`、metadata / spine / cover / DRM redline 与 popup validator。
- 已实现 `PopupFootnoteArchiveNormalizer`：same-file grouped aside、完整 Sigil section 识别/拒绝部分转换、保留本地图标、文本 marker 的默认图标和 OPF manifest 注入、XHTML 语言壳层补齐，以及加密包拒绝。
- 已实现 `Workspace`、`Transaction`、gate、rollback、RunReport 及 `epub-handbook-swift` JSON CLI；显式 CLI 命令是本能力的 approval point。
- 已完成一次 Python / Swift parity baseline；仍需三次 CI、`epub_lint.py` 和人工 diff review，才可把 popup 标记为 GUI 可用 / `swift-primary`。

**当前结果：** Swift CLI 已可完成一个可审计的 popup normalize transaction；macOS GUI 尚未绑定写入能力，继续保持只读。

### S3.1 — CSS cleanup capability

- 已实现纯 Swift `EPUBStylesheets`：lossless top-level scanner，只解析保守 qualified-rule 子集；at-rule 和无法安全解析的 CSS 保留，不参与 factoring / dedupe / scope merge。
- 已实现 Python cleanup 对应的标题装饰行、漏分号、三条系统字体链、normalized digest 去重、三份同 shape stylesheet factoring、override、可选互斥页面 scope merge，以及 `mimetype` 保持与 CSS archive removal。
- 已实现 `normalize-css` 与 `run epub.css.layering.optimize`，在 native transaction 中依次执行 `preflight`、`css-cleanup`、`text-and-anchors`、`package-redlines`；不调用 Python、skill 或 harness。
- 已在相同 fixture 下与 Python `epub_css_cleanup.py` 双跑，并对两个 output 分别运行 `epub_lint.py` 和 Python text/package redline。

**当前结果：** native CLI 已可生成可审计 CSS cleanup artifact；GUI 继续只读。此 capability 仍是 dual-run，未完成连续三次 CI 和人工 diff review 前不得标记为 GUI available 或 `swift-primary`。

### S4 — CleanupHarness 与 plan UI

- 实现 plan dependency、manual review gate、RunEvent、cancel / rollback。
- GUI 以任务队列展示 `ExecutionPlan`、阻断项、批准点、进度和 artifact。
- CLI 同步提供 JSON result 与 JSONL event 模式。

**完成标准：** CLI 和 GUI 使用同一 plan；输出路径、before、after、reports 和 transaction log 可审计。

### S5 — 高风险能力逐项扩展

- 先实现 structure normalize dry-run 与 path map。
- 再按独立 capability 迁移 apply、EPUB3 migration、legacy note fallback；CSS cleanup 的原生 core 已完成，后续只补稳定性证据和产品接入决策。
- merge / split、封面、字体、图片、Kindle 外部工具保持后置。

**完成标准：** 每项能力都有 manifest、fixture、Python dual-run、redline、对应 validator 和人工 review 证据。

### S6 — iOS 准备与接入

- 只在 S4 稳定后创建 `HandbookiOS` target。
- UIKit 使用 `UIDocumentPickerViewController` / `UIDocumentBrowserViewController` 做文件入口；SwiftUI 仅用于 report、settings、confirmation。
- 首期 iOS 仅公开 inspection 与无需外部工具的 Swift capability。

**完成标准：** iOS 与 macOS 没有两套 EPUB 逻辑；不支持的 capability 明确显示 unavailable。

## 验证矩阵

| 改动 | 至少运行 |
|---|---|
| Swift contracts / core | `cd swift && swift test`；schema compatibility；fixture golden tests。 |
| Swift CLI | integration tests；Python normalized baseline comparison；exit code comparison。 |
| macOS Project.swift | `mise install`、`cd gui && tuist generate`。 |
| macOS App | 生成 workspace 后运行对应 `xcodebuild test`。 |
| EPUB 写入 | Swift / Python text invariance、`scripts/validate-popup-notes.sh --epub <artifact>`（如适用）、`scripts/epub_lint.py <artifact>`、人工 diff review。 |
| 任何改动 | `git diff --check`。 |

## 不做

- 不让 macOS / iOS GUI 调用 PythonKit 或用户环境中的 Python。
- 不将 SwiftUI 作为整个 macOS App 的导航与生命周期框架。
- 不先做 Kotlin、Rust font sidecar、WASM、Java binding、云同步或在线书库。
- 不在没有 Python parity、contract、fixture、validator 的情况下把 Python capability 宣称为 Swift 已接管。
- 不在首个 GUI 中提供任意 XHTML 编辑或整条自动 cleanup loop。
