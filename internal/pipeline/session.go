package pipeline

import (
	"bytes"
	"slices"

	"github.com/liyafly/epub-handbook/internal/book"
)

// cleanSession holds one opened source archive and the last accepted in-memory
// state. Each stage edits a shallow Book fork and is committed only on success.
type cleanSession struct {
	original        *book.Book
	current         *book.Book
	stateID         string
	hasCandidate    bool
	normalizeReport []byte
}

func newCleanSession(original *book.Book) *cleanSession {
	return &cleanSession{original: original, current: original, stateID: "input"}
}

func (s *cleanSession) BeginStep() *book.Book {
	return s.current.Fork()
}

func (s *cleanSession) CommitStep(name string, candidate *book.Book) {
	s.current = candidate
	s.stateID = "step:" + name
	s.hasCandidate = true
}

func (s *cleanSession) SetNormalizeReport(data []byte) {
	s.normalizeReport = slices.Clone(data)
}

func (s *cleanSession) ModifiedEntries(candidate *book.Book) ([]string, error) {
	return changedBookEntries(s.current, candidate)
}

func changedBookEntries(before, after *book.Book) ([]string, error) {
	names := make(map[string]struct{}, len(before.Names())+len(after.Names()))
	for _, name := range before.Names() {
		names[name] = struct{}{}
	}
	for _, name := range after.Names() {
		names[name] = struct{}{}
	}
	changed := make([]string, 0)
	for name := range names {
		beforeHas, afterHas := before.Has(name), after.Has(name)
		if beforeHas != afterHas {
			changed = append(changed, name)
			continue
		}
		if !beforeHas || (!before.IsModified(name) && !after.IsModified(name)) {
			continue
		}
		beforeBytes, err := before.Current(name)
		if err != nil {
			return nil, err
		}
		afterBytes, err := after.Current(name)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(beforeBytes, afterBytes) {
			changed = append(changed, name)
		}
	}
	slices.Sort(changed)
	return changed, nil
}
