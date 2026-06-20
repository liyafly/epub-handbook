import Foundation
import Testing
@testable import EPUBContracts

@Test("artifact references round-trip through versioned JSON")
func artifactReferenceRoundTripsThroughJSON() throws {
    let artifact = ArtifactReference(
        uri: URL(fileURLWithPath: "/tmp/book.epub"),
        kind: .epub,
        contentDigest: "sha256:" + String(repeating: "a", count: 64),
        logicalPath: "OEBPS/package.opf"
    )

    let data = try JSONEncoder().encode(artifact)
    let decoded = try JSONDecoder().decode(ArtifactReference.self, from: data)

    #expect(decoded == artifact)
}
