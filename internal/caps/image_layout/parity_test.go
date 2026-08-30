// parity_test.go 按 internal/redline/parity_test.go 的模式做 P2 parity：
// 同一 fixture EPUB 分别跑 Python oracle 与 Go 实现，legacyReport 逐字节比对。
// epub 路径字段按原样出现在报告中，比对前替换为占位符。
package imagelayout

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
)

const advisorScript = "scripts/epub_image_layout_advisor.py"

// runPythonAdvisor 跑 Python oracle，返回 (退出码, stdout)。
func runPythonAdvisor(t *testing.T, epubPath string) (int, string) {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, advisorScript)
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/epub_image_layout_advisor.py 不存在（oracle 已删除）")
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

func assertAdvisorParity(t *testing.T, epubPath string) {
	t.Helper()
	wantCode, wantText := runPythonAdvisor(t, epubPath)
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
	got := normalizePathField(string(raw), epubPath)
	want := normalizePathField(wantText, epubPath)
	if got != want {
		t.Errorf("legacyReport 逐字节不一致:\n--- go ---\n%s\n--- python ---\n%s", got, want)
	}
	// advisor 恒定退出 0（findings 不影响退出码），Go 恒为 complete。
	if wantCode != 0 || res.Status != "complete" {
		t.Errorf("退出码/状态不一致: python=%d go=%s", wantCode, res.Status)
	}
}

func TestParityAdvisorHit(t *testing.T) {
	// 命中场景：裸图（lone / missing-alt / caption）。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"chapter.xhtml", `<img src="../Images/test.png"/><p>图注</p>`, ""},
	}, []string{"chapter.xhtml"})
	assertAdvisorParity(t, path)
}

func TestParityAdvisorMixedHit(t *testing.T) {
	// 命中场景 2：float 风险 + 图注 + 整页候选，同一书内多文件。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"bad.xhtml", `<img src="../Images/test.png" alt="test" style="float:left;width:50%"/><p>正文。</p>`, ""},
		{"fullpage.xhtml", `<figure><img src="../Images/test.png" alt="volume"/></figure>`, ""},
	}, []string{"bad.xhtml", "fullpage.xhtml"})
	assertAdvisorParity(t, path)
}

func TestParityAdvisorMiss(t *testing.T) {
	// 未命中场景：规范 figure/figcaption，无任何 finding。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"good.xhtml", `<figure><img src="../Images/test.png" alt="test"/></figure><p>这是普通正文段落。</p>`, ""},
	}, []string{"good.xhtml"})
	assertAdvisorParity(t, path)
}

func TestParityAdvisorChapterHeadAndNoteref(t *testing.T) {
	// 命中 + 豁免混合：章节头候选（CSS 规范 figure 不触发 float 风险）、
	// noteref 图标豁免；整书 finding 集与 Python 一致。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"head.xhtml", `<figure class="img-left" style="width:30%"><img src="../Images/test.png" alt="chapter" style="width:100%;height:auto"/></figure><h1>标题</h1><p>正文。</p>`, ""},
		{"notes.xhtml", `<p>正文<sup><a class="noteref-icon" href="#note"><img src="../Images/test.png" alt="注"/></a></sup>继续。</p>`, ""},
	}, []string{"head.xhtml"})
	assertAdvisorParity(t, path)
}
