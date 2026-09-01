// filemodel.go 在内存里复刻 Python 侧 `files: dict[str, bytes]` 的操作
// 语义（赋值 / pop / update / 成员判断），并在扫描结束时折叠为
// []editset.Edit 交给 book.Apply —— 中间态一律留在内存（INV-3）。
package csscleanup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
)

type fileModel struct {
	b         *book.Book
	orig      map[string]bool           // 输入投影的键集快照
	exists    map[string]bool           // 当前 files 的键集（含新增，去掉已删除）
	cache     map[string][]byte         // 当前投影缓存；不是写回计划
	generated map[string][]byte         // 新 entry 的完整内容
	patches   map[string][]editset.Edit // 既有 entry 的原始坐标编辑
	whole     map[string]bool           // 仅用于拒绝误用 set 覆盖既有 entry
}

func newFileModel(b *book.Book, names []string) *fileModel {
	m := &fileModel{
		b:         b,
		orig:      make(map[string]bool, len(names)),
		exists:    make(map[string]bool, len(names)),
		cache:     make(map[string][]byte),
		generated: make(map[string][]byte),
		patches:   make(map[string][]editset.Edit),
		whole:     make(map[string]bool),
	}
	for _, n := range names {
		m.orig[n] = true
		m.exists[n] = true
	}
	return m
}

// has 复刻 `name in files`。
func (m *fileModel) has(name string) bool { return m.exists[name] }

// raw 返回 entry 当前字节（无内存覆盖时懒加载 book.Current）。
func (m *fileModel) raw(name string) ([]byte, error) {
	data, ok := m.get(name)
	if !ok {
		return nil, fmt.Errorf("css cleanup: entry %q is not present", name)
	}
	return data, nil
}

// get 复刻 files.get：返回当前字节与成员判断（懒加载）。
func (m *fileModel) get(name string) ([]byte, bool) {
	if data, ok := m.cache[name]; ok {
		return data, true
	}
	if !m.exists[name] {
		return nil, false
	}
	if data, ok := m.generated[name]; ok {
		m.cache[name] = data
		return data, true
	}
	data, err := m.b.Current(name)
	if err != nil {
		return nil, false
	}
	if edits := m.patches[name]; len(edits) > 0 {
		data, err = editset.Apply(name, data, edits)
		if err != nil {
			return nil, false
		}
	}
	m.cache[name] = data
	return data, true
}

// set records a complete value only for a new entry. Existing entries must go
// through patch, otherwise a later edit fold could accidentally emit a
// Replace(0,len) whole-file rewrite.
func (m *fileModel) set(name string, data []byte) {
	m.exists[name] = true
	m.cache[name] = data
	if m.orig[name] {
		m.whole[name] = true
		return
	}
	m.generated[name] = data
}

// patch appends edits expressed against the current projected bytes. The
// accumulated edit set remains anchored to the original entry bytes, so
// callers can safely perform several independent scan passes without ever
// falling back to a whole-entry replacement.
func (m *fileModel) patch(name string, edits []editset.Edit) error {
	if !m.exists[name] {
		return fmt.Errorf("css cleanup: cannot patch missing entry %q", name)
	}
	if m.whole[name] {
		return fmt.Errorf("css cleanup: whole-entry replacement already requested for %q", name)
	}
	if len(edits) == 0 {
		return nil
	}
	for _, e := range edits {
		if e.Path != name {
			return fmt.Errorf("css cleanup: patch for %q contains %q", name, e.Path)
		}
		if e.Replacement == nil {
			return errors.New("css cleanup: content patch cannot delete an entry")
		}
	}
	if err := editset.Validate(edits); err != nil {
		return err
	}
	ordered := slices.Clone(edits)
	editset.Sort(ordered)
	old := m.patches[name]
	if len(old) == 0 {
		m.patches[name] = ordered
		delete(m.cache, name)
		return nil
	}
	current, ok := m.get(name)
	if !ok {
		return fmt.Errorf("css cleanup: cannot read projected entry %q", name)
	}
	// A later pass may legitimately target bytes inserted by an earlier pass
	// (for example, a link cloned during stylesheet factoring). Such bytes have
	// no original coordinate, but they are still safe to update when the new
	// range is strictly inside one prior replacement. Compose those edits into
	// the replacement payload; ranges crossing a replacement boundary remain a
	// hard error rather than an offset guess.
	type replacementSpan struct{ start, end int64 }
	spans := currentReplacementSpans(old)
	inside := make(map[int][]editset.Edit)
	var rebased []editset.Edit
	for _, e := range ordered {
		if index, ok := replacementContaining(spans, e); ok {
			local := e
			local.Offset -= spans[index].start
			inside[index] = append(inside[index], local)
			continue
		}
		if replacementCrossed(spans, e) {
			return fmt.Errorf("css cleanup: patch for %q crosses an earlier generated range", name)
		}
		offset, length, err := rebaseRange(e.Offset, e.Length, old, len(current))
		if err != nil {
			return fmt.Errorf("css cleanup: rebase %q patch at %d: %w", name, e.Offset, err)
		}
		rebased = append(rebased, editset.Replace(name, offset, length, e.Replacement))
	}
	updatedOld := slices.Clone(old)
	for index, nested := range inside {
		if err := editset.Validate(nested); err != nil {
			return err
		}
		payload, err := editset.Apply(name, updatedOld[index].Replacement, nested)
		if err != nil {
			return fmt.Errorf("css cleanup: compose generated range in %q: %w", name, err)
		}
		updatedOld[index].Replacement = payload
	}
	combined := append(updatedOld, rebased...)
	editset.Sort(combined)
	if err := editset.Validate(combined); err != nil {
		return fmt.Errorf("css cleanup: composed patch for %q is invalid (old=%v rebased=%v): %w", name, updatedOld, rebased, err)
	}
	m.patches[name] = combined
	delete(m.cache, name)
	return nil
}

func currentReplacementSpans(edits []editset.Edit) []struct{ start, end int64 } {
	spans := make([]struct{ start, end int64 }, len(edits))
	origPos, currentPos := int64(0), int64(0)
	for i, e := range edits {
		currentPos += e.Offset - origPos
		spans[i] = struct{ start, end int64 }{currentPos, currentPos + int64(len(e.Replacement))}
		currentPos = spans[i].end
		origPos = e.Offset + e.Length
	}
	return spans
}

func replacementContaining(spans []struct{ start, end int64 }, e editset.Edit) (int, bool) {
	end := e.Offset + e.Length
	for i, span := range spans {
		if e.Length > 0 {
			// Replacing exactly the payload created by an earlier pass is
			// also unambiguous: the new pass is referring to the same
			// generated byte range. Insertions at a boundary remain
			// ambiguous and are handled by original-coordinate rebasing.
			if e.Offset >= span.start && end <= span.end {
				return i, true
			}
		} else if e.Offset > span.start && e.Offset < span.end {
			return i, true
		}
	}
	return 0, false
}

func replacementCrossed(spans []struct{ start, end int64 }, e editset.Edit) bool {
	end := e.Offset + e.Length
	for _, span := range spans {
		if e.Length == 0 {
			continue
		}
		if e.Offset < span.end && end > span.start &&
			!(e.Offset >= span.start && end <= span.end) {
			return true
		}
	}
	return false
}

// drop 复刻 files.pop(name, None)。
func (m *fileModel) drop(name string) {
	delete(m.exists, name)
	delete(m.cache, name)
	delete(m.generated, name)
	delete(m.patches, name)
	delete(m.whole, name)
}

// unionHas 复刻 unique_zip_path 的 `{**files, **generated}` 成员判断。
func (m *fileModel) unionHas(name string) bool { return m.exists[name] }

// edits 把内存模型折叠为 editset：删除型（entry 级）、替换型与
// 新建型（Offset=0/Length=0）。OPF 的字节区间编辑由调用方追加。
func (m *fileModel) edits(opfPath string, opfEdits []editset.Edit) ([]editset.Edit, error) {
	var edits []editset.Edit
	for name := range m.orig {
		if !m.exists[name] {
			if name == opfPath {
				// OPF 缺失属于结构异常，交由调用方路径报错；这里按原样跳过。
				continue
			}
			edits = append(edits, editset.Delete(name))
			continue
		}
		if m.whole[name] {
			return nil, fmt.Errorf("css cleanup: refusing whole-entry replacement for existing %q", name)
		}
		if patch := m.patches[name]; len(patch) > 0 {
			edits = append(edits, patch...)
		}
	}
	for _, name := range sortedKeys(m.exists) {
		if m.orig[name] {
			continue
		}
		data, ok := m.generated[name]
		if !ok {
			continue
		}
		edits = append(edits, editset.Replace(name, 0, 0, data))
	}
	edits = append(edits, opfEdits...)
	return edits, nil
}

// rebaseRange maps a range in the currently projected bytes to original byte
// coordinates. A range that starts or ends inside an earlier replacement is
// rejected: composing such edits would require guessing the caller's intent.
func rebaseRange(offset, length int64, old []editset.Edit, currentLen int) (int64, int64, error) {
	if offset < 0 || length < 0 || offset > int64(currentLen) || length > int64(currentLen)-offset {
		return 0, 0, fmt.Errorf("range [%d,%d) out of bounds for %d bytes", offset, offset+length, currentLen)
	}
	start, ok := mapCurrentBoundary(offset, old, currentLen)
	if !ok {
		return 0, 0, errors.New("range starts inside a previous replacement")
	}
	end, ok := mapCurrentBoundary(offset+length, old, currentLen)
	if !ok {
		return 0, 0, errors.New("range ends inside a previous replacement")
	}
	if end < start {
		return 0, 0, errors.New("mapped range is reversed")
	}
	return start, end - start, nil
}

func mapCurrentBoundary(pos int64, old []editset.Edit, currentLen int) (int64, bool) {
	origPos, currentPos := int64(0), int64(0)
	for _, e := range old {
		unchanged := e.Offset - origPos
		if pos <= currentPos+unchanged {
			return origPos + (pos - currentPos), true
		}
		currentPos += unchanged
		replacementLen := int64(len(e.Replacement))
		if pos < currentPos+replacementLen {
			return 0, false
		}
		currentPos += replacementLen
		origPos = e.Offset + e.Length
		if pos == currentPos {
			return origPos, true
		}
	}
	if pos <= int64(currentLen) {
		return origPos + (pos - currentPos), true
	}
	return 0, false
}

// jsonRawMessage 把 MarshalLegacy 的输出作为 RawMessage 存入 Facts，
// 避免 []byte 被信封编码成 base64。
func jsonRawMessage(raw []byte) json.RawMessage {
	return json.RawMessage(bytes.TrimSuffix(raw, []byte("\n")))
}
