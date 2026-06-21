import Foundation
import EPUBArchive
import EPUBPackage

public enum StylesheetInventoryError: Error, Equatable, Sendable {
    case cssIsNotUTF8(String)
    case xhtmlIsNotUTF8(String)
    case invalidXHTML(String)
}

public struct StylesheetSource: Hashable, Sendable {
    public let path: ArchivePath
    public let css: String

    public init(path: ArchivePath, css: String) {
        self.path = path
        self.css = css
    }
}

public struct StylesheetReference: Hashable, Sendable {
    public let xhtmlPath: ArchivePath
    public let stylesheetPath: ArchivePath

    public init(xhtmlPath: ArchivePath, stylesheetPath: ArchivePath) {
        self.xhtmlPath = xhtmlPath
        self.stylesheetPath = stylesheetPath
    }
}

/// Read-only graph used for cleanup planning. It intentionally represents CSS
/// only when both the OPF manifest and the archive resource agree.
public struct StylesheetInventory: Sendable {
    public let opfPath: ArchivePath
    public let stylesheets: [StylesheetSource]
    public let xhtmlPaths: [ArchivePath]
    public let references: [StylesheetReference]
    public let warnings: [String]

    public init(
        opfPath: ArchivePath,
        stylesheets: [StylesheetSource],
        xhtmlPaths: [ArchivePath],
        references: [StylesheetReference],
        warnings: [String] = []
    ) {
        self.opfPath = opfPath
        self.stylesheets = stylesheets.sorted { $0.path.value < $1.path.value }
        self.xhtmlPaths = xhtmlPaths.sorted { $0.value < $1.value }
        self.references = references
        self.warnings = warnings
    }
}

public enum StylesheetInventoryReader {
    public static func analyze(epub: URL) throws -> StylesheetInventory {
        let archive = try EPUBArchiveReader(url: epub)
        let package = try EPUBPackageReader.read(from: epub)
        let archivePaths = Set(archive.entryPaths())
        var warnings: [String] = []
        var stylesheets: [StylesheetSource] = []
        var manifestStylesheetPaths = Set<ArchivePath>()

        for item in package.package.manifest where item.mediaType.lowercased() == "text/css" {
            let path = try EPUBStylesheetPath.resolve(href: item.href, relativeTo: package.opfPath)
            guard archivePaths.contains(path) else {
                warnings.append("CSS manifest item does not resolve: \(path.value)")
                continue
            }
            guard let css = String(data: try archive.data(for: path), encoding: .utf8) else {
                throw StylesheetInventoryError.cssIsNotUTF8(path.value)
            }
            stylesheets.append(.init(path: path, css: css))
            manifestStylesheetPaths.insert(path)
        }

        var xhtmlPaths: [ArchivePath] = []
        var references: [StylesheetReference] = []
        for item in package.package.manifest where item.mediaType.lowercased() == "application/xhtml+xml" {
            let xhtmlPath = try EPUBStylesheetPath.resolve(href: item.href, relativeTo: package.opfPath)
            guard archivePaths.contains(xhtmlPath) else {
                warnings.append("XHTML manifest item does not resolve: \(xhtmlPath.value)")
                continue
            }
            guard let xhtml = String(data: try archive.data(for: xhtmlPath), encoding: .utf8) else {
                throw StylesheetInventoryError.xhtmlIsNotUTF8(xhtmlPath.value)
            }
            xhtmlPaths.append(xhtmlPath)
            let hrefs = try XHTMLStylesheetLinkParser.parse(xhtml, path: xhtmlPath)
            for href in hrefs {
                let path = try EPUBStylesheetPath.resolve(href: href, relativeTo: xhtmlPath)
                guard manifestStylesheetPaths.contains(path) else {
                    warnings.append("XHTML stylesheet link does not resolve to manifest CSS: \(xhtmlPath.value) -> \(href)")
                    continue
                }
                references.append(.init(xhtmlPath: xhtmlPath, stylesheetPath: path))
            }
        }
        return .init(
            opfPath: package.opfPath,
            stylesheets: stylesheets,
            xhtmlPaths: xhtmlPaths,
            references: references,
            warnings: warnings
        )
    }
}

public enum EPUBStylesheetPath {
    public static func resolve(href: String, relativeTo basePath: ArchivePath) throws -> ArchivePath {
        let pathComponent = href.split(whereSeparator: { $0 == "?" || $0 == "#" }).first.map(String.init) ?? ""
        guard !pathComponent.isEmpty,
              !pathComponent.contains("://"),
              !pathComponent.hasPrefix("//")
        else {
            throw ArchivePathError.empty
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

    public static func relativeHref(from source: ArchivePath, to target: ArchivePath) throws -> String {
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
        guard !components.isEmpty else { throw ArchivePathError.empty }
        return components.joined(separator: "/")
    }
}

private enum XHTMLStylesheetLinkParser {
    static func parse(_ xhtml: String, path: ArchivePath) throws -> [String] {
        let delegate = LinkDelegate()
        let parser = XMLParser(data: Data(xhtml.utf8))
        parser.delegate = delegate
        parser.shouldProcessNamespaces = false
        parser.shouldResolveExternalEntities = false
        guard parser.parse() else {
            throw StylesheetInventoryError.invalidXHTML(path.value)
        }
        return delegate.hrefs
    }
}

private final class LinkDelegate: NSObject, XMLParserDelegate {
    private(set) var hrefs: [String] = []

    func parser(
        _ parser: XMLParser,
        didStartElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?,
        attributes attributeDict: [String: String] = [:]
    ) {
        guard elementName.lowercased() == "link",
              let href = attributeDict["href"],
              href.split(whereSeparator: { $0 == "?" || $0 == "#" }).first?.lowercased().hasSuffix(".css") == true
        else {
            return
        }
        hrefs.append(href)
    }
}
