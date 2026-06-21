import EPUBStylesheets
import EPUBValidation
import Foundation
import Testing
import ZIPFoundation

@Test("CSS archive cleanup writes replacements additions removals and valid manifest links")
func archiveCleanupWritesNewValidEPUB() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let source = directory.appending(path: "source.epub")
    let output = directory.appending(path: "cleaned.epub")
    try writeCSSFixture(to: source)

    let report = try CSSCleanupArchiveTransformer.transform(source: source, to: output, options: .init())

    #expect(report.factoredStylesheets == 3)
    #expect(report.cssManifestItemsRemoved == 3)
    #expect(report.cssManifestItemsAdded == 3)
    #expect(try CSSCleanupValidator.validate(epub: output).isValid)
    #expect(try TextInvarianceValidator.validate(before: source, after: output).isValid)
}

private func writeCSSFixture(to path: URL) throws {
    let archive = try Archive(url: path, accessMode: .create)
    try addCSSFixtureEntry("mimetype", data: "application/epub+zip", compression: .none, to: archive)
    try addCSSFixtureEntry("META-INF/container.xml", data: """
    <?xml version="1.0"?>
    <container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>
    """, compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/content.opf", data: """
    <?xml version="1.0" encoding="UTF-8"?>
    <package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:css-fixture</dc:identifier><dc:title>Fixture</dc:title><dc:language>en</dc:language></metadata><manifest><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="three" href="Text/three.xhtml" media-type="application/xhtml+xml"/><item id="s1" href="Styles/style0002.css" media-type="text/css"/><item id="s2" href="Styles/style0003.css" media-type="text/css"/><item id="s3" href="Styles/style0004.css" media-type="text/css"/></manifest><spine><itemref idref="one"/><itemref idref="two"/><itemref idref="three"/></spine></package>
    """, compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Text/one.xhtml", data: fixtureChapter("style0002.css", text: "One"), compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Text/two.xhtml", data: fixtureChapter("style0003.css", text: "Two"), compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Text/three.xhtml", data: fixtureChapter("style0004.css", text: "Three"), compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Styles/style0002.css", data: "h1 { color: red; margin: 0; }", compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Styles/style0003.css", data: "h1 { color: green; margin: 0; }", compression: .deflate, to: archive)
    try addCSSFixtureEntry("OEBPS/Styles/style0004.css", data: "h1 { color: blue; margin: 0; }", compression: .deflate, to: archive)
}

private func fixtureChapter(_ stylesheet: String, text: String) -> String {
    """
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE html>
    <html xmlns="http://www.w3.org/1999/xhtml"><head><title>Fixture</title><link href="../Styles/\(stylesheet)" type="text/css" rel="stylesheet"/></head><body><h1 id="heading">\(text)</h1><p>Body \(text).</p></body></html>
    """
}

private func addCSSFixtureEntry(_ path: String, data: String, compression: CompressionMethod, to archive: Archive) throws {
    let bytes = Data(data.utf8)
    try archive.addEntry(
        with: path,
        type: .file,
        uncompressedSize: Int64(bytes.count),
        compressionMethod: compression,
        provider: { position, size in
            let offset = Int(position)
            return bytes.subdata(in: offset..<(offset + size))
        }
    )
}
