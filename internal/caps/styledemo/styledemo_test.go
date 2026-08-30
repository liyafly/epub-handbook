package styledemo

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

const oracleScript = "scripts/validate_epub_style_demo.py"

// runPyOracle 运行 Python oracle，返回退出码与 stdout/stderr。
// python3 或脚本不存在时跳过（oracle 可能已按 SPEC §5.3 删除）。
func runPyOracle(t *testing.T, scriptPath, workDir string, args ...string) (int, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Skipf("%s 不存在（oracle 已删除）", oracleScript)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("parity 用例需要 python3")
	}
	cmd := exec.Command("python3", append([]string{scriptPath}, args...)...)
	cmd.Dir = workDir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	code := 0
	if ee, ok := runErr.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if runErr != nil {
		t.Fatalf("运行 python oracle 失败: %v\n%s", runErr, errb.String())
	}
	return code, out.String(), errb.String()
}

// goLegacyLines 用源树模式跑 Go 实现并取 legacyReport 行。
func goLegacyLines(t *testing.T, b *book.Book, demoDir string) (string, []string) {
	t.Helper()
	res, err := Run(context.Background(), b, Params{DemoDir: demoDir, LegacyReport: true})
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

// normalizePyArtifactStderr 剥离 epubcheck 环境相关的 WARN 行。
// Go 侧不执行 epubcheck（INV-4），该行不进 legacyReport。
func normalizePyArtifactStderr(stderr string) ([]string, bool) {
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "WARN: epubcheck skipped") {
			continue
		}
		if strings.Contains(line, "epubcheck failed") {
			return nil, false
		}
		lines = append(lines, line)
	}
	return lines, true
}

func assertLinesEqual(t *testing.T, pyLines, goLines []string) {
	t.Helper()
	if strings.Join(pyLines, "\n") != strings.Join(goLines, "\n") {
		t.Errorf("输出行不一致:\n--- python ---\n%s\n--- go ---\n%s",
			strings.Join(pyLines, "\n"), strings.Join(goLines, "\n"))
	}
}

// buildDemoEpub 用模板自带 build.sh 构建产物 EPUB（产物落在临时目录，
// 不污染仓库 dist/）。zip 缺失时跳过。
func buildDemoEpub(t *testing.T, repo, outPath string) {
	t.Helper()
	if _, err := exec.LookPath("zip"); err != nil {
		t.Skip("构建 demo 产物需要 zip 命令")
	}
	script := filepath.Join(repo, "templates", "epub-style-demo", "build.sh")
	cmd := exec.Command("sh", script, outPath)
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

// ---- parity：源树默认模式 ----

func TestParitySourceTreeDefault(t *testing.T) {
	repo := repoRoot(t)
	code, stdout, stderr := runPyOracle(t, filepath.Join(repo, oracleScript), repo)
	if code != 0 {
		t.Fatalf("python oracle 应退出 0，实际 %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "epub-style-demo validation ok" {
		t.Fatalf("python stdout: %q", stdout)
	}
	var pyLines []string
	for _, line := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
		if line != "" {
			pyLines = append(pyLines, line)
		}
	}

	status, goLines := goLegacyLines(t, nil, demoDirOf(repo))
	if code == 0 && status != "complete" {
		t.Fatalf("go status 应为 complete，实际 %s", status)
	}
	if len(pyLines) == 0 {
		pyLines = []string{"epub-style-demo validation ok"}
	}
	assertLinesEqual(t, pyLines, goLines)
}

// ---- parity：构建产物模式 ----

func TestParityArtifactOK(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	epub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, epub)

	code, stdout, stderr := runPyOracle(t, filepath.Join(repo, oracleScript), repo, "--epub", epub)
	if code != 0 {
		t.Fatalf("python oracle 应退出 0，实际 %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "epub-style-demo validation ok" {
		t.Fatalf("python stdout: %q", stdout)
	}

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

// ---- parity：破坏产物（mimetype 改 deflate + 缺一个 manifest 指向的文件） ----

func TestParityArtifactBroken(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)

	broken := filepath.Join(dir, "broken.epub")
	corruptDemoEpub(t, srcEpub, broken)

	code, _, stderr := runPyOracle(t, filepath.Join(repo, oracleScript), repo, "--epub", broken)
	if code != 1 {
		t.Fatalf("python oracle 应退出 1，实际 %d\nstderr: %s", code, stderr)
	}
	pyLines, ok := normalizePyArtifactStderr(stderr)
	if !ok {
		t.Skip("环境里 epubcheck 可用且产物触发 epubcheck 失败（环境相关，跳过）")
	}

	b, err := book.Open(broken)
	if err != nil {
		t.Fatalf("book.Open(broken) 失败: %v", err)
	}
	defer b.Close()
	status, goLines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesEqual(t, pyLines, goLines)
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

// ---- parity：产物缺 mimetype entry（KeyError 消息形状） ----

func TestParityArtifactNoMimetype(t *testing.T) {
	repo := repoRoot(t)
	dir := t.TempDir()
	srcEpub := filepath.Join(dir, "demo.epub")
	buildDemoEpub(t, repo, srcEpub)

	broken := filepath.Join(dir, "nomime.epub")
	stripMimetypeEpub(t, srcEpub, broken)

	code, _, stderr := runPyOracle(t, filepath.Join(repo, oracleScript), repo, "--epub", broken)
	if code != 1 {
		t.Fatalf("python oracle 应退出 1，实际 %d\nstderr: %s", code, stderr)
	}
	pyLines, ok := normalizePyArtifactStderr(stderr)
	if !ok {
		t.Skip("环境里 epubcheck 可用且产物触发 epubcheck 失败（环境相关，跳过）")
	}

	b, err := book.Open(broken)
	if err != nil {
		t.Fatalf("book.Open(nomime) 失败: %v", err)
	}
	defer b.Close()
	status, goLines := goLegacyLines(t, b, demoDirOf(repo))
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesEqual(t, pyLines, goLines)
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

// ---- parity：破坏源树（临时布局 + 拷贝 oracle 脚本） ----

func TestParityBrokenSourceTree(t *testing.T) {
	repo := repoRoot(t)
	if _, err := os.Stat(filepath.Join(repo, oracleScript)); err != nil {
		t.Skipf("scripts/%s 不存在（oracle 已随迁移删除）", oracleScript)
	}
	layout := t.TempDir() // <layout>/scripts/… + <layout>/templates/epub-style-demo/OEBPS
	scriptsDir := filepath.Join(layout, "scripts")
	demoDir := filepath.Join(layout, "templates", "epub-style-demo")
	if err := os.MkdirAll(filepath.Join(scriptsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(demoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(repo, oracleScript), filepath.Join(scriptsDir, "validate_epub_style_demo.py"))
	copyTree(t, filepath.Join(repo, "templates", "epub-style-demo", "OEBPS"), filepath.Join(demoDir, "OEBPS"))
	breakDemoTree(t, demoDir)

	// Python：脚本 ROOT = <layout>（从脚本自身位置推导），无需参数。
	code, _, stderr := runPyOracle(t, filepath.Join(scriptsDir, "validate_epub_style_demo.py"), layout)
	if code != 1 {
		t.Fatalf("python oracle 应退出 1，实际 %d\nstderr: %s", code, stderr)
	}
	var pyLines []string
	for _, line := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
		if line != "" {
			pyLines = append(pyLines, line)
		}
	}

	status, goLines := goLegacyLines(t, nil, demoDir)
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	assertLinesEqual(t, pyLines, goLines)
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
	if _, err := Run(context.Background(), nil, Params{}); err == nil {
		t.Error("b=nil 且无 demo_dir 应返回错误")
	}
	empty := t.TempDir()
	if _, err := Run(context.Background(), nil, Params{DemoDir: empty}); err == nil {
		t.Error("空 demo 目录缺 package.opf 应返回错误（对齐 Python 未捕获异常）")
	}
	if _, err := Run(context.Background(), &book.Book{}, Params{}); err == nil {
		t.Error("b 非 nil 且无 demo_dir 应返回错误")
	}
}

func TestRunSourceTreeOK(t *testing.T) {
	repo := repoRoot(t)
	res, err := Run(context.Background(), nil, Params{DemoDir: demoDirOf(repo), LegacyReport: true})
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
