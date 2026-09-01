//go:build !linux && !darwin && !windows

package zipfs

import "fmt"

// Unsupported targets fail closed. In particular, do not use Lstat followed
// by os.Rename here: that sequence can overwrite a destination created by a
// concurrent process.
func renameNoReplacePlatform(src, dst string) error {
	return fmt.Errorf("%w: %q -> %q", errRenameNoReplaceUnsupported, src, dst)
}
