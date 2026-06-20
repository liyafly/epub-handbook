import Foundation
import EPUBArchive
import EPUBPackage
import SwiftSoup

public struct PopupFootnoteValidationIssue: Hashable, Codable, Sendable {
    public let path: String
    public let message: String

    public init(path: String, message: String) {
        self.path = path
        self.message = message
    }
}

public struct PopupFootnoteValidationReport: Hashable, Codable, Sendable {
    public let issues: [PopupFootnoteValidationIssue]

    public init(issues: [PopupFootnoteValidationIssue]) {
        self.issues = issues
    }

    public var isValid: Bool { issues.isEmpty }
}

/// Native structural and resource validation for the popup-note contract.
/// It deliberately performs no repair and has no dependency on Python skills
/// or harnesses, making it suitable for the Apple-native execution path.
public enum PopupFootnoteValidator {
    public static func validate(epub url: URL) throws -> PopupFootnoteValidationReport {
        let archive = try EPUBArchiveReader(url: url)
        let package = try EPUBPackageReader.read(from: url)
        let imageManifestPaths = try imagePaths(in: package)
        let archivePaths = Set(archive.entryPaths())
        var issues: [PopupFootnoteValidationIssue] = []

        for xhtmlPath in archive.entryPaths().filter(isXHTMLPath).sorted(by: { $0.value < $1.value }) {
            let data = try archive.data(for: xhtmlPath)
            guard let xhtml = String(data: data, encoding: .utf8) else {
                issues.append(.init(path: xhtmlPath.value, message: "XHTML is not UTF-8."))
                continue
            }
            do {
                let document = try SwiftSoup.parseXML(xhtml)
                let iconSources = try validateDocument(document, path: xhtmlPath.value, issues: &issues)
                for source in iconSources {
                    guard let iconPath = resolveRelativePath(source, from: xhtmlPath),
                          archivePaths.contains(iconPath),
                          imageManifestPaths.contains(iconPath)
                    else {
                        issues.append(.init(path: xhtmlPath.value, message: "Noteref image is not a local manifest-backed EPUB image: \(source)"))
                        continue
                    }
                }
            } catch {
                issues.append(.init(path: xhtmlPath.value, message: "XHTML could not be parsed for popup-note validation."))
            }
        }
        return .init(issues: issues)
    }

    private static func validateDocument(
        _ document: Document,
        path: String,
        issues: inout [PopupFootnoteValidationIssue]
    ) throws -> [String] {
        let anchors = try document.getElementsByTag("a").array()
        let references = try anchors.filter(isNoteref)
        let asides = try document.getElementsByTag("aside").array().filter(isFootnoteAside)
        guard !references.isEmpty || !asides.isEmpty else { return [] }

        guard let html = try document.getElementsByTag("html").first(), html.hasAttr("xmlns:epub") else {
            issues.append(.init(path: path, message: "Files with popup notes must declare xmlns:epub."))
            return []
        }
        guard asides.count == 1 else {
            issues.append(.init(path: path, message: "Files with notes must contain exactly one grouped footnote aside."))
            return []
        }
        let aside = asides[0]
        let asideType = try hasType(aside, "footnote")
        let asideRole = try aside.attr("role")
        if !asideType || asideRole != "doc-footnote" {
            issues.append(.init(path: path, message: "The grouped aside must use epub:type=footnote and role=doc-footnote."))
        }
        let lists = try aside.getElementsByTag("ol").array().filter { $0.hasClass("footnote-list") }
        guard lists.count == 1 else {
            issues.append(.init(path: path, message: "The grouped aside must contain exactly one ol.footnote-list."))
            return []
        }
        let list = lists[0]
        let items = try list.getElementsByTag("li").array().filter { $0.hasClass("footnote-item") }
        let itemIDs = Set(try items.compactMap { item -> String? in
            let identifier = try item.attr("id")
            return identifier.isEmpty ? nil : identifier
        })
        var referenceIDs = Set<String>()
        var targetIDs = Set<String>()
        var imageSources: [String] = []

        for reference in references {
            let identifier = try reference.attr("id")
            if identifier.isEmpty {
                issues.append(.init(path: path, message: "Noteref is missing an id."))
            } else {
                referenceIDs.insert(identifier)
            }
            let referenceType = try hasType(reference, "noteref")
            let referenceRole = try reference.attr("role")
            if !referenceType || referenceRole != "doc-noteref" || !reference.hasClass("noteref-icon") {
                issues.append(.init(path: path, message: "Noteref must use canonical type, role, and class."))
            }
            let images = try reference.getElementsByTag("img").array()
            if images.count != 1 {
                issues.append(.init(path: path, message: "Noteref must contain exactly one image icon."))
            } else {
                let source = try images[0].attr("src")
                let alt = try images[0].attr("alt")
                if source.isEmpty || alt.isEmpty {
                    issues.append(.init(path: path, message: "Noteref image must have src and alt."))
                } else {
                    imageSources.append(source)
                }
            }
            let href = try reference.attr("href")
            guard href.hasPrefix("#"), href.count > 1 else {
                issues.append(.init(path: path, message: "Noteref href must be a same-file fragment."))
                continue
            }
            let targetID = String(href.dropFirst())
            targetIDs.insert(targetID)
            guard let target = try document.getElementById(targetID) else {
                issues.append(.init(path: path, message: "Noteref target is missing: #\(targetID)."))
                continue
            }
            if target.tagName().lowercased() != "li" || !target.hasClass("footnote-item") {
                issues.append(.init(path: path, message: "Noteref target must be li.footnote-item: #\(targetID)."))
            }
        }

        if !targetIDs.isSubset(of: itemIDs) {
            issues.append(.init(path: path, message: "Every noteref target must be in ol.footnote-list."))
        }
        var backlinkCount = 0
        for item in items {
            let links = try item.getElementsByTag("a").array()
            for link in links where try isBacklink(link) {
                backlinkCount += 1
                let validType = try hasType(link, "backlink")
                let role = try link.attr("role")
                let href = try link.attr("href")
                let targetID = href.hasPrefix("#") ? String(href.dropFirst()) : ""
                if !validType || role != "doc-backlink" || !referenceIDs.contains(targetID) {
                    issues.append(.init(path: path, message: "Backlink must use canonical type, role, and a local noteref target."))
                }
            }
        }
        if backlinkCount < targetIDs.count {
            issues.append(.init(path: path, message: "Each footnote target must contain a backlink."))
        }
        return imageSources
    }

    private static func imagePaths(in package: EPUBPackageSnapshot) throws -> Set<ArchivePath> {
        var paths = Set<ArchivePath>()
        for item in package.package.manifest where item.mediaType.lowercased().hasPrefix("image/") {
            guard let path = resolveRelativePath(item.href, from: package.opfPath) else {
                continue
            }
            paths.insert(path)
        }
        return paths
    }
}

private func isXHTMLPath(_ path: ArchivePath) -> Bool {
    path.value.lowercased().hasSuffix(".xhtml") || path.value.lowercased().hasSuffix(".html")
}

private func isNoteref(_ element: Element) throws -> Bool {
    try hasType(element, "noteref") || (try element.attr("role")) == "doc-noteref"
}

private func isFootnoteAside(_ element: Element) throws -> Bool {
    try hasType(element, "footnote") || (try element.attr("role")) == "doc-footnote"
}

private func isBacklink(_ element: Element) throws -> Bool {
    try hasType(element, "backlink") || (try element.attr("role")) == "doc-backlink"
}

private func hasType(_ element: Element, _ token: String) throws -> Bool {
    try element.attr("epub:type").split(whereSeparator: \.isWhitespace).map(String.init).contains(token)
}

private func resolveRelativePath(_ href: String, from basePath: ArchivePath) -> ArchivePath? {
    let component = href.split(whereSeparator: { $0 == "?" || $0 == "#" }).first.map(String.init) ?? ""
    guard !component.isEmpty, !component.contains("://"), !component.hasPrefix("//") else { return nil }
    let decoded = component.removingPercentEncoding ?? component
    var segments = basePath.value.split(separator: "/").dropLast().map(String.init)
    for segment in decoded.split(separator: "/", omittingEmptySubsequences: false) {
        switch segment {
        case "", ".":
            continue
        case "..":
            guard !segments.isEmpty else { return nil }
            segments.removeLast()
        default:
            segments.append(String(segment))
        }
    }
    return try? ArchivePath(segments.joined(separator: "/"))
}
