package cover

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestParityRealBook 用仓库样本书对 Python oracle 做真书 parity。
// 样本书含大量 href 带 "cover" 的图片 manifest 项 —— Python 规则会把
// 它们全部移出 manifest 并删除文件、把引用重写到新封面路径；
// 这里验证 Go 逐条复刻该行为（含 SVG 封面页的 viewBox/width/height 对齐）。
func TestParityRealBook(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(repo, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		t.Skip("没有样本书")
	}
	source := matches[0]

	dir := t.TempDir()
	pyDir, goDir := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	os.MkdirAll(pyDir, 0o755)
	os.MkdirAll(goDir, 0o755)
	pyCover := filepath.Join(pyDir, "new-cover.png")
	goCover := filepath.Join(goDir, "new-cover.png")
	if err := os.WriteFile(pyCover, pngDimsHeader(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goCover, pngDimsHeader(), 0o644); err != nil {
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
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
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
