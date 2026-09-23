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

func normalizeFixtureCommands(commands []string, path string) []string {
	quoted := report.ShellQuote(path)
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
		{"missing", false,
			"# EPUBCheck runs in GitHub Actions; local preflight skips it when unavailable.",
			"epubcheck " + report.ShellQuote(path)},
		{"present", true,
			"epubcheck " + report.ShellQuote(path),
			"# EPUBCheck runs in GitHub Actions; local preflight skips it when unavailable."},
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
			if !strings.Contains(joined, tc.wantCmd) {
				t.Errorf("nextCommands 缺少 %q:\n%s", tc.wantCmd, joined)
			}
			if strings.Contains(joined, tc.noCmd) {
				t.Errorf("nextCommands 不应含 %q:\n%s", tc.noCmd, joined)
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
