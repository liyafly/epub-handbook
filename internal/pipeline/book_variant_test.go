package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

func readMember(t *testing.T, epub, name string) string {
	t.Helper()
	a, err := zipfs.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	data, err := a.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// setVariants writes variants into book.json of a new-layout book.
func setVariants(t *testing.T, dir string, variants map[string]variantConfig) {
	t.Helper()
	path := filepath.Join(dir, "book.json")
	var cfg bookConfig
	if err := json.Unmarshal([]byte(mustRead(t, path)), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Variants = variants
	cfg.Readers = []string{"apple", "kindle", "reeden"}
	if err := writeBookConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
}

func (h *bookHarness) runVariant(dir, command, variant string) int {
	h.t.Helper()
	return BookRun(context.Background(), BookRunOptions{Dir: dir, Command: command, Variant: variant}, h.io())
}

func TestStarterReaderLayerIsLast(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("literary-cn")
	page := mustRead(t, filepath.Join(dir, "epub/OEBPS/Text/01-chapter.xhtml"))
	base, notes, design, reader := strings.Index(page, "base.css"), strings.Index(page, "notes.css"), strings.Index(page, "design.css"), strings.Index(page, "reader.css")
	if !(base < notes && notes < design && design < reader) {
		t.Errorf("link order wrong: base=%d notes=%d design=%d reader=%d", base, notes, design, reader)
	}
	opf := mustRead(t, filepath.Join(dir, "epub/OEBPS/package.opf"))
	if !strings.Contains(opf, `id="css-reader"`) || strings.Index(opf, "css-design") > strings.Index(opf, "css-reader") {
		t.Error("reader.css must follow design.css in the manifest")
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build with reader layer = %d: %s", code, h.err)
	}
}

func TestBookVariantsBuildWithPerOutputReceipts(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/kindle/OEBPS/Styles/reader.css"), "p { text-indent: 2em; }\n")
	mustWrite(t, filepath.Join(dir, "variants/kindle/OEBPS/Styles/extra.css"), "/* added */\n")
	setVariants(t, dir, map[string]variantConfig{"kindle": {Overlay: "variants/kindle", Output: "book-kindle.epub"}})
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	generic, kindle := filepath.Join(dir, "out", "book.epub"), filepath.Join(dir, "out", "book-kindle.epub")
	if strings.Contains(readMember(t, generic, "OEBPS/Styles/reader.css"), "text-indent") {
		t.Error("generic output must not receive the overlay")
	}
	if !strings.Contains(readMember(t, kindle, "OEBPS/Styles/reader.css"), "text-indent") ||
		readMember(t, kindle, "OEBPS/Styles/extra.css") != "/* added */\n" {
		t.Error("variant output lacks the overlay files")
	}
	if readMember(t, generic, "OEBPS/Text/01-chapter.xhtml") != readMember(t, kindle, "OEBPS/Text/01-chapter.xhtml") {
		t.Error("variant XHTML must equal the generic XHTML")
	}
	for name, out := range map[string]string{"dist-sha256": generic, "dist-sha256.kindle": kindle} {
		if strings.TrimSpace(mustRead(t, filepath.Join(dir, ".epub", name))) != sha(t, out) {
			t.Errorf("receipt %s does not match its output", name)
		}
	}
	log := mustRead(t, filepath.Join(dir, ".epub", "delivery.tsv"))
	if strings.Count(log, "\n") != 3 || !strings.Contains(log, "\tvariant:kindle\n") {
		t.Errorf("delivery log = %q", log)
	}

	// An edited variant dist blocks the whole build and is named in the error.
	genericSHA := sha(t, generic)
	mustWrite(t, kindle, "hand edited")
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "book-kindle.epub") {
		t.Fatalf("edited variant dist not protected: %d %s", code, h.err)
	}
	if sha(t, generic) != genericSHA {
		t.Error("generic output changed although the variant preflight failed")
	}
	// --variant generic-only edits: building only the generic... the variant is untouched.
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Styles/design.css"), "p { margin: 0; }\n")
	if code := h.runVariant(dir, "build", "kindle"); code == 0 {
		t.Error("--variant kindle must still refuse its own edited dist")
	}
	if err := os.Remove(kindle); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".epub", "dist-sha256.kindle")); err != nil {
		t.Fatal(err)
	}
	if code := h.runVariant(dir, "build", "kindle"); code != 0 {
		t.Fatalf("--variant kindle = %d: %s", code, h.err)
	}
	if sha(t, generic) != genericSHA {
		t.Error("--variant kindle must not touch the generic output")
	}
	if !strings.Contains(readMember(t, kindle, "OEBPS/Styles/design.css"), "margin: 0") {
		t.Error("variant did not pick up the source change")
	}
	if code := h.runVariant(dir, "build", "nope"); code != ExitUsage {
		t.Errorf("unknown variant = %d, want usage", code)
	}
}

func TestBookVariantPartialFailureLeavesOutUntouched(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/good/OEBPS/Styles/reader.css"), "p{}\n")
	mustWrite(t, filepath.Join(dir, "variants/bad/OEBPS/package.opf"), "<not-a-package>")
	setVariants(t, dir, map[string]variantConfig{
		"good": {Overlay: "variants/good"}, "bad": {Overlay: "variants/bad"},
	})
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("a broken variant must fail the build")
	}
	if !strings.Contains(h.err.String(), "bad:") || !strings.Contains(h.err.String(), "nothing was written") {
		t.Errorf("error must name the failing variant: %s", h.err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "out")); len(entries) != 0 {
		t.Errorf("out/ must stay empty, has %v", entries)
	}
	if _, err := os.Stat(filepath.Join(dir, ".epub", "dist-sha256")); err == nil {
		t.Error("no receipt may be written for a failed build")
	}
	builds, _ := filepath.Glob(filepath.Join(dir, ".epub", "build.*"))
	if len(builds) != 1 {
		t.Errorf("failed build must be retained: %v", builds)
	}
}

func TestOverlayRejectsDisallowedFiles(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/Styles/reader.css"), "p{}\n")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/Text/01-chapter.xhtml"), "<html/>")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/Text/99-new.xhtml"), "<html/>")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/Images/slice.png"), "png")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/nav.xhtml"), "<html/>")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/toc.ncx"), "<ncx/>")
	mustWrite(t, filepath.Join(dir, "variants/x/OEBPS/Fonts/a.ttf"), "x")
	mustWrite(t, filepath.Join(dir, "variants/x/.DS_Store"), "ignored")
	setVariants(t, dir, map[string]variantConfig{"x": {Overlay: "variants/x"}})
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("overlay with nav, NCX, new XHTML and fonts must be rejected")
	}
	msg := h.err.String()
	for _, want := range []string{"99-new.xhtml (new XHTML", "nav.xhtml (nav document)", "toc.ncx", "Fonts/a.ttf"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should list %s: %s", want, msg)
		}
	}
	for _, allowed := range []string{"01-chapter.xhtml", "slice.png", "reader.css", ".DS_Store"} {
		if strings.Contains(msg, allowed) {
			t.Errorf("%s is allowed and must not be listed: %s", allowed, msg)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "book.epub")); err == nil {
		t.Error("nothing may be built when an overlay is invalid")
	}
	if _, err := os.Stat(filepath.Join(dir, ".epub", "build.lock")); err == nil {
		t.Error("validation must fail before taking the lock")
	}
	if code := h.runVariant(dir, "preview", "x"); code == 0 {
		t.Error("preview must reject an invalid overlay too")
	}
	// Unsafe overlay paths are refused at configuration time.
	setVariants(t, dir, map[string]variantConfig{"x": {Overlay: "../outside"}})
	if code := h.run(dir, "build"); code == 0 {
		t.Error("overlay escaping the book must be rejected")
	}
}

// slicedBook sets up a book whose chapter shows Images/whole.png and a kindle
// overlay that swaps it for two slices: replaced chapter XHTML, new images and
// a replaced OPF listing the slices instead of the whole image.
func slicedBook(t *testing.T, h *bookHarness) (dir, chapter string) {
	t.Helper()
	dir = h.newBook("")
	oebps := filepath.Join(dir, "epub/OEBPS")
	png := mustRead(t, filepath.Join(oebps, "Images/cover.png"))
	mustWrite(t, filepath.Join(oebps, "Images/whole.png"), png)
	opfPath := filepath.Join(oebps, "package.opf")
	opfText := mustRead(t, opfPath)
	const coverItem = `<item id="cover-image" href="Images/cover.png"      media-type="image/png" properties="cover-image"/>`
	if !strings.Contains(opfText, coverItem) {
		t.Fatal("starter OPF changed; update the fixture")
	}
	generic := strings.Replace(opfText, coverItem, coverItem+"\n    <item id=\"whole\" href=\"Images/whole.png\" media-type=\"image/png\"/>", 1)
	mustWrite(t, opfPath, generic)
	chapterPath := filepath.Join(oebps, "Text/01-chapter.xhtml")
	chapter = strings.Replace(mustRead(t, chapterPath), "<h1>", `<div class="shape"><img src="../Images/whole.png" alt=""/></div>`+"\n    <h1>", 1)
	mustWrite(t, chapterPath, chapter)

	overlay := filepath.Join(dir, "variants/kindle/OEBPS")
	mustWrite(t, filepath.Join(overlay, "Images/band.png"), png)
	mustWrite(t, filepath.Join(overlay, "Images/side.png"), png)
	mustWrite(t, filepath.Join(overlay, "package.opf"), strings.Replace(opfText, coverItem, coverItem+
		"\n    <item id=\"band\" href=\"Images/band.png\" media-type=\"image/png\"/>"+
		"\n    <item id=\"side\" href=\"Images/side.png\" media-type=\"image/png\"/>", 1))
	mustWrite(t, filepath.Join(overlay, "Text/01-chapter.xhtml"), strings.Replace(chapter,
		`<div class="shape"><img src="../Images/whole.png" alt=""/></div>`,
		`<div class="band"><img src="../Images/band.png" alt=""/></div><div class="side"><img src="../Images/side.png" alt=""/></div>`, 1))
	setVariants(t, dir, map[string]variantConfig{"kindle": {Overlay: "variants/kindle", Output: "book-kindle.epub"}})
	return dir, chapter
}

func memberNames(t *testing.T, epub string) map[string]bool {
	t.Helper()
	a, err := zipfs.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	names := map[string]bool{}
	for _, name := range a.Names() {
		names[name] = true
	}
	return names
}

func TestOverlayReplacesXHTMLAndCarriesOnlyItsImages(t *testing.T) {
	h := newBookHarness(t)
	dir, chapter := slicedBook(t, h)
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "variant kindle: 1 image(s) not in its manifest left out") {
		t.Errorf("build should report the pruned image: %s", h.out)
	}
	generic, kindle := filepath.Join(dir, "out", "book.epub"), filepath.Join(dir, "out", "book-kindle.epub")
	if readMember(t, generic, "OEBPS/Text/01-chapter.xhtml") != chapter {
		t.Error("generic output must keep the source XHTML")
	}
	if !strings.Contains(readMember(t, kindle, "OEBPS/Text/01-chapter.xhtml"), "band.png") {
		t.Error("variant output lacks the replaced XHTML")
	}
	g, k := memberNames(t, generic), memberNames(t, kindle)
	if !g["OEBPS/Images/whole.png"] || g["OEBPS/Images/band.png"] || g["OEBPS/Images/side.png"] {
		t.Errorf("generic images wrong: %v", g)
	}
	if k["OEBPS/Images/whole.png"] || !k["OEBPS/Images/band.png"] || !k["OEBPS/Images/side.png"] || !k["OEBPS/Images/cover.png"] {
		t.Errorf("variant images wrong: %v", k)
	}

	// Preview applies the same overlay and pruning.
	if code := h.runVariant(dir, "preview", "kindle"); code != 0 {
		t.Fatalf("preview = %d: %s", code, h.err)
	}
	if p := memberNames(t, filepath.Join(dir, ".epub", "preview-kindle.epub")); p["OEBPS/Images/whole.png"] || !p["OEBPS/Images/side.png"] {
		t.Errorf("preview images wrong: %v", p)
	}
}

func TestOverlayXHTMLMustKeepGenericText(t *testing.T) {
	h := newBookHarness(t)
	dir, _ := slicedBook(t, h)
	sliced := filepath.Join(dir, "variants/kindle/OEBPS/Text/01-chapter.xhtml")
	mustWrite(t, sliced, strings.Replace(mustRead(t, sliced), "正文从这里开始。", "正文从这里起。", 1))
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("a variant XHTML that changes the text must fail the build")
	}
	if !strings.Contains(h.err.String(), "FAIL redline text vs generic [kindle]") {
		t.Errorf("the text redline must be what fails: %s", h.err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "out")); len(entries) != 0 {
		t.Errorf("out/ must stay empty, has %v", entries)
	}
}

// newestBuildFile reads a report from the retained diagnostic build directory.
func newestBuildFile(t *testing.T, dir, name string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(dir, ".epub", "build.*", name))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(dir, ".epub", name))
	}
	if len(matches) == 0 {
		t.Fatalf("no %s in the build state", name)
	}
	return matches[len(matches)-1]
}

func TestOverlayMissingImageFailsTheBuild(t *testing.T) {
	h := newBookHarness(t)
	dir, chapter := slicedBook(t, h)
	// The variant chapter still points at the whole image, which its OPF no longer lists.
	mustWrite(t, filepath.Join(dir, "variants/kindle/OEBPS/Text/01-chapter.xhtml"), chapter)
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("a variant XHTML pointing at a pruned image must fail the build")
	}
	if !strings.Contains(h.err.String(), "FAIL epub.package.nav.audit [kindle]") ||
		!strings.Contains(mustRead(t, newestBuildFile(t, dir, "nav-audit.kindle.json")), "whole.png") {
		t.Errorf("the nav audit must flag the missing image: %s", h.err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "out")); len(entries) != 0 {
		t.Errorf("out/ must stay empty, has %v", entries)
	}
}

func TestBookAcceptDistWithVariants(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/k/OEBPS/Styles/reader.css"), "p{}\n")
	setVariants(t, dir, map[string]variantConfig{"k": {Overlay: "variants/k"}})
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	for _, r := range []string{"dist-sha256", "dist-sha256.k"} {
		if err := os.Remove(filepath.Join(dir, ".epub", r)); err != nil {
			t.Fatal(err)
		}
	}
	if code := h.run(dir, "accept-dist"); code != 0 {
		t.Fatalf("accept-dist = %d: %s", code, h.err)
	}
	if strings.TrimSpace(mustRead(t, filepath.Join(dir, ".epub", "dist-sha256.k"))) != sha(t, filepath.Join(dir, "out", "book-k.epub")) {
		t.Error("variant receipt not registered by accept-dist")
	}
	// Overlay changes but the variant dist does not: accept-dist names the member.
	mustWrite(t, filepath.Join(dir, "variants/k/OEBPS/Styles/reader.css"), "p{color:red}\n")
	if code := h.run(dir, "accept-dist"); code == 0 || !strings.Contains(h.err.String(), "OEBPS/Styles/reader.css") {
		t.Fatalf("accept-dist must catch the variant difference: %d %s", code, h.err)
	}
}

func TestLegacyVariantsConfig(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	book := t.TempDir()
	work := filepath.Join(book, "03 制作工作区")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(src, "epub"), filepath.Join(work, "epub")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(book, "build.sh"), "export EPUB_OUTPUT_NAME='主 版.epub'\n")
	mustWrite(t, filepath.Join(work, "variants/自由/OEBPS/Styles/reader.css"), "p{}\n")
	mustWrite(t, filepath.Join(work, "variants.json"), `{"variants": {"自由": {"overlay": "variants/自由", "output": "主 版-自由.epub", "fonts": []}}}`)
	l, err := resolveBookLayout(book, readBookEnv())
	if err != nil || l.Kind != layoutLegacy || len(l.Variants) != 1 {
		t.Fatalf("variants.json must not flip detection: %+v %v", l, err)
	}
	if code := h.run(book, "build"); code != 0 {
		t.Fatalf("legacy variant build = %d: %s", code, h.err)
	}
	for _, p := range []string{"dist/主 版.epub", "dist/主 版-自由.epub", ".pipeline/dist-sha256", ".pipeline/dist-sha256.自由"} {
		if _, err := os.Stat(filepath.Join(work, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	mustWrite(t, filepath.Join(work, "variants.json"), `{"variants": {"a": {"overlay": "x", "output": "主 版.epub"}}}`)
	if _, err := resolveBookLayout(book, readBookEnv()); err == nil {
		t.Error("a variant output colliding with the generic output must be rejected")
	}
}

func TestBookPreviewDoesNotTouchDist(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/k/OEBPS/Styles/reader.css"), "p{ color: green }\n")
	setVariants(t, dir, map[string]variantConfig{"k": {Overlay: "variants/k"}})
	if code := h.run(dir, "preview"); code != 0 {
		t.Fatalf("preview = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "UNCHECKED") || !strings.Contains(h.out.String(), "epub book build") {
		t.Errorf("preview must say it is unchecked: %s", h.out)
	}
	if code := h.runVariant(dir, "preview", "k"); code != 0 {
		t.Fatalf("variant preview = %d: %s", code, h.err)
	}
	if !strings.Contains(readMember(t, filepath.Join(dir, ".epub", "preview-k.epub"), "OEBPS/Styles/reader.css"), "green") {
		t.Error("variant preview lacks the overlay")
	}
	if strings.Contains(readMember(t, filepath.Join(dir, ".epub", "preview.epub"), "OEBPS/Styles/reader.css"), "green") {
		t.Error("generic preview must not include the overlay")
	}
	for _, unwanted := range []string{"out", ".epub/dist-sha256", ".epub/delivery.tsv", ".epub/nav-audit.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(unwanted))); err == nil {
			t.Errorf("preview must not create %s", unwanted)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".epub"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "preview-tmp") {
			t.Error("preview scratch directory left behind")
		}
	}
	if code := h.runVariant(dir, "preview", "nope"); code != ExitUsage {
		t.Errorf("unknown variant = %d, want usage", code)
	}
}

func TestBookPreviewLegacyAndMultiVolume(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	book := t.TempDir()
	work := filepath.Join(book, "03 制作工作区")
	for _, name := range []string{"一", "二"} {
		vol := filepath.Join(work, name)
		if err := zipfs.CopyTree(context.Background(), filepath.Join(src, "epub"), filepath.Join(vol, "epub"), nil); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(vol, "build.sh"), "\n")
	}
	if code := h.run(book, "preview"); code != 0 {
		t.Fatalf("multi-volume preview = %d: %s", code, h.err)
	}
	for _, name := range []string{"一", "二"} {
		if _, err := os.Stat(filepath.Join(work, name, ".pipeline", "preview.epub")); err != nil {
			t.Errorf("volume %s not previewed: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(work, name, "dist")); err == nil {
			t.Errorf("preview created dist for %s", name)
		}
	}
}

func addWebP(t *testing.T, dir string) {
	t.Helper()
	opfPath := filepath.Join(dir, "epub/OEBPS/package.opf")
	opf := mustRead(t, opfPath)
	anchor := `    <item id="css-base"`
	if !strings.Contains(opf, anchor) {
		t.Fatal("starter OPF anchor missing")
	}
	mustWrite(t, opfPath, strings.Replace(opf, anchor, `    <item id="webp-art" href="Images/art.webp" media-type="image/webp"/>`+"\n"+anchor, 1))
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Images/art.webp"), "RIFF0000WEBPVP8 ")
}

func TestBuildKindleCheckBlocksOnlyKindleVariant(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("clean build = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "PASS kindle check (error: 0, warn: 0, info: 0;") {
		t.Errorf("clean book should print a passing kindle summary: %s", h.out)
	}
	generic := filepath.Join(dir, "out", "book.epub")
	before := sha(t, generic)

	addWebP(t, dir)
	mustWrite(t, filepath.Join(dir, "variants/kindle/OEBPS/Styles/reader.css"), "p { text-indent: 2em; }\n")
	setVariants(t, dir, map[string]variantConfig{"kindle": {Overlay: "variants/kindle", Output: "book-kindle.epub"}})
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("WebP in the kindle variant must fail the build")
	}
	if !strings.Contains(h.err.String(), "FAIL kindle check [kindle]") || !strings.Contains(h.err.String(), "WebP") {
		t.Errorf("missing kindle variant failure: %s", h.err)
	}
	if sha(t, generic) != before {
		t.Error("out/book.epub changed although the kindle variant failed")
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "book-kindle.epub")); err == nil {
		t.Error("failed kindle variant was written")
	}

	setVariants(t, dir, nil)
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("generic build with WebP = %d: %s %s", code, h.out, h.err)
	}
	if !strings.Contains(h.out.String(), "WARN kindle check (error: 1, warn: 0, info: 0; blocks only the kindle variant;") {
		t.Errorf("generic build should only print the kindle summary: %s", h.out)
	}
	if strings.Contains(h.out.String(), "WebP image is outside") {
		t.Errorf("terminal should get one summary line, details go to report.html: %s", h.out)
	}
	if !strings.Contains(mustRead(t, filepath.Join(dir, ".epub", "report.html")), "WebP image is outside") {
		t.Error("report.html lacks the kindle finding")
	}
}

func TestBuildKindleCheckAmznMediaQueryWarns(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	design := filepath.Join(dir, "epub/OEBPS/Styles/design.css")
	mustWrite(t, design, mustRead(t, design)+"@media amzn-kf8 { p { text-indent: 2em; } }\n")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
	if !strings.Contains(h.out.String(), "WARN kindle check (error: 0, warn: 1, info: 0;") {
		t.Errorf("want one kindle warn in the summary: %s", h.out)
	}
}
