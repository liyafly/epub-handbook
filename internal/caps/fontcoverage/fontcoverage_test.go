package fontcoverage

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// TestParityWithPythonOracle 对真书比对 Go 与 Python adapter 的完整报告。
func TestParityWithPythonOracle(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv 不可用")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "scripts", "epub_font_coverage_adapter.py")); err != nil {
		t.Skip("Python oracle 已删除")
	}
	matches, _ := filepath.Glob(filepath.Join(repoRoot, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		t.Skip("没有样本书")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}

	pyOut, err := exec.Command("python3",
		filepath.Join(repoRoot, "scripts", "epub_font_coverage_adapter.py"),
		matches[0]).Output()
	if err != nil && !isExitError(err) {
		t.Fatalf("python oracle 运行失败: %v", err)
	}
	var pyReport map[string]any
	if err := json.Unmarshal(pyOut, &pyReport); err != nil {
		t.Fatalf("oracle 输出非 JSON: %v", err)
	}

	b, err := book.Open(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{LegacyReport: true, Profile: "kindle-pessimistic"})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	goReport, ok := res.Facts["legacyReport"]
	if !ok {
		t.Fatal("缺 legacyReport")
	}

	if diff := diffJSON(t, goReport, pyReport); diff != "" {
		t.Errorf("与 Python oracle 不一致:\n%s", diff)
	}
}

func isExitError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

func diffJSON(t *testing.T, got, want any) string {
	t.Helper()
	gb := roundTrip(t, got)
	wb := roundTrip(t, want)
	if bytes.Equal(gb, wb) {
		return ""
	}
	return "--- go ---\n" + clip(string(gb), 1200) + "\n--- python ---\n" + clip(string(wb), 1200)
}

func roundTrip(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var any1 any
	_ = json.Unmarshal(raw, &any1)
	out, err := json.Marshal(any1)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
