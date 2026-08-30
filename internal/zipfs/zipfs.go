// Package zipfs 是容器读写的唯一磁盘边界（SPEC §1 第 6 层）。
//
// 未被修改的 entry 通过 zip.Writer.Copy 原样透传（INV-1），
// 这是「49MB 的书只搬运改动部分」这一性能承诺的落实点。
// 除本包外，任何包不得 import archive/zip。
package zipfs

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Method 决定重写 entry 的压缩方式。零值是 Deflate，
// 这样「不显式指定」就是最常见的正确选择。
type Method uint8

const (
	MethodDeflate Method = iota
	MethodStore
)

var (
	// ErrMissing 表示请求的 entry 在容器中不存在。
	ErrMissing = errors.New("zipfs: entry not found")
	// ErrInvalidPlan 表示写出的 Plan 既不透传也没有内容。
	ErrInvalidPlan = errors.New("zipfs: plan has neither source nor content")
)

// fixedTime 与 Python oracle（scripts/epub_lib.py FIXED_ZIP_TIME）给重写
// entry 打的固定时间戳一致，保证 parity 产物可比较。
func fixedTime() time.Time {
	return time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
}

// Entry 是输入容器里的一个成员。它是 archive/zip 在本包外的唯一替身：
// 上层只看得到名字、大小与哈希，拿不到 zip 类型。
type Entry struct {
	name string
	zf   *zip.File
}

func (e *Entry) Name() string          { return e.name }
func (e *Entry) Size() int64           { return int64(e.zf.UncompressedSize64) }
func (e *Entry) CompressedSize() int64 { return int64(e.zf.CompressedSize64) }
func (e *Entry) CRC32() uint32         { return e.zf.CRC32 }
func (e *Entry) MethodCode() uint16    { return e.zf.Method }
func (e *Entry) Modified() time.Time   { return e.zf.Modified }

// Archive 是打开的输入容器。读是惰性的：只有 Read 被调用才解压对应 entry。
type Archive struct {
	path   string
	f      *os.File
	order  []string
	byName map[string]*Entry
}

// Open 打开一个磁盘上的 zip 容器（通常是 EPUB）。调用方负责 Close。
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	r, err := zip.NewReader(f, stat.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("zipfs: %s: not a zip container: %w", path, err)
	}
	a := &Archive{
		path:   path,
		f:      f,
		order:  make([]string, 0, len(r.File)),
		byName: make(map[string]*Entry, len(r.File)),
	}
	for _, zf := range r.File {
		name := zf.Name
		a.order = append(a.order, name)
		// 与 Python zipfile.NameToInfo 一致：重名时后者覆盖。
		a.byName[name] = &Entry{name: name, zf: zf}
	}
	return a, nil
}

func (a *Archive) Path() string { return a.path }

func (a *Archive) Close() error { return a.f.Close() }

// Names 返回全部 entry 名，保持容器内的物理顺序。
func (a *Archive) Names() []string {
	return slices.Clip(slices.Clone(a.order))
}

// Lookup 返回输入容器里的 entry；不存在时第二个返回值为 false。
func (a *Archive) Lookup(name string) (*Entry, bool) {
	e, ok := a.byName[name]
	return e, ok
}

// Read 解压并返回 entry 的完整内容。
func (a *Archive) Read(name string) ([]byte, error) {
	e, ok := a.byName[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissing, name)
	}
	rc, err := e.zf.Open()
	if err != nil {
		return nil, fmt.Errorf("zipfs: open %q: %w", name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("zipfs: read %q: %w", name, err)
	}
	return data, nil
}

// Plan 描述一个 entry 在输出容器中的处置：
//   - Deleted:           不写出；
//   - Content != nil:    写出 Content（被修改或新增）；
//   - 其余:              用 zip.Writer.Copy 原样透传 Source（INV-1）。
type Plan struct {
	Name    string
	Source  *Entry
	Content []byte
	Method  Method
	Deleted bool
}

// WriteTo 把全部 Plan 写成一个新容器。写盘只发生在这里：
// 先写 <outPath>.tmp 再原子改名，中间态永不暴露。
func (a *Archive) WriteTo(outPath string, plans []Plan) error {
	seen := make(map[string]bool, len(plans))
	for i := range plans {
		p := &plans[i]
		if p.Deleted {
			continue
		}
		if p.Source == nil && p.Content == nil {
			return fmt.Errorf("%w: %s", ErrInvalidPlan, p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("zipfs: duplicate output entry %q", p.Name)
		}
		seen[p.Name] = true
	}

	if dir := filepath.Dir(outPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmpPath := outPath + ".tmp"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	written := false
	defer func() {
		if !written {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(tmp)
	for i := range plans {
		p := &plans[i]
		if p.Deleted {
			continue
		}
		if p.Content == nil && p.Source != nil {
			if err := w.Copy(p.Source.zf); err != nil {
				return fmt.Errorf("zipfs: passthrough %q: %w", p.Name, err)
			}
			continue
		}
		h := &zip.FileHeader{Name: p.Name, Modified: fixedTime()}
		if p.Method == MethodStore {
			h.Method = zip.Store
		} else {
			h.Method = zip.Deflate
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			return fmt.Errorf("zipfs: create %q: %w", p.Name, err)
		}
		if _, err := fw.Write(p.Content); err != nil {
			return fmt.Errorf("zipfs: write %q: %w", p.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("zipfs: close writer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("zipfs: close output: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return fmt.Errorf("zipfs: rename output: %w", err)
	}
	written = true
	return nil
}
