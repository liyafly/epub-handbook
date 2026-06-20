import Foundation
import EPUBContracts
import EPUBRuntime
import Testing

private struct NoOpSkill: SkillPlugin {
    let manifest: CapabilityManifest

    func inspect(_ artifact: ArtifactReference, in context: SkillContext) async throws -> InspectionReport {
        InspectionReport(schemaVersion: "1", artifact: artifact, findings: [], status: .pass)
    }

    func execute(_ plan: ExecutionPlan, in context: SkillContext) async throws -> RunReport {
        RunReport(schemaVersion: "1", status: .complete, input: plan.artifact, events: [])
    }
}

@Test("generic SkillPlugin identity comes from the neutral manifest")
func genericSkillPluginUsesManifestIdentity() async throws {
    let skill = NoOpSkill(
        manifest: CapabilityManifest(
            schemaVersion: "1",
            id: .init(rawValue: "epub.package.nav.audit"),
            version: "1.0.0",
            kind: .detector,
            legacySkillSlugs: ["epub-package-nav-auditor"],
            inputSchema: "contracts/schemas/v1/artifact-reference.schema.json",
            outputSchema: "contracts/schemas/v1/inspection-report.schema.json",
            redLines: [.metadata, .spine],
            permissions: .init(requiresWriteAccess: false, network: .none),
            requires: [],
            adapters: [.openai, .cli, .gui]
        )
    )
    let artifact = ArtifactReference(uri: URL(filePath: "/tmp/book.epub"), kind: .epub)
    let context = SkillContext(workspace: .init(root: URL(filePath: "/tmp")))

    #expect(skill.manifest.id.rawValue == "epub.package.nav.audit")
    #expect(try await skill.inspect(artifact, in: context).status == .pass)
}
