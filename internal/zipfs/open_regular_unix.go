//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package zipfs

import (
	"fmt"
	"os"
	"syscall"
)

// openRegularFile uses O_NONBLOCK so opening a FIFO cannot wait for a peer.
// The descriptor is then inspected with fstat, avoiding a stat/open race.
func openRegularFile(path string) (*os.File, os.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !stat.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil, fmt.Errorf("%w: %s", ErrNotRegularFile, path)
	}
	return f, stat, nil
}
