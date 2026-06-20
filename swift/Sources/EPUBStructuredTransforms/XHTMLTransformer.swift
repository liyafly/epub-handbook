import SwiftSoup

public enum XHTMLTransformError: Error, Equatable, Sendable {
    case parseOrSelectorFailure
    case noMatchingElements(String)
}

public struct XHTMLTransformResult: Hashable, Codable, Sendable {
    public let xhtml: String
    public let changedElementCount: Int

    public init(xhtml: String, changedElementCount: Int) {
        self.xhtml = xhtml
        self.changedElementCount = changedElementCount
    }
}

/// Explicit XML-mode DOM transforms for EPUB XHTML.
///
/// This type deliberately produces canonicalized XHTML. Callers must use a new
/// output artifact and run the existing redline and package validators before
/// committing its serialized result.
public enum XHTMLTransformer {
    public static func setAttribute(
        in xhtml: String,
        selector: String,
        name: String,
        value: String
    ) throws -> XHTMLTransformResult {
        do {
            // EPUB XHTML often omits an XML declaration. `parseXML` is required
            // so SwiftSoup does not select its HTML5 parser based on that fact.
            let document = try SwiftSoup.parseXML(xhtml)
            let elements = try document.select(selector)
            guard !elements.isEmpty() else {
                throw XHTMLTransformError.noMatchingElements(selector)
            }
            for element in elements {
                try element.attr(name, value)
            }
            return XHTMLTransformResult(
                xhtml: try document.outerHtml(),
                changedElementCount: elements.count
            )
        } catch let error as XHTMLTransformError {
            throw error
        } catch {
            throw XHTMLTransformError.parseOrSelectorFailure
        }
    }
}
