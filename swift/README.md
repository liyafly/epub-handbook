# EPUBHandbook Swift core

Swift 6.3 package for Apple-native EPUB processing. It is not a replacement for the existing Python system: Python remains the implementation for current agent skills, harnesses, and public Python CLIs; this package is the only implementation macOS / future iOS GUI may call.

## Boundaries

- `EPUBContracts` holds versioned, Codable artifact/report/plan types.
- `EPUBRuntime` supplies native workspace, gate, transaction, rollback, audit, and provider-policy primitives; it does not execute a skill, harness, or Python command.
- `EPUBArchive` rejects absolute and traversal ZIP entry paths before package code accesses them.
- `EPUBPackage` reads `container.xml` and OPF facts with Foundation `XMLParser`.
- `EPUBInspection` converts read-only package facts into `InspectionReport`.
- `EPUBValidation` compares normalized XHTML leaf-block text and anchors without treating DOM formatting normalization as a text change; metadata, spine, cover, and DRM checks follow as separate native validators.
- `EPUBStructuredTransforms` uses SwiftSoup in explicit XML mode for XHTML DOM transformations. Its serialization can normalize formatting; every write must go to a new output artifact and pass redline/package validation plus manual diff review.

`SWXMLHash` is intentionally not used: OPF/container XML has fixed structure and direct `XMLParser` delegates keep namespace/error policy explicit. The decision and POC evidence are in [../docs/experiments/2026-06-20-swift-xml-html-library-evaluation.md](../docs/experiments/2026-06-20-swift-xml-html-library-evaluation.md).

## Verify

```sh
cd swift
swift test
```

The generated macOS app is separate: see [../gui/README.md](../gui/README.md).
