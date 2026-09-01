//go:build darwin

package zipfs

import "golang.org/x/sys/unix"

// macOS's renameatx_np with RENAME_EXCL provides an atomic exclusive rename
// for files and directories. There is deliberately no os.Rename fallback.
func renameNoReplacePlatform(src, dst string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_EXCL)
}
