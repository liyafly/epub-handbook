import Foundation

public struct CapabilityID: Hashable, Codable, Sendable, RawRepresentable, Comparable {
    public let rawValue: String

    public init(rawValue: String) {
        self.rawValue = rawValue
    }

    public static func < (lhs: CapabilityID, rhs: CapabilityID) -> Bool {
        lhs.rawValue < rhs.rawValue
    }
}

public enum CapabilityKind: String, CaseIterable, Codable, Sendable {
    case detector
    case planner
    case transformer
    case validator
}

public enum RedLine: String, CaseIterable, Codable, Sendable {
    case text
    case metadata
    case spine
    case anchors
    case cover
    case drm
}

public enum NetworkPermission: String, CaseIterable, Codable, Sendable {
    case none
    case readonly
    case full
}

public struct CapabilityPermissions: Hashable, Codable, Sendable {
    public let requiresWriteAccess: Bool
    public let network: NetworkPermission

    public init(requiresWriteAccess: Bool, network: NetworkPermission) {
        self.requiresWriteAccess = requiresWriteAccess
        self.network = network
    }
}

public enum AdapterKind: String, CaseIterable, Codable, Sendable {
    case openai
    case claude
    case mcp
    case cli
    case gui
}

public struct CapabilityManifest: Hashable, Codable, Sendable {
    public let schemaVersion: String
    public let id: CapabilityID
    public let version: String
    public let kind: CapabilityKind
    public let legacySkillSlugs: [String]
    public let inputSchema: String
    public let outputSchema: String
    public let redLines: [RedLine]
    public let permissions: CapabilityPermissions
    public let requires: [CapabilityID]
    public let adapters: [AdapterKind]

    public init(
        schemaVersion: String,
        id: CapabilityID,
        version: String,
        kind: CapabilityKind,
        legacySkillSlugs: [String],
        inputSchema: String,
        outputSchema: String,
        redLines: [RedLine],
        permissions: CapabilityPermissions,
        requires: [CapabilityID],
        adapters: [AdapterKind]
    ) {
        self.schemaVersion = schemaVersion
        self.id = id
        self.version = version
        self.kind = kind
        self.legacySkillSlugs = legacySkillSlugs
        self.inputSchema = inputSchema
        self.outputSchema = outputSchema
        self.redLines = redLines
        self.permissions = permissions
        self.requires = requires
        self.adapters = adapters
    }
}
