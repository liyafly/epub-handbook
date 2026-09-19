package fontcoverage

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// fakeDetectorJSON 是 detector 的最小合法报告（schema 1.0），带 kindle 档案的
// risk 与一条 unresolved run，使 status 落到 warn。
const fakeDetectorJSON = `{
  "schema_version": "1.0",
  "summary": {"by_profile_risk": {"kindle-pessimistic": {"ok": 3, "risk": 1, "fail": 0}}, "unresolved_runs": 1},
  "char_inventory": [{"char": "𰻞", "codepoint": "U+30EDE", "locations": ["OEBPS/Text/c1.xhtml#p3"], "reason": "not in font"}],
  "unresolved": [{"selector": "p.special", "reason": "font-family not declared"}],
  "chain_health": {"serif": "ok", "sans": "degraded"},
  "text_runs": [{"file": "OEBPS/Text/c1.xhtml", "runs": 12}]
}`

// installFakeUV 在 PATH 前插一个假的 uv：忽略参数，打印固定 JSON 到 stdout、
// 一行诊断到 stderr，退出码 0。
func installFakeUV(t *testing.T, stdout string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("假 uv 依赖 POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' \"$FAKE_DETECTOR_STDOUT\"\necho 'detector: fake run' >&2\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "uv"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_DETECTOR_STDOUT", stdout)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func minimalEpub(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.epub")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	entries := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/package.opf":      `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>t</dc:title></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`,
		"OEBPS/Text/c1.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>正文</p></body></html>`,
	}
	for _, name := range []string{"mimetype", "META-INF/container.xml", "OEBPS/package.opf", "OEBPS/Text/c1.xhtml"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunPromotesDetectorSectionsToFacts 锁定 detector 各段进入正式 facts 的
// 键名与 status 判定。
func TestRunPromotesDetectorSectionsToFacts(t *testing.T) {
	installFakeUV(t, fakeDetectorJSON)
	b, err := book.Open(minimalEpub(t))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{Profile: "kindle-pessimistic", ToolRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != report.StatusComplete || res.Facts["status"] != "warn" || res.Facts["profile"] != "kindle-pessimistic" {
		t.Fatalf("status=%s facts=%v, want complete/warn", res.Status, res.Facts)
	}
	if len(res.Findings) != 1 || res.Findings[0].ID != "fontcoverage.risk" || res.Findings[0].Level != "warn" {
		t.Errorf("findings = %+v, want one warn fontcoverage.risk", res.Findings)
	}
	summary, _ := res.Facts["summary"].(map[string]any)
	if summary["unresolved_runs"] != float64(1) {
		t.Errorf("summary = %v", res.Facts["summary"])
	}
	inv, _ := res.Facts["charInventory"].([]any)
	if len(inv) != 1 {
		t.Fatalf("charInventory = %v", res.Facts["charInventory"])
	}
	if item, _ := inv[0].(map[string]any); item["codepoint"] != "U+30EDE" || item["reason"] != "not in font" {
		t.Errorf("charInventory[0] = %v", inv[0])
	}
	if unresolved, _ := res.Facts["unresolved"].([]any); len(unresolved) != 1 {
		t.Errorf("unresolved = %v", res.Facts["unresolved"])
	}
	chain, _ := res.Facts["chainHealth"].(map[string]any)
	if chain["sans"] != "degraded" || chain["serif"] != "ok" {
		t.Errorf("chainHealth = %v", res.Facts["chainHealth"])
	}
	if runs, _ := res.Facts["textRuns"].([]any); len(runs) != 1 {
		t.Errorf("textRuns = %v", res.Facts["textRuns"])
	}
	if res.Facts["detectorExitCode"] != 0 || res.Facts["detectorStderr"] != "detector: fake run" {
		t.Errorf("detectorExitCode=%v detectorStderr=%v", res.Facts["detectorExitCode"], res.Facts["detectorStderr"])
	}
	for _, oldKey := range []string{"char_inventory", "chain_health", "text_runs"} {
		if _, ok := res.Facts[oldKey]; ok {
			t.Errorf("facts 不应含 snake_case 段名 %s", oldKey)
		}
	}
}

// TestRunFailsWhenDetectorReturnsNonJSON 锁定 adapter 失败路径的 finding。
func TestRunFailsWhenDetectorReturnsNonJSON(t *testing.T) {
	installFakeUV(t, "not json")
	b, err := book.Open(minimalEpub(t))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{ToolRoot: t.TempDir()})
	if err == nil {
		t.Fatal("期望 adapter 错误")
	}
	if res.Status != report.StatusFailed || len(res.Findings) != 1 || res.Findings[0].ID != "fontcoverage.adapter" {
		t.Errorf("status=%s findings=%+v", res.Status, res.Findings)
	}
}
