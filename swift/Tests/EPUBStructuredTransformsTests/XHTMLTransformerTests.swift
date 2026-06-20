import EPUBStructuredTransforms
import Testing

@Test("SwiftSoup transforms XML-mode EPUB XHTML without requiring an XML declaration")
func xhtmlTransformerChangesAllMatchedAttributes() throws {
    let source = """
    <!DOCTYPE html>
    <html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
      <body><a epub:type="noteref" href="#note-1">1</a></body>
    </html>
    """

    let result = try XHTMLTransformer.setAttribute(
        in: source,
        selector: "a",
        name: "data-epub-handbook",
        value: "normalized"
    )

    #expect(result.changedElementCount == 1)
    #expect(result.xhtml.contains("data-epub-handbook=\"normalized\""))
    #expect(result.xhtml.contains("xmlns:epub=\"http://www.idpf.org/2007/ops\""))
    #expect(result.xhtml.contains("<!DOCTYPE html>"))
}
