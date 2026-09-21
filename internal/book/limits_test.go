package book

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestRetainedBudgetCountsOriginalsAndReplacements(t *testing.T) {
	b, _ := openSample(t)
	b.maxRetainedBytes = 30
	if _, err := b.Original("mimetype"); err != nil { // 20 bytes
		t.Fatal(err)
	}
	if _, err := b.Original("mimetype"); err != nil { // Cache hits cost nothing.
		t.Fatal(err)
	}
	if _, err := b.Original("OEBPS/content.opf"); err != nil { // Exactly 30 total.
		t.Fatal(err)
	}
	if data, err := b.Original("META-INF/container.xml"); !errors.Is(err, ErrMemoryLimit) || data != nil {
		t.Fatalf("over-budget read = %q, %v", data, err)
	}
	if err := b.Apply([]editset.Edit{editset.Replace("mimetype", 0, 20, []byte("x"))}); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("replacement must also retain original: %v", err)
	}
	if b.IsModified("mimetype") {
		t.Fatal("rejected replacement mutated book")
	}
	b.maxRetainedBytes = 33
	for _, content := range []string{"abc", "a", "abc"} {
		current, _ := b.Current("mimetype")
		if err := b.Apply([]editset.Edit{editset.Replace("mimetype", 0, int64(len(current)), []byte(content))}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Apply([]editset.Edit{editset.Delete("mimetype")}); err != nil {
		t.Fatal(err)
	}
	if err := b.Apply([]editset.Edit{editset.Replace("new.txt", 0, 0, []byte("abc"))}); err != nil {
		t.Fatalf("deleting edited bytes must release their budget: %v", err)
	}
	if err := b.Apply([]editset.Edit{editset.Replace("more.txt", 0, 0, []byte("x"))}); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("new entry exceeded cumulative budget: %v", err)
	}
}

func TestCanceledReadsDoNotReturnCachedContent(t *testing.T) {
	b, _ := openSample(t)
	if _, err := b.Current("mimetype"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, read := range []func(context.Context, string) ([]byte, error){b.OriginalContext, b.CurrentContext} {
		if data, err := read(ctx, "mimetype"); !errors.Is(err, context.Canceled) || data != nil {
			t.Fatalf("canceled cached read = %q, %v", data, err)
		}
	}
}

func TestOriginalReadFailurePoisonsWrites(t *testing.T) {
	b, _ := openSample(t)
	b.maxRetainedBytes = 5
	if _, err := b.Original("missing.xhtml"); !errors.Is(err, ErrMissingEntry) {
		t.Fatalf("missing read = %v", err)
	}
	if err := b.ReadError(); err != nil {
		t.Fatalf("missing read poisoned book: %v", err)
	}

	if data, err := b.Original("OEBPS/content.opf"); !errors.Is(err, ErrMemoryLimit) || data != nil {
		t.Fatalf("over-budget read = %q, %v", data, err)
	}
	firstErr := b.ReadError()
	if !errors.Is(firstErr, ErrMemoryLimit) {
		t.Fatalf("ReadError = %v", firstErr)
	}

	b.maxRetainedBytes = MaxRetainedBytes
	if _, err := b.Original("OEBPS/content.opf"); err != nil {
		t.Fatalf("read after increasing budget: %v", err)
	}
	if b.ReadError() != firstErr {
		t.Fatalf("first read error was replaced: got %v, want %v", b.ReadError(), firstErr)
	}

	singleOutput := filepath.Join(t.TempDir(), "single.epub")
	if err := b.WriteTo(singleOutput); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("WriteTo = %v", err)
	}
	if _, err := os.Stat(singleOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteTo left output behind: stat error = %v", err)
	}
	contextOutput := filepath.Join(t.TempDir(), "context.epub")
	if err := b.WriteToContext(t.Context(), contextOutput); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("WriteToContext = %v", err)
	}
	if _, err := os.Stat(contextOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteToContext left output behind: stat error = %v", err)
	}

	outputDir := filepath.Join(t.TempDir(), "group")
	other, _ := openSample(t)
	if err := CommitGroup(t.Context(), outputDir, []GroupOutput{{Name: "one.epub", Book: other}, {Name: "two.epub", Book: b}}); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("CommitGroup = %v", err)
	}
	if _, err := os.Stat(outputDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CommitGroup left output directory behind: stat error = %v", err)
	}
}
