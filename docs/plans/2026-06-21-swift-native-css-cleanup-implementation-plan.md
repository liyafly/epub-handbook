# Swift Native CSS Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a pure-Swift EPUB CSS cleanup capability equivalent to the existing Python cleanup path, with native transaction/CLI execution and Python parity evidence.

**Architecture:** `EPUBStylesheets` owns a lossless scanner, parsed qualified-rule subset, cleanup planning and EPUB archive transformation. Existing archive/package/validation/runtime modules remain the package and transaction boundaries. Unsupported CSS is retained and skipped with a report warning instead of being rewritten.

**Tech Stack:** Swift 6.3, Foundation, CryptoKit SHA-256, ZIPFoundation, SwiftSoup XML mode, Swift Testing, Tuist AppKit integration.

---

### Task 1: Add the stylesheet module and a lossless scanner

**Files:**
- Modify: `swift/Package.swift`
- Create: `swift/Sources/EPUBStylesheets/CSSDocument.swift`
- Create: `swift/Tests/EPUBStylesheetsTests/CSSDocumentTests.swift`

- [ ] **Step 1: Write failing scanner tests**

```swift
@Test("CSS scanner preserves comments strings functions and nested at-rules")
func scannerPreservesOpaqueCSS() throws {
    let source = """/* keep */ a { content: \";}\"; background: url(data:x;y); } @media screen { b { color: red; } }"""
    let document = try CSSDocument.parse(source)
    #expect(document.statements.count == 3)
    #expect(document.serialized == source)
}
```

- [ ] **Step 2: Verify RED**

Run: `cd swift && swift test --filter scannerPreservesOpaqueCSS`

Expected: compile failure because `CSSDocument` does not exist.

- [ ] **Step 3: Implement scanner and raw statement model**

```swift
public struct CSSDocument: Sendable, Hashable {
    public let statements: [CSSStatement]
    public var serialized: String { statements.map(\.raw).joined() }
    public static func parse(_ source: String) throws -> CSSDocument
}
```

Track comment, string, escape, parenthesis, bracket and brace depth; split only at top-level rule boundaries. Preserve every raw segment exactly.

- [ ] **Step 4: Verify GREEN and commit**

Run: `cd swift && swift test --filter CSSDocumentTests`

Expected: selected tests pass.

Commit: `git commit -m "feat: add lossless Swift CSS scanner"`

### Task 2: Implement declarations, sanitization and fingerprints

**Files:**
- Create: `swift/Sources/EPUBStylesheets/CSSCleanupPrimitives.swift`
- Create: `swift/Tests/EPUBStylesheetsTests/CSSCleanupPrimitivesTests.swift`

- [ ] **Step 1: Write failing tests for each Python-compatible normalization**

```swift
@Test("sanitizer rewrites only approved legacy font families")
func sanitizerRewritesApprovedFonts() throws {
    let output = try CSSSanitizer.sanitize("p { font-family: \"SimHei\"; } .x { font-family: Other; }")
    #expect(output.fontRewrites == 1)
    #expect(output.css.contains("Noto Sans CJK SC"))
    #expect(output.css.contains("font-family: Other"))
}
```

- [ ] **Step 2: Verify RED**

Run: `cd swift && swift test --filter sanitizerRewritesApprovedFonts`

Expected: compile failure because `CSSSanitizer` does not exist.

- [ ] **Step 3: Implement declaration parser and primitives**

Implement semicolon splitting only at zero nested depth, `font-family` mappings, ornament-line removal, missing-semicolon repair, canonical shape, and SHA-256 fingerprint after comment/whitespace removal and case fold.

- [ ] **Step 4: Verify GREEN and commit**

Run: `cd swift && swift test --filter CSSCleanupPrimitivesTests`

Expected: selected tests pass.

Commit: `git commit -m "feat: add native CSS cleanup primitives"`

### Task 3: Build inventory and cleanup planner

**Files:**
- Create: `swift/Sources/EPUBStylesheets/StylesheetInventory.swift`
- Create: `swift/Sources/EPUBStylesheets/CSSCleanupPlanner.swift`
- Create: `swift/Tests/EPUBStylesheetsTests/CSSCleanupPlannerTests.swift`

- [ ] **Step 1: Write failing planner fixtures**

```swift
@Test("planner factors three same-shape stylesheets and emits overrides")
func plannerFactorsSameShapeStylesheets() throws {
    let plan = try CSSCleanupPlanner.plan(fixtures: makeThreeStyleFixture())
    #expect(plan.generated.contains { $0.path.value.hasSuffix("clean-shared-01.css") })
    #expect(plan.replacements[try ArchivePath("OEBPS/Styles/style0003.css")]?.count == 2)
}
```

- [ ] **Step 2: Verify RED**

Run: `cd swift && swift test --filter plannerFactorsSameShapeStylesheets`

Expected: compile failure because planner does not exist.

- [ ] **Step 3: Implement read-only CSS graph and plan**

Read OPF `text/css` items, XHTML stylesheet link references and page sets. Generate plan operations for same-shape factoring, exact normalized duplicates, and optional scope merge only when page sets are disjoint. Produce explicit warnings for opaque/unsupported CSS and overlapping local stylesheets.

- [ ] **Step 4: Verify GREEN and commit**

Run: `cd swift && swift test --filter CSSCleanupPlannerTests`

Expected: selected tests pass.

Commit: `git commit -m "feat: plan native EPUB CSS cleanup"`

### Task 4: Write CSS/archive changes and validate package references

**Files:**
- Modify: `swift/Sources/EPUBArchive/EPUBArchiveRewriter.swift`
- Create: `swift/Sources/EPUBStylesheets/CSSCleanupArchiveTransformer.swift`
- Create: `swift/Sources/EPUBStylesheets/CSSCleanupValidator.swift`
- Create: `swift/Tests/EPUBStylesheetsTests/CSSCleanupArchiveTransformerTests.swift`

- [ ] **Step 1: Write failing artifact test**

```swift
@Test("CSS archive cleanup writes replacements additions removals and valid manifest links")
func archiveCleanupWritesNewValidEPUB() throws {
    let report = try CSSCleanupArchiveTransformer.transform(source: source, to: output, options: .init())
    #expect(report.factoredStylesheets == 3)
    #expect(try CSSCleanupValidator.validate(epub: output).isValid)
    #expect(try TextInvarianceValidator.validate(before: source, after: output).isValid)
}
```

- [ ] **Step 2: Verify RED**

Run: `cd swift && swift test --filter archiveCleanupWritesNewValidEPUB`

Expected: compile failure because archive transformer does not exist.

- [ ] **Step 3: Implement transform**

Extend archive rewriting with an explicit removal set. Apply CSS plan replacements/additions/removals, update OPF manifest items in XML mode, and rewrite only affected XHTML stylesheet links/body classes with SwiftSoup. Reject encrypted archives and invalid local stylesheet links.

- [ ] **Step 4: Verify GREEN and commit**

Run: `cd swift && swift test --filter CSSCleanupArchiveTransformerTests`

Expected: selected tests pass.

Commit: `git commit -m "feat: write native EPUB CSS cleanup artifacts"`

### Task 5: Add transaction and JSON CLI surface

**Files:**
- Modify: `swift/Sources/EPUBCLI/SwiftCLIService.swift`
- Modify: `swift/Sources/EPUBHandbookSwiftCLI/main.swift`
- Modify: `swift/Tests/EPUBCLITests/SwiftCLIServiceTests.swift`

- [ ] **Step 1: Write failing transaction test**

```swift
@Test("Swift CSS cleanup transaction commits only after all native redlines pass")
func swiftCLICSSCleanupIsTransactional() async throws {
    let report = await SwiftCLIService.normalizeCSS(input: input, output: output, workspaceRoot: workspace, options: .init())
    #expect(report.status == .complete)
    #expect(FileManager.default.fileExists(atPath: output.path))
}
```

- [ ] **Step 2: Verify RED**

Run: `cd swift && swift test --filter swiftCLICSSCleanupIsTransactional`

Expected: compile failure because `normalizeCSS` does not exist.

- [ ] **Step 3: Implement transaction and command parsing**

Add `normalize-css` and `run epub.css.layering.optimize`; require `preflight`, `css-cleanup`, `text-and-anchors`, and `package-redlines` gates before commit. Use the same `RunReport` JSON contract as native popup normalization.

- [ ] **Step 4: Verify GREEN and commit**

Run: `cd swift && swift test --filter SwiftCLIServiceTests && swift run epub-handbook-swift --help`

Expected: test passes and help lists both CSS commands.

Commit: `feat: expose native CSS cleanup CLI`

### Task 6: Prove Python/Swift parity and complete verification

**Files:**
- Modify: `scripts/test_swift_python_parity.py`
- Modify: `docs/plans/2026-06-20-swift-core-macos-gui-plan.md`
- Modify: `docs/plans/2026-06-20-project-three-layer-refactor-plan.md`

- [ ] **Step 1: Add failing cross-runtime fixture**

```python
def test_css_cleanup_parity() -> None:
  # Both providers produce a valid new EPUB with the same required CSS cleanup facts.
  ...
```

- [ ] **Step 2: Verify RED**

Run: `uv run python scripts/test_swift_python_parity.py`

Expected: failure because CSS command is absent or behavior is not equivalent.

- [ ] **Step 3: Add parity assertions and update plans**

Compare report counters, generated stylesheet presence, link/manifest resolution, text/package redlines and Python `epub_lint.py`; document that GUI stays read-only until native CSS has repeated CI and manual review evidence.

- [ ] **Step 4: Verify all gates and commit**

Run: `cd swift && swift test && swift build --product epub-handbook-swift`

Run: `for script in $(rg --files scripts -g 'test_*.py' | sort); do uv run python "$script" || exit $?; done`

Run: `uv run python scripts/validate_contracts.py && uv run python scripts/validate_ai_entrypoints.py && uv run python scripts/validate_skills_basic.py && git diff --check`

Run: `cd gui && mise exec -- tuist generate --no-open && xcodebuild test -workspace EPUBHandbook.xcworkspace -scheme HandbookMac -destination 'platform=macOS' -quiet`

Expected: every command succeeds; EPUBCheck may remain CI-only as specified in `AGENTS.md`.

Commit: `git commit -m "test: prove native CSS cleanup parity"`
