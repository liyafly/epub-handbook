package merge

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ---- Python oracle ----

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

// readZipEntries 解压出 name → content 映射（忽略目录项）。
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
		if zf.Name == "mimetype" || endsWithAny(zf.Name, "/") {
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

func endsWithAny(s, suf string) bool { return stringsHasSuffix(s, suf) }

// compareEntries 逐 entry 比对两个 EPUB 的解压内容；normalize 用于抹平
// 不确定字段（如 dcterms:modified 时间戳）。
func compareEntries(t *testing.T, pyPath, goPath string, normalize func(string) string, semantic map[string]bool) {
	t.Helper()
	py := readZipEntries(t, pyPath)
	go_ := readZipEntries(t, goPath)
	for name := range py {
		if _, ok := go_[name]; !ok {
			t.Errorf("Go 输出缺少 entry %s", name)
			continue
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
		pyB, goB := py[name], go_[name]
		if semantic[name] {
			pyC, goC := pyCanonicalXML(t, pyPath, name), pyCanonicalXML(t, goPath, name)
			if pyC != goC {
				t.Errorf("entry %s 语义不一致（Python 整树重序列化 vs Go 字节区间编辑）：\n--- py ---\n%s\n--- go ---\n%s",
					name, clip(pyC, 1600), clip(goC, 1600))
			}
			continue
		}
		pyN, goN := normalize(string(pyB)), normalize(string(goB))
		if pyN != goN {
			t.Errorf("entry %s 字节不一致：\n--- py ---\n%s\n--- go ---\n%s", name, clip(pyN, 1200), clip(goN, 1200))
		}
	}
}

// pyCanonicalXML 用 Python ET 把 EPUB 内某 entry 规范化为 JSON
// （ Clark 名 + 有序属性 + 文本 + 子树），用于语义比对。
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

var modifiedTSRe = regexp.MustCompile(`(<meta property="dcterms:modified">)[^<]*(</meta>)`)

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func assertLegacyJSON(t *testing.T, res report.Result, pyJSON, outA, outB string) string {
	t.Helper()
	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatalf("Facts 缺少 legacyReport")
	}
	got := normalizePaths(string(raw), outA, outB)
	want := normalizePaths(pyJSON, outA, outB)
	if got != want {
		t.Errorf("legacy JSON 与 Python oracle 不一致:\n--- go ---\n%s\n--- python ---\n%s", clip(got, 1500), clip(want, 1500))
	}
	return got
}

func normalizePaths(s string, olds ...string) string {
	for _, old := range olds {
		s = replaceAll(s, old, "<PATH>")
	}
	return s
}

func TestParityMergeRewritesConflictingResources(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.epub")
	second := filepath.Join(dir, "second.epub")
	pyOut := filepath.Join(dir, "py-merged.epub")
	goOut := filepath.Join(dir, "go-merged.epub")
	buildEpub(t, first, writeBookEntries("第一册", "book-a", []byte("cover-a")))
	buildEpub(t, second, writeBookEntries("第二册", "book-b", []byte("cover-b")))

	// Python oracle。
	_, pyJSON := runPythonHarness(t, "epub_package_merge_harness.py", first, second,
		"--output", pyOut, "--title", "合集")

	// Go 实现。
	b, err := book.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs:       []string{first, second},
		Title:        strPtr("合集"),
		Output:       goOut,
		LegacyReport: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s", res.Status)
	}
	if err := b.WriteTo(goOut); err != nil {
		t.Fatal(err)
	}

	assertLegacyJSON(t, res, pyJSON, pyOut, goOut)
	// merge 的 OPF / nav / ncx 均为新建产物：除 dcterms:modified 时间戳外
	// 应与 Python 逐字节一致（P3 级）。
	compareEntries(t, pyOut, goOut, func(s string) string {
		return string(modifiedTSRe.ReplaceAll([]byte(s), []byte("${1}TS${2}")))
	}, nil)
}

func TestMergeRefusesEncryptedAndShortInput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.epub")
	buildEpub(t, src, writeBookEntries("书", "m", []byte("c")))
	encrypted := append(writeBookEntries("书", "m", []byte("c")), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`),
	})
	encPath := filepath.Join(dir, "enc.epub")
	buildEpub(t, encPath, encrypted)

	b, err := book.Open(encPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs: []string{encPath, src}, Output: filepath.Join(dir, "out.epub"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Errorf("加密输入应 failed，got %s", res.Status)
	}
	if len(res.Findings) == 0 || res.Findings[0].Title !=
		"merge: encrypted EPUB resources detected; refusing package rewrite" {
		t.Errorf("拒绝措辞不一致: %+v", res.Findings)
	}

	b2, err := book.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	res2, err := Run(context.Background(), b2, Params{Inputs: []string{src}})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != report.StatusFailed || res2.Findings[0].Title != "merge requires at least two input EPUB files" {
		t.Errorf("单卷输入应拒绝: %+v", res2.Findings)
	}
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
