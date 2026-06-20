import Foundation
import EPUBContracts
import Testing

@Test("execution plan round-trips without implementation command strings")
func executionPlanRoundTripsAsNeutralContract() throws {
    let artifact = ArtifactReference(
        uri: URL(filePath: "/tmp/book.epub"),
        kind: .epub,
        contentDigest: "sha256:" + String(repeating: "a", count: 64),
        logicalPath: "book.epub"
    )
    let plan = ExecutionPlan(
        schemaVersion: "1",
        artifact: artifact,
        blockers: ["preflight-required"],
        steps: [
            .init(
                id: "preflight",
                capability: .init(rawValue: "epub.package.nav.audit"),
                kind: .inspect,
                dependsOn: [],
                requiresApproval: false
            ),
            .init(
                id: "normalize-notes",
                capability: .init(rawValue: "epub.notes.popup.normalize"),
                kind: .transform,
                dependsOn: ["preflight"],
                requiresApproval: true
            ),
        ]
    )

    let encoded = try JSONEncoder().encode(plan)
    let decoded = try JSONDecoder().decode(ExecutionPlan.self, from: encoded)

    #expect(decoded == plan)
    #expect(String(decoding: encoded, as: UTF8.self).contains("epub.notes.popup.normalize"))
    #expect(!String(decoding: encoded, as: UTF8.self).contains("scripts/"))
}
