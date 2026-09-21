package pipeline

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

func corruptFixtureEntry(t *testing.T, name string) string {
	t.Helper()
	data := epubFixtureBytes(t)
	// The final occurrence is the central directory filename, preceded by
	// its 46-byte header. Corrupt only its CRC; keep container parsing valid.
	header := bytes.LastIndex(data, []byte(name)) - 46
	if header < 0 || !bytes.Equal(data[header:header+4], []byte("PK\x01\x02")) {
		t.Fatal("fixture central directory entry not found")
	}
	data[header+16] ^= 1
	path := filepath.Join(t.TempDir(), "corrupt.epub")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIgnoredInputReadErrorCannotBecomeSuccessfulStage(t *testing.T) {
	for _, mode := range []string{"readonly", "write", "preview"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			id := "test.read-failure"
			writeTestContract(t, root, id, nil, mode != "readonly", nil)
			installTestRunner(t, id, func(_ context.Context, b *book.Book, _ Args, _ Upstream) (report.Result, error) {
				if _, err := b.Current("OEBPS/c1.xhtml"); err == nil {
					t.Fatal("fixture must fail checksum verification")
				}
				return report.Result{Capability: id, Status: report.StatusComplete, Facts: map[string]any{"partial": true}}, nil
			})
			out := filepath.Join(t.TempDir(), "out.epub")
			result, err := Run(t.Context(), Options{RepoRoot: root, CapabilityID: id,
				InputPath: corruptFixtureEntry(t, "OEBPS/c1.xhtml"), OutputPath: out, DryRun: mode == "preview"})
			if err != nil || result.ExitCode != ExitFailed || result.Envelope.Status != report.StatusFailed {
				t.Fatalf("ignored read failure: %+v, %v", result, err)
			}
			if _, ok := result.Envelope.Facts[id+".partial"]; ok {
				t.Fatal("partial facts escaped the failed stage")
			}
			if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed read produced an output: %v", err)
			}
		})
	}
}

func TestRedlineReadFailureDoesNotWriteCandidate(t *testing.T) {
	root := t.TempDir()
	id := "test.redline-read-failure"
	writeTestContract(t, root, id, nil, true, []string{"text"})
	installTestRunner(t, id, func(context.Context, *book.Book, Args, Upstream) (report.Result, error) {
		return report.Result{Capability: id, Status: report.StatusComplete}, nil
	})
	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{RepoRoot: root, CapabilityID: id,
		InputPath: corruptFixtureEntry(t, "OEBPS/c1.xhtml"), OutputPath: out})
	if err != nil || result.ExitCode != ExitFailed || result.Envelope.Status != report.StatusFailed {
		t.Fatalf("redline read failure: %+v, %v", result, err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("redline read failure produced an output: %v", err)
	}
}
