import Foundation

public enum InspectionStatus: String, CaseIterable, Codable, Sendable {
    case pass
    case warn
    case fail
}

public enum FindingSeverity: String, CaseIterable, Codable, Sendable {
    case info
    case warn
    case error
    case fatal
}

public struct Finding: Hashable, Codable, Sendable {
    public let code: String
    public let severity: FindingSeverity
    public let message: String
    public let logicalPath: String?
    public let params: [String: String]?

    public init(
        code: String,
        severity: FindingSeverity,
        message: String,
        logicalPath: String? = nil,
        params: [String: String]? = nil
    ) {
        self.code = code
        self.severity = severity
        self.message = message
        self.logicalPath = logicalPath
        self.params = params
    }
}

public struct InspectionReport: Hashable, Codable, Sendable {
    public let schemaVersion: String
    public let artifact: ArtifactReference
    public let findings: [Finding]
    public let status: InspectionStatus
    public let facts: [String: String]?

    public init(
        schemaVersion: String,
        artifact: ArtifactReference,
        findings: [Finding],
        status: InspectionStatus,
        facts: [String: String]? = nil
    ) {
        self.schemaVersion = schemaVersion
        self.artifact = artifact
        self.findings = findings
        self.status = status
        self.facts = facts
    }
}

public enum ExecutionStepKind: String, CaseIterable, Codable, Sendable {
    case inspect
    case transform
    case validate
    case review
}

public struct ExecutionPlan: Hashable, Codable, Sendable {
    public struct Step: Hashable, Codable, Sendable {
        public let id: String
        public let capability: CapabilityID
        public let kind: ExecutionStepKind
        public let dependsOn: [String]
        public let requiresApproval: Bool

        public init(
            id: String,
            capability: CapabilityID,
            kind: ExecutionStepKind,
            dependsOn: [String],
            requiresApproval: Bool
        ) {
            self.id = id
            self.capability = capability
            self.kind = kind
            self.dependsOn = dependsOn
            self.requiresApproval = requiresApproval
        }
    }

    public let schemaVersion: String
    public let artifact: ArtifactReference
    public let steps: [Step]
    public let blockers: [String]

    public init(schemaVersion: String, artifact: ArtifactReference, blockers: [String], steps: [Step]) {
        self.schemaVersion = schemaVersion
        self.artifact = artifact
        self.blockers = blockers
        self.steps = steps
    }
}

public enum RunStatus: String, CaseIterable, Codable, Sendable {
    case complete
    case failed
    case cancelled
    case approvalRequired = "approval-required"
}

public enum RunEventStatus: String, CaseIterable, Codable, Sendable {
    case started
    case completed
    case failed
    case skipped
}

public struct RunEvent: Hashable, Codable, Sendable {
    public let step: String
    public let status: RunEventStatus
    public let message: String?

    public init(step: String, status: RunEventStatus, message: String? = nil) {
        self.step = step
        self.status = status
        self.message = message
    }
}

public struct RunReport: Hashable, Codable, Sendable {
    public let schemaVersion: String
    public let status: RunStatus
    public let input: ArtifactReference
    public let output: ArtifactReference?
    public let events: [RunEvent]

    public init(
        schemaVersion: String,
        status: RunStatus,
        input: ArtifactReference,
        output: ArtifactReference? = nil,
        events: [RunEvent]
    ) {
        self.schemaVersion = schemaVersion
        self.status = status
        self.input = input
        self.output = output
        self.events = events
    }
}
