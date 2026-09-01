// Package book 是内存中的 EPUB 模型（SPEC §1 第 5 层）。
//
// 它持有 entry 表、惰性内容与脏标记；Apply 是唯一的写入口，
// WriteTo 是唯一触发落盘的出口（INV-3），实际 I/O 委托给 zipfs。
package book

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

var (
	// ErrMissingEntry 表示请求的 entry 不存在（或已被删除）。
	ErrMissingEntry = errors.New("book: no such entry")
	// ErrDuplicateEntry 表示容器里出现了重复的 entry 名。
	ErrDuplicateEntry = errors.New("book: duplicate entry")
	// ErrInvalidPath 表示 entry 路径会逃逸容器根或为空。
	ErrInvalidPath = errors.New("book: invalid archive path")
)

// Open 打开磁盘上的 EPUB 并建立 entry 表。
// 目录项与 macOS 元数据文件（.DS_Store）从一开始就被排除，
// 与 scripts/epub_lib.py read_epub_files 的行为一致。
func Open(path string) (*Book, error) {
	arch, err := zipfs.Open(path)
	if err != nil {
		return nil, err
	}
	b := &Book{
		arch:      arch,
		byName:    make(map[string]*zipfs.Entry),
		cur:       make(map[string][]byte),
		deleted:   make(map[string]bool),
		origCache: make(map[string][]byte),
	}
	seen := make(map[string]bool, len(arch.Names()))
	for _, name := range arch.Names() {
		if err := ValidatePath(name); err != nil {
			arch.Close()
			return nil, fmt.Errorf("book: %s: %w", name, err)
		}
		if strings.HasSuffix(name, "/") || isMacOSMetadataPath(name) {
			continue
		}
		if seen[name] {
			arch.Close()
			return nil, fmt.Errorf("book: %w: %s", ErrDuplicateEntry, name)
		}
		seen[name] = true
		e, _ := arch.Lookup(name)
		b.byName[name] = e
		b.order = append(b.order, name)
	}
	return b, nil
}

// Book 是一个打开的 EPUB：输入容器 + 待应用的修改集。
type Book struct {
	arch      *zipfs.Archive
	order     []string
	byName    map[string]*zipfs.Entry
	cur       map[string][]byte
	added     []string
	deleted   map[string]bool
	origCache map[string][]byte
}

// Close 释放底层容器句柄。
func (b *Book) Close() error { return b.arch.Close() }

// InputPath 返回输入文件路径。
func (b *Book) InputPath() string { return b.arch.Path() }

// OriginalNames 返回输入容器中的 entry 名（保持物理顺序，不含目录与 .DS_Store）。
func (b *Book) OriginalNames() []string {
	return slices.Clone(b.order)
}

// Names 返回当前输出投影：原有 entry（去掉已删除）+ 新增 entry。
func (b *Book) Names() []string {
	out := make([]string, 0, len(b.order)+len(b.added))
	for _, name := range b.order {
		if !b.deleted[name] {
			out = append(out, name)
		}
	}
	for _, name := range b.added {
		if !b.deleted[name] {
			out = append(out, name)
		}
	}
	return out
}

// Has 判断 entry 在输出投影中是否存在。
func (b *Book) Has(name string) bool {
	if b.deleted[name] {
		return false
	}
	if _, ok := b.cur[name]; ok {
		return true
	}
	_, ok := b.byName[name]
	return ok
}

// IsModified 判断 entry 相对输入是否被改动（含新增与删除）。
func (b *Book) IsModified(name string) bool {
	if b.deleted[name] {
		return true
	}
	_, ok := b.cur[name]
	return ok
}

// ModifiedNames 返回相对输入发生变化的 entry 名（按 Names 顺序）。
func (b *Book) ModifiedNames() []string {
	var out []string
	for _, name := range b.Names() {
		if b.IsModified(name) {
			out = append(out, name)
		}
	}
	return out
}

// Original 返回 entry 在输入容器中的原始字节（惰性读取并缓存）。
func (b *Book) Original(name string) ([]byte, error) {
	if data, ok := b.origCache[name]; ok {
		return data, nil
	}
	e, ok := b.byName[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingEntry, name)
	}
	data, err := b.arch.Read(e.Name())
	if err != nil {
		return nil, err
	}
	b.origCache[name] = data
	return data, nil
}

// Current 返回 entry 的当前字节：有未应用的修改返回修改后内容，否则返回原始内容。
func (b *Book) Current(name string) ([]byte, error) {
	if data, ok := b.cur[name]; ok {
		return data, nil
	}
	return b.Original(name)
}

// Apply 应用一批编辑，是本包唯一的写入口。
//
// 语义：
//   - Replacement != nil：字节区间替换；entry 不存在且 Offset==0、Length==0
//     时表示新建该 entry。
//   - Replacement == nil：删除整个 entry。
func (b *Book) Apply(edits []editset.Edit) error {
	// 按 path 分组，保持首次出现的顺序，结果与分组方式无关但报错更聚焦。
	groups := make(map[string][]editset.Edit)
	var order []string
	for _, e := range edits {
		if _, ok := groups[e.Path]; !ok {
			order = append(order, e.Path)
		}
		groups[e.Path] = append(groups[e.Path], e)
	}
	for _, path := range order {
		group := groups[path]
		if group[0].Replacement == nil {
			// 删除型：同 path 不允许混入内容编辑。
			for _, e := range group {
				if e.Replacement != nil {
					return fmt.Errorf("book: %s: mixing entry deletion with content edits", path)
				}
			}
			if err := b.deleteEntry(path); err != nil {
				return err
			}
			continue
		}
		if !b.Has(path) {
			if len(group) != 1 || group[0].Offset != 0 || group[0].Length != 0 {
				return fmt.Errorf("%w: %s", ErrMissingEntry, path)
			}
			if err := ValidatePath(path); err != nil {
				return fmt.Errorf("book: %s: %w", path, err)
			}
			if strings.HasSuffix(path, "/") {
				return fmt.Errorf("book: %s: entry name must not end with '/'", path)
			}
			delete(b.deleted, path) // 删除后按原名重建：恢复可见性，位置回到原序。
			b.cur[path] = slices.Clone(group[0].Replacement)
			if _, isOriginal := b.byName[path]; !isOriginal && !slices.Contains(b.added, path) {
				b.added = append(b.added, path)
			}
			continue
		}
		content, err := b.Current(path)
		if err != nil {
			return err
		}
		updated, err := editset.Apply(path, content, group)
		if err != nil {
			return err
		}
		b.cur[path] = updated
	}
	return nil
}

func (b *Book) deleteEntry(path string) error {
	if !b.Has(path) {
		return fmt.Errorf("%w: %s", ErrMissingEntry, path)
	}
	b.deleted[path] = true
	delete(b.cur, path)
	if i := slices.Index(b.added, path); i >= 0 {
		b.added = slices.Delete(b.added, i, i+1)
	}
	return nil
}

// WriteTo 把当前状态落盘为一个新的 EPUB。这是整本书唯一的落盘点。
// 未修改的 entry 由 zipfs 原样透传（INV-1）。
func (b *Book) WriteTo(path string) error {
	return b.arch.WriteTo(path, b.plans())
}

// WriteToContext 是 WriteTo 的可取消版本。
func (b *Book) WriteToContext(ctx context.Context, path string) error {
	return b.arch.WriteToContext(ctx, path, b.plans())
}

func (b *Book) plans() []zipfs.Plan {
	plans := make([]zipfs.Plan, 0, len(b.order)+len(b.added))
	// mimetype 永远第一个（OCF 要求）；未被修改时透传。
	if e, ok := b.byName["mimetype"]; ok && !b.deleted["mimetype"] {
		p := zipfs.Plan{Name: "mimetype", Source: e}
		if c, ok := b.cur["mimetype"]; ok {
			p.Content = c
			p.Method = zipfs.MethodStore
		}
		plans = append(plans, p)
	}
	for _, name := range b.order {
		if name == "mimetype" || b.deleted[name] {
			continue
		}
		p := zipfs.Plan{Name: name, Source: b.byName[name]}
		if c, ok := b.cur[name]; ok {
			p.Content = c
		}
		plans = append(plans, p)
	}
	// 新增 entry 按字母序追加，与 scripts/epub_lib.py write_epub 的补写顺序一致。
	for _, name := range slices.Sorted(slices.Values(b.added)) {
		p := zipfs.Plan{Name: name, Content: b.cur[name]}
		if name == "mimetype" {
			p.Method = zipfs.MethodStore
		}
		plans = append(plans, p)
	}
	return plans
}

// GroupOutput 是目录事务中的一个 EPUB 产物。Name 必须是安全 basename；
// Book 只提供内存投影，实际写盘由 zipfs 在同目录 staging 中完成。
type GroupOutput struct {
	Name string
	Book *Book
}

// ValidateOutputDirAbsent is the read-only preflight used by multi-output
// capabilities. Keeping the filesystem check behind book preserves the
// package boundary: capabilities never touch zipfs directly.
func ValidateOutputDirAbsent(path string) error {
	return zipfs.ValidateOutputDirAbsent(path)
}

// CommitGroup 将多个打开的 Book 作为一个目录事务提交。任何产物写入、
// 校验或 rename 失败都会由 zipfs 清理 staging，outputDir 不会留下半成品。
// 调用方仍负责关闭传入的 Book；split 在成功与失败路径都会统一关闭。
func CommitGroup(ctx context.Context, outputDir string, outputs []GroupOutput) error {
	if ctx == nil {
		return errors.New("book: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	plans := make([]zipfs.DirectoryPlan, 0, len(outputs))
	for _, output := range outputs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if output.Book == nil {
			return fmt.Errorf("book: nil group output %q", output.Name)
		}
		plans = append(plans, zipfs.DirectoryPlan{Name: output.Name, Plans: output.Book.plans()})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(plans) == 0 {
		return errors.New("book: group has no outputs")
	}
	// All group members are opened from the same source archive in the split
	// capability. Their plan snapshots are complete before this call, so the
	// only filesystem mutation is the zipfs directory commit.
	return outputs[0].Book.arch.WriteDirectory(ctx, outputDir, plans)
}

// ValidatePath 校验 entry 路径不会逃逸容器根（对齐 epub_lib.validate_archive_path）。
func ValidatePath(name string) error {
	if name == "" || strings.HasPrefix(name, "/") {
		return fmt.Errorf("%w: %q", ErrInvalidPath, name)
	}
	cleaned, ok := normalizePath(name)
	if !ok || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%w: %q", ErrInvalidPath, name)
	}
	return nil
}

// normalizePath 是 path.Clean 的保守版：只处理 "." 与 ".." 段，
// 出现逃逸时返回 ok=false。
func normalizePath(name string) (string, bool) {
	parts := strings.Split(name, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) == 0 {
				return "", false
			}
			out = out[:len(out)-1]
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), true
}

func isMacOSMetadataPath(name string) bool {
	base := name
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, "/") == ".DS_Store"
}
