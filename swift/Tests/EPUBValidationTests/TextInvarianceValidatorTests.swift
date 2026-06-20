import Foundation
import EPUBValidation
import Testing
import ZIPFoundation

@Test("text invariance accepts XHTML structure changes that preserve leaf block text and anchors")
func textInvarianceAcceptsEquivalentBlockText() throws {
    let before = try makeArchive([
        "OEBPS/Text/chapter.xhtml": xhtml("<p id=\"one\">Hello <em>world</em>.</p>"),
    ])
    let after = try makeArchive([
        "OEBPS/Text/chapter.xhtml": xhtml("<div id=\"one\">Hello <strong>world</strong>.</div>"),
    ])
    defer {
        try? FileManager.default.removeItem(at: before)
        try? FileManager.default.removeItem(at: after)
    }

    let report = try TextInvarianceValidator.validate(before: before, after: after)

    #expect(report.isValid)
    #expect(report.issues.isEmpty)
}

@Test("text invariance reports changed leaf block text")
func textInvarianceReportsChangedText() throws {
    let before = try makeArchive(["chapter.xhtml": xhtml("<p>Before.</p>")])
    let after = try makeArchive(["chapter.xhtml": xhtml("<p>After.</p>")])
    defer {
        try? FileManager.default.removeItem(at: before)
        try? FileManager.default.removeItem(at: after)
    }

    let report = try TextInvarianceValidator.validate(before: before, after: after)

    #expect(!report.isValid)
    #expect(report.issues.contains { $0.kind == .textModified && $0.path == "chapter.xhtml" })
}

@Test("text invariance reports removed anchors independently from text")
func textInvarianceReportsRemovedAnchors() throws {
    let before = try makeArchive(["chapter.xhtml": xhtml("<p id=\"note-1\">Same text.</p>")])
    let after = try makeArchive(["chapter.xhtml": xhtml("<p>Same text.</p>")])
    defer {
        try? FileManager.default.removeItem(at: before)
        try? FileManager.default.removeItem(at: after)
    }

    let report = try TextInvarianceValidator.validate(before: before, after: after)

    #expect(!report.isValid)
    #expect(report.issues.contains { $0.kind == .anchorsRemoved && $0.path == "chapter.xhtml" })
}

@Test("text invariance ignores popup control labels but still compares footnote body text")
func textInvarianceTreatsNoteControlsAsNonProse() throws {
    let before = try makeArchive([
        "chapter.xhtml": xhtml("<p>正文<a id=\"noteref-1\" epub:type=\"noteref\" href=\"#footnote-1\">[1]</a></p><aside epub:type=\"footnote\"><ol><li id=\"footnote-1\"><p><a href=\"#noteref-1\" epub:type=\"noteref\">[1]</a>注释正文。</p></li></ol></aside>"),
    ])
    let after = try makeArchive([
        "chapter.xhtml": xhtml("<p>正文<a id=\"noteref-1\" epub:type=\"noteref\" role=\"doc-noteref\" href=\"#footnote-1\"><img alt=\"注\" src=\"note.png\"/></a></p><aside epub:type=\"footnote\" role=\"doc-footnote\"><ol class=\"footnote-list\"><li id=\"footnote-1\" class=\"footnote-item\"><p class=\"footnote\"><a epub:type=\"backlink\" role=\"doc-backlink\" href=\"#noteref-1\">◎</a>注释正文。</p></li></ol></aside>"),
    ])
    defer {
        try? FileManager.default.removeItem(at: before)
        try? FileManager.default.removeItem(at: after)
    }

    #expect(try TextInvarianceValidator.validate(before: before, after: after).isValid)
}

@Test("text invariance still reports popup footnote body text changes")
func textInvarianceReportsPopupBodyChange() throws {
    let before = try makeArchive([
        "chapter.xhtml": xhtml("<p>正文<a id=\"noteref-1\" epub:type=\"noteref\" href=\"#footnote-1\">[1]</a></p><aside epub:type=\"footnote\"><ol><li id=\"footnote-1\"><p><a href=\"#noteref-1\" epub:type=\"noteref\">[1]</a>注释正文。</p></li></ol></aside>"),
    ])
    let after = try makeArchive([
        "chapter.xhtml": xhtml("<p>正文<a id=\"noteref-1\" epub:type=\"noteref\" role=\"doc-noteref\" href=\"#footnote-1\"><img alt=\"注\" src=\"note.png\"/></a></p><aside epub:type=\"footnote\" role=\"doc-footnote\"><ol class=\"footnote-list\"><li id=\"footnote-1\" class=\"footnote-item\"><p class=\"footnote\"><a epub:type=\"backlink\" role=\"doc-backlink\" href=\"#noteref-1\">◎</a>注释正文改。</p></li></ol></aside>"),
    ])
    defer {
        try? FileManager.default.removeItem(at: before)
        try? FileManager.default.removeItem(at: after)
    }

    let report = try TextInvarianceValidator.validate(before: before, after: after)
    #expect(report.issues.contains { $0.kind == .textModified })
}

private func makeArchive(_ entries: [String: String]) throws -> URL {
    let url = FileManager.default.temporaryDirectory.appending(path: "text-invariance-\(UUID().uuidString).epub")
    let archive = try Archive(url: url, accessMode: .create)
    for (path, contents) in entries {
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
    return url
}

private func xhtml(_ body: String) -> String {
    """
    <!DOCTYPE html>
    <html xmlns="http://www.w3.org/1999/xhtml"><body>\(body)</body></html>
    """
}
