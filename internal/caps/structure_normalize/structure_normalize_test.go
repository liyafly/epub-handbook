package structurenormalize

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// ---- fixture（逐字对齐 scripts/test_epub_structure_tool.py 的 write_fixture） ----

type fixtureEntry struct {
	name    string
	content string
}

func encryptionXML(uri, algorithm string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"
    xmlns:enc="http://www.w3.org/2001/04/xmlenc#">
  <enc:EncryptedData>
    <enc:EncryptionMethod Algorithm="` + algorithm + `"/>
    <enc:CipherData>
      <enc:CipherReference URI="` + uri + `"/>
    </enc:CipherData>
  </enc:EncryptedData>
</encryption>
`
}

func fixtureEntries(encrypted string) []fixtureEntry {
	entries := []fixtureEntry{
		{"META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`},
		{"OPS/package.opf", `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:structure-tool-test</dc:identifier>
    <dc:title>Structure Tool Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="toc" href="legacy/book.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="nav" href="legacy/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter-one.xhtml" href="legacy/%3Fmix.xhtml" media-type="application/xhtml+xml"/>
    <item id="appendix" href="legacy/appendix.xhtml" media-type="application/xhtml+xml"/>
    <item id="main-css" href="legacy/theme.css" media-type="text/css"/>
    <item id="cover-image" href="assets/%2Acover.JPG" media-type="image/jpeg"/>
    <item id="font-main" href="assets/font.ttf" media-type="font/ttf"/>
  </manifest>
  <spine toc="toc">
    <itemref idref="chapter-one.xhtml"/>
    <itemref idref="appendix"/>
  </spine>
  <guide>
    <reference type="text" title="Start" href="legacy/%3Fmix.xhtml#start"/>
  </guide>
</package>
`},
		{"OPS/legacy/book.ncx", `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="n1"><navLabel><text>第一章</text></navLabel><content src="%3Fmix.xhtml#start"/></navPoint>
  </navMap>
</ncx>
`},
		{"OPS/legacy/nav.xhtml", `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>目录</title></head>
  <body><nav><ol><li><a href="%3Fmix.xhtml#start">第一章</a></li></ol></nav></body>
</html>
`},
		{"OPS/legacy/?mix.xhtml", `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title>第一章</title>
    <link rel="stylesheet" href="theme.css"/>
  </head>
  <body style="background-image: url('../assets/%2Acover.JPG')">
    <h1 id="start">第一章</h1>
    <p>正文保留。<a href="appendix.xhtml#end">附录</a></p>
    <img src="../assets/%2Acover.JPG" srcset="data:image/svg+xml,%3Csvg%3E 1x, ../assets/%2Acover.JPG 2x, ../assets/%2Acover.JPG#hi 3x" alt="cover"/>
  </body>
</html>
`},
		{"OPS/legacy/appendix.xhtml", `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>附录</title></head><body><p id="end">附录正文。</p></body></html>
`},
		{"OPS/legacy/theme.css", "body { background-image: url('../assets/%2Acover.JPG'); }\n"},
		{"OPS/assets/*cover.JPG", "jpeg-bytes"},
		{"OPS/assets/font.ttf", "font-bytes"},
		{"OPS/extras/unlisted.bin", "unlisted-bytes"},
		{".DS_Store", "macos-metadata"},
	}
	switch encrypted {
	case "font":
		entries = append(entries, fixtureEntry{"META-INF/encryption.xml", encryptionXML("OPS/assets/font.ttf", "http://www.idpf.org/2008/embedding")})
	case "text":
		entries = append(entries, fixtureEntry{"META-INF/encryption.xml", encryptionXML("OPS/legacy/%3Fmix.xhtml", "http://www.w3.org/2001/04/xmlenc#aes128-cbc")})
	case "stale":
		entries = append(entries, fixtureEntry{"META-INF/encryption.xml", encryptionXML("OPS/Styles/dkagent.css", "http://www.w3.org/2001/04/xmlenc#aes128-ctr")})
	}
	// Python 测试最后写入内容错误、DEFLATED 的 mimetype。
	entries = append(entries, fixtureEntry{"mimetype", "wrong-on-purpose"})
	return entries
}

func buildFixture(t *testing.T, path, encrypted string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range fixtureEntries(encrypted) {
		h := &zip.FileHeader{Name: e.name}
		h.Method = zip.Deflate
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
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

// ---- 测试工具 ----

// runGo 打开 fixture、跑 Run 并落盘（模拟 pipeline 的调用方式）。
func runGo(t *testing.T, fixture, output string, mode Mode, dryRun bool) (report.Result, error) {
	t.Helper()
	b, err := book.Open(fixture)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Mode: mode, DryRun: dryRun, Output: output, LegacyReport: true})
	if err != nil {
		return res, err
	}
	if !dryRun {
		if err := b.WriteTo(output); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
	}
	return res, nil
}

func openZip(t *testing.T, path string) *zip.ReadCloser {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("打开 %s: %v", path, err)
	}
	t.Cleanup(func() { zr.Close() })
	return zr
}

func zipNames(zr *zip.ReadCloser) map[string]bool {
	out := map[string]bool{}
	for _, f := range zr.File {
		out[f.Name] = true
	}
	return out
}

func zipRead(t *testing.T, zr *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("打开 entry %s: %v", name, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("读取 entry %s: %v", name, err)
			}
			return data
		}
	}
	t.Fatalf("entry %s 不存在", name)
	return nil
}

func assertMimetypeStored(t *testing.T, zr *zip.ReadCloser) {
	t.Helper()
	if len(zr.File) == 0 {
		t.Fatal("输出为空")
	}
	first := zr.File[0]
	if first.Name != "mimetype" {
		t.Fatalf("第一个 entry 应为 mimetype，实际 %s", first.Name)
	}
	if first.Method != zip.Store {
		t.Fatalf("mimetype 应为 STORED，实际 method=%d", first.Method)
	}
	if got := zipRead(t, zr, "mimetype"); string(got) != "application/epub+zip" {
		t.Fatalf("mimetype 内容错误: %q", got)
	}
}

func legacyReportOf(t *testing.T, res report.Result) []byte {
	t.Helper()
	raw, ok := res.Facts["legacyReport"]
	if !ok {
		t.Fatal("Result.Facts 缺少 legacyReport")
	}
	switch v := raw.(type) {
	case json.RawMessage:
		return v
	case []byte:
		return v
	}
	t.Fatalf("legacyReport 类型错误: %T", raw)
	return nil
}

// ---- 语义测试（对齐 test_epub_structure_tool.py 的断言） ----

func TestFormatMatchesPythonAssertions(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "source.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "formatted.epub")

	res, err := runGo(t, fixture, output, ModeFormat, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rep legacyRewriteReport
	if err := json.Unmarshal(legacyReportOf(t, res), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.MovedResources != 7 || rep.RenamedResources != 0 {
		t.Fatalf("moved=%d renamed=%d, want 7/0；mappings=%v", rep.MovedResources, rep.RenamedResources, rep.Mappings)
	}
	if len(rep.Warnings) != 0 {
		t.Fatalf("不应有告警: %v", rep.Warnings)
	}

	zr := openZip(t, output)
	assertMimetypeStored(t, zr)
	names := zipNames(zr)
	for _, want := range []string{
		"OPS/Text/?mix.xhtml", "OPS/Text/nav.xhtml", "OPS/Styles/theme.css",
		"OPS/Images/*cover.JPG", "OPS/Fonts/font.ttf", "OPS/book.ncx",
	} {
		if !names[want] {
			t.Errorf("缺少 entry %s；实际 %v", want, names)
		}
	}
	if names[".DS_Store"] {
		t.Error(".DS_Store 应被剔除")
	}
	if got := zipRead(t, zr, "OPS/extras/unlisted.bin"); string(got) != "unlisted-bytes" {
		t.Errorf("unlisted.bin 内容变化: %q", got)
	}

	opf := string(zipRead(t, zr, "OPS/package.opf"))
	for _, want := range []string{
		`id="chapter-one.xhtml" href="Text/%3Fmix.xhtml"`,
		`id="cover-image" href="Images/%2Acover.JPG"`,
		`href="Text/%3Fmix.xhtml#start"`,
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("OPF 缺少 %q", want)
		}
	}

	chapter := string(zipRead(t, zr, "OPS/Text/?mix.xhtml"))
	for _, want := range []string{
		`href="../Styles/theme.css"`,
		`href="appendix.xhtml#end"`,
		`src="../Images/%2Acover.JPG"`,
		`srcset="data:image/svg+xml,%3Csvg%3E 1x, ../Images/%2Acover.JPG 2x, ../Images/%2Acover.JPG#hi 3x"`,
		"正文保留。",
	} {
		if !strings.Contains(chapter, want) {
			t.Errorf("chapter 缺少 %q", want)
		}
	}
	if css := string(zipRead(t, zr, "OPS/Styles/theme.css")); !strings.Contains(css, "../Images/%2Acover.JPG") {
		t.Errorf("theme.css 未重写: %q", css)
	}
	if ncx := string(zipRead(t, zr, "OPS/book.ncx")); !strings.Contains(ncx, `src="Text/%3Fmix.xhtml#start"`) {
		t.Errorf("book.ncx 未重写: %q", ncx)
	}
}

func TestFormatDryRunOnlyPlans(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "source.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "should-not-exist.epub")

	res, err := runGo(t, fixture, output, ModeFormat, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rep legacyRewriteReport
	if err := json.Unmarshal(legacyReportOf(t, res), &rep); err != nil {
		t.Fatal(err)
	}
	if !rep.DryRun || rep.RewrittenFiles != 0 || rep.MovedResources != 7 {
		t.Fatalf("dry-run 报告错误: %+v", rep)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dry-run 不应写出文件")
	}
}

func TestDeobfuscateMatchesPythonAssertions(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "font-obfuscated.epub")
	buildFixture(t, fixture, "font")
	output := filepath.Join(dir, "deobfuscated.epub")

	res, err := runGo(t, fixture, output, ModeDeobfuscate, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rep legacyRewriteReport
	if err := json.Unmarshal(legacyReportOf(t, res), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.FontObfuscationResources != 1 || rep.RenamedResources != 5 || rep.MovedResources != 7 {
		t.Fatalf("报告错误: %+v", rep)
	}

	zr := openZip(t, output)
	assertMimetypeStored(t, zr)
	names := zipNames(zr)
	for _, want := range []string{
		"OPS/Text/chapter-one.xhtml", "OPS/Text/appendix.xhtml", "OPS/Styles/main-css.css",
		"OPS/Images/cover-image.jpg", "OPS/Fonts/font-main.ttf", "OPS/toc.ncx",
	} {
		if !names[want] {
			t.Errorf("缺少 entry %s；实际 %v", want, names)
		}
	}

	opf := string(zipRead(t, zr, "OPS/package.opf"))
	for _, want := range []string{
		`id="chapter-one.xhtml" href="Text/chapter-one.xhtml"`,
		`id="cover-image" href="Images/cover-image.jpg"`,
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("OPF 缺少 %q", want)
		}
	}
	chapter := string(zipRead(t, zr, "OPS/Text/chapter-one.xhtml"))
	for _, want := range []string{
		`href="../Styles/main-css.css"`,
		`src="../Images/cover-image.jpg"`,
		`srcset="data:image/svg+xml,%3Csvg%3E 1x, ../Images/cover-image.jpg 2x, ../Images/cover-image.jpg#hi 3x"`,
		"正文保留。",
	} {
		if !strings.Contains(chapter, want) {
			t.Errorf("chapter 缺少 %q", want)
		}
	}
	enc := string(zipRead(t, zr, "META-INF/encryption.xml"))
	if !strings.Contains(enc, `URI="OPS/Fonts/font-main.ttf"`) {
		t.Errorf("encryption.xml 未同步改写: %q", enc)
	}
}

func TestRefuseDRM(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "drm.epub")
	buildFixture(t, fixture, "text")
	output := filepath.Join(dir, "should-not-exist.epub")

	_, err := runGo(t, fixture, output, ModeDeobfuscate, false)
	if err == nil {
		t.Fatal("加密 XHTML 应被拒绝")
	}
	if !errors.Is(err, ErrStructureTool) {
		t.Fatalf("应返回 ErrStructureTool: %v", err)
	}
	if !strings.Contains(err.Error(), "DRM or unsupported encrypted resources detected") {
		t.Fatalf("错误措辞不符: %v", err)
	}
	if _, serr := os.Stat(output); !errors.Is(serr, os.ErrNotExist) {
		t.Fatal("拒绝时不应写出文件")
	}
}

func TestNormalizeTwoStageWorkflow(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "workflow.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "workflow-normalized.epub")

	res, err := runGo(t, fixture, output, ModeNormalize, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var wf legacyWorkflowReport
	if err := json.Unmarshal(legacyReportOf(t, res), &wf); err != nil {
		t.Fatal(err)
	}
	if wf.Operation != "normalize" || len(wf.Stages) != 2 {
		t.Fatalf("workflow 报告形状错误: %+v", wf)
	}
	if wf.Stages[0].Operation != "format" || wf.Stages[1].Operation != "deobfuscate-filenames" {
		t.Fatalf("阶段顺序错误: %s, %s", wf.Stages[0].Operation, wf.Stages[1].Operation)
	}
	// 两阶段改名链式展开（语义同 add_path_mapping：先改既有映射中
	// value==source 的键再登记；中间名作为键保留，与 Python 一致）。
	wantRenames := map[string]string{
		// 阶段 1 登记的原始名（阶段 2 把 value 链到最终名）。
		"OPS/legacy/book.ncx":       "OPS/toc.ncx",
		"OPS/legacy/nav.xhtml":      "OPS/Text/nav.xhtml",
		"OPS/legacy/?mix.xhtml":     "OPS/Text/chapter-one.xhtml",
		"OPS/legacy/appendix.xhtml": "OPS/Text/appendix.xhtml",
		"OPS/legacy/theme.css":      "OPS/Styles/main-css.css",
		"OPS/assets/*cover.JPG":     "OPS/Images/cover-image.jpg",
		"OPS/assets/font.ttf":       "OPS/Fonts/font-main.ttf",
		// 阶段 1 的中间名作为阶段 2 的 source 保留在映射里。
		"OPS/Images/*cover.JPG": "OPS/Images/cover-image.jpg",
		"OPS/Fonts/font.ttf":    "OPS/Fonts/font-main.ttf",
		"OPS/Styles/theme.css":  "OPS/Styles/main-css.css",
		"OPS/Text/?mix.xhtml":   "OPS/Text/chapter-one.xhtml",
		"OPS/book.ncx":          "OPS/toc.ncx",
	}
	if !reflect.DeepEqual(res.Renames, wantRenames) {
		t.Fatalf("Renames 链式展开错误: %v", res.Renames)
	}

	zr := openZip(t, output)
	assertMimetypeStored(t, zr)
	names := zipNames(zr)
	if !names["OPS/Text/chapter-one.xhtml"] || !names["OPS/Styles/main-css.css"] {
		t.Fatalf("normalize 产物缺 entry: %v", names)
	}
	chapter := string(zipRead(t, zr, "OPS/Text/chapter-one.xhtml"))
	if !strings.Contains(chapter, "正文保留。") || !strings.Contains(chapter, `src="../Images/cover-image.jpg"`) {
		t.Errorf("normalize 后 chapter 内容错误: %q", chapter)
	}
}

func TestNormalizeDryRunKeepsStage1(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "workflow.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "out.epub")

	// 与 Python 一致：normalize 的 dry-run 只作用于阶段 2，
	// 阶段 1 仍完整执行（报告 dry_run=false / rewritten=4）。
	res, err := runGo(t, fixture, output, ModeNormalize, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var wf legacyWorkflowReport
	if err := json.Unmarshal(legacyReportOf(t, res), &wf); err != nil {
		t.Fatal(err)
	}
	if wf.DryRun != true || wf.Stages[0].DryRun != false || wf.Stages[1].DryRun != true {
		t.Fatalf("dry_run 传播错误: %+v", wf)
	}
	if wf.Stages[0].RewrittenFiles == 0 || wf.Stages[1].RewrittenFiles != 0 {
		t.Fatalf("阶段 rewritten 计数错误: %+v", wf.Stages)
	}
	if _, serr := os.Stat(output); !errors.Is(serr, os.ErrNotExist) {
		t.Fatal("dry-run 不应写出最终产物")
	}
}

func TestRemoveStaleEncryptionReference(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "stale-encryption.epub")
	buildFixture(t, fixture, "stale")
	output := filepath.Join(dir, "stale-encryption-normalized.epub")

	res, err := runGo(t, fixture, output, ModeNormalize, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var wf legacyWorkflowReport
	if err := json.Unmarshal(legacyReportOf(t, res), &wf); err != nil {
		t.Fatal(err)
	}
	if wf.Stages[0].RemovedStaleEncryptionResources != 1 {
		t.Fatalf("stale 计数错误: %+v", wf.Stages[0])
	}
	found := false
	for _, w := range wf.Stages[0].Warnings {
		if strings.Contains(w, "remove stale encryption reference") {
			found = true
		}
	}
	if !found {
		t.Fatalf("缺少 stale 告警: %v", wf.Stages[0].Warnings)
	}
	zr := openZip(t, output)
	if zipNames(zr)["META-INF/encryption.xml"] {
		t.Fatal("stale encryption.xml 应被整体移除")
	}
}

func TestInspectReportsWithoutEdits(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "source.epub")
	buildFixture(t, fixture, "font")

	b, err := book.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Mode: ModeInspect, LegacyReport: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rep legacyRewriteReport
	if err := json.Unmarshal(legacyReportOf(t, res), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Operation != "inspect" || rep.Output != nil || rep.DryRun {
		t.Fatalf("inspect 报告形状错误: %+v", rep)
	}
	if rep.ManifestResources != 7 || rep.FontObfuscationResources != 1 {
		t.Fatalf("inspect 计数错误: %+v", rep)
	}
	if len(res.Renames) != 0 || len(rep.Mappings) != 0 {
		t.Fatal("inspect 不应有改名")
	}
	if len(b.ModifiedNames()) != 0 {
		t.Fatalf("inspect 不应修改 book: %v", b.ModifiedNames())
	}
}

func TestDeobfuscatedBasenameRules(t *testing.T) {
	cases := []struct {
		itemID, archivePath, want string
	}{
		{"chapter-one.xhtml", "OPS/legacy/?mix.xhtml", "chapter-one.xhtml"},
		{"appendix", "OPS/legacy/appendix.xhtml", "appendix.xhtml"},
		{"main-css", "OPS/legacy/theme.css", "main-css.css"},
		{"cover-image", "OPS/assets/*cover.JPG", "cover-image.jpg"},
		{"font-main", "OPS/assets/font.ttf", "font-main.ttf"},
		{"toc", "OPS/legacy/book.ncx", "toc.ncx"},
		// slim 规则：id 上的 ~slim / -slim / _slim 后缀保留为 ~slim。
		{"main-css~slim", "OPS/legacy/theme.css", "main-css~slim.css"},
		{"main-css-slim", "OPS/legacy/theme.css", "main-css~slim.css"},
		// 源文件名带 slim：id 无 slim 也要补 ~slim。
		{"text", "OPS/legacy/chapter~slim.xhtml", "text~slim.xhtml"},
		// 非法字符清洗 + 空名回退到 sha256（Python：连续非法段替换为一个 "-"）。
		{"weird*name", "OPS/x.html", "weird-name.html"},
		{"***", "OPS/x.html", "-.html"},
		{"...", "OPS/x.html", "resource-" + sha256Hex12("...") + ".html"},
	}
	for _, tc := range cases {
		got := deobfuscatedBasename(manifestResource{itemID: tc.itemID, archivePath: tc.archivePath})
		if got != tc.want {
			t.Errorf("deobfuscatedBasename(%q, %q) = %q, want %q", tc.itemID, tc.archivePath, got, tc.want)
		}
	}
}

func TestCSSReferenceScannerEdgeCases(t *testing.T) {
	warnings := []string{}
	rw := &refRewriter{
		pathMap:  map[string]string{"old/a.png": "new/a.png"},
		files:    map[string]bool{"old/a.png": true},
		warnings: &warnings,
	}
	// 引用相对 old/doc.css：a.png → old/a.png；改写后相对 new/doc.css → a.png。
	text := "a{background:url('a.png')} b{background:url( a.png )} @import \"a.png\"; c{background:url(\"a.png\")}"
	got := rewriteCSSReferences(text, "old/doc.css", "new/doc.css", rw)
	want := "a{background:url('a.png')} b{background:url( a.png )} @import \"a.png\"; c{background:url(\"a.png\")}"
	if got != want {
		t.Fatalf("CSS 重写错误:\n got %q\nwant %q", got, want)
	}
	if len(warnings) != 0 {
		t.Fatalf("成功重写不应有告警: %v", warnings)
	}
}

func TestXHTMLReferenceScanner(t *testing.T) {
	warnings := []string{}
	rw := &refRewriter{
		pathMap: map[string]string{
			"old/a.png":     "new/img/a.png",
			"old/doc.xhtml": "new/doc.xhtml",
		},
		files:    map[string]bool{"old/a.png": true, "old/doc.xhtml": true},
		warnings: &warnings,
	}
	text := `<img src="a.png" srcset="a.png 1x, a.png#hi 2x" data-x="a.png" poster='a.png'><a href="doc.xhtml#p">x</a>`
	got := rewriteMarkupReferences(text, "old/doc.xhtml", "new/doc.xhtml", rw)
	// a.png → old/a.png → new/img/a.png，相对 new/doc.xhtml → img/a.png。
	// data-x 的名字 "data" 后是 "-"，不匹配属性正则（与 Python 一致）。
	want := `<img src="img/a.png" srcset="img/a.png 1x, img/a.png#hi 2x" data-x="a.png" poster='img/a.png'><a href="doc.xhtml#p">x</a>`
	if got != want {
		t.Fatalf("markup 重写错误:\n got %q\nwant %q", got, want)
	}
}

func TestETSerializerPinnedRules(t *testing.T) {
	// 复刻 CPython 探针的结果：声明形状、未知 ns 的 ns0/ns1 编号、
	// 属性原位更新、空元素 " />"、文本/属性转义。
	src := `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:demo" xmlns:p="urn:other" k="a &lt; b">
  <p:child />
  <empty></empty>
  <t>x &amp; y</t>
</root>
`
	root, err := parseXMLTree([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(etreeToBytes(root))
	want := `<?xml version='1.0' encoding='utf-8'?>` + "\n" +
		`<ns0:root xmlns:ns0="urn:demo" xmlns:ns1="urn:other" k="a &lt; b">` + "\n" +
		`  <ns1:child />` + "\n" +
		`  <ns0:empty />` + "\n" +
		`  <ns0:t>x &amp; y</ns0:t>` + "\n" +
		`</ns0:root>`
	if got != want {
		t.Fatalf("ET 序列化不一致:\n got %q\nwant %q", got, want)
	}
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

// pythonScriptPath 解析 oracle 脚本绝对路径（不可用时跳过测试）。
func pythonScriptPath(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repoRoot, "scripts", "epub_structure_tool.py")
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/epub_structure_tool.py 不存在（oracle 已删除）")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用")
	}
	return script
}

// runPythonStructureTool 在 dir 下跑 scripts/epub_structure_tool.py 并返回 JSON 报告。
func runPythonStructureTool(t *testing.T, dir string, args ...string) map[string]any {
	t.Helper()
	return runPythonScript(t, dir, pythonScriptPath(t), args...)
}

// runPythonScript 以显式脚本路径跑 oracle（chdir 之后的调用用它）。
func runPythonScript(t *testing.T, dir, script string, args ...string) map[string]any {
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

// zeroUncertainFields 把两边报告中的不确定字段置空（normalize stage[1].input
// 是 Python 临时目录路径，每次运行都变化；parity 比对时忽略）。
func zeroUncertainFields(v any) {
	obj, ok := v.(map[string]any)
	if !ok {
		return
	}
	stages, ok := obj["stages"].([]any)
	if !ok || len(stages) < 2 {
		return
	}
	if st, ok := stages[1].(map[string]any); ok {
		st["input"] = ""
	}
}

func readZipFiles(t *testing.T, path string) map[string]*zip.File {
	t.Helper()
	zr := openZip(t, path)
	out := map[string]*zip.File{}
	for _, f := range zr.File {
		out[f.Name] = f
	}
	return out
}

func parityCase(t *testing.T, mode Mode, encrypted string, dryRun bool) {
	t.Helper()
	dir := t.TempDir()
	buildFixture(t, filepath.Join(dir, "fixture.epub"), encrypted)
	op, _ := mode.pythonOperation()

	// 1. Python oracle（相对路径，报告里的 input/output 与 Go 对齐）。
	pyArgs := []string{op, "fixture.epub", "--output", "out.epub", "--report-format", "json"}
	if dryRun {
		pyArgs = append(pyArgs, "--dry-run")
	}
	pyReport := runPythonStructureTool(t, dir, pyArgs...)
	if !dryRun {
		if err := os.Rename(filepath.Join(dir, "out.epub"), filepath.Join(dir, "py.epub")); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Go 实现（chdir 使 InputPath 为相对路径形状）。
	restore := chdir(t, dir)
	defer restore()
	b, err := book.Open("fixture.epub")
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	res, err := Run(context.Background(), b, Params{Mode: mode, DryRun: dryRun, Output: "out.epub", LegacyReport: true})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	if !dryRun {
		if err := b.WriteTo("out.epub"); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
	}
	b.Close()

	// 3. 逐 entry 比对 CRC32 / 内容 / mimetype 方法。
	if !dryRun {
		pyFiles := readZipFiles(t, filepath.Join(dir, "py.epub"))
		goFiles := readZipFiles(t, filepath.Join(dir, "out.epub"))
		if len(pyFiles) != len(goFiles) {
			t.Fatalf("entry 数不一致: python=%d go=%d", len(pyFiles), len(goFiles))
		}
		for name, pf := range pyFiles {
			gf, ok := goFiles[name]
			if !ok {
				t.Fatalf("Go 产物缺少 entry %s", name)
			}
			if pf.CRC32 != gf.CRC32 {
				pyData := zipRead(t, openZip(t, filepath.Join(dir, "py.epub")), name)
				goData := zipRead(t, openZip(t, filepath.Join(dir, "out.epub")), name)
				t.Fatalf("entry %s CRC32 不一致 (python=%d go=%d)\npython=%q\ngo=%q",
					name, pf.CRC32, gf.CRC32, pyData, goData)
			}
		}
		// mimetype：内容规范 + STORED。
		for name, files := range map[string]map[string]*zip.File{"py": pyFiles, "go": goFiles} {
			mf, ok := files["mimetype"]
			if !ok {
				t.Fatalf("%s 产物缺少 mimetype", name)
			}
			if mf.Method != zip.Store {
				t.Fatalf("%s 产物 mimetype 应为 STORED", name)
			}
		}
	}

	// 4. legacy 报告逐字段比对（P2）。
	var goReport map[string]any
	if err := json.Unmarshal(legacyReportOf(t, res), &goReport); err != nil {
		t.Fatal(err)
	}
	zeroUncertainFields(goReport)
	zeroUncertainFields(pyReport)
	if !reflect.DeepEqual(goReport, pyReport) {
		goJSON, _ := json.MarshalIndent(goReport, "", "  ")
		pyJSON, _ := json.MarshalIndent(pyReport, "", "  ")
		t.Fatalf("legacy 报告不一致:\n--- go ---\n%s\n--- python ---\n%s", goJSON, pyJSON)
	}
}

func TestParityFormat(t *testing.T)          { parityCase(t, ModeFormat, "", false) }
func TestParityFormatDryRun(t *testing.T)    { parityCase(t, ModeFormat, "", true) }
func TestParityDeobfuscate(t *testing.T)     { parityCase(t, ModeDeobfuscate, "font", false) }
func TestParityNormalize(t *testing.T)       { parityCase(t, ModeNormalize, "", false) }
func TestParityNormalizeDryRun(t *testing.T) { parityCase(t, ModeNormalize, "", true) }

// TestParityFormatIdempotent 覆盖「输入已是 ET 规范形」的路径：对 Python
// format 过一次的产物再跑一次 format，Go 与 Python 的输出必须仍逐 entry
// 一致（真实书籍会被本工具反复处理，这正是字节保真最要紧的场景）。
func TestParityFormatIdempotent(t *testing.T) {
	script := pythonScriptPath(t)
	dir := t.TempDir()
	buildFixture(t, filepath.Join(dir, "fixture.epub"), "")
	// 第一轮：双方各自 format。
	runPythonScript(t, dir, script, "format", "fixture.epub", "--output", "round1.epub", "--report-format", "json")
	if err := os.Rename(filepath.Join(dir, "round1.epub"), filepath.Join(dir, "py1.epub")); err != nil {
		t.Fatal(err)
	}
	restore := chdir(t, dir)
	defer restore()
	b, err := book.Open("fixture.epub")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), b, Params{Mode: ModeFormat, Output: "round1.epub"}); err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	if err := b.WriteTo("round1.epub"); err != nil {
		t.Fatal(err)
	}
	b.Close()

	// 第二轮：Python 处理自己的 round1 产物，Go 处理自己的 round1 产物。
	runPythonScript(t, dir, script, "format", "round1.epub", "--output", "py2.epub", "--report-format", "json")

	b2, err := book.Open("round1.epub")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), b2, Params{Mode: ModeFormat, Output: "out.epub", LegacyReport: true})
	if err != nil {
		t.Fatalf("Go Run 2: %v", err)
	}
	if err := b2.WriteTo("out.epub"); err != nil {
		t.Fatal(err)
	}
	b2.Close()

	pyFiles := readZipFiles(t, filepath.Join(dir, "py2.epub"))
	goFiles := readZipFiles(t, filepath.Join(dir, "out.epub"))
	if len(pyFiles) != len(goFiles) {
		t.Fatalf("entry 数不一致: python=%d go=%d", len(pyFiles), len(goFiles))
	}
	for name, pf := range pyFiles {
		gf, ok := goFiles[name]
		if !ok {
			t.Fatalf("Go 产物缺少 entry %s", name)
		}
		if pf.CRC32 != gf.CRC32 {
			t.Fatalf("entry %s CRC32 不一致 (python=%d go=%d)", name, pf.CRC32, gf.CRC32)
		}
	}
	// 幂等：第二轮应零移动、零重写。
	var rep legacyRewriteReport
	if err := json.Unmarshal(legacyReportOf(t, res), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.MovedResources != 0 || rep.RewrittenFiles != 0 {
		t.Fatalf("第二次 format 应为 no-op: %+v", rep)
	}
}
