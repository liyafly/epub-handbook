import Foundation
import EPUBArchive
import EPUBPackage
import SwiftSoup

public enum CSSCleanupArchiveTransformError: Error, Equatable, Sendable {
    case protectedPackage
    case opfIsNotUTF8
    case opfManifestMissing
    case xhtmlIsNotUTF8(String)
    case xhtmlBodyMissing(String)
    case xhtmlTransformFailed(String)
}

public struct CSSCleanupReport: Hashable, Codable, Sendable {
    public let opf: String
    public let cssFilesBefore: Int
    public let cssFilesAfter: Int
    public let factoredStylesheets: Int
    public let duplicateStylesheetsRemoved: Int
    public let overridesCreated: Int
    public let fontDeclarationsRewritten: Int
    public let xhtmlFilesUpdated: Int
    public let cssManifestItemsRemoved: Int
    public let cssManifestItemsAdded: Int
    public let scopedLocalStylesheetsMerged: Int
    public let scopeClassesAdded: Int
    public let warnings: [String]

    init(
        opf: String,
        plan: CSSCleanupPlan,
        xhtmlFilesUpdated: Int,
        cssManifestItemsRemoved: Int,
        cssManifestItemsAdded: Int
    ) {
        self.opf = opf
        cssFilesBefore = plan.cssFilesBefore
        cssFilesAfter = plan.cssFilesAfter
        factoredStylesheets = plan.factoredStylesheets
        duplicateStylesheetsRemoved = plan.duplicateStylesheetsRemoved
        overridesCreated = plan.overridesCreated
        fontDeclarationsRewritten = plan.fontDeclarationsRewritten
        self.xhtmlFilesUpdated = xhtmlFilesUpdated
        self.cssManifestItemsRemoved = cssManifestItemsRemoved
        self.cssManifestItemsAdded = cssManifestItemsAdded
        scopedLocalStylesheetsMerged = plan.scopedLocalStylesheetsMerged
        scopeClassesAdded = plan.bodyClasses.values.reduce(0) { $0 + $1.count }
        warnings = plan.warnings
    }
}

public typealias CSSCleanupInventory = StylesheetInventory

/// Native EPUB cleanup writer. It never modifies the source artifact, refuses
/// encrypted packages, and delegates archive ordering to `EPUBArchiveRewriter`.
public enum CSSCleanupArchiveTransformer {
    public static func analyze(epub: URL) throws -> CSSCleanupInventory {
        try StylesheetInventoryReader.analyze(epub: epub)
    }

    public static func transform(
        source: URL,
        to destination: URL,
        options: CSSCleanupOptions = .init()
    ) throws -> CSSCleanupReport {
        let archive = try EPUBArchiveReader(url: source)
        guard !archive.entryPaths().contains(where: { $0.value.lowercased() == "meta-inf/encryption.xml" }) else {
            throw CSSCleanupArchiveTransformError.protectedPackage
        }
        let inventory = try analyze(epub: source)
        let plan = try CSSCleanupPlanner.plan(inventory: inventory, options: options)
        var replacements = Dictionary(uniqueKeysWithValues: plan.cssContentUpdates.map { ($0.key, Data($0.value.utf8)) })
        var xhtmlFilesUpdated = 0

        for xhtmlPath in inventory.xhtmlPaths {
            let shouldRewriteLinks = inventory.references.contains { reference in
                reference.xhtmlPath == xhtmlPath && plan.linkReplacements[reference.stylesheetPath] != nil
            }
            let classes = plan.bodyClasses[xhtmlPath] ?? []
            guard shouldRewriteLinks || !classes.isEmpty else { continue }
            let data = try archive.data(for: xhtmlPath)
            guard let xhtml = String(data: data, encoding: .utf8) else {
                throw CSSCleanupArchiveTransformError.xhtmlIsNotUTF8(xhtmlPath.value)
            }
            let rewritten = try rewriteXHTML(
                xhtml,
                path: xhtmlPath,
                linkReplacements: plan.linkReplacements,
                bodyClasses: classes
            )
            if rewritten.didChange {
                replacements[xhtmlPath] = Data(rewritten.xhtml.utf8)
                xhtmlFilesUpdated += 1
            }
        }

        let opfData = try archive.data(for: inventory.opfPath)
        let opfRewrite = try rewriteOPF(
            opfData,
            opfPath: inventory.opfPath,
            removed: plan.removedStylesheets,
            generated: plan.generated
        )
        replacements[inventory.opfPath] = Data(opfRewrite.opf.utf8)
        let additions = Dictionary(uniqueKeysWithValues: plan.generated.map { ($0.path, Data($0.css.utf8)) })
        try EPUBArchiveRewriter.rewrite(
            source: source,
            to: destination,
            replacements: replacements,
            additions: additions,
            removals: plan.removedStylesheets
        )
        return .init(
            opf: inventory.opfPath.value,
            plan: plan,
            xhtmlFilesUpdated: xhtmlFilesUpdated,
            cssManifestItemsRemoved: opfRewrite.removedCount,
            cssManifestItemsAdded: opfRewrite.addedCount
        )
    }

    private static func rewriteXHTML(
        _ xhtml: String,
        path: ArchivePath,
        linkReplacements: [ArchivePath: [ArchivePath]],
        bodyClasses: [String]
    ) throws -> (xhtml: String, didChange: Bool) {
        do {
            let document = try SwiftSoup.parseXML(xhtml)
            var changed = false
            for link in try document.getElementsByTag("link") {
                let href = try link.attr("href")
                guard !href.isEmpty,
                      let sourcePath = try? EPUBStylesheetPath.resolve(href: href, relativeTo: path),
                      let targets = linkReplacements[sourcePath]
                else {
                    continue
                }
                let original = try link.outerHtml()
                for target in targets {
                    let replacementHref = try EPUBStylesheetPath.relativeHref(from: path, to: target)
                    try link.before(replacingHref(in: original, with: replacementHref))
                }
                try link.remove()
                changed = true
            }
            if !bodyClasses.isEmpty {
                guard let body = try document.getElementsByTag("body").first() else {
                    throw CSSCleanupArchiveTransformError.xhtmlBodyMissing(path.value)
                }
                for className in bodyClasses where !body.hasClass(className) {
                    try body.addClass(className)
                    changed = true
                }
            }
            return (try document.outerHtml(), changed)
        } catch let error as CSSCleanupArchiveTransformError {
            throw error
        } catch {
            throw CSSCleanupArchiveTransformError.xhtmlTransformFailed(path.value)
        }
    }

    private static func rewriteOPF(
        _ data: Data,
        opfPath: ArchivePath,
        removed: Set<ArchivePath>,
        generated: [GeneratedStylesheet]
    ) throws -> (opf: String, removedCount: Int, addedCount: Int) {
        guard let opf = String(data: data, encoding: .utf8) else {
            throw CSSCleanupArchiveTransformError.opfIsNotUTF8
        }
        do {
            let document = try SwiftSoup.parseXML(opf)
            guard let manifest = try document.getElementsByTag("manifest").first() else {
                throw CSSCleanupArchiveTransformError.opfManifestMissing
            }
            var removedCount = 0
            var ids = Set<String>()
            var hrefs = Set<String>()
            for item in try manifest.getElementsByTag("item") {
                let id = try item.attr("id")
                if !id.isEmpty { ids.insert(id) }
                let href = try item.attr("href")
                if !href.isEmpty { hrefs.insert(href) }
                guard (try item.attr("media-type")).lowercased() == "text/css",
                      let path = try? EPUBStylesheetPath.resolve(href: href, relativeTo: opfPath),
                      removed.contains(path)
                else {
                    continue
                }
                try item.remove()
                removedCount += 1
            }
            var addedCount = 0
            for stylesheet in generated {
                let href = try EPUBStylesheetPath.relativeHref(from: opfPath, to: stylesheet.path)
                guard !hrefs.contains(href) else { continue }
                let item = try manifest.appendElement("item")
                let id = uniqueManifestID(base: "css-\(filenameStem(stylesheet.path))", existing: ids)
                try item.attr("id", id)
                try item.attr("href", href)
                try item.attr("media-type", "text/css")
                ids.insert(id)
                hrefs.insert(href)
                addedCount += 1
            }
            return (try document.outerHtml(), removedCount, addedCount)
        } catch let error as CSSCleanupArchiveTransformError {
            throw error
        } catch {
            throw CSSCleanupArchiveTransformError.opfManifestMissing
        }
    }
}

private func replacingHref(in html: String, with href: String) -> String {
    let pattern = "(?i)(\\bhref\\s*=\\s*)([\"']).*?\\2"
    guard let expression = try? NSRegularExpression(pattern: pattern) else { return html }
    let range = NSRange(html.startIndex..<html.endIndex, in: html)
    return expression.stringByReplacingMatches(in: html, range: range, withTemplate: "$1\"\(href)\"")
}

private func uniqueManifestID(base: String, existing: Set<String>) -> String {
    let normalized = base.unicodeScalars.map { scalar -> Character in
        CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "_.-")).contains(scalar) ? Character(String(scalar)) : "-"
    }
    let stem = String(normalized).trimmingCharacters(in: CharacterSet(charactersIn: "-"))
    let safeBase = stem.isEmpty ? "css" : stem
    guard existing.contains(safeBase) else { return safeBase }
    var index = 2
    while existing.contains("\(safeBase)-\(index)") {
        index += 1
    }
    return "\(safeBase)-\(index)"
}
