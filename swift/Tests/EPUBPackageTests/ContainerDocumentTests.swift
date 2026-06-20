import Foundation
import EPUBPackage
import Testing

@Test("container document resolves the OPF rootfile path")
func containerDocumentResolvesOPFRootfilePath() throws {
    let container = Data(
        """
        <?xml version="1.0" encoding="UTF-8"?>
        <container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
          <rootfiles>
            <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml" />
          </rootfiles>
        </container>
        """.utf8
    )

    let path = try ContainerDocument.opfPath(from: container)

    #expect(path.value == "OEBPS/package.opf")
}

@Test("container document rejects a traversal OPF rootfile path")
func containerDocumentRejectsTraversalOPFRootfilePath() {
    let container = Data(
        """
        <container><rootfiles><rootfile media-type="application/oebps-package+xml" full-path="../package.opf" /></rootfiles></container>
        """.utf8
    )

    #expect(throws: PackageParseError.unsafeOPFPath) {
        try ContainerDocument.opfPath(from: container)
    }
}
