// refs.go 只保留 split 需要的 URI 命中抽取（URI_ATTRIBUTE_RE /
// CSS_URL_RE / CSS_IMPORT_RE 的手工扫描实现，见 merge 包同名实现）。
package split

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// utf8Valid 对齐 bytes.decode("utf-8") 的严格性。
func utf8Valid(data []byte) bool { return utf8.Valid(data) }

// collectMarkupURIsStrict 收集标记类文件（.xhtml/.html/.xml/.ncx/.svg/.smil）
// 里的**真实**引用：URI 属性、srcset candidate、`style="…"` 与 `<style>` 元素
// 内容里的 CSS url()/@import。
//
// 为什么不能沿用 collectRawURIsStrict 的裸文本扫描（2026-09-07 修）：
// 那条路径把 `src="…"` 当作正则命中，不区分它出现在标签里还是出现在字符
// 数据里。本仓库的回归样书本身就是一本讲 EPUB 的书，正文里大量出现只转义
// 了尖括号的示例代码，例如
//
//	<p>替换：&lt;audio src="../Audio/XinJing.mp3"/&gt;</p>
//
// 这里的 `src="../Audio/XinJing.mp3"` 是**读者可见的正文**，不是引用；
// 而 OEBPS/Audio/XinJing.mp3 并不存在于书里。裸扫描把它当成真引用，于是
// 资源闭合检查判定「referenced target missing from source」，
// epub.package.split 对一本能正常打开的书硬拒。
//
// 判据改为「解析出的属性」而不是「文本里长得像属性」：走 scan/opf 的只读
// 区间树，只看 node.Attrs 与 <style> 元素内容 —— 与同包 validation.go 的
// validateRetainedReferences 完全同源（那一侧一直是对的，本函数是把闭合
// 收集这一侧对齐过去，消掉同包内两套判据的分叉）。
func collectMarkupURIsStrict(data []byte) ([]string, error) {
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, node := range root.Walk() {
		for _, attr := range node.Attrs {
			if attr.Name.Space == "" && attr.Name.Local == "style" {
				uris, cerr := collectCSSURIsStrict(attr.Value)
				if cerr != nil {
					return nil, fmt.Errorf("style attribute: %w", cerr)
				}
				out = append(out, uris...)
				continue
			}
			kind, ok := resourceAttributeKind(attr.Name.Space, attr.Name.Local)
			if !ok {
				continue
			}
			if kind == "srcset" {
				candidates, serr := parseSrcsetCandidates(attr.Value)
				if serr != nil {
					return nil, serr
				}
				for _, candidate := range candidates {
					out = append(out, candidate.url)
				}
				continue
			}
			out = append(out, attr.Value)
		}
		if node.Name.Local == "style" {
			uris, cerr := collectCSSURIsStrict(node.IterText())
			if cerr != nil {
				return nil, fmt.Errorf("style element: %w", cerr)
			}
			out = append(out, uris...)
		}
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
