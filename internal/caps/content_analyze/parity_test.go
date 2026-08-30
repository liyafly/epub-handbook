// parity_test.go 按 internal/redline/parity_test.go 的模式做 P2 parity：
// 同一 fixture EPUB 分别跑 Python oracle 与 Go 实现，legacyReport 逐字节比对。
// 路径字段是绝对路径，比对前替换为占位符。
package contentanalyze

import (
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
	"github.com/liyafly/epub-handbook/internal/report"
)

const analyzerScript = "scripts/epub_content_analyzer.py"

// runPythonAnalyzer 跑 Python oracle，返回 (退出码, stdout)。
func runPythonAnalyzer(t *testing.T, epubPath string) (int, string) {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, analyzerScript)
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/epub_content_analyzer.py 不存在（oracle 已删除）")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	cmd := exec.Command("python3", script, epubPath, "--format", "json")
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

// normalizePathField 把绝对路径（含符号链接解析后的形态）替换为占位符。
func normalizePathField(s, path string) string {
	s = strings.ReplaceAll(s, path, "<INPUT>")
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		s = strings.ReplaceAll(s, resolved, "<INPUT>")
	}
	if abs, err := filepath.Abs(path); err == nil {
		s = strings.ReplaceAll(s, abs, "<INPUT>")
	}
	return s
}

// runGoAnalyzer 打开 fixture 并执行 Run，返回 (结果, legacy 原始字节)。
func runGoAnalyzer(t *testing.T, epubPath string) (report.Result, []byte) {
	t.Helper()
	b, err := book.Open(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatalf("Facts 缺少 legacyReport")
	}
	return res, raw
}

func assertAnalyzerParity(t *testing.T, epubPath string) {
	t.Helper()
	wantCode, wantText := runPythonAnalyzer(t, epubPath)
	res, raw := runGoAnalyzer(t, epubPath)
	got := normalizePathField(string(raw), epubPath)
	want := normalizePathField(wantText, epubPath)
	if got != want {
		t.Errorf("legacyReport 逐字节不一致:\n--- go ---\n%s\n--- python ---\n%s", got, want)
	}
	// 退出码语义：Python 1 ⇔ status fail ⇔ Go StatusFailed；其余 ⇔ complete。
	if wantCode == 1 && res.Status != report.StatusFailed {
		t.Errorf("python 退出码 1 但 Go status = %s", res.Status)
	}
	if wantCode == 0 && res.Status != "complete" {
		t.Errorf("python 退出码 0 但 Go status = %s", res.Status)
	}
}

func TestParityAnalyzerHit(t *testing.T) {
	// 命中场景：标题 / 正文 / 待复核短句混合 → warn。
	path := spineFixture(t, wrapXHTML(
		"<h1>第一章 风雪夜归人</h1><p>这是普通正文段落，长度足以稳定识别为正文。</p><p>春风又绿江南岸</p>", "", "zh-CN"), "", false)
	assertAnalyzerParity(t, path)
}

func TestParityAnalyzerCleanHit(t *testing.T) {
	// 全部显式角色 → pass：覆盖 epub:type 祖先、blockquote、pre/code、hr。
	path := spineFixture(t, wrapXHTML(
		`<h1>书名</h1><p class="subtitle">副标题</p>`+
			`<aside epub:type="footnote"><p>注释正文。</p></aside>`+
			`<blockquote>引用内容一行。</blockquote>`+
			`<pre><code>print(&quot;ok&quot;)</code></pre>`+
			`<hr/>`+
			`<p>这是普通正文段落，长度足以稳定识别为正文。</p>`, "", "zh-CN"), "", false)
	assertAnalyzerParity(t, path)
}

func TestParityAnalyzerMissNoBlocks(t *testing.T) {
	// 未命中场景：spine 文档没有块级标签 + 一份非法 UTF-8 文档 → fail（消息固定，可逐字节比对）。
	entries := []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip")},
		{name: "META-INF/container.xml", content: []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/package.opf", content: []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="Text/c2.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`)},
		{name: "OEBPS/Text/c1.xhtml", content: []byte(wrapXHTML("<div>只有普通文字没有块级标签</div>", "", "zh-CN"))},
		{name: "OEBPS/Text/c2.xhtml", content: []byte{0xff, 0xfe, '<', 'p', '>', 0x62, '<', '/', 'p', '>'}}, // 非法 UTF-8
		{name: "OEBPS/nav.xhtml", content: []byte(wrapXHTML(`<nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">目录</a></li></ol></nav>`, "", "zh-CN"))},
	}
	path := filepath.Join(t.TempDir(), "miss.epub")
	writeFixtureEpub(t, path, entries)
	assertAnalyzerParity(t, path)
}

func TestParityAnalyzerEncryptionRefused(t *testing.T) {
	// encryption.xml 拒绝：Python 打印 ERROR 到 stderr 且退出 1；Go 返回同文错误。
	path := spineFixture(t, wrapXHTML("<p>正文</p>", "", "zh-CN"), "", true)
	repo, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	script := filepath.Join(repo, analyzerScript)
	if _, err := os.Stat(script); err != nil {
		t.Skip("oracle 已删除")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	cmd := exec.Command("python3", script, path, "--format", "json")
	cmd.Dir = repo
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	code := 0
	if ee, ok := runErr.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	if code != 1 {
		t.Fatalf("python 退出码 = %d（want 1）stderr=%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "ERROR: encryption marker detected; content analysis stopped") {
		t.Errorf("python stderr = %q", errb.String())
	}
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, goErr := Run(context.Background(), b, Params{})
	if goErr == nil || goErr.Error() != "encryption marker detected; content analysis stopped" {
		t.Errorf("Go 错误 = %v", goErr)
	}
}
