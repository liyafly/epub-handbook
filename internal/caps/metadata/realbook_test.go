package metadata

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestParityRealBook 用仓库样本书（references/epubs）对 Python oracle 做
// 真书 parity：legacy JSON 逐键一致、非 OPF entry 逐字节一致、OPF 语义
// 一致（Go 字节区间编辑保留原格式，Python 是整树重序列化）。
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
	pyOut := filepath.Join(pyDir, "metadata.epub")
	goOut := filepath.Join(goDir, "metadata.epub")
	fieldsJSON := `{"title": "EPub指南（Go parity 校验）", "author": "parity 作者", "publisher": "parity 出版社"}`

	_, pyJSON := runPythonHarness(t, "epub_metadata_edit_harness.py", source,
		"--output", pyOut, "--metadata-json", fieldsJSON)

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		MetadataJSON: fieldsJSON,
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
