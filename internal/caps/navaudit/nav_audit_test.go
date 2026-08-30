package navaudit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// TestParityWithPythonOracle 用仓库样本书对比 Go 与 Python preflight 的
// 完整 legacy JSON（键级等价）。oracle 缺失时跳过（W5 删除后自动跳）。
func TestParityWithPythonOracle(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Join(repoRoot, "scripts")
	if _, err := os.Stat(filepath.Join(scripts, "epub_preflight_harness.py")); err != nil {
		t.Skip("scripts/epub_preflight_harness.py 不存在（oracle 已删除）")
	}
	matches, _ := filepath.Glob(filepath.Join(repoRoot, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		t.Skip("没有样本书")
	}

	// Go 侧。
	b, err := book.Open(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(testContext(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	goReport := res.Facts["legacyReport"]

	// Python 侧。
	pyReport, code := runPyPreflight(t, repoRoot, scripts, matches[0])
	if res.Status == "failed" && code != 1 {
		t.Errorf("退出码语义不一致: go status=%s py code=%d", res.Status, code)
	}

	if diff := diffJSON(t, goReport, pyReport); diff != "" {
		t.Errorf("legacy JSON 与 Python oracle 不一致:\n%s", diff)
	}
}
