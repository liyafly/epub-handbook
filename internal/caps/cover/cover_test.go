package cover

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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

// svgCoverFixture 复刻 test_replace_cover_resizes_svg_cover_page 的
// 改造版输入：加 cover-page.xhtml（内联 SVG 包裹封面图）+ PNG 头封面。
func svgCoverFixture() []zipEntry {
	entries := writeBookEntries("封面书", "cover", []byte("old-cover"))
	var out []zipEntry
	for _, e := range entries {
		if e.name == "OEBPS/content.opf" {
			e.content = bytes.Replace(e.content,
				[]byte(`<item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>`),
				[]byte(`<item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>\n    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml" properties="svg"/>`),
				1)
		}
		out = append(out, e)
	}
	out = append(out[:len(out)-1], // 保持 mimetype 仍在末尾前插入
		zipEntry{name: "OEBPS/Text/cover.xhtml", content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1654 2362"><image width="1654" height="2362" href="../Images/cover.jpg"/></svg></body>
</html>
`)},
		zipEntry{name: "mimetype", content: []byte("wrong-on-purpose")})
	return out
}

// pngDimsHeader 构造 1024x1536 的最小 PNG 头（与 Python 测试一致）。
func pngDimsHeader() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x06, 0x00,
		0x08, 0x02, 0x00, 0x00, 0x00,
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
// cover 的 OPF 在 Python 侧是整树重序列化、Go 侧是字节区间编辑
// （保留原格式与 dcterms:modified 等原字节）——因此 OPF 只比语义；
// XHTML/CSS 引用重写两侧使用同语义的正则扫描器，字节一致。
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
				t.Errorf("entry %s 语义不一致：\n--- py ---\n%s\n--- go ---\n%s", name, clip(pyC, 2000), clip(goC, 2000))
			}
			continue
		}
		if !bytes.Equal(py[name], go_[name]) {
			t.Errorf("entry %s 字节不一致：\n--- py ---\n%s\n--- go ---\n%s",
				name, clip(string(py[name]), 1200), clip(string(go_[name]), 1200))
		}
	}
}

func runCoverParity(t *testing.T, source string, coverBytes []byte, coverName string) {
	t.Helper()
	dir := filepath.Dir(source)
	pyDir, goDir := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	os.MkdirAll(pyDir, 0o755)
	os.MkdirAll(goDir, 0o755)
	pyCover := filepath.Join(pyDir, coverName)
	goCover := filepath.Join(goDir, coverName)
	if err := os.WriteFile(pyCover, coverBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goCover, coverBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	pyOut := filepath.Join(pyDir, "cover.epub")
	goOut := filepath.Join(goDir, "cover.epub")

	_, pyJSON := runPythonHarness(t, "epub_cover_replace_harness.py", source,
		"--output", pyOut, "--cover", pyCover)

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Cover:        goCover,
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

	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatalf("Facts 缺少 legacyReport")
	}
	norm := func(s string) string {
		s = replaceAll(s, dir, "<TMP>")
		s = replaceAll(s, "/go/", "/SIDE/")
		return replaceAll(s, "/py/", "/SIDE/")
	}
	if norm(string(raw)) != norm(pyJSON) {
		t.Errorf("legacy JSON 与 Python oracle 不一致:\n--- go ---\n%s\n--- python ---\n%s",
			clip(norm(string(raw)), 1500), clip(norm(pyJSON), 1500))
	}
	compareEntries(t, pyOut, goOut, map[string]bool{"OEBPS/content.opf": true})
}

func TestParityReplaceCover(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("封面书", "cover", []byte("old-cover")))
	runCoverParity(t, source, []byte("new-cover"), "new-cover.png")
}

func TestParityReplaceCoverResizesSVGPage(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, svgCoverFixture())
	runCoverParity(t, source, pngDimsHeader(), "new-cover.png")
}

func TestCoverMissingFileRefused(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("书", "m", []byte("c")))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Cover:  filepath.Join(dir, "nope.png"),
		Output: filepath.Join(dir, "out.epub"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed ||
		res.Findings[0].Title != "cover image not found: "+filepath.Join(dir, "nope.png") {
		t.Errorf("缺封面文件应拒绝: %+v", res.Findings)
	}
}
