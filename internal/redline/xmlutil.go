package redline

import (
	"strings"
	"unicode/utf8"
)

// sanitizeXML 复刻 validate_text_invariance.sanitize_xml：
// utf-8 宽容解码（非法字节 → U+FFFD）→ 删除全部 DOCTYPE。
func sanitizeXML(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			b.WriteRune(0xFFFD)
			i++
			continue
		}
		b.WriteRune(r)
		i += size
	}
	text := b.String()
	text = doctypeRe.ReplaceAllString(text, "")
	return text
}
