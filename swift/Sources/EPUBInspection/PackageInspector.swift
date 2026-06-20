import EPUBContracts
import EPUBPackage

/// Read-only package inspection. It reports facts and blockers; it never
/// repairs the input artifact or decides which transformation should follow.
public enum PackageInspector {
    public static func inspect(_ artifact: ArtifactReference) -> InspectionReport {
        guard artifact.kind == .epub else {
            return InspectionReport(
                schemaVersion: "1",
                artifact: artifact,
                findings: [
                    .init(
                        code: "artifact.kind.unsupported",
                        severity: .fatal,
                        message: "Package inspection requires an EPUB artifact."
                    ),
                ],
                status: .fail
            )
        }
        do {
            let snapshot = try EPUBPackageReader.read(from: artifact.uri)
            return InspectionReport(
                schemaVersion: "1",
                artifact: artifact,
                findings: [],
                status: .pass,
                facts: [
                    "opfPath": snapshot.opfPath.value,
                    "manifestItemCount": String(snapshot.package.manifest.count),
                    "spineItemCount": String(snapshot.package.spineItemIDs.count),
                    "navigationPath": snapshot.package.navigationItem?.href ?? "",
                ]
            )
        } catch {
            return InspectionReport(
                schemaVersion: "1",
                artifact: artifact,
                findings: [
                    .init(
                        code: "epub.package.unreadable",
                        severity: .fatal,
                        message: "Unable to read the EPUB container or OPF package."
                    ),
                ],
                status: .fail
            )
        }
    }
}
