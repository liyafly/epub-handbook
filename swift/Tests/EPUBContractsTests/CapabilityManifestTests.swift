import Foundation
import Testing
@testable import EPUBContracts

@Test("capability manifests decode the shared contract shape")
func capabilityManifestDecodesSharedContractShape() throws {
    let data = Data(
        """
        {
          "schemaVersion": "1",
          "id": "epub.notes.popup-normalize",
          "version": "1.0.0",
          "kind": "transformer",
          "legacySkillSlugs": ["epub-popup-footnote-converter"],
          "inputSchema": "contracts/schemas/v1/artifact-reference.schema.json",
          "outputSchema": "contracts/schemas/v1/run-report.schema.json",
          "redLines": ["text", "anchors", "metadata"],
          "permissions": {"requiresWriteAccess": true, "network": "none"},
          "requires": ["epub.package.nav.audit"],
          "adapters": ["openai", "claude", "mcp", "cli", "gui"]
        }
        """.utf8
    )

    let manifest = try JSONDecoder().decode(CapabilityManifest.self, from: data)

    #expect(manifest.schemaVersion == "1")
    #expect(manifest.id.rawValue == "epub.notes.popup-normalize")
    #expect(manifest.kind == .transformer)
    #expect(manifest.redLines == [.text, .anchors, .metadata])
    #expect(manifest.permissions.requiresWriteAccess)
    #expect(manifest.adapters.contains(.gui))
}
