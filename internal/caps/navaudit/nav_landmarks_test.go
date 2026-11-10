package navaudit

import (
	"bytes"
	"path/filepath"
	"testing"
)

// withNavAndChapter 在干净的 EPUB 3 fixture 上替换 nav 的额外 nav 块与章节正文。
func withNavAndChapter(t *testing.T, extraNav, chapterBody string) string {
	t.Helper()
	base := writeEPUB3WithImageFixture(t, true)
	withNav := filepath.Join(t.TempDir(), "nav.epub")
	rewriteZipEntry(t, base, withNav, "OEBPS/nav.xhtml", func(data []byte) []byte {
		return bytes.Replace(data, []byte("</nav></body>"), []byte("</nav>"+extraNav+"</body>"), 1)
	})
	if chapterBody == "" {
		return withNav
	}
	out := filepath.Join(t.TempDir(), "chapter.epub")
	rewriteZipEntry(t, withNav, out, "OEBPS/Text/chapter.xhtml", func(data []byte) []byte {
		return bytes.Replace(data, []byte(`<img src="../Images/scan.tiff"`), []byte(chapterBody+`<img src="../Images/scan.tiff"`), 1)
	})
	return out
}

const goodLandmarks = `<nav epub:type="landmarks" hidden=""><ol><li><a epub:type="bodymatter" href="Text/chapter.xhtml">Start</a></li></ol></nav>`

func TestNavLandmarks(t *testing.T) {
	if got := findingKinds(t, withNavAndChapter(t, "", ""))["nav-landmarks-missing"]; got != "info" {
		t.Errorf("missing landmarks = %q, want info", got)
	}
	good := findingKinds(t, withNavAndChapter(t, goodLandmarks, ""))
	for _, k := range []string{"nav-landmarks-missing", "nav-landmarks-target-missing"} {
		if _, ok := good[k]; ok {
			t.Errorf("valid landmarks reported %s", k)
		}
	}
	badFile := `<nav epub:type="landmarks"><ol><li><a epub:type="bodymatter" href="Text/missing.xhtml">x</a></li></ol></nav>`
	badID := `<nav epub:type="landmarks"><ol><li><a epub:type="bodymatter" href="Text/chapter.xhtml#nope">x</a></li></ol></nav>`
	for name, nav := range map[string]string{"file": badFile, "id": badID} {
		kinds := findingKinds(t, withNavAndChapter(t, nav, ""))
		if kinds["nav-landmarks-target-missing"] != "error" {
			t.Errorf("landmarks missing %s target not an error: %v", name, kinds)
		}
		if _, dup := kinds["xhtml-missing-target"]; dup {
			t.Errorf("landmarks missing %s also reported the generic finding", name)
		}
		if _, dup := kinds["xhtml-missing-fragment"]; dup {
			t.Errorf("landmarks missing %s also reported the generic finding", name)
		}
	}
}

func TestNavPageList(t *testing.T) {
	marker := `<span epub:type="pagebreak" role="doc-pagebreak" id="page-1" aria-label="1"/>`
	pageList := `<nav epub:type="page-list" hidden=""><ol><li><a href="Text/chapter.xhtml#page-1">1</a></li></ol></nav>`
	both := findingKinds(t, withNavAndChapter(t, goodLandmarks+pageList, marker))
	for _, k := range []string{"nav-page-list-missing", "nav-page-list-without-pagebreaks", "nav-page-list-target-missing"} {
		if _, ok := both[k]; ok {
			t.Errorf("matching page-list reported %s", k)
		}
	}
	if got := findingKinds(t, withNavAndChapter(t, goodLandmarks, marker))["nav-page-list-missing"]; got != "warn" {
		t.Errorf("pagebreak without page-list = %q, want warn", got)
	}
	if got := findingKinds(t, withNavAndChapter(t, goodLandmarks+pageList, ""))["nav-page-list-without-pagebreaks"]; got != "warn" {
		t.Errorf("page-list without pagebreak = %q, want warn", got)
	}
	roleOnly := `<span role="doc-pagebreak" id="page-1" aria-label="1"/>`
	if _, ok := findingKinds(t, withNavAndChapter(t, goodLandmarks+pageList, roleOnly))["nav-page-list-without-pagebreaks"]; ok {
		t.Error("role=doc-pagebreak should count as a page marker")
	}
	badTarget := `<nav epub:type="page-list"><ol><li><a href="Text/chapter.xhtml#page-9">9</a></li></ol></nav>`
	if got := findingKinds(t, withNavAndChapter(t, goodLandmarks+badTarget, marker))["nav-page-list-target-missing"]; got != "error" {
		t.Errorf("page-list missing target = %q, want error", got)
	}
}
