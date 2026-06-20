import Foundation
import EPUBArchive
import Testing
import ZIPFoundation

@Test("archive rewriter preserves unmodified entries and writes replacements to a new EPUB")
func archiveRewriterWritesNewArtifact() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let source = directory.appending(path: "source.epub")
    let destination = directory.appending(path: "output.epub")
    let archive = try Archive(url: source, accessMode: .create)
    try addEntry("mimetype", data: Data("application/epub+zip".utf8), compression: .none, to: archive)
    try addEntry("OEBPS/chapter.xhtml", data: Data("<p>before</p>".utf8), compression: .deflate, to: archive)
    try addEntry("OEBPS/cover.png", data: Data([0x89, 0x50, 0x4E, 0x47]), compression: .deflate, to: archive)

    try EPUBArchiveRewriter.rewrite(
        source: source,
        to: destination,
        replacements: [try ArchivePath("OEBPS/chapter.xhtml"): Data("<p>after</p>".utf8)]
    )

    let rewritten = try EPUBArchiveReader(url: destination)
    #expect(try rewritten.data(for: .init("OEBPS/chapter.xhtml")) == Data("<p>after</p>".utf8))
    #expect(try rewritten.data(for: .init("OEBPS/cover.png")) == Data([0x89, 0x50, 0x4E, 0x47]))
    let outputArchive = try Archive(url: destination, accessMode: .read)
    let first = outputArchive.makeIterator().next()
    #expect(first?.path == "mimetype")
    #expect(first?.compressedSize == first?.uncompressedSize)
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
