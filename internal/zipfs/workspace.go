package zipfs

// 书级工作区所需的通用磁盘原语。
//
// INV-3 规定 internal/zipfs 与 internal/extern 是仅有的磁盘写边界。书级工作流
// （epub book ...）需要建目录、复制源树、原子替换交付件、写收据与建锁，这些
// 操作不理解 EPUB 语义，统一收口在本文件，由 internal/pipeline 调用；
// pipeline 自身不直接调用 os 的写 API。所有路径一律走 filepath，替换在
// Windows 上对“目标正被占用”做有限次重试。

import (
	"archive/zip"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// MkdirAll creates a directory tree (idempotent).
func MkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("zipfs: create directory %q: %w", dir, err)
	}
	return nil
}

// MkdirExclusive creates a single directory and fails with ErrOutputExists
// when it is already present. Directory creation is atomic on every platform,
// so it doubles as a cross-process lock (the build lock uses it).
func MkdirExclusive(dir string) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrOutputExists, dir)
		}
		return fmt.Errorf("zipfs: create directory %q: %w", dir, err)
	}
	return nil
}

// MkdirTemp creates a uniquely named directory inside parent.
func MkdirTemp(parent, pattern string) (string, error) {
	dir, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		return "", fmt.Errorf("zipfs: create temporary directory in %q: %w", parent, err)
	}
	return dir, nil
}

// Remove deletes one file or one empty directory; a missing path is not an error.
func Remove(name string) error {
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("zipfs: remove %q: %w", name, err)
	}
	return nil
}

// RemoveAll deletes a tree; a missing path is not an error.
func RemoveAll(name string) error {
	if err := os.RemoveAll(name); err != nil {
		return fmt.Errorf("zipfs: remove %q: %w", name, err)
	}
	return nil
}

// RenameNoReplace atomically renames a file or directory and refuses an
// existing destination (ErrOutputExists).
func RenameNoReplace(src, dst string) error {
	return renameNoReplace(src, dst)
}

// ReplaceFile renames src over dst, replacing an existing regular file. On
// Windows a destination held open by another process (reader, antivirus,
// Explorer preview) fails transiently, so the rename is retried briefly before
// the error is reported; src and the old dst are left untouched on failure.
func ReplaceFile(src, dst string) error {
	attempts := 1
	if runtime.GOOS == "windows" {
		attempts = 20
	}
	var err error
	for i := range attempts {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		if i+1 < attempts {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("zipfs: replace %q: %w", dst, err)
}

// WriteFileAtomic writes data beside path and renames it over path, so a
// reader never sees a partial file and an existing file is replaced atomically.
func WriteFileAtomic(filePath string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(filePath)
	if err := MkdirAll(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(filePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("zipfs: create temporary file beside %q: %w", filePath, err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("zipfs: write %q: %w", filePath, err)
	}
	if err := tmp.Chmod(mode); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("zipfs: set permissions on %q: %w", filePath, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("zipfs: sync %q: %w", filePath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("zipfs: close %q: %w", filePath, err)
	}
	if err := ReplaceFile(tmpPath, filePath); err != nil {
		return err
	}
	committed = true
	return nil
}

// CopyFile copies one regular file to a new path (the destination must not
// exist). The source must not be a symlink.
func CopyFile(ctx context.Context, src, dst string, mode os.FileMode) error {
	in, _, err := openRegularFile(src)
	if err != nil {
		return fmt.Errorf("zipfs: open %q: %w", src, err)
	}
	defer in.Close()
	if err := MkdirAll(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("zipfs: create %q: %w", dst, err)
	}
	if _, err := io.Copy(contextWriter{ctx: ctx, writer: out}, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("zipfs: copy %q: %w", src, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("zipfs: close %q: %w", dst, err)
	}
	return nil
}

// CopyTree copies src (a directory) into dst recursively. keep decides for
// every entry directly below src (top == true) and deeper (top == false)
// whether it is copied; pass nil to copy everything. Symlinks and special
// files are rejected rather than followed. Files are written 0644, directories
// 0755, mirroring the normalised permissions of a reproducible package.
func CopyTree(ctx context.Context, src, dst string, keep func(rel string, top bool) bool) error {
	if err := MkdirAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := contextErr(ctx); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		top := !strings.ContainsRune(rel, filepath.Separator)
		if keep != nil && !keep(filepath.ToSlash(rel), top) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("zipfs: refusing to copy symlink %q", p)
		case d.IsDir():
			return MkdirAll(target)
		case d.Type().IsRegular():
			return CopyFile(ctx, p, target, 0o644)
		default:
			return fmt.Errorf("zipfs: refusing to copy special file %q", p)
		}
	})
}

// FindSymlink returns the first symlink below root (including root itself) or
// "" when the tree has none.
func FindSymlink(root string) (string, error) {
	found := ""
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if info, lerr := os.Lstat(root); lerr == nil && info.Mode()&fs.ModeSymlink != 0 {
		return root, nil
	}
	return found, err
}

// ExtractTo unpacks every entry into dir after rejecting absolute names, "..",
// backslashes, drive/stream colons on Windows, and symlink entries. Duplicate
// entry names are refused so an archive cannot overwrite itself.
func (a *Archive) ExtractTo(ctx context.Context, dir string) error {
	seen := make(map[string]bool, len(a.order))
	for _, name := range a.order {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("zipfs: duplicate ZIP entry %q", name)
		}
		seen[name] = true
		entry := a.byName[name]
		clean := path.Clean(name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) ||
			clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) ||
			(runtime.GOOS == "windows" && strings.Contains(name, ":")) ||
			entry.zf.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("zipfs: unsafe ZIP entry %q", name)
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		if strings.HasSuffix(name, "/") {
			if err := MkdirAll(target); err != nil {
				return err
			}
			continue
		}
		if err := MkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		rc, err := entry.zf.Open()
		if err != nil {
			return fmt.Errorf("zipfs: open %q: %w", name, err)
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("zipfs: create %q: %w", target, err)
		}
		err = copyBoundedContext(ctx, out, rc, a.limits.MaxEntryBytes)
		_ = rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("zipfs: extract %q: %w", name, err)
		}
	}
	return nil
}

// PackTree writes the regular files of dir into a new ZIP at outPath
// (outPath must not exist). If storedFirst names a file present in the tree
// it is written first, uncompressed and without extra fields (the OCF
// "mimetype" rule); every other file follows in byte-wise path order with the
// deterministic 1980-01-01 timestamp, mode 0644 and maximum deflate. Hidden
// files and directories (names starting with "."), Thumbs.db and backup files
// ending in "~" are left out; no directory entries are written.
func PackTree(ctx context.Context, dir, outPath, storedFirst string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := validateOutputFilePath(outPath); err != nil {
		return err
	}
	var names []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if !d.Type().IsRegular() || slash == storedFirst || packExcluded(slash) {
			return nil
		}
		names = append(names, slash)
		return nil
	})
	if err != nil {
		return fmt.Errorf("zipfs: scan %q: %w", dir, err)
	}
	slices.Sort(names)
	if _, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(storedFirst))); storedFirst != "" && statErr == nil {
		names = slices.Insert(names, 0, storedFirst)
	}

	if err := MkdirAll(filepath.Dir(outPath)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outPath), "."+filepath.Base(outPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("zipfs: create temporary output beside %q: %w", outPath, err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	w := zip.NewWriter(contextWriter{ctx: ctx, writer: tmp})
	w.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	for _, name := range names {
		if err := contextErr(ctx); err != nil {
			return err
		}
		// 1980-01-01 00:00 以 DOS 日期字段写入且不带扩展时间戳 extra 字段，
		// 与 `zip -X` 的输出同构；mimetype 不得带任何 extra 字段。
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, ModifiedDate: 1<<5 | 1}
		h.SetMode(0o644)
		if name == storedFirst {
			h.Method = zip.Store
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			return fmt.Errorf("zipfs: create %q: %w", name, err)
		}
		in, _, err := openRegularFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("zipfs: open %q: %w", name, err)
		}
		_, err = io.Copy(fw, in)
		_ = in.Close()
		if err != nil {
			return fmt.Errorf("zipfs: pack %q: %w", name, err)
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("zipfs: close writer: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("zipfs: sync output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("zipfs: close output: %w", err)
	}
	if err := renameNoReplace(tmpPath, outPath); err != nil {
		return fmt.Errorf("zipfs: rename output: %w", err)
	}
	committed = true
	return nil
}

// packExcluded mirrors the historical packaging filter: hidden basenames,
// hidden directory components, Thumbs.db and editor backup files.
func packExcluded(slash string) bool {
	parts := strings.Split(slash, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	base := parts[len(parts)-1]
	return base == "Thumbs.db" || strings.HasSuffix(base, "~")
}

// Chmod changes the permission bits of a path (a no-op for most bits on Windows).
func Chmod(name string, mode os.FileMode) error {
	if err := os.Chmod(name, mode); err != nil {
		return fmt.Errorf("zipfs: chmod %q: %w", name, err)
	}
	return nil
}
