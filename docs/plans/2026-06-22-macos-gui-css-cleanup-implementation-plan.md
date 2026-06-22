# macOS GUI CSS Cleanup Implementation Plan

> 执行状态：2026-06-22 已完成。保留这份测试驱动的执行记录，`HandbookMac` 只暴露单文件 CSS cleanup 写入动作。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AppKit-only “执行 CSS Cleanup” write path that creates a new EPUB through the validated native Swift transaction.

**Architecture:** `PreflightViewController` remains the window-level coordinator. It retains only the user-selected input URL, gathers a user-selected output URL through `NSSavePanel`, then invokes `SwiftCLIService.normalizeCSS` inside a temporary App Support workspace. `CSSCleanupRunPresentation` is a pure formatter tested without AppKit panels.

**Tech Stack:** AppKit, Foundation, UniformTypeIdentifiers, `EPUBCLI`, Swift Testing/XCTest, Tuist.

---

### Task 1: Add project dependency, write entitlement, and presentation tests

**Files:**
- Modify: `gui/Project.swift`
- Modify: `gui/Targets/HandbookMac/HandbookMac.entitlements`
- Create: `gui/Targets/HandbookMac/Sources/Features/Cleanup/CSSCleanupRunPresentation.swift`
- Create: `gui/Targets/HandbookMacTests/Sources/CSSCleanupRunPresentationTests.swift`

- [x] **Step 1: Write failing presentation tests**

```swift
func testSuggestedOutputNameKeepsStemAndAddsCleanupSuffix() {
    XCTAssertEqual(
        CSSCleanupRunPresentation.suggestedOutputName(for: URL(filePath: "/tmp/book.epub")),
        "book-css-cleaned.epub"
    )
}

func testCompleteReportShowsOutputAndCompletedGates() {
    let presentation = CSSCleanupRunPresentation.detail(for: completeReport)
    XCTAssertTrue(presentation.contains("status: complete"))
    XCTAssertTrue(presentation.contains("output: /tmp/cleaned.epub"))
    XCTAssertTrue(presentation.contains("css-cleanup: completed"))
}
```

- [x] **Step 2: Verify RED**

Run: `cd gui && mise exec -- tuist generate --no-open && xcodebuild test -workspace EPUBHandbook.xcworkspace -scheme HandbookMac -destination 'platform=macOS' -only-testing:HandbookMacTests/CSSCleanupRunPresentationTests`

Expected: compile failure because `CSSCleanupRunPresentation` does not exist.

- [x] **Step 3: Add smallest GUI-facing formatter and package access**

Add `EPUBCLI` to the app target dependency. Implement only:

```swift
enum CSSCleanupRunPresentation {
    static func suggestedOutputName(for input: URL) -> String
    static func detail(for report: RunReport) -> String
}
```

Change the Sandbox entitlement from `com.apple.security.files.user-selected.read-only` to `com.apple.security.files.user-selected.read-write`.

- [x] **Step 4: Verify GREEN and commit**

Run the targeted test from Step 2.

Commit: `git commit -m "feat: prepare GUI CSS cleanup write access"`

### Task 2: Add the AppKit cleanup action

**Files:**
- Modify: `gui/Targets/HandbookMac/Sources/Features/Preflight/PreflightViewController.swift`
- Modify: `gui/Targets/HandbookMacTests/Sources/PreflightPresentationTests.swift`

- [x] **Step 1: Write failing action-state test**

Extract a narrow pure state helper used by the controller:

```swift
func testCleanupAvailabilityRequiresSuccessfulInspectionAndInput() {
    XCTAssertFalse(CSSCleanupAvailability.isEnabled(reportStatus: .fail, hasInput: true))
    XCTAssertFalse(CSSCleanupAvailability.isEnabled(reportStatus: .pass, hasInput: false))
    XCTAssertTrue(CSSCleanupAvailability.isEnabled(reportStatus: .pass, hasInput: true))
}
```

- [x] **Step 2: Verify RED**

Run: `cd gui && xcodebuild test -workspace EPUBHandbook.xcworkspace -scheme HandbookMac -destination 'platform=macOS' -only-testing:HandbookMacTests/PreflightPresentationTests`

Expected: compile failure because `CSSCleanupAvailability` does not exist.

- [x] **Step 3: Implement the minimal AppKit flow**

Add a disabled `执行 CSS Cleanup…` button. On click:

1. show `NSAlert` explaining that a new artifact will be written and four native gates must pass;
2. show `NSSavePanel` with `suggestedOutputName` and `.epub` content type;
3. create `Application Support/CSSCleanup/<UUID>`;
4. start security-scoped access for input/output, invoke `SwiftCLIService.normalizeCSS`, then stop access;
5. update the existing status label and detail text with `CSSCleanupRunPresentation.detail`.

The controller must retain no global panel or workspace state and must not replace its selected input with the output artifact.

- [x] **Step 4: Verify GREEN and commit**

Run the focused GUI test from Step 2, then the full `HandbookMac` test scheme.

Commit: `git commit -m "feat: add GUI CSS cleanup action"`

### Task 3: Complete native and repository validation

**Files:**
- Modify: `docs/plans/2026-06-20-swift-core-macos-gui-plan.md`

- [x] **Step 1: Update the Swift GUI plan**

Record that native CSS cleanup is GUI-available only through the explicit one-file AppKit action; arbitrary CSS editing and other write capabilities remain unavailable.

- [x] **Step 2: Run every gate**

Run:

```sh
cd swift && swift test && swift build --product epub-handbook-swift
cd .. && uv run python scripts/test_swift_python_parity.py
uv run python scripts/validate_contracts.py
uv run python scripts/validate_ai_entrypoints.py
uv run python scripts/validate_skills_basic.py
git diff --check
cd gui && mise exec -- tuist generate --no-open
xcodebuild test -workspace EPUBHandbook.xcworkspace -scheme HandbookMac -destination 'platform=macOS' -quiet
```

Expected: every command succeeds. EPUBCheck remains CI-only.

- [x] **Step 3: Commit**

Commit: `git commit -m "test: validate GUI CSS cleanup action"`
