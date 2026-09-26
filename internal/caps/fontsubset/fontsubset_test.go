package fontsubset

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/book"
)

func TestRunReplacesOnlyManifestFontInMemory(t *testing.T) {
	provider := makeProvider(t, `
import sys, zipfile
source = sys.argv[2]
output = sys.argv[sys.argv.index("--out") + 1]
with zipfile.ZipFile(source) as src, zipfile.ZipFile(output, "w") as dst:
    for entry in src.infolist():
        data = b"SUBSET FONT" if entry.filename == "OEBPS/Fonts/full.ttf" else src.read(entry)
        dst.writestr(entry, data)
`)
	b, input := openFontBook(t)
	defer b.Close()

	chapterBefore, err := b.Current("OEBPS/Text/chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(t.Context(), b, Params{ToolPath: provider})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "complete" {
		t.Fatalf("status = %q", result.Status)
	}
	font, err := b.Current("OEBPS/Fonts/full.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(font, []byte("SUBSET FONT")) {
		t.Fatalf("font bytes = %q", font)
	}
	chapterAfter, err := b.Current("OEBPS/Text/chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(chapterBefore, chapterAfter) {
		t.Fatal("font subsetting changed the chapter")
	}
	changed, ok := result.Facts["changedFonts"].([]string)
	if !ok || !slices.Equal(changed, []string{"OEBPS/Fonts/full.ttf"}) {
		t.Fatalf("changedFonts = %#v", result.Facts["changedFonts"])
	}
	onDisk, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	original, err := book.OpenBytesContext(t.Context(), input, onDisk)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	font, err = original.Original("OEBPS/Fonts/full.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(font, []byte("FULL FONT")) {
		t.Fatal("provider changed the frozen input EPUB")
	}
}

func TestRunProviderFailureLeavesBookUnchanged(t *testing.T) {
	provider := makeFailingProvider(t)
	b, _ := openFontBook(t)
	defer b.Close()

	_, err := Run(t.Context(), b, Params{ToolPath: provider})
	if err == nil {
		t.Fatal("Run() succeeded with a failing provider")
	}
	font, readErr := b.Current("OEBPS/Fonts/full.ttf")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(font, []byte("FULL FONT")) {
		t.Fatalf("font changed after provider failure: %q", font)
	}
}

func TestRunSupportsRelativeInputPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("provider shim uses a POSIX executable")
	}
	provider := makeProvider(t, `import sys, zipfile
source = sys.argv[2]
output = sys.argv[sys.argv.index("--out") + 1]
with zipfile.ZipFile(source) as src, zipfile.ZipFile(output, "w") as dst:
    for entry in src.infolist():
        data = b"SUBSET FONT" if entry.filename == "OEBPS/Fonts/full.ttf" else src.read(entry)
        dst.writestr(entry, data)
`)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("source.epub", fontFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open("source.epub")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := Run(t.Context(), b, Params{ToolPath: provider}); err != nil {
		t.Fatalf("Run() with relative input path: %v", err)
	}
}

func makeProvider(t *testing.T, pythonBody string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("provider shim uses a POSIX executable")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is unavailable")
	}
	path := filepath.Join(t.TempDir(), "epub-font")
	script := "#!/bin/sh\nexec " + shellQuote(python) + " -c '" + pythonBody + "' \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func makeFailingProvider(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("provider shim uses a POSIX executable")
	}
	path := filepath.Join(t.TempDir(), "epub-font")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho provider failed >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func openFontBook(t *testing.T) (*book.Book, string) {
	t.Helper()
	input := filepath.Join(t.TempDir(), "source.epub")
	if err := os.WriteFile(input, fontFixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	return b, input
}

func fontFixture(t *testing.T) []byte {
	t.Helper()
	files := []struct {
		name string
		data []byte
	}{
		{"mimetype", []byte("application/epub+zip")},
		{"META-INF/container.xml", []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf"/></rootfiles></container>`)},
		{"OEBPS/package.opf", []byte(`<package xmlns="http://www.idpf.org/2007/opf"><manifest><item id="font" href="Fonts/full.ttf" media-type="application/vnd.ms-opentype"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>`)},
		{"OEBPS/Fonts/full.ttf", []byte("FULL FONT")},
		{"OEBPS/Text/chapter.xhtml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>正文</p></body></html>`)},
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
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
	return output.Bytes()
}
