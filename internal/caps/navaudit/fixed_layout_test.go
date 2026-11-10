package navaudit

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// packFXL 把 testdata/fxl/minimal 打成 EPUB；edit 可按容器内路径改写或新增文件。
func packFXL(t *testing.T, edit map[string]func(string) string) string {
	t.Helper()
	src := filepath.Join("..", "..", "..", "testdata", "fxl", "minimal")
	files := map[string]string{}
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(src, p)
		files[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, f := range edit {
		files[name] = f(files[name])
	}
	out := filepath.Join(t.TempDir(), "fxl.epub")
	fh, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(fh)
	write := func(name, data string, method uint16) {
		wr, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := wr.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	write("mimetype", files["mimetype"], zip.Store)
	for name, data := range files {
		if name != "mimetype" {
			write(name, data, zip.Deflate)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func replacer(old, new string) func(string) string {
	return func(s string) string { return strings.Replace(s, old, new, 1) }
}

func TestFixedLayoutMinimalIsClean(t *testing.T) {
	res := runNativeAudit(t, packFXL(t, nil))
	for _, f := range res.Findings {
		if f.Level != "info" {
			t.Errorf("minimal FXL fixture finding: %+v", f)
		}
	}
	for _, f := range res.Findings {
		if strings.HasPrefix(f.Detail, "css-") {
			t.Errorf("font-size checks must skip FXL-only stylesheets: %+v", f)
		}
	}
}

func TestFixedLayoutViewport(t *testing.T) {
	missing := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/p2.xhtml": replacer(`  <meta name="viewport" content="width=600, height=800"/>`+"\n", ""),
	}))
	if missing["fxl-viewport-missing"] != "error" {
		t.Errorf("missing viewport = %v", missing)
	}
	noHeight := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/p2.xhtml": replacer(`content="width=600, height=800"`, `content="width=600"`),
	}))
	if noHeight["fxl-viewport-size-missing"] != "error" {
		t.Errorf("viewport without height = %v", noHeight)
	}
	mixed := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/p2.xhtml": replacer(`content="width=600, height=800"`, `content="width=800, height=600"`),
	}))
	if mixed["fxl-aspect-ratio-mixed"] != "warn" {
		t.Errorf("mixed aspect ratio = %v", mixed)
	}
	scaled := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/p2.xhtml": replacer(`content="width=600, height=800"`, `content="width=1200, height=1600"`),
	}))
	if _, ok := scaled["fxl-aspect-ratio-mixed"]; ok {
		t.Error("same ratio at a different size must not warn")
	}
}

func TestFixedLayoutReflowableOverrides(t *testing.T) {
	// p2 改成可重排：共享的 page.css 被可重排页面引用，字号检查照常报告。
	shared := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/package.opf": replacer(`<itemref idref="p2" properties="page-spread-left"/>`, `<itemref idref="p2" properties="rendition:layout-reflowable"/>`),
	}))
	if shared["css-fixed-font-size"] != "warn" {
		t.Errorf("stylesheet shared with a reflowable page must still be checked: %v", shared)
	}
	if _, ok := shared["page-spread-on-reflowable"]; ok {
		t.Error("no page-spread on the reflowable item in this case")
	}
	// 整本可重排却用了 page-spread-*。
	reflow := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/package.opf": replacer(`<meta property="rendition:layout">pre-paginated</meta>`, `<meta property="rendition:layout">reflowable</meta>`),
	}))
	if reflow["page-spread-on-reflowable"] != "warn" {
		t.Errorf("page-spread on reflowable items = %v", reflow)
	}
	if reflow["nav-page-list-without-pagebreaks"] != "warn" {
		t.Errorf("reflowable book with page-list but no markers should still warn: %v", reflow)
	}
	if _, ok := reflow["fxl-viewport-missing"]; ok {
		t.Error("viewport checks apply to fixed-layout pages only")
	}
}

func TestFixedLayoutNavStylesheetDoesNotCount(t *testing.T) {
	// 不在 spine 里的 nav 链接同一份样式表，仍按“只被固定版式页面引用”处理。
	kinds := findingKinds(t, packFXL(t, map[string]func(string) string{
		"OEBPS/nav.xhtml": replacer(`<head><title>Contents</title></head>`, `<head><title>Contents</title><link rel="stylesheet" type="text/css" href="page.css"/></head>`),
	}))
	if _, ok := kinds["css-fixed-font-size"]; ok {
		t.Errorf("a non-spine nav sharing the stylesheet must not re-enable font-size checks: %v", kinds)
	}
}
