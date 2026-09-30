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
		parts := pypath.URLSplit(ref.Value)
		if ref.DataURL || parts.Scheme != "" || parts.Netloc != "" {
			continue
		}
		if strings.HasPrefix(ref.Value, "/") {
			rewrite(ref.Value)
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

// MapEntityDecodedEdits maps CSS edits made against decoded attribute bytes back
// to the original source spans. rawOffsets has one source byte offset for every
// decoded byte boundary, as returned by xhtml.DecodeAttrWithMap. Escape is
// applied only to replacement text before the original attribute is edited.
func MapEntityDecodedEdits(path string, raw []byte, rawOffsets []int, edits []editset.Edit, escape func(string) string) ([]editset.Edit, error) {
	if len(edits) == 0 {
		return nil, nil
	}
	if escape == nil {
		escape = func(value string) string { return value }
	}
	if len(rawOffsets) == 0 {
		return nil, fmt.Errorf("%s: CSS reference span cannot be mapped to source text", path)
	}
	maxOffset := int64(len(rawOffsets) - 1)
	mapped := make([]editset.Edit, 0, len(edits))
	for _, edit := range edits {
		if edit.Offset < 0 || edit.Length < 0 || edit.Offset > maxOffset || edit.Length > maxOffset-edit.Offset {
			return nil, fmt.Errorf("%s: CSS reference span cannot be mapped to source text", path)
		}
		start := rawOffsets[int(edit.Offset)]
		end := rawOffsets[int(edit.Offset+edit.Length)]
		if start < 0 || end < start || end > len(raw) {
			return nil, fmt.Errorf("%s: CSS reference span cannot be mapped to source text", path)
		}
		replacement := escape(string(edit.Replacement))
		mapped = append(mapped, editset.Replace(path, int64(start), int64(end-start), []byte(replacement)))
	}
	return mapped, nil
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
