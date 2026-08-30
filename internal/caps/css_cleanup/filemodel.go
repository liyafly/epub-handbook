// filemodel.go 在内存里复刻 Python 侧 `files: dict[str, bytes]` 的操作
// 语义（赋值 / pop / update / 成员判断），并在扫描结束时折叠为
// []editset.Edit 交给 book.Apply —— 中间态一律留在内存（INV-3）。
package csscleanup

import (
	"bytes"
	"encoding/json"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
)

type fileModel struct {
	b    *book.Book
	orig map[string]bool   // 输入投影的键集快照
	exists map[string]bool // 当前 files 的键集（含新增，去掉已删除）
	contents map[string][]byte
}

func newFileModel(b *book.Book, names []string) *fileModel {
	m := &fileModel{
		b:        b,
		orig:     make(map[string]bool, len(names)),
		exists:   make(map[string]bool, len(names)),
		contents: map[string][]byte{},
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
	if data, ok := m.contents[name]; ok {
		return data, nil
	}
	return m.b.Current(name)
}

// get 复刻 files.get：返回当前字节与成员判断（懒加载）。
func (m *fileModel) get(name string) ([]byte, bool) {
	if data, ok := m.contents[name]; ok {
		return data, true
	}
	if !m.exists[name] {
		return nil, false
	}
	data, err := m.b.Current(name)
	if err != nil {
		return nil, false
	}
	m.contents[name] = data
	return data, true
}

// set 复刻 files[name] = value。
func (m *fileModel) set(name string, data []byte) {
	m.exists[name] = true
	m.contents[name] = data
}

// drop 复刻 files.pop(name, None)。
func (m *fileModel) drop(name string) {
	delete(m.exists, name)
	delete(m.contents, name)
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
		data, ok := m.contents[name]
		if !ok {
			continue
		}
		cur, err := m.b.Current(name)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(data, cur) {
			edits = append(edits, editset.Replace(name, 0, int64(len(cur)), data))
		}
	}
	for _, name := range sortedKeys(m.exists) {
		if m.orig[name] {
			continue
		}
		data, ok := m.contents[name]
		if !ok {
			continue
		}
		edits = append(edits, editset.Replace(name, 0, 0, data))
	}
	edits = append(edits, opfEdits...)
	return edits, nil
}

// jsonRawMessage 把 MarshalLegacy 的输出作为 RawMessage 存入 Facts，
// 避免 []byte 被信封编码成 base64。
func jsonRawMessage(raw []byte) json.RawMessage {
	return json.RawMessage(bytes.TrimSuffix(raw, []byte("\n")))
}
