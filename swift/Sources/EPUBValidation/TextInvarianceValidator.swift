import Foundation
#if canImport(FoundationXML)
import FoundationXML
#endif
import EPUBArchive

public enum TextInvarianceCheck: String, CaseIterable, Codable, Sendable {
    case text
    case anchors
}

public enum TextInvarianceIssueKind: String, Codable, Sendable {
    case textModified = "text-modified"
    case xhtmlDeleted = "xhtml-deleted"
    case xhtmlAdded = "xhtml-added"
    case anchorsRemoved = "anchors-removed"
}

public struct TextInvarianceIssue: Hashable, Codable, Sendable {
    public let kind: TextInvarianceIssueKind
    public let path: String
    public let message: String

    public init(kind: TextInvarianceIssueKind, path: String, message: String) {
        self.kind = kind
        self.path = path
        self.message = message
    }
}

public struct TextInvarianceReport: Hashable, Codable, Sendable {
    public let issues: [TextInvarianceIssue]

    public init(issues: [TextInvarianceIssue]) {
        self.issues = issues
    }

    public var isValid: Bool {
        issues.isEmpty
    }
}

public struct TextInvarianceOptions: Sendable {
    public let checks: Set<TextInvarianceCheck>
    public let pathMap: [String: String]

    public init(
        checks: Set<TextInvarianceCheck> = Set(TextInvarianceCheck.allCases),
        pathMap: [String: String] = [:]
    ) {
        self.checks = checks
        self.pathMap = pathMap
    }
}

public enum TextInvarianceError: Error, Equatable, Sendable {
    case invalidXHTML(String)
}

/// Native Swift text and anchor redline validation for GUI-native workflows.
/// It intentionally compares normalized leaf-block content rather than XHTML
/// serialization so approved DOM formatting changes do not create false alarms.
public enum TextInvarianceValidator {
    public static func validate(
        before beforeURL: URL,
        after afterURL: URL,
        options: TextInvarianceOptions = .init()
    ) throws -> TextInvarianceReport {
        let before = try EPUBArchiveReader(url: beforeURL)
        let after = try EPUBArchiveReader(url: afterURL)
        let beforePaths = Set(before.entryPaths().map(\.value).filter(isXHTMLPath))
        let afterPaths = Set(after.entryPaths().map(\.value).filter(isXHTMLPath))
        let expectedAfterPaths = Set(beforePaths.map { options.pathMap[$0] ?? $0 })
        var issues: [TextInvarianceIssue] = []

        for beforePath in beforePaths.sorted() {
            let afterPath = options.pathMap[beforePath] ?? beforePath
            guard afterPaths.contains(afterPath) else {
                issues.append(.init(
                    kind: .xhtmlDeleted,
                    path: beforePath,
                    message: "XHTML file is missing from after artifact: \(beforePath)"
                ))
                continue
            }
            let beforeAnalysis = try analyze(before.data(for: try ArchivePath(beforePath)), path: beforePath)
            let afterAnalysis = try analyze(after.data(for: try ArchivePath(afterPath)), path: afterPath)
            if options.checks.contains(.text), beforeAnalysis.blocks != afterAnalysis.blocks {
                issues.append(.init(
                    kind: .textModified,
                    path: beforePath,
                    message: "Normalized leaf-block text changed: \(beforePath) -> \(afterPath)"
                ))
            }
            if options.checks.contains(.anchors) {
                let missing = beforeAnalysis.anchorIDs.subtracting(afterAnalysis.anchorIDs)
                if !missing.isEmpty {
                    issues.append(.init(
                        kind: .anchorsRemoved,
                        path: beforePath,
                        message: "Anchor ids removed: \(missing.sorted().joined(separator: ", "))"
                    ))
                }
            }
        }

        for path in afterPaths.subtracting(expectedAfterPaths).sorted() {
            issues.append(.init(
                kind: .xhtmlAdded,
                path: path,
                message: "Unexpected XHTML file in after artifact: \(path)"
            ))
        }
        return TextInvarianceReport(issues: issues)
    }

    private static func isXHTMLPath(_ path: String) -> Bool {
        path.lowercased().hasSuffix(".xhtml") || path.lowercased().hasSuffix(".html")
    }

    private static func analyze(_ data: Data, path: String) throws -> XHTMLAnalysis {
        let delegate = XHTMLAnalysisDelegate()
        let parser = XMLParser(data: sanitizedXML(data))
        parser.delegate = delegate
        parser.shouldProcessNamespaces = false
        parser.shouldResolveExternalEntities = false
        guard parser.parse() else {
            throw TextInvarianceError.invalidXHTML(path)
        }
        return XHTMLAnalysis(blocks: delegate.blocks, anchorIDs: delegate.anchorIDs)
    }

    private static func sanitizedXML(_ data: Data) -> Data {
        let raw = String(decoding: data, as: UTF8.self)
        let doctypePattern = "(?is)<!DOCTYPE[^>]*(?:\\[[\\s\\S]*?\\]\\s*)?>"
        let range = NSRange(raw.startIndex..., in: raw)
        let regex = try? NSRegularExpression(pattern: doctypePattern)
        let withoutDoctype = regex?.stringByReplacingMatches(in: raw, range: range, withTemplate: "") ?? raw
        return Data(withoutDoctype.replacingOccurrences(of: "&nbsp;", with: "&#160;").utf8)
    }
}

private struct XHTMLAnalysis {
    let blocks: [String]
    let anchorIDs: Set<String>
}

private struct BlockContext {
    var text = ""
    var hasBlockDescendant = false
}

private final class XHTMLAnalysisDelegate: NSObject, XMLParserDelegate {
    private static let blockTags: Set<String> = ["p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "td", "blockquote", "pre", "div"]
    private static let ignoredTextTags: Set<String> = ["rt", "rp", "script", "style"]

    private(set) var blocks: [String] = []
    private(set) var anchorIDs = Set<String>()
    private var blockContexts: [BlockContext] = []
    private var ignoredTextDepth = 0
    private var controlElementStack: [Bool] = []

    func parser(
        _ parser: XMLParser,
        didStartElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?,
        attributes attributeDict: [String: String] = [:]
    ) {
        let name = elementName.lowercased()
        if let id = attributeDict["id"], !id.isEmpty {
            anchorIDs.insert(id)
        }
        let isNoteControl = Self.isNoteControl(name, attributes: attributeDict)
        controlElementStack.append(isNoteControl)
        if Self.ignoredTextTags.contains(name) || isNoteControl {
            ignoredTextDepth += 1
        }
        if Self.blockTags.contains(name) {
            if !blockContexts.isEmpty {
                blockContexts[blockContexts.count - 1].hasBlockDescendant = true
            }
            blockContexts.append(.init())
        }
    }

    func parser(_ parser: XMLParser, foundCharacters string: String) {
        guard ignoredTextDepth == 0 else {
            return
        }
        for index in blockContexts.indices {
            blockContexts[index].text.append(string)
        }
    }

    func parser(
        _ parser: XMLParser,
        didEndElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?
    ) {
        let name = elementName.lowercased()
        if Self.blockTags.contains(name), let context = blockContexts.popLast(), !context.hasBlockDescendant {
            let text = normalize(context.text)
            if !text.isEmpty {
                blocks.append(text)
            }
        }
        if Self.ignoredTextTags.contains(name) || controlElementStack.popLast() == true {
            ignoredTextDepth = max(0, ignoredTextDepth - 1)
        }
    }

    private func normalize(_ text: String) -> String {
        let noBreakSpaces = text.replacingOccurrences(of: "\u{00A0}", with: " ")
        return noBreakSpaces
            .components(separatedBy: .whitespacesAndNewlines)
            .filter { !$0.isEmpty }
            .joined(separator: " ")
            .precomposedStringWithCanonicalMapping
    }

    private static func isNoteControl(_ name: String, attributes: [String: String]) -> Bool {
        guard name == "a" else { return false }
        for (attribute, value) in attributes {
            let localName = attribute.split(separator: ":").last.map(String.init) ?? attribute
            if localName == "type", ["noteref", "backlink"].contains(where: { value.split(whereSeparator: \.isWhitespace).contains(Substring($0)) }) {
                return true
            }
        }
        return false
    }
}
