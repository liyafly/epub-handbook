package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/report"
)

func TestFontSubsetEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake provider currently uses a POSIX shell")
	}
	pathDir := t.TempDir()
	// An isolated PATH keeps optional EPUBCheck availability out of the golden.
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	writeFontSubsetProvider(t, testBinary, filepath.Join(pathDir, "epub-font"))
	t.Setenv("PATH", pathDir)

	input := writeFontSubsetEPUB(t)
	output := filepath.Join(t.TempDir(), "candidate.epub")
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.font.subset",
		InputPath:    input,
		OutputPath:   output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("status=%q exit=%d findings=%+v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.font.subset.changedFonts"]; fmt.Sprint(got) != "[OEBPS/Fonts/full.ttf]" {
		t.Fatalf("changedFonts = %#v", got)
	}
	var sawSubset, sawAudit, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.font.subset":
			sawSubset = event.Status == "completed"
		case "epub.package.nav.audit":
			sawAudit = event.Status == "completed"
		case "redline":
			sawRedline = event.Status == "completed"
		}
	}
	if !sawSubset || !sawAudit || !sawRedline {
		t.Fatalf("events=%+v, want subset, nav audit, and redline completion", outcome.Envelope.Events)
	}

	sourceBytes, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(sourceBytes, []byte("FULL FONT")) {
		t.Fatal("font capability changed the complete-font source")
	}
	outputBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(outputBytes, []byte("SUBSET FONT")) {
		t.Fatal("output does not contain the provider's subset font")
	}

	goldenPath := filepath.Join(repoRootForTest(t), "testdata", "font_subset", "basic.report.json")
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	outcome.Envelope.Output.Path = "<candidate.epub>"
	outcome.Envelope.Output.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "<fixture.epub>")
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(outcome.Envelope.NextCommands[i], output, "<candidate.epub>")
	}
	for i, event := range outcome.Envelope.Events {
		outcome.Envelope.Events[i].Message = strings.ReplaceAll(event.Message, output, "<candidate.epub>")
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
		t.Fatalf("font subset E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func writeFontSubsetProvider(t *testing.T, testBinary, path string) {
	t.Helper()
	script := "#!/bin/sh\nif [ \"$#\" -lt 4 ] || [ \"$1\" != subset ]; then exit 2; fi\n" +
		"export GO_WANT_FONT_SUBSET_PROVIDER=1\nexport FONT_SUBSET_INPUT=\"$2\"\nexport FONT_SUBSET_OUTPUT=\"$4\"\nexec " +
		quoteFontSubsetShell(testBinary) + " -test.run='^TestFontSubsetProviderProcess$'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFontSubsetProviderProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FONT_SUBSET_PROVIDER") != "1" {
		return
	}
	if code := rewriteFontSubset(os.Getenv("FONT_SUBSET_INPUT"), os.Getenv("FONT_SUBSET_OUTPUT")); code != 0 {
		os.Exit(code)
	}
	os.Exit(0)
}

func rewriteFontSubset(input, outputPath string) int {
	reader, err := zip.OpenReader(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer reader.Close()
	var fontFile *zip.File
	for _, entry := range reader.File {
		if entry.Name == "OEBPS/Fonts/full.ttf" {
			fontFile = entry
			break
		}
	}
	if fontFile == nil {
		fmt.Fprintln(os.Stderr, "provider input does not contain the font master")
		return 1
	}
	font, err := fontFile.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fontBytes, err := io.ReadAll(font)
	_ = font.Close()
	if err != nil || string(fontBytes) != "FULL FONT" {
		fmt.Fprintln(os.Stderr, "provider input did not contain the complete font master")
		return 1
	}
	output, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	writer := zip.NewWriter(output)
	for _, file := range reader.File {
		if file.Name != "OEBPS/Fonts/full.ttf" {
			if err := writer.Copy(file); err != nil {
				fmt.Fprintln(os.Stderr, err)
				_ = writer.Close()
				_ = output.Close()
				return 1
			}
			continue
		}
		header := &zip.FileHeader{Name: file.Name, Method: file.Method}
		header.Modified = file.Modified
		entry, err := writer.CreateHeader(header)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			_ = writer.Close()
			_ = output.Close()
			return 1
		}
		if _, err := io.WriteString(entry, "SUBSET FONT"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			_ = writer.Close()
			_ = output.Close()
			return 1
		}
	}
	if err := writer.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = output.Close()
		return 1
	}
	if err := output.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func quoteFontSubsetShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func writeFontSubsetEPUB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "font-source.epub")
	files := []struct {
		name string
		data []byte
	}{
		{"mimetype", []byte("application/epub+zip")},
		{"META-INF/container.xml", []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{"OEBPS/package.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="id"><metadata><dc:identifier id="id">urn:uuid:font-subset</dc:identifier><dc:title>Font subset test</dc:title><dc:creator>Test</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-01-01T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="font" href="Fonts/full.ttf" media-type="application/vnd.ms-opentype"/></manifest><spine><itemref idref="chapter"/></spine></package>`)},
		{"OEBPS/nav.xhtml", []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body></html>`)},
		{"OEBPS/chapter.xhtml", []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title></head><body><p>Hello.</p></body></html>`)},
		{"OEBPS/Fonts/full.ttf", []byte("FULL FONT")},
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(f)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		if file.name == "mimetype" {
			header.Method = zip.Store
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
