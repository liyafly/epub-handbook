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

```text
epub-pro/
├── spec/
│   ├── ir.schema.json
│   ├── annotations.schema.json
│   ├── fontspec.schema.json
│   ├── opf-metadata.template.xml
│   └── fixtures/
│       └── 01-basic-cjk/
│           ├── input.ir.json
│           ├── annotations.json
│           ├── fontspec.json
│           ├── expected.epub
│           └── expected.epubcheck.txt
├── core-swift/
├── core-kotlin/
├── sidecar-font-subset/
├── apps/
└── docs/
    └── 技术架构_v1.md（本文件软链）
```

---

## 4. 共享规范 spec/

所有平台**只信 spec**。任何字段变动先改 schema、再改两侧实现，CI 拒绝单边修改。

### 4.1 Manuscript IR (`ir.schema.json`)

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
    "direction": "ltr|rtl|vertical-rl"
  }
}
```

### 4.2 弹注候选 (`annotations.schema.json`)

```jsonc
{
  "version": "1",
  "items": [
    {
      "id": "fn-001",
      "chapter": "ch01",
      "anchor": { "blockIndex": 3, "runStart": 12, "runEnd": 18 },
      "content": [{ "kind": "p", "text": "「沂水」：今山东省临沂市。" }],
      "kind": "footnote|glossary|reference"
    }
  ]
}
```

### 4.3 字体规范 (`fontspec.schema.json`)

```jsonc
{
  "version": "1",
  "roles": {
    "body": { "fontFamilyStack": ["SourceHanSerifSC", "serif"], "files": ["fonts/SourceHanSerifSC-Regular.otf"] }
  },
  "subset": {
    "strategy": "auto|forceAll|none",
    "extraCodepoints": ["U+4E00..U+9FA5"]
  }
}
```

### 4.4 Golden 夹具协议

- 输入：`input.ir.json` / `annotations.json` / `fontspec.json`
- 期望：`expected.epub` 与 `expected.epubcheck.txt`
- 规范化：`meta@property` 排序、manifest 按 id 排序、XHTML 规范化、字体按 SHA-256 比对。

---

## 5. Swift 核心 `core-swift/`

- 依赖：SwiftSoup、ZIPFoundation、swift-argument-parser、swift-log、swift-collections。
- 顶层 API：`EPUBBuilder.build(manuscript, annotations, fonts, output)`。
- 关键点：弹注同文件闭环、`NoSubsetter/BundledRustSubsetter/PyftsubsetSubsetter`、`mimetype` 首条 STORED。

---

## 6. Kotlin 核心 `core-kotlin/`

- 依赖：jsoup、commons-compress、kotlinx.serialization、Clikt、kotlin-logging、json-schema-validator。
- 接口与 Swift 对称：`FontSubsetter`、`EPUBBuilder`。
- Android：通过 `BinarySource` 适配 `ContentResolver`；sidecar 强制禁用（`NoSubsetter`）。

---

## 7. font-subset sidecar (Rust)

- 目标：跨平台静态二进制（约 3MB 量级）。
- CLI：文件模式 / stdin-stdout 模式。
- 退出码：`0` 成功，`2~5` 分别表示参数/解析/子集/woff2 失败。
- stderr：分阶段 JSON 日志，供 BuildReport 汇总。

---

## 8. 子集器：发现、配置、降级

- 发现顺序：Bundled → 用户配置 pyftsubset → PATH font-subset。
- 配置：桌面保存偏好（Swift AppStorage / JVM config）。
- 策略：**不降级**。用户选定子集器不可用时构建立刻失败。
- 元数据回写：subsetter、font stats、built-at。

---

## 9. Golden 测试与一致性

- Swift 与 Kotlin 对同一 fixture 双流水线产出，均需对齐 `expected.epub` 与 `expected.epubcheck.txt`。
- 首批 fixture：`01-basic-cjk`、`02-footnotes`、`03-fontspec-no-subset`、`04-fontspec-subset`、`05-vertical-cjk`。

---

## 10. epubcheck 集成

- 桌面端可 bundle `epubcheck.jar` + JRE（17+），或依赖系统 Java。
- Swift/Kotlin 均走 `java -jar` 调用；移动端不集成。

---

## 11. 构建与分发

- Swift：Xcode（iOS/macOS）、`swift build`（Linux/Windows CLI）。
- Kotlin：Android `assembleRelease`、JVM `:cli:installDist`/`shadowJar`。
- Rust sidecar：按目标三元组 `cargo build --release --target ...`。

---

## 12. 错误模型

统一错误码：`IR_SCHEMA_VIOLATION`、`ANNOTATION_ANCHOR_NOT_FOUND`、`FONT_FILE_UNREADABLE`、`SUBSETTER_UNAVAILABLE`、`SUBSETTER_FAILED`、`ZIP_WRITE_FAILED`、`EPUBCHECK_FAILED`。

---

## 13. 里程碑

- M1 Swift 骨架
- M2 弹注
- M3 字体 + sidecar
- M4 Kotlin 镜像
- M5 pyftsubset 路径
- M6 桌面壳 + epubcheck UI

---

## 14. 不做（v1 不包含）

- 仓内不做 AI 推理，只消费 Skill JSON。
- 不直接输出 KF8/MOBI。
- 不做在线书库/云同步。
- 不走 KMP 到 iOS。
- 不做 Android 字体子集化。
