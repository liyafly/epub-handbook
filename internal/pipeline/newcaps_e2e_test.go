package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

func TestKindleCompatibilityCheckEndToEnd(t *testing.T) {
	// nav.audit records whether the EPUBCheck executable is available. Keep the
	// golden stable across developer machines by isolating this read-only probe.
	t.Setenv("PATH", t.TempDir())
	input := writeNewCapabilityEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.kindle.compatibility.check",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("status=%q exit=%d findings=%+v, want complete / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	var sawKindle, sawNavAudit, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.kindle.compatibility.check":
			sawKindle = event.Status == "completed"
		case "epub.package.nav.audit":
			sawNavAudit = event.Status == "completed"
		case "redline":
			sawRedline = true
		}
	}
	if !sawKindle || !sawNavAudit || sawRedline {
		t.Fatalf("events=%+v, want nav audit + read-only Kindle check without redline", outcome.Envelope.Events)
	}
	if outcome.Envelope.Facts["epub.kindle.compatibility.check.staticOnly"] != true {
		t.Fatalf("Kindle facts=%#v, want staticOnly=true", outcome.Envelope.Facts)
	}
	if _, ok := outcome.Envelope.Facts["epub.kindle.compatibility.check.counts"]; !ok {
		t.Fatalf("missing Kindle counts fact: %#v", outcome.Envelope.Facts)
	}

	root := repoRootForTest(t)
	goldenPath := filepath.Join(root, "testdata", "kindle_check", "basic.report.json")
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("Kindle E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func writeNewCapabilityEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>New capability fixture</dc:title><dc:identifier id="uid">urn:uuid:new-capability</dc:identifier><dc:language>en</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/chapter.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en"><head><title>Chapter</title></head><body><h1>Chapter</h1><p id="p1">English text</p></body></html>`,
		"OEBPS/toc.ncx":          `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":        "PNG fixture bytes",
		"OEBPS/styles.css":       `.chapter { color: black; }`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "fixture.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}
