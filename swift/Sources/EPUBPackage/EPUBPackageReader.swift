import Foundation
import EPUBArchive

public struct EPUBPackageSnapshot: Hashable, Codable, Sendable {
    public let opfPath: ArchivePath
    public let package: OPFPackageSnapshot

    public init(opfPath: ArchivePath, package: OPFPackageSnapshot) {
        self.opfPath = opfPath
        self.package = package
    }
}

/// Composes archive validation, container parsing, and OPF parsing into the
/// read-only package facts consumed by inspection and GUI code.
public enum EPUBPackageReader {
    public static func read(from url: URL) throws -> EPUBPackageSnapshot {
        let archive = try EPUBArchiveReader(url: url)
        let containerPath = try ArchivePath("META-INF/container.xml")
        let opfPath = try ContainerDocument.opfPath(from: archive.data(for: containerPath))
        let package = try OPFDocument.parse(archive.data(for: opfPath))
        return EPUBPackageSnapshot(opfPath: opfPath, package: package)
    }
}
