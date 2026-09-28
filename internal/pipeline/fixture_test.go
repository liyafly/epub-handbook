package pipeline

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// epubFixtureBytes 构造最小合法 EPUB。
func epubFixtureBytes(t testing.TB) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	mk := func(name, content string) {
		t.Helper()
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	mk("mimetype", "application/epub+zip")
	mk("META-INF/container.xml", `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)
	mk("OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="id"><metadata><dc:title>书</dc:title><dc:creator>作者</dc:creator><dc:identifier id="id">urn:uuid:x</dc:identifier><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="nav"/><itemref idref="c1"/></spine></package>`)
	mk("OEBPS/c1.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN"><head><title>c1</title></head><body><p id="p1">段落。</p></body></html>`)
	mk("OEBPS/nav.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN"><head><title>导航</title></head><body><nav epub:type="toc"><ol><li><a href="c1.xhtml">c1</a></li></ol></nav></body></html>`)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildEpubWithOPF(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.epub")
	if err := os.WriteFile(path, epubFixtureBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func epub2NoNavFixtureBytes(t testing.TB) []byte {
	t.Helper()
	original := epubFixtureBytes(t)
	reader, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		if file.Name == "OEBPS/nav.xhtml" {
			continue
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if file.Name == "OEBPS/content.opf" {
			opf := string(content)
			for _, expected := range []string{
				`version="3.0"`,
				`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`,
				`<itemref idref="nav"/>`,
			} {
				if !strings.Contains(opf, expected) {
					t.Fatalf("fixture OPF missing %q", expected)
				}
			}
			opf = strings.Replace(opf, `version="3.0"`, `version="2.0"`, 1)
			opf = strings.Replace(opf, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`, "", 1)
			opf = strings.Replace(opf, `<itemref idref="nav"/>`, "", 1)
			content = []byte(opf)
		}
		header := file.FileHeader
		entryWriter, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entryWriter.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
