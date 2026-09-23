//go:build unix

package zipfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteToCreatesWorldReadableOutput(t *testing.T) {
	input := writeTempZip(t, buildInputZip(t))
	a, err := Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	plans := []Plan{{Name: "mimetype", Source: mustLookup(t, a, "mimetype")}}

	out := filepath.Join(t.TempDir(), "output.epub")
	if err := a.WriteTo(out, plans); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("output mode = %04o, want 0644", got)
	}

	parent := t.TempDir()
	outDir := filepath.Join(parent, "segments")
	if err := a.WriteDirectory(t.Context(), outDir, []DirectoryPlan{{Name: "one.epub", Plans: plans}}); err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o755 {
		t.Fatalf("output directory mode = %04o, want 0755", got)
	}
	fileInfo, err := os.Stat(filepath.Join(outDir, "one.epub"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("directory output mode = %04o, want 0644", got)
	}
}
