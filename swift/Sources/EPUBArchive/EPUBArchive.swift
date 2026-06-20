import EPUBContracts
import ZIPFoundation

public enum ArchivePathError: Error, Equatable, Sendable {
    case empty
    case absolute
    case traversal
    case invalidSeparator
}

public struct ArchivePath: Hashable, Codable, Sendable {
    public let value: String

    public init(_ value: String) throws {
        guard !value.isEmpty else {
            throw ArchivePathError.empty
        }
        guard !value.hasPrefix("/") else {
            throw ArchivePathError.absolute
        }
        guard !value.contains("\\") && !value.contains("\0") else {
            throw ArchivePathError.invalidSeparator
        }
        let components = value.split(separator: "/", omittingEmptySubsequences: false)
        guard !components.contains("..") && !components.contains(".") else {
            throw ArchivePathError.traversal
        }
        self.value = value
    }
}
