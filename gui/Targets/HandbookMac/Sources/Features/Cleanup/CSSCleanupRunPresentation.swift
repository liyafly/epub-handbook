import Foundation
import EPUBContracts

enum CSSCleanupRunPresentation {
    static func suggestedOutputName(for input: URL) -> String {
        "\(input.deletingPathExtension().lastPathComponent)-css-cleaned.epub"
    }

    static func detail(for report: RunReport) -> String {
        var lines = [
            "status: \(report.status.rawValue)",
            "input: \(report.input.uri.path)",
        ]
        if let output = report.output {
            lines.append("output: \(output.uri.path)")
        }
        lines.append(contentsOf: report.events.map { event in
            event.message.map { "\(event.step): \(event.status.rawValue) — \($0)" }
                ?? "\(event.step): \(event.status.rawValue)"
        })
        return lines.joined(separator: "\n")
    }
}

enum CSSCleanupAvailability {
    static func isEnabled(reportStatus: InspectionStatus, hasInput: Bool) -> Bool {
        reportStatus == .pass && hasInput
    }
}
