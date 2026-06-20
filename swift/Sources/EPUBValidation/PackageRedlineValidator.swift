import Foundation
import EPUBArchive
import EPUBPackage

public enum PackageRedlineIssueKind: String, Codable, Sendable {
    case metadataChanged = "metadata-changed"
    case spineChanged = "spine-changed"
    case coverChanged = "cover-changed"
    case drmDetected = "drm-detected"
}

public struct PackageRedlineIssue: Hashable, Codable, Sendable {
    public let kind: PackageRedlineIssueKind
    public let message: String

    public init(kind: PackageRedlineIssueKind, message: String) {
        self.kind = kind
        self.message = message
    }
}

public struct PackageRedlineReport: Hashable, Codable, Sendable {
    public let issues: [PackageRedlineIssue]

    public init(issues: [PackageRedlineIssue]) {
        self.issues = issues
    }

    public var isValid: Bool { issues.isEmpty }
}

/// Options that mirror the Python redline gate. A path map only permits an
/// already-approved archive rename; it never permits metadata or byte changes.
public struct PackageRedlineOptions: Hashable, Codable, Sendable {
    public let pathMap: [String: String]
    public let allowStandardFontObfuscation: Bool

    public init(
        pathMap: [String: String] = [:],
        allowStandardFontObfuscation: Bool = false
    ) {
        self.pathMap = pathMap
        self.allowStandardFontObfuscation = allowStandardFontObfuscation
    }
}

/// Native package redline checks for metadata, spine, cover resources, and
/// encryption. They intentionally use the same narrow exception model as the
/// Python redline gate: stale references are safe; standard font obfuscation
/// needs an explicit approval flag; all other encryption blocks mutation.
public enum PackageRedlineValidator {
    public static func validate(
        before beforeURL: URL,
        after afterURL: URL,
        options: PackageRedlineOptions = .init()
    ) throws -> PackageRedlineReport {
        let beforeArchive = try EPUBArchiveReader(url: beforeURL)
        let afterArchive = try EPUBArchiveReader(url: afterURL)
        let beforePackage = try EPUBPackageReader.read(from: beforeURL)
        let afterPackage = try EPUBPackageReader.read(from: afterURL)
        var issues: [PackageRedlineIssue] = []

        if beforePackage.package.coreMetadata != afterPackage.package.coreMetadata {
            issues.append(.init(kind: .metadataChanged, message: "Core OPF metadata changed."))
        }
        if beforePackage.package.spineItemIDs != afterPackage.package.spineItemIDs {
            issues.append(.init(kind: .spineChanged, message: "OPF spine itemref order changed."))
        }
        if try coverChanged(
            beforePackage: beforePackage,
            beforeArchive: beforeArchive,
            afterPackage: afterPackage,
            afterArchive: afterArchive,
            pathMap: options.pathMap
        ) {
            issues.append(.init(kind: .coverChanged, message: "Cover-image path or bytes changed."))
        }

        let beforeEncryption = try EncryptionState.inspect(archive: beforeArchive, package: beforePackage)
        let afterEncryption = try EncryptionState.inspect(archive: afterArchive, package: afterPackage)
        if !encryptionIsAllowed(
            before: beforeEncryption,
            after: afterEncryption,
            allowStandardFontObfuscation: options.allowStandardFontObfuscation
        ) {
            issues.append(.init(kind: .drmDetected, message: "DRM or unsupported encrypted resources detected; native apply is blocked."))
        }
        return PackageRedlineReport(issues: issues)
    }

    private static func coverChanged(
        beforePackage: EPUBPackageSnapshot,
        beforeArchive: EPUBArchiveReader,
        afterPackage: EPUBPackageSnapshot,
        afterArchive: EPUBArchiveReader,
        pathMap: [String: String]
    ) throws -> Bool {
        let beforePath = try coverPath(package: beforePackage)
        let afterPath = try coverPath(package: afterPackage)
        guard let beforePath else {
            return afterPath != nil
        }
        guard let afterPath, pathMap[beforePath.value] ?? beforePath.value == afterPath.value else {
            return true
        }
        return try beforeArchive.data(for: beforePath) != afterArchive.data(for: afterPath)
    }

    private static func coverPath(package: EPUBPackageSnapshot) throws -> ArchivePath? {
        guard let item = package.package.manifest.first(where: { $0.properties.contains("cover-image") }) else {
            return nil
        }
        return try resolvePackageRelativePath(item.href, opfPath: package.opfPath)
    }

    private static func encryptionIsAllowed(
        before: EncryptionState,
        after: EncryptionState,
        allowStandardFontObfuscation: Bool
    ) -> Bool {
        guard before != .none || after != .none else {
            return true
        }
        if before.isOnlyStaleReferences && after.isOnlyStaleReferences {
            return true
        }
        return allowStandardFontObfuscation
            && before.isOnlyStandardFontObfuscation
            && after.isOnlyStandardFontObfuscation
    }
}

private let standardFontObfuscationAlgorithms: Set<String> = [
    "http://www.idpf.org/2008/embedding",
    "http://ns.adobe.com/pdf/enc#RC",
]

private enum EncryptionState: Equatable {
    case none
    case staleReferences
    case standardFontObfuscation
    case protectedOrMalformed

    var isOnlyStaleReferences: Bool { self == .none || self == .staleReferences }
    var isOnlyStandardFontObfuscation: Bool { self == .none || self == .standardFontObfuscation }

    static func inspect(archive: EPUBArchiveReader, package: EPUBPackageSnapshot) throws -> EncryptionState {
        guard let encryptionPath = archive.entryPaths().first(where: { $0.value.lowercased() == "meta-inf/encryption.xml" }) else {
            return .none
        }
        let records: [EncryptionRecord]
        do {
            records = try EncryptionDocument.parse(archive.data(for: encryptionPath))
        } catch {
            return .protectedOrMalformed
        }
        guard !records.isEmpty, records.allSatisfy({ !$0.references.isEmpty }) else {
            return .protectedOrMalformed
        }

        let referencePaths = records.flatMap(\.references)
        guard !referencePaths.isEmpty else {
            return .protectedOrMalformed
        }
        let existingPaths = Set(archive.entryPaths())
        if referencePaths.allSatisfy({ !existingPaths.contains($0) }) {
            return .staleReferences
        }

        guard records.allSatisfy({ standardFontObfuscationAlgorithms.contains($0.algorithm) }) else {
            return .protectedOrMalformed
        }
        var fontPaths = Set<ArchivePath>()
        for item in package.package.manifest where isFont(item) {
            fontPaths.insert(try resolvePackageRelativePath(item.href, opfPath: package.opfPath))
        }
        guard referencePaths.allSatisfy({ fontPaths.contains($0) && existingPaths.contains($0) }) else {
            return .protectedOrMalformed
        }
        return .standardFontObfuscation
    }
}

private func isFont(_ item: OPFManifestItem) -> Bool {
    item.mediaType.lowercased().contains("font")
        || item.href.lowercased().hasSuffix(".otf")
        || item.href.lowercased().hasSuffix(".ttf")
        || item.href.lowercased().hasSuffix(".woff")
        || item.href.lowercased().hasSuffix(".woff2")
}

private struct EncryptionRecord {
    let algorithm: String
    let references: [ArchivePath]
}

private enum EncryptionDocument {
    static func parse(_ data: Data) throws -> [EncryptionRecord] {
        let delegate = EncryptionDelegate()
        let parser = XMLParser(data: data)
        parser.delegate = delegate
        parser.shouldProcessNamespaces = false
        parser.shouldResolveExternalEntities = false
        guard parser.parse(), !delegate.malformed else {
            throw EncryptionDocumentError.malformed
        }
        return delegate.records
    }
}

private enum EncryptionDocumentError: Error {
    case malformed
}

private final class EncryptionDelegate: NSObject, XMLParserDelegate {
    private(set) var records: [EncryptionRecord] = []
    private(set) var malformed = false
    private var currentAlgorithm = ""
    private var currentReferences: [ArchivePath] = []
    private var insideEncryptedData = false

    func parser(
        _ parser: XMLParser,
        didStartElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?,
        attributes attributeDict: [String: String] = [:]
    ) {
        switch localName(elementName) {
        case "EncryptedData":
            guard !insideEncryptedData else {
                malformed = true
                return
            }
            insideEncryptedData = true
            currentAlgorithm = ""
            currentReferences = []
        case "EncryptionMethod":
            guard insideEncryptedData, let algorithm = attributeDict["Algorithm"], !algorithm.isEmpty else {
                malformed = true
                return
            }
            currentAlgorithm = algorithm
        case "CipherReference":
            guard insideEncryptedData,
                  let uri = attributeDict["URI"],
                  let path = rootRelativeArchivePath(uri)
            else {
                malformed = true
                return
            }
            currentReferences.append(path)
        default:
            break
        }
    }

    func parser(
        _ parser: XMLParser,
        didEndElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?
    ) {
        guard localName(elementName) == "EncryptedData" else {
            return
        }
        guard insideEncryptedData else {
            malformed = true
            return
        }
        records.append(.init(algorithm: currentAlgorithm, references: currentReferences))
        insideEncryptedData = false
    }
}

private func localName(_ name: String) -> String {
    name.split(separator: ":").last.map(String.init) ?? name
}

private func rootRelativeArchivePath(_ uri: String) -> ArchivePath? {
    let pathComponent = uri.split(whereSeparator: { $0 == "?" || $0 == "#" }).first.map(String.init) ?? ""
    guard !pathComponent.isEmpty,
          !pathComponent.contains("://"),
          !pathComponent.hasPrefix("//"),
          !pathComponent.hasPrefix("../")
    else {
        return nil
    }
    let decoded = pathComponent.removingPercentEncoding ?? pathComponent
    let trimmed = decoded.hasPrefix("/") ? String(decoded.drop(while: { $0 == "/" })) : decoded
    return try? ArchivePath(trimmed)
}

private func resolvePackageRelativePath(_ href: String, opfPath: ArchivePath) throws -> ArchivePath {
    let pathComponent = href.split(whereSeparator: { $0 == "?" || $0 == "#" }).first.map(String.init) ?? ""
    guard !pathComponent.isEmpty,
          !pathComponent.contains("://"),
          !pathComponent.hasPrefix("//")
    else {
        throw ArchivePathError.empty
    }
    let decoded = pathComponent.removingPercentEncoding ?? pathComponent
    var components = opfPath.value.split(separator: "/").dropLast().map(String.init)
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
