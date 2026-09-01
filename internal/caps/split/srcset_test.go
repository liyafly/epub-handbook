package split

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

func buildSrcsetEntries(manifestB, includeB bool) []zipEntry {
	manifestBItem := ""
	if manifestB {
		manifestBItem = `<item id="b" href="Images/b.webp" media-type="image/webp"/>`
	}
	entries := []zipEntry{
		{name: "META-INF/container.xml", content: []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:srcset</dc:identifier><dc:title>srcset</dc:title><dc:creator>Author</dc:creator><dc:language>en</dc:language></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chap" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="a" href="Images/a.webp" media-type="image/webp"/>` + manifestBItem + `</manifest><spine><itemref idref="nav" linear="no"/><itemref idref="chap"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", content: []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Text/chapter.xhtml", content: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="start">Chapter</h1><picture><source srcset="../Images/a.webp 1x, ../Images/b.webp 2x"/><img src="../Images/a.webp"/></picture></body></html>`)},
		{name: "OEBPS/Images/a.webp", content: []byte("a")},
	}
	if includeB {
		entries = append(entries, zipEntry{name: "OEBPS/Images/b.webp", content: []byte("b")})
	}
	entries = append(entries, zipEntry{name: "mimetype", content: []byte("wrong")})
	return entries
}

func TestSplitRetainsAllLocalSrcsetCandidates(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "srcset.epub")
	outDir := filepath.Join(dir, "out")
	buildEpub(t, source, buildSrcsetEntries(true, true))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatalf("missing srcset target returned Go error instead of failed result: %v", err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s, findings = %+v", res.Status, res.Findings)
	}
	entries := readZipEntries(t, filepath.Join(outDir, "srcset_01.epub"))
	for _, name := range []string{"OEBPS/Images/a.webp", "OEBPS/Images/b.webp"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("srcset resource %s was not retained", name)
		}
	}
}

func TestSplitFailsWhenSrcsetTargetIsMissing(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "srcset.epub")
	outDir := filepath.Join(dir, "out")
	buildEpub(t, source, buildSrcsetEntries(true, false))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("missing srcset target should fail: %+v", res)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed split left output directory: %v", statErr)
	}
}

func TestSplitFailsWhenSrcsetTargetIsNotManifested(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "srcset.epub")
	outDir := filepath.Join(dir, "out")
	buildEpub(t, source, buildSrcsetEntries(false, true))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("unmanifested srcset target should fail: %+v", res)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed split left output directory: %v", statErr)
	}
}

func buildInlineCSSReferenceEntries(manifestInline bool) []zipEntry {
	inlineItem := ""
	if manifestInline {
		inlineItem = `<item id="inline" href="Images/inline.webp" media-type="image/webp"/>`
	}
	return []zipEntry{
		{name: "META-INF/container.xml", content: []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:inline-css</dc:identifier><dc:title>inline css</dc:title><dc:creator>Author</dc:creator><dc:language>en</dc:language></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chap" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style-image" href="Images/style.webp" media-type="image/webp"/>` + inlineItem + `</manifest><spine><itemref idref="nav" linear="no"/><itemref idref="chap"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", content: []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Text/chapter.xhtml", content: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><style>.hero{background:url("../Images/style.webp")}</style></head><body><h1 id="start" class="hero" style="background:url('../Images/inline.webp')">Chapter</h1></body></html>`)},
		{name: "OEBPS/Images/style.webp", content: []byte("style")},
		{name: "OEBPS/Images/inline.webp", content: []byte("inline")},
		{name: "mimetype", content: []byte("wrong")},
	}
}

func TestSplitRetainsInlineCSSReferences(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "inline-css.epub")
	outDir := filepath.Join(dir, "out")
	buildEpub(t, source, buildInlineCSSReferenceEntries(true))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s, findings = %+v", res.Status, res.Findings)
	}
	entries := readZipEntries(t, filepath.Join(outDir, "inline-css_01.epub"))
	for _, name := range []string{"OEBPS/Images/style.webp", "OEBPS/Images/inline.webp"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("inline CSS resource %s was not retained", name)
		}
	}
}

func TestSplitFailsWhenInlineCSSReferenceIsNotManifested(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "inline-css.epub")
	outDir := filepath.Join(dir, "out")
	buildEpub(t, source, buildInlineCSSReferenceEntries(false))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("unmanifested inline CSS target should fail: %+v", res)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed split left output directory: %v", statErr)
	}
}
