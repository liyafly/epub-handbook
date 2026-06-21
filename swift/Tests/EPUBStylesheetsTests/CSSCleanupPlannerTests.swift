import EPUBArchive
import EPUBStylesheets
import Testing

@Test("planner factors three same-shape stylesheets and emits overrides")
func plannerFactorsSameShapeStylesheets() throws {
    let first = try ArchivePath("OEBPS/Styles/style0002.css")
    let second = try ArchivePath("OEBPS/Styles/style0003.css")
    let third = try ArchivePath("OEBPS/Styles/style0004.css")
    let inventory = StylesheetInventory(
        opfPath: try ArchivePath("OEBPS/content.opf"),
        stylesheets: [
            .init(path: first, css: "h1 { color: red; margin: 0; }"),
            .init(path: second, css: "h1 { color: green; margin: 0; }"),
            .init(path: third, css: "h1 { color: blue; margin: 0; }"),
        ],
        xhtmlPaths: [
            try ArchivePath("OEBPS/Text/one.xhtml"),
            try ArchivePath("OEBPS/Text/two.xhtml"),
            try ArchivePath("OEBPS/Text/three.xhtml"),
        ],
        references: [
            .init(xhtmlPath: try ArchivePath("OEBPS/Text/one.xhtml"), stylesheetPath: first),
            .init(xhtmlPath: try ArchivePath("OEBPS/Text/two.xhtml"), stylesheetPath: second),
            .init(xhtmlPath: try ArchivePath("OEBPS/Text/three.xhtml"), stylesheetPath: third),
        ]
    )

    let plan = try CSSCleanupPlanner.plan(inventory: inventory, options: .init())

    #expect(plan.generated.contains { $0.path.value.hasSuffix("clean-shared-01.css") })
    #expect(plan.linkReplacements[second]?.count == 2)
    #expect(plan.removedStylesheets == Set([first, second, third]))
    #expect(plan.factoredStylesheets == 3)
    #expect(plan.overridesCreated == 2)
}

@Test("planner scopes only disjoint local stylesheets")
func plannerMergesDisjointLocalStylesheets() throws {
    let localOne = try ArchivePath("OEBPS/Styles/one.css")
    let localTwo = try ArchivePath("OEBPS/Styles/two.css")
    let global = try ArchivePath("OEBPS/Styles/global.css")
    let firstPage = try ArchivePath("OEBPS/Text/one.xhtml")
    let secondPage = try ArchivePath("OEBPS/Text/two.xhtml")
    let thirdPage = try ArchivePath("OEBPS/Text/three.xhtml")
    let inventory = StylesheetInventory(
        opfPath: try ArchivePath("OEBPS/content.opf"),
        stylesheets: [
            .init(path: localOne, css: "h1 { color: red; }"),
            .init(path: localTwo, css: ".toc { margin: 0; }"),
            .init(path: global, css: "body { line-height: 1.5; }"),
        ],
        xhtmlPaths: [firstPage, secondPage, thirdPage],
        references: [
            .init(xhtmlPath: firstPage, stylesheetPath: localOne),
            .init(xhtmlPath: firstPage, stylesheetPath: global),
            .init(xhtmlPath: secondPage, stylesheetPath: localTwo),
            .init(xhtmlPath: secondPage, stylesheetPath: global),
            .init(xhtmlPath: thirdPage, stylesheetPath: global),
        ]
    )

    let plan = try CSSCleanupPlanner.plan(
        inventory: inventory,
        options: .init(mergeScopedLocalStylesheets: true)
    )

    #expect(plan.scopedLocalStylesheetsMerged == 2)
    #expect(plan.bodyClasses[firstPage] == ["css-local-01"])
    #expect(plan.bodyClasses[secondPage] == ["css-local-02"])
    #expect(plan.bodyClasses[thirdPage] == nil)
    #expect(plan.linkReplacements[localOne]?.last?.value.hasSuffix("clean-scoped-local.css") == true)
}

@Test("planner retains opaque at-rule stylesheets even when their normalized bytes match")
func plannerDoesNotDedupeOpaqueStylesheets() throws {
    let first = try ArchivePath("OEBPS/Styles/first.css")
    let second = try ArchivePath("OEBPS/Styles/second.css")
    let inventory = StylesheetInventory(
        opfPath: try ArchivePath("OEBPS/content.opf"),
        stylesheets: [
            .init(path: first, css: "@media screen { p { color: red; } }"),
            .init(path: second, css: "@media screen { p { color: red; } }"),
        ],
        xhtmlPaths: [],
        references: []
    )

    let plan = try CSSCleanupPlanner.plan(inventory: inventory, options: .init())

    #expect(plan.duplicateStylesheetsRemoved == 0)
    #expect(plan.removedStylesheets.isEmpty)
}
