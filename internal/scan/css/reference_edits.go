package css

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
)

// ReferenceEdits scans CSS resource references and returns byte-range edits for
// rewritten local references. It preserves all source bytes outside the value
// spans reported by ScanReferences.
func ReferenceEdits(path string, data []byte, rewrite func(string) string) ([]editset.Edit, error) {
	if rewrite == nil {
		return nil, fmt.Errorf("%s: CSS reference rewrite callback is nil", path)
	}

	references, err := ScanReferences(data)
	if err != nil {
		return nil, fmt.Errorf("%s: scan CSS references: %w", path, err)
	}

	var edits []editset.Edit
	for _, ref := range references {
		if ref.DataURL || pypath.IsExternalURI(ref.Value) {
			continue
		}
		if strings.ContainsRune(ref.Value, '\\') {
			return nil, fmt.Errorf("%s: CSS reference %q contains a backslash escape that cannot be safely interpreted", path, ref.Value)
		}

		rewritten := rewrite(ref.Value)
		if rewritten == ref.Value {
			continue
		}
		if err := validateReferenceValue(rewritten); err != nil {
			return nil, fmt.Errorf("%s: rewritten CSS reference %q is unsafe: %w", path, ref.Value, err)
		}

		edits = append(edits, editset.Replace(
			path,
			int64(ref.ValueSpan.Start),
			int64(ref.ValueSpan.Len()),
			[]byte(rewritten),
		))
	}
	return edits, nil
}

func validateReferenceValue(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("value is not valid UTF-8")
	}
	for _, r := range value {
		switch r {
		case '\'', '"', '(', ')', '\\':
			return fmt.Errorf("contains unsafe CSS character %q", r)
		}
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("contains whitespace or control character U+%04X", r)
		}
	}
	return nil
}
