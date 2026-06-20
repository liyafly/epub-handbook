import Foundation
import EPUBArchive
import EPUBStructuredTransforms
import Testing
import ZIPFoundation

@Test("native XHTML attribute transform writes a new EPUB artifact")
func xhtmlAttributeTransformWritesNewEPUB() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let source = directory.appending(path: "source.epub")
    let destination = directory.appending(path: "output.epub")
    let archive = try Archive(url: source, accessMode: .create)
    try addEntry("mimetype", data: Data("application/epub+zip".utf8), compression: .none, to: archive)
    try addEntry("OEBPS/chapter.xhtml", data: Data("<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p id=\"target\">Text</p></body></html>".utf8), compression: .deflate, to: archive)

    let result = try XHTMLAttributeArchiveTransformer.rewrite(
        source: source,
        to: destination,
        xhtmlPath: .init("OEBPS/chapter.xhtml"),
        selector: "#target",
        name: "data-epub-handbook",
        value: "normalized"
    )

    let rewritten = try EPUBArchiveReader(url: destination)
    let xhtml = String(decoding: try rewritten.data(for: .init("OEBPS/chapter.xhtml")), as: UTF8.self)
    #expect(result.changedElementCount == 1)
    #expect(xhtml.contains("data-epub-handbook=\"normalized\""))
}

private func addEntry(_ path: String, data: Data, compression: CompressionMethod, to archive: Archive) throws {
    try archive.addEntry(
        with: path,
        type: .file,
        uncompressedSize: Int64(data.count),
        compressionMethod: compression,
        provider: { position, size in
            let offset = Int(position)
            return data.subdata(in: offset..<(offset + size))
        }
    )
}
