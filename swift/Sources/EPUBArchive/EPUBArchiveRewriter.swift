import Foundation
import ZIPFoundation

public enum EPUBArchiveRewriterError: Error, Equatable, Sendable {
    case destinationAlreadyExists
    case missingMimetype
    case replacementPathMissing(String)
    case additionPathAlreadyExists(String)
}

/// Repackages an EPUB into a new archive while preserving every unreplaced
/// resource byte-for-byte. The writer normalizes ZIP entry order so `mimetype`
/// is first and stored, as required by EPUB packaging conventions.
public enum EPUBArchiveRewriter {
    public static func rewrite(
        source: URL,
        to destination: URL,
        replacements: [ArchivePath: Data],
        additions: [ArchivePath: Data] = [:]
    ) throws {
        guard !FileManager.default.fileExists(atPath: destination.path) else {
            throw EPUBArchiveRewriterError.destinationAlreadyExists
        }
        let reader = try EPUBArchiveReader(url: source)
        let sourcePaths = reader.entryPaths()
        guard let mimetypePath = sourcePaths.first(where: { $0.value == "mimetype" }) else {
            throw EPUBArchiveRewriterError.missingMimetype
        }
        let sourcePathSet = Set(sourcePaths)
        for path in replacements.keys where !sourcePathSet.contains(path) {
            throw EPUBArchiveRewriterError.replacementPathMissing(path.value)
        }
        for path in additions.keys where sourcePathSet.contains(path) || replacements[path] != nil {
            throw EPUBArchiveRewriterError.additionPathAlreadyExists(path.value)
        }

        try FileManager.default.createDirectory(at: destination.deletingLastPathComponent(), withIntermediateDirectories: true)
        var completed = false
        defer {
            if !completed {
                try? FileManager.default.removeItem(at: destination)
            }
        }
        let archive = try Archive(url: destination, accessMode: .create)
        let orderedPaths = [mimetypePath] + sourcePaths.filter { $0 != mimetypePath }
        for path in orderedPaths {
            let data: Data
            if let replacement = replacements[path] {
                data = replacement
            } else {
                data = try reader.data(for: path)
            }
            try archive.addEntry(
                with: path.value,
                type: .file,
                uncompressedSize: Int64(data.count),
                compressionMethod: path == mimetypePath ? .none : .deflate,
                provider: { position, size in
                    let offset = Int(position)
                    return data.subdata(in: offset..<(offset + size))
                }
            )
        }
        for path in additions.keys.sorted(by: { $0.value < $1.value }) {
            let data = additions[path]!
            try archive.addEntry(
                with: path.value,
                type: .file,
                uncompressedSize: Int64(data.count),
                compressionMethod: .deflate,
                provider: { position, size in
                    let offset = Int(position)
                    return data.subdata(in: offset..<(offset + size))
                }
            )
        }
        completed = true
    }
}
