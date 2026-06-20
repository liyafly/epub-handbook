import SwiftSoup

public enum PopupFootnoteNormalizeError: Error, Equatable, Sendable {
    case parseFailure
    case missingHTMLRoot
    case missingBody
    case duplicateIdentifier(String)
    case referenceWithoutLocalTarget(String)
    case referenceTargetMissing(String)
    case referenceRequiresExactlyOneImage(String)
    case referenceImageMissingSource(String)
    case duplicateTarget(String)
    case unreferencedFootnoteBody(String)
}

public struct PopupFootnoteNormalizationResult: Hashable, Codable, Sendable {
    public let xhtml: String
    public let normalizedReferenceCount: Int
    public let imageSources: [String]

    public init(xhtml: String, normalizedReferenceCount: Int, imageSources: [String]) {
        self.xhtml = xhtml
        self.normalizedReferenceCount = normalizedReferenceCount
        self.imageSources = imageSources
    }
}

/// Normalizes already-local, image-backed EPUB notes into the project popup
/// shape. Text-only markers are deliberately rejected: supplying a default
/// icon requires an OPF resource mutation and is a separate, explicit action.
public enum PopupFootnoteNormalizer {
    public static func normalize(in xhtml: String) throws -> PopupFootnoteNormalizationResult {
        do {
            let document = try SwiftSoup.parseXML(xhtml)
            guard let html = try document.getElementsByTag("html").first() else {
                throw PopupFootnoteNormalizeError.missingHTMLRoot
            }
            guard let body = try document.getElementsByTag("body").first() else {
                throw PopupFootnoteNormalizeError.missingBody
            }

            let allElements = try document.getAllElements().array()
            var identifiers = Set<String>()
            for element in allElements {
                let identifier = try element.attr("id")
                guard !identifier.isEmpty else { continue }
                guard identifiers.insert(identifier).inserted else {
                    throw PopupFootnoteNormalizeError.duplicateIdentifier(identifier)
                }
            }

            let references = try document.getElementsByTag("a").array().filter(isNoteReference)
            guard !references.isEmpty else {
                return .init(xhtml: try document.outerHtml(), normalizedReferenceCount: 0, imageSources: [])
            }
            let oldAsides = try document.getElementsByTag("aside").array().filter(isFootnoteAside)
            var records: [PopupNoteRecord] = []
            var targetIdentifiers = Set<String>()
            var imageSources: [String] = []

            for (index, reference) in references.enumerated() {
                let referenceID = try normalizedReferenceID(reference, ordinal: index + 1, knownIdentifiers: &identifiers)
                let href = try reference.attr("href")
                guard href.hasPrefix("#"), href.count > 1 else {
                    throw PopupFootnoteNormalizeError.referenceWithoutLocalTarget(referenceID)
                }
                let targetID = String(href.dropFirst())
                guard let target = try document.getElementById(targetID) else {
                    throw PopupFootnoteNormalizeError.referenceTargetMissing(targetID)
                }
                guard target !== reference else {
                    throw PopupFootnoteNormalizeError.referenceTargetMissing(targetID)
                }
                guard targetIdentifiers.insert(targetID).inserted else {
                    throw PopupFootnoteNormalizeError.duplicateTarget(targetID)
                }
                let images = try reference.getElementsByTag("img").array()
                guard images.count == 1 else {
                    throw PopupFootnoteNormalizeError.referenceRequiresExactlyOneImage(referenceID)
                }
                let image = images[0]
                let imageSource = try image.attr("src")
                guard !imageSource.isEmpty else {
                    throw PopupFootnoteNormalizeError.referenceImageMissingSource(referenceID)
                }
                if try image.attr("alt").isEmpty {
                    try image.attr("alt", "注")
                }
                try reference.html(image.outerHtml())
                try reference.attr("id", referenceID)
                try reference.attr("href", "#\(targetID)")
                try reference.attr("epub:type", "noteref")
                try reference.attr("role", "doc-noteref")
                try normalizeClasses(reference, required: "noteref-icon")
                imageSources.append(imageSource)
                records.append(.init(referenceID: referenceID, targetID: targetID, target: target))
            }

            try ensureNoUnreferencedBodies(in: oldAsides, targetIdentifiers: targetIdentifiers)
            try html.attr("xmlns:epub", "http://www.idpf.org/2007/ops")

            let groupedAside = try body.appendElement("aside")
            try groupedAside.attr("epub:type", "footnote")
            try groupedAside.attr("role", "doc-footnote")
            let divider = try groupedAside.appendElement("div")
            let line = try divider.appendElement("hr")
            try line.attr("class", "footnote-line xian")
            let list = try groupedAside.appendElement("ol")
            try list.attr("class", "footnote-list")

            for record in records {
                try normalizeTarget(record.target, targetID: record.targetID, referenceID: record.referenceID)
                try list.appendChild(record.target)
            }
            for aside in oldAsides {
                try aside.remove()
            }

            return .init(
                xhtml: try document.outerHtml(),
                normalizedReferenceCount: records.count,
                imageSources: imageSources
            )
        } catch let error as PopupFootnoteNormalizeError {
            throw error
        } catch {
            throw PopupFootnoteNormalizeError.parseFailure
        }
    }

    private static func normalizedReferenceID(
        _ reference: Element,
        ordinal: Int,
        knownIdentifiers: inout Set<String>
    ) throws -> String {
        let existing = try reference.attr("id")
        if !existing.isEmpty {
            return existing
        }
        var ordinal = ordinal
        var candidate = "note-\(ordinal)"
        while knownIdentifiers.contains(candidate) {
            ordinal += 1
            candidate = "note-\(ordinal)"
        }
        knownIdentifiers.insert(candidate)
        return candidate
    }

    private static func normalizeTarget(_ target: Element, targetID: String, referenceID: String) throws {
        try removeExistingBacklinks(from: target, referenceID: referenceID)
        let originalTag = target.tagName().lowercased()
        let originalContent = try target.html()
        try target.tagName("li")
        try target.attr("id", targetID)
        try normalizeClasses(target, required: "footnote-item")

        let paragraph: Element
        if originalTag == "p" {
            target.empty()
            paragraph = try target.appendElement("p")
            try paragraph.html(originalContent)
        } else if let firstParagraph = target.children().array().first(where: { $0.tagName().lowercased() == "p" }) {
            paragraph = firstParagraph
        } else {
            paragraph = try target.prependElement("p")
        }
        try normalizeClasses(paragraph, required: "footnote")
        try paragraph.prepend("<a class=\"footnote-back\" epub:type=\"backlink\" role=\"doc-backlink\" href=\"#\(referenceID)\">◎</a>")
    }

    private static func removeExistingBacklinks(from target: Element, referenceID: String) throws {
        let links = try target.getElementsByTag("a").array()
        for link in links {
            let href = try link.attr("href")
            let text = try link.text()
            let duplicateSymbolLink = href == "#\(referenceID)" && text == "◎"
            let backlink = try isBacklink(link)
            if backlink || duplicateSymbolLink {
                try link.remove()
            }
        }
    }

    private static func ensureNoUnreferencedBodies(in asides: [Element], targetIdentifiers: Set<String>) throws {
        for aside in asides {
            for element in try aside.getAllElements().array() where element !== aside {
                let identifier = try element.attr("id")
                if !identifier.isEmpty, !targetIdentifiers.contains(identifier) {
                    throw PopupFootnoteNormalizeError.unreferencedFootnoteBody(identifier)
                }
            }
        }
    }
}

private struct PopupNoteRecord {
    let referenceID: String
    let targetID: String
    let target: Element
}

private func isNoteReference(_ element: Element) throws -> Bool {
    try hasType(element, "noteref")
        || element.hasClass("duokan-footnote")
        || (try element.attr("role")) == "doc-noteref"
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

private func normalizeClasses(_ element: Element, required: String) throws {
    let obsolete = try element.classNames().filter { $0.hasPrefix("duokan-") }
    for className in obsolete {
        try element.removeClass(className)
    }
    try element.addClass(required)
}
