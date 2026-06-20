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
