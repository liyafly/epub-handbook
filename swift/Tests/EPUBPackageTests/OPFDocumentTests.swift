import Foundation
import EPUBPackage
import Testing

@Test("OPF parser reads manifest, navigation item, and spine in document order")
func opfDocumentReadsPackageFacts() throws {
    let opf = Data(
        """
        <?xml version="1.0" encoding="UTF-8"?>
        <package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
          <manifest>
            <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav" />
            <item id="chapter-1" href="text/chapter-1.xhtml" media-type="application/xhtml+xml" />
          </manifest>
          <spine><itemref idref="chapter-1" /></spine>
        </package>
        """.utf8
    )

    let snapshot = try OPFDocument.parse(opf)

    #expect(snapshot.uniqueIdentifier == "book-id")
    #expect(snapshot.manifest.map(\.id) == ["nav", "chapter-1"])
    #expect(snapshot.navigationItem?.href == "nav.xhtml")
    #expect(snapshot.spineItemIDs == ["chapter-1"])
}
