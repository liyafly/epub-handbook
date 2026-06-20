import EPUBArchive
import Testing

@Test("archive paths reject absolute and traversal paths")
func archivePathsRejectAbsoluteAndTraversalPaths() throws {
    let path = try ArchivePath("OEBPS/Text/01.xhtml")

    #expect(path.value == "OEBPS/Text/01.xhtml")
    #expect(throws: ArchivePathError.self) {
        try ArchivePath("../outside.xhtml")
    }
    #expect(throws: ArchivePathError.self) {
        try ArchivePath("/absolute.xhtml")
    }
}
