package verticalruby

import (
	"archive/zip"
	"bytes"
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

const rubyPath = "OEBPS/Text/chapter.xhtml"
const cssPath = "OEBPS/Styles/vertical.css"

func TestRubyRPFillsEveryDirectRTAndIsIdempotent(t *testing.T) {
	input := makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p><ruby>漢<rt>かん</rt>字<rt>じ</rt></ruby></p></body></html>`, "")
	b := openFixture(t, input)
	first, err := Run(context.Background(), b, Params{Op: OpRubyRP})
	if err != nil {
		t.Fatal(err)
	}
	got := current(t, b, rubyPath)
	want := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p><ruby>漢<rp>（</rp><rt>かん</rt><rp>）</rp>字<rp>（</rp><rt>じ</rt><rp>）</rp></ruby></p></body></html>`
	if string(got) != want {
		t.Fatalf("Ruby output = %s\nwant       = %s", got, want)
	}
	if first.Facts["editCount"] != 4 {
		t.Fatalf("editCount=%v, want 4", first.Facts["editCount"])
	}
	planned := first.Facts["plannedEdits"].([]plannedEdit)
	if len(planned) != 1 || planned[0].Target != 1 {
		t.Fatalf("plannedEdits=%#v, want one first Ruby", planned)
	}
	second, err := Run(context.Background(), b, Params{Op: OpRubyRP})
	if err != nil {
		t.Fatal(err)
	}
	if second.Facts["editCount"] != 0 || !findingHasID(second.Findings, "vertical.ruby-has-rp") {
		t.Fatalf("second run facts=%#v findings=%+v, want idempotent no-op", second.Facts, second.Findings)
	}
}

func TestRubyRPPartialComplexAndPrefixedStructures(t *testing.T) {
	chapter := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:x="urn:test"><body>` +
		`<ruby>完<rp>(</rp><rt>かん</rt><rp>)</rp></ruby>` +
		`<ruby>部<rp>(</rp><rt>ぶ</rt></ruby>` +
		`<ruby>複<rtc><rt>ふく</rt></rtc></ruby>` +
		`<ruby><ruby>内<rt>ない</rt></ruby><rt>がい</rt></ruby>` +
		`<ruby><rt/></ruby>` +
		`<x:ruby><x:rt>prefixed</x:rt></x:ruby>` +
		`<ruby><x:rt>prefixed rt</x:rt></ruby>` +
		`<ruby>安<rt>あん</rt></ruby></body></html>`
	b := openFixture(t, makeFixture(t, chapter, ""))
	result, err := Run(context.Background(), b, Params{Op: OpRubyRP})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 2 {
		t.Fatalf("editCount=%v, want only the final simple Ruby (2 insertions)", result.Facts["editCount"])
	}
	for _, id := range []string{"vertical.ruby-has-rp", "vertical.ruby-partial-rp", "vertical.ruby-complex", "vertical.ruby-empty-rt", "vertical.ruby-prefixed"} {
		if !findingHasID(result.Findings, id) {
			t.Errorf("missing finding %q in %+v", id, result.Findings)
		}
	}
	got := string(current(t, b, rubyPath))
	if !strings.Contains(got, `<ruby>安<rp>（</rp><rt>あん</rt><rp>）</rp></ruby>`) {
		t.Fatalf("simple Ruby not repaired: %s", got)
	}
	if strings.Contains(got, `<ruby>複<rp>`) || strings.Contains(got, `<x:ruby><rp>`) {
		t.Fatalf("complex or prefixed Ruby was modified: %s", got)
	}
}

func TestRubyRPBracketOverrideAndAtomicScopeError(t *testing.T) {
	input := makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><ruby>漢<rt>かん</rt></ruby></body></html>`, "")
	b := openFixture(t, input)
	before := current(t, b, rubyPath)
	failed, err := Run(context.Background(), b, Params{Op: OpRubyRP, ScopePaths: []string{rubyPath, "OEBPS/Text/missing.xhtml"}, RPOpen: "[", RPClose: "]"})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Facts["editCount"] != 0 || !findingHasLevel(failed.Findings, "error") {
		t.Fatalf("result=%+v, want atomic scope error", failed)
	}
	if !bytes.Equal(current(t, b, rubyPath), before) {
		t.Fatal("scope error applied a partial Ruby edit")
	}
	good, err := Run(context.Background(), b, Params{Op: OpRubyRP, RPOpen: "[", RPClose: "]"})
	if err != nil {
		t.Fatal(err)
	}
	if good.Facts["editCount"] != 2 || !strings.Contains(string(current(t, b, rubyPath)), `<rp>[</rp><rt>かん</rt><rp>]</rp>`) {
		t.Fatalf("custom bracket result=%s facts=%#v", current(t, b, rubyPath), good.Facts)
	}
	if ValidRPToken("<") || ValidRPToken(" ") || ValidRPToken("xy") || !ValidRPToken("[") {
		t.Fatal("ValidRPToken did not enforce the documented one-rune safe-token rule")
	}
}

func TestRubyRPParseErrorAppliesNoEdits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		chapter string
		finding string
	}{
		{name: "malformed", chapter: `<html xmlns="http://www.w3.org/1999/xhtml"><body><ruby>漢<rt>かん</rt></ruby>`, finding: "vertical.xhtml-parse-failed"},
		{name: "UTF-8 BOM", chapter: "\uFEFF<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><ruby>漢<rt>かん</rt></ruby></body></html>", finding: "vertical.unsupported-encoding"},
		{name: "UTF-16 BOM", chapter: string([]byte{0xFF, 0xFE}) + `<html xmlns="http://www.w3.org/1999/xhtml"><body><ruby>漢<rt>かん</rt></ruby></body></html>`, finding: "vertical.unsupported-encoding"},
		{name: "non-UTF-8 declaration", chapter: `<?xml version="1.0" encoding="UTF-16"?><html xmlns="http://www.w3.org/1999/xhtml"><body><ruby>漢<rt>かん</rt></ruby></body></html>`, finding: "vertical.unsupported-encoding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := openFixture(t, makeFixture(t, tc.chapter, ""))
			before := current(t, b, rubyPath)
			result, err := Run(context.Background(), b, Params{Op: OpRubyRP})
			if err != nil {
				t.Fatal(err)
			}
			if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, tc.finding) {
				t.Fatalf("facts=%#v findings=%+v", result.Facts, result.Findings)
			}
			if !bytes.Equal(current(t, b, rubyPath), before) {
				t.Fatal("encoding or parse error applied an edit")
			}
		})
	}
}

func TestWritingModePrefixCombinationsAtRulesAndConflicts(t *testing.T) {
	css := `.one { writing-mode: vertical-rl; }` + "\n" +
		`.two { -epub-writing-mode: vertical-lr; writing-mode: vertical-lr; }` + "\n" +
		`.three { writing-mode: horizontal-tb; -webkit-writing-mode: horizontal-tb; -epub-writing-mode: horizontal-tb; }` + "\n" +
		`@media (min-width: 20em) { .four { writing-mode: vertical-rl; } }` + "\n" +
		`.conflict { -webkit-writing-mode: horizontal-tb; writing-mode: vertical-rl; }` + "\n" +
		`.important { writing-mode: vertical-rl !important; }`
	b := openFixture(t, makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`, css))
	result, err := Run(context.Background(), b, Params{Op: OpWritingModePrefix})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 3 {
		t.Fatalf("editCount=%v, want inserts for .one, .two, and @media .four", result.Facts["editCount"])
	}
	if !findingHasID(result.Findings, "vertical.prefix-conflict") || !findingHasID(result.Findings, "vertical.writing-mode-unsupported-value") {
		t.Fatalf("findings=%+v, want conflict and unsupported warnings", result.Findings)
	}
	got := string(current(t, b, cssPath))
	for _, want := range []string{
		`.one { -webkit-writing-mode: vertical-rl; -epub-writing-mode: vertical-rl; writing-mode: vertical-rl; }`,
		`.two { -epub-writing-mode: vertical-lr; -webkit-writing-mode: vertical-lr; writing-mode: vertical-lr; }`,
		`.three { writing-mode: horizontal-tb; -webkit-writing-mode: horizontal-tb; -epub-writing-mode: horizontal-tb; }`,
		`.four { -webkit-writing-mode: vertical-rl; -epub-writing-mode: vertical-rl; writing-mode: vertical-rl; }`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CSS missing repaired fragment %q in %s", want, got)
		}
	}
	second, err := Run(context.Background(), b, Params{Op: OpWritingModePrefix})
	if err != nil {
		t.Fatal(err)
	}
	if second.Facts["editCount"] != 0 {
		t.Fatalf("second prefix pass editCount=%v, want idempotent no-op", second.Facts["editCount"])
	}
}

func TestWritingModePrefixParseAndScopeErrorsAreAtomic(t *testing.T) {
	input := makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><ruby>漢<rt>かん</rt></ruby></body></html>`, `.one { writing-mode: vertical-rl; }`)
	b := openFixture(t, input)
	rubyBefore := current(t, b, rubyPath)
	cssBefore := current(t, b, cssPath)
	badScope, err := Run(context.Background(), b, Params{Op: OpWritingModePrefix, ScopePaths: []string{rubyPath}})
	if err != nil {
		t.Fatal(err)
	}
	if badScope.Facts["editCount"] != 0 || !findingHasID(badScope.Findings, "vertical.scope-not-css-manifest-item") {
		t.Fatalf("bad scope result=%+v", badScope)
	}
	broken := openFixture(t, makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`, `.one { writing-mode: vertical-rl;`))
	brokenBefore := current(t, broken, cssPath)
	parseFailure, err := Run(context.Background(), broken, Params{Op: OpWritingModePrefix})
	if err != nil {
		t.Fatal(err)
	}
	if parseFailure.Facts["editCount"] != 0 || !findingHasID(parseFailure.Findings, "vertical.css-parse-failed") {
		t.Fatalf("parse failure=%+v", parseFailure)
	}
	if !bytes.Equal(current(t, broken, cssPath), brokenBefore) {
		t.Fatal("CSS parse error applied an edit")
	}
	if !bytes.Equal(current(t, b, rubyPath), rubyBefore) || !bytes.Equal(current(t, b, cssPath), cssBefore) {
		t.Fatal("scope error changed book entries")
	}
}

func TestWritingModePrefixScopeMustNameCSSManifestItem(t *testing.T) {
	b := openFixture(t, makeFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`, `.one { writing-mode: vertical-rl; }`))
	result, err := Run(context.Background(), b, Params{Op: OpWritingModePrefix, ScopePaths: []string{cssPath}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 1 {
		t.Fatalf("editCount=%v, want one insertion for the targeted rule", result.Facts["editCount"])
	}
	if got := current(t, b, cssPath); !bytes.Contains(got, []byte("-webkit-writing-mode: vertical-rl; -epub-writing-mode: vertical-rl; writing-mode")) {
		t.Fatalf("scope-targeted prefix output=%s", got)
	}
}

func TestWritingModePrefixInvalidCSSIsAtomicEvenAfterEarlierValidCSS(t *testing.T) {
	input := makeFixtureWithExtraCSS(t,
		`<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		`.one { writing-mode: vertical-rl; }`,
		`OEBPS/Styles/broken.css`, `.bad { writing-mode: vertical-lr;`,
	)
	b := openFixture(t, input)
	beforeGood := current(t, b, cssPath)
	beforeBad := current(t, b, "OEBPS/Styles/broken.css")
	result, err := Run(context.Background(), b, Params{Op: OpWritingModePrefix})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 0 || !findingHasID(result.Findings, "vertical.css-parse-failed") {
		t.Fatalf("result=%+v", result)
	}
	if !bytes.Equal(current(t, b, cssPath), beforeGood) || !bytes.Equal(current(t, b, "OEBPS/Styles/broken.css"), beforeBad) {
		t.Fatal("error in later CSS resource applied earlier planned edits")
	}
}

func findingHasID(findings []report.Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func findingHasLevel(findings []report.Finding, level string) bool {
	for _, finding := range findings {
		if finding.Level == level {
			return true
		}
	}
	return false
}

func makeFixture(t *testing.T, chapter, css string) string {
	t.Helper()
	return makeFixtureWithExtraCSS(t, chapter, css, "", "")
}

func makeFixtureWithExtraCSS(t *testing.T, chapter, css, extraCSSPath, extraCSS string) string {
	t.Helper()
	manifestCSS := ""
	entries := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		rubyPath:                 []byte(chapter),
	}
	if css != "" || extraCSSPath != "" {
		manifestCSS = `<item id="css" href="Styles/vertical.css" media-type="text/css"/>`
		entries[cssPath] = []byte(css)
	}
	if extraCSSPath != "" {
		manifestCSS += `<item id="css-extra" href="` + strings.TrimPrefix(extraCSSPath, "OEBPS/") + `" media-type="text/css"/>`
		entries[extraCSSPath] = []byte(extraCSS)
	}
	entries["OEBPS/content.opf"] = []byte(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:identifier id="uid">fixture</dc:identifier><dc:title>Fixture</dc:title><dc:language>ja</dc:language></metadata><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>` + manifestCSS + `</manifest><spine><itemref idref="chapter"/></spine></package>`)
	return writeArchive(t, entries)
}

func writeArchive(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		header := &zip.FileHeader{Name: name}
		if name == "mimetype" {
			header.Method = zip.Store
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(entries[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.epub")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func openFixture(t *testing.T, path string) *book.Book {
	t.Helper()
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func current(t *testing.T, b *book.Book, path string) []byte {
	t.Helper()
	data, err := b.Current(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
