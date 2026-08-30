package redline

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type zipEntry struct {
	name    string
	content []byte
	method  uint16
}

func buildEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name}
		h.Method = e.method
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

const opfXML = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="id">
  <metadata>
    <dc:title>测试书</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:identifier id="id">urn:uuid:1234</dc:identifier>
    <dc:language>zh-CN</dc:language>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.png" media-type="image/png" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="nav" linear="no"/>
    <itemref idref="c1"/>
  </spine>
</package>
`

const c1XHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN">
  <head><title>第一章</title></head>
  <body>
    <p id="p1">第一段落。</p>
    <p>第二段落　全角空格。</p>
    <p>汉字<ruby>字<rt>zì</rt></ruby>注音。</p>
  </body>
</html>
`

func baseEntries() []zipEntry {
	return []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip"), method: 0},
		{name: "META-INF/container.xml", content: []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: []byte(opfXML)},
		{name: "OEBPS/Text/c1.xhtml", content: []byte(c1XHTML)},
		{name: "OEBPS/nav.xhtml", content: []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">第一章</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Styles/main.css", content: []byte("p { margin: 0; }\n")},
		{name: "OEBPS/Images/cover.png", content: bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 32)},
	}
}

// runPythonOracle 用仓库里的 validate_text_invariance.py 跑同一组输入，
// 返回 (退出码, 报告文本)。脚本不可用时 t.Skip。
func runPythonOracle(t *testing.T, before, after, check string, extra ...string) (int, string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repoRoot, "scripts", "validate_text_invariance.py")
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/validate_text_invariance.py 不存在（oracle 已删除）")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	outFile := filepath.Join(t.TempDir(), "report.txt")
	args := append([]string{script, before, after, "--check", check, "--output", outFile}, extra...)
	cmd := exec.Command("python3", args...)
	cmd.Dir = repoRoot
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err = cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
		if code != 0 && code != 1 && code != 2 {
			t.Fatalf("python oracle 异常退出: %v\nstderr: %s", err, stderr.String())
		}
	} else if err != nil {
		t.Fatalf("运行 python oracle 失败: %v\n%s", err, stderr.String())
	}
	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		t.Fatalf("oracle 未写出报告文件: %v\nstderr: %s", readErr, stderr.String())
	}
	return code, string(data)
}

// assertParity 比对 Go CompareFiles 与 Python oracle 的退出码与逐字节报告。
func assertParity(t *testing.T, before, after, check string, o Options, extraArgs ...string) {
	t.Helper()
	wantCode, wantText := runPythonOracle(t, before, after, check, extraArgs...)
	rep, err := CompareFiles(before, after, check, o)
	if err != nil {
		t.Fatalf("CompareFiles: %v", err)
	}
	var got strings.Builder
	if len(rep.Lines) > 0 {
		got.WriteString(strings.Join(rep.Lines, "\n") + "\n")
	}
	if rep.Code != wantCode {
		t.Errorf("退出码不一致: go=%d python=%d\ngo 输出:\n%s", rep.Code, wantCode, got.String())
	}
	if got.String() != wantText {
		t.Errorf("报告逐字节不一致:\n--- go ---\n%s\n--- python ---\n%s", got.String(), wantText)
	}
}

func TestParityIdentical(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	buildEpub(t, after, baseEntries())
	assertParity(t, before, after, "all", Options{})
}

func TestParityTextChanged(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	entries := baseEntries()
	buildEpub(t, before, entries)
	modified := entries
	for i := range modified {
		if modified[i].name == "OEBPS/Text/c1.xhtml" {
			modified[i].content = bytes.Replace(entries[i].content, []byte("第一段落"), []byte("第一段落!"), 1)
		}
	}
	buildEpub(t, after, modified)
	assertParity(t, before, after, "text", Options{})
}

func TestParityAnchorDeleted(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	modified := baseEntries()
	for i := range modified {
		if modified[i].name == "OEBPS/Text/c1.xhtml" {
			modified[i].content = bytes.Replace(entries(modified)[i].content, []byte(` id="p1"`), nil, 1)
		}
	}
	buildEpub(t, after, modified)
	assertParity(t, before, after, "anchors", Options{})
}

func TestParityMetadataAndSpine(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	modified := baseEntries()
	for i := range modified {
		if modified[i].name == "OEBPS/content.opf" {
			modified[i].content = bytes.Replace(modified[i].content, []byte("测试书"), []byte("新书名"), 1)
		}
	}
	buildEpub(t, after, modified)
	assertParity(t, before, after, "metadata,spine", Options{})
}

func TestParityCoverChanged(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	modified := baseEntries()
	for i := range modified {
		if modified[i].name == "OEBPS/Images/cover.png" {
			modified[i].content = append(modified[i].content, 0xFF)
		}
	}
	buildEpub(t, after, modified)
	assertParity(t, before, after, "cover", Options{})
}

func TestParityDRMRefused(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	modified := append(baseEntries(), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="http://www.example.com/unknown"/><CipherReference URI="OEBPS/Text/c1.xhtml"/></EncryptedData></encryption>`),
	})
	buildEpub(t, after, modified)
	assertParity(t, before, after, "all", Options{})
}

func TestParityDRMStaleAllowed(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	stale := append(baseEntries(), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/><CipherReference URI="Fonts/gone.ttf"/></EncryptedData></encryption>`),
	})
	buildEpub(t, before, stale)
	buildEpub(t, after, stale)
	assertParity(t, before, after, "drm", Options{})
}

func TestParityPathMapRename(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	renamed := baseEntries()
	for i := range renamed {
		if renamed[i].name == "OEBPS/Text/c1.xhtml" {
			renamed[i].name = "OEBPS/Text/chapter1.xhtml"
		}
	}
	buildEpub(t, after, renamed)
	// path-map 报告（structure_tool 形状）。
	mapJSON := `{"stages":[{"mappings":[{"from":"OEBPS/Text/c1.xhtml","to":"OEBPS/Text/chapter1.xhtml"}]}]}`
	mapPath := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapPath, []byte(mapJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	pm, err := LoadPathMap([]byte(mapJSON))
	if err != nil {
		t.Fatal(err)
	}
	assertParity(t, before, after, "all", Options{PathMap: pm}, "--path-map", mapPath)
}

func TestParityAllowList(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpub(t, before, baseEntries())
	modified := baseEntries()
	for i := range modified {
		if modified[i].name == "OEBPS/nav.xhtml" {
			modified[i].content = append(modified[i].content, []byte("<!-- changed -->")...)
		}
	}
	buildEpub(t, after, modified)
	assertParity(t, before, after, "text", Options{AllowList: []string{"*/nav.xhtml"}}, "--allow-list", "*/nav.xhtml")
}

func entries(es []zipEntry) []zipEntry { return es }
