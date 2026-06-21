import Foundation

public struct CSSCleanupValidationIssue: Hashable, Codable, Sendable {
    public let path: String
    public let message: String

    public init(path: String, message: String) {
        self.path = path
        self.message = message
    }
}

public struct CSSCleanupValidationReport: Hashable, Codable, Sendable {
    public let issues: [CSSCleanupValidationIssue]

    public init(issues: [CSSCleanupValidationIssue]) {
        self.issues = issues
    }

    public var isValid: Bool { issues.isEmpty }
}

public enum CSSCleanupValidator {
    public static func validate(epub: URL) throws -> CSSCleanupValidationReport {
        let inventory = try StylesheetInventoryReader.analyze(epub: epub)
        var issues: [CSSCleanupValidationIssue] = inventory.warnings.map {
            .init(path: "EPUB", message: $0)
        }
        for stylesheet in inventory.stylesheets {
            do {
                _ = try CSSDocument.parse(stylesheet.css)
            } catch {
                issues.append(.init(path: stylesheet.path.value, message: "CSS scanner rejected stylesheet: \(error)"))
            }
        }
        return .init(issues: issues)
    }
}
