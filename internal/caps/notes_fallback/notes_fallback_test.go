package notesfallback

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

const (
	noteXHTMLPath = "OEBPS/Text/chapter.xhtml"
	fixtureOPF    = `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Notes fixture</dc:title><dc:identifier id="uid">urn:uuid:notes-fixture</dc:identifier><dc:language>zh-CN</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="note-icon" href="Icons/note.png" media-type="image/png"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="Styles/notes.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`
	fixtureNav    = `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">Chapter</a></li></ol></nav></body></html>`
	fixtureNCX    = `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Notes fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="Text/chapter.xhtml"/></navPoint></navMap></ncx>`
	fixtureCover  = "PNG fixture bytes"
	fixtureIcon   = "PNG fixture bytes"
	fixtureStyles = `.footnote-list { border-top: 1px solid; }`
	fixtureXHTML  = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN">
  <head><title>Notes</title></head>
  <body>
    <p id="p1">正文<a id="nr1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#nr1">↩</a></li>
      </ol>
    </aside>
  </body>
</html>
`
)

func TestLegacyFallbackGoldenRedlineAndIdempotent(t *testing.T) {
	root := repoRoot(t)
	before, err := os.ReadFile(filepath.Join(root, "testdata", "notes_fallback", "basic.before.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "testdata", "notes_fallback", "basic.after.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	files := notesFiles(string(before))
	b := openNotesBook(t, files)
	defer b.Close()

	first, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != report.StatusComplete || first.Facts["editCount"] != 3 {
		t.Fatalf("first result status=%q facts=%#v findings=%+v", first.Status, first.Facts, first.Findings)
	}
	got, err := b.Current(noteXHTMLPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, after) {
		t.Fatalf("XHTML differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, after)
	}
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText, redline.CheckMetadata, redline.CheckSpine, redline.CheckAnchors, redline.CheckCover, redline.CheckDRM}, redline.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("all redlines should pass, got %+v", findings)
	}

	second, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if second.Facts["editCount"] != 0 || len(second.Facts["plannedEdits"].([]plannedEdit)) != 0 {
		t.Fatalf("second run should be idempotent: %#v", second.Facts)
	}
}

func TestNotesFallbackRootLevelOPF(t *testing.T) {
	files := notesFiles(fixtureXHTML)
	rootFiles := make(map[string]string, len(files))
	for name, content := range files {
		if name == "META-INF/container.xml" {
			content = strings.Replace(content, "OEBPS/content.opf", "content.opf", 1)
		}
		name = strings.TrimPrefix(name, "OEBPS/")
		rootFiles[name] = content
	}
	b := openNotesBook(t, rootFiles)
	defer b.Close()

	result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusComplete || result.Facts["editCount"] != 3 {
		t.Fatalf("root-level OPF fallback result=%+v, want complete with three edits", result)
	}
}

func TestLegacyFallbackAddsOnlyMissingClasses(t *testing.T) {
	partial := strings.Replace(fixtureXHTML, `class="noteref-icon"`, `class="noteref-icon duokan-footnote"`, 1)
	partial = strings.Replace(partial, `class="footnote-item"`, `class="footnote-item duokan-footnote-item"`, 1)
	b := openNotesBook(t, notesFiles(partial))
	defer b.Close()
	result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusComplete || result.Facts["editCount"] != 1 {
		t.Fatalf("result=%+v, want only the ol class edit", result)
	}
	got, err := b.Current(noteXHTMLPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`class="noteref-icon duokan-footnote"`,
		`class="footnote-list duokan-footnote-content"`,
		`class="footnote-item duokan-footnote-item"`,
	} {
		if !strings.Contains(string(got), fragment) {
			t.Errorf("missing class fragment %q in %s", fragment, got)
		}
	}
}

func TestLegacyFallbackParseAndStructuralErrorsApplyNoEdits(t *testing.T) {
	tests := []struct {
		name  string
		xhtml string
		id    string
		notes int
	}{
		{name: "malformed unquoted class", xhtml: strings.Replace(fixtureXHTML, `class="noteref-icon"`, `class=noteref-icon`, 1), id: "notes-fallback.parse-failed", notes: 0},
		{name: "multiple note lists", xhtml: strings.Replace(fixtureXHTML, `</ol>`, `</ol><ol class="footnote-list"><li class="footnote-item" id="n2">另一条</li></ol>`, 1), id: "notes-fallback.multiple-lists", notes: 1},
		{name: "noteref without icon", xhtml: strings.Replace(fixtureXHTML, `<img src="../Icons/note.png" alt="注"/>`, ``, 1), id: "notes-fallback.noteref-without-icon", notes: 1},
		{name: "content class on li", xhtml: strings.Replace(fixtureXHTML, `class="footnote-item"`, `class="footnote-item duokan-footnote-content"`, 1), id: "notes-fallback.content-class-on-li", notes: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := openNotesBook(t, notesFiles(tt.xhtml))
			defer b.Close()
			result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: tt.notes})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 {
				t.Fatalf("failed result applied edits: status=%s facts=%#v modified=%v", result.Status, result.Facts, b.ModifiedNames())
			}
			if !hasFinding(result.Findings, tt.id) {
				t.Fatalf("missing %s in %+v", tt.id, result.Findings)
			}
		})
	}
}

func TestLegacyFallbackProcessesWholeSpineAndHandlesNoNotes(t *testing.T) {
	files := notesFiles(fixtureXHTML)
	second := strings.Replace(fixtureOPF, `</manifest>`, `<item id="chapter2" href="Text/second.xhtml" media-type="application/xhtml+xml"/></manifest>`, 1)
	second = strings.Replace(second, `</spine>`, `<itemref idref="chapter2"/></spine>`, 1)
	files["OEBPS/content.opf"] = second
	files["OEBPS/Text/second.xhtml"] = strings.Replace(fixtureXHTML, `id="nr1"`, `id="nr2"`, 1)
	b := openNotesBook(t, files)
	defer b.Close()
	originalSecond, err := b.Original("OEBPS/Text/second.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["editCount"] != 6 || !slices.Equal(b.ModifiedNames(), []string{noteXHTMLPath, "OEBPS/Text/second.xhtml"}) {
		t.Fatalf("whole-spine result facts=%#v modified=%v", result.Facts, b.ModifiedNames())
	}
	currentSecond, err := b.Current("OEBPS/Text/second.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(originalSecond, currentSecond) {
		t.Fatal("whole-spine processing left the second note XHTML unchanged")
	}

	noNotes := strings.Replace(fixtureXHTML, fixtureXHTML[strings.Index(fixtureXHTML, "    <p id=\"p1\">"):strings.Index(fixtureXHTML, "    <aside")], "    <p id=\"p1\">正文。</p>\n", 1)
	noNotes = noNotes[:strings.Index(noNotes, "    <aside")] + "  </body>\n</html>\n"
	b2 := openNotesBook(t, notesFiles(noNotes))
	defer b2.Close()
	noNoteResult, err := Run(t.Context(), b2, Params{UpstreamViolations: 0, UpstreamNoterefs: 0})
	if err != nil {
		t.Fatal(err)
	}
	if noNoteResult.Status != report.StatusComplete || noNoteResult.Facts["editCount"] != 0 || !hasFinding(noNoteResult.Findings, "notes-fallback.no-notes") {
		t.Fatalf("no-notes result=%+v", noNoteResult)
	}
}

func TestLegacyFallbackRejectsBadUpstream(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Params
		id   string
	}{
		{name: "upstream violations", p: Params{UpstreamViolations: 2, UpstreamNoterefs: 1}, id: "notes-fallback.upstream-not-clean"},
		{name: "upstream missing", p: Params{UpstreamViolations: -1, UpstreamNoterefs: -1}, id: "notes-fallback.upstream-not-clean"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := openNotesBook(t, notesFiles(fixtureXHTML))
			defer b.Close()
			result, err := Run(t.Context(), b, tt.p)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 || !hasFinding(result.Findings, tt.id) {
				t.Fatalf("result=%+v modified=%v", result, b.ModifiedNames())
			}
		})
	}
}

func TestUpstreamViolationDetailNamesStandardViolations(t *testing.T) {
	b := openNotesBook(t, notesFiles(fixtureXHTML))
	defer b.Close()
	result, err := Run(t.Context(), b, Params{UpstreamViolations: 2, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range result.Findings {
		if finding.ID == "notes-fallback.upstream-not-clean" {
			if want := UpstreamID + " standardViolations=2"; finding.Detail != want {
				t.Fatalf("upstream Detail=%q, want %q", finding.Detail, want)
			}
			return
		}
	}
	t.Fatalf("missing upstream-not-clean finding: %+v", result.Findings)
}

func TestLegacyFallbackRejectsUpstreamCoverageMismatch(t *testing.T) {
	b := openNotesBook(t, notesFiles(fixtureXHTML))
	defer b.Close()
	result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 0})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusFailed || len(b.ModifiedNames()) != 0 {
		t.Fatalf("result=%+v modified=%v, want failure without edits", result, b.ModifiedNames())
	}
	if !hasFinding(result.Findings, "notes-fallback.upstream-coverage-mismatch") {
		t.Fatalf("missing upstream coverage mismatch: %+v", result.Findings)
	}
}

func TestLegacyFallbackRejectsUTF16BOM(t *testing.T) {
	files := notesFiles(fixtureXHTML)
	files[noteXHTMLPath] = string([]byte{0xFF, 0xFE}) + fixtureXHTML
	b := openNotesBook(t, files)
	defer b.Close()
	result, err := Run(t.Context(), b, Params{UpstreamViolations: 0, UpstreamNoterefs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusFailed || result.Facts["editCount"] != 0 || len(b.ModifiedNames()) != 0 || !hasFinding(result.Findings, "notes-fallback.unsupported-encoding") {
		t.Fatalf("result=%+v modified=%v", result, b.ModifiedNames())
	}
}

func notesFiles(xhtml string) map[string]string {
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      fixtureOPF,
		"OEBPS/nav.xhtml":        fixtureNav,
		"OEBPS/toc.ncx":          fixtureNCX,
		"OEBPS/cover.png":        fixtureCover,
		"OEBPS/Icons/note.png":   fixtureIcon,
		noteXHTMLPath:            xhtml,
		"OEBPS/Styles/notes.css": fixtureStyles,
	}
}

func openNotesBook(t *testing.T, files map[string]string) *book.Book {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.epub")
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
