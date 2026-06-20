import Foundation

/// Filesystem locations for a native Swift transaction. It contains no agent,
/// skill, harness, or Python execution information.
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
