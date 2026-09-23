// Capability-specific text compatibility helpers. Paths are provided by book/pypath.
package typography

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// decodeUTF8Replace 复刻 bytes.decode("utf-8", errors="replace")：
// 非法子序列替换为 U+FFFD。
func decodeUTF8Replace(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	return strings.ToValidUTF8(string(data), "\uFFFD")
}

// ---- 文本处理（Python str 语义） ----

func isSpaceRune(r rune) bool { return unicode.IsSpace(r) }

// ---- 正则语义助手（RE2 无反向引用/前瞻处手工实现） ----

// isWordRune 对齐 Python \w（字母、数字、下划线，Unicode 感知）。
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// prevIsWord 报告 text[i] 之前的字符是否属于 \w。
func prevIsWord(text string, i int) bool {
	if i <= 0 || i > len(text) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return isWordRune(r)
}

// wordBoundaryAt 对齐 Python \b（位置 i 处的词边界）。
func wordBoundaryAt(text string, i int) bool {
	before := prevIsWord(text, i)
	after := false
	if i < len(text) {
		r, _ := utf8.DecodeRuneInString(text[i:])
		after = isWordRune(r)
	}
	return before != after
}

// skipPySpace 跳过 Unicode 空白（含换行），对齐正则 \s* 的贪心消耗。
func skipPySpace(text string, i int) int {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !isSpaceRune(r) {
			break
		}
		i += size
	}
	return i
}

// indexFold 返回 s[from:] 中首个与 sub 大小写不敏感匹配的位置。
func indexFold(s, sub string, from int) int {
	n := len(sub)
	if n == 0 {
		return from
	}
	for i := from; i+n <= len(s); i++ {
		if strings.EqualFold(s[i:i+n], sub) {
			return i
		}
	}
	return -1
}

// pyRepr 近似 Python 的 str repr（错误消息里的 {layer!r}）。
func pyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// decodeRune / decodeLastRune 是 utf8 包的便捷封装。
func decodeRune(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

func decodeLastRune(s string) (rune, int) { return utf8.DecodeLastRuneInString(s) }

// pyLineCount 复刻 len(text.splitlines())：按 \r\n / \r / \n 以及
// \v \f \x1c \x1d \x1e \x85 \u2028 \u2029 切行的行数。
func pyLineCount(text string) int {
	if text == "" {
		return 0
	}
	n := 0
	i := 0
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch r {
		case '\r':
			if i < len(text) && text[i] == '\n' {
				i++
			}
			n++
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			n++
		}
	}
	return n
}
