import Foundation
import EPUBArchive
import EPUBPackage
import SwiftSoup

public enum PopupFootnoteArchiveNormalizeError: Error, Equatable, Sendable {
    case protectedPackage
    case xhtmlIsNotUTF8(String)
    case opfIsNotUTF8
    case packageManifestMissing
    case defaultIconPathConflicts(String)
    case imageResourceUnavailable(xhtml: String, source: String)
}

public struct PopupFootnoteArchiveNormalizeResult: Hashable, Codable, Sendable {
    public let normalizedXHTMLPaths: [ArchivePath]
    public let normalizedReferenceCount: Int
    public let defaultIconAdded: Bool

    public init(
        normalizedXHTMLPaths: [ArchivePath],
        normalizedReferenceCount: Int,
        defaultIconAdded: Bool = false
    ) {
        self.normalizedXHTMLPaths = normalizedXHTMLPaths
        self.normalizedReferenceCount = normalizedReferenceCount
        self.defaultIconAdded = defaultIconAdded
    }
}

/// Applies the native popup-note normalizer to selected XHTML resources and
/// writes a new EPUB. Existing note icons remain untouched. Text markers use a
/// package-local fallback only when the EPUB needs one, with the icon and its
/// OPF manifest item created in the same output transaction.
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
        let archivePaths = Set(archive.entryPaths())
        let manifestImages = try imageManifestPaths(package)
        let manifestPaths = try manifestPaths(package)
        let defaultIconPath = try defaultIconPath(relativeTo: package.opfPath)
        let defaultLanguage = package.package.coreMetadata["language"]?.first
        var replacements: [ArchivePath: Data] = [:]
        var normalizedPaths: [ArchivePath] = []
        var normalizedReferenceCount = 0
        var usesDefaultIcon = false

        for path in selectedPaths {
            let data = try archive.data(for: path)
            guard let xhtml = String(data: data, encoding: .utf8) else {
                throw PopupFootnoteArchiveNormalizeError.xhtmlIsNotUTF8(path.value)
            }
            let defaultSource = try relativeHref(from: path, to: defaultIconPath)
            let result = try PopupFootnoteNormalizer.normalize(
                in: xhtml,
                defaultIconSource: defaultSource,
                defaultLanguage: defaultLanguage
            )
            guard result.didChange else {
                continue
            }
            for imageSource in result.imageSources {
                let imagePath = try localResourcePath(imageSource, relativeTo: path)
                if result.usedDefaultIcon && imagePath == defaultIconPath {
                    continue
                }
                guard archivePaths.contains(imagePath), manifestImages.contains(imagePath) else {
                    throw PopupFootnoteArchiveNormalizeError.imageResourceUnavailable(xhtml: path.value, source: imageSource)
                }
            }
            replacements[path] = Data(result.xhtml.utf8)
            normalizedPaths.append(path)
            normalizedReferenceCount += result.normalizedReferenceCount
            usesDefaultIcon = usesDefaultIcon || result.usedDefaultIcon
        }

        var additions: [ArchivePath: Data] = [:]
        if usesDefaultIcon {
            if manifestPaths.contains(defaultIconPath) && !manifestImages.contains(defaultIconPath) {
                throw PopupFootnoteArchiveNormalizeError.defaultIconPathConflicts(defaultIconPath.value)
            }
            if !archivePaths.contains(defaultIconPath) {
                additions[defaultIconPath] = defaultIconPNG
            }
            if !manifestImages.contains(defaultIconPath) {
                replacements[package.opfPath] = try addingDefaultIconManifestItem(
                    to: archive.data(for: package.opfPath),
                    package: package,
                    defaultIconPath: defaultIconPath
                )
            }
        }

        try EPUBArchiveRewriter.rewrite(
            source: source,
            to: destination,
            replacements: replacements,
            additions: additions
        )
        return .init(
            normalizedXHTMLPaths: normalizedPaths,
            normalizedReferenceCount: normalizedReferenceCount,
            defaultIconAdded: usesDefaultIcon && !archivePaths.contains(defaultIconPath)
        )
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

    private static func manifestPaths(_ package: EPUBPackageSnapshot) throws -> Set<ArchivePath> {
        try Set(package.package.manifest.map { try localResourcePath($0.href, relativeTo: package.opfPath) })
    }

    private static func defaultIconPath(relativeTo opfPath: ArchivePath) throws -> ArchivePath {
        let parent = opfPath.value.split(separator: "/").dropLast().map(String.init)
        return try ArchivePath((parent + ["Images", "note.png"]).joined(separator: "/"))
    }

    private static func relativeHref(from source: ArchivePath, to target: ArchivePath) throws -> String {
        let sourceParent = source.value.split(separator: "/").dropLast().map(String.init)
        let targetComponents = target.value.split(separator: "/").map(String.init)
        var commonCount = 0
        while commonCount < sourceParent.count,
              commonCount < targetComponents.count,
              sourceParent[commonCount] == targetComponents[commonCount] {
            commonCount += 1
        }
        let upward = Array(repeating: "..", count: sourceParent.count - commonCount)
        let downward = Array(targetComponents.dropFirst(commonCount))
        let components = upward + downward
        guard !components.isEmpty else {
            throw PopupFootnoteArchiveNormalizeError.imageResourceUnavailable(xhtml: source.value, source: target.value)
        }
        return components.joined(separator: "/")
    }

    private static func addingDefaultIconManifestItem(
        to opfData: Data,
        package: EPUBPackageSnapshot,
        defaultIconPath: ArchivePath
    ) throws -> Data {
        guard let opf = String(data: opfData, encoding: .utf8) else {
            throw PopupFootnoteArchiveNormalizeError.opfIsNotUTF8
        }
        do {
            let document = try SwiftSoup.parseXML(opf)
            guard let manifest = try document.getElementsByTag("manifest").first() else {
                throw PopupFootnoteArchiveNormalizeError.packageManifestMissing
            }
            let ids = Set(package.package.manifest.map(\.id))
            let id = uniqueManifestID(base: "note-icon", existing: ids)
            let item = try manifest.appendElement("item")
            try item.attr("id", id)
            try item.attr("href", try relativeHref(from: package.opfPath, to: defaultIconPath))
            try item.attr("media-type", "image/png")
            return Data(try document.outerHtml().utf8)
        } catch let error as PopupFootnoteArchiveNormalizeError {
            throw error
        } catch {
            throw PopupFootnoteArchiveNormalizeError.packageManifestMissing
        }
    }

    private static func uniqueManifestID(base: String, existing: Set<String>) -> String {
        guard existing.contains(base) else { return base }
        var ordinal = 2
        while existing.contains("\(base)-\(ordinal)") {
            ordinal += 1
        }
        return "\(base)-\(ordinal)"
    }
}

private let defaultIconPNG = Data(base64Encoded: "iVBORw0KGgoAAAANSUhEUgAAAAwAAAAMCAYAAABWdVznAAAAHklEQVR4nGNgGAWjYBSMglEwCkbBKBgFo2AUDAMABRwAAf1xD6YAAAAASUVORK5CYII=")!

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
