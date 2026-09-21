//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package zipfs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestFIFOInputsReturnWithoutWaitingForWriter(t *testing.T) {
	if path := os.Getenv("EPUB_TEST_FIFO_PATH"); path != "" {
		if _, err := OpenContext(t.Context(), path); !errors.Is(err, ErrNotRegularFile) {
			t.Fatalf("OpenContext FIFO: %v", err)
		}
		if _, err := ReadFileContext(t.Context(), path, 1024); !errors.Is(err, ErrNotRegularFile) {
			t.Fatalf("ReadFileContext FIFO: %v", err)
		}
		if _, err := FileSHA256Context(t.Context(), path); !errors.Is(err, ErrNotRegularFile) {
			t.Fatalf("FileSHA256Context FIFO: %v", err)
		}
		return
	}
	path := filepath.Join(t.TempDir(), "input.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	// Isolate the blocking regression so a broken opener cannot hang go test.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFIFOInputsReturnWithoutWaitingForWriter$")
	cmd.Env = append(os.Environ(), "EPUB_TEST_FIFO_PATH="+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("FIFO helper: %v (context: %v)\n%s", err, ctx.Err(), out)
	}
}
