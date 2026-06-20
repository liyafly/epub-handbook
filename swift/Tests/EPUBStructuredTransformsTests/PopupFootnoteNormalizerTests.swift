import Foundation
import EPUBArchive
import EPUBStructuredTransforms
import EPUBValidation
import Testing
import ZIPFoundation

@Test("native popup normalizer groups legacy local notes and preserves icon resources")
func popupNormalizerBuildsOneGroupedAside() throws {
    let source = """
    <html xmlns="http://www.w3.org/1999/xhtml"><body>
      <p>正文<a id="note-one" class="duokan-footnote" role="doc-noteref" href="#footnote-one"><img src="../Images/custom-note.png" alt="自定义图标"/></a>继续。</p>
      <p>第二处<a id="note-two" role="doc-noteref" href="#footnote-two"><img src="../Images/custom-note.png" alt="自定义图标"/></a></p>
      <aside role="doc-footnote"><p id="footnote-one">第一条注释。<a href="#note-one">◎</a></p></aside>
      <aside role="doc-footnote"><p id="footnote-two">第二条注释。</p></aside>
    </body></html>
    """

    let result = try PopupFootnoteNormalizer.normalize(in: source)

    #expect(result.normalizedReferenceCount == 2)
    #expect(result.xhtml.contains("xmlns:epub=\"http://www.idpf.org/2007/ops\""))
    #expect(result.xhtml.contains("class=\"noteref-icon\""))
    #expect(result.xhtml.contains("epub:type=\"noteref\""))
    #expect(result.xhtml.contains("src=\"../Images/custom-note.png\""))
    #expect(result.xhtml.contains("<aside epub:type=\"footnote\" role=\"doc-footnote\""))
    #expect(result.xhtml.contains("<ol class=\"footnote-list\""))
    #expect(result.xhtml.contains("id=\"footnote-one\""))
    #expect(result.xhtml.contains("class=\"footnote-item\""))
    #expect(result.xhtml.contains("epub:type=\"backlink\""))
    #expect(result.xhtml.contains("第一条注释。"))
    #expect(result.xhtml.contains("第二条注释。"))
    #expect(!result.xhtml.contains("duokan-footnote"))
}

@Test("native popup normalizer refuses a text marker without an existing image icon")
func popupNormalizerRefusesTextOnlyMarker() throws {
    let source = """
    <html xmlns="http://www.w3.org/1999/xhtml"><body>
      <p>正文<a id="note-one" role="doc-noteref" href="#footnote-one">[1]</a></p>
      <aside role="doc-footnote"><p id="footnote-one">注释。</p></aside>
    </body></html>
    """

    #expect(throws: PopupFootnoteNormalizeError.referenceRequiresExactlyOneImage("note-one")) {
        try PopupFootnoteNormalizer.normalize(in: source)
    }
}

@Test("native popup normalizer merges a complete Sigil footnotes section without retaining old controls")
func popupNormalizerMergesSigilLegacySection() throws {
    let source = """
    <html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>
      <p>正文<sup><a id="noteref_1" epub:type="noteref" href="#footnote_1"><img src="../Images/note.png" alt="注"/></a></sup>继续<sup><a id="noteref_2" epub:type="noteref" href="#footnote_2"><img src="../Images/note.png" alt="注"/></a></sup>。</p>
      <section epub:type="footnotes">
        <aside id="footnote_1" epub:type="footnote"><p><a href="#noteref_1" epub:type="noteref">[1]</a>第一条注释正文。</p></aside>
        <aside id="footnote_2" epub:type="footnote"><p><a href="#noteref_2" epub:type="noteref">[2]</a>第二条注释正文。</p></aside>
      </section>
    </body></html>
    """

    let result = try PopupFootnoteNormalizer.normalize(in: source)

    #expect(result.normalizedReferenceCount == 2)
    #expect(result.xhtml.contains("id=\"footnote_1\""))
    #expect(result.xhtml.contains("id=\"footnote_2\""))
    #expect(result.xhtml.contains("class=\"footnote-item\""))
    #expect(result.xhtml.contains("第一条注释正文。"))
    #expect(result.xhtml.contains("第二条注释正文。"))
    #expect(!result.xhtml.contains(">[1]</a>"))
    #expect(!result.xhtml.contains("<section epub:type=\"footnotes\""))
}

@Test("native popup normalizer rejects a partially recognized Sigil footnotes section")
func popupNormalizerRejectsPartialSigilSection() throws {
    let source = """
    <html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>
      <p>正文<a id="noteref_1" epub:type="noteref" href="#footnote_1"><img src="../Images/note.png" alt="注"/></a></p>
      <section epub:type="footnotes"><aside id="footnote_1" epub:type="footnote"><p>注释正文。</p></aside><p>未识别内容</p></section>
    </body></html>
    """

    #expect(throws: PopupFootnoteNormalizeError.unrecognizedFootnoteSection) {
        try PopupFootnoteNormalizer.normalize(in: source)
    }
}

@Test("native popup archive normalizer writes a new EPUB and requires a manifest-backed icon")
func popupArchiveNormalizerWritesVerifiedArtifact() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let source = directory.appending(path: "source.epub")
    let destination = directory.appending(path: "normalized.epub")
    let archive = try Archive(url: source, accessMode: .create)
    try add("mimetype", "application/epub+zip", .none, archive)
    try add("META-INF/container.xml", "<container><rootfiles><rootfile full-path=\"OEBPS/package.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>", .deflate, archive)
    try add("OEBPS/package.opf", """
    <package xmlns="http://www.idpf.org/2007/opf"><metadata/><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="icon" href="Images/custom.png" media-type="image/png"/></manifest><spine><itemref idref="chapter"/></spine></package>
    """, .deflate, archive)
    let sourceXHTML = """
    <html xmlns="http://www.w3.org/1999/xhtml"><body><p>正文<a id="note-one" role="doc-noteref" href="#footnote-one"><img src="../Images/custom.png" alt="注"/></a></p><aside role="doc-footnote"><p id="footnote-one">注释正文。</p></aside></body></html>
    """
    try add("OEBPS/Text/chapter.xhtml", sourceXHTML, .deflate, archive)
    try addData("OEBPS/Images/custom.png", Data([0x89, 0x50]), .deflate, archive)

    let result = try PopupFootnoteArchiveNormalizer.normalize(source: source, to: destination)

    let before = try EPUBArchiveReader(url: source)
    let after = try EPUBArchiveReader(url: destination)
    let normalized = String(decoding: try after.data(for: .init("OEBPS/Text/chapter.xhtml")), as: UTF8.self)
    #expect(result.normalizedReferenceCount == 1)
    #expect(result.normalizedXHTMLPaths == [try ArchivePath("OEBPS/Text/chapter.xhtml")])
    #expect(String(decoding: try before.data(for: .init("OEBPS/Text/chapter.xhtml")), as: UTF8.self) == sourceXHTML)
    #expect(normalized.contains("epub:type=\"footnote\""))
    #expect(normalized.contains("src=\"../Images/custom.png\""))
}

@Test("native popup archive normalizer supplies an OPF-backed icon for Sigil text markers")
func popupArchiveNormalizerSuppliesDefaultIconForSigilTextMarkers() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let source = directory.appending(path: "source.epub")
    let destination = directory.appending(path: "normalized.epub")
    let archive = try Archive(url: source, accessMode: .create)
    try add("mimetype", "application/epub+zip", .none, archive)
    try add("META-INF/container.xml", "<container><rootfiles><rootfile full-path=\"OEBPS/package.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>", .deflate, archive)
    try add("OEBPS/package.opf", """
    <package xmlns="http://www.idpf.org/2007/opf"><metadata><dc:language xmlns:dc="http://purl.org/dc/elements/1.1/">zh-CN</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>
    """, .deflate, archive)
    let sourceXHTML = """
    <html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><p>正文<sup><a id="noteref_1" epub:type="noteref" href="#footnote_1">[1]</a></sup>继续。</p><section epub:type="footnotes"><aside id="footnote_1" epub:type="footnote"><p><a href="#noteref_1" epub:type="noteref">[1]</a>注释正文。</p></aside></section></body></html>
    """
    try add("OEBPS/Text/chapter.xhtml", sourceXHTML, .deflate, archive)

    let result = try PopupFootnoteArchiveNormalizer.normalize(source: source, to: destination)

    let after = try EPUBArchiveReader(url: destination)
    let normalized = String(decoding: try after.data(for: .init("OEBPS/Text/chapter.xhtml")), as: UTF8.self)
    let opf = String(decoding: try after.data(for: .init("OEBPS/package.opf")), as: UTF8.self)
    #expect(result.normalizedReferenceCount == 1)
    #expect(result.defaultIconAdded)
    #expect(after.entryPaths().contains(try ArchivePath("OEBPS/Images/note.png")))
    #expect(normalized.contains("src=\"../Images/note.png\""))
    #expect(normalized.contains("lang=\"zh-CN\""))
    #expect(normalized.contains("xml:lang=\"zh-CN\""))
    #expect(opf.contains("href=\"Images/note.png\""))
    #expect(opf.contains("media-type=\"image/png\""))
    #expect(try PopupFootnoteValidator.validate(epub: destination).isValid)
    #expect(try TextInvarianceValidator.validate(before: source, after: destination).isValid)
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
