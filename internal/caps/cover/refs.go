// Reference rewriting preserves markup regions and uses lossless CSS token spans.
package cover

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// rewriteURI 复刻 core.rewrite_uri（静默失败）。
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
	if resolved, err := resolveRelativePath(newDocument, parts.path); err == nil && resolved == target {
		return uri
	}
	newPath := relativeURI(newDocument, target)
	return pyURLUnsplitPath(newPath, parts.query, parts.fragment)
}

// pyQuote 复刻 quote(value, safe="/:@-._~")。
func pyQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/', c == ':', c == '@':
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}

// pyRelPath 复刻 posixpath.relpath 的段级计算。
func pyRelPath(target, base string) string {
	startList := splitSegments(base)
	pathList := splitSegments(target)
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	rel := make([]string, 0, len(startList)-i+len(pathList)-i)
	for k := 0; k < len(startList)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func splitSegments(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// relativeURI 复刻 core.relative_uri。
func relativeURI(fromArchivePath, toArchivePath string) string {
	base := pyDirname(fromArchivePath)
	rel := toArchivePath
	if base != "" {
		rel = pyRelPath(toArchivePath, base)
	}
	return pyQuote(rel)
}

// pyURLUnsplitPath 复刻 urlunsplit(("", "", path, query, fragment))。
func pyURLUnsplitPath(pathPart, query, fragment string) string {
	out := pathPart
	if query != "" {
		out += "?" + query
	}
	if fragment != "" {
		out += "#" + fragment
	}
	return out
}

// rewriteMarkupReferences delegates region and tag scanning to scan/xhtml while
// retaining this capability's URI resolution and CSS/entity escaping adapters.
func rewriteMarkupReferences(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool, warn func(format string, a ...any)) (string, error) {
	rewriteURIValue := func(uri string) string {
		return rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles)
	}
	rewriteCSSValue := func(raw, document string, quote byte, rewrite func(string) string) (string, error) {
		if strings.Contains(raw, "&") {
			return rewriteCSSWithEntityMap(raw, document, quote, rewrite)
		}
		edits, err := css.RewriteCSS(document, []byte(raw), rewrite)
		if err != nil {
			return "", toolErrf("%s: CSS reference scan: %v", document, err)
		}
		updated, err := editset.Apply(document, []byte(raw), edits)
		return string(updated), err
	}
	return xhtml.RewriteMarkup(text, oldDocument, rewriteURIValue, rewriteCSSValue, attrEscapeFor, warn)
}

func rewriteCSSWithEntityMap(raw, path string, quote byte, rewrite func(string) string) (string, error) {
	decoded, rawOff, err := xhtml.DecodeAttrWithMap(raw)
	if err != nil {
		return "", fmt.Errorf("%s: CSS entity decode: %w", path, err)
	}
	edits, err := css.ReferenceEdits(path, []byte(decoded), rewrite)
	if err != nil {
		return "", toolErrf("%s: CSS reference scan: %v", path, err)
	}
	if len(edits) == 0 {
		return raw, nil
	}
	mapped := make([]editset.Edit, 0, len(edits))
	for _, edit := range edits {
		start := int(edit.Offset)
		end := start + int(edit.Length)
		if start < 0 || end < start || end >= len(rawOff) {
			return "", fmt.Errorf("%s: CSS reference span cannot be mapped to source text", path)
		}
		replacement := string(edit.Replacement)
		if quote == 0 {
			replacement = pypath.EscapeText(replacement)
		} else {
			replacement = attrEscapeFor(quote, replacement)
		}
		rawStart, rawEnd := rawOff[start], rawOff[end]
		mapped = append(mapped, editset.Replace(path, int64(rawStart), int64(rawEnd-rawStart), []byte(replacement)))
	}
	updated, err := editset.Apply(path, []byte(raw), mapped)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}

func hasAttrName(names []string, candidate string) bool {
	for _, name := range names {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

// rewriteCSSOnly 只做 CSS url()/@import 重写（不含 srcset / URI 属性），
// 用于 <style> 元素内容与 style="…" 属性值——这两处都已经确定是 CSS 语义，
// 不需要也不应该再跑属性名匹配。
func rewriteCSSOnly(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) (string, error) {
	edits, err := css.RewriteCSS(oldDocument, []byte(text), func(uri string) string {
		return rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles)
	})
	if err != nil {
		return "", toolErrf("%s: CSS reference scan: %v", oldDocument, err)
	}
	updated, err := editset.Apply(oldDocument, []byte(text), edits)
	return string(updated), err
}

// transformResource 复刻 core.transform_resource，但按文件类型分流重写
// 策略：独立 .css 文件整份就是 CSS，全文正则替换是正确语义；markupExtensions
// （XHTML/NCX/OPF 同族标记文件）改用区域感知重写，避免字符数据里的转义
// 示例文本被当成标记误改（见上方 rewriteMarkupReferences 注释）。
func transformResource(data []byte, oldPath, newPath string, pathMap map[string]string, knownFiles map[string]bool, warn func(format string, a ...any)) ([]byte, error) {
	ext := strings.ToLower(pathExt(oldPath))
	if ext != ".css" && !markupExtensions[ext] {
		return data, nil
	}
	if !utf8.Valid(data) {
		return nil, toolErrf("%s: reference resource is not UTF-8", oldPath)
	}
	var updated string
	var err error
	if ext == ".css" {
		updated, err = rewriteCSSOnly(string(data), oldPath, newPath, pathMap, knownFiles)
	} else {
		updated, err = rewriteMarkupReferences(string(data), oldPath, newPath, pathMap, knownFiles, warn)
	}
	if err != nil {
		return nil, err
	}
	return []byte(updated), nil
}

// ---- 通用扫描器 ----

type uriMatch struct {
	start, end int
	prefix     string
	quote      byte
	uri        string
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
