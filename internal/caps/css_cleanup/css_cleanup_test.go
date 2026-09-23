package csscleanup

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

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
		"OEBPS/Text/chapter1.xhtml":  cssCleanupChapter("style0002.css", "", "component.css"),
		"OEBPS/Text/chapter2.xhtml":  cssCleanupChapter("style0004.css", "", "component.css"),
		"OEBPS/Text/chapter3.xhtml":  cssCleanupChapter("style0006.css", "", "component.css"),
		"OEBPS/Text/toc1.xhtml":      cssCleanupChapter("style0003.css", tocBody, ""),
		"OEBPS/Text/toc2.xhtml":      cssCleanupChapter("style0005.css", tocBody2, ""),
		"OEBPS/Styles/style0002.css": cssCleanupLegacyCSS("#876c4f"),
		"OEBPS/Styles/style0003.css": ".toc { margin-left: 0; }\n",
		"OEBPS/Styles/style0004.css": cssCleanupLegacyCSS("#876c4f"),
		"OEBPS/Styles/style0005.css": ".toc { margin-left: 0; }\n",
		"OEBPS/Styles/style0006.css": cssCleanupLegacyCSS("#3fbbd6"),
		"OEBPS/Styles/component.css": ".component { margin: 0 auto; }\n",
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
	slices.Sort(names)
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

func mustRun(t *testing.T, input, output string, mergeScoped bool) cleanupReport {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{Output: output, MergeScopedLocalCSS: mergeScoped})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := b.WriteToContext(t.Context(), output); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return reportFromFacts(t, res.Facts)
}

// reportFromFacts 从统一信封的 facts 还原 cleanupReport，缺键或类型不符即失败，
// 以此锁定每个正式 facts 键都被实际发出。
func reportFromFacts(t *testing.T, facts map[string]any) cleanupReport {
	t.Helper()
	intFact := func(key string) int {
		v, ok := facts[key].(int)
		if !ok {
			t.Fatalf("facts[%q] 缺失或不是 int: %#v", key, facts[key])
		}
		return v
	}
	boolFact := func(key string) bool {
		v, ok := facts[key].(bool)
		if !ok {
			t.Fatalf("facts[%q] 缺失或不是 bool: %#v", key, facts[key])
		}
		return v
	}
	strFact := func(key string) string {
		v, ok := facts[key].(string)
		if !ok {
			t.Fatalf("facts[%q] 缺失或不是 string: %#v", key, facts[key])
		}
		return v
	}
	warnings, ok := facts["warnings"].([]string)
	if !ok {
		t.Fatalf("facts[\"warnings\"] 缺失或不是 []string: %#v", facts["warnings"])
	}
	return cleanupReport{
		OPF:                          strFact("opf"),
		CSSFilesBefore:               intFact("cssFilesBefore"),
		CSSFilesAfter:                intFact("cssFilesAfter"),
		FactoredStylesheets:          intFact("factoredStylesheets"),
		DuplicateStylesheetsRemoved:  intFact("duplicateStylesheetsRemoved"),
		OverridesCreated:             intFact("overridesCreated"),
		FontDeclarationsRewritten:    intFact("fontDeclarationsRewritten"),
		XHTMLFilesUpdated:            intFact("xhtmlFilesUpdated"),
		CSSManifestItemsRemoved:      intFact("cssManifestItemsRemoved"),
		CSSManifestItemsAdded:        intFact("cssManifestItemsAdded"),
		ScopedLocalStylesheetsMerged: intFact("scopedLocalStylesheetsMerged"),
		ScopeClassesAdded:            intFact("scopeClassesAdded"),
		SemanticFactoringDisabled:    boolFact("semanticFactoringDisabled"),
		ScopedMergeDisabled:          boolFact("scopedMergeDisabled"),
		DuplicateDeduplication:       strFact("duplicateDeduplication"),
		Warnings:                     warnings,
	}
}

// ---- 单测（镜像 scripts/test_epub_css_cleanup.py 的断言） ----

func TestCSSCleanupFixture(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	output := filepath.Join(dir, "cleaned.epub")
	buildFixtureEpub(t, source, cssCleanupFixtureFiles())

	rep := mustRun(t, source, output, true)
	if rep.CSSFilesBefore != 6 || rep.FactoredStylesheets != 0 ||
		rep.DuplicateStylesheetsRemoved != 0 || rep.ScopedLocalStylesheetsMerged != 0 ||
		rep.ScopeClassesAdded != 0 {
		t.Fatalf("报告计数不符: %+v", rep)
	}
	if rep.OverridesCreated != 0 || rep.FontDeclarationsRewritten != 9 ||
		rep.XHTMLFilesUpdated != 0 || rep.CSSManifestItemsRemoved != 0 ||
		rep.CSSManifestItemsAdded != 0 || rep.CSSFilesAfter != 6 || len(rep.Warnings) != 1 ||
		!strings.Contains(rep.Warnings[0], "disabled for lossless safety") {
		t.Fatalf("报告计数不符: %+v", rep)
	}
	if !rep.SemanticFactoringDisabled || !rep.ScopedMergeDisabled || rep.DuplicateDeduplication != "disabled" {
		t.Fatalf("安全策略事实不符: %+v", rep)
	}

	files := readZipData(t, output)
	for _, name := range []string{
		"OEBPS/Styles/style0002.css",
		"OEBPS/Styles/style0003.css",
		"OEBPS/Styles/style0004.css",
		"OEBPS/Styles/style0005.css",
		"OEBPS/Styles/style0006.css",
		"OEBPS/Styles/component.css",
	} {
		if _, ok := files[name]; !ok {
			t.Fatalf("缺少 %s", name)
		}
	}
	for _, name := range []string{
		"OEBPS/Styles/clean-shared-01.css",
		"OEBPS/Styles/clean-scoped-local.css",
	} {
		if _, ok := files[name]; ok {
			t.Fatalf("不应存在 %s", name)
		}
	}

	style0002 := string(files["OEBPS/Styles/style0002.css"])
	for _, want := range []string{
		`"Songti SC", "SimSun", "Noto Serif CJK SC", serif`,
		`"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`,
		`"Kaiti SC", "STKaiti", "KaiTi", serif`,
	} {
		if !strings.Contains(style0002, want) {
			t.Fatalf("style0002 缺少 %q:\n%s", want, style0002)
		}
	}
	if strings.Contains(style0002, "————————————————") {
		t.Fatalf("style0002 不应包含装饰行:\n%s", style0002)
	}

	chapter1 := string(files["OEBPS/Text/chapter1.xhtml"])
	chapter2 := string(files["OEBPS/Text/chapter2.xhtml"])
	chapter3 := string(files["OEBPS/Text/chapter3.xhtml"])
	toc1 := string(files["OEBPS/Text/toc1.xhtml"])
	toc2 := string(files["OEBPS/Text/toc2.xhtml"])
	for _, check := range []struct{ text, want string }{
		{chapter1, `href="../Styles/style0002.css"`},
		{chapter1, `href="../Styles/component.css"`},
		{chapter2, `href="../Styles/style0004.css"`},
		{chapter3, `href="../Styles/style0006.css"`},
		{toc1, `href="../Styles/style0003.css"`},
		{toc2, `href="../Styles/style0005.css"`},
		{chapter1, "正文（补充说明）继续。"},
	} {
		if !strings.Contains(check.text, check.want) {
			t.Fatalf("缺少 %q:\n%s", check.want, check.text)
		}
	}
	if strings.Contains(chapter3, "css-local-") || strings.Contains(chapter3, `class="`) {
		t.Fatalf("scoped merge disabled but XHTML was rewritten: %s", chapter3)
	}

	// OPF manifest：css href 集合断言。
	opfHrefs := manifestCSSHrefs(t, files["OEBPS/content.opf"])
	for _, want := range []string{"Styles/style0002.css", "Styles/style0003.css", "Styles/style0004.css", "Styles/style0005.css", "Styles/style0006.css", "Styles/component.css"} {
		if !contains(opfHrefs, want) {
			t.Fatalf("manifest 缺少 %s: %v", want, opfHrefs)
		}
	}
	for _, gone := range []string{"Styles/clean-shared-01.css", "Styles/clean-scoped-local.css"} {
		if contains(opfHrefs, gone) {
			t.Fatalf("manifest 不应包含 %s: %v", gone, opfHrefs)
		}
	}

	// 幂等：对输出再跑一遍，报告仍只说明 requested scoped merge was skipped。
	secondOutput := filepath.Join(dir, "cleaned-again.epub")
	second := mustRun(t, output, secondOutput, true)
	if second.CSSFilesBefore != 6 || second.CSSFilesAfter != 6 ||
		second.FactoredStylesheets != 0 || second.ScopedLocalStylesheetsMerged != 0 ||
		len(second.Warnings) != 1 {
		t.Fatalf("第二次运行计数不符: %+v", second)
	}
	secondFiles := readZipData(t, secondOutput)
	if _, ok := secondFiles["OEBPS/Styles/clean-shared-01.css"]; ok {
		t.Fatal("第二次运行不应生成 shared stylesheet")
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

// TestMergeScopedLocalCSSOnlyWarns pins the sole observable effect of
// MergeScopedLocalCSS now that the scoped-local-merge implementation has
// been deleted as dead code: a warning when requested, and nothing else.
// The fixture gives every chapter its own private, non-shared stylesheet
// (style0002/0004/0006.css), i.e. exactly the shape the old scoped-merge
// algorithm would have picked as a merge candidate, so "nothing merged"
// here is provably the disabled-safety path and not an accident of the
// fixture having no candidates.
func TestMergeScopedLocalCSSOnlyWarns(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildFixtureEpub(t, source, cssCleanupFixtureFiles())

	cases := []struct {
		name  string
		merge bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "out.epub")
			b, err := book.Open(source)
			if err != nil {
				t.Fatalf("book.Open: %v", err)
			}
			defer b.Close()
			res, err := Run(t.Context(), b, Params{Output: output, MergeScopedLocalCSS: tc.merge})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if err := b.WriteToContext(t.Context(), output); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}

			if got, ok := res.Facts["mergeScopedLocalCss"].(bool); !ok || got != tc.merge {
				t.Fatalf("facts[mergeScopedLocalCss]=%#v, want echoed %v", res.Facts["mergeScopedLocalCss"], tc.merge)
			}
			warnings, ok := res.Facts["warnings"].([]string)
			if !ok {
				t.Fatalf("facts[warnings] missing or wrong type: %#v", res.Facts["warnings"])
			}
			hasDisabledWarning := slices.Contains(warnings, scopedMergeDisabledWarning)
			if tc.merge && (!hasDisabledWarning || len(warnings) != 1) {
				t.Fatalf("merge=true must produce exactly the disabled-merge warning: %v", warnings)
			}
			if !tc.merge && hasDisabledWarning {
				t.Fatalf("merge=false must not produce the disabled-merge warning: %v", warnings)
			}

			if got := res.Facts["scopedLocalStylesheetsMerged"]; got != 0 {
				t.Fatalf("scopedLocalStylesheetsMerged=%v, want 0", got)
			}
			if got := res.Facts["scopeClassesAdded"]; got != 0 {
				t.Fatalf("scopeClassesAdded=%v, want 0", got)
			}
			if got, ok := res.Facts["scopedMergeDisabled"].(bool); !ok || !got {
				t.Fatalf("scopedMergeDisabled=%#v, want true regardless of the input flag", res.Facts["scopedMergeDisabled"])
			}

			// No scoped-merge artifact must ever be produced, and every
			// private per-chapter stylesheet/link/body must be untouched.
			files := readZipData(t, output)
			if _, ok := files["OEBPS/Styles/clean-scoped-local.css"]; ok {
				t.Fatal("scoped-merge output file must never be generated")
			}
			chapter1 := string(files["OEBPS/Text/chapter1.xhtml"])
			if strings.Contains(chapter1, "css-local-") || strings.Contains(chapter1, `class="`) {
				t.Fatalf("body must not gain a scope class: %s", chapter1)
			}
			if !strings.Contains(chapter1, `href="../Styles/style0002.css"`) {
				t.Fatalf("private stylesheet link must be left untouched: %s", chapter1)
			}
		})
	}
}

func TestCleanupAcceptsAtRulesWithoutReserialization(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "at-rules.epub")
	output := filepath.Join(dir, "out.epub")
	files := cssCleanupFixtureFiles()
	want := "@font-face {\n" +
		"  font-family: \"BookFace\";\n" +
		"  src: url(\"../Fonts/book.woff2\");\n" +
		"}\n" +
		"@media screen and (min-width: 40em) {\n" +
		"  p[data-x=\"A  B\"] { content: \"a  b\"; }\n" +
		"}\n"
	files["OEBPS/Styles/style0002.css"] = want
	files["OEBPS/Styles/style0004.css"] = `p { content: "different"; }` + "\n"
	buildFixtureEpub(t, input, files)
	mustRun(t, input, output, false)
	got := readZipData(t, output)["OEBPS/Styles/style0002.css"]
	if string(got) != want {
		t.Fatalf("合法 at-rule 不得导致失败或重序列化:\n got %q\nwant %q", got, want)
	}
}

func TestCleanupDoesNotSemanticallyDeduplicateCSSStrings(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "strings.epub")
	output := filepath.Join(dir, "out.epub")
	files := cssCleanupFixtureFiles()
	files["OEBPS/Styles/style0002.css"] = `p[data-x="A  B"] { content: "a b"; }` + "\n"
	files["OEBPS/Styles/style0004.css"] = `p[data-x="a b"] { content: "ab"; }` + "\n"
	buildFixtureEpub(t, input, files)
	mustRun(t, input, output, false)
	got := readZipData(t, output)
	for _, name := range []string{"OEBPS/Styles/style0002.css", "OEBPS/Styles/style0004.css"} {
		if string(got[name]) != files[name] {
			t.Fatalf("%s was removed or normalized: got %q want %q", name, got[name], files[name])
		}
	}
}

func TestCleanupDoesNotDeduplicateAcrossCSSBaseDirectories(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "base-uri.epub")
	output := filepath.Join(dir, "out.epub")
	files := cssCleanupFixtureFiles()
	css := `.cover { background: url("../Images/cover.png"); }` + "\n"
	files["OEBPS/Styles/style0002.css"] = css
	delete(files, "OEBPS/Styles/style0004.css")
	files["OEBPS/Other/style0004.css"] = css
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"],
		`id="s4" href="Styles/style0004.css"`, `id="s4" href="Other/style0004.css"`, 1)
	files["OEBPS/Text/chapter2.xhtml"] = strings.Replace(files["OEBPS/Text/chapter2.xhtml"],
		`../Styles/style0004.css`, `../Other/style0004.css`, 1)
	buildFixtureEpub(t, input, files)
	mustRun(t, input, output, false)
	got := readZipData(t, output)
	for _, name := range []string{"OEBPS/Styles/style0002.css", "OEBPS/Other/style0004.css"} {
		if string(got[name]) != css {
			t.Fatalf("%s must survive because CSS base URI differs: %q", name, got[name])
		}
	}
}

func TestCleanupPreservesByteIdenticalCSSWithDistinctReferences(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "referenced-duplicates.epub")
	output := filepath.Join(dir, "out.epub")
	files := cssCleanupFixtureFiles()
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"],
		`<dc:title>CSS Cleanup Fixture</dc:title>`,
		`<dc:title>CSS Cleanup Fixture</dc:title><meta refines="#s4" property="role">duplicate-style</meta>`, 1)
	files["OEBPS/Styles/component.css"] = `@import "style0004.css";` + "\n.component { margin: 0 auto; }\n"
	buildFixtureEpub(t, input, files)
	rep := mustRun(t, input, output, false)
	if rep.DuplicateStylesheetsRemoved != 0 || rep.DuplicateDeduplication != "disabled" {
		t.Fatalf("referenced duplicate CSS must not be removed: %+v", rep)
	}
	got := readZipData(t, output)
	if _, ok := got["OEBPS/Styles/style0004.css"]; !ok {
		t.Fatal("byte-identical CSS with its own manifest id/import target was removed")
	}
	opfText := string(got["OEBPS/content.opf"])
	if !strings.Contains(opfText, `id="s4"`) || !strings.Contains(opfText, `refines="#s4"`) {
		t.Fatalf("manifest identity/refines changed: %s", opfText)
	}
	if !strings.Contains(string(got["OEBPS/Styles/component.css"]), `@import "style0004.css"`) {
		t.Fatal("CSS @import target changed")
	}
}

func TestCleanupPreservesEmbeddedFontFamilyReferencesAcrossStylesheets(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "font-face.epub")
	output := filepath.Join(dir, "out.epub")
	files := cssCleanupFixtureFiles()
	files["OEBPS/Styles/style0002.css"] = `body { font-family: "SimHei"; }
p { font-family: "STKaiti"; }
`
	files["OEBPS/Styles/style0004.css"] = `@font-face {
  font-family: "SimHei";
  src: url("../Fonts/book.woff2");
}
`
	buildFixtureEpub(t, input, files)
	mustRun(t, input, output, false)
	outputFiles := readZipData(t, output)
	consumer := string(outputFiles["OEBPS/Styles/style0002.css"])
	face := string(outputFiles["OEBPS/Styles/style0004.css"])
	if !strings.Contains(face, `@font-face {
  font-family: "SimHei";`) {
		t.Fatalf("@font-face family descriptor was rewritten: %s", face)
	}
	if !strings.Contains(consumer, `body { font-family: "SimHei"; }`) {
		t.Fatalf("consumer stopped referencing the embedded font face: %s", consumer)
	}
	if !strings.Contains(consumer, `p { font-family: `+kaiChain+`; }`) {
		t.Fatalf("unrelated system font family was not rewritten: %s", consumer)
	}
}

func TestCleanupIgnoresNonUTF8XHTML(t *testing.T) {
	files := cssCleanupFixtureFiles()
	text := `<?xml version="1.0" encoding="UTF-16"?><html xmlns="http://www.w3.org/1999/xhtml"><head><link rel="stylesheet" href="../Styles/style0002.css"/></head><body>正文</body></html>`
	units := utf16.Encode([]rune(text))
	data := []byte{0xff, 0xfe}
	for _, unit := range units {
		var pair [2]byte
		binary.LittleEndian.PutUint16(pair[:], unit)
		data = append(data, pair[:]...)
	}
	files["OEBPS/Text/chapter1.xhtml"] = string(data)
	input := filepath.Join(t.TempDir(), "utf16.xhtml.epub")
	output := filepath.Join(t.TempDir(), "out.epub")
	buildFixtureEpub(t, input, files)
	mustRun(t, input, output, false)
	got := readZipData(t, output)["OEBPS/Text/chapter1.xhtml"]
	if !bytes.Equal(got, data) {
		t.Fatalf("UTF-16 XHTML changed: got %d bytes, want %d", len(got), len(data))
	}
}

func TestCleanupResolvesPercentEncodedCSSHref(t *testing.T) {
	files := cssCleanupFixtureFiles()
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], `href="Styles/style0002.css"`, `href="Styles/style%200002.css"`, 1)
	files["OEBPS/Text/chapter1.xhtml"] = strings.Replace(files["OEBPS/Text/chapter1.xhtml"], `../Styles/style0002.css`, `../Styles/style%200002.css`, 1)
	files["OEBPS/Styles/style 0002.css"] = files["OEBPS/Styles/style0002.css"]
	delete(files, "OEBPS/Styles/style0002.css")
	input := filepath.Join(t.TempDir(), "encoded-css.epub")
	buildFixtureEpub(t, input, files)
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	result, err := Run(t.Context(), b, Params{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	warnings, ok := result.Facts["warnings"].([]string)
	if !ok {
		t.Fatalf("warnings fact type = %T", result.Facts["warnings"])
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "CSS manifest item does not resolve") {
			t.Fatalf("encoded CSS href was not resolved: %v", warnings)
		}
	}
	updated, err := b.Current("OEBPS/Styles/style 0002.css")
	if err != nil || !strings.Contains(string(updated), `font-family: "Heiti SC"`) {
		t.Fatalf("encoded CSS entry was not processed: %q (%v)", updated, err)
	}
}

func manifestCSSHrefs(t *testing.T, opfData []byte) []string {
	t.Helper()
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, item := range opf.ManifestNodes(root) {
		mt, _ := item.AttrByLocal("", "media-type")
		if mt != "text/css" {
			continue
		}
		href, _ := item.AttrByLocal("", "href")
		out = append(out, href)
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestSanitizeCSSUnits 覆盖安全的三个子变换与只读 shape 诊断。
func TestSanitizeCSSUnits(t *testing.T) {
	got, rewrites := sanitizeCSS("————————————————标题————————————————\nh1 {\n  font-family: \"SimHei\";\n}\n")
	if rewrites != 1 || strings.Contains(got, "——") || !strings.Contains(got, heiChain) {
		t.Fatalf("sanitizeCSS 不符: %q, %d", got, rewrites)
	}
	got, _ = sanitizeCSS("p {\n  margin: 0\n  padding: 0;\n}\n")
	if !strings.Contains(got, "margin: 0;") {
		t.Fatalf("补分号失败: %q", got)
	}
	if _, err := parseStylesheetSafe([]byte("@font-face { font-family: X; src: url(a.ttf); }")); !errors.Is(err, ErrUnsupportedCSSShape) {
		t.Fatalf("@font-face 应报告 unsupported shape，而非 syntax error: %v", err)
	}
	if _, err := parseStylesheetSafe([]byte("@media screen { .a { color: red; } }")); !errors.Is(err, ErrUnsupportedCSSShape) {
		t.Fatalf("@media 应报告 unsupported shape，而非 syntax error: %v", err)
	}
	if _, err := parseStylesheetSafe([]byte("/* 只剩注释 */")); !errors.Is(err, ErrUnsupportedCSSShape) {
		t.Fatalf("无 qualified rule 应报告 unsupported shape: %v", err)
	}
	if _, err := parseStylesheetSafe([]byte("h1 { color: red;")); err == nil || errors.Is(err, ErrUnsupportedCSSShape) {
		t.Fatalf("非法 CSS 应报告 syntax error: %v", err)
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
