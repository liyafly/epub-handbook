import XCTest
@testable import HandbookMac
import EPUBContracts

final class PreflightPresentationTests: XCTestCase {
    func testSummaryRendersPackageFacts() throws {
        let report = InspectionReport(
            schemaVersion: "1",
            artifact: .init(uri: URL(filePath: "/tmp/book.epub"), kind: .epub),
            findings: [],
            status: .pass,
            facts: [
                "opfPath": "OEBPS/package.opf",
                "manifestItemCount": "2",
                "spineItemCount": "1",
                "navigationPath": "nav.xhtml",
            ]
        )

        XCTAssertEqual(
            PreflightPresentation.summary(for: report),
            """
            status: pass
            OPF: OEBPS/package.opf
            manifest items: 2
            spine items: 1
            navigation: nav.xhtml
            """
        )
    }

    func testCleanupAvailabilityRequiresSuccessfulInspectionAndInput() {
        XCTAssertFalse(CSSCleanupAvailability.isEnabled(reportStatus: .fail, hasInput: true))
        XCTAssertFalse(CSSCleanupAvailability.isEnabled(reportStatus: .pass, hasInput: false))
        XCTAssertTrue(CSSCleanupAvailability.isEnabled(reportStatus: .pass, hasInput: true))
    }
}
