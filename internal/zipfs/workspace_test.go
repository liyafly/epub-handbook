package zipfs

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPackTreeOrderFilterAndMimetype(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"mimetype": "application/epub+zip", "b.txt": "b", "A/z.txt": "z", "a.txt": "a",
		".hidden": "x", "dir/.secret/file": "x", "Thumbs.db": "x", "note.txt~": "x", "中文 名.xhtml": "c",
	})
	out := filepath.Join(t.TempDir(), "out.epub")
	if err := PackTree(context.Background(), root, out, "mimetype"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	want := []string{"mimetype", "A/z.txt", "a.txt", "b.txt", "中文 名.xhtml"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
	if zr.File[0].Method != zip.Store || len(zr.File[0].Extra) != 0 {
		t.Error("mimetype must be stored without extra fields")
	}
	if !bytes.Equal(raw[38:58], []byte("application/epub+zip")) {
		t.Error("mimetype payload not at the OCF offset")
	}
	if err := PackTree(context.Background(), root, out, "mimetype"); err == nil {
		t.Error("PackTree must not overwrite an existing output")
	}
}

func TestExtractToRejectsUnsafeEntries(t *testing.T) {
	for _, name := range []string{"../evil", "/abs", "a\\b", "ok/../../evil"} {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		fw, _ := w.Create(name)
		_, _ = fw.Write([]byte("x"))
		_ = w.Close()
		a, err := OpenBytesContext(context.Background(), "mem.zip", buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if err := a.ExtractTo(context.Background(), filepath.Join(dir, "out")); err == nil {
			t.Errorf("entry %q was not rejected", name)
		}
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for range 2 {
		fw, _ := w.Create("same.txt")
		_, _ = fw.Write([]byte("x"))
	}
	_ = w.Close()
	a, err := OpenBytesContext(context.Background(), "dup.zip", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ExtractTo(context.Background(), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Error("duplicate entries must be rejected")
	}
}

func TestReplaceFileAndExclusiveMkdir(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.txt")
	if err := WriteFileAtomic(dst, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(dst, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "two" {
		t.Errorf("content = %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
	lock := filepath.Join(dir, "lock")
	if err := MkdirExclusive(lock); err != nil {
		t.Fatal(err)
	}
	if err := MkdirExclusive(lock); err == nil {
		t.Error("second exclusive mkdir must fail")
	}
}
