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

// brokenFixture 覆盖 12 类错误措辞；validFixture 只有合法弹注。
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
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li>
      </ol>
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
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li>
      </ol>
    </aside>
`
	return map[string]string{
		"META-INF/container.xml": popupContainer,
		"OEBPS/content.opf":      strings.Replace(popupOPF, `"/><item id="t-a"`, `"/><item id="t-v" href="Text/valid.xhtml" media-type="application/xhtml+xml"/><item id="t-a"`, 1),
		"OEBPS/Icons/note.png":   "png",
		"OEBPS/Text/valid.xhtml": popupXHTML(body),
	}
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
	b, err := book.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, f := range res.Findings {
		if f.Level != "error" || f.ID != "popupnotes" {
			t.Fatalf("finding 形状错误: %+v", f)
		}
		titles = append(titles, f.Title)
	}
	return res.Status, titles, res.Facts
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
	if facts["violations"] != len(want) || facts["noterefs"] != 4 || facts["text_files"] != 4 {
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
	if facts["violations"] != 0 || facts["noterefs"] != 1 || facts["text_files"] != 1 {
		t.Errorf("facts = %v", facts)
	}
}
