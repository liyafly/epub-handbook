package csscleanup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// ---- fixture（逐字对齐 scripts/test_epub_css_cleanup.py） ----

func cssCleanupChapter(cssName, body, extraCSS string) string {
	if body == "" {
		body = "<h1>标题</h1>\n" +
			"    <p>正文（补充说明）继续。</p>\n" +
			"    <aside epub:type=\"footnote\" role=\"doc-footnote\"><p>脚注（不要二次缩小）</p></aside>"
	}
	extraLink := ""
	if extraCSS != "" {
		extraLink = "    <link href=\"../Styles/" + extraCSS + "\" type=\"text/css\" rel=\"stylesheet\"/>\n"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head>
    <title>Fixture</title>
    <link href="../Styles/` + cssName + `" type="text/css" rel="stylesheet"/>
` + extraLink + `  </head>
  <body>
    ` + body + `
  </body>
</html>
`
}

func cssCleanupLegacyCSS(color string) string {
	return "————————————————标题————————————————\n" +
		"h1 {\n" +
		"  color: " + color + ";\n" +
		"  font-family: \"SimHei\";\n" +
		"}\n" +
		"body {\n" +
		"  font-family: \"cnepub\",serif;\n" +
		"}\n" +
		".part-text {\n" +
		"  font-family: \"STKaiti\"\n" +
		"}\n"
}

func cssCleanupFixtureFiles() map[string]string {
	tocBody := "<p class=\"toc\">目录甲</p>"
	tocBody2 := "<p class=\"toc\">目录乙</p>"
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:test-css-cleanup</dc:identifier>
    <dc:title>CSS Cleanup Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="c1" href="Text/chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="Text/chapter2.xhtml" media-type="application/xhtml+xml"/>
    <item id="c3" href="Text/chapter3.xhtml" media-type="application/xhtml+xml"/>
    <item id="toc1" href="Text/toc1.xhtml" media-type="application/xhtml+xml"/>
    <item id="toc2" href="Text/toc2.xhtml" media-type="application/xhtml+xml"/>
    <item id="s2" href="Styles/style0002.css" media-type="text/css"/>
    <item id="s3" href="Styles/style0003.css" media-type="text/css"/>
    <item id="s4" href="Styles/style0004.css" media-type="text/css"/>
    <item id="s5" href="Styles/style0005.css" media-type="text/css"/>
    <item id="s6" href="Styles/style0006.css" media-type="text/css"/>
    <item id="component" href="Styles/component.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
    <itemref idref="c3"/>
    <itemref idref="toc1"/>
    <itemref idref="toc2"/>
  </spine>
</package>
`,
		"OEBPS/Text/chapter1.xhtml":      cssCleanupChapter("style0002.css", "", "component.css"),
		"OEBPS/Text/chapter2.xhtml":      cssCleanupChapter("style0004.css", "", "component.css"),
		"OEBPS/Text/chapter3.xhtml":      cssCleanupChapter("style0006.css", "", "component.css"),
		"OEBPS/Text/toc1.xhtml":          cssCleanupChapter("style0003.css", tocBody, ""),
		"OEBPS/Text/toc2.xhtml":          cssCleanupChapter("style0005.css", tocBody2, ""),
		"OEBPS/Styles/style0002.css":     cssCleanupLegacyCSS("#876c4f"),
		"OEBPS/Styles/style0003.css":     ".toc { margin-left: 0; }\n",
		"OEBPS/Styles/style0004.css":     cssCleanupLegacyCSS("#876c4f"),
		"OEBPS/Styles/style0005.css":     ".toc { margin-left: 0; }\n",
		"OEBPS/Styles/style0006.css":     cssCleanupLegacyCSS("#3fbbd6"),
		"OEBPS/Styles/component.css":     ".component { margin: 0 auto; }\n",
	}
}

func buildFixtureEpub(t *testing.T, path string, files map[string]string) {
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
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(files[name])); err != nil {
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

// readZipData 读取 zip 全部 entry 的解压内容（懒句柄不可跨 Close 使用，
// 故一次性读出）。
func readZipData(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string][]byte{}
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
		out[f.Name] = data
	}
	return out
}

func mustRun(t *testing.T, input, output string, mergeScoped bool) legacyCleanupReport {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Output: output, MergeScopedLocalCSS: mergeScoped, LegacyReport: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	var rep legacyCleanupReport
	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatal("缺少 legacyReport")
	}
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

// ---- 单测（镜像 scripts/test_epub_css_cleanup.py 的断言） ----

func TestCSSCleanupFixture(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	output := filepath.Join(dir, "cleaned.epub")
	buildFixtureEpub(t, source, cssCleanupFixtureFiles())

	rep := mustRun(t, source, output, true)
	if rep.CSSFilesBefore != 6 || rep.FactoredStylesheets != 3 ||
		rep.DuplicateStylesheetsRemoved != 1 || rep.ScopedLocalStylesheetsMerged != 2 ||
		rep.ScopeClassesAdded != 3 {
		t.Fatalf("报告计数不符: %+v", rep)
	}
	if rep.OverridesCreated != 1 || rep.FontDeclarationsRewritten != 9 ||
		rep.XHTMLFilesUpdated != 4 || rep.CSSManifestItemsRemoved != 5 ||
		rep.CSSManifestItemsAdded != 2 || rep.CSSFilesAfter != 3 || len(rep.Warnings) != 0 {
		t.Fatalf("报告计数不符: %+v", rep)
	}

	files := readZipData(t, output)
	for _, name := range []string{
		"OEBPS/Styles/clean-shared-01.css",
		"OEBPS/Styles/clean-scoped-local.css",
		"OEBPS/Styles/component.css",
	} {
		if _, ok := files[name]; !ok {
			t.Fatalf("缺少 %s", name)
		}
	}
	for _, name := range []string{
		"OEBPS/Styles/clean-override-style0006.css",
		"OEBPS/Styles/style0003.css",
		"OEBPS/Styles/style0005.css",
	} {
		if _, ok := files[name]; ok {
			t.Fatalf("不应存在 %s", name)
		}
	}

	shared := string(files["OEBPS/Styles/clean-shared-01.css"])
	for _, want := range []string{
		`"Songti SC", "SimSun", "Noto Serif CJK SC", serif`,
		`"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`,
		`"Kaiti SC", "STKaiti", "KaiTi", serif`,
	} {
		if !strings.Contains(shared, want) {
			t.Fatalf("shared 缺少 %q:\n%s", want, shared)
		}
	}
	if strings.Contains(shared, "————————————————") {
		t.Fatalf("shared 不应包含装饰行:\n%s", shared)
	}

	scoped := string(files["OEBPS/Styles/clean-scoped-local.css"])
	for _, want := range []string{"#3fbbd6", "body.css-local-01 h1", "body.css-local-02 .toc"} {
		if !strings.Contains(scoped, want) {
			t.Fatalf("scoped 缺少 %q:\n%s", want, scoped)
		}
	}

	chapter1 := string(files["OEBPS/Text/chapter1.xhtml"])
	chapter3 := string(files["OEBPS/Text/chapter3.xhtml"])
	toc1 := string(files["OEBPS/Text/toc1.xhtml"])
	for _, check := range []struct{ text, want string }{
		{chapter1, `href="../Styles/clean-shared-01.css"`},
		{chapter1, `href="../Styles/component.css"`},
		{chapter3, `href="../Styles/clean-scoped-local.css"`},
		{chapter3, `class="css-local-01"`},
		{toc1, `href="../Styles/clean-scoped-local.css"`},
		{toc1, `class="css-local-02"`},
		{chapter1, "正文（补充说明）继续。"},
	} {
		if !strings.Contains(check.text, check.want) {
			t.Fatalf("缺少 %q:\n%s", check.want, check.text)
		}
	}

	// OPF manifest：css href 集合断言。
	opfHrefs := manifestCSSHrefs(t, files["OEBPS/content.opf"])
	for _, want := range []string{"Styles/clean-shared-01.css", "Styles/clean-scoped-local.css", "Styles/component.css"} {
		if !contains(opfHrefs, want) {
			t.Fatalf("manifest 缺少 %s: %v", want, opfHrefs)
		}
	}
	for _, gone := range []string{"Styles/clean-override-style0006.css", "Styles/style0003.css", "Styles/style0005.css"} {
		if contains(opfHrefs, gone) {
			t.Fatalf("manifest 不应包含 %s: %v", gone, opfHrefs)
		}
	}

	// 幂等：对输出再跑一遍，报告归零、产物字节一致。
	secondOutput := filepath.Join(dir, "cleaned-again.epub")
	second := mustRun(t, output, secondOutput, true)
	if second.CSSFilesBefore != 3 || second.CSSFilesAfter != 3 ||
		second.FactoredStylesheets != 0 || second.ScopedLocalStylesheetsMerged != 0 {
		t.Fatalf("第二次运行计数不符: %+v", second)
	}
	secondFiles := readZipData(t, secondOutput)
	if _, ok := secondFiles["OEBPS/Styles/clean-shared-01-2.css"]; ok {
		t.Fatal("第二次运行不应生成 clean-shared-01-2.css")
	}
	firstBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("幂等性失败：第二次运行产物与输入不一致")
	}
}

func manifestCSSHrefs(t *testing.T, opfData []byte) []string {
	t.Helper()
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, item := range opfManifestItems(root) {
		mt, _ := nodeAttr(item, "media-type")
		if mt != "text/css" {
			continue
		}
		href, _ := nodeAttr(item, "href")
		out = append(out, href)
	}
	return out
}

// ---- parity（同一 fixture 分别跑 Python oracle 与 Go 实现，逐 entry 比对） ----

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(old) }
}

func pythonScriptPath(t *testing.T, name string) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repoRoot, "scripts", name)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("scripts/%s 不存在（oracle 已删除）", name)
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用")
	}
	return script
}

func runPythonJSON(t *testing.T, dir, script string, args ...string) map[string]any {
	t.Helper()
	full := append([]string{script}, args...)
	cmd := exec.Command("python3", full...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("python oracle 运行失败: %v\nstderr: %s", err, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("python oracle 输出不是 JSON: %v\nstdout: %s", err, stdout.String())
	}
	return out
}

// canonOPF 用同一解析器+序列化规则把两边 OPF 归一成可比较文本
// （Go 对 OPF 做字节区间编辑，Python oracle 做 ET 整体重写 —— P3 允许差异，
// 这里比对语义等价）。
func canonOPF(t *testing.T, path string) string {
	t.Helper()
	data := readZipData(t, path)
	opfData, ok := data["OEBPS/content.opf"]
	if !ok {
		t.Fatalf("%s: 缺少 OEBPS/content.opf", path)
	}
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var b strings.Builder
	writeCanonNode(&b, root)
	return b.String()
}

func writeCanonNode(b *strings.Builder, n *opf.SpanNode) {
	attrs := make([]string, 0, len(n.Attrs))
	for _, a := range n.Attrs {
		attrs = append(attrs, fmt.Sprintf(` %s="%s"`, a.Name.Local, canonEscape(a.Value)))
	}
	sort.Strings(attrs)
	name := n.Name.Local
	if len(n.Kids) == 0 && n.Text == "" {
		fmt.Fprintf(b, "<%s%s />", name, strings.Join(attrs, ""))
		return
	}
	fmt.Fprintf(b, "<%s%s>%s", name, strings.Join(attrs, ""), canonEscape(n.Text))
	for _, k := range n.Kids {
		writeCanonNode(b, k)
	}
	fmt.Fprintf(b, "</%s>", name)
}

func canonEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func compareEpubEntries(t *testing.T, pyPath, goPath string) {
	t.Helper()
	pyFiles := readZipData(t, pyPath)
	goFiles := readZipData(t, goPath)
	if len(pyFiles) != len(goFiles) {
		t.Fatalf("entry 数不一致: python=%d go=%d", len(pyFiles), len(goFiles))
	}
	for name, pData := range pyFiles {
		gData, ok := goFiles[name]
		if !ok {
			t.Fatalf("Go 产物缺少 entry %s", name)
		}
		if name == "OEBPS/content.opf" {
			continue // OPF：字节区间编辑 vs ET 重写，P3 预期差异，语义另行比对
		}
		if !bytes.Equal(pData, gData) {
			t.Fatalf("entry %s 内容不一致\npython=%q\ngo=%q", name, pData, gData)
		}
	}
	if canonOPF(t, pyPath) != canonOPF(t, goPath) {
		t.Fatalf("OPF 语义不一致:\n--- python ---\n%s\n--- go ---\n%s", canonOPF(t, pyPath), canonOPF(t, goPath))
	}
}

func parityCaseCSSCleanup(t *testing.T, mergeScoped bool) {
	t.Helper()
	script := pythonScriptPath(t, "epub_css_cleanup.py")
	dir := t.TempDir()
	buildFixtureEpub(t, filepath.Join(dir, "fixture.epub"), cssCleanupFixtureFiles())

	args := []string{"fixture.epub", "--output", "py-out.epub", "--format", "json"}
	if mergeScoped {
		args = append(args, "--merge-scoped-local-css")
	}
	pyReport := runPythonJSON(t, dir, script, args...)
	if err := os.Rename(filepath.Join(dir, "py-out.epub"), filepath.Join(dir, "py.epub")); err != nil {
		t.Fatal(err)
	}

	restore := chdir(t, dir)
	defer restore()
	b, err := book.Open("fixture.epub")
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	res, err := Run(context.Background(), b, Params{Output: "py-out.epub", MergeScopedLocalCSS: mergeScoped, LegacyReport: true})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	if err := b.WriteTo("go-out.epub"); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	b.Close()
	restore()

	compareEpubEntries(t, filepath.Join(dir, "py.epub"), filepath.Join(dir, "go-out.epub"))

	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatal("缺少 legacyReport")
	}
	var goReport map[string]any
	if err := json.Unmarshal(raw, &goReport); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(goReport, pyReport) {
		goJSON, _ := json.MarshalIndent(goReport, "", "  ")
		pyJSON, _ := json.MarshalIndent(pyReport, "", "  ")
		t.Fatalf("legacy 报告不一致:\n--- go ---\n%s\n--- python ---\n%s", goJSON, pyJSON)
	}

	// 第二轮（幂等 parity）：Python 处理自己的产物，Go 处理自己的产物。
	py2Args := []string{"py.epub", "--output", "py2.epub", "--format", "json"}
	if mergeScoped {
		py2Args = append(py2Args, "--merge-scoped-local-css")
	}
	runPythonJSON(t, dir, script, py2Args...)
	b2, err := book.Open(filepath.Join(dir, "go-out.epub"))
	if err != nil {
		t.Fatalf("book.Open 2: %v", err)
	}
	defer b2.Close()
	if _, err := Run(context.Background(), b2, Params{Output: filepath.Join(dir, "go2.epub"), MergeScopedLocalCSS: mergeScoped}); err != nil {
		t.Fatalf("Go Run 2: %v", err)
	}
	if err := b2.WriteTo(filepath.Join(dir, "go2.epub")); err != nil {
		t.Fatal(err)
	}
	py2 := filepath.Join(dir, "py2.epub")
	if _, err := os.Stat(py2); err == nil {
		compareEpubEntries(t, py2, filepath.Join(dir, "go2.epub"))
	}
}

func TestParityCSSCleanup(t *testing.T)          { parityCaseCSSCleanup(t, false) }
func TestParityCSSCleanupScopedMerge(t *testing.T) { parityCaseCSSCleanup(t, true) }

// TestSanitizeCSSUnits 覆盖 sanitize 的三个子变换与 normalize/shape 语义。
func TestSanitizeCSSUnits(t *testing.T) {
	got, rewrites := sanitizeCSS("————————————————标题————————————————\nh1 {\n  font-family: \"SimHei\";\n}\n")
	if rewrites != 1 || strings.Contains(got, "——") || !strings.Contains(got, heiChain) {
		t.Fatalf("sanitizeCSS 不符: %q, %d", got, rewrites)
	}
	got, _ = sanitizeCSS("p {\n  margin: 0\n  padding: 0;\n}\n")
	if !strings.Contains(got, "margin: 0;") {
		t.Fatalf("补分号失败: %q", got)
	}
	if _, ok := parseStylesheet("@font-face { font-family: X; src: url(a.ttf); }"); ok {
		t.Fatal("@ 规则（声明块）应导致整表不可解析")
	}
	// @media/@supports 头不构成规则，内层规则照常捕获（对齐 RULE_RE 语义）。
	if rules, ok := parseStylesheet("@media screen { .a { color: red; } }"); !ok ||
		len(rules) != 1 || rules[0].selector != ".a" {
		t.Fatalf("@media 内层规则应被捕获: %+v", rules)
	}
	if _, ok := parseStylesheet("/* 只剩注释 */"); ok {
		t.Fatal("空样式表应不可解析")
	}
	rules, ok := parseStylesheet("h1 { color: red; FONT-SIZE: 2em }\n")
	if !ok || rules[0].selector != "h1" || rules[0].declarations[0] != [2]string{"color", "red"} ||
		rules[0].declarations[1] != [2]string{"FONT-SIZE", "2em"} {
		t.Fatalf("parseStylesheet 不符: %+v", rules)
	}
	if systemFontFamily(`"CNEPUB",SERIF`) != songChain {
		t.Fatal("字体链查表（压缩+小写）失败")
	}
}
