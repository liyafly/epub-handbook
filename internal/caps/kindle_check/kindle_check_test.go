package kindlecheck

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestCleanFixtureHasZeroFindingsAndAllCounts(t *testing.T) {
	b := openFixture(t, baseFiles())
	defer b.Close()
	result, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != report.StatusComplete || len(result.Findings) != 0 {
		t.Fatalf("status=%s findings=%+v, want complete with no findings", result.Status, result.Findings)
	}
	checks, ok := result.Facts["checks"].([]string)
	if !ok || !slices.Equal(checks, checkIDs()) {
		t.Fatalf("checks=%#v, want ordered Kindle check IDs", result.Facts["checks"])
	}
	counts, ok := result.Facts["counts"].(map[string]int)
	if !ok || len(counts) != len(checks) {
		t.Fatalf("counts=%#v, want all %d check ids", result.Facts["counts"], len(checks))
	}
	for _, id := range checks {
		if counts[id] != 0 {
			t.Errorf("counts[%q]=%d, want 0", id, counts[id])
		}
	}
	if result.Facts["staticOnly"] != true || result.Facts["cssFilesScanned"] != 1 || result.Facts["xhtmlFilesScanned"] != 2 {
		t.Errorf("scan facts=%#v", result.Facts)
	}
	if got := b.ModifiedNames(); len(got) != 0 {
		t.Fatalf("read-only validator modified entries: %v", got)
	}
}

func TestFindingRulesAndLevels(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		level string
		edit  func(map[string]string)
	}{
		{name: "missing ncx item", id: "kindle.ncx-missing", level: "warn", edit: func(files map[string]string) {
			replaceFixture(t, files, "OEBPS/content.opf", `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`, "")
		}},
		{name: "missing spine toc", id: "kindle.ncx-missing", level: "warn", edit: func(files map[string]string) {
			replaceFixture(t, files, "OEBPS/content.opf", ` toc="ncx"`, "")
		}},
		{name: "missing cover item", id: "kindle.cover-image-missing", level: "warn", edit: func(files map[string]string) {
			replaceFixture(t, files, "OEBPS/content.opf", `<item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/>`, "")
		}},
		{name: "wrong cover metadata", id: "kindle.cover-meta-missing", level: "warn", edit: func(files map[string]string) {
			replaceFixture(t, files, "OEBPS/content.opf", `content="cover"`, `content="chapter"`)
		}},
		{name: "svg cover", id: "kindle.cover-not-raster", level: "warn", edit: func(files map[string]string) {
			replaceFixture(t, files, "OEBPS/content.opf", `href="cover.png" media-type="image/png"`, `href="cover.svg" media-type="image/svg+xml"`)
			delete(files, "OEBPS/cover.png")
			files["OEBPS/cover.svg"] = `<svg xmlns="http://www.w3.org/2000/svg"/>`
		}},
		{name: "webp media type", id: "kindle.image-webp", level: "error", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="webp" href="image.webp" media-type="image/webp"/>`, "OEBPS/image.webp")
		}},
		{name: "webp extension", id: "kindle.image-webp", level: "error", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="webp" href="image.webp" media-type="application/octet-stream"/>`, "OEBPS/image.webp")
		}},
		{name: "tiff media type", id: "kindle.image-tiff", level: "warn", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="tiff" href="scan.bin" media-type="image/tiff"/>`, "OEBPS/scan.bin")
		}},
		{name: "tiff extension", id: "kindle.image-tiff", level: "warn", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="tiff" href="scan.tif" media-type="application/octet-stream"/>`, "OEBPS/scan.tif")
		}},
		{name: "gif frames need review", id: "kindle.image-gif", level: "warn", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="gif" href="anim.gif" media-type="image/gif"/>`, "OEBPS/anim.gif")
		}},
		{name: "non-cover svg", id: "kindle.image-svg", level: "info", edit: func(files map[string]string) {
			addManifestImage(files, `<item id="svg" href="diagram.svg" media-type="image/svg+xml"/>`, "OEBPS/diagram.svg")
		}},
		{name: "mathml property missing", id: "kindle.mathml-properties-missing", level: "error", edit: func(files map[string]string) {
			files["OEBPS/chapter.xhtml"] = strings.Replace(files["OEBPS/chapter.xhtml"], `<p id="p1">Text</p>`, `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`, 1)
		}},
		{name: "rotating transform", id: "kindle.css-transform-rotate", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.box { transform: rotate(3deg); }`
		}},
		{name: "webkit rotating transform", id: "kindle.css-transform-rotate", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.box { -webkit-transform: rotate(3deg); }`
		}},
		{name: "underline shorthand enhancement", id: "kindle.css-styled-underline", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.box { text-decoration: underline wavy; }`
		}},
		{name: "style without earlier fallback", id: "kindle.css-styled-underline", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.box { text-decoration-style: wavy; }`
		}},
		{name: "kindle media query", id: "kindle.css-amzn-media-query", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `@media amzn-kf8 { .box { color: red; } }`
		}},
		{name: "kindle mobi media query", id: "kindle.css-amzn-media-query", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `@media amzn-mobi { .box { color: red; } }`
		}},
		{name: "direct img float", id: "kindle.css-img-direct-float", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.x img { float: left; }`
		}},
		{name: "comma selector direct img float", id: "kindle.css-img-direct-float", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `img.icon, p { float: right; }`
		}},
		{name: "font unicode range", id: "kindle.css-unicode-range", level: "info", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `@font-face { font-family: Demo; src: url(demo.woff2); unicode-range: U+0000-00FF; }`
		}},
		{name: "css parse failure", id: "kindle.css-parse-failed", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/styles.css"] = `.broken { color: red;`
			files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], `</manifest>`, `<item id="extra-style" href="extra.css" media-type="text/css"/></manifest>`, 1)
			files["OEBPS/extra.css"] = `.rotation { transform: rotate(2deg); }`
		}},
		{name: "xhtml parse failure", id: "kindle.xhtml-parse-failed", level: "warn", edit: func(files map[string]string) {
			files["OEBPS/chapter.xhtml"] = `<html><body><p>unfinished`
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := baseFiles()
			tt.edit(files)
			result := runCheck(t, files)
			finding, ok := findFinding(result.Findings, tt.id)
			if !ok {
				t.Fatalf("missing finding %q; got %+v", tt.id, result.Findings)
			}
			if finding.Level != tt.level {
				t.Errorf("finding %s level=%s want %s", tt.id, finding.Level, tt.level)
			}
			if tt.id == "kindle.image-gif" && !strings.Contains(finding.Detail, "frame count cannot be determined statically") {
				t.Errorf("GIF detail should require manual frame review, got %q", finding.Detail)
			}
			if tt.id == "kindle.css-parse-failed" && !hasFinding(result.Findings, "kindle.css-transform-rotate") {
				t.Error("valid stylesheet was not inspected after another stylesheet failed to parse")
			}
			if tt.id == "kindle.mathml-properties-missing" && result.Status != report.StatusFailed {
				t.Errorf("error finding status=%q, want failed", result.Status)
			}
			if tt.name == "svg cover" && hasFinding(result.Findings, "kindle.image-svg") {
				t.Error("cover SVG should be checked as cover-not-raster, not as a non-cover SVG")
			}
		})
	}
}

func TestNegativeRulesAndDeterministicOrder(t *testing.T) {
	files := baseFiles()
	files["OEBPS/styles.css"] = `/* @media amzn-kf8 */
@media (min-width: 40em) { figure.img-left { float: left; } }
.ok { transform: /* rotate */ translateX(1px); text-decoration: underline; text-decoration-style: wavy; }`
	files["OEBPS/chapter.xhtml"] = strings.Replace(files["OEBPS/chapter.xhtml"], `<p id="p1">Text</p>`, `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`, 1)
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], `href="chapter.xhtml" media-type="application/xhtml+xml"`, `href="chapter.xhtml" media-type="application/xhtml+xml" properties="mathml"`, 1)
	b := openFixture(t, files)
	defer b.Close()
	first, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"kindle.css-transform-rotate", "kindle.css-styled-underline", "kindle.css-amzn-media-query",
		"kindle.css-img-direct-float", "kindle.mathml-properties-missing", "kindle.css-parse-failed", "kindle.xhtml-parse-failed",
	} {
		if hasFinding(first.Findings, id) {
			t.Errorf("valid or fallback rule produced false positive %q: %+v", id, first.Findings)
		}
	}
	if len(first.Findings) != 0 {
		t.Fatalf("negative fixture findings=%+v, want none", first.Findings)
	}
	second, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("same input produced non-deterministic output\nfirst: %s\nsecond: %s", firstJSON, secondJSON)
	}
	if got := b.ModifiedNames(); len(got) != 0 {
		t.Fatalf("read-only validator modified entries: %v", got)
	}
}

func TestUnderlineFallbackOrdering(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want bool
	}{
		{name: "enhancement after fallback", css: `.x { text-decoration: underline; text-decoration-style: wavy; }`},
		{name: "enhancement before fallback", css: `.x { text-decoration-style: wavy; text-decoration: underline; }`, want: true},
		{name: "underline line fallback", css: `.x { text-decoration-line: underline; text-decoration-style: dotted; }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := baseFiles()
			files["OEBPS/styles.css"] = tt.css
			found := hasFinding(runCheck(t, files).Findings, "kindle.css-styled-underline")
			if found != tt.want {
				t.Fatalf("styled underline finding=%v, want %v", found, tt.want)
			}
		})
	}
}

func TestFindingsFollowCheckPathAndOffsetOrder(t *testing.T) {
	files := baseFiles()
	addManifestImage(files, `<item id="webp" href="image.webp" media-type="image/webp"/>`, "OEBPS/image.webp")
	files["OEBPS/chapter.xhtml"] = strings.Replace(files["OEBPS/chapter.xhtml"], `<p id="p1">Text</p>`, `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`, 1)
	files["OEBPS/styles.css"] = `.start { transform: rotate(1deg); }`
	addManifestStylesheet(files, "z-style", "z.css", `@media amzn-kf8 { .z img { float: left; } }
@font-face { font-family: Demo; src: url(demo.woff2); unicode-range: U+0000-00FF; }`)
	addManifestStylesheet(files, "a-style", "a.css", `.z { transform: rotate(2deg); } .a { transform: rotate(3deg); }`)
	b := openFixture(t, files)
	defer b.Close()
	first, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("same input produced non-deterministic output\nfirst: %s\nsecond: %s", firstJSON, secondJSON)
	}
	wantIDs := []string{
		"kindle.image-webp",
		"kindle.mathml-properties-missing",
		"kindle.css-transform-rotate",
		"kindle.css-transform-rotate",
		"kindle.css-transform-rotate",
		"kindle.css-amzn-media-query",
		"kindle.css-img-direct-float",
		"kindle.css-unicode-range",
	}
	gotIDs := make([]string, len(first.Findings))
	for index, finding := range first.Findings {
		gotIDs[index] = finding.ID
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("finding order=%v, want %v", gotIDs, wantIDs)
	}
	transformLocations := []string{}
	transformDetails := []string{}
	for _, finding := range first.Findings {
		if finding.ID == "kindle.css-transform-rotate" {
			transformLocations = append(transformLocations, finding.Location)
			transformDetails = append(transformDetails, finding.Detail)
		}
	}
	if want := []string{"OEBPS/a.css", "OEBPS/a.css", "OEBPS/styles.css"}; !slices.Equal(transformLocations, want) {
		t.Fatalf("transform locations=%v, want %v", transformLocations, want)
	}
	if !strings.Contains(transformDetails[0], `selector ".z"`) || !strings.Contains(transformDetails[1], `selector ".a"`) {
		t.Fatalf("same-file findings were not offset-sorted: %v", transformDetails)
	}
	if got := b.ModifiedNames(); len(got) != 0 {
		t.Fatalf("read-only validator modified entries: %v", got)
	}
}

func runCheck(t *testing.T, files map[string]string) report.Result {
	t.Helper()
	b := openFixture(t, files)
	defer b.Close()
	result, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if got := b.ModifiedNames(); len(got) != 0 {
		t.Fatalf("read-only validator modified entries: %v", got)
	}
	return result
}

func openFixture(t *testing.T, files map[string]string) *book.Book {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	writeEntry := func(name, content string, method uint16) {
		t.Helper()
		header := &zip.FileHeader{Name: name, Method: method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	writeEntry("mimetype", "application/epub+zip", zip.Store)
	paths := make([]string, 0, len(files))
	for name := range files {
		if name != "mimetype" {
			paths = append(paths, name)
		}
	}
	slices.Sort(paths)
	for _, name := range paths {
		writeEntry(name, files[name], zip.Deflate)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "fixture.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func baseFiles() map[string]string {
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Fixture</dc:title><dc:identifier id="uid">urn:uuid:fixture</dc:identifier><dc:language>en</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/chapter.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title></head><body><h1>Chapter</h1><p id="p1">Text</p></body></html>`,
		"OEBPS/toc.ncx":          `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":        "PNG fixture bytes",
		"OEBPS/styles.css":       `.chapter { color: black; }`,
	}
}

func replaceFixture(t *testing.T, files map[string]string, name, old, replacement string) {
	t.Helper()
	content, ok := files[name]
	if !ok || !strings.Contains(content, old) {
		t.Fatalf("fixture %s does not contain %q", name, old)
	}
	files[name] = strings.Replace(content, old, replacement, 1)
}

func addManifestImage(files map[string]string, manifestEntry, filePath string) {
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</manifest>", manifestEntry+"</manifest>", 1)
	files[filePath] = "image fixture bytes"
}

func addManifestStylesheet(files map[string]string, id, href, content string) {
	item := `<item id="` + id + `" href="` + href + `" media-type="text/css"/>`
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</manifest>", item+"</manifest>", 1)
	files["OEBPS/"+href] = content
}

func findFinding(findings []report.Finding, id string) (report.Finding, bool) {
	for _, finding := range findings {
		if finding.ID == id {
			return finding, true
		}
	}
	return report.Finding{}, false
}

func hasFinding(findings []report.Finding, id string) bool {
	_, ok := findFinding(findings, id)
	return ok
}
