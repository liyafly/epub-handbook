# EPUBHandbook Swift core

Swift 6.3 package for Apple-native EPUB processing. It is not a replacement for the existing Python system: Python remains the implementation for current agent skills, harnesses, and public Python CLIs; this package is the only implementation macOS / future iOS GUI may call.

## Boundaries

- `EPUBContracts` holds versioned, Codable artifact/report/plan types.
- `EPUBRuntime` supplies native workspace, gate, transaction, rollback, audit, and provider-policy primitives; it does not execute a skill, harness, or Python command.
- `EPUBArchive` rejects absolute and traversal ZIP entry paths before package code accesses them.
- `EPUBPackage` reads `container.xml` and OPF facts with Foundation `XMLParser`.
- `EPUBInspection` converts read-only package facts into `InspectionReport`.
- `EPUBValidation` covers normalized XHTML text / anchors, core OPF metadata, spine, cover resource bytes and paths, DRM (including only the documented stale-reference and opt-in standard-font exceptions), plus a native popup-note structure/resource validator.
- `EPUBStructuredTransforms` uses SwiftSoup in explicit XML mode for XHTML DOM transformations. `PopupFootnoteArchiveNormalizer` preserves existing local icon resources, recognizes a complete Sigil `noteref_N → footnote_N` section, injects a package-local default icon plus OPF manifest item only for text markers, normalizes missing `lang` / `xml:lang` from OPF language, emits a same-file grouped `aside + ol/li` body, and writes a new EPUB. It never invokes a skill, harness, or Python process.
- `EPUBCLI` is the native service boundary for Swift CLI transactions; `epub-handbook-swift` is the executable JSON surface. The macOS/iOS GUI does not invoke this executable—it calls the same libraries directly when that feature is wired.

`SWXMLHash` is intentionally not used: OPF/container XML has fixed structure and direct `XMLParser` delegates keep namespace/error policy explicit. The decision and POC evidence are in [../docs/experiments/2026-06-20-swift-xml-html-library-evaluation.md](../docs/experiments/2026-06-20-swift-xml-html-library-evaluation.md).

## Verify

```sh
cd swift
swift test

# Read-only native package report
swift run epub-handbook-swift inspect \
  --input /absolute/path/book.epub --format json

# Full native text/anchor/package redline report
swift run epub-handbook-swift validate-redlines \
  --before /absolute/path/before.epub \
  --after /absolute/path/after.epub --format json

# Explicitly approved native popup transaction; workspace keeps before/staging audit data
swift run epub-handbook-swift run epub.notes.popup.normalize \
  --input /absolute/path/before.epub \
  --output /absolute/path/after.epub \
  --workspace /absolute/path/work --format json
```

`validate-redlines` emits the versioned shape described by
[`swift-redline-validation.schema.json`](../contracts/schemas/v1/swift-redline-validation.schema.json).
`normalize-popup` runs native popup, text/anchor, metadata/spine/cover/DRM gates
before `Transaction` commits. Noteref/backlink display controls (numeric labels,
icons, and `◎`) are ignored for text-invariance comparison, while every note
body remains protected by the same redline gate.

The generated macOS app is separate: see [../gui/README.md](../gui/README.md).
