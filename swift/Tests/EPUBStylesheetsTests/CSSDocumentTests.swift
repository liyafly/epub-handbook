import EPUBStylesheets
import Testing

@Test("CSS scanner preserves comments strings functions and nested at-rules")
func scannerPreservesOpaqueCSS() throws {
    let source = "/* keep */ a { content: \";}\"; background: url(data:x;y); } @media screen { b { color: red; } }"

    let document = try CSSDocument.parse(source)

    #expect(document.statements.count == 3)
    #expect(document.serialized == source)
}
