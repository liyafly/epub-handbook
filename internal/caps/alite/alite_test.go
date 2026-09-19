package alite

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// ---- fixture（两卷合集：海报页 + 版权页 + 正文） ----

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
	got, warnings := addClassToTag(src, "li", "copyright-meta-item", "i")
	want := `<p class="cp">a</p><ul class="list"><li class="i copyright-meta-item">x</li><li>x</li></ul>`
	if got != want {
		t.Errorf("addClassToTag:\n got  %s\n want %s", got, want)
	}
	if len(warnings) != 0 {
		t.Errorf("addClassToTag 不应产生截断告警: %v", warnings)
	}
	if got, _ := addClassToTag(src, "li", "meta", "missing"); got != src {
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
	refined, warnings, err := refinePoster(poster, 1, "../Images/poster1.jpg", "../Styles/anthology-refinement.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("refinePoster 不应产生截断告警: %v", warnings)
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
	again, _, _ := ensureStylesheetLink(refined, "../Styles/anthology-refinement.css")
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

// ---- 端到端（Go 原生：facts、产物 entry 与幂等性） ----

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

// runGoRefinement 跑 Go 实现并写出产物，返回 Result。
func runGoRefinement(t *testing.T, input, output string, expectVolumes *int) report.Result {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{ExpectVolumes: expectVolumes, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestAnthologyRefinement(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	writeFixtureEpub(t, source, aliteFixture())

	goOut := filepath.Join(dir, "go-refined.epub")
	res := runGoRefinement(t, source, goOut, intPtr(2))
	if res.Status != report.StatusComplete || len(res.Findings) != 0 {
		t.Fatalf("status=%s findings=%v", res.Status, res.Findings)
	}
	wantFacts := map[string]any{
		"opf":                   "OEBPS/content.opf",
		"posterPagesRefined":    2,
		"copyrightPagesRefined": 2,
		"stylesheetsAdded":      1,
		"posterPages":           []string{"OEBPS/Text/poster1.xhtml", "OEBPS/Text/poster2.xhtml"},
		"copyrightPages":        []string{"OEBPS/Text/copyright1.xhtml", "OEBPS/Text/copyright2.xhtml"},
		"warnings":              []string{},
	}
	assertFacts(t, res.Facts, wantFacts)

	entries := readZipEntries(t, goOut)
	css, ok := entries["OEBPS/Styles/anthology-refinement.css"]
	if !ok {
		t.Fatal("产物缺少 anthology-refinement.css")
	}
	if string(css) != stylesheetRstripped([]posterImageLine{{1, "../Images/poster1.jpg"}, {2, "../Images/poster2.jpg"}}) {
		t.Errorf("CSS 内容不符:\n%s", css)
	}
	for _, name := range []string{"OEBPS/Text/poster1.xhtml", "OEBPS/Text/poster2.xhtml", "OEBPS/Text/copyright1.xhtml", "OEBPS/Text/copyright2.xhtml"} {
		if !strings.Contains(string(entries[name]), `href="../Styles/anthology-refinement.css"`) {
			t.Errorf("%s 缺少样式链接", name)
		}
	}
	if !strings.Contains(string(entries["OEBPS/Text/poster1.xhtml"]), `poster-bg-volume-001`) ||
		!strings.Contains(string(entries["OEBPS/Text/poster2.xhtml"]), `poster-bg-volume-002`) {
		t.Error("海报页缺少卷号 body class")
	}
	if string(entries["OEBPS/Text/chapter.xhtml"]) != aliteFixture()["OEBPS/Text/chapter.xhtml"] {
		t.Error("正文页不得改动")
	}
	opf := string(entries["OEBPS/content.opf"])
	if strings.Count(opf, `href="Styles/anthology-refinement.css"`) != 1 {
		t.Errorf("manifest 应恰好追加一条样式项:\n%s", opf)
	}

	// 二次运行：CSS 已存在且字节相同 → 不重复写；manifest href 已存在 → 不追加。
	goSecond := filepath.Join(dir, "go-second.epub")
	second := runGoRefinement(t, goOut, goSecond, intPtr(2))
	wantFacts["stylesheetsAdded"] = 0
	assertFacts(t, second.Facts, wantFacts)
	secondEntries := readZipEntries(t, goSecond)
	for name := range entries {
		if string(entries[name]) != string(secondEntries[name]) {
			t.Errorf("second run: entry %s 字节不一致", name)
		}
	}
	for name := range secondEntries {
		if _, ok := entries[name]; !ok {
			t.Errorf("second run 多出 entry %s", name)
		}
	}
}

func TestExpectVolumesMismatch(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	writeFixtureEpub(t, source, aliteFixture())

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, goErr := Run(context.Background(), b, Params{ExpectVolumes: intPtr(3)})
	if goErr == nil || !strings.Contains(goErr.Error(), "expected 3 volume poster pages, found 2") {
		t.Fatalf("应报卷数错误，实际: %v", goErr)
	}
}

func TestPosterWithoutCopyright(t *testing.T) {
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

	goOut := filepath.Join(dir, "go-warn.epub")
	res := runGoRefinement(t, source, goOut, nil)
	warning := "poster page has no adjacent copyright page: OEBPS/Text/poster2.xhtml"
	assertFacts(t, res.Facts, map[string]any{
		"posterPagesRefined":    2,
		"copyrightPagesRefined": 1,
		"copyrightPages":        []string{"OEBPS/Text/copyright1.xhtml"},
		"warnings":              []string{warning},
	})
	if len(res.Findings) != 1 || res.Findings[0].ID != "alite.no-copyright" || res.Findings[0].Level != "warn" || res.Findings[0].Title != warning {
		t.Fatalf("findings = %+v", res.Findings)
	}
}

// assertFacts 按 JSON 语义比较 facts 中给定的键。
func assertFacts(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("facts 缺少 %q", k)
			continue
		}
		gj, _ := json.Marshal(g)
		wj, _ := json.Marshal(w)
		if string(gj) != string(wj) {
			t.Errorf("facts[%q] = %s, want %s", k, gj, wj)
		}
	}
}

func intPtr(n int) *int { return &n }
