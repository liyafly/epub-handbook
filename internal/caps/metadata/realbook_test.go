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

// TestRealBookMetadataWriteOnlyChangesMetadata 用仓库样本书（51MB 真实
// EPUB，references/epubs/）验证 metadata.edit 在真书体量下仍然只改 OPF
// 的 dc 字段：dc:title/author/publisher 必须真的变成新值，同时正文、
// spine 顺序与锚点必须零发现（redline 门禁）。Python oracle 已删除，这里
// 不再对拍 Python 输出，改成对真书本身的语义断言。样本书文件缺失时才
// 跳过（CI 里它入 git；本地缺书是环境问题，不是能力缺陷，不许因为 oracle
// 缺失而跳过）。
func TestRealBookMetadataWriteOnlyChangesMetadata(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(repo, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		t.Skip("没有样本书")
	}
	source := matches[0]

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	fieldsJSON := `{"title": "EPub指南（Go 语义校验）", "author": "语义校验作者", "publisher": "语义校验出版社"}`
	res, err := Run(context.Background(), b, Params{MetadataJSON: fieldsJSON})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	updated, _ := res.Facts["fieldsUpdated"].(int)
	if updated == 0 {
		t.Fatalf("fieldsUpdated = %v，真书至少应有一个字段被改动", res.Facts["fieldsUpdated"])
	}

	opfPath, _ := res.Facts["opf"].(string)
	opfData, err := b.Current(opfPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"EPub指南（Go 语义校验）",
		"语义校验作者",
		"语义校验出版社",
	} {
		if !bytes.Contains(opfData, []byte(want)) {
			t.Errorf("真书 OPF 缺少写入的新值 %q", want)
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
