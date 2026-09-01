package styledemo

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// repoRoot 定位仓库根（popupnotes_test 同款：包目录向上三级）。
func repoRoot(t *testing.T) string {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// goLegacyLines 用源树模式跑 Go 实现并取 legacyReport 行。
func goLegacyLines(t *testing.T, b *book.Book, demoDir string) (string, []string) {
	t.Helper()
	res, err := Run(t.Context(), b, Params{DemoDir: demoDir, LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := res.Facts["legacyReport"].(map[string]any)
	if !ok {
		t.Fatalf("legacyReport 形状错误: %T", res.Facts["legacyReport"])
	}
	lines, ok := raw["lines"].([]string)
	if !ok {
		t.Fatalf("legacyReport.lines 形状错误: %T", raw["lines"])
	}
	return res.Status, lines
}

func assertLinesContain(t *testing.T, lines []string, wants ...string) {
	t.Helper()
	joined := strings.Join(lines, "\n")
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("输出缺少 %q:\n%s", want, joined)
		}
	}
}

// buildDemoEpub 用模板自带 build.sh 构建产物 EPUB（产物落在临时目录，
// 不污染仓库 dist/）。
func buildDemoEpub(t *testing.T, repo, outPath string) {
	t.Helper()
	script := filepath.Join(repo, "templates", "epub-style-demo", "build.sh")
	cmd := exec.CommandContext(t.Context(), "sh", script, outPath)
	cmd.Dir = repo
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("build.sh 失败: %v\n%s", err, errb.String())
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("build.sh 未产出 %s: %s", outPath, out.String())
	}
}

func demoDirOf(repo string) string {
	return filepath.Join(repo, "templates", "epub-style-demo")
}

// ---- 源树默认模式 ----

func TestSourceTreeDefault(t *testing.T) {
	repo := repoRoot(t)
	status, goLines := goLegacyLines(t, nil, demoDirOf(repo))
	if status != "complete" {
		t.Fatalf("go status 应为 complete，实际 %s", status)
	}
	if len(goLines) != 1 || goLines[0] != "epub-style-demo validation ok" {
		t.Fatalf("go lines: %v", goLines)
	}
}

// ---- 构建产物模式 ----

func TestArtifactOK(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	epub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, epub)

	b, err := book.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	status, goLines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "complete" {
		t.Fatalf("go status 应为 complete，实际 %s\ngo lines: %v", status, goLines)
	}
	if len(goLines) != 1 || goLines[0] != "epub-style-demo validation ok" {
		t.Fatalf("go lines: %v", goLines)
	}
}

func TestArtifactChapterOpeningContractBroken(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)

	broken := filepath.Join(dir, "broken-chapter-opening.epub")
	rewriteEpubEntry(t, srcEpub, broken, zipPosterCSS, func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("background-size: 5.5em auto;"), []byte("background-size: 6em auto;"))
	})

	b, err := book.Open(broken)
	if err != nil {
		t.Fatalf("book.Open(broken chapter opening) 失败: %v", err)
	}
	defer b.Close()
	status, lines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesContain(t, lines,
		"ERROR: chapter-opening block background must be shared CSS at left bottom / 5.5em auto")
}

func TestArtifactChapterOpeningNavigationContractBroken(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)
	cases := []struct {
		name, entry, old, replacement, want string
	}{
		{
			name: "manifest duplicate", entry: zipPackage,
			old: `    <item id="chapter-opening-block" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>`,
			replacement: `    <item id="chapter-opening-block" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-opening-block-duplicate" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>`,
			want: "must appear exactly once in manifest",
		},
		{
			name: "spine missing", entry: zipPackage,
			old: `idref="chapter-opening-block"`, replacement: `idref="chapter-opening-block-missing"`,
			want: "must appear exactly once in spine",
		},
		{
			name: "nav missing", entry: zipNav,
			old: `href="Text/28-chapter-opening-block.xhtml"`, replacement: `href="Text/26-prosody-fallback.xhtml"`,
			want: "must appear exactly once in nav.xhtml",
		},
		{
			name: "ncx missing", entry: zipNCX,
			old: `src="Text/28-chapter-opening-block.xhtml"`, replacement: `src="Text/26-prosody-fallback.xhtml"`,
			want: "must appear exactly once in toc.ncx",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".epub")
			rewriteEpubEntry(t, srcEpub, broken, tc.entry, func(data []byte) []byte {
				return replaceBytesRequired(t, data, []byte(tc.old), []byte(tc.replacement))
			})
			b, err := book.Open(broken)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			status, lines := goLegacyLines(t, b, demoDirOf(repo))
			if status != "failed" {
				t.Fatalf("go status 应为 failed，实际 %s", status)
			}
			assertLinesContain(t, lines, tc.want)
		})
	}
}

// ---- 破坏产物（mimetype 改 deflate + 缺一个 manifest 指向的文件） ----

func TestArtifactBroken(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)

	broken := filepath.Join(dir, "broken.epub")
	corruptDemoEpub(t, srcEpub, broken)

	b, err := book.Open(broken)
	if err != nil {
		t.Fatalf("book.Open(broken) 失败: %v", err)
	}
	defer b.Close()
	status, goLines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesContain(t, goLines,
		"ERROR: EPUB mimetype must be stored",
		"ERROR: EPUB manifest href missing in zip: Text/18-english-fiction.xhtml")
}

// corruptDemoEpub 复制产物 zip：mimetype 改为 deflate，并删除
// OEBPS/Text/18-english-fiction.xhtml（manifest 指向它 → zip 缺失错误）。
func corruptDemoEpub(t *testing.T, srcPath, dstPath string) {
	t.Helper()
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		if f.Name == "OEBPS/Text/18-english-fiction.xhtml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: f.Name, Method: f.Method}
		if f.Name == "mimetype" {
			h.Method = zip.Deflate
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// ---- 产物缺 mimetype entry（KeyError 消息形状） ----

func TestArtifactNoMimetype(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)

	broken := filepath.Join(dir, "nomime.epub")
	stripMimetypeEpub(t, srcEpub, broken)

	b, err := book.Open(broken)
	if err != nil {
		t.Fatalf("book.Open(nomime) 失败: %v", err)
	}
	defer b.Close()
	status, goLines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesContain(t, goLines,
		"ERROR: EPUB mimetype must be first zip entry",
		"EPUB validation failed:")
}

// stripMimetypeEpub 复制产物 zip 但删除 mimetype entry。
func stripMimetypeEpub(t *testing.T, srcPath, dstPath string) {
	t.Helper()
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		if f.Name == "mimetype" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: f.Method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func rewriteEpubEntry(t *testing.T, srcPath, dstPath, entryName string, mutate func([]byte) []byte) {
	t.Helper()
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	found := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == entryName {
			found = true
			data = mutate(data)
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: f.Method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("EPUB 中找不到待改 entry %s", entryName)
	}
}

// ---- 破坏源树 ----

func TestBrokenSourceTree(t *testing.T) {
	repo := repoRoot(t)
	demoDir := filepath.Join(t.TempDir(), "epub-style-demo")
	if err := os.MkdirAll(demoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyTree(t, filepath.Join(repo, "templates", "epub-style-demo", "OEBPS"), filepath.Join(demoDir, "OEBPS"))
	breakDemoTree(t, demoDir)

	status, goLines := goLegacyLines(t, nil, demoDir)
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesContain(t, goLines,
		"ERROR: Spine idref missing from manifest: poster-contain",
		"ERROR: MathML content missing OPF properties=mathml: Text/16-math.xhtml",
		"ERROR: 03c-poster-contain.xhtml must be in manifest",
		"ERROR: classical-text must float left in the wide enhancement")
}

func TestSourceChapterOpeningContractBroken(t *testing.T) {
	repo := repoRoot(t)
	demoDir := filepath.Join(t.TempDir(), "epub-style-demo")
	copyTree(t, filepath.Join(repo, "templates", "epub-style-demo", "OEBPS"), filepath.Join(demoDir, "OEBPS"))
	posterPath := filepath.Join(demoDir, zipPosterCSS)
	poster := readSmall(t, posterPath)
	poster = strings.Replace(poster, "margin: 25% 5% 0 0;", "margin: 20% 5% 0 0;", 1)
	writeSmall(t, posterPath, poster)

	status, lines := goLegacyLines(t, nil, demoDir)
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesContain(t, lines,
		"ERROR: chapter-opening block title group must retain its production-derived top/right spacing")
}

func TestSourceChapterOpeningNavigationContractBroken(t *testing.T) {
	repo := repoRoot(t)
	cases := []struct {
		name, rel, old, replacement, want string
	}{
		{
			name: "manifest duplicate", rel: relPackage,
			old: `    <item id="chapter-opening-block" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>`,
			replacement: `    <item id="chapter-opening-block" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-opening-block-duplicate" href="Text/28-chapter-opening-block.xhtml" media-type="application/xhtml+xml"/>`,
			want: "must appear exactly once in manifest",
		},
		{
			name: "spine missing", rel: relPackage,
			old: `idref="chapter-opening-block"`, replacement: `idref="chapter-opening-block-missing"`,
			want: "must appear exactly once in spine",
		},
		{
			name: "nav missing", rel: relNav,
			old: `href="Text/28-chapter-opening-block.xhtml"`, replacement: `href="Text/26-prosody-fallback.xhtml"`,
			want: "must appear exactly once in nav.xhtml",
		},
		{
			name: "ncx missing", rel: relNCX,
			old: `src="Text/28-chapter-opening-block.xhtml"`, replacement: `src="Text/26-prosody-fallback.xhtml"`,
			want: "must appear exactly once in toc.ncx",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			demoDir := filepath.Join(t.TempDir(), "epub-style-demo")
			copyTree(t, filepath.Join(repo, "templates", "epub-style-demo", "OEBPS"), filepath.Join(demoDir, "OEBPS"))
			path := filepath.Join(demoDir, tc.rel)
			data := replaceBytesRequired(t, []byte(readSmall(t, path)), []byte(tc.old), []byte(tc.replacement))
			writeSmall(t, path, string(data))
			status, lines := goLegacyLines(t, nil, demoDir)
			if status != "failed" {
				t.Fatalf("go status 应为 failed，实际 %s", status)
			}
			assertLinesContain(t, lines, tc.want)
		})
	}
}

func replaceBytesRequired(t *testing.T, data, old, replacement []byte) []byte {
	t.Helper()
	updated := bytes.Replace(data, old, replacement, 1)
	if bytes.Equal(updated, data) {
		t.Fatalf("fixture mutation target not found: %q", old)
	}
	return updated
}

// breakDemoTree 与 parity 探针一致的三处破坏：
//  1. 删除 poster-contain manifest item → spine idref 与 must-be-in-manifest 错误；
//  2. 去掉 16-math 的 properties=mathml → MathML props 错误；
//  3. 删掉 literary.css classical-text 块里的 float: left。
func breakDemoTree(t *testing.T, demoDir string) {
	t.Helper()
	pkgPath := filepath.Join(demoDir, "OEBPS", "package.opf")
	pkg := readSmall(t, pkgPath)
	pkg = strings.Replace(pkg,
		"    <item id=\"poster-contain\" href=\"Text/03c-poster-contain.xhtml\" media-type=\"application/xhtml+xml\"/>\n",
		"", 1)
	pkg = strings.Replace(pkg,
		" href=\"Text/16-math.xhtml\" media-type=\"application/xhtml+xml\" properties=\"mathml\"/>",
		" href=\"Text/16-math.xhtml\" media-type=\"application/xhtml+xml\"/>", 1)
	writeSmall(t, pkgPath, pkg)

	litPath := filepath.Join(demoDir, "OEBPS", "Styles", "literary.css")
	css := readSmall(t, litPath)
	i := strings.Index(css, ".parallel-float-pair .classical-text")
	if i < 0 {
		t.Fatal("literary.css 里找不到 .parallel-float-pair .classical-text")
	}
	j := i + strings.Index(css[i:], "}")
	seg := strings.Replace(css[i:j], "float: left", "/* removed */", 1)
	writeSmall(t, litPath, css[:i]+seg+css[j:])
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		copyFile(t, s, d)
	}
}

func readSmall(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeSmall(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- 纯 Go 单测 ----

func TestPyFormatG(t *testing.T) {
	cases := []struct {
		v    float64
		want string
	}{
		{33.0, "33"},
		{33.5, "33.5"},
		{1234567.0, "1.23457e+06"},
		{0.0001, "0.0001"},
		{0.00001, "1e-05"},
	}
	for _, c := range cases {
		if got := pyFormatG(c.v); got != c.want {
			t.Errorf("pyFormatG(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestPyListOrNoneAndBool(t *testing.T) {
	if got := pyListOrNone(nil); got != "none" {
		t.Errorf("empty = %q", got)
	}
	if got := pyListOrNone([]string{"Text/16-math.xhtml"}); got != "['Text/16-math.xhtml']" {
		t.Errorf("single = %q", got)
	}
	if got := pyListOrNone([]string{"a", "b"}); got != "['a', 'b']" {
		t.Errorf("pair = %q", got)
	}
	if pyBool(true) != "True" || pyBool(false) != "False" {
		t.Errorf("pyBool mismatch")
	}
}

func TestPyPathHelpers(t *testing.T) {
	if got := stripFragment("Text/a.xhtml#sec"); got != "Text/a.xhtml" {
		t.Errorf("stripFragment = %q", got)
	}
	if got := pyJoin("OEBPS", "x"); got != "OEBPS/x" {
		t.Errorf("pyJoin rel = %q", got)
	}
	if got := pyJoin("OEBPS", "/abs"); got != "/abs" {
		t.Errorf("pyJoin abs = %q", got)
	}
	cases := []struct{ in, want string }{
		{"OEBPS/../x", "x"},
		{"../x", "../x"},
		{"OEBPS/", "OEBPS"},
		{"", "."},
		{"a/./b", "a/b"},
	}
	for _, c := range cases {
		if got := pyNormPath(c.in); got != c.want {
			t.Errorf("pyNormPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripCSSCommentsAndSelectors(t *testing.T) {
	css := "/* lead */ .img-left { width: 33%; }\n" +
		"h1, .img-left { width: 30%; }\n" +
		".figure-pair .image-pair-narrow { float: left; }\n" +
		".img-right { width: 40%; }"
	if w, ok := percentageWidth(css, ".img-left"); !ok || w != 33 {
		t.Errorf(".img-left width = %v, %v", w, ok)
	}
	if w, ok := percentageWidth(css, ".img-right"); !ok || w != 40 {
		t.Errorf(".img-right width = %v, %v", w, ok)
	}
	if _, ok := percentageWidth(css, ".missing"); ok {
		t.Errorf(".missing 不应有宽度")
	}
	blocks := selectorBlocks(css, ".img-left")
	if len(blocks) != 2 {
		t.Fatalf(".img-left 应有 2 个 block，got %d: %v", len(blocks), blocks)
	}
	narrow := selectorBlocks(css, ".figure-pair .image-pair-narrow")
	if len(narrow) != 1 || !strings.Contains(narrow[0], "float: left;") {
		t.Errorf("narrow blocks = %v", narrow)
	}
}

func TestSelectorBlockQuoteMeta(t *testing.T) {
	css := ".a.b { color: red; }\n.plain { color: blue; }"
	if got := selectorBlock(css, ".a.b"); got != " color: red; " {
		t.Errorf("selectorBlock(.a.b) = %q", got)
	}
	if got := selectorBlock(css, ".plain"); got != " color: blue; " {
		t.Errorf("selectorBlock(.plain) = %q", got)
	}
	if got := selectorBlock(css, ".none"); got != "" {
		t.Errorf("selectorBlock(.none) = %q", got)
	}
}

func TestHasBodyFontLockedMarkup(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`<body class="a body-font-locked b">`, true},
		{`<body class='body-font-locked'>`, true},
		{`<BODY CLASS="Body-Font-Locked">`, true},
		{`<body class="body-font-lockedx">`, false},
		{`<body class="body-font-locked`, false},
		{`<body>`, false},
		{`<div class="body-font-locked">`, false},
	}
	for _, c := range cases {
		if got := hasBodyFontLockedMarkup(c.in); got != c.want {
			t.Errorf("hasBodyFontLockedMarkup(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHasDirectBodyFontFamily(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"body { font-family: serif }", true},
		{"h1, body { font-family: serif }", true},
		{"body { FONT-FAMILY : serif }", true},
		{"body.something { font-family: serif }", false},
		{".a { font-family: serif }", false},
		{"body { color: red }", false},
	}
	for _, c := range cases {
		if got := hasDirectBodyFontFamily(c.in); got != c.want {
			t.Errorf("hasDirectBodyFontFamily(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHasIbooksSpecifiedFonts(t *testing.T) {
	opf := func(meta string) string {
		return `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf"><metadata>` +
			`<meta property="dcterms:modified">2026-01-01T00:00:00Z</meta>` + meta +
			`</metadata></package>`
	}
	cases := []struct {
		in   string
		want bool
	}{
		{opf(`<meta property="ibooks:specified-fonts">true</meta>`), true},
		{opf(`<meta property="ibooks:specified-fonts"> TRUE </meta>`), true},
		{opf(`<meta property="ibooks:specified-fonts">false</meta>`), false},
		{opf(``), false},
	}
	for _, c := range cases {
		root, err := parseXMLDoc([]byte(c.in))
		if err != nil {
			t.Fatal(err)
		}
		if got := hasIbooksSpecifiedFonts(root); got != c.want {
			t.Errorf("hasIbooksSpecifiedFonts(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestManifestAndHrefMaps(t *testing.T) {
	opfXML := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <manifest>
    <item id="a" href="one.xhtml" properties="nav"/>
    <item id="b" href="two.xhtml"/>
    <item id="a" href="one-last.xhtml" properties="mathml"/>
    <item id="c" href="three.xhtml" properties="nav"/>
    <item href="no-id.xhtml"/>
  </manifest>
  <spine><itemref idref="a"/><itemref idref="ghost"/></spine>
</package>`
	root, err := parseXMLDoc([]byte(opfXML))
	if err != nil {
		t.Fatal(err)
	}
	items := findallPath(root, [2]string{opfNS, "manifest"}, [2]string{opfNS, "item"})
	if len(items) != 5 {
		t.Fatalf("manifest items = %d, want 5", len(items))
	}
	m := buildManifestMap(items)
	if len(m.order) != 3 || m.order[0] != "a" || m.order[1] != "b" || m.order[2] != "c" {
		t.Errorf("manifest order = %v, want [a b c]（无 id 项跳过）", m.order)
	}
	last, _ := m.get("a")
	if got := last.attrOr("href"); got != "one-last.xhtml" {
		t.Errorf("同 id 后者覆盖: %q", got)
	}
	if !m.has("b") || m.has("ghost") {
		t.Errorf("manifest.has 错误")
	}
	navCount := 0
	for _, it := range m.values() {
		if tokenSetOf(it.attrOr("properties"))["nav"] {
			navCount++
		}
	}
	if navCount != 1 {
		t.Errorf("nav 计数 = %d, want 1", navCount)
	}
	h := buildHrefMap(m.values())
	if len(h.order) != 3 || h.order[0] != "one-last.xhtml" || h.order[1] != "two.xhtml" || h.order[2] != "three.xhtml" {
		t.Errorf("href order = %v", h.order)
	}
	if h.has("Text/16-math.xhtml") {
		t.Errorf("href map 不应包含未声明的 href")
	}
	if !h.has("one-last.xhtml") {
		t.Errorf("href map 应包含 one-last.xhtml")
	}
}

func TestFindAllDescAndIterAll(t *testing.T) {
	doc := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body><nav><ul><li><a href="one.xhtml">1</a></li></ul></nav>
  <svg xmlns="http://www.w3.org/2000/svg"><g/></svg></body>
</html>`
	root, err := parseXMLDoc([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	links := findAllDesc(root, xhtmlNS, "a")
	if len(links) != 1 || links[0].attrOr("href") != "one.xhtml" {
		t.Errorf("links = %v", links)
	}
	iter := iterAll(root)
	found := false
	for _, el := range iter {
		if el.space == svgURI && el.local == "svg" {
			found = true
		}
	}
	if !found {
		t.Error("iterAll 应包含 svg 元素")
	}
}

func TestRunRequiresDemoDir(t *testing.T) {
	if _, err := Run(t.Context(), nil, Params{}); err == nil {
		t.Error("b=nil 且无 demo_dir 应返回错误")
	}
	empty := t.TempDir()
	if _, err := Run(t.Context(), nil, Params{DemoDir: empty}); err == nil {
		t.Error("空 demo 目录缺 package.opf 应返回错误（对齐 Python 未捕获异常）")
	}
	if _, err := Run(t.Context(), &book.Book{}, Params{}); err == nil {
		t.Error("b 非 nil 且无 demo_dir 应返回错误")
	}
}

func TestRunSourceTreeOK(t *testing.T) {
	repo := repoRoot(t)
	res, err := Run(t.Context(), nil, Params{DemoDir: demoDirOf(repo), LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s", res.Status)
	}
	if res.Facts["mode"] != "source-tree" {
		t.Errorf("mode = %v", res.Facts["mode"])
	}
	lines := res.Facts["legacyReport"].(map[string]any)["lines"].([]string)
	if len(lines) != 1 || lines[0] != "epub-style-demo validation ok" {
		t.Errorf("lines = %v", lines)
	}
}
