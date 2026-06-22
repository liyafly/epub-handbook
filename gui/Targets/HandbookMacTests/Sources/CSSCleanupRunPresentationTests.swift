import XCTest
@testable import HandbookMac
import EPUBContracts

final class CSSCleanupRunPresentationTests: XCTestCase {
    func testSuggestedOutputNameKeepsStemAndAddsCleanupSuffix() {
        XCTAssertEqual(
            CSSCleanupRunPresentation.suggestedOutputName(for: URL(filePath: "/tmp/book.epub")),
            "book-css-cleaned.epub"
        )
    }

    func testCompleteReportShowsOutputAndCompletedGates() {
        let report = RunReport(
            schemaVersion: "1",
            status: .complete,
            input: .init(uri: URL(filePath: "/tmp/source.epub"), kind: .epub),
            output: .init(uri: URL(filePath: "/tmp/cleaned.epub"), kind: .epub),
            events: [
                .init(step: "css-cleanup", status: .completed, message: "CSS files 3 -> 2"),
                .init(step: "package-redlines", status: .completed),
            ]
        )

        let presentation = CSSCleanupRunPresentation.detail(for: report)

        XCTAssertTrue(presentation.contains("status: complete"))
        XCTAssertTrue(presentation.contains("output: /tmp/cleaned.epub"))
        XCTAssertTrue(presentation.contains("css-cleanup: completed"))
    }
}
