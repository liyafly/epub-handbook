import Foundation
import EPUBContracts

public enum CapabilityManifestCatalogError: Error, Equatable, Sendable {
    case duplicateCapability(CapabilityID)
    case duplicateLegacySkillSlug(String)
}

/// The generic-plugin catalog. It gives capabilities a stable identity without
/// requiring callers to inspect an agent-specific `SKILL.md` directory.
public struct CapabilityManifestCatalog: Sendable {
    private let manifestsByID: [CapabilityID: CapabilityManifest]
    private let manifestsByLegacySkillSlug: [String: CapabilityManifest]

    public init(manifests: [CapabilityManifest]) throws {
        var byID: [CapabilityID: CapabilityManifest] = [:]
        var bySlug: [String: CapabilityManifest] = [:]

        for manifest in manifests {
            guard byID[manifest.id] == nil else {
                throw CapabilityManifestCatalogError.duplicateCapability(manifest.id)
            }
            byID[manifest.id] = manifest

            for slug in manifest.legacySkillSlugs {
                guard bySlug[slug] == nil else {
                    throw CapabilityManifestCatalogError.duplicateLegacySkillSlug(slug)
                }
                bySlug[slug] = manifest
            }
        }

        manifestsByID = byID
        manifestsByLegacySkillSlug = bySlug
    }

    public static func load(from directory: URL) throws -> CapabilityManifestCatalog {
        let urls = try FileManager.default.contentsOfDirectory(
            at: directory,
            includingPropertiesForKeys: nil,
            options: [.skipsHiddenFiles]
        )
        let decoder = JSONDecoder()
        let manifests = try urls
            .filter { $0.pathExtension == "json" }
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
            .map { try decoder.decode(CapabilityManifest.self, from: Data(contentsOf: $0)) }
        return try CapabilityManifestCatalog(manifests: manifests)
    }

    public func manifest(for capability: CapabilityID) -> CapabilityManifest? {
        manifestsByID[capability]
    }

    public func manifest(forLegacySkillSlug slug: String) -> CapabilityManifest? {
        manifestsByLegacySkillSlug[slug]
    }

    public var manifests: [CapabilityManifest] {
        manifestsByID.values.sorted { $0.id < $1.id }
    }
}
