import Foundation
import EPUBContracts
import EPUBInspection
import Testing
import ZIPFoundation

@Test("package inspector reports OPF facts through the neutral inspection contract")
func packageInspectorReportsOPFFacts() throws {
    let url = FileManager.default.temporaryDirectory.appending(path: "package-inspector-test.epub")
    defer { try? FileManager.default.removeItem(at: url) }
    let archive = try Archive(url: url, accessMode: .create)
    try addEntry("META-INF/container.xml", contents: """
    <container><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml" /></rootfiles></container>
    """, to: archive)
    try addEntry("OEBPS/package.opf", contents: """
    <package><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml" /></manifest><spine><itemref idref="chapter" /></spine></package>
    """, to: archive)

    let report = PackageInspector.inspect(.init(uri: url, kind: .epub))

    #expect(report.status == .pass)
    #expect(report.findings.isEmpty)
    #expect(report.facts?["opfPath"] == "OEBPS/package.opf")
    #expect(report.facts?["manifestItemCount"] == "1")
}

private func addEntry(_ path: String, contents: String, to archive: Archive) throws {
    let data = Data(contents.utf8)
    try archive.addEntry(
        with: path,
        type: .file,
        uncompressedSize: Int64(data.count),
        compressionMethod: .deflate,
        provider: { position, size in
            let offset = Int(position)
            return data.subdata(in: offset..<(offset + size))
        }
    )
}
