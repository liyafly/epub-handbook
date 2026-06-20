import Foundation
import EPUBContracts
import EPUBRuntime
import Testing

@Test("manifest catalog resolves a capability and its legacy skill slug")
func manifestCatalogResolvesCapabilityAndLegacySkillSlug() throws {
    let directory = FileManager.default.temporaryDirectory
        .appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }

    let manifest = CapabilityManifest(
        schemaVersion: "1",
        id: .init(rawValue: "epub.notes.popup-normalize"),
        version: "1.0.0",
        kind: .transformer,
        legacySkillSlugs: ["epub-popup-footnote-converter"],
        inputSchema: "contracts/schemas/v1/artifact-reference.schema.json",
        outputSchema: "contracts/schemas/v1/run-report.schema.json",
        redLines: [.text, .anchors, .metadata],
        permissions: .init(requiresWriteAccess: true, network: .none),
        requires: [],
        adapters: [.openai, .claude, .mcp, .cli, .gui]
    )
    let url = directory.appending(path: "epub.notes.popup-normalize.json")
    try JSONEncoder().encode(manifest).write(to: url)

    let catalog = try CapabilityManifestCatalog.load(from: directory)

    #expect(catalog.manifest(for: manifest.id) == manifest)
    #expect(catalog.manifest(forLegacySkillSlug: "epub-popup-footnote-converter") == manifest)
}
