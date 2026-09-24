package literarystructure

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

const (
	testXHTMLPath = "OEBPS/Text/chapter.xhtml"
	testCSSPath   = "OEBPS/Styles/literary.css"
)

func TestClassVocabularyIsExactAndDocumentBound(t *testing.T) {
	want := []string{
		"dialog", "poetry", "letter", "scene-break", "chapter-head", "chapter-head-art",
		"chapter-head-banner", "chapter-header", "epigraph", "copyright-page", "dedication",
		"epigraph-page", "english-fiction", "classical-modern", "parallel-entry", "parallel-pair",
		"parallel-float-pair", "parallel-stack-pair", "parallel-entry-title", "parallel-source",
		"classical-text", "modern-text", "parallel-return", "parallel-ratio-balanced",
		"parallel-ratio-source-wide", "parallel-clear",
	}
	vocabulary := classVocabulary()
	if len(vocabulary) != 26 {
		t.Fatalf("vocabulary has %d entries, want 26", len(vocabulary))
	}
	for _, class := range want {
		if !vocabulary[class] {
			t.Errorf("vocabulary is missing %q", class)
		}
	}
	if vocabulary["frontmatter"] {
		t.Fatal("frontmatter is not in the documented class vocabulary")
	}
}

func TestRunAddsClassByIDAndIsIdempotent(t *testing.T) {
	input := literaryFixture(t, literaryXHTML(`<blockquote id="epigraph" class="x">Quote</blockquote>`), true)
	b := openLiteraryFixture(t, input)
	assignment := Assignment{Path: testXHTMLPath, ID: "epigraph", Class: "epigraph"}

	first, err := Run(t.Context(), b, Params{Assignments: []Assignment{assignment}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Facts["editCount"] != 1 || !strings.Contains(string(literaryCurrent(t, b)), `class="x epigraph"`) {
		t.Fatalf("first result facts=%#v XHTML=%s", first.Facts, literaryCurrent(t, b))
	}
	second, err := Run(t.Context(), b, Params{Assignments: []Assignment{assignment}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Facts["editCount"] != 0 || !findingHasID(second.Findings, "literary.already-has-class") {
		t.Fatalf("second result facts=%#v findings=%+v, want idempotent no-op", second.Facts, second.Findings)
	}
}

func TestRunFindsTagIndexInDocumentOrder(t *testing.T) {
	chapter := literaryXHTML(`<p>first</p><p>second</p><p>third</p>`)
	b := openLiteraryFixture(t, literaryFixture(t, chapter, true))
	index := 2
	result, err := Run(t.Context(), b, Params{Assignments: []Assignment{{
		Path: testXHTMLPath, Tag: "p", Index: &index, Class: "dialog",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(literaryCurrent(t, b))
	if result.Facts["editCount"] != 1 || !strings.Contains(got, `<p class="dialog">third</p>`) || strings.Contains(got, `<p class="dialog">first`) {
		t.Fatalf("facts=%#v XHTML=%s, want only the third p modified", result.Facts, got)
	}
}

func TestRunCombinesClassesForOneElementInAssignmentOrder(t *testing.T) {
	chapter := literaryXHTML(`<p id="target" class="x">text</p>`)
	b := openLiteraryFixture(t, literaryFixture(t, chapter, true))
	result, err := Run(t.Context(), b, Params{Assignments: []Assignment{
		{Path: testXHTMLPath, ID: "target", Class: "dialog"},
		{Path: testXHTMLPath, ID: "target", Class: "letter"},
		{Path: testXHTMLPath, ID: "target", Class: "dialog"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(literaryCurrent(t, b)); !strings.Contains(got, `class="x dialog letter"`) {
		t.Fatalf("combined classes were not appended in order: %s", got)
	}
	if result.Facts["editCount"] != 1 || result.Facts["assignmentsTotal"] != 3 {
		t.Fatalf("facts=%#v, want one class insertion from three assignments", result.Facts)
	}
	planned := result.Facts["plannedEdits"].([]plannedEdit)
	if len(planned) != 1 || planned[0].Value != "dialog letter" {
		t.Fatalf("plannedEdits=%#v, want deduplicated assignment order", planned)
	}
}

func TestRunRejectsClassOutsideVocabularyAtomically(t *testing.T) {
	b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p id="good">first</p><p id="bad">second</p>`), true))
	before := literaryCurrent(t, b)
	result, err := Run(t.Context(), b, Params{Assignments: []Assignment{
		{Path: testXHTMLPath, ID: "good", Class: "epigraph"},
		{Path: testXHTMLPath, ID: "bad", Class: "my-fancy"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.class-not-allowed") {
		t.Fatalf("result=%+v, want class-not-allowed with zero edits", result)
	}
	if !bytes.Equal(before, literaryCurrent(t, b)) {
		t.Fatal("invalid class changed the XHTML")
	}
	if len(result.Facts["plannedEdits"].([]plannedEdit)) != 0 {
		t.Fatalf("plannedEdits=%#v, want empty on any error", result.Facts["plannedEdits"])
	}
}

func TestRunReportsPathsAndTargetsOutsideRequestedScope(t *testing.T) {
	t.Run("path is not in spine", func(t *testing.T) {
		b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p id="target">text</p>`), true))
		result, err := Run(t.Context(), b, Params{Assignments: []Assignment{
			{Path: "OEBPS/Text/appendix.xhtml", ID: "target", Class: "epigraph"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.path-not-in-spine") {
			t.Fatalf("result=%+v, want path-not-in-spine and zero edits", result)
		}
	})

	t.Run("tag index is out of range", func(t *testing.T) {
		b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p>only paragraph</p>`), true))
		index := 2
		result, err := Run(t.Context(), b, Params{Assignments: []Assignment{{
			Path: testXHTMLPath, Tag: "p", Index: &index, Class: "epigraph",
		}}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.target-not-found") {
			t.Fatalf("result=%+v, want target-not-found and zero edits", result)
		}
	})
}

func TestRunRejectsDuplicateIDAsAmbiguous(t *testing.T) {
	chapter := literaryXHTML(`<p id="dup">one</p><p id="dup">two</p>`)
	b := openLiteraryFixture(t, literaryFixture(t, chapter, true))
	result, err := Run(t.Context(), b, Params{Assignments: []Assignment{
		{Path: testXHTMLPath, ID: "dup", Class: "epigraph"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.target-ambiguous") {
		t.Fatalf("result=%+v, want ambiguous target with zero edits", result)
	}
}

func TestRunLinksStylesheetOnlyWhenMissing(t *testing.T) {
	t.Run("already linked by resolved relative URI", func(t *testing.T) {
		chapter := literaryXHTML(`<link rel="stylesheet" href="../Styles/literary.css?theme=book#base"/><p id="target">text</p>`)
		b := openLiteraryFixture(t, literaryFixture(t, chapter, true))
		result, err := Run(t.Context(), b, Params{
			Assignments: []Assignment{{Path: testXHTMLPath, ID: "target", Class: "epigraph"}},
			Stylesheet:  testCSSPath,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := string(literaryCurrent(t, b))
		if result.Facts["editCount"] != 1 || !strings.Contains(got, `href="../Styles/literary.css?theme=book#base"`) {
			t.Fatalf("facts=%#v XHTML=%s, want class only", result.Facts, got)
		}
		if !findingHasID(result.Findings, "literary.stylesheet-already-linked") {
			t.Fatalf("findings=%+v, want stylesheet-already-linked info", result.Findings)
		}
	})

	t.Run("adds relative link before closing head", func(t *testing.T) {
		b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p id="target">text</p>`), true))
		result, err := Run(t.Context(), b, Params{
			Assignments: []Assignment{{Path: testXHTMLPath, ID: "target", Class: "epigraph"}},
			Stylesheet:  testCSSPath,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := string(literaryCurrent(t, b))
		if result.Facts["editCount"] != 2 || !strings.Contains(got, `<link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>`) {
			t.Fatalf("facts=%#v XHTML=%s, want class and relative stylesheet link", result.Facts, got)
		}
		if strings.Index(got, `href="../Styles/literary.css"`) > strings.Index(got, `</head>`) {
			t.Fatalf("stylesheet link was not inserted before </head>: %s", got)
		}
		planned := result.Facts["plannedEdits"].([]plannedEdit)
		if len(planned) != 2 || planned[1].Action != "add-link" || planned[1].Value != "../Styles/literary.css" {
			t.Fatalf("plannedEdits=%#v, want relative add-link plan", planned)
		}
	})
}

func TestRunRejectsUnknownStylesheetAndForbiddenHeadTargetAtomically(t *testing.T) {
	t.Run("stylesheet outside manifest", func(t *testing.T) {
		b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p id="target">text</p>`), true))
		before := literaryCurrent(t, b)
		result, err := Run(t.Context(), b, Params{
			Assignments: []Assignment{{Path: testXHTMLPath, ID: "target", Class: "epigraph"}},
			Stylesheet:  "OEBPS/Styles/missing.css",
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.stylesheet-not-in-manifest") {
			t.Fatalf("result=%+v, want stylesheet error with zero edits", result)
		}
		if !bytes.Equal(before, literaryCurrent(t, b)) {
			t.Fatal("invalid stylesheet changed the XHTML")
		}
	})

	t.Run("title in head", func(t *testing.T) {
		b := openLiteraryFixture(t, literaryFixture(t, literaryXHTML(`<p id="target">text</p>`), true))
		result, err := Run(t.Context(), b, Params{Assignments: []Assignment{
			{Path: testXHTMLPath, ID: "title", Class: "epigraph"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.target-forbidden") {
			t.Fatalf("result=%+v, want forbidden head target", result)
		}
	})
}

func TestRunRejectsMissingClosedHeadAtomically(t *testing.T) {
	chapter := `<html xmlns="http://www.w3.org/1999/xhtml"><head/><body><p id="target">text</p></body></html>`
	b := openLiteraryFixture(t, literaryFixture(t, chapter, true))
	before := literaryCurrent(t, b)
	result, err := Run(t.Context(), b, Params{
		Assignments: []Assignment{{Path: testXHTMLPath, ID: "target", Class: "epigraph"}},
		Stylesheet:  testCSSPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "literary.no-head") {
		t.Fatalf("result=%+v, want no-head error and zero edits", result)
	}
	if !bytes.Equal(before, literaryCurrent(t, b)) {
		t.Fatal("missing closed head applied the class edit")
	}
}

func literaryXHTML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title id="title">Book</title></head><body>` + body + `</body></html>`
}

func literaryFixture(t *testing.T, chapter string, withCSS bool) string {
	t.Helper()
	manifestCSS := ""
	entries := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		testXHTMLPath:            []byte(chapter),
	}
	if withCSS {
		manifestCSS = `<item id="literary" href="Styles/literary.css" media-type="text/css"/>`
		entries[testCSSPath] = []byte(`.epigraph { font-style: italic; }`)
	}
	entries["OEBPS/content.opf"] = []byte(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:identifier id="uid">fixture</dc:identifier><dc:title>Fixture</dc:title><dc:language>zh-CN</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>` + manifestCSS + `</manifest><spine><itemref idref="chapter"/></spine></package>`)
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		header := &zip.FileHeader{Name: path}
		if path == "mimetype" {
			header.Method = zip.Store
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(entries[path]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "literary.epub")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func openLiteraryFixture(t *testing.T, path string) *book.Book {
	t.Helper()
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func literaryCurrent(t *testing.T, b *book.Book) []byte {
	t.Helper()
	data, err := b.Current(testXHTMLPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findingHasID(findings []report.Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
