// Package zipfs 是容器读写的唯一磁盘边界（SPEC §1 第 6 层）。
//
// 未被修改的 entry 通过 zip.Writer.Copy 原样透传（INV-1），
// 这是「49MB 的书只搬运改动部分」这一性能承诺的落实点。
// 除本包外，任何包不得 import archive/zip。
package zipfs

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	// ErrOutputExists 表示调用者指定的输出目标已经存在。
	// 写操作默认不覆盖任何既有文件或目录。
	ErrOutputExists = errors.New("zipfs: output already exists")
	// ErrInvalidOutputPath 表示输出路径为空或不是一个安全的路径。
	ErrInvalidOutputPath = errors.New("zipfs: invalid output path")
	// ErrInvalidOutputName 表示目录事务中的产物名不是安全 basename。
	ErrInvalidOutputName = errors.New("zipfs: invalid output name")
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

// DirectoryPlan 描述目录事务中的一个最终 EPUB 文件。
// Name 必须是安全 basename；Plans 内的 entry 名仍然是 EPUB 归档路径。
type DirectoryPlan struct {
	Name  string
	Plans []Plan
}

// WriteTo 把全部 Plan 写成一个新容器。写盘只发生在这里：
// 先在同目录创建不可预测的随机临时文件，再原子改名；既有输出和
// 传统的 <outPath>.tmp sidecar 都不会被覆盖。
func (a *Archive) WriteTo(outPath string, plans []Plan) error {
	return a.writeTo(nil, outPath, plans)
}

// WriteToContext 是 WriteTo 的可取消版本，供需要把 context 贯穿到大
// ZIP 写循环的调用方使用。WriteTo 保留原有 API 作为兼容入口。
func (a *Archive) WriteToContext(ctx context.Context, outPath string, plans []Plan) error {
	return a.writeTo(ctx, outPath, plans)
}

func (a *Archive) writeTo(ctx context.Context, outPath string, plans []Plan) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := validateOutputFilePath(outPath); err != nil {
		return err
	}
	seen := make(map[string]bool, len(plans))
	for i := range plans {
		p := &plans[i]
		if p.Deleted {
			continue
		}
		if err := ValidateEntryName(p.Name); err != nil {
			return fmt.Errorf("zipfs: plan %q: %w", p.Name, err)
		}
		if p.Source == nil && p.Content == nil {
			return fmt.Errorf("%w: %s", ErrInvalidPlan, p.Name)
		}
		if p.Source != nil && p.Content == nil && p.Source.Name() != p.Name {
			return fmt.Errorf("zipfs: passthrough source name %q does not match plan name %q", p.Source.Name(), p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("zipfs: duplicate output entry %q", p.Name)
		}
		seen[p.Name] = true
	}

	dir := filepath.Dir(outPath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("zipfs: create output parent %q: %w", dir, err)
		}
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(outPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("zipfs: create temporary output beside %q: %w", outPath, err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(tmp)
	for i := range plans {
		if err := contextErr(ctx); err != nil {
			return err
		}
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
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("zipfs: close writer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("zipfs: close output: %w", err)
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := renameNoReplace(tmpPath, outPath); err != nil {
		return fmt.Errorf("zipfs: rename output: %w", err)
	}
	committed = true
	return nil
}

// WriteDirectory 以目录为单位提交多个 EPUB。所有文件先写入 outputDir
// 的同目录随机 sibling staging 目录；全部成功后只执行一次
// staging→outputDir 的 rename。outputDir 必须在调用前不存在，失败时
// 自动清理 staging，不会留下半成品或可预测的 <out>.tmp。
func (a *Archive) WriteDirectory(ctx context.Context, outputDir string, outputs []DirectoryPlan) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	parent, err := prepareOutputDir(outputDir)
	if err != nil {
		return err
	}
	if len(outputs) == 0 {
		return fmt.Errorf("zipfs: output directory has no planned files")
	}
	seen := make(map[string]bool, len(outputs))
	for _, output := range outputs {
		if err := ValidateOutputName(output.Name); err != nil {
			return fmt.Errorf("zipfs: %q: %w", output.Name, err)
		}
		if seen[output.Name] {
			return fmt.Errorf("zipfs: duplicate output file %q", output.Name)
		}
		seen[output.Name] = true
	}

	base := filepath.Base(filepath.Clean(outputDir))
	stage, err := os.MkdirTemp(parent, "."+base+".staging-*")
	if err != nil {
		return fmt.Errorf("zipfs: create staging directory beside %q: %w", outputDir, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stage)
		}
	}()

	for _, output := range outputs {
		if err := contextErr(ctx); err != nil {
			return err
		}
		finalPath := filepath.Join(stage, output.Name)
		if err := a.writeTo(ctx, finalPath, output.Plans); err != nil {
			return fmt.Errorf("zipfs: write staged output %q: %w", output.Name, err)
		}
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if err := renameDirNoReplace(stage, outputDir); err != nil {
		return fmt.Errorf("zipfs: commit output directory: %w", err)
	}
	committed = true
	return nil
}

// WriteDir 是 WriteDirectory 的简短别名，供 book 层使用。
func (a *Archive) WriteDir(ctx context.Context, outputDir string, outputs []DirectoryPlan) error {
	return a.WriteDirectory(ctx, outputDir, outputs)
}

// ValidateOutputDirAbsent performs the read-only portion of the directory
// transaction preflight. It deliberately does not create parent directories;
// callers can use it for dry-run validation without any filesystem mutation.
func ValidateOutputDirAbsent(outputDir string) error {
	if err := validateOutputDirPath(outputDir); err != nil {
		return err
	}
	if _, err := os.Lstat(outputDir); err == nil {
		return fmt.Errorf("%w: %s", ErrOutputExists, outputDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("zipfs: inspect output directory %q: %w", outputDir, err)
	}
	return nil
}

// ValidateOutputName 校验目录事务中最终文件的名称必须是 basename。
func ValidateOutputName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) ||
		filepath.Base(name) != name || name == filepath.VolumeName(name) {
		return ErrInvalidOutputName
	}
	return nil
}

// ValidateEntryName 校验 ZIP entry 名不会是空路径、绝对路径或目录。
func ValidateEntryName(name string) error {
	if name == "" || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") {
		return ErrInvalidOutputPath
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ErrInvalidOutputPath
	}
	return nil
}

func validateOutputFilePath(path string) error {
	if path == "" || strings.ContainsRune(path, 0) {
		return ErrInvalidOutputPath
	}
	if info, err := os.Lstat(path); err == nil {
		_ = info
		return fmt.Errorf("%w: %s", ErrOutputExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("zipfs: inspect output %q: %w", path, err)
	}
	base := filepath.Base(path)
	if base == "" || base == "." || base == ".." {
		return ErrInvalidOutputPath
	}
	return nil
}

func prepareOutputDir(outputDir string) (string, error) {
	clean := filepath.Clean(outputDir)
	if err := ValidateOutputDirAbsent(outputDir); err != nil {
		return "", err
	}
	parent := filepath.Dir(clean)
	if parent == "" {
		parent = "."
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("zipfs: create output parent %q: %w", parent, err)
	}
	if _, err := os.Lstat(outputDir); err == nil {
		return "", fmt.Errorf("%w: %s", ErrOutputExists, outputDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("zipfs: inspect output directory %q: %w", outputDir, err)
	}
	return parent, nil
}

func validateOutputDirPath(outputDir string) error {
	if outputDir == "" || strings.ContainsRune(outputDir, 0) {
		return ErrInvalidOutputPath
	}
	clean := filepath.Clean(outputDir)
	base := filepath.Base(clean)
	if clean == "." || clean == string(filepath.Separator) || base == "." || base == ".." || base == "" {
		return ErrInvalidOutputPath
	}
	return nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
