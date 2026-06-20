import Foundation
import EPUBContracts
import EPUBInspection
import EPUBRuntime
import EPUBStructuredTransforms
import EPUBValidation

public struct SwiftRedlineValidationResult: Hashable, Codable, Sendable {
    public let schemaVersion: String
    public let status: RunStatus
    public let before: ArtifactReference
    public let after: ArtifactReference
    public let text: TextInvarianceReport
    public let package: PackageRedlineReport

    public init(
        schemaVersion: String = "1",
        status: RunStatus,
        before: ArtifactReference,
        after: ArtifactReference,
        text: TextInvarianceReport,
        package: PackageRedlineReport
    ) {
        self.schemaVersion = schemaVersion
        self.status = status
        self.before = before
        self.after = after
        self.text = text
        self.package = package
    }
}

public enum SwiftCLIService {
    public static func inspect(input: URL) -> InspectionReport {
        PackageInspector.inspect(.init(uri: input, kind: .epub))
    }

    public static func validateRedlines(
        before: URL,
        after: URL,
        pathMap: [String: String] = [:],
        allowStandardFontObfuscation: Bool = false
    ) throws -> SwiftRedlineValidationResult {
        let beforeArtifact = ArtifactReference(uri: before, kind: .epub)
        let afterArtifact = ArtifactReference(uri: after, kind: .epub)
        let text = try TextInvarianceValidator.validate(
            before: before,
            after: after,
            options: .init(pathMap: pathMap)
        )
        let package = try PackageRedlineValidator.validate(
            before: before,
            after: after,
            options: .init(
                pathMap: pathMap,
                allowStandardFontObfuscation: allowStandardFontObfuscation
            )
        )
        return .init(
            status: text.isValid && package.isValid ? .complete : .failed,
            before: beforeArtifact,
            after: afterArtifact,
            text: text,
            package: package
        )
    }

    /// Reads the same chained `stages[].mappings[]` shape used by the legacy
    /// structure normalizer, without importing or invoking that Python tool.
    public static func loadPathMap(from url: URL) throws -> [String: String] {
        let payload = try JSONSerialization.jsonObject(with: Data(contentsOf: url))
        let sources: [[String: Any]]
        if let dictionary = payload as? [String: Any], let stages = dictionary["stages"] as? [[String: Any]] {
            sources = stages
        } else if let dictionary = payload as? [String: Any] {
            sources = [dictionary]
        } else {
            throw PathMapError.invalidPayload
        }
        var map: [String: String] = [:]
        for source in sources {
            guard let mappings = source["mappings"] else { continue }
            guard let values = mappings as? [[String: Any]] else { throw PathMapError.invalidMappings }
            for value in values {
                guard let from = value["from"] as? String, let to = value["to"] as? String else {
                    throw PathMapError.invalidMappings
                }
                let chainedSources = map.compactMap { original, mapped in mapped == from ? original : nil }
                for original in chainedSources {
                    map[original] = to
                }
                map[from] = to
            }
        }
        return map
    }

    /// The explicit CLI command is the approval point for this write action.
    /// The work directory holds the before baseline, staged EPUB, and audit
    /// events so the output can be independently reviewed after commit.
    public static func normalizePopup(
        input: URL,
        output: URL,
        workspaceRoot: URL
    ) async -> RunReport {
        let inputArtifact = ArtifactReference(uri: input, kind: .epub)
        let workspace = Workspace(root: workspaceRoot)
        let transaction: Transaction
        do {
            transaction = try Transaction(
                workspace: workspace,
                requiredGateIDs: ["preflight", "popup-structure", "text-and-anchors", "package-redlines"]
            )
        } catch {
            return .init(
                schemaVersion: "1",
                status: .failed,
                input: inputArtifact,
                events: [.init(step: "transaction", status: .failed, message: String(describing: error))]
            )
        }

        do {
            let baseline = try await transaction.captureInput(inputArtifact)
            let inspection = PackageInspector.inspect(baseline)
            let preflight = try PackageRedlineValidator.validate(before: baseline.uri, after: baseline.uri)
            guard inspection.status != .fail, preflight.isValid else {
                let detail = inspection.findings.map(\.message).joined(separator: "; ")
                try await transaction.failGate("preflight", message: detail.isEmpty ? "Native package preflight failed." : detail)
                return try await rollbackReport(transaction: transaction, input: inputArtifact)
            }
            try await transaction.passGate("preflight")

            let nativeOutput = workspaceRoot.appending(path: "staging/native-popup-\(UUID().uuidString).epub")
            _ = try PopupFootnoteArchiveNormalizer.normalize(source: baseline.uri, to: nativeOutput)
            let staged = try await transaction.stage(
                Data(contentsOf: nativeOutput),
                filename: output.lastPathComponent.isEmpty ? "normalized.epub" : output.lastPathComponent
            )
            let popup = try PopupFootnoteValidator.validate(epub: staged)
            guard popup.isValid else {
                try await transaction.failGate("popup-structure", message: popup.issues.map(\.message).joined(separator: "; "))
                return try await rollbackReport(transaction: transaction, input: inputArtifact)
            }
            try await transaction.passGate("popup-structure")

            let text = try TextInvarianceValidator.validate(before: baseline.uri, after: staged)
            guard text.isValid else {
                try await transaction.failGate("text-and-anchors", message: text.issues.map(\.message).joined(separator: "; "))
                return try await rollbackReport(transaction: transaction, input: inputArtifact)
            }
            try await transaction.passGate("text-and-anchors")

            let package = try PackageRedlineValidator.validate(before: baseline.uri, after: staged)
            guard package.isValid else {
                try await transaction.failGate("package-redlines", message: package.issues.map(\.message).joined(separator: "; "))
                return try await rollbackReport(transaction: transaction, input: inputArtifact)
            }
            try await transaction.passGate("package-redlines")
            let artifact = try await transaction.commit(to: output)
            return .init(schemaVersion: "1", status: .complete, input: inputArtifact, output: artifact, events: await transaction.events())
        } catch {
            let currentEvents = await transaction.events()
            if !currentEvents.contains(where: { $0.step == "unexpected-error" }) {
                try? await transaction.rollback()
            }
            var events = await transaction.events()
            events.append(.init(step: "unexpected-error", status: .failed, message: String(describing: error)))
            return .init(schemaVersion: "1", status: .failed, input: inputArtifact, events: events)
        }
    }

    private static func rollbackReport(transaction: Transaction, input: ArtifactReference) async throws -> RunReport {
        try await transaction.rollback()
        return .init(schemaVersion: "1", status: .failed, input: input, events: await transaction.events())
    }
}

public enum PathMapError: Error, Equatable, Sendable {
    case invalidPayload
    case invalidMappings
}
