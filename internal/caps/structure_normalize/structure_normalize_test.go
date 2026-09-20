package structurenormalize

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
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
	res, err := Run(context.Background(), b, Params{Mode: mode, DryRun: dryRun})
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

// normFacts 是统一信封 facts 的测试视图（camelCase 键与 buildResult 一致）。
type normFacts struct {
	Operation                       string        `json:"operation"`
	Mode                            string        `json:"mode"`
	DryRun                          bool          `json:"dryRun"`
	OPF                             string        `json:"opf"`
	ManifestResources               int           `json:"manifestResources"`
	MovedResources                  int           `json:"movedResources"`
	RenamedResources                int           `json:"renamedResources"`
	RewrittenFiles                  int           `json:"rewrittenFiles"`
	FontObfuscationResources        int           `json:"fontObfuscationResources"`
	RemovedStaleEncryptionResources int           `json:"removedStaleEncryptionResources"`
	Mappings                        []mapping     `json:"mappings"`
	Warnings                        []string      `json:"warnings"`
	Stages                          []stageReport `json:"stages"`
}

// factsOf 经 JSON 往返读取 Result.Facts，同时保证 facts 可序列化。
func factsOf(t *testing.T, res report.Result) normFacts {
	t.Helper()
	raw, err := json.Marshal(res.Facts)
	if err != nil {
		t.Fatalf("facts 不可序列化: %v", err)
	}
	var out normFacts
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("facts 解码失败: %v\n%s", err, raw)
	}
	return out
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
	rep := factsOf(t, res)
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

func TestFormatDryRunProjectsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "source.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "should-not-exist.epub")

	res, err := runGo(t, fixture, output, ModeFormat, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	rep := factsOf(t, res)
	if !rep.DryRun || rep.RewrittenFiles == 0 || rep.MovedResources != 7 {
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
	rep := factsOf(t, res)
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
	wf := factsOf(t, res)
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

func TestNormalizeDryRunProjectsBothStages(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "workflow.epub")
	buildFixture(t, fixture, "")
	output := filepath.Join(dir, "out.epub")

	// Both stages are projected in memory; only the final disk write is skipped.
	res, err := runGo(t, fixture, output, ModeNormalize, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	wf := factsOf(t, res)
	if !wf.DryRun || !wf.Stages[0].DryRun || !wf.Stages[1].DryRun {
		t.Fatalf("dry_run 传播错误: %+v", wf)
	}
	if wf.Stages[0].RewrittenFiles == 0 || wf.Stages[1].RewrittenFiles == 0 {
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
	wf := factsOf(t, res)
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
	res, err := Run(context.Background(), b, Params{Mode: ModeInspect})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	rep := factsOf(t, res)
	if rep.Operation != "inspect" || rep.Mode != "inspect" || rep.DryRun || len(rep.Stages) != 0 {
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

// ---- 信封回归与幂等（原 Python oracle parity 的 Go 原生替代） ----

func TestFormatIdempotent(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.epub")
	buildFixture(t, fixture, "")
	round1 := filepath.Join(dir, "round1.epub")
	if _, err := runGo(t, fixture, round1, ModeFormat, false); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	round2 := filepath.Join(dir, "round2.epub")
	res, err := runGo(t, round1, round2, ModeFormat, false)
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	// 幂等：第二轮应零移动、零重写、零映射，产物逐 entry 与第一轮一致。
	rep := factsOf(t, res)
	if rep.MovedResources != 0 || rep.RewrittenFiles != 0 || len(rep.Mappings) != 0 {
		t.Fatalf("第二次 format 应为 no-op: %+v", rep)
	}
	first := openZip(t, round1)
	second := openZip(t, round2)
	if len(first.File) != len(second.File) {
		t.Fatalf("entry 数不一致: %d vs %d", len(first.File), len(second.File))
	}
	for _, f := range first.File {
		if got := zipRead(t, second, f.Name); !bytes.Equal(got, zipRead(t, first, f.Name)) {
			t.Fatalf("entry %s 第二轮字节变化", f.Name)
		}
	}
}

// TestFactsKeysAreStable 锁定单阶段与两阶段的 facts 键集合、mappings 形状
// （{from,to}，供 `epub redline --path-map` 直接消费）以及 warnings→findings 映射。
func TestFactsKeysAreStable(t *testing.T) {
	base := []string{
		"operation", "mode", "dryRun", "opf", "manifestResources", "movedResources", "renamedResources",
		"rewrittenFiles", "fontObfuscationResources", "removedStaleEncryptionResources", "mappings", "warnings",
	}
	cases := []struct {
		name    string
		mode    Mode
		variant string
		extra   []string
	}{
		{"format", ModeFormat, "", nil},
		{"deobfuscate", ModeDeobfuscate, "font", nil},
		{"normalize", ModeNormalize, "stale", []string{"stages"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := filepath.Join(dir, "fixture.epub")
			buildFixture(t, fixture, tc.variant)
			res, err := runGo(t, fixture, filepath.Join(dir, "out.epub"), tc.mode, false)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			want := append(append([]string{}, base...), tc.extra...)
			for _, k := range want {
				if _, ok := res.Facts[k]; !ok {
					t.Errorf("facts 缺少 %q", k)
				}
			}
			if len(res.Facts) != len(want) {
				t.Errorf("facts 键数 = %d, want %d: %v", len(res.Facts), len(want), res.Facts)
			}
			raw, err := json.Marshal(res.Facts)
			if err != nil {
				t.Fatal(err)
			}
			var generic map[string]any
			if err := json.Unmarshal(raw, &generic); err != nil {
				t.Fatal(err)
			}
			maps, ok := generic["mappings"].([]any)
			if !ok {
				t.Fatalf("mappings 必须是数组: %T", generic["mappings"])
			}
			if tc.mode != ModeFormat && len(maps) == 0 {
				t.Fatal("deobfuscate/normalize 应产生改名映射")
			}
			for _, m := range maps {
				item, _ := m.(map[string]any)
				if _, ok := item["from"].(string); !ok {
					t.Fatalf("mapping 缺 from: %v", m)
				}
				if _, ok := item["to"].(string); !ok {
					t.Fatalf("mapping 缺 to: %v", m)
				}
			}
			rep := factsOf(t, res)
			if rep.Operation == "" || rep.Mode != string(tc.mode) || rep.OPF == "" {
				t.Errorf("operation/mode/opf 错误: %+v", rep)
			}
			if len(rep.Warnings) != len(res.Findings) {
				t.Errorf("warnings(%d) 应逐条映射为 findings(%d)", len(rep.Warnings), len(res.Findings))
			}
			if tc.variant == "stale" {
				if rep.RemovedStaleEncryptionResources != 1 || len(rep.Warnings) == 0 {
					t.Errorf("顶层 stale 计数/告警应汇总各阶段: %+v", rep)
				}
				if len(rep.Stages) != 2 || len(rep.Mappings) != len(rep.Stages[0].Mappings)+len(rep.Stages[1].Mappings) {
					t.Errorf("顶层 mappings 应为各阶段拼接: %+v", rep)
				}
			}
		})
	}
}

// TestStageSlicesSerializeAsArrays 锁定 facts["stages"] 里每个阶段的
// mappings / warnings 始终序列化为数组而不是 null。stageReport 的唯一构造点
// 已把两者初始化为空切片；这个测试保证以后新增构造路径时不会退回 null，
// 因为 SKILL.md 记录的 jq 取长度用法在 null 上会失败。
func TestStageSlicesSerializeAsArrays(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.epub")
	buildFixture(t, fixture, "")
	res, err := runGo(t, fixture, filepath.Join(dir, "out.epub"), ModeNormalize, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw, err := json.Marshal(res.Facts)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	stages, ok := generic["stages"].([]any)
	if !ok || len(stages) == 0 {
		t.Fatalf("stages 必须是非空数组: %T", generic["stages"])
	}
	sawEmpty := false
	for i, s := range stages {
		stage, ok := s.(map[string]any)
		if !ok {
			t.Fatalf("stage %d 不是对象: %T", i, s)
		}
		for _, key := range []string{"mappings", "warnings"} {
			list, ok := stage[key].([]any)
			if !ok {
				t.Errorf("stage %d 的 %q 必须是数组，实际 %T (%v)", i, key, stage[key], stage[key])
				continue
			}
			if len(list) == 0 {
				sawEmpty = true
			}
		}
	}
	if !sawEmpty {
		t.Error("fixture 未产生空的阶段切片，本测试无法验证空数组形状")
	}
}
