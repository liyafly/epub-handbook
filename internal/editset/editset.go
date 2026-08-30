// Package editset 是纯字节区间编辑的模型（SPEC §1 第 6 层）。
//
// Edit 只描述「在某个 entry 的哪个字节区间替换成什么」，
// 不理解 XML/CSS 语义 —— 语义判断属于 scan 层（INV-2）。
package editset

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Edit 是对一个 entry 的一段字节区间的替换。
//
//   - Replacement != nil：把 [Offset, Offset+Length) 替换为 Replacement。
//     Length 为 0 表示插入；Replacement 为空切片表示删除该区间。
//   - Replacement == nil：删除整个 entry（Offset/Length 无意义）。
//     指向尚不存在 entry 的创建型编辑见 book.Apply 的语义。
type Edit struct {
	Path        string
	Offset      int64
	Length      int64
	Replacement []byte
}

// Delete 返回删除整个 entry 的编辑。
func Delete(path string) Edit {
	return Edit{Path: path}
}

// Replace 返回把 [offset, offset+length) 替换为 replacement 的编辑。
func Replace(path string, offset, length int64, replacement []byte) Edit {
	return Edit{Path: path, Offset: offset, Length: length, Replacement: replacement}
}

// Insert 返回在 offset 处插入 replacement 的编辑。
func Insert(path string, offset int64, replacement []byte) Edit {
	return Edit{Path: path, Offset: offset, Replacement: replacement}
}

// Sort 按 Path 升序、Offset 升序稳定排序。同一位置的插入保持相对次序，
// 保证批量应用的结果确定。
func Sort(edits []Edit) {
	slices.SortStableFunc(edits, func(a, b Edit) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return cmp.Compare(a.Offset, b.Offset)
	})
}

// ErrOverlap 表示同一段字节区间被两个编辑命中。
var ErrOverlap = errors.New("editset: overlapping edits")

// Apply 把全部属于 name 的编辑应用到 content 上并返回新字节。
// 混入其它 entry 的编辑、越界区间与重叠区间都是错误。
// Replacement 为 nil 的（删除 entry 型）编辑在这里被跳过，
// 由 book 层处理 —— 字节拼接无法表达删除整个文件。
func Apply(name string, content []byte, edits []Edit) ([]byte, error) {
	group := make([]Edit, 0, len(edits))
	for _, e := range edits {
		if e.Path != name {
			return nil, fmt.Errorf("editset: batch for %q contains edit for %q", name, e.Path)
		}
		if e.Replacement != nil {
			group = append(group, e)
		}
	}
	if len(group) == 0 {
		return content, nil
	}
	Sort(group)

	grow := int64(len(content))
	for _, e := range group {
		grow += int64(len(e.Replacement)) - e.Length
	}
	out := make([]byte, 0, max(grow, 0))
	var end int64
	for _, e := range group {
		switch {
		case e.Offset < 0 || e.Offset > int64(len(content)) || e.Offset+e.Length > int64(len(content)):
			return nil, fmt.Errorf("editset: %s: range [%d,%d) out of bounds for %d bytes",
				name, e.Offset, e.Offset+e.Length, len(content))
		case e.Offset < end:
			return nil, fmt.Errorf("%w: %s at offset %d", ErrOverlap, name, e.Offset)
		}
		out = append(out, content[end:e.Offset]...)
		out = append(out, e.Replacement...)
		end = e.Offset + e.Length
	}
	out = append(out, content[end:]...)
	return out, nil
}

// Validate 在不触碰内容的情况下检查一组编辑能否应用：
// 区间必须不重叠且按 (Path, Offset) 唯一可判定。返回第一个错误。
func Validate(edits []Edit) error {
	sorted := slices.Clone(edits)
	Sort(sorted)
	type span struct{ lo, hi int64 }
	var (
		cur   string
		found bool
		last  span
	)
	for _, e := range sorted {
		if !found || e.Path != cur {
			cur, found, last = e.Path, true, span{e.Offset, e.Offset + e.Length}
			continue
		}
		if e.Offset < last.hi {
			return fmt.Errorf("%w: %s at offset %d", ErrOverlap, e.Path, e.Offset)
		}
		if e.Offset+e.Length > last.hi {
			last.hi = e.Offset + e.Length
		}
		last.lo = e.Offset
	}
	return nil
}
