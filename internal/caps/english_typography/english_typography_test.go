package englishtypography

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const englishPagePath = "OEBPS/Text/english.xhtml"

func TestXMLLangUsesXMLNamespace(t *testing.T) {
	root, err := opf.ScanXHTMLSpanTree([]byte(`<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en"><body/></html>`))
	if err != nil {
		t.Fatal(err)
	}
	value, ok := root.AttrByLocal(opf.XMLURI, "lang")
	if !ok || value != "en" {
		t.Fatalf("xml:lang = %q, present=%v; want XML namespace value en", value, ok)
	}
	if _, wrongNamespace := root.AttrByLocal("", "lang"); wrongNamespace {
		t.Fatal("xml:lang was exposed as an unqualified lang attribute")
	}
}

func TestEnglishTypographyGoldenRedlineAndIdempotent(t *testing.T) {
	root := repoRoot(t)
	before, err := os.ReadFile(filepath.Join(root, "testdata", "english_typography", "basic.before.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "testdata", "english_typography", "basic.after.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: string(before)}}, "zh-CN")
	defer b.Close()
	originalOPF, err := b.Original("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(t.Context(), b, Params{Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusComplete || result.Facts["editCount"] != 1 {
		t.Fatalf("result=%+v", result)
	}
	planned := result.Facts["plannedEdits"].([]plannedEdit)
	if len(planned) != 1 || planned[0] != (plannedEdit{Path: englishPagePath, Action: "add-lang", Value: "en"}) {
		t.Fatalf("plannedEdits=%#v", planned)
	}
	if !hasFinding(result.Findings, "english.opf-language-differs") {
		t.Fatalf("missing OPF language info finding: %+v", result.Findings)
	}
	got, err := b.Current(englishPagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, after) {
		t.Fatalf("XHTML differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, after)
	}
	currentOPF, err := b.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(currentOPF, originalOPF) {
		t.Fatal("OPF metadata changed")
	}
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText, redline.CheckAnchors}, redline.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("text and anchor redlines should pass, got %+v", findings)
	}
	second, err := Run(t.Context(), b, Params{Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Facts["editCount"] != 0 || !hasFinding(second.Findings, "english.already-declared") {
		t.Fatalf("second run is not idempotent: %+v", second)
	}
}

func TestEnglishTypographyMirrorsExistingXMLLang(t *testing.T) {
	xhtml := `<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en-GB"><body>English words</body></html>`
	b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: xhtml}}, "en")
	defer b.Close()
	result, err := Run(t.Context(), b, Params{Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Current(englishPagePath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 1 || !strings.Contains(string(got), `lang="en-GB"`) || !strings.Contains(string(got), `xml:lang="en-GB"`) {
		t.Fatalf("result=%+v XHTML=%s", result, got)
	}
}

func TestEnglishTypographySkipsOrRejectsOtherLanguage(t *testing.T) {
	xhtml := `<html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN"><body>中文内容</body></html>`
	t.Run("implicit scope skips", func(t *testing.T) {
		b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: xhtml}}, "zh-CN")
		defer b.Close()
		result, err := Run(t.Context(), b, Params{Lang: "en"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != report.StatusComplete || result.Facts["editCount"] != 0 || !hasFinding(result.Findings, "english.skipped-other-lang") {
			t.Fatalf("result=%+v", result)
		}
	})
	t.Run("explicit scope rejects atomically", func(t *testing.T) {
		b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: xhtml}}, "zh-CN")
		defer b.Close()
		result, err := Run(t.Context(), b, Params{Lang: "en", ScopePaths: []string{englishPagePath}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 || !hasFinding(result.Findings, "english.lang-conflict") {
			t.Fatalf("result=%+v modified=%v", result, b.ModifiedNames())
		}
	})
}

func TestEnglishTypographyMismatchSkipsOneFileAndContinues(t *testing.T) {
	pages := []englishPage{
		{Path: "OEBPS/Text/mismatch.xhtml", XHTML: `<html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="fr"><body>Words</body></html>`},
		{Path: englishPagePath, XHTML: englishNoLangPage("English words remain here.")},
	}
	b := openEnglishBook(t, pages, "en")
	defer b.Close()
	result, err := Run(t.Context(), b, Params{Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusComplete || result.Facts["editCount"] != 1 || !hasFinding(result.Findings, "english.lang-mismatch") {
		t.Fatalf("result=%+v", result)
	}
	if !slices.Equal(b.ModifiedNames(), []string{englishPagePath}) {
		t.Fatalf("modified names=%v", b.ModifiedNames())
	}
}

func TestEnglishTypographyCJKAndBodyLanguageSkips(t *testing.T) {
	t.Run("CJK ratio", func(t *testing.T) {
		b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: englishNoLangPage("汉字假名한국語 words")}}, "zh-CN")
		defer b.Close()
		result, err := Run(t.Context(), b, Params{Lang: "en"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !hasFinding(result.Findings, "english.skipped-cjk-text") {
			t.Fatalf("result=%+v", result)
		}
	})
	t.Run("body declaration", func(t *testing.T) {
		xhtml := `<html xmlns="http://www.w3.org/1999/xhtml"><body xml:lang="en">English words</body></html>`
		b := openEnglishBook(t, []englishPage{{Path: englishPagePath, XHTML: xhtml}}, "en")
		defer b.Close()
		result, err := Run(t.Context(), b, Params{Lang: "en"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 0 || !hasFinding(result.Findings, "english.declared-on-body") {
			t.Fatalf("result=%+v", result)
		}
	})
}

func TestEnglishTypographyScopeAndAtomicEncodingFailure(t *testing.T) {
	pages := []englishPage{
		{Path: englishPagePath, XHTML: englishNoLangPage("English words here")},
		{Path: "OEBPS/Text/outside.xhtml", XHTML: englishNoLangPage("Other English words")},
	}
	t.Run("scope preserves outside file", func(t *testing.T) {
		b := openEnglishBook(t, pages, "en")
		defer b.Close()
		before, err := b.Original("OEBPS/Text/outside.xhtml")
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(t.Context(), b, Params{Lang: "en", ScopePaths: []string{englishPagePath}})
		if err != nil {
			t.Fatal(err)
		}
		after, err := b.Current("OEBPS/Text/outside.xhtml")
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["editCount"] != 1 || !bytes.Equal(before, after) || !slices.Equal(b.ModifiedNames(), []string{englishPagePath}) {
			t.Fatalf("facts=%#v modified=%v outside changed=%v", result.Facts, b.ModifiedNames(), !bytes.Equal(before, after))
		}
	})
	for _, tc := range []struct {
		name  string
		xhtml string
		id    string
	}{
		{name: "UTF-8 BOM", xhtml: "\uFEFF" + englishNoLangPage("English words"), id: "english.unsupported-encoding"},
		{name: "declared UTF-16", xhtml: `<?xml version="1.0" encoding="UTF-16"?><html xmlns="http://www.w3.org/1999/xhtml"><body>English words</body></html>`, id: "english.unsupported-encoding"},
		{name: "malformed XML", xhtml: `<html xmlns="http://www.w3.org/1999/xhtml"><body>English words`, id: "english.parse-failed"},
	} {
		t.Run(tc.name+" applies no edits", func(t *testing.T) {
			bad := []englishPage{
				{Path: englishPagePath, XHTML: englishNoLangPage("English words here")},
				{Path: "OEBPS/Text/bad.xhtml", XHTML: tc.xhtml},
			}
			b := openEnglishBook(t, bad, "en")
			defer b.Close()
			result, err := Run(t.Context(), b, Params{Lang: "en"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 || !hasFinding(result.Findings, tc.id) {
				t.Fatalf("result=%+v modified=%v", result, b.ModifiedNames())
			}
		})
	}
	t.Run("scope outside spine", func(t *testing.T) {
		b := openEnglishBook(t, pages, "en")
		defer b.Close()
		result, err := Run(t.Context(), b, Params{Lang: "en", ScopePaths: []string{"OEBPS/Text/missing.xhtml"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 || !hasFinding(result.Findings, "english.scope-not-in-spine") {
			t.Fatalf("result=%+v modified=%v", result, b.ModifiedNames())
		}
	})
}

func TestValidLang(t *testing.T) {
	for _, value := range []string{"en", "zh-Hans", "en-GB", "abc-Latn-US"} {
		if !ValidLang(value) {
			t.Errorf("ValidLang(%q)=false", value)
		}
	}
	for _, value := range []string{"", "e", "en_US", "-en", "en-", `en" lang="zh`} {
		if ValidLang(value) {
			t.Errorf("ValidLang(%q)=true", value)
		}
	}
}

type englishPage struct {
	Path  string
	XHTML string
}

func englishNoLangPage(text string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>English</title></head><body><p>` + text + `</p></body></html>`
}

func openEnglishBook(t *testing.T, pages []englishPage, language string) *book.Book {
	t.Helper()
	files := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
	}
	var opfDoc strings.Builder
	_, _ = fmt.Fprintf(&opfDoc, `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:identifier id="uid">urn:uuid:english-fixture</dc:identifier><dc:title>English fixture</dc:title><dc:language>%s</dc:language></metadata><manifest>`, language)
	for i, page := range pages {
		href := strings.TrimPrefix(page.Path, "OEBPS/")
		_, _ = fmt.Fprintf(&opfDoc, `<item id="page%d" href="%s" media-type="application/xhtml+xml"/>`, i+1, href)
		files[page.Path] = page.XHTML
	}
	opfDoc.WriteString(`</manifest><spine>`)
	for i := range pages {
		_, _ = fmt.Fprintf(&opfDoc, `<itemref idref="page%d"/>`, i+1)
	}
	opfDoc.WriteString(`</spine></package>`)
	files["OEBPS/content.opf"] = opfDoc.String()

	path := filepath.Join(t.TempDir(), "english.epub")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func hasFinding(findings []report.Finding, id string) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.ID == id })
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
