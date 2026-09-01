//go:build windows

package zipfs

import "golang.org/x/sys/windows"

// MoveFileEx without MOVEFILE_REPLACE_EXISTING performs a same-volume move
// that fails when the destination already exists. The staging and output
// paths are siblings, so a cross-volume copy fallback is neither needed nor
// safe for this transaction.
func renameNoReplacePlatform(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, 0)
}
