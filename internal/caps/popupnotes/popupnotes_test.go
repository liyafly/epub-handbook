package popupnotes

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// fixture XHTML 模板（Python 侧用 ET 解析，必须是良构 XML）。
func popupXHTML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>fixture</title></head>
  <body>
` + body + `  </body>
</html>
`
}

const popupOPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:test:popup</dc:identifier>
    <dc:title>Popup Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="note-icon" href="Icons/note.png" media-type="image/png"/>
    <item id="t-a" href="Text/a-bad-noteref.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-b" href="Text/b-bad-aside.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-c" href="Text/c-bad-backlink.xhtml" media-type="application/xhtml+xml"/>
    <item id="t-v" href="Text/valid.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="t-a"/><itemref idref="t-b"/><itemref idref="t-c"/><itemref idref="t-v"/>
  </spine>
</package>
`

const popupContainer = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`

// brokenFixture 覆盖 12 类错误措辞；validFixture 只有合法弹注。
func brokenFixture() map[string]string {
	bodyA := `    <p id="dup">一</p>
    <p id="dup">二</p>
    <p>正文<a class="noteref-icon" role="doc-noteref" href="#a1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="a1">注<a epub:type="backlink" role="doc-backlink" href="#nr-missing">↩</a></li>
      </ol>
    </aside>
`
	bodyB := `    <p>正文<a id="b1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#missing"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside role="doc-footnote">
      <ol class="footnote-list"><li class="footnote-item" id="x1">甲</li></ol>
      <ol class="footnote-list"><li class="footnote-item" id="x2">乙</li></ol>
    </aside>
`
	bodyC := `    <p>正文<a id="nr1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#c1"><img src="../Icons/missing-icon.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="c1">注<a epub:type="backlink" href="#c1">↩</a></li>
      </ol>
    </aside>
`
	bodyV := `    <p>正文<a id="v1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li>
      </ol>
    </aside>
`
	files := map[string]string{
		"META-INF/container.xml":          popupContainer,
		"OEBPS/content.opf":               popupOPF,
		"OEBPS/Icons/note.png":            "png",
		"OEBPS/Text/a-bad-noteref.xhtml":  popupXHTML(bodyA),
		"OEBPS/Text/b-bad-aside.xhtml":    popupXHTML(bodyB),
		"OEBPS/Text/c-bad-backlink.xhtml": popupXHTML(bodyC),
		"OEBPS/Text/valid.xhtml":          popupXHTML(bodyV),
	}
	return files
}

func validFixture() map[string]string {
	body := `    <p>正文<a id="v1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p>
    <aside epub:type="footnote" role="doc-footnote">
      <ol class="footnote-list">
        <li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#v1">↩</a></li>
      </ol>
    </aside>
`
	return map[string]string{
		"META-INF/container.xml": popupContainer,
		"OEBPS/content.opf":      strings.Replace(popupOPF, `"/><item id="t-a"`, `"/><item id="t-v" href="Text/valid.xhtml" media-type="application/xhtml+xml"/><item id="t-a"`, 1),
		"OEBPS/Icons/note.png":   "png",
		"OEBPS/Text/valid.xhtml": popupXHTML(body),
	}
}

func writePopupEpub(t *testing.T, path string, files map[string]string) {
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

// normPyLine 把 Python 临时目录前缀归一为 zip 路径形态。
func normPyLine(line string) string {
	if i := strings.Index(line, "/OEBPS"); i >= 0 {
		return "ERROR: " + line[i+1:]
	}
	return line
}

func runPopupOracle(t *testing.T, epub string) (int, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, "scripts", "validate_popup_notes.py")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("scripts/validate_popup_notes.py 不存在（oracle 已删除）")
	}
	cmd := exec.Command("python3", script, "--epub", epub)
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
	return code, out.String(), errb.String()
}

func runGoPopup(t *testing.T, epub string) (string, []string) {
	t.Helper()
	b, err := book.Open(epub)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := res.Facts["legacyReport"].(map[string]any)
	if !ok {
		t.Fatalf("legacyReport 形状错误: %T", res.Facts["legacyReport"])
	}
	lines, ok := raw["lines"].([]string)
	if !ok {
		t.Fatalf("legacyReport.lines 形状错误: %T", raw["lines"])
	}
	return res.Status, lines
}

func TestParityPopupNotesErrors(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "broken.epub")
	writePopupEpub(t, epub, brokenFixture())

	code, _, stderr := runPopupOracle(t, epub)
	if code != 1 {
		t.Fatalf("python oracle 应退出 1，实际 %d\nstderr: %s", code, stderr)
	}
	var pyLines []string
	for _, line := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
		if line != "" {
			pyLines = append(pyLines, normPyLine(line))
		}
	}

	status, goLines := runGoPopup(t, epub)
	if status != "failed" {
		t.Fatalf("go status 应为 failed，实际 %s", status)
	}
	if strings.Join(goLines, "\n") != strings.Join(pyLines, "\n") {
		t.Errorf("错误措辞不一致:\n--- python ---\n%s\n--- go ---\n%s",
			strings.Join(pyLines, "\n"), strings.Join(goLines, "\n"))
	}
}

func TestParityPopupNotesOK(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "valid.epub")
	writePopupEpub(t, epub, validFixture())

	code, stdout, stderr := runPopupOracle(t, epub)
	if code != 0 {
		t.Fatalf("python oracle 应退出 0，实际 %d\nstderr: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "popup note validation ok" {
		t.Fatalf("python stdout: %q", stdout)
	}
	status, goLines := runGoPopup(t, epub)
	if status != "complete" {
		t.Fatalf("go status 应为 complete，实际 %s", status)
	}
	if len(goLines) != 1 || goLines[0] != "popup note validation ok" {
		t.Fatalf("go lines: %v", goLines)
	}
}
