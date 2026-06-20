import Foundation
import EPUBPackage
import Testing
import ZIPFoundation

@Test("EPUB package reader resolves container XML and reads the OPF snapshot")
func epubPackageReaderResolvesContainerAndOPF() throws {
    let url = FileManager.default.temporaryDirectory.appending(path: "package-reader-test.epub")
    defer { try? FileManager.default.removeItem(at: url) }
    let archive = try Archive(url: url, accessMode: .create)
    try addEntry("META-INF/container.xml", contents: """
    <?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml" /></rootfiles></container>
    """, to: archive)
    try addEntry("OEBPS/package.opf", contents: """
    <package unique-identifier="book-id"><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml" /></manifest><spine><itemref idref="chapter" /></spine></package>
    """, to: archive)

    let snapshot = try EPUBPackageReader.read(from: url)

    #expect(snapshot.opfPath.value == "OEBPS/package.opf")
    #expect(snapshot.package.manifest.map(\.href) == ["chapter.xhtml"])
    #expect(snapshot.package.spineItemIDs == ["chapter"])
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
