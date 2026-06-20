import Foundation
import EPUBArchive

public enum XHTMLArchiveTransformError: Error, Equatable, Sendable {
    case xhtmlIsNotUTF8(String)
}

public struct XHTMLArchiveTransformResult: Hashable, Codable, Sendable {
    public let xhtmlPath: ArchivePath
    public let changedElementCount: Int

    public init(xhtmlPath: ArchivePath, changedElementCount: Int) {
        self.xhtmlPath = xhtmlPath
        self.changedElementCount = changedElementCount
    }
}

/// Applies one explicit XML-mode DOM change and writes a new EPUB artifact.
/// Approval, redline validation, and transaction commit remain callers' gates.
public enum XHTMLAttributeArchiveTransformer {
    public static func rewrite(
        source: URL,
        to destination: URL,
        xhtmlPath: ArchivePath,
        selector: String,
        name: String,
        value: String
    ) throws -> XHTMLArchiveTransformResult {
        let archive = try EPUBArchiveReader(url: source)
        let original = try archive.data(for: xhtmlPath)
        guard let xhtml = String(data: original, encoding: .utf8) else {
            throw XHTMLArchiveTransformError.xhtmlIsNotUTF8(xhtmlPath.value)
        }
        let transformed = try XHTMLTransformer.setAttribute(
            in: xhtml,
            selector: selector,
            name: name,
            value: value
        )
        try EPUBArchiveRewriter.rewrite(
            source: source,
            to: destination,
            replacements: [xhtmlPath: Data(transformed.xhtml.utf8)]
        )
        return XHTMLArchiveTransformResult(
            xhtmlPath: xhtmlPath,
            changedElementCount: transformed.changedElementCount
        )
    }
}
