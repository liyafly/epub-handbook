package popupnotes

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// fixture XHTML 模板（必须是良构 XML）。
func popupXHTML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>fixture</title></head>
  <body>
` + body + `  </body>
</html>
`
}

const popupOPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:test:popup</dc:identifier>
    <dc:title>Popup Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="note-icon" href="Icons/note.png" media-type="image/png"/>
    <item id="t-a" href="Text/a-bad-noteref.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-b" href="Text/b-bad-aside.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-c" href="Text/c-bad-backlink.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-v" href="Text/valid.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="t-a"/><itemref idref="t-b"/><itemref idref="t-c"/><itemref idref="t-v"/>
  </spine>
</package>
`

const popupContainer = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`

// brokenFixture 覆盖旧 grouped 结构的 12 类错误措辞；validFixture 只有合法的
// 标准结构弹注。
func brokenFixture() map[string]string {
	bodyA := `    <p id="dup">一</p>
    <p id="dup">二</p>
    <p>正文<a class="noteref-icon" role="doc-noteref" href="#a1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="a1">注<a epub:type="backlink" role="doc-backlink" href="#nr-missing">↩</a></li>
      </ol>
    </aside>
`
	bodyB := `    <p>正文<a id="b1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#missing"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside role="doc-footnote">
      <ol class="footnote-list"><li class="footnote-item" id="x1">甲</li></ol>
      <ol class="footnote-list"><li class="footnote-item" id="x2">乙</li></ol>
    </aside>
`
	bodyC := `    <p>正文<a id="nr1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#c1"><img src="../Icons/missing-icon.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="c1">注<a epub:type="backlink" href="#c1">↩</a></li>
      </ol>
    </aside>
`
	bodyV := `    <p>正文<a id="v1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnotes">
      <div class="footnote-list">
        <aside class="footnote-item" id="n1" epub:type="footnote" role="doc-footnote">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></aside>
      </div>
    </aside>
`
	files := map[string]string{
		"META-INF/container.xml":          popupContainer,
		"OEBPS/content.opf":               popupOPF,
		"OEBPS/Icons/note.png":            "png",
		"OEBPS/Text/a-bad-noteref.xhtml":  popupXHTML(bodyA),
		"OEBPS/Text/b-bad-aside.xhtml":    popupXHTML(bodyB),
		"OEBPS/Text/c-bad-backlink.xhtml": popupXHTML(bodyC),
		"OEBPS/Text/valid.xhtml":          popupXHTML(bodyV),
	}
	return files
}

func validFixture() map[string]string {
	body := `    <p>正文<a id="v1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnotes">
      <div class="footnote-list">
        <aside class="footnote-item" id="n1" epub:type="footnote" role="doc-footnote">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></aside>
      </div>
    </aside>
`
	return map[string]string{
		"META-INF/container.xml": popupContainer,
		"OEBPS/content.opf":      strings.Replace(popupOPF, `"/><item id="t-a"`, `"/><item id="t-v" href="Text/valid.xhtml" media-type="application/xhtml+xml"/><item id="t-a"`, 1),
		"OEBPS/Icons/note.png":   "png",
		"OEBPS/Text/valid.xhtml": popupXHTML(body),
	}
}

// legacyFixture 是旧 grouped 结构：整组 ol.footnote-list 包在
// aside[epub:type=footnote] 中，仅作为 Duokan fallback 保留。
func legacyFixture() map[string]string {
	files := validFixture()
	files["OEBPS/Text/valid.xhtml"] = strings.Replace(files["OEBPS/Text/valid.xhtml"], `    <aside epub:type="footnotes">
      <div class="footnote-list">
        <aside class="footnote-item" id="n1" epub:type="footnote" role="doc-footnote">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></aside>
      </div>
    </aside>
`, `    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li>
      </ol>
    </aside>
`, 1)
	return files
}

func writePopupEpub(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
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

func runGoPopup(t *testing.T, epub string) (string, []string, map[string]any) {
	t.Helper()
	status, titles, _, facts := runGoPopupWithWarnings(t, epub)
	return status, titles, facts
}

// runGoPopupWithWarnings 额外返回旧 grouped 结构的 warn 措辞。
func runGoPopupWithWarnings(t *testing.T, epub string) (string, []string, []string, map[string]any) {
	t.Helper()
	b, err := book.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	var titles, warns []string
	for _, f := range res.Findings {
		switch {
		case f.Level == "error" && f.ID == "popupnotes":
			titles = append(titles, f.Title)
		case f.Level == "warn" && f.ID == "popupnotes.legacy-container":
			warns = append(warns, f.Title)
		default:
			t.Fatalf("finding 形状错误: %+v", f)
		}
	}
	return res.Status, titles, warns, res.Facts
}

// TestPopupNotesErrors 锁定坏 fixture 的逐条错误措辞、顺序与 status=failed
// （措辞与顺序沿用原校验器，title 以 zip 路径开头）。
func TestPopupNotesErrors(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "broken.epub")
	writePopupEpub(t, epub, brokenFixture())

	status, titles, facts := runGoPopup(t, epub)
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	want := []string{
		"OEBPS/Text/a-bad-noteref.xhtml: duplicate id: dup",
		"OEBPS/Text/a-bad-noteref.xhtml: noteref missing id",
		"OEBPS/Text/a-bad-noteref.xhtml: noteref must have epub:type=noteref",
		"OEBPS/Text/a-bad-noteref.xhtml: backlink target must be a noteref id: #nr-missing",
		"OEBPS/Text/b-bad-aside.xhtml: footnote aside must have epub:type=footnote",
		"OEBPS/Text/b-bad-aside.xhtml: footnote aside must contain exactly one ol.footnote-list",
		"OEBPS/Text/b-bad-aside.xhtml: noteref target missing: #missing",
		"OEBPS/Text/b-bad-aside.xhtml: every noteref target must be in ol.footnote-list",
		"OEBPS/Text/b-bad-aside.xhtml: each footnote item should contain a backlink",
		"OEBPS/Text/c-bad-backlink.xhtml: backlink must have role=doc-backlink",
		"OEBPS/Text/c-bad-backlink.xhtml: backlink target must be a noteref id: #c1",
		"OEBPS/content.opf: manifest must include noteref icon Icons/missing-icon.png",
		"OEBPS: noteref icon missing on disk: Icons/missing-icon.png",
	}
	if strings.Join(titles, "\n") != strings.Join(want, "\n") {
		t.Errorf("错误措辞不一致:\n--- want ---\n%s\n--- got ---\n%s",
			strings.Join(want, "\n"), strings.Join(titles, "\n"))
	}
	if facts["violations"] != len(want) || facts["standardViolations"] != len(want) || facts["noterefs"] != 4 || facts["text_files"] != 4 {
		t.Errorf("facts = %v", facts)
	}
}

func TestPopupNotesOK(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "valid.epub")
	writePopupEpub(t, epub, validFixture())

	status, titles, facts := runGoPopup(t, epub)
	if status != "complete" {
		t.Fatalf("go status 应为 complete，实际 %s", status)
	}
	if len(titles) != 0 {
		t.Fatalf("不应有 error findings: %v", titles)
	}
	if facts["violations"] != 0 || facts["standardViolations"] != 0 || facts["noterefs"] != 1 || facts["text_files"] != 1 {
		t.Errorf("facts = %v", facts)
	}
}

// TestPopupWarnsLegacyGroupedAside 锁定旧 grouped 结构：仍可通过，但给出 warn。
func TestPopupWarnsLegacyGroupedAside(t *testing.T) {
	epub := filepath.Join(t.TempDir(), "legacy.epub")
	writePopupEpub(t, epub, legacyFixture())
	status, titles, warns, facts := runGoPopupWithWarnings(t, epub)
	if status != "complete" || len(titles) != 0 || facts["standardViolations"] != 0 {
		t.Fatalf("status=%q titles=%v facts=%#v, want complete without errors", status, titles, facts)
	}
	want := "OEBPS/Text/valid.xhtml: legacy grouped footnote aside marks the whole list as one note; use aside[epub:type=footnotes] with one aside[epub:type=footnote] per note"
	if len(warns) != 1 || warns[0] != want {
		t.Fatalf("warns=%q, want [%q]", warns, want)
	}
}

// TestPopupStandardStructureErrors 锁定标准结构的错误措辞与顺序。
func TestPopupStandardStructureErrors(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
		want []string
	}{
		{
			name: "container-role",
			old:  `<aside epub:type="footnotes">`,
			new:  `<aside epub:type="footnotes" role="doc-footnote">`,
			want: []string{"OEBPS/Text/valid.xhtml: footnotes container must not have role=doc-footnote"},
		},
		{
			name: "note-role-and-class",
			old:  `<aside class="footnote-item" id="n1" epub:type="footnote" role="doc-footnote">`,
			new:  `<aside id="n1" epub:type="footnote">`,
			want: []string{
				"OEBPS/Text/valid.xhtml: noteref target aside must have class=footnote-item",
				"OEBPS/Text/valid.xhtml: footnote aside must have role=doc-footnote: #n1",
				"OEBPS/Text/valid.xhtml: footnote aside must have class=footnote-item: #n1",
			},
		},
		{
			name: "target-li",
			old:  `<aside class="footnote-item" id="n1" epub:type="footnote" role="doc-footnote">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></aside>`,
			new:  `<ol><li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li></ol>`,
			want: []string{
				"OEBPS/Text/valid.xhtml: noteref target must be aside: #n1",
				"OEBPS/Text/valid.xhtml: every noteref target must be a footnote aside in .footnote-list",
				"OEBPS/Text/valid.xhtml: each footnote aside should contain a backlink",
			},
		},
		{
			name: "duokan-on-standard",
			old:  `class="noteref-icon"`,
			new:  `class="noteref-icon duokan-footnote"`,
			want: []string{"OEBPS/Text/valid.xhtml: Duokan fallback is not defined for the footnotes container; keep the legacy grouped ol structure"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := validFixture()
			files["OEBPS/Text/valid.xhtml"] = strings.Replace(files["OEBPS/Text/valid.xhtml"], tc.old, tc.new, 1)
			epub := filepath.Join(t.TempDir(), tc.name+".epub")
			writePopupEpub(t, epub, files)
			status, titles, _ := runGoPopup(t, epub)
			if status != "failed" || strings.Join(titles, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("status=%q titles:\n%s\nwant:\n%s", status, strings.Join(titles, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestPopupRejectsRootedIconSrc(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "rooted-icon.epub")
	files := validFixture()
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"],
		`href="Icons/note.png"`, `href="Text/Icons/n.png"`, 1)
	delete(files, "OEBPS/Icons/note.png")
	files["OEBPS/Text/Icons/n.png"] = "png"
	files["OEBPS/Text/valid.xhtml"] = strings.Replace(files["OEBPS/Text/valid.xhtml"],
		`src="../Icons/note.png"`, `src="/Icons/n.png"`, 1)
	writePopupEpub(t, epub, files)

	status, titles, facts := runGoPopup(t, epub)
	if status != "failed" || facts["violations"] != 1 {
		t.Fatalf("status=%s violations=%v, want one rooted-icon violation", status, facts["violations"])
	}
	want := "OEBPS/Text/valid.xhtml: noteref img src must be a local EPUB resource: /Icons/n.png"
	if len(titles) != 1 || titles[0] != want {
		t.Fatalf("violations=%q, want [%q]", titles, want)
	}
}

func TestPopupNotesSeparatesStandardAndDuokanViolations(t *testing.T) {
	files := legacyFixture()
	files["OEBPS/Text/valid.xhtml"] = strings.Replace(
		files["OEBPS/Text/valid.xhtml"],
		`class="noteref-icon"`,
		`class="noteref-icon duokan-footnote"`,
		1,
	)
	epub := filepath.Join(t.TempDir(), "partial-duokan.epub")
	writePopupEpub(t, epub, files)
	status, _, facts := runGoPopup(t, epub)
	if status != "failed" || facts["violations"] != 2 || facts["standardViolations"] != 0 {
		t.Fatalf("status=%q facts=%#v, want two Duokan-only violations and zero standard violations", status, facts)
	}
}

func TestPopupRejectsDuokanContentClassOnLIOutsideDuokanMode(t *testing.T) {
	files := legacyFixture()
	files["OEBPS/Text/valid.xhtml"] = strings.Replace(
		files["OEBPS/Text/valid.xhtml"],
		`class="footnote-item"`,
		`class="footnote-item duokan-footnote-content"`,
		1,
	)
	epub := filepath.Join(t.TempDir(), "misplaced-duokan-content.epub")
	writePopupEpub(t, epub, files)
	status, titles, facts := runGoPopup(t, epub)
	if status != "failed" || facts["violations"] != 1 || facts["standardViolations"] != 0 {
		t.Fatalf("status=%q titles=%v facts=%#v, want one Duokan-only error outside Duokan mode", status, titles, facts)
	}
	if len(titles) != 1 || !strings.Contains(titles[0], "duokan-footnote-content must not be on li") {
		t.Fatalf("titles=%v, want content-class-on-li error", titles)
	}
}

func TestPopupScansManifestXHTMLOutsideOEBPSText(t *testing.T) {
	files := map[string]string{
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="EPUB/package.opf"/></rootfiles></container>`,
		"EPUB/package.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Fixture</dc:title></metadata>
  <manifest><item id="chapter" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="chapter"/></spine>
</package>`,
		"EPUB/c1.xhtml": popupXHTML(`<p><a epub:type="noteref" href="#missing" id="r1">1</a></p>`),
	}
	epub := filepath.Join(t.TempDir(), "outside-text.epub")
	writePopupEpub(t, epub, files)
	b, err := book.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	result, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["text_files"] != 1 || result.Facts["noterefs"] != 1 || result.Facts["violations"] == 0 || result.Facts["standardViolations"] == 0 {
		t.Fatalf("facts=%#v, want one scanned XHTML, one noteref, and violations", result.Facts)
	}
}

func TestPopupAcceptsRootLevelOPF(t *testing.T) {
	files := validFixture()
	rootFiles := make(map[string]string, len(files))
	for name, content := range files {
		if name == "META-INF/container.xml" {
			content = strings.Replace(content, "OEBPS/content.opf", "content.opf", 1)
		}
		name = strings.TrimPrefix(name, "OEBPS/")
		rootFiles[name] = content
	}
	epub := filepath.Join(t.TempDir(), "root-opf.epub")
	writePopupEpub(t, epub, rootFiles)

	status, titles, facts := runGoPopup(t, epub)
	if status != "complete" || len(titles) != 0 {
		t.Fatalf("root-level OPF status=%q titles=%v, want complete without errors", status, titles)
	}
	if facts["violations"] != 0 || facts["standardViolations"] != 0 || facts["noterefs"] != 1 {
		t.Fatalf("root-level OPF facts = %#v", facts)
	}
}

func TestPopupCountsDuplicateSpineItemOnce(t *testing.T) {
	files := validFixture()
	opf := strings.Replace(files["OEBPS/content.opf"],
		`<itemref idref="t-v"/>`,
		`<itemref idref="t-v"/><itemref idref="t-v"/>`, 1)
	files["OEBPS/content.opf"] = opf
	epub := filepath.Join(t.TempDir(), "duplicate-spine.epub")
	writePopupEpub(t, epub, files)

	status, titles, facts := runGoPopup(t, epub)
	if status != "complete" || len(titles) != 0 {
		t.Fatalf("duplicate spine status=%q titles=%v, want complete without errors", status, titles)
	}
	if facts["violations"] != 0 || facts["noterefs"] != 1 {
		t.Fatalf("duplicate spine facts = %#v, want one scanned noteref", facts)
	}
}
