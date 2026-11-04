package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

type bookHarness struct {
	t   *testing.T
	out *bytes.Buffer
	err *bytes.Buffer
}

func newBookHarness(t *testing.T) *bookHarness {
	t.Helper()
	t.Setenv("EPUB_LOCAL_BIN", "")
	t.Setenv("EPUB_OUTPUT_NAME", "")
	t.Setenv("EPUB_LEGACY_LAYOUT", "")
	t.Setenv("EPUB_FONT_LIBRARY", "")
	return &bookHarness{t: t, out: &bytes.Buffer{}, err: &bytes.Buffer{}}
}

func (h *bookHarness) io() BookIO {
	h.out.Reset()
	h.err.Reset()
	return BookIO{Out: h.out, Err: h.err}
}

func (h *bookHarness) run(dir, command string) int {
	h.t.Helper()
	return BookRun(context.Background(), BookRunOptions{Dir: dir, Command: command}, h.io())
}

func (h *bookHarness) newBook(preset string) string {
	h.t.Helper()
	dir := filepath.Join(h.t.TempDir(), "我的 书")
	if code := BookNew(context.Background(), BookNewOptions{Dir: dir, Preset: preset}, h.io()); code != 0 {
		h.t.Fatalf("book new = %d: %s", code, h.err)
	}
	return dir
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha(t *testing.T, p string) string {
	t.Helper()
	s, err := zipfs.FileSHA256Context(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBookNewLayoutAndPreset(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("literary-cn")
	for _, rel := range []string{"book.json", "epub/mimetype", "epub/META-INF/container.xml", "epub/OEBPS/package.opf", "epub/OEBPS/Styles/notes.css"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	for _, unwanted := range []string{"02 校对材料", "制作说明.md", "THIRD_PARTY.md", ".git", "build.sh"} {
		if _, err := os.Stat(filepath.Join(dir, unwanted)); err == nil {
			t.Errorf("minimal layout must not create %s", unwanted)
		}
	}
	if got := mustRead(t, filepath.Join(dir, "epub", "mimetype")); got != "application/epub+zip" {
		t.Errorf("mimetype = %q", got)
	}
	if !strings.Contains(mustRead(t, filepath.Join(dir, "epub/OEBPS/package.opf")), `id="css-notes"`) {
		t.Error("notes layer not linked in OPF")
	}
	if !strings.Contains(mustRead(t, filepath.Join(dir, "epub/OEBPS/Text/01-chapter.xhtml")), "../Styles/notes.css") {
		t.Error("notes layer not linked in XHTML")
	}
	design := mustRead(t, filepath.Join(dir, "epub/OEBPS/Styles/design.css"))
	if !strings.Contains(design, "preset literary-cn: literary.css") || strings.Contains(design, "\n@charset") {
		t.Error("preset layers not appended correctly")
	}
	if code := BookNew(context.Background(), BookNewOptions{Dir: dir}, h.io()); code == 0 {
		t.Error("new over an existing directory must fail")
	}
	if code := BookNew(context.Background(), BookNewOptions{Dir: filepath.Join(t.TempDir(), "x"), Preset: "../etc"}, h.io()); code != ExitUsage {
		t.Errorf("invalid preset = %d, want usage", code)
	}
}

func TestBookBuildReceiptAndAtomicReplace(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("first build = %d: %s", code, h.err)
	}
	dist := filepath.Join(dir, "out", "book.epub")
	receipt := filepath.Join(dir, ".epub", "dist-sha256")
	first := sha(t, dist)
	if strings.TrimSpace(mustRead(t, receipt)) != first {
		t.Fatal("receipt does not match dist")
	}
	if _, err := os.Stat(filepath.Join(dir, ".epub", "build.lock")); err == nil {
		t.Error("build lock must be released")
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".epub"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "build.") {
			t.Errorf("successful build left %s", e.Name())
		}
	}
	if !strings.HasPrefix(mustRead(t, filepath.Join(dir, ".epub", "delivery.tsv")), "日期\t书源commit") {
		t.Error("delivery record missing header")
	}

	// mimetype first and stored, no extra fields: bytes 30.. = "mimetype" then payload.
	raw, err := os.ReadFile(dist)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw[30:38], []byte("mimetype")) || !bytes.Equal(raw[38:58], []byte("application/epub+zip")) {
		t.Error("mimetype is not the first stored entry without extra fields")
	}

	// Source change rebuilds and replaces.
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Styles/design.css"), "p { margin: 0; }\n")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("rebuild = %d: %s", code, h.err)
	}
	second := sha(t, dist)
	if second == first {
		t.Fatal("dist did not change after a source change")
	}

	// A manually edited dist is never overwritten.
	mustWrite(t, dist, "hand edited")
	edited := sha(t, dist)
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "differs from the last accepted build") {
		t.Fatalf("edited dist was not protected: code=%d %s", code, h.err)
	}
	if sha(t, dist) != edited {
		t.Error("edited dist was modified")
	}

	// A receipt-less dist is never overwritten either.
	if err := os.Remove(receipt); err != nil {
		t.Fatal(err)
	}
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "without its accepted SHA-256 receipt") {
		t.Fatalf("receipt-less dist was not protected: code=%d %s", code, h.err)
	}
}

func TestBookAcceptDistVerifiesMembers(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	dist := filepath.Join(dir, "out", "book.epub")
	receipt := filepath.Join(dir, ".epub", "dist-sha256")
	distSHA := sha(t, dist)

	// Receipt lost: accept-dist re-registers when every member still matches.
	if err := os.Remove(receipt); err != nil {
		t.Fatal(err)
	}
	if code := h.run(dir, "accept-dist"); code != 0 {
		t.Fatalf("accept-dist = %d: %s", code, h.err)
	}
	if strings.TrimSpace(mustRead(t, receipt)) != distSHA || sha(t, dist) != distSHA {
		t.Error("accept-dist must register the existing dist untouched")
	}

	// Source changed but not dist: members differ, nothing is registered.
	mustWrite(t, filepath.Join(dir, "epub/OEBPS/Styles/design.css"), "p { color: red; }\n")
	if code := h.run(dir, "accept-dist"); code == 0 || !strings.Contains(h.err.String(), "OEBPS/Styles/design.css") {
		t.Fatalf("accept-dist must name the differing member: code=%d %s", code, h.err)
	}
	if sha(t, dist) != distSHA || strings.TrimSpace(mustRead(t, receipt)) != distSHA {
		t.Error("failed accept-dist changed dist or receipt")
	}
}

func TestBookFailedBuildRetainedAndClean(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	dist := filepath.Join(dir, "out", "book.epub")
	good := sha(t, dist)

	mustWrite(t, filepath.Join(dir, "epub/OEBPS/package.opf"), "<not-a-package>")
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("broken package unexpectedly built")
	}
	if sha(t, dist) != good {
		t.Fatal("failed build replaced the last good dist")
	}
	builds, _ := filepath.Glob(filepath.Join(dir, ".epub", "build.*"))
	if len(builds) != 1 {
		t.Fatalf("failed build retention: %v", builds)
	}
	if _, err := os.Stat(filepath.Join(builds[0], "final.epub")); err != nil {
		t.Errorf("failed build lost its candidate: %v", err)
	}

	// clean keeps only the newest diagnostic build.
	older := filepath.Join(dir, ".epub", "build.older")
	if err := os.Mkdir(older, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}
	if code := h.run(dir, "clean"); code != 0 {
		t.Fatalf("clean = %d: %s", code, h.err)
	}
	if _, err := os.Stat(older); err == nil {
		t.Error("clean kept an older build")
	}
	if _, err := os.Stat(builds[0]); err != nil {
		t.Error("clean removed the latest diagnostic build")
	}

	// clean refuses while the lock exists; a stale lock also blocks build.
	lock := filepath.Join(dir, ".epub", "build.lock")
	if err := os.Mkdir(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := h.run(dir, "clean"); code == 0 {
		t.Error("clean must stop while a build lock exists")
	}
	if code := h.run(dir, "build"); code == 0 || !strings.Contains(h.err.String(), "Another build is active") {
		t.Errorf("build must stop on an existing lock: %d %s", code, h.err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Error("a foreign lock must not be removed")
	}
}

func TestBookLegacyLayoutAndForwardingScript(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	book := filepath.Join(t.TempDir(), "旧书 x")
	work := filepath.Join(book, "03 制作工作区")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(src, "epub"), filepath.Join(work, "epub")); err != nil {
		t.Fatal(err)
	}
	// CRLF forwarding script, as written by an editor on Windows.
	mustWrite(t, filepath.Join(book, "build.sh"),
		"#!/bin/sh\r\nexport EPUB_OUTPUT_NAME='旧 书_V1.epub'\r\nexec sh \"$X/build.sh\" \"$(dirname \"$0\")\" \"$@\"\r\n")
	if code := h.run(book, "build"); code != 0 {
		t.Fatalf("legacy build = %d: %s", code, h.err)
	}
	dist := filepath.Join(work, "dist", "旧 书_V1.epub")
	if _, err := os.Stat(dist); err != nil {
		t.Fatalf("legacy dist name not honoured: %v", err)
	}
	if strings.TrimSpace(mustRead(t, filepath.Join(work, ".pipeline", "dist-sha256"))) != sha(t, dist) {
		t.Error("legacy receipt mismatch")
	}
	log := mustRead(t, filepath.Join(work, "交付记录.tsv"))
	if strings.Count(log, "\n") != 2 {
		t.Errorf("legacy delivery log should append: %q", log)
	}
	if code := h.run(book, "build"); code != 0 {
		t.Fatalf("legacy rebuild = %d: %s", code, h.err)
	}
	if strings.Count(mustRead(t, filepath.Join(work, "交付记录.tsv")), "\n") != 3 {
		t.Error("legacy delivery log must keep history")
	}
	if code := h.run(book, "fonts"); code != 0 || !strings.Contains(h.out.String(), "No fonts.json") {
		t.Errorf("fonts without config: %d %s", code, h.out)
	}
}

func TestBookMultiVolume(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	book := t.TempDir()
	work := filepath.Join(book, "03 制作工作区")
	for i, name := range []string{"第一册", "第二册"} {
		vol := filepath.Join(work, name)
		if i == 0 {
			if err := os.Rename(filepath.Join(src, "epub"), filepath.Join(vol, "epub")); err != nil {
				_ = os.MkdirAll(vol, 0o755)
				if err := os.Rename(filepath.Join(src, "epub"), filepath.Join(vol, "epub")); err != nil {
					t.Fatal(err)
				}
			}
		} else {
			if err := zipfs.CopyTree(context.Background(), filepath.Join(work, "第一册", "epub"), filepath.Join(vol, "epub"), nil); err != nil {
				t.Fatal(err)
			}
		}
		mustWrite(t, filepath.Join(vol, "build.sh"), "export EPUB_OUTPUT_NAME="+name+".epub\n")
	}
	mustWrite(t, filepath.Join(book, "build.sh"), "#!/bin/sh\n")
	if code := h.run(book, "build"); code != 0 {
		t.Fatalf("multi-volume build = %d: %s", code, h.err)
	}
	for _, name := range []string{"第一册", "第二册"} {
		if _, err := os.Stat(filepath.Join(work, name, "dist", name+".epub")); err != nil {
			t.Errorf("volume %s not built: %v", name, err)
		}
	}
}

func TestResolveBookLayoutDetection(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	l, err := resolveBookLayout(dir, readBookEnv())
	if err != nil || l.Kind != layoutBook || filepath.Base(l.DistDir) != "out" || filepath.Base(l.StateDir) != ".epub" {
		t.Fatalf("new layout = %+v, %v", l, err)
	}
	legacy := t.TempDir()
	mustWrite(t, filepath.Join(legacy, "03 制作工作区", "epub", "mimetype"), "application/epub+zip")
	l, err = resolveBookLayout(legacy, readBookEnv())
	if err != nil || l.Kind != layoutLegacy || filepath.Base(l.StateDir) != ".pipeline" || l.OutputName != "book.epub" {
		t.Fatalf("legacy layout = %+v, %v", l, err)
	}
	if _, err := resolveBookLayout(filepath.Join(legacy, "missing"), readBookEnv()); err == nil {
		t.Error("missing directory must be a usage error")
	}
	t.Setenv("EPUB_OUTPUT_NAME", "a/b.epub")
	l, _ = resolveBookLayout(legacy, readBookEnv())
	if validOutputName(l.OutputName) {
		t.Error("path separators in the output name must be rejected")
	}
}

func TestBookImportFreezesAndBuilds(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	original := filepath.Join(t.TempDir(), "原书 test.epub")
	if err := zipfs.PackTree(context.Background(), filepath.Join(src, "epub"), original, "mimetype"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "导入 书")
	if code := BookImport(context.Background(), BookImportOptions{Original: original, Dir: dest}, h.io()); code != 0 {
		t.Fatalf("import = %d: %s", code, h.err)
	}
	frozen := filepath.Join(dest, "original.epub")
	if sha(t, frozen) != sha(t, original) {
		t.Error("frozen original differs")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(frozen); info.Mode().Perm()&0o222 != 0 {
			t.Error("frozen original must be read-only")
		}
	}
	var cfg bookConfig
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(dest, "book.json"))), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Original == nil || cfg.Original.SHA256 != sha(t, original) {
		t.Errorf("book.json original = %+v", cfg.Original)
	}
	if _, err := os.Stat(filepath.Join(dest, ".epub", "import", "nav-audit.json")); err != nil {
		t.Error("import audit report missing")
	}
	if code := h.run(dest, "build"); code != 0 {
		t.Fatalf("build after import = %d: %s", code, h.err)
	}
	// destination already exists
	if code := BookImport(context.Background(), BookImportOptions{Original: original, Dir: dest}, h.io()); code == 0 {
		t.Error("import over an existing directory must fail")
	}
}

func TestBookImportStopsOnEncryptionAndNonOEBPS(t *testing.T) {
	h := newBookHarness(t)
	src := h.newBook("")
	tree := filepath.Join(t.TempDir(), "tree")
	if err := zipfs.CopyTree(context.Background(), filepath.Join(src, "epub"), tree, nil); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tree, "META-INF", "encryption.xml"), "<encryption/>")
	drm := filepath.Join(t.TempDir(), "drm.epub")
	if err := zipfs.PackTree(context.Background(), tree, drm, "mimetype"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "drm-book")
	if code := BookImport(context.Background(), BookImportOptions{Original: drm, Dir: dest}, h.io()); code == 0 ||
		!strings.Contains(h.err.String(), "DRM is not decrypted") || !strings.Contains(h.err.String(), "retained") {
		t.Fatalf("encrypted import must stop and retain: %d %s", code, h.err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("a stopped import must not create the destination")
	}

	// Non-OEBPS: refused by default, accepted explicitly and then built as-is.
	legacyTree := filepath.Join(t.TempDir(), "legacy")
	if err := zipfs.CopyTree(context.Background(), filepath.Join(src, "epub"), legacyTree, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(legacyTree, "OEBPS"), filepath.Join(legacyTree, "EPUB")); err != nil {
		t.Fatal(err)
	}
	container := strings.ReplaceAll(mustRead(t, filepath.Join(legacyTree, "META-INF", "container.xml")), "OEBPS/", "EPUB/")
	mustWrite(t, filepath.Join(legacyTree, "META-INF", "container.xml"), container)
	flat := filepath.Join(t.TempDir(), "flat.epub")
	if err := zipfs.PackTree(context.Background(), legacyTree, flat, "mimetype"); err != nil {
		t.Fatal(err)
	}
	dest2 := filepath.Join(t.TempDir(), "flat-book")
	if code := BookImport(context.Background(), BookImportOptions{Original: flat, Dir: dest2}, h.io()); code == 0 ||
		!strings.Contains(h.err.String(), "Non-OEBPS") {
		t.Fatalf("non-OEBPS must stop by default: %d %s", code, h.err)
	}
	dest3 := filepath.Join(t.TempDir(), "flat-book-ok")
	if code := BookImport(context.Background(), BookImportOptions{Original: flat, Dir: dest3, LegacyLayout: true}, h.io()); code != 0 {
		t.Fatalf("--legacy-layout import = %d: %s", code, h.err)
	}
	if code := h.run(dest3, "build"); code != 0 {
		t.Fatalf("legacy-layout build = %d: %s", code, h.err)
	}
}
