# Swift XML / HTML 库评估：SwiftSoup 与 SWXMLHash

> 结论：**基础 Swift EPUB 核心不引入 SWXMLHash；XHTML 结构化转换 target 采用 SwiftSoup。**
>
> `EPUBArchive`、`EPUBPackage` 不使用 SwiftSoup。SwiftSoup 用于允许规范化重写的 XHTML 结构化转换 / 源材料接入；SWXMLHash 不引入。macOS/iOS GUI 不直接依赖任一解析库。

## 评估目标

`epub-handbook` 的 Swift 核心需要同时满足：

1. 读取 EPUB ZIP、`container.xml`、OPF、nav、NCX 和 XML 型 XHTML。
2. 对已有 EPUB 保持最小 diff；正文不变性、锚点、metadata、spine、cover 是红线。
3. 支持未来 macOS/iOS GUI，但 GUI 不持有 EPUB DOM 或 XML parser。
4. 不把“能容忍脏 HTML”误当作“能安全重写 EPUB XHTML”。

## 调研快照（2026-06-20）

| 项目 | 当前验证版本 | 发布 / 维护信号 | SwiftPM 信息 |
|---|---|---|---|
| [SwiftSoup](https://github.com/scinfu/SwiftSoup) | `2.13.5` | release feed 显示 2026-05-14 发布 `2.13.5`。 | `Package.swift` 为 tools 6.0，声明 macOS 10.15 / iOS 13 等平台。 |
| [SWXMLHash](https://github.com/drmohundro/SWXMLHash) | `8.1.1` | release feed 显示 2025-07-01 发布 `8.1.1`；8.0.0 标注 Swift 6 支持。 | `Package.swift` 为 tools 6.0，并使用 Swift 6 language mode。 |

来源：[SwiftSoup README](https://github.com/scinfu/SwiftSoup/blob/master/README.md)、[SwiftSoup Package.swift](https://github.com/scinfu/SwiftSoup/blob/master/Package.swift)、[SwiftSoup releases](https://github.com/scinfu/SwiftSoup/releases)、[SWXMLHash README](https://github.com/drmohundro/SWXMLHash/blob/master/README.md)、[SWXMLHash Package.swift](https://github.com/drmohundro/SWXMLHash/blob/master/Package.swift)、[SWXMLHash releases](https://github.com/drmohundro/SWXMLHash/releases)。

## POC 方法与结果

临时 SwiftPM POC 固定 `SwiftSoup 2.13.5`、`SWXMLHash 8.1.1`，使用一段**没有 XML 声明**、但带 XHTML namespace、`epub:type` 和 `<!DOCTYPE html>` 的 EPUB XHTML。

### SwiftSoup

POC 对同一输入分别调用：

```swift
let htmlDocument = try SwiftSoup.parse(xhtml)
let xmlDocument = try SwiftSoup.parseXML(xhtml)
```

验证结果：

- 默认 `parse(...)` 走 HTML5 模式，输出把 `<!DOCTYPE html>` 变成小写 `<!doctype html>`，并重排格式与空白。
- `parseXML(...)` 保留大写 doctype 和 namespace，但仍对格式 / 空白重序列化。
- SwiftSoup README 说明：自动 XML 检测依赖开头的 `<?xml ...?>`。许多 EPUB XHTML 没有该声明，因此对 EPUB 不能使用默认 `parse(...)`。

结论：它的 CSS selector、DOM traversal、宽容 HTML 解析很适合**源材料接入**；但任何 `outerHtml()` / DOM serialization 都不满足现有 EPUB 的最小 diff / 文本红线写回要求。

### SWXMLHash

POC 对同一 XHTML 调用：

```swift
let indexer = XMLHash.config { config in
    config.shouldProcessNamespaces = true
    config.detectParsingErrors = true
}.parse(xhtml)
```

验证结果：

- `epub:type` 可通过索引树读取，但必须明确决定 namespace 配置。
- README 与源码均表明它是 `XMLParser` 的 dictionary-of-arrays / indexer 包装。
- `detectParsingErrors` 默认是 `false`；POC 对不闭合 XML 验证了默认配置不返回 `parsingError`，开启配置后才会报错。
- lazy 模式虽可降低整树内存，但 README 明确说明 lazy 情况下无法在访问元素前可靠获知错误。

结论：它适合小型、一次性的只读 XML 查询，但不能保存原始 XML，也不提供 EPUB 所需的 source-preserving writer。对 OPF、nav、NCX 的固定结构读取，直接使用 `XMLParser` 更透明、错误策略更可控，且少一个运行时依赖。

## 对 Swift 核心的决策

| 用例 | 决策 | 原因 |
|---|---|---|
| OPF / nav / NCX / `container.xml` 读取 | Foundation `XMLParser` + 专用 delegate / stack model | XML 型格式、结构固定、需精确 namespace 和错误策略。 |
| EPUB XHTML 结构化写回 | `EPUBStructuredTransforms` 采用 SwiftSoup `parseXML(...)` + DOM serialization | 已接受规范化 diff；selector、属性编辑和节点操作无需自建 DOM。 |
| XHTML 只读选择器分析 | `EPUBStructuredTransforms` 采用 SwiftSoup | 使用与变换相同的 XML 解析模式和 fixture。 |
| 任意 / 脏 HTML 源材料接入 | `EPUBHTMLIngest` 可依赖同一 SwiftSoup target | HTML5 容错正是该场景需要的能力。 |
| 核心 XML 解析 | 不引入 SWXMLHash | 其底层仍是 `XMLParser`，默认错误策略与树索引抽象不符合核心需求。 |
| macOS / iOS GUI | 不直接依赖二者 | GUI 只消费 `InspectionReport`、`ExecutionPlan`、`RunEvent`。 |

在 Apple 平台，`XMLParser` 由 `Foundation` 提供，POC 也确认 `shouldResolveExternalEntities` 可设为 `false`。跨平台 source 如未来需要 Linux，可采用：

```swift
import Foundation
#if canImport(FoundationXML)
import FoundationXML
#endif
```

## 未来引入 SwiftSoup 的门槛

SwiftSoup 的 `EPUBStructuredTransforms` / `EPUBHTMLIngest` target 必须满足全部条件：

1. 只新增到 `EPUBStructuredTransforms`、`EPUBHTMLIngest` 或只读 analysis target；`EPUBArchive`、`EPUBPackage` 不依赖它。
2. EPUB XHTML 只能调用 `parseXML(...)` 或 `Parser.xmlParser()`；禁止默认 `parse(...)`。
3. `outerHtml()` / DOM serialization 只允许在用户批准的结构化 transform transaction 中写入**新 output artifact**；不原地改输入。
4. 用无 XML 声明、namespace、MathML、SVG、弹注、竖排、Ruby 和实体的 fixture 做回归；每次都跑文本不变性、package validation 和人工 diff review。
5. 固定版本并纳入 SwiftPM lockfile；升级必须单独评审解析 / 序列化差异。

## 结论

SwiftSoup 是一个健康且能力强的**隔离的结构化转换依赖**，不是 EPUB 基础包。它能把 XHTML 节点查询、属性替换、节点插入 / 删除的实现和测试成本从“自建 DOM”降到业务变换本身；代价是 serialization diff，现在作为可接受并受验证门控制的行为。`swift/EPUBStructuredTransforms` 已固定 SwiftSoup `2.13.5` 并以 XML-mode 无 XML 声明的 EPUB XHTML fixture 验证。SWXMLHash 可用，但对于本项目的 OPF / XHTML 处理没有超过直接 `XMLParser` 的收益，因此不采用。
