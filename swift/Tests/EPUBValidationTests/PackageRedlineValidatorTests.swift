import Foundation
import EPUBValidation
import Testing
import ZIPFoundation

@Test("package redline reports metadata spine cover and DRM changes")
func packageRedlineReportsProtectedPackageChanges() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let before = try makeEPUB(at: directory.appending(path: "before.epub"), title: "Before", spine: ["chapter"], cover: Data([1, 2]), encryption: nil)
    let after = try makeEPUB(at: directory.appending(path: "after.epub"), title: "After", spine: [], cover: Data([3, 4]), encryption: "<encryption />")

    let report = try PackageRedlineValidator.validate(before: before, after: after)

    #expect(report.issues.map(\.kind).contains(.metadataChanged))
    #expect(report.issues.map(\.kind).contains(.spineChanged))
    #expect(report.issues.map(\.kind).contains(.coverChanged))
    #expect(report.issues.map(\.kind).contains(.drmDetected))
}

@Test("package redline permits only the documented encryption exceptions")
func packageRedlineAppliesEncryptionExceptionPolicy() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }

    let stale = try makeEPUB(
        at: directory.appending(path: "stale.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1]),
        encryption: encryptionXML(algorithm: "http://www.w3.org/2001/04/xmlenc#aes128-ctr", uri: "OEBPS/Styles/missing.css")
    )
    let plain = try makeEPUB(
        at: directory.appending(path: "plain.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1]),
        encryption: nil
    )
    #expect(try PackageRedlineValidator.validate(before: stale, after: plain).isValid)

    let fontProtected = try makeEPUB(
        at: directory.appending(path: "font.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1]),
        encryption: encryptionXML(algorithm: "http://www.idpf.org/2008/embedding", uri: "OEBPS/Fonts/book.ttf"),
        includesFont: true
    )
    let strict = try PackageRedlineValidator.validate(before: fontProtected, after: fontProtected)
    #expect(strict.issues.map(\.kind).contains(.drmDetected))
    let permitted = try PackageRedlineValidator.validate(
        before: fontProtected,
        after: fontProtected,
        options: .init(allowStandardFontObfuscation: true)
    )
    #expect(permitted.isValid)

    let protectedContent = try makeEPUB(
        at: directory.appending(path: "protected-content.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1]),
        encryption: encryptionXML(algorithm: "http://www.idpf.org/2008/embedding", uri: "OEBPS/cover.png")
    )
    let rejected = try PackageRedlineValidator.validate(
        before: protectedContent,
        after: protectedContent,
        options: .init(allowStandardFontObfuscation: true)
    )
    #expect(rejected.issues.map(\.kind).contains(.drmDetected))
}

@Test("package redline permits only an explicit approved cover path map")
func packageRedlineUsesCoverPathMap() throws {
    let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }
    let before = try makeEPUB(
        at: directory.appending(path: "before.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1, 2]),
        encryption: nil
    )
    let after = try makeEPUB(
        at: directory.appending(path: "after.epub"),
        title: "Book",
        spine: ["chapter"],
        cover: Data([1, 2]),
        encryption: nil,
        coverHref: "cover-renamed.png"
    )

    let unmapped = try PackageRedlineValidator.validate(before: before, after: after)
    #expect(unmapped.issues.map(\.kind).contains(.coverChanged))
    let mapped = try PackageRedlineValidator.validate(
        before: before,
        after: after,
        options: .init(pathMap: ["OEBPS/cover.png": "OEBPS/cover-renamed.png"])
    )
    #expect(mapped.isValid)
}

private func makeEPUB(
    at url: URL,
    title: String,
    spine: [String],
    cover: Data,
    encryption: String?,
    includesFont: Bool = false,
    coverHref: String = "cover.png"
) throws -> URL {
    let archive = try Archive(url: url, accessMode: .create)
    try add("mimetype", "application/epub+zip", .none, archive)
    try add("META-INF/container.xml", "<container><rootfiles><rootfile full-path=\"OEBPS/package.opf\" media-type=\"application/oebps-package+xml\" /></rootfiles></container>", .deflate, archive)
    let items = spine.map { "<itemref idref=\"\($0)\" />" }.joined()
    let fontManifest = includesFont ? "<item id=\"font\" href=\"Fonts/book.ttf\" media-type=\"font/ttf\"/>" : ""
    try add("OEBPS/package.opf", """
    <package xmlns=\"http://www.idpf.org/2007/opf\"><metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\"><dc:title>\(title)</dc:title><dc:creator>A</dc:creator><dc:identifier>I</dc:identifier><dc:language>en</dc:language></metadata><manifest><item id=\"chapter\" href=\"chapter.xhtml\" media-type=\"application/xhtml+xml\"/><item id=\"cover\" href=\"\(coverHref)\" media-type=\"image/png\" properties=\"cover-image\"/>\(fontManifest)</manifest><spine>\(items)</spine></package>
    """, .deflate, archive)
    try addData("OEBPS/\(coverHref)", cover, .deflate, archive)
    if includesFont { try addData("OEBPS/Fonts/book.ttf", Data([0, 1, 2]), .deflate, archive) }
    if let encryption { try add("META-INF/encryption.xml", encryption, .deflate, archive) }
    return url
}

private func encryptionXML(algorithm: String, uri: String) -> String {
    """
    <encryption xmlns:enc=\"http://www.w3.org/2001/04/xmlenc#\"><enc:EncryptedData><enc:EncryptionMethod Algorithm=\"\(algorithm)\"/><enc:CipherData><enc:CipherReference URI=\"\(uri)\"/></enc:CipherData></enc:EncryptedData></encryption>
    """
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
