# epub-pro 技术架构 v1

> 双核（Swift / Kotlin）+ 桌面端可插拔字体子集 sidecar。本文是落地施工蓝图：版本、依赖、接口、目录、测试、分发，都按可执行的精度给出。

---

## 1. 设计原则

1. **AI 与执行解耦**：Skill 只负责语义判断（弹注候选、字体推荐），输出 JSON；客户端只跑确定性管线，不接 AI。
2. **双原生核心**：Apple 平台用 Swift，Android 用 Kotlin。核心代码量预期 2–3k 行/侧，靠**共享 spec + golden fixtures** 保持一致，**不**走 FFI / Multiplatform。
3. **字体子集化外置**：核心只做"安排字体"（@font-face、fallback 链），**子集化通过 sidecar**。桌面端默认内置 Rust 实现，进阶用户可指向自己安装的 `pyftsubset`。移动端不做子集（`NoSubsetter`）。
4. **失败要响**：用户选择的子集器不可用时**抛错**，绝不静默降级——避免产物悄悄变样。
5. **打包元数据可回溯**：每个产物在 OPF metadata 内写入子集器名称/版本/字形数。

---

## 2. 平台 × 职责矩阵

| 平台 | 核心库 | 子集器 | UI | 产物 |
|---|---|---|---|---|
| iOS / iPadOS | EPUBCore (Swift) | NoSubsetter | SwiftUI | App |
| macOS | EPUBCore + EPUBCLI | Bundled Rust（默认）/ pyftsubset | SwiftUI | App + 内嵌 sidecar |
| Linux 桌面 | EPUBCLI | Bundled Rust（默认）/ pyftsubset | 暂 CLI；后续 Tauri 壳 | deb/rpm |
| Windows 桌面 | EPUBCLI | Bundled Rust（默认）/ pyftsubset | 后续 Tauri 壳 | installer |
| Android | EPUBCore (Kotlin) | NoSubsetter | Jetpack Compose | App |
| JVM 服务端/CI | EPUBCore (Kotlin) `:cli` | Bundled Rust / pyftsubset | CLI | jar/zip |

---

## 3. 仓库结构

```
epub-pro/
├── spec/                            # 跨核心共享，唯一真理源
│   ├── ir.schema.json               # Manuscript IR JSON Schema
│   ├── annotations.schema.json
│   ├── fontspec.schema.json
│   ├── opf-metadata.template.xml
│   └── fixtures/
│       └── 01-basic-cjk/
│           ├── input.ir.json
│           ├── annotations.json
│           ├── fontspec.json
│           ├── expected.epub
│           └── expected.epubcheck.json
│
├── core-swift/                      # Swift Package
│   ├── Package.swift
│   ├── Sources/
│   │   ├── EPUBCore/
│   │   │   ├── IR/
│   │   │   ├── Parsing/
│   │   │   ├── Annotation/
│   │   │   ├── Rendering/
│   │   │   ├── Packaging/
│   │   │   ├── Fonts/
│   │   │   ├── Zip/
│   │   │   └── Pipeline/
│   │   └── EPUBCLI/
│   └── Tests/EPUBCoreTests/
│
├── core-kotlin/                     # Gradle multi-module
│   ├── settings.gradle.kts
│   ├── core/                        # JVM + Android target
│   ├── cli/                         # JVM
│   └── android/                     # Android 适配（ContentResolver 等）
│
├── sidecar-font-subset/             # Rust workspace
│   ├── Cargo.toml
│   └── src/main.rs
│
├── apps/
│   ├── ios/                         # Xcode 项目
│   ├── macos/
│   ├── android/
│   └── desktop-tauri/               # 可选
│
└── docs/
    └── 技术架构_v1.md（本文件软链）
```

---

## 4. 共享规范 spec/

所有平台**只信 spec**。任何字段变动先改 schema、再改两侧实现，CI 拒绝单边修改。

### 4.1 Manuscript IR (`ir.schema.json`)

最小字段集：

```jsonc
{
  "version": "1",
  "book": {
    "id": "urn:uuid:…",
    "title": "…",
    "language": "zh-Hans",
    "author": ["…"],
    "publisher": "…",
    "pubdate": "2026-05-19",
    "pageProgression": "ltr|rtl"
  },
  "spine": [
    {
      "id": "ch01",
      "title": "第一章",
      "writingMode": "horizontal-tb|vertical-rl",
      "blocks": [
        { "kind": "h1", "text": "…" },
        { "kind": "p",  "text": "…", "runs": [/* 内联标记 */] },
        { "kind": "image", "src": "img/01.png", "alt": "…" },
        { "kind": "blockquote", "blocks": [/* nested */] }
      ]
    }
  ],
  "resources": {
    "images": [ { "id":"img1","path":"img/01.png","mime":"image/png" } ]
  }
}
```

`pageProgression` 只映射 OPF `spine page-progression-direction`。局部竖排不改翻页方向，使用章节或块级 `writingMode: vertical-rl` 渲染为 CSS `writing-mode`。

### 4.2 弹注候选 (`annotations.schema.json`)

```jsonc
{
  "version": "1",
  "items": [
    {
      "id": "fn-001",
      "chapter": "ch01",
      "anchor": { "blockIndex": 3, "runStart": 12, "runEnd": 18 },
      "content": [
        { "kind": "p", "text": "「沂水」：今山东省临沂市。" }
      ],
      "kind": "footnote|glossary|reference"
    }
  ]
}
```

> 由 Skill 产出或人工标注；脚本完全不判断"该不该标"。
> `runStart` / `runEnd` 使用 NFC 规范化后的 Unicode scalar/code point 半开区间，避免 Swift 与 Kotlin 字符串索引模型不一致。

### 4.3 字体规范 (`fontspec.schema.json`)

```jsonc
{
  "version": "1",
  "roles": {
    "body":     { "fontFamilyStack": ["SourceHanSerifSC","serif"], "files": ["fonts/SourceHanSerifSC-Regular.otf"] },
    "heading":  { "fontFamilyStack": ["SourceHanSerifSC","serif"], "weight": 700, "files": ["fonts/SourceHanSerifSC-Bold.otf"] },
    "quote":    { "fontFamilyStack": ["Kaiti","serif"],            "files": ["fonts/Kaiti.ttf"] },
    "monospace":{ "fontFamilyStack": ["JetBrainsMono","monospace"],"files": ["fonts/JetBrainsMono-Regular.woff2"] }
  },
  "subset": {
    "strategy": "auto|forceAll|none",
    "extraCodepoints": ["U+4E00..U+9FA5"]
  }
}
```

### 4.4 Golden 夹具协议

`spec/fixtures/<case>/`：
- `input.ir.json` / `annotations.json` / `fontspec.json` 是输入
- `expected.epub` 是期望产物（二进制；CI 解压后逐文件按规范化后比对）
- `expected.epubcheck.json` 是固定 epubcheck 版本后的规范化 JSON 摘要

规范化规则（避免无意义 diff）：
- `<meta>` 元素按 `@property` 字典序排序输出
- OPF `<manifest>` 按 `@id` 排序，`<spine>` 保持原序
- XHTML 在比对前用 SwiftSoup / jsoup 规范化输出
- 字体二进制按 SHA-256 比对，不做语义对比

---

## 5. Swift 核心 `core-swift/`

### 5.1 `Package.swift`

```swift
// swift-tools-version: 5.10
import PackageDescription

let package = Package(
    name: "EPUBCore",
    platforms: [.iOS(.v16), .macOS(.v13), .visionOS(.v1)],
    products: [
        .library(name: "EPUBCore", targets: ["EPUBCore"]),
        .executable(name: "epub-pro", targets: ["EPUBCLI"]),
    ],
    dependencies: [
        .package(url: "https://github.com/scinfu/SwiftSoup.git",                from: "2.7.2"),
        .package(url: "https://github.com/weichsel/ZIPFoundation.git",          from: "0.9.19"),
        .package(url: "https://github.com/apple/swift-argument-parser.git",     from: "1.5.0"),
        .package(url: "https://github.com/apple/swift-log.git",                 from: "1.6.0"),
        .package(url: "https://github.com/apple/swift-collections.git",         from: "1.1.4"),
    ],
    targets: [
        .target(name: "EPUBCore", dependencies: [
            "SwiftSoup", "ZIPFoundation",
            .product(name: "Logging", package: "swift-log"),
            .product(name: "OrderedCollections", package: "swift-collections"),
        ]),
        .executableTarget(name: "EPUBCLI", dependencies: [
            "EPUBCore",
            .product(name: "ArgumentParser", package: "swift-argument-parser"),
        ]),
        .testTarget(name: "EPUBCoreTests", dependencies: ["EPUBCore"],
                    resources: [.copy("Fixtures")]),
    ]
)
```

`Tests/EPUBCoreTests/Fixtures` 由 CI 从仓库根目录的 `spec/fixtures` 同步或软链生成；SPM package 本身不直接引用 package root 之外的资源路径。

**库选择理由**

| 用途 | 选 | 否决 | 理由 |
|---|---|---|---|
| XHTML 解析/构建 | **SwiftSoup** | XMLCoder, Fuzi | jQuery 风格 DOM 操作最适合弹注注入；同时能做规范化序列化 |
| ZIP | **ZIPFoundation** | Compression(系统) | 纯 Swift、支持指定首条 `mimetype` STORED 无 extra |
| CLI | **swift-argument-parser** | Commander | Apple 出品、与 Swift Concurrency 兼容 |
| 日志 | **swift-log** | os.log | 跨平台（Linux/Windows） |
| 有序 Map | **swift-collections** | NSDictionary | OPF manifest 需要稳定序 |

> 不引入 `SwiftXML`、`AEXML` 等：写 OPF/nav/NCX 完全用 SwiftSoup 的 XML 模式（`Parser.xmlParser()`），减少一道依赖。

### 5.2 模块拓扑

```
EPUBCore
├── IR              // 纯数据类型 + Codable，对照 spec/ir.schema.json
├── Parsing         // .md/.docx/.html/.txt → IR  (后置功能，先支持 IR 直入)
├── Annotation      // AnnotationCandidates 注入 XHTML
├── Rendering       // IR + assets → XHTML 文档集合
├── Packaging       // OPF / nav.xhtml / META-INF / container.xml
├── Fonts           // FontPlan + FontSubsetter 协议 + 三实现
├── Zip             // EPUB 容器写入（mimetype 规则）
└── Pipeline        // EPUBBuilder：把以上串起来
```

### 5.3 顶层 API

```swift
public struct EPUBBuilder: Sendable {
    public let logger: Logger
    public let subsetter: FontSubsetter

    public init(subsetter: FontSubsetter = NoSubsetter(),
                logger: Logger = Logger(label: "epub-pro.core")) {
        self.subsetter = subsetter
        self.logger = logger
    }

    public func build(
        manuscript: Manuscript,
        annotations: AnnotationCandidates = .empty,
        fonts: FontPlan = .none,
        to output: URL
    ) async throws -> BuildReport
}

public struct BuildReport: Sendable, Codable {
    public let outputURL: URL
    public let bytes: Int
    public let durationMS: Int
    public let subsetterUsed: String?           // 写进 OPF 元数据
    public let fontStats: [FontStat]
    public let warnings: [Warning]
}
```

### 5.4 关键算法：弹注注入

```swift
public struct AnnotationInjector {
    public func inject(into document: SwiftSoup.Document,
                       chapterID: String,
                       items: [AnnotationItem]) throws {
        let body = try document.body()!
        let aside = try ensureFootnoteAside(in: body)
        let list = try ensureFootnoteList(in: aside)

        for item in items where item.chapter == chapterID {
            // 1. 定位文本节点：按 (blockIndex, runStart..runEnd) 锚点找到 inline range 末尾
            let insertionPoint = try locateInsertionPoint(document, anchor: item.anchor)

            // 2. 在锚点后插入图片图标 noteref，href 指向本文件内对应 li.footnote-item
            let noteID = "note-\(item.id)"
            let footnoteID = "footnote-\(item.id)"
            try insertionPoint.after("""
                <sup>
                  <a id="\(noteID)" class="noteref-icon"
                     epub:type="noteref" role="doc-noteref"
                     href="#\(footnoteID)">\(renderNoteIcon())</a>
                </sup>
            """)

            // 3. 所有注释聚合到同一个 aside > ol 内，每条注释是可回跳的 li
            let li = try Element(Tag("li"), "")
                .addClass("footnote-item")
                .attr("id", footnoteID)
            try li.html("""
                <p class="footnote">
                  <a class="footnote-back"
                     epub:type="backlink"
                     role="doc-backlink"
                     href="#\(noteID)">◎</a>
                  \(renderInline(item.content))
                </p>
            """)
            try list.appendChild(li)
        }
    }
}
```

**重要不变量**
- `epub:type="noteref"` 与 `aside epub:type="footnote"` 必须**在同一 XHTML 文件**——这是 EPUB 3 弹注 reader 的硬性要求。
- 每个 XHTML 文件最多一个 `aside epub:type="footnote"`，本文件内所有注释聚合到 `ol.footnote-list > li.footnote-item`。
- noteref 的 `href` 指向对应 `li.footnote-item`，而不是指向外层 aside。
- noteref 必须有唯一 `id`，用于 backlink 从注释回跳。
- `role="doc-noteref" / role="doc-footnote"` 一起给，提升无障碍与不支持弹注的 reader 的回退体验。

### 5.5 FontPlan & FontSubsetter

```swift
public protocol FontSubsetter: Sendable {
    var name: String { get }                                  // "bundled-rust" / "pyftsubset" / "none"
    func version() async throws -> String
    func subset(font: Data,
                codepoints: Set<Unicode.Scalar>,
                output: FontFormat) async throws -> Data
}

public enum FontFormat: String, Sendable { case otf, ttf, woff2 }

public struct NoSubsetter: FontSubsetter {
    public init() {}
    public var name: String { "none" }
    public func version() async throws -> String { "n/a" }
    public func subset(font: Data, codepoints: Set<Unicode.Scalar>, output: FontFormat) async throws -> Data {
        return font  // 直通：全字形嵌入
    }
}

public struct BundledRustSubsetter: FontSubsetter {
    public let binaryURL: URL
    public init(binaryURL: URL) { self.binaryURL = binaryURL }
    public var name: String { "bundled-rust" }
    public func version() async throws -> String {
        try await ProcessRunner.run(binaryURL, args: ["--version"]).stdout
    }
    public func subset(font: Data, codepoints: Set<Unicode.Scalar>, output: FontFormat) async throws -> Data {
        let cps = codepoints.map { String(format: "U+%04X", $0.value) }.joined(separator: ",")
        return try await ProcessRunner.runReturningStdoutData(
            binaryURL,
            args: ["--stdin-stdout", "--codepoints", cps, "--format", output.rawValue],
            stdin: font
        )
    }
}

public struct PyftsubsetSubsetter: FontSubsetter {
    public let binaryURL: URL
    public init(binaryURL: URL) { self.binaryURL = binaryURL }
    public var name: String { "pyftsubset" }
    public func version() async throws -> String {
        // pyftsubset 没有 --version；通过 fonttools 入口检测
        try await ProcessRunner.run(binaryURL, args: ["--help"]).firstLine
    }
    public func subset(font: Data, codepoints: Set<Unicode.Scalar>, output: FontFormat) async throws -> Data {
        // pyftsubset 用 --unicodes=U+xxxx,U+yyyy；woff2 用 --flavor=woff2
        let cps = codepoints.map { String(format: "U+%04X", $0.value) }.joined(separator: ",")
        let flavorArg = output == .woff2 ? ["--flavor=woff2"] : []
        return try await ProcessRunner.viaTempFiles(
            binaryURL,
            args: ["--unicodes=\(cps)"] + flavorArg + ["--output-file={out}", "{in}"],
            input: font
        )
    }
}
```

> `ProcessRunner` 是 EPUBCore 内部的小工具，封装 `Foundation.Process`，提供 stdin/stdout 字节流、退出码、超时。iOS 编译时通过 `#if !os(iOS)` 排除 sidecar 实现，强制使用 `NoSubsetter`。

### 5.6 ZIP 容器

```swift
import ZIPFoundation

public struct EPUBPackager {
    public func write(to url: URL, entries: OrderedDictionary<String, Data>) throws {
        try? FileManager.default.removeItem(at: url)
        guard let archive = Archive(url: url, accessMode: .create) else { throw EPUBError.zipCreate }

        // 1. mimetype 必须第一个，STORED，无 extra
        let mimetype = entries["mimetype"]!
        try archive.addEntry(with: "mimetype",
                             type: .file,
                             uncompressedSize: Int64(mimetype.count),
                             compressionMethod: .none,
                             provider: { pos, sz in mimetype.subdata(in: Int(pos)..<Int(pos)+sz) })

        // 2. 其余按 DEFLATE
        for (path, data) in entries where path != "mimetype" {
            try archive.addEntry(with: path, type: .file,
                                 uncompressedSize: Int64(data.count),
                                 compressionMethod: .deflate,
                                 provider: { pos, sz in data.subdata(in: Int(pos)..<Int(pos)+sz) })
        }
    }
}
```

---

## 6. Kotlin 核心 `core-kotlin/`

### 6.1 模块与 `build.gradle.kts`

`settings.gradle.kts`：
```kotlin
include(":core", ":cli", ":android")
```

`core/build.gradle.kts`：
```kotlin
plugins {
    kotlin("jvm") version "2.0.21"
    kotlin("plugin.serialization") version "2.0.21"
}
dependencies {
    implementation("org.jsoup:jsoup:1.18.1")
    implementation("org.apache.commons:commons-compress:1.27.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0")
    implementation("io.github.oshai:kotlin-logging-jvm:7.0.0")
    implementation("com.networknt:json-schema-validator:1.5.3") // 校验 spec
    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.11.3")
}
```

`cli/build.gradle.kts`（JVM CLI）：
```kotlin
plugins { application; kotlin("jvm") version "2.0.21" }
application { mainClass = "pro.epub.cli.MainKt" }
dependencies {
    implementation(project(":core"))
    implementation("com.github.ajalt.clikt:clikt:5.0.1")
}
```

`android/build.gradle.kts`：
```kotlin
plugins { id("com.android.library"); kotlin("android") }
android {
    namespace = "pro.epub.android"
    compileSdk = 35
    defaultConfig { minSdk = 26 }
}
dependencies {
    implementation(project(":core"))
    implementation("androidx.documentfile:documentfile:1.0.1")
    implementation("androidx.datastore:datastore-preferences:1.1.1")
}
```

**库选择理由**

| 用途 | 选 | 否决 | 理由 |
|---|---|---|---|
| XHTML | **jsoup** | Ksoup, kotlinx-html | 与 SwiftSoup API 对称（同源 fork），跨平台 EPUB 解析事实标准；Android 上无问题 |
| ZIP | **commons-compress** | java.util.zip, zip4j | 控制 `setMethod(STORED)` 且能精准操纵 entry extra 字段；EPUB 的 mimetype 规则需要这个 |
| JSON | **kotlinx.serialization** | Moshi, Gson | Kotlin 原生、与 sealed class 直接配合 |
| CLI | **Clikt** | kotlinx-cli | 文档/生态最成熟 |
| 日志 | **kotlin-logging** | SLF4J 直用 | Kotlin DSL，底层仍是 SLF4J |
| JSON Schema | **networknt json-schema-validator** | 手写校验 | 共享 spec 校验 |

### 6.2 接口（与 Swift 对称）

```kotlin
sealed interface FontSubsetter {
    val name: String
    suspend fun version(): String
    suspend fun subset(font: ByteArray,
                       codepoints: IntArray,
                       output: FontFormat): ByteArray
}

enum class FontFormat { OTF, TTF, WOFF2 }

data object NoSubsetter : FontSubsetter {
    override val name = "none"
    override suspend fun version() = "n/a"
    override suspend fun subset(font: ByteArray, codepoints: IntArray, output: FontFormat) = font
}

class BundledRustSubsetter(val binaryPath: Path) : FontSubsetter { /* 同语义 */ }
class PyftsubsetSubsetter(val binaryPath: Path) : FontSubsetter { /* 同语义 */ }
```

```kotlin
class EPUBBuilder(
    private val subsetter: FontSubsetter = NoSubsetter,
    private val logger: KLogger = KotlinLogging.logger {}
) {
    suspend fun build(
        manuscript: Manuscript,
        annotations: AnnotationCandidates = AnnotationCandidates.EMPTY,
        fonts: FontPlan = FontPlan.NONE,
        output: Path
    ): BuildReport
}
```

### 6.3 Android 适配（`:android` 模块）

```kotlin
// 文件来源用 ContentResolver，不用 java.io.File
interface BinarySource {
    suspend fun bytes(): ByteArray
    val displayName: String
}
class AndroidUriSource(private val resolver: ContentResolver, val uri: Uri) : BinarySource { … }
class FileSource(val path: Path) : BinarySource { … }
```

- `EPUBCore` 的所有 IO 入口接受 `BinarySource`，桌面端用 `FileSource`，Android 用 `AndroidUriSource`。
- Android 上**禁用** sidecar：`AndroidSubsetterFactory` 永远返回 `NoSubsetter`，并把 `fonts.subset.strategy` 强制视为 `none`，再在 BuildReport 里加 warning。

---

## 7. font-subset sidecar (Rust)

### 7.1 `Cargo.toml`

```toml
[package]
name = "font-subset"
version = "0.1.0"
edition = "2021"

[dependencies]
clap        = { version = "4.5", features = ["derive"] }
subsetter   = "0.2"          # Typst-team CFF/glyf subsetter
ttf-parser  = "0.24"
woff2       = "0.3"
anyhow      = "1"
serde       = { version = "1", features = ["derive"] }
serde_json  = "1"

[profile.release]
strip = true
lto = true
codegen-units = 1
```

> 体积目标：macOS/Linux/Windows 各 ~3 MB 静态二进制。
> 如需更强的 OT 特性覆盖（GSUB/GPOS 复杂连写），后续可切 `harfbuzz_rs` 的 `subset`；先用 `subsetter` 上线，节省门槛。

### 7.2 CLI 契约

```
font-subset --input <path>      --output <path>
            --codepoints <list>             # "U+4E00..U+9FA5,U+0041..U+007A"
            --format otf|ttf|woff2

font-subset --stdin-stdout
            --codepoints <list>
            --format otf|ttf|woff2          # 字体字节流过 stdin / stdout
```

退出码：

| code | 含义 |
|---|---|
| 0 | 成功 |
| 2 | 参数错误 |
| 3 | 字体解析错误 |
| 4 | 子集化失败 |
| 5 | woff2 编码失败 |

stderr 每阶段输出一行 JSON 日志：
```json
{"event":"parsed","glyphs":65535,"format":"otf"}
{"event":"subsetted","glyphs":3221,"bytes":287340}
{"event":"encoded","format":"woff2","bytes":98344}
```

> Swift / Kotlin 侧捕获 stderr 用于 BuildReport 的 fontStats。

---

## 8. 子集器：发现、配置、降级

### 8.1 发现顺序（桌面）

1. **Bundled**：app bundle / 安装目录里查 `font-subset(.exe)`。在则可用。
2. **Pyftsubset**：读取偏好里用户配置的路径；若空，再扫 PATH。运行 `--help`/`--version` 探测，缓存结果。
3. **PATH 上的 font-subset**（用户自编译版本，可作高级选项）。

### 8.2 偏好存储

**Swift / macOS**：
```swift
@AppStorage("subsetter.kind") var kind: SubsetterKind = .bundled
@AppStorage("subsetter.pyftsubsetPath") var pyftsubsetPath: String = ""
```

**Kotlin / Android & JVM**：
- Android：`DataStore<Preferences>`，key `subsetter_kind`、`subsetter_pyftsubset_path`，但 UI 不暴露选项（永远 None）。
- JVM CLI：读 `~/.config/epub-pro/config.toml`。

### 8.3 选择 UI（桌面 Settings → 字体处理）

```
○ 内嵌 (推荐)              基于 Rust subsetter；零配置，~3 MB
                          已检测：bundled-rust 0.1.0 ✓

○ 系统 pyftsubset          需本机安装 fonttools
                          路径：[/usr/local/bin/pyftsubset    ] [浏览…]
                          已检测：fonttools 4.55.0 ✓
                          适用：复杂 OpenType 特性 / 中日韩出版精修

○ 不子集化（整字库嵌入）       仅调试用；产物会大幅膨胀
```

### 8.4 降级策略：**不降级**

用户当前选择的子集器不可用 → **构建立刻失败**，UI 弹出"选项不可用，请检查路径或切换到 Bundled"。绝不悄悄退回到别的子集器，避免产物风格漂移、用户误以为还是按设置走的。

### 8.5 OPF 元数据回写

```xml
<package prefix="epubpro: https://epub-pro.local/vocab#">
  <metadata>
    <meta property="epubpro:subsetter">pyftsubset 4.55.0</meta>
    <meta property="epubpro:font-stats">SourceHanSerifSC: 3221/65535 glyphs, 287KB→98KB woff2</meta>
    <meta property="epubpro:built-at">2026-05-19T12:00:00Z</meta>
  </metadata>
</package>
```

自定义 `property` 必须通过 OPF `prefix` 声明，避免 epubcheck 将 `x-*` property 视为未知元数据属性；示例 namespace 后续可替换为项目固定 IRI。

---

## 9. Golden 测试与一致性

### 9.1 测试矩阵

每个 fixture 跑两条流水线，分别在 Swift 和 Kotlin CI 上：

```
input.ir.json + annotations.json + fontspec.json
        │
        ├── core-swift  → out.epub  → diff with expected.epub (规范化)
        │                          → epubcheck JSON → diff with expected.epubcheck.json
        │
        └── core-kotlin → out.epub  → 同上
```

### 9.2 规范化比对

新建 `spec/tools/epub-normalize`（小脚本，Swift 或 Python 均可）：
- 解 ZIP → 对每个 XHTML/OPF/NCX 用 jsoup XML 模式重新序列化，元素属性按字典序排
- 二进制资源（图/字体）按 SHA-256 比对
- 输出归一化 manifest 文本，diff 之

### 9.3 第一批必备 fixture

| name | 说明 | 范围 |
|---|---|---|
| `01-basic-cjk` | 三章中文，封面 + 一张图，无弹注，无字体子集 | 全平台 |
| `02-footnotes` | 同上 + 5 处脚注（noteref/aside/li/backlink 闭环） | 全平台 |
| `03-fontspec-no-subset` | 同 01 + 嵌入两款字体，`subset.strategy="none"` | 全平台 |
| `04-fontspec-subset` | 同 03，`subset.strategy="auto"` | 仅桌面 CI |
| `05-vertical-cjk` | 局部 `writingMode = vertical-rl`，`pageProgression` 仍为 `ltr` | 全平台 |

---

## 10. epubcheck 集成

桌面端 bundle 固定版本的 `epubcheck.jar` + 内嵌 JRE（17+）或要求用户已装 Java。Golden 测试只比对规范化后的 JSON 摘要，避免路径、顺序和版本文本造成无意义 diff。

```swift
public struct EPUBValidator {
    public let javaURL: URL
    public let jarURL: URL
    public func validate(_ epub: URL) async throws -> ValidationReport {
        let out = try await ProcessRunner.run(javaURL,
            args: ["-jar", jarURL.path, "--json", "-", epub.path])
        return try JSONDecoder().decode(ValidationReport.self, from: out.stdoutData)
    }
}
```

Kotlin 等价 `EPUBValidator` 调用 `java -jar`。移动端不集成 epubcheck（包体 + 法务理由）。

---

## 11. 构建与分发

### 11.1 Swift

| 目标 | 命令 |
|---|---|
| iOS / macOS app | Xcode 工程消费 SPM；签名走 fastlane |
| Linux CLI | `swift build -c release --triple x86_64-unknown-linux-gnu` |
| Windows CLI | `swift build -c release --triple x86_64-unknown-windows-msvc` |

### 11.2 Kotlin

| 目标 | 命令 |
|---|---|
| Android app | `:app:assembleRelease` |
| JVM CLI | `./gradlew :cli:installDist` → zip |
| 制作 fat jar（CI 用） | `./gradlew :cli:shadowJar` |

### 11.3 sidecar `font-subset`

```
cargo build --release --target x86_64-apple-darwin
cargo build --release --target aarch64-apple-darwin
cargo build --release --target x86_64-pc-windows-msvc
cargo build --release --target x86_64-unknown-linux-gnu
cargo build --release --target aarch64-unknown-linux-gnu
```

分发位置：
- macOS app: `EPUBPro.app/Contents/MacOS/font-subset`
- Windows installer: `font-subset.exe` 与主程序同目录
- Linux deb: `/usr/lib/epub-pro/font-subset`
- 不进 iOS / Android bundle

---

## 12. 错误模型

统一 `EPUBError`，Swift / Kotlin 各自实现同名 case：

| Code | 触发 | 用户行动 |
|---|---|---|
| `IR_SCHEMA_VIOLATION` | 输入 JSON 不符 schema | 修正输入 |
| `ANNOTATION_ANCHOR_NOT_FOUND` | 弹注定位失败 | 检查 anchor 偏移 |
| `FONT_FILE_UNREADABLE` | 字体文件读不到 | 修路径 |
| `SUBSETTER_UNAVAILABLE` | 选定的子集器探测失败 | 改设置 |
| `SUBSETTER_FAILED` | 子集器进程非零退出 | 看 stderr 日志 |
| `ZIP_WRITE_FAILED` | 容器写入失败 | 看磁盘 |
| `EPUBCHECK_FAILED` | epubcheck 报错 | 看 ValidationReport |

---

## 13. 里程碑

| M | 目标 | DoD |
|---|---|---|
| **M1** Swift 骨架 | macOS CLI 可把 `01-basic-cjk` 打包成 epub | `epub-pro build --fixture 01-basic-cjk` 通过 epubcheck |
| **M2** 弹注 | 完成 `02-footnotes`，golden 对比通过 | Swift CI 绿 |
| **M3** 字体 + sidecar | Rust `font-subset` 上线，BundledRustSubsetter 接入；fixture 03/04 通过 | 桌面端 GUI 弹出子集器选择 |
| **M4** Kotlin 镜像 | Kotlin core 跑通 01–03；Android demo 能本地预览生成 EPUB | 同一组 fixture，Kotlin CI 也绿 |
| **M5** pyftsubset 路径 | 桌面端 Settings 接 pyftsubset；fixture 04 在两种子集器下产物均合规 | 文档录入两路对比表 |
| **M6** 桌面壳 + epubcheck UI | macOS app 完整可用；Windows/Linux Tauri 壳跑通同一 CLI | 用户能从 GUI 完成整本书 |

---

## 14. 不做（v1 不包含）

- 任何 AI 推理 / Skill 端：本仓只做执行层，Skill 在另一个仓库 + 通过 JSON 喂入。
- KF8 / MOBI 直出：用 kindlegen 二次转换。
- 在线书库 / 云同步。
- Multiplatform Kotlin → iOS 共享：不走 KMP，避免双重维护负担。
- 安卓上的字体子集化。
