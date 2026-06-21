import Foundation
import EPUBArchive

public struct CSSCleanupOptions: Hashable, Codable, Sendable {
    public var mergeScopedLocalStylesheets: Bool

    public init(mergeScopedLocalStylesheets: Bool = false) {
        self.mergeScopedLocalStylesheets = mergeScopedLocalStylesheets
    }
}

public struct GeneratedStylesheet: Hashable, Sendable {
    public let path: ArchivePath
    public let css: String

    public init(path: ArchivePath, css: String) {
        self.path = path
        self.css = css
    }
}

/// A complete, non-mutating stylesheet rewrite plan. Archive and DOM writers
/// consume it only after all source CSS and XHTML references have been read.
public struct CSSCleanupPlan: Sendable {
    public let cssContentUpdates: [ArchivePath: String]
    public let generated: [GeneratedStylesheet]
    public let removedStylesheets: Set<ArchivePath>
    public let linkReplacements: [ArchivePath: [ArchivePath]]
    public let bodyClasses: [ArchivePath: [String]]
    public let cssFilesBefore: Int
    public let cssFilesAfter: Int
    public let factoredStylesheets: Int
    public let duplicateStylesheetsRemoved: Int
    public let overridesCreated: Int
    public let fontDeclarationsRewritten: Int
    public let scopedLocalStylesheetsMerged: Int
    public let warnings: [String]

    init(
        cssContentUpdates: [ArchivePath: String],
        generated: [GeneratedStylesheet],
        removedStylesheets: Set<ArchivePath>,
        linkReplacements: [ArchivePath: [ArchivePath]],
        bodyClasses: [ArchivePath: [String]],
        cssFilesBefore: Int,
        factoredStylesheets: Int,
        duplicateStylesheetsRemoved: Int,
        overridesCreated: Int,
        fontDeclarationsRewritten: Int,
        scopedLocalStylesheetsMerged: Int,
        warnings: [String]
    ) {
        self.cssContentUpdates = cssContentUpdates
        self.generated = generated.sorted { $0.path.value < $1.path.value }
        self.removedStylesheets = removedStylesheets
        self.linkReplacements = linkReplacements
        self.bodyClasses = bodyClasses
        self.cssFilesBefore = cssFilesBefore
        cssFilesAfter = cssContentUpdates.count + generated.count
        self.factoredStylesheets = factoredStylesheets
        self.duplicateStylesheetsRemoved = duplicateStylesheetsRemoved
        self.overridesCreated = overridesCreated
        self.fontDeclarationsRewritten = fontDeclarationsRewritten
        self.scopedLocalStylesheetsMerged = scopedLocalStylesheetsMerged
        self.warnings = warnings
    }
}

public enum CSSCleanupPlanner {
    public static func plan(inventory: StylesheetInventory, options: CSSCleanupOptions) throws -> CSSCleanupPlan {
        let sourceByPath = Dictionary(uniqueKeysWithValues: inventory.stylesheets.map { ($0.path, $0) })
        var sanitized: [ArchivePath: String] = [:]
        var parsedRules: [ArchivePath: [CSSRule]] = [:]
        var warnings = inventory.warnings
        var fontRewrites = 0

        for source in inventory.stylesheets {
            let result = try CSSSanitizer.sanitize(source.css)
            sanitized[source.path] = result.css
            fontRewrites += result.fontRewrites
            if let rules = try CSSRuleParser.parse(result.css) {
                parsedRules[source.path] = rules
            } else {
                warnings.append("skipped unsupported stylesheet syntax: \(source.path.value)")
            }
        }

        var occupied = Set(sourceByPath.keys)
        var generated: [ArchivePath: String] = [:]
        var generatedRules: [ArchivePath: [CSSRule]] = [:]
        var removed = Set<ArchivePath>()
        var initialLinkReplacements: [ArchivePath: [ArchivePath]] = [:]
        var factored = 0
        var duplicates = 0
        var overrides = 0

        var groups: [[CSSRuleShape]: [ArchivePath]] = [:]
        for (path, rules) in parsedRules where !isCleanupGenerated(path) {
            groups[rules.shape, default: []].append(path)
        }
        var sharedIndex = 1
        for paths in groups.values.sorted(by: { ($0.map(\.value).min() ?? "") < ($1.map(\.value).min() ?? "") }) {
            let sortedPaths = paths.sorted { $0.value < $1.value }
            guard sortedPaths.count >= 3,
                  Set(sortedPaths.compactMap { sanitized[$0].map(CSSFingerprint.make) }).count >= 2,
                  let canonical = sortedPaths.first,
                  let canonicalRules = parsedRules[canonical]
            else {
                continue
            }
            let base = try siblingPath(of: canonical, filename: String(format: "clean-shared-%02d.css", sharedIndex))
            let sharedPath = try uniquePath(base: base, occupied: &occupied)
            sharedIndex += 1
            generated[sharedPath] = CSSRuleFormatter.format(canonicalRules)
            generatedRules[sharedPath] = canonicalRules
            for path in sortedPaths {
                guard let rules = parsedRules[path] else { continue }
                var replacements = [sharedPath]
                let changedRules = zip(rules, canonicalRules).compactMap { pair in
                    pair.0.declarations == pair.1.declarations ? nil : pair.0
                }
                if !changedRules.isEmpty {
                    let stem = filenameStem(path)
                    let overrideBase = try siblingPath(of: canonical, filename: "clean-override-\(stem).css")
                    let overridePath = try uniquePath(base: overrideBase, occupied: &occupied)
                    generated[overridePath] = CSSRuleFormatter.format(changedRules)
                    generatedRules[overridePath] = changedRules
                    replacements.append(overridePath)
                    overrides += 1
                }
                initialLinkReplacements[path] = replacements
                removed.insert(path)
            }
            factored += sortedPaths.count
        }

        var canonicalByDigest: [String: ArchivePath] = [:]
        for path in sanitized.keys.sorted(by: { $0.value < $1.value }) where !removed.contains(path) {
            guard let css = sanitized[path] else { continue }
            let digest = CSSFingerprint.make(css)
            if let canonical = canonicalByDigest[digest] {
                initialLinkReplacements[path] = [canonical]
                removed.insert(path)
                duplicates += 1
            } else {
                canonicalByDigest[digest] = path
            }
        }

        var cssContentUpdates = sanitized.filter { !removed.contains($0.key) }
        var finalLinkReplacements = initialLinkReplacements
        var bodyClasses: [ArchivePath: [String]] = [:]
        var scopedMerged = 0

        if options.mergeScopedLocalStylesheets {
            let scopeResult = try mergeDisjointLocalStylesheets(
                inventory: inventory,
                cssContentUpdates: &cssContentUpdates,
                generated: &generated,
                generatedRules: &generatedRules,
                occupied: &occupied,
                initialLinkReplacements: initialLinkReplacements,
                finalLinkReplacements: &finalLinkReplacements,
                removed: &removed,
                warnings: &warnings
            )
            bodyClasses = scopeResult.bodyClasses
            scopedMerged = scopeResult.mergedCount
        }

        return CSSCleanupPlan(
            cssContentUpdates: cssContentUpdates,
            generated: generated.map { .init(path: $0.key, css: $0.value) },
            removedStylesheets: removed,
            linkReplacements: finalLinkReplacements,
            bodyClasses: bodyClasses,
            cssFilesBefore: inventory.stylesheets.count,
            factoredStylesheets: factored,
            duplicateStylesheetsRemoved: duplicates,
            overridesCreated: overrides,
            fontDeclarationsRewritten: fontRewrites,
            scopedLocalStylesheetsMerged: scopedMerged,
            warnings: warnings
        )
    }

    private static func mergeDisjointLocalStylesheets(
        inventory: StylesheetInventory,
        cssContentUpdates: inout [ArchivePath: String],
        generated: inout [ArchivePath: String],
        generatedRules: inout [ArchivePath: [CSSRule]],
        occupied: inout Set<ArchivePath>,
        initialLinkReplacements: [ArchivePath: [ArchivePath]],
        finalLinkReplacements: inout [ArchivePath: [ArchivePath]],
        removed: inout Set<ArchivePath>,
        warnings: inout [String]
    ) throws -> (bodyClasses: [ArchivePath: [String]], mergedCount: Int) {
        var effectiveRules: [ArchivePath: [CSSRule]] = [:]
        for path in cssContentUpdates.keys {
            if let rules = try CSSRuleParser.parse(cssContentUpdates[path] ?? "") {
                effectiveRules[path] = rules
            }
        }
        effectiveRules.merge(generatedRules, uniquingKeysWith: { _, right in right })

        var references: [ArchivePath: Set<ArchivePath>] = [:]
        for reference in inventory.references {
            let destinations = initialLinkReplacements[reference.stylesheetPath] ?? [reference.stylesheetPath]
            for destination in destinations where effectiveRules[destination] != nil {
                references[destination, default: []].insert(reference.xhtmlPath)
            }
        }

        let excludedNames: Set<String> = ["epub3-enhancements.css", "anthology-refinement.css", "clean-scoped-local.css"]
        var candidates: [ArchivePath] = []
        for path in effectiveRules.keys.sorted(by: { $0.value < $1.value }) {
            let name = filename(path).lowercased()
            guard !excludedNames.contains(name),
                  !name.hasPrefix("clean-shared-"),
                  let pages = references[path],
                  !pages.isEmpty,
                  pages.count * 2 <= inventory.xhtmlPaths.count
            else {
                continue
            }
            candidates.append(path)
        }

        var overlapping = Set<ArchivePath>()
        for (index, path) in candidates.enumerated() {
            for other in candidates.dropFirst(index + 1) where !(references[path, default: []].intersection(references[other, default: []])).isEmpty {
                overlapping.insert(path)
                overlapping.insert(other)
            }
        }
        if !overlapping.isEmpty {
            warnings.append("skipped overlapping local stylesheets: \(overlapping.map(\.value).sorted().joined(separator: ", "))")
        }
        let mergePaths = candidates.filter { !overlapping.contains($0) }
        guard !mergePaths.isEmpty else {
            return ([:], 0)
        }

        var chunks: [String] = []
        var scopeByPath: [ArchivePath: String] = [:]
        for (offset, path) in mergePaths.enumerated() {
            guard let rules = effectiveRules[path] else { continue }
            let scope = String(format: "css-local-%02d", offset + 1)
            guard let scopedRules = scopedRules(rules, scopeClass: scope) else {
                warnings.append("skipped unsafe selector while merging local stylesheet: \(path.value)")
                continue
            }
            scopeByPath[path] = scope
            chunks.append("/* Scoped from \(path.value). */")
            chunks.append(CSSRuleFormatter.format(scopedRules).trimmingCharacters(in: .whitespacesAndNewlines))
        }
        guard !scopeByPath.isEmpty else {
            return ([:], 0)
        }

        let scopedBase = try siblingPath(of: inventory.opfPath, suffixDirectory: "Styles", filename: "clean-scoped-local.css")
        let scopedPath = try uniquePath(base: scopedBase, occupied: &occupied)
        generated[scopedPath] = chunks.joined(separator: "\n\n").trimmingCharacters(in: .whitespacesAndNewlines) + "\n"
        for path in scopeByPath.keys {
            if generated.removeValue(forKey: path) != nil {
                generatedRules.removeValue(forKey: path)
            } else {
                cssContentUpdates.removeValue(forKey: path)
                removed.insert(path)
            }
        }

        var bodyClasses: [ArchivePath: [String]] = [:]
        for reference in inventory.references {
            let initial = initialLinkReplacements[reference.stylesheetPath] ?? [reference.stylesheetPath]
            let final = initial.map { scopeByPath[$0].map { _ in scopedPath } ?? $0 }
            if final != initial {
                finalLinkReplacements[reference.stylesheetPath] = final
            }
            for path in initial {
                guard let scope = scopeByPath[path] else { continue }
                if !(bodyClasses[reference.xhtmlPath] ?? []).contains(scope) {
                    bodyClasses[reference.xhtmlPath, default: []].append(scope)
                }
            }
        }
        return (bodyClasses, scopeByPath.count)
    }
}

private func isCleanupGenerated(_ path: ArchivePath) -> Bool {
    let name = filename(path)
    return name.hasPrefix("clean-shared-") || name.hasPrefix("clean-override-") || name.hasPrefix("clean-scoped-local")
}

private func filename(_ path: ArchivePath) -> String {
    path.value.split(separator: "/").last.map(String.init) ?? path.value
}

private func filenameStem(_ path: ArchivePath) -> String {
    let name = filename(path)
    guard let dot = name.lastIndex(of: ".") else { return name }
    return String(name[..<dot])
}

private func siblingPath(of path: ArchivePath, filename: String) throws -> ArchivePath {
    let parent = path.value.split(separator: "/").dropLast()
    return try ArchivePath((parent + [Substring(filename)]).map(String.init).joined(separator: "/"))
}

private func siblingPath(of path: ArchivePath, suffixDirectory: String, filename: String) throws -> ArchivePath {
    let parent = path.value.split(separator: "/").dropLast().map(String.init)
    return try ArchivePath((parent + [suffixDirectory, filename]).joined(separator: "/"))
}

private func uniquePath(base: ArchivePath, occupied: inout Set<ArchivePath>) throws -> ArchivePath {
    var candidate = base
    var index = 2
    while occupied.contains(candidate) {
        let name = filename(candidate)
        let stem = filenameStem(candidate)
        let ext = name.dropFirst(stem.count)
        candidate = try siblingPath(of: candidate, filename: "\(stem)-\(index)\(ext)")
        index += 1
    }
    occupied.insert(candidate)
    return candidate
}

private func scopedRules(_ rules: [CSSRule], scopeClass: String) -> [CSSRule]? {
    var result: [CSSRule] = []
    for rule in rules {
        guard let selector = scopedSelector(rule.selector, scopeClass: scopeClass) else {
            return nil
        }
        result.append(.init(selector: selector, declarations: rule.declarations))
    }
    return result
}

private func scopedSelector(_ selector: String, scopeClass: String) -> String? {
    let selectors = splitSelectorList(selector)
    guard !selectors.isEmpty else { return nil }
    var scoped: [String] = []
    for rawSelector in selectors {
        let selector = rawSelector.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !selector.isEmpty, !selector.hasPrefix("@") else { return nil }
        let lower = selector.lowercased()
        if lower == "body" || lower.hasPrefix("body.") || lower.hasPrefix("body#") || lower.hasPrefix("body:") || lower.hasPrefix("body[") || lower.hasPrefix("body ") {
            let afterBody = selector.index(selector.startIndex, offsetBy: 4)
            scoped.append("body.\(scopeClass)\(selector[afterBody...])")
        } else {
            scoped.append("body.\(scopeClass) \(selector)")
        }
    }
    return scoped.joined(separator: ",\n")
}

private func splitSelectorList(_ selector: String) -> [String] {
    var parts: [String] = []
    var current = ""
    var parenthesisDepth = 0
    var bracketDepth = 0
    var quote: Character?
    let characters = Array(selector)
    var index = 0
    while index < characters.count {
        let character = characters[index]
        current.append(character)
        if let activeQuote = quote {
            if character == "\\", index + 1 < characters.count {
                current.append(characters[index + 1])
                index += 2
                continue
            }
            if character == activeQuote { quote = nil }
        } else if character == "\"" || character == "'" {
            quote = character
        } else if character == "(" {
            parenthesisDepth += 1
        } else if character == ")" {
            parenthesisDepth = max(0, parenthesisDepth - 1)
        } else if character == "[" {
            bracketDepth += 1
        } else if character == "]" {
            bracketDepth = max(0, bracketDepth - 1)
        } else if character == ",", parenthesisDepth == 0, bracketDepth == 0 {
            current.removeLast()
            parts.append(current)
            current = ""
        }
        index += 1
    }
    guard quote == nil, parenthesisDepth == 0, bracketDepth == 0 else { return [] }
    parts.append(current)
    return parts
}
