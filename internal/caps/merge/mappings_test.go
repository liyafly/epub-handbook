package merge

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestMergeExposesRenamesAsPathMapFact 锁定「改名信息必须出得了信封」：
// Result.Renames 只是 pipeline 内部喂给红线的通道，agent 看不到；合并时
// 冲突资源被改名后，`epub redline --path-map` 需要同一份映射才能不把改名
// 判成"文件缺失 + 新增文件"。因此 facts 里必须有与 Renames 等价、且
// redline.LoadPathMap 能直接吃下的 {from,to} 数组。
func TestMergeExposesRenamesAsPathMapFact(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.epub")
	second := filepath.Join(dir, "second.epub")
	out := filepath.Join(dir, "merged.epub")
	buildEpub(t, first, writeBookEntries("第一册", "book-a", []byte("cover-a")))
	buildEpub(t, second, writeBookEntries("第二册", "book-b", []byte("cover-b")))
	b, err := book.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs: []string{first, second},
		Title:  strPtr("合集"),
		Output: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if len(res.Renames) == 0 {
		t.Fatal("fixture 应当产生资源改名，Renames 为空说明用例失效")
	}

	// facts.mappings 必须与 Renames 一一对应。
	got, ok := res.Facts["mappings"].([]map[string]string)
	if !ok {
		t.Fatalf("facts[\"mappings\"] 类型 = %T", res.Facts["mappings"])
	}
	if len(got) != len(res.Renames) {
		t.Fatalf("mappings 条数 = %d, Renames = %d", len(got), len(res.Renames))
	}
	for _, m := range got {
		if to, exists := res.Renames[m["from"]]; !exists || to != m["to"] {
			t.Errorf("mappings 条目 %v 与 Renames 不一致（Renames[%q]=%q）", m, m["from"], to)
		}
	}

	// 走一遍真实消费路径：包成 v2 信封后 redline.LoadPathMap 必须认得。
	envelope, err := json.Marshal(map[string]any{
		"schemaVersion": "2",
		"capability":    CapabilityID,
		"status":        string(report.StatusComplete),
		"facts":         map[string]any{CapabilityID + ".mappings": got},
	})
	if err != nil {
		t.Fatal(err)
	}
	pathMap, err := redline.LoadPathMap(envelope)
	if err != nil {
		t.Fatalf("LoadPathMap 拒绝了本能力自己的信封: %v", err)
	}
	for from, to := range res.Renames {
		if pathMap[from] != to {
			t.Errorf("path map 缺少 %q -> %q（得到 %q）", from, to, pathMap[from])
		}
	}
}
