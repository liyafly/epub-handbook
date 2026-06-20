import Foundation
import ZIPFoundation

public enum EPUBArchiveReaderError: Error, Equatable, Sendable {
    case missingEntry(String)
    case invalidArchivePath(String)
    case unreadableEntry(String)
}

/// A read-only ZIPFoundation adapter that rejects archive path traversal before
/// exposing any EPUB resource to higher layers.
public struct EPUBArchiveReader {
    private let archive: Archive
    private let paths: [ArchivePath]

    public init(url: URL) throws {
        archive = try Archive(url: url, accessMode: .read)
        do {
            paths = try archive.map { entry in
                try ArchivePath(entry.path)
            }
        } catch {
            throw EPUBArchiveReaderError.invalidArchivePath("archive entry")
        }
    }

    public func entryPaths() -> [ArchivePath] {
        paths
    }

    public func data(for path: ArchivePath) throws -> Data {
        guard let entry = archive[path.value] else {
            throw EPUBArchiveReaderError.missingEntry(path.value)
        }
        var result = Data()
        do {
            _ = try archive.extract(entry) { chunk in
                result.append(chunk)
            }
            return result
        } catch {
            throw EPUBArchiveReaderError.unreadableEntry(path.value)
        }
    }
}
