package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeImportedTree 写一棵像导入书那样的源树：样式在 OEBPS/css/，页面 link 写法各异。
func writeImportedTree(t *testing.T, chapter2 string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "mimetype"), "application/epub+zip")
	mustWrite(t, filepath.Join(dir, "META-INF", "container.xml"), `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)
	mustWrite(t, filepath.Join(dir, "OEBPS", "content.opf"), `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title><dc:identifier id="id">x</dc:identifier></metadata>
  <manifest>
    <item href="css/main.css" id="css-reader" media-type="text/css"/>
    <item href="Text/c1.xhtml" id="c1" media-type="application/xhtml+xml"/>
    <item href="Text/c2.xhtml" id="c2" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>
`)
	mustWrite(t, filepath.Join(dir, "OEBPS", "css", "main.css"), "p{}\n")
	mustWrite(t, filepath.Join(dir, "OEBPS", "Text", "c1.xhtml"), `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
    <title>一</title>
    <link href="../css/main.css" rel="stylesheet" type="text/css"/>
</head>
<body><p>一</p></body>
</html>
`)
	mustWrite(t, filepath.Join(dir, "OEBPS", "Text", "c2.xhtml"), chapter2)
	return dir
}

func TestReaderLayerLinksLastWithoutTouchingOtherBytes(t *testing.T) {
	dir := writeImportedTree(t, `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>二</title></head><body><p>二&nbsp;</p></body></html>`)
	before := mustRead(t, filepath.Join(dir, "OEBPS", "Text", "c1.xhtml"))
	got, err := addSourceReaderLayer(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.cssPath != "OEBPS/css/reader.css" || got.pages != 2 {
		t.Fatalf("result = %+v", got)
	}
	opfText := mustRead(t, filepath.Join(dir, "OEBPS", "content.opf"))
	// 已被占用的 id 换一个，新项排在 manifest 最后、沿用缩进。
	if !strings.Contains(opfText, "\n    <item id=\"css-reader-2\" href=\"css/reader.css\" media-type=\"text/css\" />\n  </manifest>") {
		t.Errorf("OPF manifest edit:\n%s", opfText)
	}
	c1 := mustRead(t, filepath.Join(dir, "OEBPS", "Text", "c1.xhtml"))
	want := strings.Replace(before, "</head>", `    <link rel="stylesheet" type="text/css" href="../css/reader.css"/>`+"\n</head>", 1)
	if c1 != want {
		t.Errorf("c1 =\n%s\nwant\n%s", c1, want)
	}
	c2 := mustRead(t, filepath.Join(dir, "OEBPS", "Text", "c2.xhtml"))
	if !strings.Contains(c2, `<title>二</title>`+"\n"+`<link rel="stylesheet" type="text/css" href="../css/reader.css"/></head>`) || !strings.Contains(c2, "二&nbsp;") {
		t.Errorf("c2 = %s", c2)
	}
	if _, err := addSourceReaderLayer(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second run err = %v", err)
	}
}

func TestReaderLayerChangesNothingWhenAPageCannotBeEdited(t *testing.T) {
	dir := writeImportedTree(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>没有 head</p></body></html>`)
	opfBefore := mustRead(t, filepath.Join(dir, "OEBPS", "content.opf"))
	c1Before := mustRead(t, filepath.Join(dir, "OEBPS", "Text", "c1.xhtml"))
	if _, err := addSourceReaderLayer(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "c2.xhtml") {
		t.Fatalf("err = %v", err)
	}
	if mustRead(t, filepath.Join(dir, "OEBPS", "content.opf")) != opfBefore || mustRead(t, filepath.Join(dir, "OEBPS", "Text", "c1.xhtml")) != c1Before {
		t.Error("a failed run must not leave partial edits")
	}
	if _, err := os.Stat(filepath.Join(dir, "OEBPS", "css", "reader.css")); err == nil {
		t.Error("a failed run must not write reader.css")
	}
}

func TestBookReaderLayerCommandRefusesWhenPresent(t *testing.T) {
	h := newBookHarness(t)
	book := h.newBook("")
	if code := BookReaderLayer(context.Background(), BookReaderLayerOptions{Dir: book}, h.io()); code != ExitUsage {
		t.Fatalf("exit = %d, want usage error: %s", code, h.err)
	}
}

func TestBookBuildNotesStylesOutsideTheLayers(t *testing.T) {
	h := newBookHarness(t)
	book := h.newBook("")
	if code := h.run(book, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	if strings.Contains(h.out.String(), "style layers") {
		t.Errorf("a new book only has standard layers:\n%s", h.out)
	}
	mustWrite(t, filepath.Join(book, "epub", "OEBPS", "Styles", "frontmatter.css"), "p{}\n")
	_ = h.run(book, "build")
	if !strings.Contains(h.out.String(), "INFO style layers: outside the standard layers (SPEC-实现约束 §7): OEBPS/Styles/frontmatter.css") {
		t.Errorf("missing style-layer note:\n%s", h.out)
	}
}
