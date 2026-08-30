package redline

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestInProcessCheckDetectsTextChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.epub")
	buildEpub(t, path, baseEntries())
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	orig, err := b.Original("OEBPS/Text/c1.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(orig, []byte("第一段落"), []byte("第一段落改"), 1)
	if err := b.Apply([]editset.Edit{
		{Path: "OEBPS/Text/c1.xhtml", Offset: 0, Length: int64(len(orig)), Replacement: edited},
	}); err != nil {
		t.Fatal(err)
	}

	findings, err := Check(OriginalState(b), CurrentState(b), []string{CheckText}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 4 || findings[0].Check != CheckText {
		t.Fatalf("findings = %+v", findings)
	}
	want := "text: modified OEBPS/Text/c1.xhtml -> OEBPS/Text/c1.xhtml: 3 blocks before, 3 after"
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
	if findings[3].Message != "    after:  第一段落改。" {
		t.Errorf("明细行不符: %q", findings[3].Message)
	}

	// 未修改的书必须零发现。
	b2, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	clean, err := Check(OriginalState(b2), CurrentState(b2), nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Errorf("干净的书不应有 findings: %+v", clean)
	}
}
