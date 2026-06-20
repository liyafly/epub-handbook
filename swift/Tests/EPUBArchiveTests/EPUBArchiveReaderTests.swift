import Foundation
import EPUBArchive
import Testing
import ZIPFoundation

@Test("archive reader reads EPUB entries through validated archive paths")
func archiveReaderReadsValidatedEntry() throws {
    let url = FileManager.default.temporaryDirectory.appending(path: "archive-reader-test.epub")
    defer { try? FileManager.default.removeItem(at: url) }

    let data = Data("application/epub+zip".utf8)
    let archive = try Archive(url: url, accessMode: .create)
    try archive.addEntry(
        with: "mimetype",
        type: .file,
        uncompressedSize: Int64(data.count),
        compressionMethod: .none,
        provider: { position, size in
            let offset = Int(position)
            return data.subdata(in: offset..<(offset + size))
        }
    )

    let reader = try EPUBArchiveReader(url: url)
    let path = try ArchivePath("mimetype")

    #expect(reader.entryPaths() == [path])
    #expect(try reader.data(for: path) == data)
}
