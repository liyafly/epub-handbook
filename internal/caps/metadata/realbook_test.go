package metadata

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestMetadataWriteOnlyChangesMetadata 用合成 EPUB 验证 metadata.edit 只改
// OPF 的 dc 字段，同时正文、spine 顺序与锚点必须零发现（redline 门禁）。
func TestMetadataWriteOnlyChangesMetadata(t *testing.T) {
	source := filepath.Join(t.TempDir(), "metadata-source.epub")
	buildEpub(t, source, writeBookEntries("原题", "metadata"))

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	fieldsJSON := `{"title": "EPUB 元数据测试", "author": "语义校验作者", "publisher": "语义校验出版社"}`
	res, err := Run(context.Background(), b, Params{MetadataJSON: fieldsJSON})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	updated, _ := res.Facts["fieldsUpdated"].(int)
	if updated == 0 {
		t.Fatalf("fieldsUpdated = %v，至少应有一个字段被改动", res.Facts["fieldsUpdated"])
	}

	opfPath, _ := res.Facts["opf"].(string)
	opfData, err := b.Current(opfPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"EPUB 元数据测试",
		"语义校验作者",
		"语义校验出版社",
	} {
		if !bytes.Contains(opfData, []byte(want)) {
			t.Errorf("OPF 缺少写入的新值 %q", want)
		}
	}

	// 红线门禁：正文 / spine / 锚点必须零发现。CheckMetadata 会被这次写入
	// 本身触发，不放进这一组，否则会掩盖真正的回归。
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText, redline.CheckSpine, redline.CheckAnchors}, redline.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("redline %s: %s", f.Check, f.Message)
	}
}
