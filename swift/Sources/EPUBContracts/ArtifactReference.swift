import Foundation

public struct ArtifactReference: Hashable, Codable, Sendable {
    public enum Kind: String, CaseIterable, Codable, Sendable {
        case epub
        case sourceDirectory = "source-directory"
        case markdown
        case html
        case pdf
        case imageSet = "image-set"
        case unknown
    }

    public let uri: URL
    public let kind: Kind
    public let contentDigest: String?
    public let logicalPath: String?

    public init(
        uri: URL,
        kind: Kind,
        contentDigest: String? = nil,
        logicalPath: String? = nil
    ) {
        self.uri = uri
        self.kind = kind
        self.contentDigest = contentDigest
        self.logicalPath = logicalPath
    }
}
