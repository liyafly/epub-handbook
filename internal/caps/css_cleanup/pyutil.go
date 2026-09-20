// Capability-specific text compatibility helpers. Paths are provided by book/pypath.
package csscleanup

import (
	"strings"
	"unicode"
)

// ---- 文本处理（Python str 语义） ----

func isSpaceRune(r rune) bool { return unicode.IsSpace(r) }

// removeAllSpace 复刻 re.sub(r"\s+", "", value)。
func removeAllSpace(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if !isSpaceRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
