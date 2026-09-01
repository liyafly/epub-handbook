//go:build linux

package zipfs

import "golang.org/x/sys/unix"

// Linux exposes an atomic no-replace rename through renameat2. It applies to
// both regular files and directories and never falls back to os.Rename when
// the kernel or filesystem does not support the requested operation.
func renameNoReplacePlatform(src, dst string) error {
	return unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
}
