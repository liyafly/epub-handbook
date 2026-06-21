import Foundation

public enum CSSStatementKind: String, Codable, Hashable, Sendable {
    /// A top-level comment intentionally kept apart from adjacent rules.
    case trivia
    /// A non-`@` rule with one balanced declaration block.
    case qualifiedRule
    /// At-rules and malformed/unsupported top-level CSS that must be retained verbatim.
    case opaque
}

/// One top-level, byte-preserving CSS segment. The parser classifies only a
/// conservative subset; consumers must retain `.opaque` statements verbatim.
public struct CSSStatement: Codable, Hashable, Sendable {
    public let raw: String
    public let kind: CSSStatementKind

    public init(raw: String, kind: CSSStatementKind) {
        self.raw = raw
        self.kind = kind
    }
}

public enum CSSDocumentError: Error, Equatable, Sendable {
    case unterminatedComment
    case unterminatedString
    case unbalancedBlock
}

/// A deliberately small, lossless CSS Syntax scanner. It does not attempt to
/// compute a cascade; it only identifies top-level blocks while preserving the
/// original spelling, whitespace, comments and unsupported syntax.
public struct CSSDocument: Hashable, Sendable {
    public let statements: [CSSStatement]

    public init(statements: [CSSStatement]) {
        self.statements = statements
    }

    public var serialized: String {
        statements.map(\.raw).joined()
    }

    public static func parse(_ source: String) throws -> CSSDocument {
        let scanner = CSSScanner(characters: Array(source))
        return try CSSDocument(statements: scanner.scan())
    }
}

private struct CSSScanner {
    private let characters: [Character]

    init(characters: [Character]) {
        self.characters = characters
    }

    func scan() throws -> [CSSStatement] {
        var statements: [CSSStatement] = []
        var current = ""
        var index = 0
        var braceDepth = 0
        var parenthesisDepth = 0
        var bracketDepth = 0
        var quote: Character?

        while index < characters.count {
            let character = characters[index]

            if let activeQuote = quote {
                current.append(character)
                if character == "\\" {
                    let escapedIndex = index + 1
                    guard escapedIndex < characters.count else {
                        throw CSSDocumentError.unterminatedString
                    }
                    current.append(characters[escapedIndex])
                    index += 2
                    continue
                }
                if character == activeQuote {
                    quote = nil
                }
                index += 1
                continue
            }

            if character == "/", index + 1 < characters.count, characters[index + 1] == "*" {
                let commentEndIndex = try consumeComment(startingAt: index)
                let rawComment = String(characters[index..<commentEndIndex])
                if current.isEmpty, braceDepth == 0, parenthesisDepth == 0, bracketDepth == 0 {
                    statements.append(.init(raw: rawComment, kind: .trivia))
                } else {
                    current.append(contentsOf: rawComment)
                }
                index = commentEndIndex
                continue
            }

            current.append(character)
            if character == "\"" || character == "'" {
                quote = character
            } else if character == "(" {
                parenthesisDepth += 1
            } else if character == ")" {
                parenthesisDepth = max(0, parenthesisDepth - 1)
            } else if character == "[" {
                bracketDepth += 1
            } else if character == "]" {
                bracketDepth = max(0, bracketDepth - 1)
            } else if parenthesisDepth == 0, bracketDepth == 0, character == "{" {
                braceDepth += 1
            } else if parenthesisDepth == 0, bracketDepth == 0, character == "}" {
                guard braceDepth > 0 else {
                    throw CSSDocumentError.unbalancedBlock
                }
                braceDepth -= 1
                if braceDepth == 0 {
                    statements.append(statement(from: current))
                    current = ""
                }
            } else if parenthesisDepth == 0, bracketDepth == 0, braceDepth == 0, character == ";" {
                statements.append(statement(from: current))
                current = ""
            }
            index += 1
        }

        if quote != nil {
            throw CSSDocumentError.unterminatedString
        }
        if braceDepth != 0 {
            throw CSSDocumentError.unbalancedBlock
        }
        if !current.isEmpty {
            statements.append(statement(from: current))
        }
        return statements
    }

    private func consumeComment(startingAt start: Int) throws -> Int {
        var index = start + 2
        while index + 1 < characters.count {
            if characters[index] == "*", characters[index + 1] == "/" {
                return (index + 2)
            }
            index += 1
        }
        throw CSSDocumentError.unterminatedComment
    }

    private func statement(from raw: String) -> CSSStatement {
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return .init(raw: raw, kind: .trivia)
        }
        guard let openingBrace = trimmed.firstIndex(of: "{") else {
            return .init(raw: raw, kind: .opaque)
        }
        let prelude = trimmed[..<openingBrace].trimmingCharacters(in: .whitespacesAndNewlines)
        guard !prelude.isEmpty, !prelude.hasPrefix("@") else {
            return .init(raw: raw, kind: .opaque)
        }
        return .init(raw: raw, kind: .qualifiedRule)
    }
}
