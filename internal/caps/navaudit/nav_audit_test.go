package navaudit

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestLooseScanRetainsSourceAndHonorsCancellation(t *testing.T) {
	data := []byte(`<html lang="zh-CN"><body><p>one <b>two</b></p><svg xmlns="http://www.w3.org/2000/svg"/></body></html>`)
	doc, err := parseXHTMLLoose(t.Context(), data)
	if err != nil {
		t.Fatal(err)
	}
	if doc.rawText != string(data) || doc.rootAttrs["lang"] != "zh-CN" {
		t.Fatalf("source projection was lost: %+v", doc)
	}
	if doc.elements[1].local != "p" || doc.elements[1].text != "one two" {
		t.Fatalf("nested visible text changed: %+v", doc.elements)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if doc, err := parseXHTMLLoose(ctx, data); doc != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled parse: doc=%v err=%v", doc, err)
	}
}

// stubProbe 返回一个固定结果的 toolProbe：测试绝不能读开发机 PATH，
// 否则 `brew install epubcheck` 会让 golden 无故变红。
func stubProbe(available bool) toolProbe {
	return func(string) bool { return available }
}

// TestNativeFixtureGolden 锁定 nav.audit 的 Go 原生报告和推荐命令。
// 该测试不调用已删除的 Python oracle；golden 只包含稳定的报告字段，
// 不把 t.TempDir() 生成的输入绝对路径写入仓库。
// 外部工具探测被固定为「不可用」，使 golden 与本机 PATH 无关。
func TestNativeFixtureGolden(t *testing.T) {
	path := writeNativeFixture(t)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	nextCommands := normalizeFixtureCommands(res.NextCommands, path)

	got := struct {
		Status             string           `json:"status"`
		AuditStatus        any              `json:"auditStatus"`
		Summary            any              `json:"summary"`
		Findings           []report.Finding `json:"findings"`
		FindingsByLevel    any              `json:"findingsByLevel"`
		RecommendedSkills  any              `json:"recommendedSkills"`
		ToolAvailability   any              `json:"toolAvailability"`
		ActionableFindings any              `json:"actionableFindings"`
		NextCommands       []string         `json:"nextCommands"`
	}{
		Status:             res.Status,
		AuditStatus:        res.Facts["auditStatus"],
		Summary:            res.Facts["summary"],
		Findings:           res.Findings,
		FindingsByLevel:    res.Facts["findingsByLevel"],
		RecommendedSkills:  res.Facts["recommendedSkills"],
		ToolAvailability:   res.Facts["toolAvailability"],
		ActionableFindings: res.Facts["actionableFindings"],
		NextCommands:       nextCommands,
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

func TestActionableMissingHTMLLangAppearsInFindings(t *testing.T) {
	path := writeNativeFixture(t)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Facts["auditStatus"]; got == "pass" {
		t.Fatalf("auditStatus = %v, want warning or failure for missing html lang", got)
	}
	for _, finding := range res.Findings {
		if finding.Detail == "missing-html-lang" {
			return
		}
	}
	t.Fatalf("findings do not include missing-html-lang: %+v", res.Findings)
}

func TestAllClearInfoRequiresNoActionableFindings(t *testing.T) {
	t.Run("all clear", func(t *testing.T) {
		ins := &inspector{}
		ins.addActionableFindings(nil)
		if len(ins.findings) != 1 || ins.findings[0].Level != "info" {
			t.Fatalf("all-clear findings = %+v, want one info finding", ins.findings)
		}
	})

	t.Run("actionable issue", func(t *testing.T) {
		ins := &inspector{}
		ins.addActionableFindings([]detectorFinding{{Kind: "missing-html-lang", File: "nav.xhtml"}})
		if len(ins.findings) != 1 || ins.findings[0].Level != "warn" || ins.findings[0].Kind != "missing-html-lang" {
			t.Fatalf("actionable findings = %+v, want only a warning for missing-html-lang", ins.findings)
		}
	})
}

func normalizeFixtureCommands(commands []string, path string) []string {
	quoted := report.ShellQuote(path)
	out := make([]string, len(commands))
	for i, command := range commands {
		out[i] = strings.ReplaceAll(command, quoted, "fixture.epub")
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
  <body><nav epub:type="toc" id="contents"><ol><li><a href="Text/ch%3Fapter.xhtml?mode=print#chapter-heading">Chapter</a></li></ol></nav></body>
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
  <head><title>Chapter</title><link rel="stylesheet" href="../Styles/main.css"/></head>
  <body><h1 id="chapter-heading">第一章</h1><img src="../Images/cover.png"/><p id="章节">这是 Go 原生 nav.audit fixture。</p><p xml:id="xml-legacy">XML ID.</p><a href="#chapter-heading">回到标题</a><a href="../nav.xhtml#contents">目录</a></body>
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
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
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
	if strings.Contains(joined, "epub redline --check all") {
		t.Errorf("nav.audit must not suggest redline with unspecified before/after paths:\n%s", joined)
	}
}

// TestEmptySpineIsErrorInPreflightOnly 锁定 spine 特判：preflight 族在 spine 为空时
// 追加一条 error finding 并置 failed；layout-audit 族不做此特判。
func TestEmptySpineIsErrorInPreflightOnly(t *testing.T) {
	path := writeNativeFixture(t)
	noSpine := filepath.Join(t.TempDir(), "no-spine.epub")
	rewriteZipEntry(t, path, noSpine, "OEBPS/content.opf", func(data []byte) []byte {
		return bytes.Replace(data, []byte(`<spine toc="ncx"><itemref idref="chapter"/></spine>`), []byte(`<spine toc="ncx"></spine>`), 1)
	})

	for _, tc := range []struct {
		name       string
		params     Params
		wantStatus string
		wantSpine  bool
	}{
		{"preflight", Params{}, report.StatusFailed, true},
		{"layout-audit", Params{Report: "layout-audit"}, report.StatusComplete, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := book.Open(noSpine)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			res, err := run(t.Context(), b, tc.params, stubProbe(false))
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", res.Status, tc.wantStatus)
			}
			found := false
			for _, f := range res.Findings {
				if f.Title == "OPF spine is missing or empty" && f.Level == "error" {
					found = true
				}
			}
			if found != tc.wantSpine {
				t.Errorf("spine finding present = %v, want %v\n%+v", found, tc.wantSpine, res.Findings)
			}
			levels := res.Facts["findingsByLevel"].(findingsByLevel)
			gotErrors := 0
			for _, f := range res.Findings {
				if f.Level == "error" {
					gotErrors++
				}
			}
			if levels.Error != gotErrors {
				t.Errorf("findingsByLevel.error = %d, want %d", levels.Error, gotErrors)
			}
			if got := res.Facts["auditStatus"]; (got == "fail") != tc.wantSpine {
				t.Errorf("auditStatus = %v", got)
			}
		})
	}
}

// rewriteZipEntry 复制 zip 并用 fn 改写指定 entry。
func rewriteZipEntry(t *testing.T, src, dst, entry string, fn func([]byte) []byte) {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == entry {
			data = fn(data)
		}
		h := &zip.FileHeader{Name: f.Name, Method: f.Method}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addZipEntry(t *testing.T, src, dst, name string, body []byte) {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: f.Name, Method: f.Method}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runNativeAudit(t *testing.T, path string) report.Result {
	t.Helper()
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func requireFindingKind(t *testing.T, res report.Result, kind string) report.Finding {
	t.Helper()
	for _, finding := range res.Findings {
		if finding.Detail == kind {
			return finding
		}
	}
	t.Fatalf("finding kind %q not found: %+v", kind, res.Findings)
	return report.Finding{}
}

func TestXHTMLLocalResourceTargets(t *testing.T) {
	chapter := "OEBPS/Text/ch?apter.xhtml"
	for _, tc := range []struct {
		name       string
		mutate     func(t *testing.T, path string) string
		wantKind   string
		wantTitle  string
		wantInPath []string
	}{
		{
			name: "missing image",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "broken-image.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`src="../Images/cover.png"`), []byte(`src="../Images/missing.png"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-target", wantTitle: "image target is missing",
			wantInPath: []string{chapter, `"../Images/missing.png"`, "OEBPS/Images/missing.png"},
		},
		{
			name: "missing stylesheet",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "broken-stylesheet.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="../Styles/main.css"`), []byte(`href="../Styles/missing.css"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-target", wantTitle: "stylesheet target is missing",
			wantInPath: []string{chapter, `"../Styles/missing.css"`, "OEBPS/Styles/missing.css"},
		},
		{
			name: "missing hyperlink target",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "broken-link.epub")
				rewriteZipEntry(t, path, dst, "OEBPS/nav.xhtml", func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="Text/ch%3Fapter.xhtml?mode=print#chapter-heading"`), []byte(`href="Text/missing.xhtml"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-target", wantTitle: "link target is missing",
			wantInPath: []string{"OEBPS/nav.xhtml", `"Text/missing.xhtml"`, "OEBPS/Text/missing.xhtml"},
		},
		{
			name: "image exists but has no manifest item",
			mutate: func(t *testing.T, path string) string {
				withImage := filepath.Join(t.TempDir(), "unmanifested-image-source.epub")
				addZipEntry(t, path, withImage, "OEBPS/Images/unlisted.png", []byte("png"))
				dst := filepath.Join(t.TempDir(), "unmanifested-image.epub")
				rewriteZipEntry(t, withImage, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`src="../Images/cover.png"`), []byte(`src="../Images/unlisted.png"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-manifest-item", wantTitle: "missing from the OPF manifest",
			wantInPath: []string{chapter, `"../Images/unlisted.png"`, "OEBPS/Images/unlisted.png"},
		},
		{
			name: "stylesheet exists but has no manifest item",
			mutate: func(t *testing.T, path string) string {
				withCSS := filepath.Join(t.TempDir(), "unmanifested-css-source.epub")
				addZipEntry(t, path, withCSS, "OEBPS/Styles/unlisted.css", []byte("body {}"))
				dst := filepath.Join(t.TempDir(), "unmanifested-css.epub")
				rewriteZipEntry(t, withCSS, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="../Styles/main.css"`), []byte(`href="../Styles/unlisted.css"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-manifest-item", wantTitle: "missing from the OPF manifest",
			wantInPath: []string{chapter, `"../Styles/unlisted.css"`, "OEBPS/Styles/unlisted.css"},
		},
		{
			name: "stylesheet manifest item has wrong media type",
			mutate: func(t *testing.T, path string) string {
				withItem := filepath.Join(t.TempDir(), "wrong-css-type-source.epub")
				rewriteZipEntry(t, path, withItem, "OEBPS/content.opf", func(data []byte) []byte {
					return bytes.Replace(data, []byte(`media-type="text/css"`), []byte(`media-type="application/octet-stream"`), 1)
				})
				return withItem
			},
			wantKind: "xhtml-stylesheet-manifest-type", wantTitle: "no text/css OPF manifest declaration",
			wantInPath: []string{chapter, `"../Styles/main.css"`, "OEBPS/Styles/main.css"},
		},
		{
			name: "escaping path",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "escaping-path.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`src="../Images/cover.png"`), []byte(`src="../../../outside.png"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-invalid-target", wantTitle: "target is invalid",
			wantInPath: []string{chapter, `"../../../outside.png"`, "<invalid>"},
		},
		{
			name: "unsupported base URL",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "base-url.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte("</head>"), []byte(`<base href="../"/></head>`), 1)
				})
				return dst
			},
			wantKind: "xhtml-base-unsupported", wantTitle: "base URL semantics are not supported",
			wantInPath: []string{chapter, `"../"`, "<unresolved>"},
		},
		{
			name: "unsupported xml base URL",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "xml-base-url.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`xml:lang="zh-CN"`), []byte(`xml:base="../" xml:lang="zh-CN"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-base-unsupported", wantTitle: "base URL semantics are not supported",
			wantInPath: []string{chapter, `"../"`, "<unresolved>"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.mutate(t, writeNativeFixture(t))
			res := runNativeAudit(t, path)
			if res.Status != report.StatusFailed || res.Facts["auditStatus"] != "fail" {
				t.Fatalf("audit status = %q/%v, want failed/fail", res.Status, res.Facts["auditStatus"])
			}
			finding := requireFindingKind(t, res, tc.wantKind)
			if !strings.Contains(finding.Title, tc.wantTitle) {
				t.Errorf("finding title = %q, want to contain %q", finding.Title, tc.wantTitle)
			}
			for _, want := range tc.wantInPath {
				if !strings.Contains(finding.Location, want) {
					t.Errorf("finding location %q does not contain %q", finding.Location, want)
				}
			}
		})
	}
}

func TestXHTMLResourceScannerIgnoresExternalAndNonElementMarkup(t *testing.T) {
	path := writeNativeFixture(t)
	withExternal := filepath.Join(t.TempDir(), "external-and-encoded.epub")
	rewriteZipEntry(t, path, withExternal, "OEBPS/Text/ch?apter.xhtml", func(data []byte) []byte {
		data = bytes.Replace(data, []byte(`src="../Images/cover.png"`), []byte(`src="../Images/cover%2Epng?download=1&amp;lang=zh"`), 1)
		return bytes.Replace(data, []byte("</body>"), []byte(`<!-- <img src="../Images/comment-missing.png"/> -->
  <script><![CDATA[<a href="missing-script.xhtml">not markup</a> <img src="missing-script.png"/>]]></script>
  <img src="https://example.test/remote.png"/><a href="mailto:reader@example.test">mail</a></body>`), 1)
	})
	res := runNativeAudit(t, withExternal)
	for _, finding := range res.Findings {
		if strings.HasPrefix(finding.Detail, "xhtml-") {
			t.Errorf("unexpected XHTML reference finding: %+v", finding)
		}
	}
}

func TestXHTMLResourceTargetResolvesEncodedUnicodeAndSpaces(t *testing.T) {
	path := writeNativeFixture(t)
	withImage := filepath.Join(t.TempDir(), "encoded-image-source.epub")
	addZipEntry(t, path, withImage, "OEBPS/Images/章节 封面.png", []byte("png"))
	withManifest := filepath.Join(t.TempDir(), "encoded-image-manifest.epub")
	rewriteZipEntry(t, withImage, withManifest, "OEBPS/content.opf", func(data []byte) []byte {
		return bytes.Replace(data, []byte("</manifest>"), []byte(`<item id="encoded-image" href="Images/章节 封面.png" media-type="image/png"/></manifest>`), 1)
	})
	withReference := filepath.Join(t.TempDir(), "encoded-image-reference.epub")
	rewriteZipEntry(t, withManifest, withReference, "OEBPS/Text/ch?apter.xhtml", func(data []byte) []byte {
		return bytes.Replace(data, []byte(`src="../Images/cover.png"`),
			[]byte(`src="../Images/%E7%AB%A0%E8%8A%82%20%E5%B0%81%E9%9D%A2.png?download=1&amp;lang=zh"`), 1)
	})
	res := runNativeAudit(t, withReference)
	for _, finding := range res.Findings {
		if strings.HasPrefix(finding.Detail, "xhtml-") {
			t.Errorf("unexpected XHTML reference finding: %+v", finding)
		}
	}
}

func TestXHTMLFragmentTargets(t *testing.T) {
	chapter := "OEBPS/Text/ch?apter.xhtml"
	navLink := []byte(`href="Text/ch%3Fapter.xhtml?mode=print#chapter-heading"`)
	for _, tc := range []struct {
		name      string
		mutate    func(t *testing.T, path string) string
		wantKind  string
		wantLevel string
		wantTitle string
		wantPath  []string
	}{
		{
			name: "missing same-file fragment",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "missing-same-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#absent"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-fragment", wantLevel: "error", wantTitle: "fragment target ID is missing",
			wantPath: []string{chapter, `"#absent"`, chapter + "#absent"},
		},
		{
			name: "missing cross-file fragment",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "missing-cross-fragment.epub")
				rewriteZipEntry(t, path, dst, "OEBPS/nav.xhtml", func(data []byte) []byte {
					return bytes.Replace(data, navLink, []byte(`href="Text/ch%3Fapter.xhtml?mode=print#absent"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-missing-fragment", wantLevel: "error", wantTitle: "fragment target ID is missing",
			wantPath: []string{"OEBPS/nav.xhtml", `"Text/ch%3Fapter.xhtml?mode=print#absent"`, chapter + "#absent"},
		},
		{
			name: "percent-encoded unicode ID",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "encoded-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#%E7%AB%A0%E8%8A%82"`), 1)
				})
				return dst
			},
		},
		{
			name: "xml id",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "xml-id-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#xml-legacy"`), 1)
				})
				return dst
			},
		},
		{
			name: "empty fragment",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "empty-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#"`), 1)
				})
				return dst
			},
		},
		{
			name: "invalid fragment encoding",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "invalid-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#%ZZ"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-invalid-fragment", wantLevel: "error", wantTitle: "invalid percent-encoding",
			wantPath: []string{chapter, `"#%ZZ"`, "<invalid-fragment>"},
		},
		{
			name: "EPUB CFI is reported as unverified",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "cfi-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="#epubcfi(/6/2[chapter])"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-cfi-unverified", wantLevel: "warn", wantTitle: "not resolved by this audit",
			wantPath: []string{chapter, `"#epubcfi(/6/2[chapter])"`},
		},
		{
			name: "fragment on non-XHTML target is outside ID validation",
			mutate: func(t *testing.T, path string) string {
				dst := filepath.Join(t.TempDir(), "non-xhtml-fragment.epub")
				rewriteZipEntry(t, path, dst, chapter, func(data []byte) []byte {
					return bytes.Replace(data, []byte(`href="#chapter-heading"`), []byte(`href="../Images/cover.png#figure"`), 1)
				})
				return dst
			},
		},
		{
			name: "unmanifested XHTML fragment target",
			mutate: func(t *testing.T, path string) string {
				withTarget := filepath.Join(t.TempDir(), "unmanifested-xhtml-source.epub")
				addZipEntry(t, path, withTarget, "OEBPS/Text/unlisted.xhtml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="there">target</p></body></html>`))
				dst := filepath.Join(t.TempDir(), "unmanifested-xhtml.epub")
				rewriteZipEntry(t, withTarget, dst, "OEBPS/nav.xhtml", func(data []byte) []byte {
					return bytes.Replace(data, navLink, []byte(`href="Text/unlisted.xhtml#there"`), 1)
				})
				return dst
			},
			wantKind: "xhtml-fragment-target-unverified", wantLevel: "error", wantTitle: "not declared as a readable XHTML manifest item",
			wantPath: []string{"OEBPS/nav.xhtml", `"Text/unlisted.xhtml#there"`, "OEBPS/Text/unlisted.xhtml#there"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.mutate(t, writeNativeFixture(t))
			res := runNativeAudit(t, path)
			var found *report.Finding
			for i := range res.Findings {
				if res.Findings[i].Detail == tc.wantKind {
					found = &res.Findings[i]
					break
				}
			}
			if tc.wantKind == "" {
				for _, finding := range res.Findings {
					if strings.HasPrefix(finding.Detail, "xhtml-") {
						t.Errorf("unexpected XHTML fragment finding: %+v", finding)
					}
				}
				return
			}
			if found == nil {
				t.Fatalf("finding kind %q not found: %+v", tc.wantKind, res.Findings)
			}
			if found.Level != tc.wantLevel || !strings.Contains(found.Title, tc.wantTitle) {
				t.Errorf("finding = %+v, want level %q and title containing %q", *found, tc.wantLevel, tc.wantTitle)
			}
			for _, want := range tc.wantPath {
				if !strings.Contains(found.Location, want) {
					t.Errorf("finding location %q does not contain %q", found.Location, want)
				}
			}
		})
	}
}

func TestXHTMLParseFailureIsAnAuditError(t *testing.T) {
	path := writeNativeFixture(t)
	broken := filepath.Join(t.TempDir(), "broken-xhtml.epub")
	rewriteZipEntry(t, path, broken, "OEBPS/Text/ch?apter.xhtml", func([]byte) []byte {
		return []byte(`<html><body><img src="missing.png"></body>`)
	})
	res := runNativeAudit(t, broken)
	finding := requireFindingKind(t, res, "xhtml-parse-error")
	if res.Status != report.StatusFailed || res.Facts["auditStatus"] != "fail" {
		t.Fatalf("audit status = %q/%v, want failed/fail", res.Status, res.Facts["auditStatus"])
	}
	if finding.Location != "OEBPS/Text/ch?apter.xhtml" {
		t.Errorf("parse error location = %q", finding.Location)
	}
}

func TestNavAuditFlagsUndefinedEntity(t *testing.T) {
	path := writeNativeFixture(t)
	broken := filepath.Join(t.TempDir(), "undefined-entity.epub")
	rewriteZipEntry(t, path, broken, "OEBPS/Text/ch?apter.xhtml", func(data []byte) []byte {
		return bytes.Replace(data, []byte("这是 Go 原生 nav.audit fixture。"), []byte("这是 &nbsp; Go 原生 nav.audit fixture。"), 1)
	})
	res := runNativeAudit(t, broken)
	if res.Status != report.StatusFailed || res.Facts["auditStatus"] != "fail" {
		t.Fatalf("audit status = %q/%v, want failed/fail", res.Status, res.Facts["auditStatus"])
	}
	finding := requireFindingKind(t, res, "xhtml-not-well-formed")
	if finding.Location != "OEBPS/Text/ch?apter.xhtml" {
		t.Errorf("finding location = %q", finding.Location)
	}
}

func TestNavAuditAcceptsEntitiesDeclaredByXHTML11Doctype(t *testing.T) {
	path := writeNativeFixture(t)
	withDTD := filepath.Join(t.TempDir(), "xhtml11-entities.epub")
	doctype := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">`
	rewriteZipEntry(t, path, withDTD, "OEBPS/Text/ch?apter.xhtml", func(data []byte) []byte {
		data = bytes.Replace(data,
			[]byte(`<?xml version="1.0" encoding="UTF-8"?>`),
			[]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+doctype), 1)
		return bytes.Replace(data, []byte("这是 Go 原生 nav.audit fixture。"),
			[]byte("这是&nbsp;Go 原生&mdash; nav.audit fixture。"), 1)
	})
	res := runNativeAudit(t, withDTD)
	for _, finding := range res.Findings {
		if finding.Detail == "xhtml-not-well-formed" {
			t.Fatalf("XHTML 1.1 DTD entities should be resolved for strict parsing: %+v", finding)
		}
	}
}

func TestNavAuditAcceptsEntitiesDeclaredByXHTML10Doctype(t *testing.T) {
	path := writeNativeFixture(t)
	withDTD := filepath.Join(t.TempDir(), "xhtml10-entities.epub")
	doctype := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">`
	rewriteZipEntry(t, path, withDTD, "OEBPS/Text/ch?apter.xhtml", func(data []byte) []byte {
		data = bytes.Replace(data,
			[]byte(`<?xml version="1.0" encoding="UTF-8"?>`),
			[]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+doctype), 1)
		return bytes.Replace(data, []byte("这是 Go 原生 nav.audit fixture。"),
			[]byte("这是&nbsp;Go 原生&mdash; nav.audit fixture。"), 1)
	})
	res := runNativeAudit(t, withDTD)
	for _, finding := range res.Findings {
		if finding.Detail == "xhtml-not-well-formed" {
			t.Fatalf("XHTML 1.0 DTD entities should be accepted for EPUB2 strict parsing: %+v", finding)
		}
	}
}

func TestNavAuditFlagsMalformedNavDocument(t *testing.T) {
	path := writeNativeFixture(t)
	malformed := filepath.Join(t.TempDir(), "malformed-nav.epub")
	rewriteZipEntry(t, path, malformed, "OEBPS/nav.xhtml", func(data []byte) []byte {
		return bytes.Replace(data, []byte("<title>Contents</title>"), []byte("<title>Contents&nbsp;</title>"), 1)
	})
	res := runNativeAudit(t, malformed)
	if res.Status != report.StatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	for _, finding := range res.Findings {
		if finding.Detail == "xhtml-not-well-formed" && finding.Location == "OEBPS/nav.xhtml" {
			return
		}
	}
	t.Fatalf("missing malformed nav XHTML finding: %+v", res.Findings)
}

// TestToolAvailabilityFollowsInjectedProbe 把 PATH 依赖从 golden 里隔离出来：
// toolAvailability 与 epubcheck 相关的 nextCommands 只由注入的探测器决定。
func TestToolAvailabilityFollowsInjectedProbe(t *testing.T) {
	path := writeNativeFixture(t)
	for _, tc := range []struct {
		name      string
		available bool
		wantCmd   string
		noCmd     string
	}{
		{"missing", false, "", "epubcheck " + report.ShellQuote(path)},
		{"present", true,
			"epubcheck " + report.ShellQuote(path),
			""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := book.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			res, err := run(t.Context(), b, Params{}, stubProbe(tc.available))
			if err != nil {
				t.Fatal(err)
			}
			tools, ok := res.Facts["toolAvailability"].(map[string]bool)
			if !ok {
				t.Fatalf("toolAvailability 类型 = %T", res.Facts["toolAvailability"])
			}
			if tools["epubcheck"] != tc.available {
				t.Errorf("toolAvailability[epubcheck] = %v, want %v", tools["epubcheck"], tc.available)
			}
			joined := strings.Join(res.NextCommands, "\n")
			if tc.wantCmd != "" && !strings.Contains(joined, tc.wantCmd) {
				t.Errorf("nextCommands 缺少 %q:\n%s", tc.wantCmd, joined)
			}
			if tc.noCmd != "" && strings.Contains(joined, tc.noCmd) {
				t.Errorf("nextCommands 不应含 %q:\n%s", tc.noCmd, joined)
			}
			for _, command := range res.NextCommands {
				if strings.Contains(command, "<") || strings.HasPrefix(strings.TrimSpace(command), "#") || strings.Contains(command, "work/after") {
					t.Errorf("nextCommand 不是可执行建议：%q", command)
				}
			}
		})
	}
}

// TestRunDefaultsToExternProbe 断言导出的 Run 走 extern.LookPath（INV-4），
// 且探测结果与 extern 一致 —— 这条不依赖 PATH 上是否真有 epubcheck。
func TestRunDefaultsToExternProbe(t *testing.T) {
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
	tools, ok := res.Facts["toolAvailability"].(map[string]bool)
	if !ok {
		t.Fatalf("toolAvailability 类型 = %T", res.Facts["toolAvailability"])
	}
	want, _ := extern.LookPath("epubcheck")
	if tools["epubcheck"] != want {
		t.Errorf("Run 的 epubcheck 探测 = %v，extern.LookPath = %v", tools["epubcheck"], want)
	}
}

// TestActionableFindingsSerialisesAsArray 锁定 MEDIUM-2：零条可执行发现时
// facts.actionableFindings 必须是 []，不能是 null（消费方会做 | length）。
func TestActionableFindingsSerialisesAsArray(t *testing.T) {
	path := writeNativeFixture(t)
	clean := filepath.Join(t.TempDir(), "clean.epub")
	// 给 nav.xhtml 补上 lang，使四个 detector 全部落空。
	rewriteZipEntry(t, path, clean, "OEBPS/nav.xhtml", func(data []byte) []byte {
		return bytes.Replace(data,
			[]byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">`),
			[]byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN">`), 1)
	})
	b, err := book.Open(clean)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := res.Facts["actionableFindings"].([]detectorFinding)
	if !ok {
		t.Fatalf("actionableFindings 类型 = %T", res.Facts["actionableFindings"])
	}
	if len(got) != 0 {
		t.Fatalf("fixture 应产生 0 条可执行发现，实际 %d 条：%+v", len(got), got)
	}
	raw, err := json.Marshal(res.Facts["actionableFindings"])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Errorf("actionableFindings 序列化 = %s, want []", raw)
	}
	// nextCommands 同理：空列表也必须是 []。
	if raw, err := json.Marshal((&inspector{}).nextCommands()); err != nil {
		t.Fatal(err)
	} else if string(raw) != "[]" {
		t.Errorf("空 nextCommands 序列化 = %s, want []", raw)
	}
}

func TestCSSURLAuditScansOnlyURLFunctions(t *testing.T) {
	path := writeNativeFixture(t)
	cssOnlyText := filepath.Join(t.TempDir(), "css-text.epub")
	rewriteZipEntry(t, path, cssOnlyText, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`[data-icon="url(../Images/old-cover.png)"]::before {
  content: "url(../Images/old-cover.png)";
} /* url(../Images/old-cover.png) */
`)
	})

	b, err := book.Open(cssOnlyText)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range res.Findings {
		if finding.Title == "CSS url() target missing" && strings.Contains(finding.Location, "old-cover.png") {
			t.Fatalf("URL-like CSS text produced a missing-resource error: %+v", finding)
		}
	}
}

func TestCSSURLAuditStillReportsMissingURLFunctionTarget(t *testing.T) {
	path := writeNativeFixture(t)
	missingURL := filepath.Join(t.TempDir(), "missing-url.epub")
	rewriteZipEntry(t, path, missingURL, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`a { background-image: url("../Images/old-cover.png"); }
`)
	})

	b, err := book.Open(missingURL)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range res.Findings {
		if finding.Level == "error" && finding.Title == "CSS url() target missing" &&
			finding.Location == "Styles/main.css -> ../Images/old-cover.png" {
			return
		}
	}
	t.Fatalf("missing url() target did not produce an error finding: %+v", res.Findings)
}

func TestCSSURLAuditIgnoresQueryAndDowngradesEscapes(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "query-and-escape.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`a { background: url("../Images/cover.png?edition=2#cover"); }
b { background: url("../Images/missing\\20 cover.png"); }
`)
	})
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	foundEscapedWarning := false
	for _, finding := range res.Findings {
		if finding.Level == "error" && finding.Title == "CSS url() target missing" {
			t.Fatalf("query or escaped URL produced a missing-target error: %+v", finding)
		}
		if finding.Level == "warn" && finding.Title == "CSS url() uses escapes; target not verified" && finding.Detail == "css-reference-escaped" {
			foundEscapedWarning = true
		}
	}
	if !foundEscapedWarning {
		t.Fatalf("escaped URL warning missing: %+v", res.Findings)
	}
}

func TestCSSURLAuditReportsReferenceScannerFailures(t *testing.T) {
	path := writeNativeFixture(t)
	malformedCSS := filepath.Join(t.TempDir(), "malformed-css.epub")
	rewriteZipEntry(t, path, malformedCSS, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte{0xff}
	})

	b, err := book.Open(malformedCSS)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range res.Findings {
		if finding.Level == "error" && strings.HasPrefix(finding.Title, "CSS reference scan failed:") &&
			finding.Location == "Styles/main.css" && finding.Detail == "css-reference-scan" {
			return
		}
	}
	t.Fatalf("scanner failure did not produce a specific error finding: %+v", res.Findings)
}
