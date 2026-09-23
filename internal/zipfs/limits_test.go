package zipfs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWithLimitsRejectsNonPositiveLimits(t *testing.T) {
	path := writeTempZip(t, buildInputZip(t))
	base := DefaultLimits()
	tests := []struct {
		name   string
		change func(*Limits)
	}{
		{name: "archive bytes", change: func(l *Limits) { l.MaxArchiveBytes = 0 }},
		{name: "entries", change: func(l *Limits) { l.MaxEntries = 0 }},
		{name: "entry bytes", change: func(l *Limits) { l.MaxEntryBytes = 0 }},
		{name: "total bytes", change: func(l *Limits) { l.MaxTotalBytes = 0 }},
		{name: "path bytes", change: func(l *Limits) { l.MaxPathBytes = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := base
			test.change(&limits)
			if _, err := OpenWithLimits(t.Context(), path, limits); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("OpenWithLimits error = %v, want ErrInvalidLimits", err)
			}
		})
	}
}

func TestOpenWithLimitsEnforcesArchiveAndEntryMetadata(t *testing.T) {
	data := buildInputZip(t)
	path := writeTempZip(t, data)
	base := DefaultLimits()
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*Limits)
	}{
		{name: "archive bytes", change: func(l *Limits) { l.MaxArchiveBytes = stat.Size() - 1 }},
		{name: "entry count", change: func(l *Limits) { l.MaxEntries = 4 }},
		{name: "single entry size", change: func(l *Limits) { l.MaxEntryBytes = 1 }},
		{name: "total uncompressed size", change: func(l *Limits) { l.MaxTotalBytes = 1 }},
		{name: "path bytes", change: func(l *Limits) { l.MaxPathBytes = len("a/chapter.xhtml") - 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := base
			test.change(&limits)
			if _, err := OpenWithLimits(t.Context(), path, limits); !errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("OpenWithLimits error = %v, want ErrLimitExceeded", err)
			}
		})
	}
}

func TestOpenRejectsDirectoryAndUnreadableFile(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotRegularFile) {
		t.Fatalf("Open(directory) error = %v, want ErrNotRegularFile", err)
	}

	path := writeTempZip(t, buildInputZip(t))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	_ = os.Chmod(path, 0o600)
	if err == nil {
		t.Skip("current process can read a mode-000 file")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Open(mode-000 file) error = %v, want permission error", err)
	}
}

func TestOpenWithLimitsHonorsPreCanceledContext(t *testing.T) {
	path := writeTempZip(t, buildInputZip(t))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := OpenWithLimits(ctx, path, DefaultLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenWithLimits error = %v, want context.Canceled", err)
	}
}

func TestReadContextHonorsLimitsAndCancellation(t *testing.T) {
	path := writeTempZip(t, buildInputZip(t))
	limits := DefaultLimits()
	a, err := OpenWithLimits(t.Context(), path, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.limits.MaxEntryBytes = 1
	if _, err := a.ReadContext(t.Context(), "a/chapter.xhtml"); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ReadContext error = %v, want ErrLimitExceeded", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := a.ReadContext(ctx, "mimetype"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadContext canceled error = %v, want context.Canceled", err)
	}
}

func TestReadContextChecksActualBytesWhenHeaderUnderreports(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "payload.txt", Method: zip.Deflate}
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(strings.Repeat("x", 128))); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	central := bytes.LastIndex(data, []byte("PK\x01\x02"))
	if central < 0 {
		t.Fatal("central directory header not found")
	}
	// ZIP central-directory uncompressed-size field is at byte offset 24.
	binary.LittleEndian.PutUint32(data[central+24:central+28], 1)
	path := writeTempZip(t, data)
	limits := DefaultLimits()
	limits.MaxEntryBytes = 16
	limits.MaxTotalBytes = 16
	a, err := OpenWithLimits(t.Context(), path, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	entry, ok := a.Lookup("payload.txt")
	if !ok || entry.Size() != 1 {
		t.Fatalf("forged entry metadata not retained: entry=%v ok=%t", entry, ok)
	}
	if data, err := a.ReadContext(t.Context(), "payload.txt"); err == nil || data != nil {
		t.Fatalf("forged entry returned data=%v err=%v", data, err)
	}
}

func TestReadFileContextBoundsRegularFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.json")
	want := []byte(`{"cover":true}`)
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFileContext(t.Context(), path, int64(len(want)))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ReadFileContext exact fit = %q, %v; want %q", got, err, want)
	}
	if _, err := ReadFileContext(t.Context(), path, int64(len(want)-1)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ReadFileContext error = %v, want ErrLimitExceeded", err)
	}
	if _, err := ReadFileContext(t.Context(), t.TempDir(), 100); !errors.Is(err, ErrNotRegularFile) {
		t.Fatalf("ReadFileContext(directory) error = %v, want ErrNotRegularFile", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ReadFileContext(ctx, path, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadFileContext canceled error = %v, want context.Canceled", err)
	}
}

func TestBoundedEmptyReadIsContentNotDeletion(t *testing.T) {
	data, err := readBoundedContext(t.Context(), strings.NewReader(""), 1)
	if err != nil || data == nil || len(data) != 0 {
		t.Fatalf("empty content must remain non-nil: %#v, %v", data, err)
	}
}

func TestFileSHA256ContextHashesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.epub")
	wantData := []byte("bounded file hash input\n")
	if err := os.WriteFile(path, wantData, 0o600); err != nil {
		t.Fatal(err)
	}
	wantSum := sha256.Sum256(wantData)
	want := hex.EncodeToString(wantSum[:])
	got, err := FileSHA256Context(t.Context(), path)
	if err != nil {
		t.Fatalf("FileSHA256Context() error = %v", err)
	}
	if got != want {
		t.Fatalf("FileSHA256Context() = %q, want %q", got, want)
	}
}

func TestArchiveSHA256MatchesFileSHA256(t *testing.T) {
	path := writeTempZip(t, buildInputZip(t))
	want, err := FileSHA256Context(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	got, err := a.SHA256Context(t.Context())
	if err != nil {
		t.Fatalf("Archive.SHA256Context: %v", err)
	}
	if got != want {
		t.Fatalf("Archive.SHA256Context() = %q, want %q", got, want)
	}
}

func TestFileSHA256ContextPreCanceledReturnsNoHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.epub")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := FileSHA256Context(ctx, path)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FileSHA256Context() error = %v, want context.Canceled", err)
	}
	if got != "" {
		t.Fatalf("FileSHA256Context() returned hash %q after cancellation", got)
	}
}

func TestFileSHA256ContextRejectsOversizedSparseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.epub")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(DefaultLimits().MaxArchiveBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := FileSHA256Context(t.Context(), path)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("FileSHA256Context() error = %v, want ErrLimitExceeded (hash %q)", err, got)
	}
	if got != "" {
		t.Fatalf("FileSHA256Context() returned hash %q for oversized file", got)
	}
}

type cancelOnRead struct {
	cancel context.CancelFunc
	read   bool
}

func (r *cancelOnRead) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	n := copy(p, []byte("first chunk"))
	r.cancel()
	return n, nil
}

func TestReadBoundedContextStopsOnDeterministicMidstreamCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	_, err := readBoundedContext(ctx, &cancelOnRead{cancel: cancel}, 1024)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("readBoundedContext error = %v, want context.Canceled", err)
	}
}

func TestContextWriterStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	w := contextWriter{ctx: ctx, writer: cancelingWriter{cancel: cancel}}
	if _, err := w.Write([]byte("chunk")); !errors.Is(err, context.Canceled) {
		t.Fatalf("contextWriter error = %v, want context.Canceled", err)
	}
}

type cancelingWriter struct{ cancel context.CancelFunc }

func (w cancelingWriter) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}
