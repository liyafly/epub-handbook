package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestNotesFallbackEndToEnd(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackEPUB(t, false)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.notes.legacy-fallback.editCount"]; got != 3 {
		t.Fatalf("editCount=%#v, want 3", got)
	}
	if got := outcome.Envelope.Facts["epub.notes.popup.normalize.violations"]; got != 0 {
		t.Fatalf("popup violations=%#v, want 0", got)
	}
	if got := outcome.Envelope.Facts["modified_entries"]; !slices.Equal(got.([]string), []string{"OEBPS/Text/chapter.xhtml"}) {
		t.Fatalf("modified_entries=%#v", got)
	}
	var sawFallback, sawPopup, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.notes.legacy-fallback":
			sawFallback = event.Status == "completed"
		case "epub.notes.popup.normalize":
			sawPopup = event.Status == "completed"
		case "redline":
			sawRedline = event.Status == "completed"
		}
	}
	if !sawFallback || !sawPopup || !sawRedline {
		t.Fatalf("events=%+v, want popup, fallback, and redline completion", outcome.Envelope.Events)
	}
	root := repoRootForTest(t)
	goldenPath := filepath.Join(root, "testdata", "notes_fallback", "basic.report.json")
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "<fixture.epub>")
	}
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
		t.Fatalf("notes fallback E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestNotesFallbackRejectsInvalidPopupUpstream(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackEPUB(t, true)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitFailed || outcome.Envelope.Status != report.StatusFailed {
		t.Fatalf("status=%q exit=%d facts=%#v findings=%+v events=%+v, want failed / 1", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Facts, outcome.Envelope.Findings, outcome.Envelope.Events)
	}
	if !slices.ContainsFunc(outcome.Envelope.Findings, func(finding report.Finding) bool {
		return finding.ID == "notes-fallback.upstream-not-clean"
	}) {
		t.Fatalf("missing upstream-not-clean finding: %+v", outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["modified_entries"]; len(got.([]string)) != 0 {
		t.Fatalf("modified_entries=%#v, want []", got)
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

func writeNotesFallbackEPUB(t *testing.T, missingBacklink bool) string {
	t.Helper()
	chapter := `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Chapter</title></head><body><p id="p1">正文<a id="nr1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p><aside epub:type="footnote" role="doc-footnote"><ol class="footnote-list"><li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#nr1">↩</a></li></ol></aside></body></html>`
	if missingBacklink {
		chapter = strings.Replace(chapter, `href="#nr1">↩`, `href="#missing">↩`, 1)
	}
	entries := map[string]string{
		"META-INF/container.xml":   `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":        `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Notes fallback fixture</dc:title><dc:identifier id="uid">urn:uuid:notes-fallback</dc:identifier><dc:language>zh-CN</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="note-icon" href="Icons/note.png" media-type="image/png"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":          `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/Text/chapter.xhtml": chapter,
		"OEBPS/toc.ncx":            `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="Text/chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":          "PNG fixture bytes",
		"OEBPS/Icons/note.png":     "PNG fixture bytes",
		"OEBPS/styles.css":         `.footnote-list { border-top: 1px solid; }`,
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
	input := filepath.Join(t.TempDir(), "notes-fallback.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}
