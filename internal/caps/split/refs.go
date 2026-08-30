// refs.go 只保留 split 需要的 URI 命中抽取（URI_ATTRIBUTE_RE /
// CSS_URL_RE / CSS_IMPORT_RE 的手工扫描实现，见 merge 包同名实现）。
package split

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// utf8Valid 对齐 bytes.decode("utf-8") 的严格性。
func utf8Valid(data []byte) bool { return utf8.Valid(data) }

// collectRawURIs 抽取三种 URI 正则命中的 uri 值（BFS 用）。
func collectRawURIs(text string) []string {
	var out []string
	for _, m := range findNameQuoteMatches(text, 0, uriAttrNames) {
		out = append(out, m.uri)
	}
	for _, m := range findURLMatches(text, 0) {
		out = append(out, m.uri)
	}
	for _, m := range findImportMatches(text, 0) {
		out = append(out, m.uri)
	}
	return out
}

type uriMatch struct {
	start, end int
	prefix     string
	quote      byte
	uri        string
	suffix     string
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func wordBoundary(text string, i int) bool {
	before := false
	if i > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:i])
		before = isWordRune(r)
	}
	after := false
	if i < len(text) {
		r, _ := utf8.DecodeRuneInString(text[i:])
		after = isWordRune(r)
	}
	return before != after
}

func skipPySpace(text string, i int) int {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

func findNameQuoteMatches(text string, from int, names []string) []uriMatch {
	var out []uriMatch
	i := from
	for i < len(text) {
		if wordBoundary(text, i) {
			matched := false
			for _, name := range names {
				if i+len(name) > len(text) || !strings.EqualFold(text[i:i+len(name)], name) {
					continue
				}
				j := skipPySpace(text, i+len(name))
				if j >= len(text) || text[j] != '=' {
					continue
				}
				j = skipPySpace(text, j+1)
				if j >= len(text) || (text[j] != '"' && text[j] != '\'') {
					continue
				}
				quote := text[j]
				uriStart := j + 1
				idx := strings.IndexByte(text[uriStart:], quote)
				if idx < 0 {
					continue
				}
				uriEnd := uriStart + idx
				out = append(out, uriMatch{
					start: i, end: uriEnd + 1,
					prefix: text[i:j],
					quote:  quote,
					uri:    text[uriStart:uriEnd],
				})
				i = uriEnd + 1
				matched = true
				break
			}
			if matched {
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return out
}

func findURLMatches(text string, from int) []uriMatch {
	var out []uriMatch
	i := from
	for i < len(text) {
		if wordBoundary(text, i) && i+4 <= len(text) && strings.EqualFold(text[i:i+4], "url(") {
			j := skipPySpace(text, i+4)
			quote := byte(0)
			quoteStart := -1
			if j < len(text) && (text[j] == '"' || text[j] == '\'') {
				quote = text[j]
				quoteStart = j
				j++
			}
			uriStart := j
			found := false
			if quote != 0 {
				p := j
				for p < len(text) {
					idx := strings.IndexByte(text[p:], quote)
					if idx < 0 {
						break
					}
					qPos := p + idx
					k := skipPySpace(text, qPos+1)
					if k < len(text) && text[k] == ')' {
						out = append(out, uriMatch{
							start: i, end: k + 1,
							prefix: text[i:quoteStart],
							quote:  quote,
							uri:    text[uriStart:qPos],
							suffix: text[qPos+1 : k+1],
						})
						i = k + 1
						found = true
						break
					}
					p = qPos + 1
				}
				if found {
					continue
				}
				uriStart = quoteStart
			}
			idx := strings.IndexByte(text[uriStart:], ')')
			if idx >= 0 {
				closePos := uriStart + idx
				wsStart := closePos
				for wsStart > uriStart {
					r, size := utf8.DecodeLastRuneInString(text[uriStart:wsStart])
					if !unicode.IsSpace(r) {
						break
					}
					wsStart -= size
				}
				out = append(out, uriMatch{
					start: i, end: closePos + 1,
					prefix: text[i:uriStart],
					quote:  0,
					uri:    text[uriStart:wsStart],
					suffix: text[wsStart : closePos+1],
				})
				i = closePos + 1
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return out
}

func findImportMatches(text string, from int) []uriMatch {
	var out []uriMatch
	for i := from; i+7 <= len(text); i++ {
		if strings.EqualFold(text[i:i+7], "@import") {
			j := skipPySpace(text, i+7)
			if j > i+7 && j < len(text) && (text[j] == '"' || text[j] == '\'') {
				quote := text[j]
				uriStart := j + 1
				idx := strings.IndexByte(text[uriStart:], quote)
				if idx >= 0 {
					uriEnd := uriStart + idx
					out = append(out, uriMatch{
						start: i, end: uriEnd + 1,
						prefix: text[i:j],
						quote:  quote,
						uri:    text[uriStart:uriEnd],
					})
					i = uriEnd
				}
			}
		}
	}
	return out
}
