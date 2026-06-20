import Foundation
import EPUBCLI
import EPUBContracts
import Testing
import ZIPFoundation

@Test("Swift CLI popup run records a baseline, passes native gates, and commits a new EPUB")
func swiftCLIPopupRunIsTransactional() async throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let input = directory.appending(path: "source.epub")
    let output = directory.appending(path: "normalized.epub")
    let workspace = directory.appending(path: "audit", directoryHint: .isDirectory)
    try makePopupEPUB(at: input)
    let sourceBytes = try Data(contentsOf: input)

    let report = await SwiftCLIService.normalizePopup(input: input, output: output, workspaceRoot: workspace)

    #expect(report.status == .complete)
    #expect(report.output?.uri == output)
    #expect(FileManager.default.fileExists(atPath: output.path))
    #expect(try Data(contentsOf: input) == sourceBytes)
    #expect(FileManager.default.fileExists(atPath: workspace.appending(path: "before/source.epub").path))
    #expect(report.events.contains { $0.step == "package-redlines" && $0.status == .completed })
}

@Test("Swift CLI popup run treats generated backlink text as a note control")
func swiftCLIPopupRunAllowsGeneratedBacklinkControl() async throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let input = directory.appending(path: "source.epub")
    let output = directory.appending(path: "normalized.epub")
    let workspace = directory.appending(path: "audit", directoryHint: .isDirectory)
    try makePopupEPUB(at: input, hasBacklinkSymbol: false)

    let report = await SwiftCLIService.normalizePopup(input: input, output: output, workspaceRoot: workspace)

    #expect(report.status == .complete)
    #expect(FileManager.default.fileExists(atPath: output.path))
    #expect(report.events.contains { $0.step == "text-and-anchors" && $0.status == .completed })
    #expect(FileManager.default.fileExists(atPath: workspace.appending(path: "before/source.epub").path))
}

private func makePopupEPUB(at url: URL, hasBacklinkSymbol: Bool = true) throws {
    let archive = try Archive(url: url, accessMode: .create)
    try add("mimetype", "application/epub+zip", .none, archive)
    try add("META-INF/container.xml", "<container><rootfiles><rootfile full-path=\"OEBPS/package.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>", .deflate, archive)
    try add("OEBPS/package.opf", """
    <package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Book</dc:title><dc:creator>A</dc:creator><dc:identifier>I</dc:identifier><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="icon" href="Images/note.png" media-type="image/png"/></manifest><spine><itemref idref="chapter"/></spine></package>
    """, .deflate, archive)
    let backlink = hasBacklinkSymbol ? "<a epub:type=\"backlink\" href=\"#note-one\">◎</a>" : ""
    try add("OEBPS/Text/chapter.xhtml", """
    <html xmlns="http://www.w3.org/1999/xhtml"><body><p>正文<a id="note-one" role="doc-noteref" href="#footnote-one"><img src="../Images/note.png" alt="注"/></a></p><aside role="doc-footnote"><p id="footnote-one">\(backlink)注释正文。</p></aside></body></html>
    """, .deflate, archive)
    try addData("OEBPS/Images/note.png", Data([0x89, 0x50]), .deflate, archive)
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
