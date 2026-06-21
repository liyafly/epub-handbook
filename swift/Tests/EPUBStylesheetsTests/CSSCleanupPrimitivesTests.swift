import EPUBStylesheets
import Testing

@Test("sanitizer rewrites only approved legacy font families")
func sanitizerRewritesApprovedFonts() throws {
    let output = try CSSSanitizer.sanitize("p { font-family: \"SimHei\"; } .x { font-family: Other; }")

    #expect(output.fontRewrites == 1)
    #expect(output.css.contains("Noto Sans CJK SC"))
    #expect(output.css.contains("font-family: Other"))
}

@Test("sanitizer removes title ornament and repairs missing declaration terminator")
func sanitizerRepairsLegacyFormatting() throws {
    let output = try CSSSanitizer.sanitize("""
    ————标题————
    p {
      color: red
      font-family: \"SimSun\";
    }
    """)

    #expect(!output.css.contains("标题"))
    #expect(output.css.contains("color: red;"))
    #expect(output.css.contains("Songti SC"))
}

@Test("stylesheet parser returns a shape only for supported qualified rules")
func parserRejectsAtRulesAndPreservesDeclarationShape() throws {
    let rules = try CSSRuleParser.parse("h1, h2 { color: red; margin: 0; } p { color: blue; }")

    #expect(rules?.count == 2)
    #expect(rules?.shape == [
        CSSRuleShape(selector: "h1, h2", properties: ["color", "margin"]),
        CSSRuleShape(selector: "p", properties: ["color"]),
    ])
    #expect(try CSSRuleParser.parse("@media screen { p { color: red; } }") == nil)
}

@Test("CSS fingerprint ignores comments whitespace and case")
func fingerprintMatchesPythonNormalization() throws {
    let first = CSSFingerprint.make("/* note */ P { COLOR: red; }")
    let second = CSSFingerprint.make("p{color:red;}")

    #expect(first == second)
}
