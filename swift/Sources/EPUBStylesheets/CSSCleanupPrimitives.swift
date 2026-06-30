import Foundation
import Crypto

public struct CSSSanitizationResult: Hashable, Sendable {
    public let css: String
    public let fontRewrites: Int

    public init(css: String, fontRewrites: Int) {
        self.css = css
        self.fontRewrites = fontRewrites
    }
}

public enum CSSSanitizer {
    public static let songChain = "\"Songti SC\", \"SimSun\", \"Noto Serif CJK SC\", serif"
    public static let heiChain = "\"Heiti SC\", \"Microsoft YaHei\", \"Noto Sans CJK SC\", sans-serif"
    public static let kaiChain = "\"Kaiti SC\", \"STKaiti\", \"KaiTi\", serif"

    /// Mirrors the legacy Python cleanup's narrow set of font mappings. All
    /// other font-family values remain untouched, including generic families.
    public static func sanitize(_ source: String) throws -> CSSSanitizationResult {
        var css = try replaceMatches(
            in: source,
            pattern: "(?m)^\\s*[—-]{3,}.*?标题.*?[—-]{3,}\\s*$",
            replacement: ""
        )
        css = try replaceMatches(
            in: css,
            pattern: "(?m)(^\\s*[-\\w]+\\s*:\\s*[^;{}\\n]+)\\n(?=\\s*[-\\w]+\\s*:)",
            replacement: "$1;\\n"
        )

        let expression = try NSRegularExpression(
            pattern: "(font-family\\s*:\\s*)([^;}}]+)",
            options: [.caseInsensitive]
        )
        let range = NSRange(css.startIndex..<css.endIndex, in: css)
        let matches = expression.matches(in: css, range: range)
        var rewrites = 0
        for match in matches.reversed() {
            guard let valueRange = Range(match.range(at: 2), in: css),
                  let declarationRange = Range(match.range(at: 1), in: css),
                  let replacement = systemFontFamily(String(css[valueRange]))
            else {
                continue
            }
            css.replaceSubrange(valueRange, with: replacement)
            // Retain the original property spelling and spacing. The range is
            // intentionally read above to ensure a malformed zero-length
            // capture never counts as a rewrite.
            _ = declarationRange
            rewrites += 1
        }
        return .init(
            css: css.trimmingCharacters(in: .whitespacesAndNewlines) + "\n",
            fontRewrites: rewrites
        )
    }

    public static func systemFontFamily(_ value: String) -> String? {
        let compact = value.unicodeScalars
            .filter { !CharacterSet.whitespacesAndNewlines.contains($0) }
            .map(String.init)
            .joined()
            .lowercased()
        return switch compact {
        case "\"cnepub\",serif", "\"simsun\"":
            songChain
        case "\"simhei\"":
            heiChain
        case "\"stkaiti\"":
            kaiChain
        default:
            nil
        }
    }
}

public struct CSSDeclaration: Hashable, Sendable {
    public let name: String
    public let value: String

    public init(name: String, value: String) {
        self.name = name
        self.value = value
    }
}

public struct CSSRule: Hashable, Sendable {
    public let selector: String
    public let declarations: [CSSDeclaration]

    public init(selector: String, declarations: [CSSDeclaration]) {
        self.selector = selector
        self.declarations = declarations
    }
}

public struct CSSRuleShape: Hashable, Sendable {
    public let selector: String
    public let properties: [String]

    public init(selector: String, properties: [String]) {
        self.selector = selector
        self.properties = properties
    }
}

public extension Array where Element == CSSRule {
    var shape: [CSSRuleShape] {
        map { rule in
            CSSRuleShape(
                selector: rule.selector,
                properties: rule.declarations.map { $0.name.lowercased() }
            )
        }
    }
}

public enum CSSRuleParser {
    /// Returns `nil` whenever the stylesheet has any opaque at-rule or a
    /// declaration outside the conservative qualified-rule subset.
    public static func parse(_ source: String) throws -> [CSSRule]? {
        let document = try CSSDocument.parse(source)
        var rules: [CSSRule] = []
        for statement in document.statements {
            switch statement.kind {
            case .trivia:
                continue
            case .opaque:
                return nil
            case .qualifiedRule:
                guard let rule = try parseQualifiedRule(statement.raw) else {
                    return nil
                }
                rules.append(rule)
            }
        }
        return rules.isEmpty ? nil : rules
    }

    private static func parseQualifiedRule(_ raw: String) throws -> CSSRule? {
        let characters = Array(raw)
        guard let opening = try firstStructuralBrace(in: characters),
              let closing = try matchingBrace(in: characters, opening: opening)
        else {
            return nil
        }
        guard characters[(closing + 1)...].allSatisfy({ $0.isWhitespace }) else {
            return nil
        }
        let selector = normalizedSpace(stripComments(String(characters[..<opening])))
        guard !selector.isEmpty else {
            return nil
        }
        let body = stripComments(String(characters[(opening + 1)..<closing]))
        var declarations: [CSSDeclaration] = []
        for rawDeclaration in try splitTopLevel(body, separator: ";") {
            let declaration = rawDeclaration.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !declaration.isEmpty else {
                continue
            }
            guard let colon = try firstTopLevelSeparator(in: declaration, separator: ":") else {
                return nil
            }
            let name = declaration[..<colon].trimmingCharacters(in: .whitespacesAndNewlines)
            let value = declaration[declaration.index(after: colon)...].trimmingCharacters(in: .whitespacesAndNewlines)
            guard !name.isEmpty, !value.isEmpty else {
                return nil
            }
            declarations.append(.init(name: name, value: normalizedSpace(value)))
        }
        return CSSRule(selector: selector, declarations: declarations)
    }
}

public enum CSSFingerprint {
    /// The same normalization basis as the Python cleanup: remove comments and
    /// whitespace, then case-fold before computing SHA-256.
    public static func make(_ source: String) -> String {
        let normalized = stripComments(source)
            .unicodeScalars
            .filter { !CharacterSet.whitespacesAndNewlines.contains($0) }
            .map(String.init)
            .joined()
            .lowercased()
        return SHA256.hash(data: Data(normalized.utf8)).map { String(format: "%02x", $0) }.joined()
    }
}

public enum CSSRuleFormatter {
    public static func format(_ rules: [CSSRule]) -> String {
        var chunks: [String] = []
        for rule in rules {
            chunks.append("\(rule.selector) {")
            chunks.append(contentsOf: rule.declarations.map { "  \($0.name): \($0.value);" })
            chunks.append("}")
            chunks.append("")
        }
        return chunks.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines) + "\n"
    }
}

private func replaceMatches(in source: String, pattern: String, replacement: String) throws -> String {
    let expression = try NSRegularExpression(pattern: pattern)
    let range = NSRange(source.startIndex..<source.endIndex, in: source)
    return expression.stringByReplacingMatches(in: source, range: range, withTemplate: replacement)
}

private func stripComments(_ source: String) -> String {
    let characters = Array(source)
    var output = ""
    var index = 0
    var quote: Character?
    while index < characters.count {
        let character = characters[index]
        if let activeQuote = quote {
            output.append(character)
            if character == "\\", index + 1 < characters.count {
                output.append(characters[index + 1])
                index += 2
                continue
            }
            if character == activeQuote {
                quote = nil
            }
            index += 1
            continue
        }
        if character == "\"" || character == "'" {
            quote = character
            output.append(character)
            index += 1
            continue
        }
        if character == "/", index + 1 < characters.count, characters[index + 1] == "*" {
            index += 2
            while index + 1 < characters.count,
                  !(characters[index] == "*" && characters[index + 1] == "/") {
                index += 1
            }
            if index + 1 < characters.count {
                index += 2
            }
            continue
        }
        output.append(character)
        index += 1
    }
    return output
}

private func normalizedSpace(_ value: String) -> String {
    value.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
}

private func firstStructuralBrace(in characters: [Character]) throws -> Int? {
    var index = 0
    var quote: Character?
    while index < characters.count {
        let character = characters[index]
        if let activeQuote = quote {
            if character == "\\" {
                index += 2
                continue
            }
            if character == activeQuote {
                quote = nil
            }
            index += 1
            continue
        }
        if character == "\"" || character == "'" {
            quote = character
        } else if character == "/", index + 1 < characters.count, characters[index + 1] == "*" {
            index = try skipComment(in: characters, startingAt: index)
            continue
        } else if character == "{" {
            return index
        }
        index += 1
    }
    return nil
}

private func matchingBrace(in characters: [Character], opening: Int) throws -> Int? {
    var index = opening + 1
    var depth = 1
    var quote: Character?
    while index < characters.count {
        let character = characters[index]
        if let activeQuote = quote {
            if character == "\\" {
                index += 2
                continue
            }
            if character == activeQuote {
                quote = nil
            }
            index += 1
            continue
        }
        if character == "\"" || character == "'" {
            quote = character
        } else if character == "/", index + 1 < characters.count, characters[index + 1] == "*" {
            index = try skipComment(in: characters, startingAt: index)
            continue
        } else if character == "{" {
            depth += 1
        } else if character == "}" {
            depth -= 1
            if depth == 0 {
                return index
            }
        }
        index += 1
    }
    return nil
}

private func splitTopLevel(_ source: String, separator: Character) throws -> [String] {
    var parts: [String] = []
    var current = ""
    var parenthesisDepth = 0
    var bracketDepth = 0
    var quote: Character?
    let characters = Array(source)
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
            if character == activeQuote {
                quote = nil
            }
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
        } else if character == separator, parenthesisDepth == 0, bracketDepth == 0 {
            current.removeLast()
            parts.append(current)
            current = ""
        }
        index += 1
    }
    if quote != nil {
        throw CSSDocumentError.unterminatedString
    }
    parts.append(current)
    return parts
}

private func firstTopLevelSeparator(in source: String, separator: Character) throws -> String.Index? {
    var parenthesisDepth = 0
    var bracketDepth = 0
    var quote: Character?
    var index = source.startIndex
    while index < source.endIndex {
        let character = source[index]
        if let activeQuote = quote {
            if character == "\\" {
                index = source.index(after: index)
            } else if character == activeQuote {
                quote = nil
            }
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
        } else if character == separator, parenthesisDepth == 0, bracketDepth == 0 {
            return index
        }
        index = source.index(after: index)
    }
    if quote != nil {
        throw CSSDocumentError.unterminatedString
    }
    return nil
}

private func skipComment(in characters: [Character], startingAt start: Int) throws -> Int {
    var index = start + 2
    while index + 1 < characters.count {
        if characters[index] == "*", characters[index + 1] == "/" {
            return index + 2
        }
        index += 1
    }
    throw CSSDocumentError.unterminatedComment
}
