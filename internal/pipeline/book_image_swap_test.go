package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// swapBook sets up a book whose chapter shows Images/art.webp and a kindle
// overlay that carries only Images/art.jpg (no OPF, no XHTML).
func swapBook(t *testing.T, h *bookHarness) (dir, chapter string) {
	t.Helper()
	dir = h.newBook("")
	addWebP(t, dir)
	chapterPath := filepath.Join(dir, "epub/OEBPS/Text/01-chapter.xhtml")
	chapter = strings.Replace(mustRead(t, chapterPath), "<h1>", `<div class="art"><img src="../Images/art.webp" alt=""/></div>`+"\n    <h1>", 1)
	mustWrite(t, chapterPath, chapter)
	mustWrite(t, filepath.Join(dir, "variants/kindle/OEBPS/Images/art.jpg"), "\xff\xd8\xff\xe0 jpeg")
	setVariants(t, dir, map[string]variantConfig{"kindle": {Overlay: "variants/kindle", Output: "book-kindle.epub"}})
	return dir, chapter
}

func TestOverlayImageSwapRewritesReferences(t *testing.T) {
	h := newBookHarness(t)
	dir, chapter := swapBook(t, h)
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
	if !strings.Contains(h.out.String(), "variant kindle: 1 image(s) swapped to another format, references rewritten in 2 file(s)") {
		t.Errorf("build should report the swap: %s", h.out)
	}
	generic, kindle := filepath.Join(dir, "out", "book.epub"), filepath.Join(dir, "out", "book-kindle.epub")

	if readMember(t, generic, "OEBPS/Text/01-chapter.xhtml") != chapter {
		t.Error("generic output must keep the source XHTML")
	}
	if g := memberNames(t, generic); !g["OEBPS/Images/art.webp"] || g["OEBPS/Images/art.jpg"] {
		t.Errorf("generic images wrong: %v", g)
	}

	want := strings.Replace(chapter, "../Images/art.webp", "../Images/art.jpg", 1)
	if got := readMember(t, kindle, "OEBPS/Text/01-chapter.xhtml"); got != want {
		t.Errorf("variant chapter must differ only in the extension:\n%s", got)
	}
	if k := memberNames(t, kindle); k["OEBPS/Images/art.webp"] || !k["OEBPS/Images/art.jpg"] {
		t.Errorf("variant images wrong: %v", k)
	}
	opf := readMember(t, kindle, "OEBPS/package.opf")
	if !strings.Contains(opf, `<item id="webp-art" href="Images/art.jpg" media-type="image/jpeg"/>`) || strings.Contains(opf, "art.webp") {
		t.Errorf("variant OPF not rewritten:\n%s", opf)
	}
	if strings.Contains(h.err.String(), "FAIL kindle check [kindle]") {
		t.Errorf("the swapped variant must pass the kindle check: %s", h.err)
	}

	// The source tree is untouched.
	if _, err := os.Stat(filepath.Join(dir, "epub/OEBPS/Images/art.webp")); err != nil {
		t.Errorf("source image removed: %v", err)
	}
	if mustRead(t, filepath.Join(dir, "epub/OEBPS/Text/01-chapter.xhtml")) != chapter {
		t.Error("source chapter changed")
	}

	// Preview applies the same swap.
	if code := h.runVariant(dir, "preview", "kindle"); code != 0 {
		t.Fatalf("preview = %d: %s", code, h.err)
	}
	if p := memberNames(t, filepath.Join(dir, ".epub", "preview-kindle.epub")); p["OEBPS/Images/art.webp"] || !p["OEBPS/Images/art.jpg"] {
		t.Errorf("preview images wrong: %v", p)
	}
}

func TestOverlayImageSwapRefusesCSSReference(t *testing.T) {
	h := newBookHarness(t)
	dir, _ := swapBook(t, h)
	css := filepath.Join(dir, "epub/OEBPS/Styles/reader.css")
	mustWrite(t, css, mustRead(t, css)+"\n.bg { background: url(../Images/art.webp); }\n")
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("a CSS reference to a swapped image must fail the build")
	}
	if !strings.Contains(h.err.String(), "image format swaps only rewrite XHTML and the OPF") {
		t.Errorf("missing CSS refusal: %s", h.err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "out")); len(entries) != 0 {
		t.Errorf("out/ must stay empty, has %v", entries)
	}
}

func TestOverlayImageSwapRefusesAmbiguousSource(t *testing.T) {
	h := newBookHarness(t)
	dir, _ := swapBook(t, h)
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Images/art.png"), "png")
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("two source candidates for one overlay image must fail the build")
	}
	if !strings.Contains(h.err.String(), "could replace more than one source image") {
		t.Errorf("missing ambiguity refusal: %s", h.err)
	}
}
