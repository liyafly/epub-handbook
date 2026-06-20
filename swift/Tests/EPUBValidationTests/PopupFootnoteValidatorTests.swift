import Foundation
import EPUBValidation
import Testing
import ZIPFoundation

@Test("native popup validator accepts canonical grouped notes and rejects missing manifest icons")
func popupValidatorChecksStructureAndResources() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let valid = try makePopupEPUB(at: directory.appending(path: "valid.epub"), includesIconManifest: true)
    let invalid = try makePopupEPUB(at: directory.appending(path: "invalid.epub"), includesIconManifest: false)

    #expect(try PopupFootnoteValidator.validate(epub: valid).isValid)
    let report = try PopupFootnoteValidator.validate(epub: invalid)
    #expect(!report.isValid)
    #expect(report.issues.contains { $0.message.contains("manifest-backed") })
}

private func makePopupEPUB(at url: URL, includesIconManifest: Bool) throws -> URL {
    let archive = try Archive(url: url, accessMode: .create)
    try add("mimetype", "application/epub+zip", .none, archive)
    try add("META-INF/container.xml", "<container><rootfiles><rootfile full-path=\"OEBPS/package.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>", .deflate, archive)
    let iconManifest = includesIconManifest ? "<item id=\"icon\" href=\"Images/note.png\" media-type=\"image/png\"/>" : ""
    try add("OEBPS/package.opf", """
    <package xmlns="http://www.idpf.org/2007/opf"><metadata/><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>\(iconManifest)</manifest><spine><itemref idref="chapter"/></spine></package>
    """, .deflate, archive)
    try add("OEBPS/Text/chapter.xhtml", """
    <html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><p>正文<a id="note-one" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#footnote-one"><img src="../Images/note.png" alt="注"/></a></p><aside epub:type="footnote" role="doc-footnote"><ol class="footnote-list"><li id="footnote-one" class="footnote-item"><p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#note-one">◎</a>注释正文。</p></li></ol></aside></body></html>
    """, .deflate, archive)
    try addData("OEBPS/Images/note.png", Data([0x89, 0x50]), .deflate, archive)
    return url
}

private func add(_ path: String, _ value: String, _ compression: CompressionMethod, _ archive: Archive) throws {
    try addData(path, Data(value.utf8), compression, archive)
}

private func addData(_ path: String, _ data: Data, _ compression: CompressionMethod, _ archive: Archive) throws {
    try archive.addEntry(with: path, type: .file, uncompressedSize: Int64(data.count), compressionMethod: compression) { position, size in
        let offset = Int(position)
        return data.subdata(in: offset..<(offset + size))
    }
}
