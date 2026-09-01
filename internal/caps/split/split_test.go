package split

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// ---- fixture：逐字节复刻 scripts/test_epub_package_tool.py 的 write_book ----

type zipEntry struct {
	name    string
	content []byte
}

func buildEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name}
		h.Method = zip.Deflate
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(e.content); err != nil {
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

func writeBookEntries(title, marker string, coverBytes []byte) []zipEntry {
	f := func(s string) []byte { return []byte(s) }
	return []zipEntry{
		{name: "META-INF/container.xml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`)},
		{name: "OEBPS/content.opf", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:` + marker + `</dc:identifier>
    <dc:title id="main-title">` + title + `</dc:title>
    <dc:creator>Author ` + marker + `</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:publisher>Publisher ` + marker + `</dc:publisher>
    <dc:description>Description ` + marker + `</dc:description>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="style" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="nav" linear="no"/>
    <itemref idref="chap"/>
  </spine>
</package>
`)},
		{name: "OEBPS/nav.xhtml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>` + title + `</title></head>
  <body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml#start">` + title + `</a></li></ol></nav></body>
</html>
`)},
		{name: "OEBPS/Text/chapter.xhtml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>` + title + `</title><link rel="stylesheet" href="../Styles/main.css"/></head>
  <body>
    <h1 id="start">` + title + `</h1>
    <p>` + marker + ` 正文保留。<img src="../Images/cover.jpg" alt="cover"/></p>
  </body>
</html>
`)},
		{name: "OEBPS/Styles/main.css", content: f("body { background: url('../Images/cover.jpg'); }\n")},
		{name: "OEBPS/Images/cover.jpg", content: coverBytes},
		{name: ".DS_Store", content: f("macos-metadata")},
		{name: "mimetype", content: f("wrong-on-purpose")},
	}
}

// ---- Python oracle 与比较工具 ----

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
	if code != 0 {
		t.Fatalf("python oracle 退出码 %d: %s", code, errb.String())
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

// pyCanonicalXML 用 Python ET 把 EPUB 内某 entry 规范化为 JSON。
// split 的 OPF 在 Python 侧是整树重序列化、Go 侧是字节区间编辑
// （metadata 与根属性保留源字节，与 Python 输出语义一致但字节不同，
// 例如空白 tail、guide 之外的额外元素在 Go 侧保留）——因此 OPF 只比
// 语义；nav/ncx/container 与资源两侧都是逐字节同源产物。
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

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func replaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func stringsHasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func compareEntries(t *testing.T, pyPath, goPath string, semantic map[string]bool) {
	t.Helper()
	py := readZipEntries(t, pyPath)
	go_ := readZipEntries(t, goPath)
	for name := range py {
		if _, ok := go_[name]; !ok {
			t.Errorf("Go 输出缺少 entry %s", name)
		}
	}
	for name := range go_ {
		if _, ok := py[name]; !ok {
			t.Errorf("Go 输出多出 entry %s", name)
		}
	}
	names := make([]string, 0, len(py))
	for name := range py {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		if semantic[name] {
			pyC, goC := pyCanonicalXML(t, pyPath, name), pyCanonicalXML(t, goPath, name)
			if pyC != goC {
				t.Errorf("entry %s 语义不一致：\n--- py ---\n%s\n--- go ---\n%s", name, clip(pyC, 1600), clip(goC, 1600))
			}
			continue
		}
		if !bytes.Equal(py[name], go_[name]) {
			t.Errorf("entry %s 字节不一致：\n--- py ---\n%s\n--- go ---\n%s",
				name, clip(string(py[name]), 1200), clip(string(go_[name]), 1200))
		}
	}
}

func TestParitySplitBuildsIndependentSegments(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("拆分书", "split", []byte("cover")))
	pyOutDir := filepath.Join(dir, "py-split")
	goOutDir := filepath.Join(dir, "go-split")

	_, pyJSON := runPythonHarness(t, "epub_package_split_harness.py", source,
		"--output-dir", pyOutDir, "--split-points", "0")

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{
		SplitPoints:  []int{0},
		OutputDir:    goOutDir,
		LegacyReport: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s", res.Status)
	}

	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatalf("Facts 缺少 legacyReport")
	}
	got := replaceAll(string(raw), goOutDir, "<OUT>")
	want := replaceAll(pyJSON, pyOutDir, "<OUT>")
	if got != want {
		t.Errorf("legacy JSON 与 Python oracle 不一致:\n--- go ---\n%s\n--- python ---\n%s", clip(got, 1500), clip(want, 1500))
	}

	pySeg := filepath.Join(pyOutDir, "source_01.epub")
	goSeg := filepath.Join(goOutDir, "source_01.epub")
	compareEntries(t, pySeg, goSeg, map[string]bool{
		"OEBPS/content.opf": true, // 整树重序列化 vs 字节区间编辑：语义一致
	})
}

func TestSplitRefusesEncryptionAndBadPoints(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.epub")
	entries := append(writeBookEntries("书", "m", []byte("c")), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`),
	})
	encPath := filepath.Join(dir, "enc.epub")
	buildEpub(t, encPath, entries)
	buildEpub(t, src, writeBookEntries("书", "m", []byte("c")))
	outDir := filepath.Join(dir, "out")

	b, err := book.Open(encPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || res.Findings[0].Title !=
		"split: encrypted EPUB resources detected; refusing package rewrite" {
		t.Errorf("加密输入应拒绝: %+v", res.Findings)
	}

	b2, err := book.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	res2, err := Run(t.Context(), b2, Params{SplitPoints: []int{5}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != report.StatusFailed || res2.Findings[0].Title != "split point out of range: 5" {
		t.Errorf("越界切分点应拒绝: %+v", res2.Findings)
	}
	res3, err := Run(t.Context(), b2, Params{OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res3.Status != report.StatusFailed || res3.Findings[0].Title != "split: at least one split point is required" {
		t.Errorf("缺切分点应拒绝: %+v", res3.Findings)
	}
}

func TestSplitProjectionValidationManual(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("拆分书", "split", []byte("cover")))
	outDir := filepath.Join(dir, "out")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s findings=%+v", res.Status, res.Findings)
	}
	if _, err := os.Stat(filepath.Join(outDir, "source_01.epub")); err != nil {
		t.Fatalf("segment not committed: %v", err)
	}
}

func writeTwoChapterEntries(badSecond bool) []zipEntry {
	f := func(s string) []byte { return []byte(s) }
	second := `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="two">第二章</h1><p>第二段正文。</p></body></html>`
	if badSecond {
		second = `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="two">第二章`
	}
	return []zipEntry{
		{name: "META-INF/container.xml", content: f(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: f(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:two</dc:identifier><dc:title>两章书</dc:title><dc:creator>作者</dc:creator><dc:language>zh-CN</dc:language><meta name="cover" content="cover-image"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/></manifest><spine><itemref idref="nav" linear="no"/><itemref idref="one"/><itemref idref="two"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", content: f(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/one.xhtml#one">第一章</a></li><li><a href="Text/two.xhtml#two">第二章</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Text/one.xhtml", content: f(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="one">第一章</h1><p>第一段正文。</p></body></html>`)},
		{name: "OEBPS/Text/two.xhtml", content: f(second)},
		{name: "OEBPS/Images/cover.jpg", content: []byte("cover bytes")},
		{name: "mimetype", content: f("wrong")},
	}
}

func TestSplitOutputDirectoryPreflightAndDryRun(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: ""})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("empty output dir should fail: %+v", res)
	}
	if _, statErr := os.Stat("source_01.epub"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty output dir may not write cwd: %v", statErr)
	}

	existingDir := filepath.Join(dir, "existing-dir")
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existingDir, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: existingDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Fatalf("existing output dir should fail: %+v", res)
	}
	if got, readErr := os.ReadFile(marker); readErr != nil || string(got) != "keep" {
		t.Fatalf("existing output dir changed: %q (%v)", got, readErr)
	}

	existingFile := filepath.Join(dir, "existing-file")
	if err := os.WriteFile(existingFile, []byte("keep file"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: existingFile})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Fatalf("existing output file should fail: %+v", res)
	}
	if got, readErr := os.ReadFile(existingFile); readErr != nil || string(got) != "keep file" {
		t.Fatalf("existing output file changed: %q (%v)", got, readErr)
	}

	dryDir := filepath.Join(dir, "dry-run")
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: dryDir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("dry-run capability result should remain complete for pipeline: %s", res.Status)
	}
	if _, statErr := os.Stat(dryDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dry-run created output directory: %v", statErr)
	}
	planned, ok := res.Facts["plannedOutputs"].([]string)
	if !ok || len(planned) != 2 {
		t.Fatalf("dry-run planned outputs = %#v", res.Facts["plannedOutputs"])
	}
}

func TestSplitValidationFailureLeavesNoArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(true))
	outDir := filepath.Join(dir, "segments")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("malformed second segment should fail with finding: %+v", res)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed split left output directory: %v", statErr)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(source) {
			t.Errorf("failed split left sibling artifact: %s", entry.Name())
		}
	}
}

func TestSplitCommitsAllSegmentsTogether(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))
	outDir := filepath.Join(dir, "segments")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s findings=%+v", res.Status, res.Findings)
	}
	for _, name := range []string{"source_01.epub", "source_02.epub"} {
		if _, statErr := os.Stat(filepath.Join(outDir, name)); statErr != nil {
			t.Fatalf("missing committed segment %s: %v", name, statErr)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected transaction siblings: %+v", entries)
	}
}
