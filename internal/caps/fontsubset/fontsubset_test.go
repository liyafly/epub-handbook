package fontsubset

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestRunReplacesOnlyManifestFontInMemory(t *testing.T) {
	provider := makeReportingProvider(t)
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
	if len(result.Findings) != 1 || result.Findings[0].ID != "font-subset.not-in-master" || result.Findings[0].Level != "warn" {
		t.Fatalf("findings = %+v, want one structured missing-master warning", result.Findings)
	}
	if result.Findings[0].Location != "OEBPS/Fonts/full.ttf" || !strings.Contains(result.Findings[0].Detail, "1 required character") {
		t.Fatalf("finding = %+v, want target and missing count", result.Findings[0])
	}
	data, err := json.Marshal(result.Facts["providerReport"])
	if err != nil {
		t.Fatal(err)
	}
	var providerReport map[string]any
	if err := json.Unmarshal(data, &providerReport); err != nil {
		t.Fatal(err)
	}
	if providerReport["providerVersion"] != "test-1.0" || providerReport["fontToolsVersion"] != "4.test" {
		t.Fatalf("providerReport versions = %#v", providerReport)
	}
	fonts, ok := providerReport["fonts"].([]any)
	if !ok || len(fonts) != 1 {
		t.Fatalf("providerReport fonts = %#v", providerReport["fonts"])
	}
	fontSummary, ok := fonts[0].(map[string]any)
	if !ok || fontSummary["target"] != "OEBPS/Fonts/full.ttf" || fontSummary["notInMasterCount"] != float64(1) {
		t.Fatalf("providerReport font = %#v", fonts[0])
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

	result, err := Run(t.Context(), b, Params{ToolPath: provider})
	if err != nil || result.Status != "failed" || len(result.Findings) != 1 || result.Findings[0].ID != "font-subset.check-failed" {
		t.Fatalf("Run() = result %+v, error %v; want structured provider failure", result, err)
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
	provider := makeReportingProvider(t)
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

func TestRunRejectsModifiedBookState(t *testing.T) {
	b, _ := openFontBook(t)
	defer b.Close()
	chapter, err := b.Current("OEBPS/Text/chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	updated := bytes.Replace(chapter, []byte("正文"), []byte("正文新增"), 1)
	if bytes.Equal(updated, chapter) {
		t.Fatal("test fixture does not contain the expected chapter text")
	}
	if err := b.Apply([]editset.Edit{editset.Replace(
		"OEBPS/Text/chapter.xhtml", 0, int64(len(chapter)), updated,
	)}); err != nil {
		t.Fatal(err)
	}

	result, err := Run(t.Context(), b, Params{})
	if err != nil || result.Status != "failed" || len(result.Findings) != 1 ||
		result.Findings[0].ID != "font-subset.stale-input" {
		t.Fatalf("Run() = result %+v, error %v; want stale-input finding", result, err)
	}
}

func TestRunRejectsInvalidProviderReportsWithoutApplyingCandidate(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "oversized", "wrong-target", "wrong-manifest-id", "wrong-media-type", "wrong-output-sha"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("EPUB_FONT_REPORT_TEST_MODE", mode)
			provider := makeReportingProvider(t)
			b, _ := openFontBook(t)
			defer b.Close()

			result, err := Run(t.Context(), b, Params{ToolPath: provider})
			if err != nil || result.Status != "failed" {
				t.Fatalf("Run() = status %q, error %v; want failed report validation", result.Status, err)
			}
			if len(result.Findings) == 0 || result.Findings[0].ID != "font-subset.report-invalid" {
				t.Fatalf("findings = %+v, want report-invalid", result.Findings)
			}
			font, readErr := b.Current("OEBPS/Fonts/full.ttf")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(font, []byte("FULL FONT")) {
				t.Fatalf("font changed after invalid report: %q", font)
			}
		})
	}
}

func TestRunRejectsProviderReportOmittingManifestFont(t *testing.T) {
	provider := makeProvider(t, omittingFontProviderPython)
	input := filepath.Join(t.TempDir(), "source.epub")
	if err := os.WriteFile(input, fontFixtureWithSecondFont(t), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	result, err := Run(t.Context(), b, Params{ToolPath: provider})
	if err != nil || result.Status != "failed" || len(result.Findings) != 1 ||
		result.Findings[0].ID != "font-subset.unhandled-font" {
		t.Fatalf("Run() = status %q, findings %+v, error %v; want unhandled-font failure", result.Status, result.Findings, err)
	}
	if result.Findings[0].Location != "OEBPS/Fonts/second.ttf" &&
		!strings.Contains(result.Findings[0].Detail, "OEBPS/Fonts/second.ttf") {
		t.Fatalf("unhandled-font finding = %+v, want the omitted font path", result.Findings[0])
	}
	for path, want := range map[string][]byte{
		"OEBPS/Fonts/full.ttf":   []byte("FULL FONT"),
		"OEBPS/Fonts/second.ttf": []byte("SECOND FONT"),
	} {
		got, readErr := b.Current(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed after omitted-font failure: got %q, want %q", path, got, want)
		}
	}
}

func TestRunRejectsUnmanifestedFontEntry(t *testing.T) {
	input := filepath.Join(t.TempDir(), "source.epub")
	if err := os.WriteFile(input, fontFixtureWithUnmanifestedFont(t), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	result, err := Run(t.Context(), b, Params{ToolPath: filepath.Join(t.TempDir(), "missing-provider")})
	if err != nil || result.Status != "failed" || len(result.Findings) != 1 ||
		result.Findings[0].ID != "font-subset.unmanifested-font" {
		t.Fatalf("Run() = status %q, findings %+v, error %v; want unmanifested-font failure", result.Status, result.Findings, err)
	}
	if !strings.Contains(result.Findings[0].Detail, "OEBPS/Fonts/unlisted-master.ttf") {
		t.Fatalf("unmanifested-font finding = %+v, want the unlisted font path", result.Findings[0])
	}
	got, err := b.Current("OEBPS/Fonts/unlisted-master.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("UNLISTED MASTER")) {
		t.Fatalf("unmanifested font changed: got %q", got)
	}
}

func TestRunProviderCancellationReturnsCancelledErrorWithoutApplyingCandidate(t *testing.T) {
	provider := makeProvider(t, "import time; time.sleep(30)")
	b, _ := openFontBook(t)
	defer b.Close()

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := Run(ctx, b, Params{ToolPath: provider})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	font, readErr := b.Current("OEBPS/Fonts/full.ttf")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(font, []byte("FULL FONT")) {
		t.Fatalf("font changed after cancellation: %q", font)
	}
}

func makeReportingProvider(t *testing.T) string {
	t.Helper()
	return makeProvider(t, reportingProviderPython)
}

const reportingProviderPython = `
import hashlib, json, os, sys, zipfile
from pathlib import Path
source = Path(sys.argv[2])
output = Path(sys.argv[sys.argv.index("--out") + 1])
target = "OEBPS/Fonts/full.ttf"
with zipfile.ZipFile(source) as src, zipfile.ZipFile(output, "w") as dst:
    for entry in src.infolist():
        data = b"SUBSET FONT" if entry.filename == target else src.read(entry)
        dst.writestr(entry, data)
source_font = zipfile.ZipFile(source).read(target)
output_font = zipfile.ZipFile(output).read(target)
sha = lambda data: hashlib.sha256(data).hexdigest()
report = {
    "schemaVersion": 1,
    "tool": "epub-font subset",
    "providerVersion": "test-1.0",
    "fontTools": "4.test",
    "input": {"path": str(source), "sha256": sha(source.read_bytes())},
    "config": None,
    "charset": {"total": 2, "bySource": {"text": 2}},
    "fonts": [{
        "target": target, "manifestId": "font", "mediaType": "application/vnd.ms-opentype",
        "action": "subset",
        "master": {"source": "epub:" + target, "sha256": sha(source_font), "bytes": len(source_font),
                   "glyphs": 12, "outline": "glyf", "axes": []},
        "variation": {"mode": "keep", "axes": {}},
        "original": {"sha256": sha(source_font), "bytes": len(source_font)},
        "output": {"sha256": sha(output_font), "bytes": len(output_font), "glyphs": 11,
                   "outline": "glyf", "flavor": None, "axes": [], "tables": ["cmap", "glyf"]},
        "requiredCodepoints": 2, "notInMaster": ["U+9F98 龘"], "notInMasterCount": 1,
        "checks": {"cmap-coverage": {"ok": True, "wanted": 2, "present": 1}},
        "ok": True,
        "warnings": [target + ": 1 required characters are not in the master font (fallback fonts must cover them)"]
    }],
    "ok": True,
    "output": {"path": str(output), "sha256": sha(output.read_bytes()), "warnings": []}
}
mode = os.environ.get("EPUB_FONT_REPORT_TEST_MODE", "valid")
report_path = output.with_name(output.stem + ".font-report.json")
if mode == "missing":
    sys.exit(0)
if mode == "corrupt":
    report_path.write_text("{", encoding="utf-8")
    sys.exit(0)
if mode == "oversized":
    report_path.write_bytes(b" " * (8 * 1024 * 1024 + 1))
    sys.exit(0)
if mode == "wrong-target":
    report["fonts"][0]["target"] = "OEBPS/Fonts/unlisted.ttf"
if mode == "wrong-manifest-id":
    report["fonts"][0]["manifestId"] = "not-the-font-item"
if mode == "wrong-media-type":
    report["fonts"][0]["mediaType"] = "application/xhtml+xml"
if mode == "wrong-output-sha":
    report["output"]["sha256"] = "0" * 64
report_path.write_text(json.dumps(report), encoding="utf-8")
`

const omittingFontProviderPython = `
import hashlib, json, sys, zipfile
from pathlib import Path
source = Path(sys.argv[2])
output = Path(sys.argv[sys.argv.index("--out") + 1])
target = "OEBPS/Fonts/full.ttf"
with zipfile.ZipFile(source) as src, zipfile.ZipFile(output, "w") as dst:
    for entry in src.infolist():
        dst.writestr(entry, src.read(entry))
source_bytes = source.read_bytes()
font = zipfile.ZipFile(source).read(target)
sha = lambda data: hashlib.sha256(data).hexdigest()
report = {
    "schemaVersion": 1,
    "tool": "epub-font subset",
    "providerVersion": "test-1.0",
    "fontTools": "4.test",
    "input": {"path": str(source), "sha256": sha(source_bytes)},
    "config": None,
    "charset": {"total": 2, "bySource": {"text": 2}},
    "fonts": [{
        "target": target, "manifestId": "font", "mediaType": "application/vnd.ms-opentype",
        "action": "subset",
        "master": {"source": "epub:" + target, "sha256": sha(font), "bytes": len(font),
                   "glyphs": 12, "outline": "glyf", "axes": []},
        "variation": {"mode": "keep", "axes": {}},
        "original": {"sha256": sha(font), "bytes": len(font)},
        "output": {"sha256": sha(font), "bytes": len(font), "glyphs": 12,
                   "outline": "glyf", "flavor": None, "axes": [], "tables": ["cmap", "glyf"]},
        "requiredCodepoints": 2, "notInMaster": [], "notInMasterCount": 0,
        "checks": {"cmap-coverage": {"ok": True, "wanted": 2, "present": 2}},
        "ok": True,
        "warnings": []
    }],
    "ok": True,
    "output": {"path": str(output), "sha256": sha(output.read_bytes()), "warnings": []}
}
output.with_name(output.stem + ".font-report.json").write_text(json.dumps(report), encoding="utf-8")
`

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
	return makeFontFixture(t, false, false)
}

func fontFixtureWithSecondFont(t *testing.T) []byte {
	return makeFontFixture(t, true, false)
}

func fontFixtureWithUnmanifestedFont(t *testing.T) []byte {
	return makeFontFixture(t, false, true)
}

func makeFontFixture(t *testing.T, includeSecondFont, includeUnmanifestedFont bool) []byte {
	t.Helper()
	opf := `<package xmlns="http://www.idpf.org/2007/opf"><manifest><item id="font" href="Fonts/full.ttf" media-type="application/vnd.ms-opentype"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>`
	if includeSecondFont {
		opf = strings.Replace(opf, `<item id="font"`, `<item id="second-font" href="Fonts/second.ttf" media-type="application/vnd.ms-opentype"/><item id="font"`, 1)
	}
	files := []struct {
		name string
		data []byte
	}{
		{"mimetype", []byte("application/epub+zip")},
		{"META-INF/container.xml", []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf"/></rootfiles></container>`)},
		{"OEBPS/package.opf", []byte(opf)},
		{"OEBPS/Fonts/full.ttf", []byte("FULL FONT")},
		{"OEBPS/Text/chapter.xhtml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>正文</p></body></html>`)},
	}
	if includeSecondFont {
		files = append(files, struct {
			name string
			data []byte
		}{"OEBPS/Fonts/second.ttf", []byte("SECOND FONT")})
	}
	if includeUnmanifestedFont {
		files = append(files, struct {
			name string
			data []byte
		}{"OEBPS/Fonts/unlisted-master.ttf", []byte("UNLISTED MASTER")})
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
