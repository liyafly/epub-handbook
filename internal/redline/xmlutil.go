package redline

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// sanitizeXML 复刻 validate_text_invariance.sanitize_xml：
// utf-8 宽容解码（非法字节 → U+FFFD）→ 删除全部 DOCTYPE → &nbsp; → &#160;。
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
	return strings.ReplaceAll(text, "&nbsp;", "&#160;")
}

// matchFnmatch 把 Python fnmatch 模式转成 RE2 再匹配。
// Python fnmatch 的 * 跨目录段，? 匹配单字符，[...] 为字符类。
func matchFnmatch(pattern, name string) bool {
	var b strings.Builder
	b.WriteString("(?s)^")
	for i := 0; i < len(pattern); {
		c := pattern[i]
		switch c {
		case '*':
			b.WriteString(".*")
			i++
		case '?':
			b.WriteString(".")
			i++
		case '[':
			j := i + 1
			negate := false
			if j < len(pattern) && (pattern[j] == '!' || pattern[j] == '^') {
				negate = true
				j++
			}
			// 找到闭合 ]（首个 ] 若紧跟 [ 或 [! 则视为字面量，与 Python 一致的处理从简）。
			k := j
			for k < len(pattern) && pattern[k] != ']' {
				k++
			}
			if k >= len(pattern) {
				b.WriteString("\\[")
				i++
				continue
			}
			body := pattern[j:k]
			body = strings.ReplaceAll(body, `\`, `\\`)
			if negate {
				b.WriteString("[^" + body + "]")
			} else {
				b.WriteString("[" + body + "]")
			}
			i = k + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(name)
}
