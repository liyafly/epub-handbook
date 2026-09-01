package navaudit

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestNativeFixtureGolden 锁定 nav.audit 的 Go 原生报告和推荐命令。
// 该测试不调用已删除的 Python oracle；golden 只包含稳定的报告字段，
// 不把 t.TempDir() 生成的输入绝对路径写入仓库。
func TestNativeFixtureGolden(t *testing.T) {
	path := writeNativeFixture(t)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	res, err := Run(t.Context(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	legacyRaw, err := json.Marshal(res.Facts["legacyReport"])
	if err != nil {
		t.Fatalf("marshal legacy report: %v", err)
	}
	var legacy struct {
		SuggestedCommands []string `json:"suggested_commands"`
	}
	if err := json.Unmarshal(legacyRaw, &legacy); err != nil {
		t.Fatalf("decode legacy report: %v", err)
	}
	nextCommands := normalizeFixtureCommands(res.NextCommands, path)
	suggestedCommands := normalizeFixtureCommands(legacy.SuggestedCommands, path)

	got := struct {
		Status            string           `json:"status"`
		Summary           any              `json:"summary"`
		Findings          []report.Finding `json:"findings"`
		NextCommands      []string         `json:"nextCommands"`
		SuggestedCommands []string         `json:"suggestedCommands"`
	}{
		Status:            res.Status,
		Summary:           res.Facts["summary"],
		Findings:          res.Findings,
		NextCommands:      nextCommands,
		SuggestedCommands: suggestedCommands,
	}

	wantPath := filepath.Join("..", "..", "..", "testdata", "navaudit", "native-golden.json")
	wantRaw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", wantPath, err)
	}
	var want any
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if diff := diffJSON(t, got, want); diff != "" {
		t.Fatalf("native nav.audit golden mismatch:\n%s", diff)
	}
}

func normalizeFixtureCommands(commands []string, path string) []string {
	quoted := shlexQuote(path)
	out := make([]string, len(commands))
	for i, command := range commands {
		out[i] = strings.ReplaceAll(command, quoted, "<fixture.epub>")
	}
	return out
}

type nativeZipEntry struct {
	name string
	body []byte
}

func writeNativeFixture(t *testing.T) string {
	t.Helper()
	manifest := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` +
		`<item id="css" href="Styles/main.css" media-type="text/css"/>` +
		`<item id="cover-image" href="Images/cover.png" media-type="image/png" properties="cover-image"/>` +
		`<item id="legacy-gif" href="Images/legacy.gif" media-type="image/gif"/>` +
		`<item id="font" href="Fonts/Body.ttf" media-type="font/ttf"/>` +
		`<item id="chapter" href="Text/ch?apter.xhtml" media-type="application/xhtml+xml"/>` +
		`<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`

	entries := []nativeZipEntry{
		{name: "mimetype", body: []byte("application/epub+zip")},
		{name: "META-INF/container.xml", body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`)},
		{name: "OEBPS/content.opf", body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="2.0" unique-identifier="book-id">
  <metadata>
    <dc:identifier id="book-id">urn:uuid:native-fixture</dc:identifier>
    <dc:title>Native fixture</dc:title>
    <dc:language>zh-CN</dc:language>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>` + manifest + `</manifest>
  <spine toc="ncx"><itemref idref="chapter"/></spine>
</package>
`)},
		{name: "OEBPS/nav.xhtml", body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>Contents</title></head>
  <body><nav epub:type="toc"><ol><li><a href="Text/ch?apter.xhtml">Chapter</a></li></ol></nav></body>
</html>
`)},
		{name: "OEBPS/Styles/main.css", body: []byte(`@font-face { font-family: Native; src: url("../Fonts/Missing.ttf"); }
body { font-family: Native, serif; }
`)},
		{name: "OEBPS/Images/cover.png", body: []byte("png")},
		{name: "OEBPS/Images/legacy.gif", body: []byte("gif")},
		{name: "OEBPS/Fonts/Body.ttf", body: []byte("font")},
		{name: "OEBPS/Text/ch?apter.xhtml", body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN" xml:lang="zh-CN">
  <head><title>Chapter</title></head>
  <body><h1>第一章</h1><p>这是 Go 原生 nav.audit fixture。</p></body>
</html>
`)},
		{name: "OEBPS/toc.ncx", body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="Text/ch?apter.xhtml"/></navPoint></navMap></ncx>
`)},
	}

	path := filepath.Join(t.TempDir(), "native-fixture.epub")
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.name == "mimetype" {
			h.Method = zip.Store
		}
		writer, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Ensure the fixture intentionally exercises all conditional command branches.
func TestNativeFixtureShape(t *testing.T) {
	path := writeNativeFixture(t)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %q, want %q", res.Status, report.StatusComplete)
	}
	joined := strings.Join(res.NextCommands, "\n")
	for _, want := range []string{
		"epub run epub.package.nav.audit",
		"epub run epub.layout.audit",
		"epub run epub.notes.popup.normalize",
		"epub redline --check all",
		"epub capabilities --json",
		"epub run epub.structure.normalize",
		"epub run epub.package.migrate.epub3",
		"epub run epub.text.content.analyze",
		"epub run epub.font.coverage.analyze",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("nextCommands 缺少 %q:\n%s", want, joined)
		}
	}
}
