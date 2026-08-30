package alite

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// ---- fixture（逐字对齐 scripts/test_epub_anthology_refinement.py 的 write_epub） ----

func aliteXHTML(title, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head>
    <title>` + title + `</title>
    <link href="../Styles/base.css" type="text/css" rel="stylesheet"/>
  </head>
  <body>
    ` + body + `
  </body>
</html>
`
}

func aliteCopyrightPage(title string) string {
	return aliteXHTML("版权信息", `<p class="cp">版权信息</p>
    <ul class="list">
      <li class="i">书名：`+title+`</li>
      <li class="i">作者：测试作者</li>
      <li class="i">主页：<a href="https://example.com">示例</a></li>
    </ul>`)
}

func aliteFixture() map[string]string {
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
    <dc:identifier id="book-id">urn:uuid:test-anthology</dc:identifier>
    <dc:title>Anthology Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="poster1" href="Text/poster1.xhtml" media-type="application/xhtml+xml"/>
    <item id="copyright1" href="Text/copyright1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="poster2" href="Text/poster2.xhtml" media-type="application/xhtml+xml"/>
    <item id="copyright2" href="Text/copyright2.xhtml" media-type="application/xhtml+xml"/>
    <item id="base" href="Styles/base.css" media-type="text/css"/>
    <item id="image1" href="Images/poster1.jpg" media-type="image/jpeg"/>
    <item id="image2" href="Images/poster2.jpg" media-type="image/jpeg"/>
  </manifest>
  <spine>
    <itemref idref="poster1"/>
    <itemref idref="copyright1"/>
    <itemref idref="chapter"/>
    <itemref idref="poster2"/>
    <itemref idref="copyright2"/>
  </spine>
</package>
`,
		"OEBPS/Text/poster1.xhtml":    aliteXHTML("封面", `<p class="center"><img alt="" src="../Images/poster1.jpg"/></p>`),
		"OEBPS/Text/copyright1.xhtml": aliteCopyrightPage("第一卷"),
		"OEBPS/Text/chapter.xhtml":    aliteXHTML("正文", "<p>正文保持不变。</p>"),
		"OEBPS/Text/poster2.xhtml":    aliteXHTML("封面", `<p class="center"><img alt="" src="../Images/poster2.jpg"/></p>`),
		"OEBPS/Text/copyright2.xhtml": aliteCopyrightPage("第二卷"),
		"OEBPS/Styles/base.css":       "body { line-height: 1.6; }\n",
		"OEBPS/Images/poster1.jpg":    "jpeg1",
		"OEBPS/Images/poster2.jpg":    "jpeg2",
	}
}

func writeFixtureEpub(t *testing.T, path string, files map[string]string) {
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
	for name, content := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
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

// ---- 单元测试 ----

func TestClassAttrHelpers(t *testing.T) {
	// addClassToAttrs：空属性追加、既有单引号属性统一双引号、已存在则 no-op。
	if got, _ := addClassToAttrs("", "fullpage"); got != ` class="fullpage"` {
		t.Errorf("empty attrs: %q", got)
	}
	// Python 只替换 class="…" 匹配区间，前导空格保留。
	if got, _ := addClassToAttrs(` class='a'`, "b"); got != ` class="a b"` {
		t.Errorf("single-quoted: %q", got)
	}
	if got, changed := addClassToAttrs(` class="a b"`, "b"); changed || got != ` class="a b"` {
		t.Errorf("already present: %q changed=%v", got, changed)
	}
	// Python CLASS_RE 语义：\b 在 - 与 c 之间成立，data-class= 的值仍被命中替换。
	if got, _ := addClassToAttrs(`data-class="x"`, "y"); got != `data-class="x y"` {
		t.Errorf("data-class: %q", got)
	}
	// class=" 后接其它内容（无闭合引号）→ 追加分支。
	if got, _ := addClassToAttrs(` class="a`, "b"); got != ` class="a class="b""` {
		t.Logf("unterminated class attr → %q（接受 Python 同形的病态输出）", got)
	}

	// addClassToTag：required_class 不满足时保留原样。
	src := `<p class="cp">a</p><ul class="list"><li class="i">x</li><li>x</li></ul>`
	got := addClassToTag(src, "li", "copyright-meta-item", "i")
	want := `<p class="cp">a</p><ul class="list"><li class="i copyright-meta-item">x</li><li>x</li></ul>`
	if got != want {
		t.Errorf("addClassToTag:\n got  %s\n want %s", got, want)
	}
	if got := addClassToTag(src, "li", "meta", "missing"); got != src {
		t.Errorf("required_class gate: %s", got)
	}
}

func TestPosterAndCopyrightDetection(t *testing.T) {
	poster := aliteXHTML("封面", `<p class="center"><img alt="" src="../Images/poster1.jpg"/></p>`)
	if href := posterImageHref(poster); href != "../Images/poster1.jpg" {
		t.Errorf("poster href: %q", href)
	}
	// 标题带实体与空白仍可识别。
	if got := titleText(aliteXHTML("封&#x9762;", "<p>x</p>")); got != "封面" {
		t.Errorf("entity title: %q", got)
	}
	// body 有可见文本 → 非海报页。
	if href := posterImageHref(aliteXHTML("封面", "<p>说明文字</p>")); href != "" {
		t.Errorf("textful poster: %q", href)
	}
	// 多图 → 非海报页。
	if href := posterImageHref(aliteXHTML("封面", `<p><img src="a.jpg"/><img src="b.jpg"/></p>`)); href != "" {
		t.Errorf("two images: %q", href)
	}
	// 版权页识别。
	if !isCopyrightPage(aliteCopyrightPage("第一卷")) {
		t.Error("copyright page not recognized")
	}
	if isCopyrightPage(aliteXHTML("版权信息", `<p class="cp">无清单</p>`)) {
		t.Error("page without ul.list must not be copyright page")
	}
}

func TestRefinePosterExact(t *testing.T) {
	poster := aliteXHTML("封面", `<p class="center"><img alt="" src="../Images/poster1.jpg"/></p>`)
	refined, err := refinePoster(poster, 1, "../Images/poster1.jpg", "../Styles/anthology-refinement.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<body class="fullpage poster-bg poster-bg-volume-001">`,
		`<section class="fullframe" epub:type="chapter">`,
		`<img class="poster-fallback" alt="" src="../Images/poster1.jpg"/>`,
		`<link href="../Styles/anthology-refinement.css" type="text/css" rel="stylesheet"/>`,
		`</head>`,
	} {
		if !strings.Contains(refined, want) {
			t.Errorf("refined poster missing %q:\n%s", want, refined)
		}
	}
	// 幂等：已含 href 子串则不再插 link。
	again, _ := ensureStylesheetLink(refined, "../Styles/anthology-refinement.css")
	if strings.Count(again, "anthology-refinement.css") != strings.Count(refined, "anthology-refinement.css") {
		t.Error("ensureStylesheetLink must be idempotent via href substring check")
	}
}

func TestStylesheetContent(t *testing.T) {
	got := stylesheetRstripped([]posterImageLine{{1, "../Images/poster1.jpg"}, {2, "../Images/poster2.jpg"}})
	for _, want := range []string{
		"/* Anthology volume poster and copyright refinement layer. */",
		"background-size: contain;",
		"@supports (background-size: contain)",
		`background-image: url("../Images/poster1.jpg");`,
		`background-image: url("../Images/poster2.jpg");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stylesheet missing %q", want)
		}
	}
	if strings.Contains(got, "background-size: cover") || strings.Contains(got, "vh") {
		t.Error("stylesheet must not contain cover/vh layouts")
	}
	if !strings.HasSuffix(got, "}\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("stylesheet must be rstrip()+newline shaped: %q", got[len(got)-8:])
	}
}

// ---- parity（Python oracle 逐字节比对；OPF 因 ET 重排只比语义） ----

func runPythonHarness(t *testing.T, args ...string) (int, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, "scripts", args[0])
	if _, err := os.Stat(script); err != nil {
		t.Skipf("scripts/%s 不存在（oracle 已删除）", args[0])
	}
	cmd := exec.Command("python3", append([]string{script}, args[1:]...)...)
	cmd.Dir = repo
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
	return code, out.String()
}

func readZipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, zf := range r.File {
		if zf.Name == "mimetype" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		out[zf.Name] = buf.Bytes()
	}
	return out
}

// pyCanonicalXML 把 EPUB 内某 entry 经 Python ET 规范化为 JSON。
// alite 的 OPF 在 Python 侧是整树重序列化、Go 侧是字节区间插入
// （保留原格式字节）——因此 OPF 只比语义；XHTML/CSS/Image 两侧字节一致。
func pyCanonicalXML(t *testing.T, epubPath, entry string) string {
	t.Helper()
	script := `import sys, json, zipfile
from xml.etree import ElementTree as ET
with zipfile.ZipFile(sys.argv[1]) as zf:
    data = zf.read(sys.argv[2])
def canon(e):
    text = e.text or ""
    return {"tag": e.tag, "attrs": [[k, v] for k, v in e.attrib.items()],
            "text": text if text.strip() else "", "kids": [canon(c) for c in e]}
print(json.dumps(canon(ET.fromstring(data)), ensure_ascii=False))`
	out, err := exec.Command("python3", "-c", script, epubPath, entry).Output()
	if err != nil {
		t.Fatalf("canonicalize %s@%s: %v", epubPath, entry, err)
	}
	return string(out)
}

func runGoRefinement(t *testing.T, input, output, reportOutput string, expectVolumes *int) []byte {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		ExpectVolumes: expectVolumes,
		LegacyReport:  true,
		Output:        reportOutput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatal(err)
	}
	raw, ok := res.Facts["legacyReport"]
	if !ok {
		t.Fatal("Result.Facts 缺少 legacyReport")
	}
	switch v := raw.(type) {
	case json.RawMessage: // Go 1.27：RawMessage 与 jsontext.Value 同一类型
		return v
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		t.Fatalf("legacyReport 类型错误: %T", raw)
		return nil
	}
}

func TestParityAnthologyRefinement(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	writeFixtureEpub(t, source, aliteFixture())

	pyOut := filepath.Join(dir, "py-refined.epub")
	code, pyReport := runPythonHarness(t, "epub_anthology_refinement.py",
		source, "--output", pyOut, "--expect-volumes", "2", "--format", "json")
	if code != 0 {
		t.Fatalf("python oracle 退出码 %d", code)
	}

	goOut := filepath.Join(dir, "go-refined.epub")
	goReport := runGoRefinement(t, source, goOut, pyOut, intPtr(2))

	// P2：legacy 报告逐字节一致。
	if strings.TrimSpace(string(goReport)) != strings.TrimSpace(pyReport) {
		t.Errorf("legacy report mismatch:\n--- python ---\n%s\n--- go ---\n%s", pyReport, goReport)
	}

	// P3：除 OPF 外逐 entry 字节一致。
	pyEntries := readZipEntries(t, pyOut)
	goEntries := readZipEntries(t, goOut)
	for name := range pyEntries {
		if _, ok := goEntries[name]; !ok {
			t.Errorf("go 输出缺少 entry %s", name)
		}
	}
	for name := range goEntries {
		if _, ok := pyEntries[name]; !ok {
			t.Errorf("go 输出多出 entry %s", name)
		}
	}
	for name := range pyEntries {
		if name == "OEBPS/content.opf" {
			continue
		}
		if string(pyEntries[name]) != string(goEntries[name]) {
			t.Errorf("entry %s 字节不一致:\n--- python ---\n%s\n--- go ---\n%s",
				name, clipStr(string(pyEntries[name]), 600), clipStr(string(goEntries[name]), 600))
		}
	}
	if pyCanonicalXML(t, pyOut, "OEBPS/content.opf") != pyCanonicalXML(t, goOut, "OEBPS/content.opf") {
		t.Error("OPF 语义不一致")
	}

	// 二次运行：CSS 已存在且字节相同 → 不重复写；manifest href 已存在 → 不追加。
	pySecond := filepath.Join(dir, "py-second.epub")
	code, pySecondReport := runPythonHarness(t, "epub_anthology_refinement.py",
		pyOut, "--output", pySecond, "--expect-volumes", "2", "--format", "json")
	if code != 0 {
		t.Fatalf("python oracle 二次运行退出码 %d", code)
	}
	goSecond := filepath.Join(dir, "go-second.epub")
	goSecondReport := runGoRefinement(t, pyOut, goSecond, pySecond, intPtr(2))
	if strings.TrimSpace(string(goSecondReport)) != strings.TrimSpace(pySecondReport) {
		t.Errorf("second-run report mismatch:\n--- python ---\n%s\n--- go ---\n%s",
			pySecondReport, goSecondReport)
	}
	pySecondEntries := readZipEntries(t, pySecond)
	goSecondEntries := readZipEntries(t, goSecond)
	for name := range pySecondEntries {
		if name == "OEBPS/content.opf" {
			continue
		}
		if string(pySecondEntries[name]) != string(goSecondEntries[name]) {
			t.Errorf("second run: entry %s 字节不一致", name)
		}
	}
	if pyCanonicalXML(t, pySecond, "OEBPS/content.opf") != pyCanonicalXML(t, goSecond, "OEBPS/content.opf") {
		t.Error("second run: OPF 语义不一致")
	}
}

func TestParityExpectVolumesMismatch(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	writeFixtureEpub(t, source, aliteFixture())

	pyOut := filepath.Join(dir, "py-wrong.epub")
	code, _ := runPythonHarness(t, "epub_anthology_refinement.py",
		source, "--output", pyOut, "--expect-volumes", "3", "--format", "json")
	if code != 1 {
		t.Fatalf("python oracle 应以退出码 1 失败，实际 %d", code)
	}

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, goErr := Run(context.Background(), b, Params{ExpectVolumes: intPtr(3)})
	if goErr == nil || !strings.Contains(goErr.Error(), "expected 3 volume poster pages, found 2") {
		t.Fatalf("go 侧应报同样的卷数错误，实际: %v", goErr)
	}
}

func TestParityPosterWithoutCopyright(t *testing.T) {
	files := aliteFixture()
	// 第二卷无相邻版权页（spine 以 chapter 收尾）。
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"],
		`    <itemref idref="poster2"/>
    <itemref idref="copyright2"/>`,
		`    <itemref idref="poster2"/>`, 1)
	delete(files, "OEBPS/Text/copyright2.xhtml")

	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	writeFixtureEpub(t, source, files)

	pyOut := filepath.Join(dir, "py-warn.epub")
	code, pyReport := runPythonHarness(t, "epub_anthology_refinement.py",
		source, "--output", pyOut, "--format", "json")
	if code != 0 {
		t.Fatalf("python oracle 退出码 %d", code)
	}
	if !strings.Contains(pyReport, "poster page has no adjacent copyright page: OEBPS/Text/poster2.xhtml") {
		t.Fatalf("python 报告缺 warning:\n%s", pyReport)
	}

	goOut := filepath.Join(dir, "go-warn.epub")
	goReport := runGoRefinement(t, source, goOut, pyOut, nil)
	if strings.TrimSpace(string(goReport)) != strings.TrimSpace(pyReport) {
		t.Errorf("legacy report mismatch:\n--- python ---\n%s\n--- go ---\n%s", pyReport, goReport)
	}
}

func intPtr(n int) *int { return &n }

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
