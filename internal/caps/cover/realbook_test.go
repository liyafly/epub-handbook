package cover

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestReplaceCoverKeepsEscapedProse 用合成 EPUB 覆盖图片引用改名和正文
// 不变门禁；外形像图片标记的转义示例必须保持原文。
func TestReplaceCoverKeepsEscapedProse(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	entries := writeBookEntries("封面测试书", "cover", []byte("old-cover"))
	for i := range entries {
		if entries[i].name == "OEBPS/Text/chapter.xhtml" {
			entries[i].content = bytes.Replace(entries[i].content, []byte("</body>"),
				[]byte(`<p>示例：&lt;img src="../Images/cover.jpg"/&gt;</p></body>`), 1)
		}
	}
	buildEpub(t, source, entries)

	cover := filepath.Join(dir, "new-cover.png")
	dims := pngDimsHeader()
	if err := os.WriteFile(cover, dims, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cover.epub")

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Cover: cover, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	assertOperationFacts(t, res, out)
	if len(res.Renames) == 0 {
		t.Fatal("fixture should rename the cover image, otherwise this regression is ineffective")
	}
	mappings, ok := res.Facts["mappings"].([]map[string]string)
	if !ok || len(mappings) != len(res.Renames) {
		t.Fatalf("facts[mappings] 应与 Renames 一一对应: mappings=%v renames=%v", res.Facts["mappings"], res.Renames)
	}
	for _, m := range mappings {
		if to, exists := res.Renames[m["from"]]; !exists || to != m["to"] {
			t.Errorf("mappings 条目 %v 与 Renames 不一致（Renames[%q]=%q）", m, m["from"], to)
		}
	}

	// 正文不变：区域感知错误必须由红线发现重写器对转义示例文本的误改。
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText}, redline.Options{PathMap: res.Renames})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("redline %s: %s", f.Check, f.Message)
	}
	chapter := readZipEntries(t, out)["OEBPS/Text/chapter.xhtml"]
	if !bytes.Contains(chapter, []byte(`<p>示例：&lt;img src="../Images/cover.jpg"/&gt;</p>`)) {
		t.Errorf("escaped prose was changed: %s", chapter)
	}
}
