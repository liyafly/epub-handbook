import Foundation
import EPUBArchive
import EPUBPackage

public enum PopupFootnoteArchiveNormalizeError: Error, Equatable, Sendable {
    case protectedPackage
    case xhtmlIsNotUTF8(String)
    case imageResourceUnavailable(xhtml: String, source: String)
}

public struct PopupFootnoteArchiveNormalizeResult: Hashable, Codable, Sendable {
    public let normalizedXHTMLPaths: [ArchivePath]
    public let normalizedReferenceCount: Int

    public init(normalizedXHTMLPaths: [ArchivePath], normalizedReferenceCount: Int) {
        self.normalizedXHTMLPaths = normalizedXHTMLPaths
        self.normalizedReferenceCount = normalizedReferenceCount
    }
}

/// Applies the native popup-note normalizer to selected XHTML resources and
/// writes a new EPUB. It never creates an icon, modifies OPF, or rewrites an
/// encrypted package: those are separate capabilities with separate gates.
public enum PopupFootnoteArchiveNormalizer {
    public static func normalize(
        source: URL,
        to destination: URL,
        xhtmlPaths: [ArchivePath]? = nil
    ) throws -> PopupFootnoteArchiveNormalizeResult {
        let archive = try EPUBArchiveReader(url: source)
        guard !archive.entryPaths().contains(where: { $0.value.lowercased() == "meta-inf/encryption.xml" }) else {
            throw PopupFootnoteArchiveNormalizeError.protectedPackage
        }
        let package = try EPUBPackageReader.read(from: source)
        let selectedPaths = try selectedXHTMLPaths(xhtmlPaths, in: archive)
        let manifestImages = try imageManifestPaths(package)
        var replacements: [ArchivePath: Data] = [:]
        var normalizedPaths: [ArchivePath] = []
        var normalizedReferenceCount = 0

        for path in selectedPaths {
            let data = try archive.data(for: path)
            guard let xhtml = String(data: data, encoding: .utf8) else {
                throw PopupFootnoteArchiveNormalizeError.xhtmlIsNotUTF8(path.value)
            }
            let result = try PopupFootnoteNormalizer.normalize(in: xhtml)
            guard result.normalizedReferenceCount > 0 else {
                continue
            }
            for imageSource in result.imageSources {
                let imagePath = try localResourcePath(imageSource, relativeTo: path)
                guard archive.entryPaths().contains(imagePath), manifestImages.contains(imagePath) else {
                    throw PopupFootnoteArchiveNormalizeError.imageResourceUnavailable(xhtml: path.value, source: imageSource)
                }
            }
            replacements[path] = Data(result.xhtml.utf8)
            normalizedPaths.append(path)
            normalizedReferenceCount += result.normalizedReferenceCount
        }

        try EPUBArchiveRewriter.rewrite(source: source, to: destination, replacements: replacements)
        return .init(normalizedXHTMLPaths: normalizedPaths, normalizedReferenceCount: normalizedReferenceCount)
    }

    private static func selectedXHTMLPaths(_ explicit: [ArchivePath]?, in archive: EPUBArchiveReader) throws -> [ArchivePath] {
        let allPaths = Set(archive.entryPaths())
        if let explicit {
            for path in explicit where !allPaths.contains(path) {
                throw EPUBArchiveRewriterError.replacementPathMissing(path.value)
            }
            return explicit.sorted { $0.value < $1.value }
        }
        return archive.entryPaths()
            .filter { $0.value.lowercased().hasSuffix(".xhtml") || $0.value.lowercased().hasSuffix(".html") }
            .sorted { $0.value < $1.value }
    }

    private static func imageManifestPaths(_ package: EPUBPackageSnapshot) throws -> Set<ArchivePath> {
        var paths = Set<ArchivePath>()
        for item in package.package.manifest where item.mediaType.lowercased().hasPrefix("image/") {
            paths.insert(try localResourcePath(item.href, relativeTo: package.opfPath))
        }
        return paths
    }
}

private func localResourcePath(_ href: String, relativeTo basePath: ArchivePath) throws -> ArchivePath {
    let pathComponent = href.split(whereSeparator: { $0 == "?" || $0 == "#" }).first.map(String.init) ?? ""
    guard !pathComponent.isEmpty,
          !pathComponent.contains("://"),
          !pathComponent.hasPrefix("//")
    else {
        throw PopupFootnoteArchiveNormalizeError.imageResourceUnavailable(xhtml: basePath.value, source: href)
    }
    let decoded = pathComponent.removingPercentEncoding ?? pathComponent
    var components = basePath.value.split(separator: "/").dropLast().map(String.init)
    for component in decoded.split(separator: "/", omittingEmptySubsequences: false) {
        switch component {
        case "", ".":
            continue
        case "..":
            guard !components.isEmpty else { throw ArchivePathError.traversal }
            components.removeLast()
        default:
            components.append(String(component))
        }
    }
    return try ArchivePath(components.joined(separator: "/"))
}
