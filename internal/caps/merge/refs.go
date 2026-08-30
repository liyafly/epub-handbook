// refs.go 复刻 core.py 的引用重写正则族（URI_ATTRIBUTE_RE / SRCSET_RE /
// CSS_URL_RE / CSS_IMPORT_RE）。Go RE2 不支持反向引用，这里按 Python re
// 的匹配/回溯语义手工实现，输出与 re.sub 逐字节一致。
package merge

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// rewriteURI 复刻 core.rewrite_uri（静默失败：解析失败或目标未知时原样返回）。
func rewriteURI(uri, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	if uri == "" || strings.HasPrefix(uri, "#") || pyIsExternalURI(uri) {
		return uri
	}
	parts := pyURLSplit(uri)
	if parts.path == "" {
		return uri
	}
	oldTarget, err := resolveRelativePath(oldDocument, parts.path)
	if err != nil {
		return uri
	}
	if !knownFiles[oldTarget] {
		return uri
	}
	target := oldTarget
	if mapped, ok := pathMap[oldTarget]; ok {
		target = mapped
	}
	newPath := relativeURI(newDocument, target)
	return pyURLUnsplitPath(newPath, parts.query, parts.fragment)
}

// splitSrcsetCandidates 逐行复刻 core.split_srcset_candidates。
func splitSrcsetCandidates(value string) []string {
	var candidates []string
	start := 0
	inURL := true
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		if isPySpace(r) && strings.TrimSpace(value[start:index]) != "" {
			inURL = false
		} else if r == ',' {
			current := strings.TrimSpace(value[start:index])
			currentURL := ""
			if parts := splitPyWhitespace(current); len(parts) > 0 {
				currentURL = parts[0]
			}
			if inURL && strings.HasPrefix(strings.ToLower(currentURL), "data:") {
				index += size
				continue
			}
			candidates = append(candidates, value[start:index])
			start = index + size
			inURL = true
		}
		index += size
	}
	candidates = append(candidates, value[start:])
	return candidates
}

func isPySpace(r rune) bool {
	return unicode.IsSpace(r)
}

// splitPyWhitespace 复刻 str.split()（Unicode 空白切分、去空段）。
func splitPyWhitespace(s string) []string {
	return strings.Fields(s)
}

// rewriteSrcset 复刻 core.rewrite_srcset。
func rewriteSrcset(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	return subNameQuoteURI(text, []string{"srcset"}, func(prefix, quote, uri string) string {
		var candidates []string
		for _, candidate := range splitSrcsetCandidates(uri) {
			parts := splitPyWhitespace(strings.TrimSpace(candidate))
			if len(parts) == 0 {
				continue
			}
			url := rewriteURI(parts[0], oldDocument, newDocument, pathMap, knownFiles)
			descriptor := strings.Join(parts[1:], " ")
			candidates = append(candidates, strings.TrimSpace(url+" "+descriptor))
		}
		return prefix + quote + strings.Join(candidates, ", ") + quote
	})
}

// rewriteTextReferences 复刻 core.rewrite_text_references（srcset → URI 属性
// → CSS url() → CSS @import，与 Python 的调用顺序一致）。
func rewriteTextReferences(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	text = rewriteSrcset(text, oldDocument, newDocument, pathMap, knownFiles)
	text = subNameQuoteURI(text, uriAttrNames, func(prefix, quote, uri string) string {
		return prefix + quote + rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles) + quote
	})
	text = subCSSURL(text, func(prefix, quote, uri, suffix string) string {
		return prefix + quote + rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles) + quote + suffix
	})
	return subCSSImport(text, func(prefix, quote, uri string) string {
		return prefix + quote + rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles) + quote
	})
}

// transformResource 复刻 core.transform_resource：仅 CSS / 标记类参与重写，
// 非 UTF-8 字节原样返回。
func transformResource(data []byte, oldPath, newPath string, pathMap map[string]string, knownFiles map[string]bool) []byte {
	ext := strings.ToLower(pathExt(oldPath))
	if ext != ".css" && !markupExtensions[ext] {
		return data
	}
	if !utf8.Valid(data) {
		return data
	}
	text := string(data)
	return []byte(rewriteTextReferences(text, oldPath, newPath, pathMap, knownFiles))
}

// collectRawURIs 抽取三种 URI 正则命中的 uri 值（split 的 BFS 用）。
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

// ---- 通用扫描器（对齐 Python re 语义） ----

type uriMatch struct {
	start, end int
	prefix     string
	quote      byte // 0 表示无引号
	uri        string
	suffix     string
}

// isWordRune 对齐 Python \w。
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordBoundary 对齐 Python \b。
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

// subNameQuoteURI 复刻 `\b(?:names)\s*=\s*(["'])(.*?)\1` 的 re.sub。
func subNameQuoteURI(text string, names []string, repl func(prefix, quote, uri string) string) string {
	var out strings.Builder
	last := 0
	for _, m := range findNameQuoteMatches(text, last, names) {
		out.WriteString(text[last:m.start])
		out.WriteString(repl(m.prefix, string(m.quote), m.uri))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
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

// subCSSURL 复刻 `\burl\(\s*(["']?)(.*?)\1\s*\)` 的 re.sub（含引号分支
// 失败后回退为无引号的回溯行为）。
func subCSSURL(text string, repl func(prefix, quote, uri, suffix string) string) string {
	var out strings.Builder
	last := 0
	for _, m := range findURLMatches(text, last) {
		out.WriteString(text[last:m.start])
		q := ""
		if m.quote != 0 {
			q = string(m.quote)
		}
		out.WriteString(repl(m.prefix, q, m.uri, m.suffix))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
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
				// 回溯：引号组视为空，uri 从引号字符处开始。
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

// subCSSImport 复刻 `@import\s+(["'])(.*?)\1` 的 re.sub。
func subCSSImport(text string, repl func(prefix, quote, uri string) string) string {
	var out strings.Builder
	last := 0
	for _, m := range findImportMatches(text, last) {
		out.WriteString(text[last:m.start])
		out.WriteString(repl(m.prefix, string(m.quote), m.uri))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
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
