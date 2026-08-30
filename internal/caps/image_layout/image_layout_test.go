package imagelayout

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// wrapXHTML 组装带样式链接的最小 XHTML（对齐 Python 测试的 xhtml() 助手）。
func wrapXHTML(body, bodyClass string) string {
	cls := ""
	if bodyClass != "" {
		cls = ` class="` + bodyClass + `"`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Fixture</title><link rel="stylesheet" href="../Styles/media.css"/></head>
  <body` + cls + `>` + body + `</body>
</html>
`
}

type zipEntry struct {
	name    string
	content []byte
}

// writeFixtureEpub 打包 entries 为 EPUB（对齐 Python make_epub 结构）。
func writeFixtureEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.name == "mimetype" {
			h.Method = zip.Store
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(e.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// advisorFixture 组装带 nav / css / 图片的 EPUB；pages 有序，navEntries 进 toc。
func advisorFixture(t *testing.T, pages []struct{ name, body, bodyClass string }, navEntries []string) string {
	t.Helper()
	var manifest, spine strings.Builder
	manifest.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`)
	manifest.WriteString(`<item id="css" href="Styles/media.css" media-type="text/css"/>`)
	manifest.WriteString(`<item id="img" href="Images/test.png" media-type="image/png"/>`)
	for i, p := range pages {
		manifest.WriteString(`<item id="p` + strconv.Itoa(i+1) + `" href="Text/` + p.name + `" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="p` + strconv.Itoa(i+1) + `"/>`)
	}
	var navItems strings.Builder
	for _, name := range navEntries {
		navItems.WriteString(`<li><a href="Text/` + name + `">` + name + `</a></li>`)
	}
	entries := []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip")},
		{name: "META-INF/container.xml", content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`)},
		{name: "OEBPS/content.opf", content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata/>
  <manifest>` + manifest.String() + `</manifest>
  <spine>` + spine.String() + `</spine>
</package>
`)},
		{name: "OEBPS/nav.xhtml", content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>Nav</title></head>
  <body><nav epub:type="toc"><ol>` + navItems.String() + `</ol></nav></body>
</html>
`)},
		{name: "OEBPS/Styles/media.css", content: []byte(`.img-left { float: left; width: 30%; }
.img-right { float: right; width: 30%; }
.img-left img, .img-right img { width: 100%; height: auto; }
`)},
		{name: "OEBPS/Images/test.png", content: []byte("png")},
	}
	for _, p := range pages {
		entries = append(entries, zipEntry{name: "OEBPS/Text/" + p.name, content: []byte(wrapXHTML(p.body, p.bodyClass))})
	}
	path := filepath.Join(t.TempDir(), "fixture.epub")
	writeFixtureEpub(t, path, entries)
	return path
}

// runAdvisor 打开并执行 Run。
func runAdvisor(t *testing.T, path string, legacy bool) (report.Result, string) {
	t.Helper()
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: legacy})
	if err != nil {
		t.Fatal(err)
	}
	raw := ""
	if legacy {
		raw = string(res.Facts["legacyReport"].(json.RawMessage))
	}
	return res, raw
}

// findingsByKind 过滤指定 kind 的 finding（可按文件名后缀过滤）。
func findingsByKind(t *testing.T, raw, kind, filename string) []legacyFinding {
	t.Helper()
	var rep legacyReport
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatalf("legacyReport 不是合法 JSON: %v", err)
	}
	var out []legacyFinding
	for _, f := range rep.Findings {
		if f.Finding == kind && (filename == "" || strings.HasSuffix(f.File, filename)) {
			out = append(out, f)
		}
	}
	return out
}

func hasKind(t *testing.T, raw, kind, filename string) bool {
	return len(findingsByKind(t, raw, kind, filename)) > 0
}

func TestLoneImageAndPosterExclusion(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"bare.xhtml", `<img src="../Images/test.png" alt="test"/><p>正文足够长。</p>`, ""},
		{"figure.xhtml", `<figure><img src="../Images/test.png" alt="test"/></figure><p>正文。</p>`, ""},
		{"poster.xhtml", `<img src="../Images/test.png" alt=""/>`, "poster-bg"},
	}, []string{"bare.xhtml", "figure.xhtml", "poster.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "lone-image-no-figure", "bare.xhtml") {
		t.Error("bare.xhtml 应命中 lone-image-no-figure")
	}
	if hasKind(t, raw, "lone-image-no-figure", "figure.xhtml") {
		t.Error("figure.xhtml 不应命中")
	}
	if hasKind(t, raw, "lone-image-no-figure", "poster.xhtml") {
		t.Error("poster-bg 正文不应命中")
	}
}

func TestCaptionDetection(t *testing.T) {
	long := "<p>" + strings.Repeat("这是普通正文段落，不应因为紧跟图片就被当成图注。", 5) + "</p>"
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"short.xhtml", `<img src="../Images/test.png" alt="test"/><p>十二字以内图注</p>`, ""},
		{"long.xhtml", `<img src="../Images/test.png" alt="test"/>` + long, ""},
	}, []string{"short.xhtml", "long.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "caption-detached", "short.xhtml") {
		t.Error("short.xhtml 应命中 caption-detached")
	}
	if hasKind(t, raw, "caption-detached", "long.xhtml") {
		t.Error("long.xhtml 不应命中")
	}
}

func TestFloatWidthRisk(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"bad.xhtml", `<img src="../Images/test.png" alt="test" style="float:left;width:50%"/><p>正文。</p>`, ""},
		{"good.xhtml", `<figure class="img-left" style="width:30%"><img src="../Images/test.png" alt="test" style="width:100%;height:auto"/></figure><p>正文。</p>`, ""},
	}, []string{"bad.xhtml", "good.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "float-width-risk", "bad.xhtml") {
		t.Error("bad.xhtml 应命中 float-width-risk")
	}
	if hasKind(t, raw, "float-width-risk", "good.xhtml") {
		t.Error("good.xhtml 不应命中")
	}
}

func TestMissingAlt(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"missing.xhtml", `<figure><img src="../Images/test.png"/></figure><p>正文。</p>`, ""},
		{"present.xhtml", `<figure><img src="../Images/test.png" alt=""/></figure><p>正文。</p>`, ""},
	}, []string{"missing.xhtml", "present.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "missing-alt", "missing.xhtml") {
		t.Error("missing.xhtml 应命中 missing-alt")
	}
	if hasKind(t, raw, "missing-alt", "present.xhtml") {
		t.Error("空 alt 不应命中")
	}
}

func TestNoterefIconExempt(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"notes.xhtml", `<p>正文<sup><a class="noteref-icon" href="#note"><img src="../Images/test.png" alt="注"/></a></sup>继续。</p>`, ""},
	}, []string{"notes.xhtml"})
	_, raw := runAdvisor(t, path, true)
	var rep legacyReport
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if strings.HasSuffix(f.File, "notes.xhtml") {
			t.Errorf("noteref 图标不应产生任何 finding: %s", f.Finding)
		}
	}
}

func TestChapterHeadCandidate(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"chapter.xhtml", `<figure><img src="../Images/test.png" alt="chapter"/></figure><h1>标题</h1><p>正文。</p>`, ""},
		{"not-first.xhtml", `<h1>标题</h1><figure><img src="../Images/test.png" alt="later"/></figure><p>正文。</p>`, ""},
		{"not-nav.xhtml", `<figure><img src="../Images/test.png" alt="hidden"/></figure><p>正文。</p>`, ""},
	}, []string{"chapter.xhtml", "not-first.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "chapter-head-image-candidate", "chapter.xhtml") {
		t.Error("chapter.xhtml 应命中 chapter-head-image-candidate")
	}
	if hasKind(t, raw, "chapter-head-image-candidate", "not-first.xhtml") {
		t.Error("图不在首元素不应命中")
	}
	if hasKind(t, raw, "chapter-head-image-candidate", "not-nav.xhtml") {
		t.Error("不在 nav toc 不应命中")
	}
}

func TestFullpageAliteCandidate(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"fullpage.xhtml", `<figure><img src="../Images/test.png" alt="volume"/></figure>`, ""},
		{"normal.xhtml", `<p>普通正文超过二十个字符，不能被识别成整页单图候选。</p><img src="../Images/test.png" alt="inline"/>`, ""},
	}, []string{"fullpage.xhtml", "normal.xhtml"})
	_, raw := runAdvisor(t, path, true)
	if !hasKind(t, raw, "fullpage-image-alite-candidate", "fullpage.xhtml") {
		t.Error("fullpage.xhtml 应命中 fullpage-image-alite-candidate")
	}
	if hasKind(t, raw, "fullpage-image-alite-candidate", "normal.xhtml") {
		t.Error("normal.xhtml 不应命中")
	}
}

func TestRunIsReadOnlyAndComplete(t *testing.T) {
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"chapter.xhtml", `<img src="../Images/test.png"/><p>图注</p>`, ""},
	}, []string{"chapter.xhtml"})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, raw := runAdvisor(t, path, true)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("输入 EPUB 字节被改动")
	}
	if res.Status != "complete" {
		t.Errorf("advisor 恒为 complete，got %s", res.Status)
	}
	if len(res.Findings) == 0 {
		t.Error("应产生新信封 findings")
	}
	for _, f := range res.Findings {
		if f.Level != "warn" {
			t.Errorf("finding level = %s, want warn", f.Level)
		}
	}
	var rep legacyReport
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Version != "1" {
		t.Errorf("version = %q", rep.Version)
	}
	for _, f := range rep.Findings {
		if f.Scene != "image-layout" {
			t.Errorf("scene = %q", f.Scene)
		}
		if len(f.Candidates) == 0 {
			t.Errorf("finding %s 无候选", f.Finding)
		}
	}
}
