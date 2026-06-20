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
    case unrecognizedFootnoteSection
}

public struct PopupFootnoteNormalizationResult: Hashable, Codable, Sendable {
    public let xhtml: String
    public let normalizedReferenceCount: Int
    public let imageSources: [String]
    public let usedDefaultIcon: Bool
    public let didChange: Bool

    public init(
        xhtml: String,
        normalizedReferenceCount: Int,
        imageSources: [String],
        usedDefaultIcon: Bool = false,
        didChange: Bool = false
    ) {
        self.xhtml = xhtml
        self.normalizedReferenceCount = normalizedReferenceCount
        self.imageSources = imageSources
        self.usedDefaultIcon = usedDefaultIcon
        self.didChange = didChange
    }
}

/// Normalizes local EPUB notes into the project popup shape. Callers that
/// allow a text marker to be repaired must pass the package-local icon source
/// explicitly; direct callers remain strict by default.
public enum PopupFootnoteNormalizer {
    public static func normalize(
        in xhtml: String,
        defaultIconSource: String? = nil,
        defaultLanguage: String? = nil
    ) throws -> PopupFootnoteNormalizationResult {
        do {
            let document = try SwiftSoup.parseXML(xhtml)
            guard let html = try document.getElementsByTag("html").first() else {
                throw PopupFootnoteNormalizeError.missingHTMLRoot
            }
            guard let body = try document.getElementsByTag("body").first() else {
                throw PopupFootnoteNormalizeError.missingBody
            }
            let languageChanged = try normalizeLanguage(on: html, defaultLanguage: defaultLanguage)

            let allElements = try document.getAllElements().array()
            var identifiers = Set<String>()
            for element in allElements {
                let identifier = try element.attr("id")
                guard !identifier.isEmpty else { continue }
                guard identifiers.insert(identifier).inserted else {
                    throw PopupFootnoteNormalizeError.duplicateIdentifier(identifier)
                }
            }

            let candidates = try document.getElementsByTag("a").array().filter(isNoteReference)
            let references = try candidates.filter { reference in
                let href = try reference.attr("href")
                guard href.hasPrefix("#"), href.count > 1,
                      let target = try document.getElementById(String(href.dropFirst()))
                else {
                    return true
                }
                guard target.tagName().lowercased() == "a" else {
                    return true
                }
                return try !isNoteReference(target)
            }
            guard !references.isEmpty else {
                return .init(
                    xhtml: try document.outerHtml(),
                    normalizedReferenceCount: 0,
                    imageSources: [],
                    didChange: languageChanged
                )
            }
            let oldAsides = try document.getElementsByTag("aside").array().filter(isFootnoteAside)
            let oldSections = try document.getElementsByTag("section").array().filter(isFootnoteSection)
            var records: [PopupNoteRecord] = []
            var targetIdentifiers = Set<String>()
            var imageSources: [String] = []
            var usedDefaultIcon = false

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
                let existingImages = try reference.getElementsByTag("img").array()
                let image: Element
                switch existingImages.count {
                case 1:
                    image = existingImages[0]
                case 0:
                    guard let defaultIconSource, !defaultIconSource.isEmpty else {
                        throw PopupFootnoteNormalizeError.referenceRequiresExactlyOneImage(referenceID)
                    }
                    reference.empty()
                    image = try reference.appendElement("img")
                    try image.attr("src", defaultIconSource)
                    try image.attr("alt", "注")
                    usedDefaultIcon = true
                default:
                    throw PopupFootnoteNormalizeError.referenceRequiresExactlyOneImage(referenceID)
                }
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
            try ensureSectionsAreFullyRecognized(oldSections, targetIdentifiers: targetIdentifiers)
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
            for aside in oldAsides where !records.contains(where: { $0.target === aside }) {
                try aside.remove()
            }
            for section in oldSections {
                try section.remove()
            }

            return .init(
                xhtml: try document.outerHtml(),
                normalizedReferenceCount: records.count,
                imageSources: imageSources,
                usedDefaultIcon: usedDefaultIcon,
                didChange: true
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

    private static func normalizeLanguage(on html: Element, defaultLanguage: String?) throws -> Bool {
        let existingLanguage = try html.attr("lang")
        let existingXMLLanguage = try html.attr("xml:lang")
        if !existingLanguage.isEmpty && !existingXMLLanguage.isEmpty {
            return false
        }
        if !existingLanguage.isEmpty {
            try html.attr("xml:lang", existingLanguage)
            return true
        }
        if !existingXMLLanguage.isEmpty {
            try html.attr("lang", existingXMLLanguage)
            return true
        }
        guard let defaultLanguage, !defaultLanguage.isEmpty else {
            return false
        }
        try html.attr("lang", defaultLanguage)
        try html.attr("xml:lang", defaultLanguage)
        return true
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
            let isLegacySourceControl = try isNoteReference(link)
            let legacySourceControl = href == "#\(referenceID)" && isLegacySourceControl
            if backlink || duplicateSymbolLink || legacySourceControl {
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

    private static func ensureSectionsAreFullyRecognized(_ sections: [Element], targetIdentifiers: Set<String>) throws {
        for section in sections {
            let asides = try section.getElementsByTag("aside").array().filter(isFootnoteAside)
            guard !asides.isEmpty else {
                throw PopupFootnoteNormalizeError.unrecognizedFootnoteSection
            }
            let nonAsideChildren = section.children().array().filter { $0.tagName().lowercased() != "aside" }
            guard nonAsideChildren.isEmpty else {
                throw PopupFootnoteNormalizeError.unrecognizedFootnoteSection
            }
            for aside in asides {
                let identifier = try aside.attr("id")
                guard targetIdentifiers.contains(identifier) else {
                    throw PopupFootnoteNormalizeError.unrecognizedFootnoteSection
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

private func isFootnoteSection(_ element: Element) throws -> Bool {
    try hasType(element, "footnotes")
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
