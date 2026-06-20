import Foundation
import EPUBContracts

/// Paths created for a single run. Inputs and committed outputs remain separate
/// so a plugin cannot accidentally overwrite its source artifact.
public struct Workspace: Hashable, Codable, Sendable {
    public let root: URL
    public let before: URL?
    public let staging: URL?
    public let reports: URL?

    public init(root: URL, before: URL? = nil, staging: URL? = nil, reports: URL? = nil) {
        self.root = root
        self.before = before
        self.staging = staging
        self.reports = reports
    }
}

public struct SkillContext: Hashable, Codable, Sendable {
    public let workspace: Workspace

    public init(workspace: Workspace) {
        self.workspace = workspace
    }
}

public struct HarnessContext: Hashable, Codable, Sendable {
    public let workspace: Workspace

    public init(workspace: Workspace) {
        self.workspace = workspace
    }
}

/// A single capability implementation. Its identity is always the versioned
/// manifest, not a Markdown file, Python path, prompt, or GUI feature name.
public protocol SkillPlugin: Sendable {
    var manifest: CapabilityManifest { get }

    func inspect(_ artifact: ArtifactReference, in context: SkillContext) async throws -> InspectionReport
    func execute(_ plan: ExecutionPlan, in context: SkillContext) async throws -> RunReport
}

/// A multi-capability orchestrator. It supplies neutral reports and plans that
/// can be projected by agent, CLI, and GUI adapters.
public protocol HarnessPlugin: Sendable {
    var manifest: CapabilityManifest { get }

    func inspect(_ artifact: ArtifactReference, in context: HarnessContext) async throws -> InspectionReport
    func plan(from report: InspectionReport, in context: HarnessContext) async throws -> ExecutionPlan
    func run(_ plan: ExecutionPlan, in context: HarnessContext) async throws -> RunReport
}
