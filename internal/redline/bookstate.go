package redline

import (
	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// 确认 *zipfs.Archive 天然满足 State。
var _ State = (*zipfs.Archive)(nil)

type bookState struct {
	b        *book.Book
	original bool
}

func (s bookState) Path() string { return s.b.InputPath() }

func (s bookState) Names() []string {
	if s.original {
		return s.b.OriginalNames()
	}
	return s.b.Names()
}

func (s bookState) Read(name string) ([]byte, error) {
	if s.original {
		return s.b.Original(name)
	}
	return s.b.Current(name)
}

// OriginalState 返回 Book 的输入侧视图（未应用的编辑之前的状态）。
func OriginalState(b *book.Book) State { return bookState{b: b, original: true} }

// CurrentState 返回 Book 的当前视图（已应用全部编辑之后的状态）。
func CurrentState(b *book.Book) State { return bookState{b: b} }
