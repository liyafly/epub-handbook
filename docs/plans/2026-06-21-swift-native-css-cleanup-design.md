# Swift 原生 CSS Cleanup 设计

> 状态：已批准实施。Python 保留为 CLI / Agent provider 与行为 oracle；Swift 是 macOS/iOS 和原生 CLI provider。

## 目标

以纯 Swift 完整接管 `scripts/epub_css_cleanup.py` 的 EPUB CSS cleanup 能力。它必须在新 EPUB artifact 中完成样式清洗、重复去重、同构 stylesheet factoring、可选局部 stylesheet scope merge、OPF manifest 和 XHTML stylesheet link 更新；不得调用 Python、Node、Java、Rust 或 C/C++ bridge。

## 决策

不采用第三方 CSS parser。现有 Swift 候选不是过期、面向 UIKit 查询，便是 C++ / LGPL bridge，且会在 AST serializer 中丢失 comment 或未知 token。EPUB cleanup 的首要安全性是保留未知 CSS，而不是实现浏览器的 cascade 或 layout engine。

新模块名为 `EPUBStylesheets`。它采用 lossless scanner：每个 top-level CSS statement 都保留原始文本；只有明确支持的 qualified rule 才解析 declaration。`@font-face`、`@media`、`@supports`、custom at-rule 与无法完整解析的规则保留为 opaque raw statement，不能被 factoring、dedupe rewrite 或 scope merge 删除。

## 模块边界

```text
EPUBStylesheets
├── CSSScanner                 Unicode-safe token / block scanner
├── CSSDocument                raw statement + supported qualified-rule model
├── CSSSanitizer               ornament、missing semicolon、font-family patch
├── StylesheetInventory        manifest、XHTML link、reference graph、fingerprint
├── CSSCleanupPlanner          factoring / duplicate / scope merge 决策
├── CSSCleanupArchiveTransform archive replacement / addition / deletion
└── CSSCleanupValidator        manifest、link resolution、layer/report consistency
```

`EPUBArchive` 只增加 archive removal 支持；`EPUBPackage` 只提供已有的 OPF facts。XHTML link/body-class 变更复用 SwiftSoup XML DOM，允许规范化写回；未引用或未被选择的 XHTML 保持字节不变。

## 无损与安全规则

- 输入 CSS 必须是 UTF-8；不能解码即停止，不使用 lossy replacement。
- scanner 必须正确跳过 comment、quoted string、escape、`url()` / 任意 function、`[]`、`()` 和 nested `{}`，以免在 value 内错误切分 `;` 或 `}`。
- `font-family` 仅在声明 property 完全匹配时替换三条已有 Python 映射：`cnepub, serif` / `SimSun`、`SimHei`、`STKaiti`。
- identical dedupe 的 fingerprint 与 Python 一致：去 comment、去 whitespace、case-fold 后 SHA-256；只重写被移除 stylesheet 的 XHTML link。
- factoring 仅处理至少三份、全部 statement 为 supported qualified rule、selector 与 declaration property shape 相同、且至少两份 fingerprint 不同的 stylesheet。
- scope merge 仅处理引用 page 集合完全不交叠的 local stylesheet；在 `<body>` 添加 `css-local-NN`，把 selector 安全前缀化；任一 selector 或 at-rule 无法安全处理时整组跳过并报告 warning。
- archive 写入必须保持 `mimetype` stored first，输入不覆盖，DRM/encryption 立即阻断，所有输出均走 native transaction gate。

## 原生 API 与 CLI

`EPUBStylesheets` 对外提供：

```swift
public struct CSSCleanupOptions: Sendable, Codable, Hashable {
    public var mergeScopedLocalStylesheets: Bool
}

public enum CSSCleanupArchiveTransformer {
    public static func analyze(epub: URL) throws -> CSSCleanupInventory
    public static func transform(source: URL, to: URL, options: CSSCleanupOptions) throws -> CSSCleanupReport
}
```

`SwiftCLIService.normalizeCSS(...)` 使用 `Workspace` / `Transaction`，执行 `preflight`、`css-cleanup`、`text-and-anchors`、`package-redlines` gate。`epub-handbook-swift` 增加：

```text
run epub.css.layering.optimize --input <epub> --output <epub> --workspace <dir> --format json
normalize-css --input <epub> --output <epub> --workspace <dir> [--merge-scoped-local-css] --format json
```

GUI 继续只读；未来 GUI 直接调用 `CSSCleanupArchiveTransformer`，不启动这个 executable。

## 验证标准

1. 每个 Swift CSS scanner、sanitizer、planner、archive transform 先写失败的 Swift Testing test。
2. 使用与 Python `test_epub_css_cleanup.py` 等价的 fixture，逐项验证 generated CSS、manifest、link order、scope body class 与 warning。
3. 每个写出 artifact 跑 `TextInvarianceValidator`、`PackageRedlineValidator`、native CSS validator、Python `scripts/epub_lint.py`。
4. `scripts/test_swift_python_parity.py` 增加 CSS fixture，比较 report 关键计数与 required artifact invariants；两个 runtime 不互相调用。
5. 最终运行 `swift test`、全部 Python `test_*.py`、`validate_contracts.py`、`validate_ai_entrypoints.py`、`validate_skills_basic.py`、Tuist generation 和 `xcodebuild test`。

## 非目标

- 不实现 CSS cascade、computed style、selector matching 或 renderer。
- 不修改 `docs/final/` 规则；现有 CSS layer 契约仍是规范来源。
- 不迁移 style preset 的模板资源到 Swift bundle；preset apply 是与 cleanup 分开的后续 capability。
