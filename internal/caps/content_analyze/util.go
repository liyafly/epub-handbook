// util.go 收纳 Python 语义的字符串工具。
package contentanalyze

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// isPySpace 覆盖 Python str.isspace / 正则 \s 的全集：
// unicode.White_Space + \x1c-\x1f。
func isPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// pyTrimSpace 复刻 str.strip()（按 Unicode 空白去两端）。
func pyTrimSpace(s string) string {
	return strings.TrimFunc(s, isPySpace)
}

// splitPyFields 复刻 str.split()（按 Unicode 空白切分，无空元素）。
func splitPyFields(value string) []string {
	fields := strings.FieldsFunc(value, isPySpace)
	if fields == nil {
		return []string{}
	}
	return fields
}

// pySplitLines 复刻 str.splitlines() 的行边界全集：
// \n \r \r\n \v \f \x1c \x1d \x1e \x85 \u2028 \u2029，且末尾边界不产生空尾行。
func pySplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\r':
			out = append(out, s[start:i])
			i += size
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		case '\n', '\v', '\f', '\x85', '\u2028', '\u2029', '\x1c', '\x1d', '\x1e':
			out = append(out, s[start:i])
			i += size
			start = i
		default:
			i += size
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// runeCut 按码点截断（Python text[:n] 语义，不是按字节）。
func runeCut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
