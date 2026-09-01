package zipfs

import (
	"errors"
	"fmt"
	"os"
)

// errRenameNoReplaceUnsupported is returned on platforms for which this
// package has no atomic no-replace rename primitive. Falling back to
// Lstat+Rename would reintroduce a TOCTOU window in which a concurrent
// creator can have its output replaced.
var errRenameNoReplaceUnsupported = errors.New("zipfs: atomic no-replace rename unsupported")

// renameNoReplace is the common wrapper for file and directory commits. The
// platform implementation must refuse an existing destination atomically.
// Both paths use the native operation: a link-then-remove file fallback would
// have a second cleanup step whose failure could leave a committed destination
// while reporting an error.
func renameNoReplace(src, dst string) error {
	return normalizeNoReplaceRenameError(renameNoReplacePlatform(src, dst), dst)
}

// renameDirNoReplace is kept as a distinct wrapper so the directory
// transaction makes its no-replace requirement explicit at the call site.
func renameDirNoReplace(src, dst string) error {
	return normalizeNoReplaceRenameError(renameNoReplacePlatform(src, dst), dst)
}

func normalizeNoReplaceRenameError(err error, dst string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrOutputExists, dst)
	}
	return err
}
