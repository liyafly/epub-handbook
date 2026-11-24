package navaudit

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// packMediaOverlay 用 testdata/media_overlay 的能力 golden 输出拼一本带有声同步的书。
func packMediaOverlay(t *testing.T, edit map[string]func(string) string) string {
	t.Helper()
	golden := filepath.Join("..", "..", "..", "testdata", "media_overlay")
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(golden, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      read("OEBPS_content.opf"),
		"OEBPS/one.xhtml":        read("OEBPS_one.xhtml"),
		"OEBPS/two.xhtml":        read("OEBPS_two.xhtml"),
		"OEBPS/Smil/one.smil":    read("OEBPS_Smil_one.smil"),
		"OEBPS/Smil/two.smil":    read("OEBPS_Smil_two.smil"),
		"OEBPS/styles.css":       read("OEBPS_styles.css"),
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="one.xhtml">One</a></li><li><a href="two.xhtml">Two</a></li></ol></nav></body></html>`,
		"OEBPS/cover.png":        "PNG",
		"OEBPS/Audio/one.mp3":    "ID3",
		"OEBPS/Audio/two.mp3":    "ID3",
	}
	for name, f := range edit {
		files[name] = f(files[name])
	}
	out := filepath.Join(t.TempDir(), "mo.epub")
	fh, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(fh)
	wr, _ := w.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	wr.Write([]byte(files["mimetype"]))
	for name, data := range files {
		if name == "mimetype" {
			continue
		}
		wr, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		wr.Write([]byte(data))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	return out
}

func mediaOverlayFindings(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	for _, f := range runNativeAudit(t, path).Findings {
		if strings.HasPrefix(f.Detail, "media-overlay") {
			out = append(out, f.Level+" "+f.Detail+" "+f.Title)
		}
	}
	return out
}

func TestMediaOverlayCleanBook(t *testing.T) {
	if got := mediaOverlayFindings(t, packMediaOverlay(t, nil)); len(got) != 0 {
		t.Fatalf("clean read-along fixture: %v", got)
	}
}

func TestMediaOverlayProblems(t *testing.T) {
	path := packMediaOverlay(t, map[string]func(string) string{
		"OEBPS/Smil/one.smil": func(s string) string {
			s = strings.Replace(s, "#mo-one-s2", "#no-such-id", 1)
			s = strings.Replace(s, `clipBegin="5.600s" clipEnd="13.400s"`, `clipBegin="13.400s" clipEnd="5.600s"`, 1)
			return strings.Replace(s, "../Audio/one.mp3\" clipBegin=\"26.800s\"", "../Audio/gone.mp3\" clipBegin=\"26.800s\"", 1)
		},
		"OEBPS/content.opf": func(s string) string {
			return strings.Replace(s, `refines="#mo-smil-two">0:00:37.900`, `refines="#mo-smil-two">0:05:00.000`, 1)
		},
	})
	got := strings.Join(mediaOverlayFindings(t, path), "\n")
	for _, want := range []string{
		"error media-overlay-clip text@src does not resolve: ../one.xhtml#no-such-id",
		"is not a positive time range",
		"error media-overlay-clip audio@src is missing from the package: ../Audio/gone.mp3",
		"warn media-overlay-duration media:duration 0:05:00.000 differs",
		"warn media-overlay-duration total media:duration",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestParseClock(t *testing.T) {
	for in, want := range map[string]float64{"12.5s": 12.5, "300ms": 0.3, "1.5min": 90, "0:01:02.5": 62.5, "01:02": 62, "7": 7, "0.5h": 1800} {
		if got, err := parseClock(in); err != nil || got != want {
			t.Errorf("parseClock(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseClock("soon"); err == nil {
		t.Error("bad clock accepted")
	}
}
