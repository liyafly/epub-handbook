import EPUBContracts

enum PreflightPresentation {
    static func summary(for report: InspectionReport) -> String {
        """
        status: \(report.status.rawValue)
        OPF: \(report.facts?["opfPath"] ?? "unknown")
        manifest items: \(report.facts?["manifestItemCount"] ?? "0")
        spine items: \(report.facts?["spineItemCount"] ?? "0")
        navigation: \(report.facts?["navigationPath"].flatMap { $0.isEmpty ? nil : $0 } ?? "none")
        """
    }
}
