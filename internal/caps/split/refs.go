// refs.go 只保留 split 需要的 URI 命中抽取（URI_ATTRIBUTE_RE /
// CSS_URL_RE / CSS_IMPORT_RE 的手工扫描实现，见 merge 包同名实现）。
package split

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// utf8Valid 对齐 bytes.decode("utf-8") 的严格性。
func utf8Valid(data []byte) bool { return utf8.Valid(data) }

// collectRawURIs 抽取引用中的 URI 值（BFS 用）。保留这个无 error 的
// 兼容包装；实际资源收集使用 collectRawURIsStrict，不能静默丢弃坏的
// srcset。
func collectRawURIs(text string) []string {
	out, _ := collectRawURIsStrict(text)
	return out
}

// collectRawURIsStrict 抽取 URI 属性、srcset candidate、CSS url() 和
// @import 命中的 URI 值。srcset 不是单一 URI，必须先按 HTML candidate
// list 规则拆开再返回其每个 URL。
func collectRawURIsStrict(text string) ([]string, error) {
	var out []string
	for _, m := range findNameQuoteMatches(text, 0, uriAttrNames) {
		if strings.EqualFold(m.name, "srcset") {
			candidates, err := parseSrcsetCandidates(m.uri)
			if err != nil {
				return nil, err
			}
			for _, candidate := range candidates {
				out = append(out, candidate.url)
			}
			continue
		}
		out = append(out, m.uri)
	}
	for _, m := range findURLMatches(text, 0) {
		out = append(out, m.uri)
	}
	for _, m := range findImportMatches(text, 0) {
		out = append(out, m.uri)
	}
	return out, nil
}

// collectCSSURIsStrict scans CSS url() and quoted @import references while
// respecting comments and strings. It deliberately rejects an unterminated
// comment/string/function and CSS escapes inside a URL: retaining uncertain
// input is safer than silently dropping a local resource from a segment.
func collectCSSURIsStrict(text string) ([]string, error) {
	var out []string
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "/*") {
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("invalid CSS: unterminated comment at byte %d", i)
			}
			i += end + 4
			continue
		}
		if text[i] == '\'' || text[i] == '"' {
			next, err := skipCSSString(text, i)
			if err != nil {
				return nil, err
			}
			i = next
			continue
		}
		if i+4 <= len(text) && strings.EqualFold(text[i:i+4], "url(") && wordBoundary(text, i) {
			uri, next, err := parseCSSURL(text, i)
			if err != nil {
				return nil, err
			}
			if uri != "" {
				out = append(out, uri)
			}
			i = next
			continue
		}
		if i+7 <= len(text) && text[i] == '@' && strings.EqualFold(text[i:i+7], "@import") {
			j := i + 7
			if j < len(text) {
				r, _ := utf8.DecodeRuneInString(text[j:])
				if isWordRune(r) {
					i++
					continue
				}
			}
			j = skipPySpace(text, j)
			if j < len(text) && (text[j] == '\'' || text[j] == '"') {
				uri, next, err := parseCSSQuotedURL(text, j)
				if err != nil {
					return nil, err
				}
				if uri != "" {
					out = append(out, uri)
				}
				i = next
				continue
			}
		}
		i++
	}
	return out, nil
}

func skipCSSString(text string, start int) (int, error) {
	quote := text[start]
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			if i+1 >= len(text) {
				return 0, fmt.Errorf("invalid CSS: unterminated escape at byte %d", i)
			}
			i++
		case quote:
			return i + 1, nil
		case '\n', '\r':
			return 0, fmt.Errorf("invalid CSS: newline in string at byte %d", i)
		}
	}
	return 0, fmt.Errorf("invalid CSS: unterminated string at byte %d", start)
}

func parseCSSQuotedURL(text string, start int) (string, int, error) {
	quote := text[start]
	uriStart := start + 1
	for i := uriStart; i < len(text); i++ {
		switch text[i] {
		case '\\':
			return "", 0, fmt.Errorf("invalid CSS: escaped URL at byte %d", i)
		case quote:
			return text[uriStart:i], i + 1, nil
		case '\n', '\r':
			return "", 0, fmt.Errorf("invalid CSS: newline in URL at byte %d", i)
		}
	}
	return "", 0, fmt.Errorf("invalid CSS: unterminated URL at byte %d", start)
}

func parseCSSURL(text string, start int) (string, int, error) {
	i := skipPySpace(text, start+4)
	if i >= len(text) {
		return "", 0, fmt.Errorf("invalid CSS: unterminated url() at byte %d", start)
	}
	if text[i] == '\'' || text[i] == '"' {
		uri, next, err := parseCSSQuotedURL(text, i)
		if err != nil {
			return "", 0, err
		}
		next = skipPySpace(text, next)
		if next >= len(text) || text[next] != ')' {
			return "", 0, fmt.Errorf("invalid CSS: quoted url() missing closing ')' at byte %d", start)
		}
		return uri, next + 1, nil
	}
	uriStart := i
	for i < len(text) && text[i] != ')' {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			return "", 0, fmt.Errorf("invalid CSS: invalid UTF-8 at byte %d", i)
		}
		if r == '\\' || r == '\'' || r == '"' || r == '\n' || r == '\r' {
			return "", 0, fmt.Errorf("invalid CSS: escaped or quoted URL at byte %d", i)
		}
		i += size
	}
	if i >= len(text) {
		return "", 0, fmt.Errorf("invalid CSS: unterminated url() at byte %d", start)
	}
	uri := strings.TrimSpace(text[uriStart:i])
	if len(strings.Fields(uri)) > 1 {
		return "", 0, fmt.Errorf("invalid CSS: whitespace inside unquoted URL at byte %d", uriStart)
	}
	return uri, i + 1, nil
}

// srcsetCandidate 是一个已拆分的 HTML srcset candidate。descriptor 仅用于
// 保留解析结果的结构；资源闭包只需要 URL。
type srcsetCandidate struct {
	url        string
	descriptor string
}

// parseSrcsetCandidates 以保守的 HTML srcset candidate list 语义解析 value。
//
// URL 先读到空白（data: URL 允许 URL 内逗号），随后读取一个 descriptor；
// 普通 URL 的逗号分隔 candidate。引号、反斜杠、空 candidate、未知或重复
// descriptor 都返回错误，避免把不确定输入静默当成另一条本地资源引用。
func parseSrcsetCandidates(value string) ([]srcsetCandidate, error) {
	var out []srcsetCandidate
	i := 0
	for {
		i = skipPySpace(value, i)
		if i == len(value) {
			return out, nil
		}
		if value[i] == ',' {
			return nil, fmt.Errorf("invalid srcset: empty candidate at byte %d", i)
		}

		urlStart := i
		for i < len(value) {
			r, size := utf8.DecodeRuneInString(value[i:])
			if r == utf8.RuneError && size == 1 {
				return nil, fmt.Errorf("invalid srcset: invalid UTF-8 at byte %d", i)
			}
			if unicode.IsSpace(r) {
				break
			}
			if r == ',' && !strings.HasPrefix(strings.ToLower(value[urlStart:i]), "data:") {
				break
			}
			if r == '\\' || r == '\'' || r == '"' {
				return nil, fmt.Errorf("invalid srcset: escaped or quoted URL at byte %d", i)
			}
			i += size
		}
		if i == urlStart {
			return nil, fmt.Errorf("invalid srcset: missing URL at byte %d", i)
		}
		candidateURL := value[urlStart:i]

		// Skip the whitespace separating URL and descriptor. A comma here is
		// the candidate separator, while a data: URL may already have consumed
		// arbitrary commas above.
		i = skipPySpace(value, i)
		descriptorStart := i
		for i < len(value) && value[i] != ',' {
			r, size := utf8.DecodeRuneInString(value[i:])
			if r == utf8.RuneError && size == 1 {
				return nil, fmt.Errorf("invalid srcset: invalid UTF-8 at byte %d", i)
			}
			if r == '\\' || r == '\'' || r == '"' {
				return nil, fmt.Errorf("invalid srcset: escaped or quoted descriptor at byte %d", i)
			}
			i += size
		}
		descriptorText := strings.TrimSpace(value[descriptorStart:i])
		if descriptorText != "" {
			fields := strings.Fields(descriptorText)
			if len(fields) != 1 || !validSrcsetDescriptor(fields[0]) {
				return nil, fmt.Errorf("invalid srcset: unsupported descriptor %q", descriptorText)
			}
		}
		out = append(out, srcsetCandidate{url: candidateURL, descriptor: descriptorText})

		if i == len(value) {
			return out, nil
		}
		// A comma must be followed by another non-empty candidate. Leading,
		// doubled, and trailing commas are rejected instead of being dropped.
		i++
		next := skipPySpace(value, i)
		if next == len(value) || value[next] == ',' {
			return nil, fmt.Errorf("invalid srcset: empty candidate after byte %d", i-1)
		}
		i = next
	}
}

func validSrcsetDescriptor(value string) bool {
	if len(value) < 2 {
		return false
	}
	suffix := value[len(value)-1]
	number := value[:len(value)-1]
	if suffix != 'w' && suffix != 'x' {
		return false
	}
	if number == "" || strings.Count(number, ".") > 1 {
		return false
	}
	for i := 0; i < len(number); i++ {
		if number[i] < '0' || number[i] > '9' {
			if number[i] != '.' {
				return false
			}
		}
	}
	if number == "." || number == "0" || strings.Trim(number, "0.") == "" {
		return false
	}
	return true
}

type uriMatch struct {
	start, end int
	prefix     string
	quote      byte
	uri        string
	suffix     string
	name       string
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
					name:   name,
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
