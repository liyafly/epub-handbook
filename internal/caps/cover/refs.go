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

// splitSrcsetCandidates 逐行复刻 core.split_srcset_candidates。
func splitSrcsetCandidates(value string) []string {
	var candidates []string
	start := 0
	inURL := true
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		if unicode.IsSpace(r) && strings.TrimSpace(value[start:index]) != "" {
			inURL = false
		} else if r == ',' {
			current := strings.TrimSpace(value[start:index])
			currentURL := ""
			if parts := strings.Fields(current); len(parts) > 0 {
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

// rewriteSrcset 复刻 core.rewrite_srcset。
func rewriteSrcset(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	return rewriteQuotedTagAttrs(text, []string{"srcset"}, func(_ string, _ byte, uri string) string {
		var candidates []string
		for _, candidate := range splitSrcsetCandidates(uri) {
			parts := strings.Fields(strings.TrimSpace(candidate))
			if len(parts) == 0 {
				continue
			}
			url := rewriteURI(parts[0], oldDocument, newDocument, pathMap, knownFiles)
			descriptor := strings.Join(parts[1:], " ")
			candidates = append(candidates, strings.TrimSpace(url+" "+descriptor))
		}
		return strings.Join(candidates, ", ")
	})
}

// ---- 区域感知的标记引用重写（xhtml/svg/ncx/… markupExtensions） ----
//
// 全文裸匹配对 XHTML 生效时，会把字符
// 数据里被实体转义写出的示例文本（`&lt;img src="…"/&gt;`）当成真标记一起
// 改掉——那是正文损坏（redline text 红线的最高安全属性）。这里改用
// internal/scan/xhtml.ScanRegions 先定位真实标记区域，再分流：属性/srcset/
// 内联 style 只在标签内部改；url()/@import 只在 <style> 元素内容与
// style="…" 属性值内改；<?xml-stylesheet …?> 只改 href 伪属性；字符数据、
// 注释、CDATA、其它 PI、DOCTYPE、<script> 内容原样透传。做法与
// internal/caps/structure_normalize 的 rewriteMarkupReferences 同源。

// rewriteMarkupReferences 是 markupExtensions 文件（.html/.htm/.xhtml/.xml/
// .ncx/.svg/.smil）的引用重写入口。扫描截断时通过 warn 上报文件名与字节
// 偏移（截断点之后的引用保持不变，不静默半改）；warn 为 nil 时静默丢弃。
func rewriteMarkupReferences(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool, warn func(format string, a ...any)) (string, error) {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete && warn != nil {
		warn("%s: markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); references after this offset left unchanged", oldDocument, stop)
	}
	if len(regions) == 0 {
		return text, nil
	}
	var out strings.Builder
	out.Grow(len(text))
	last := 0
	for _, r := range regions {
		out.WriteString(text[last:r.Start])
		segment := text[r.Start:r.End]
		var err error
		switch r.Kind {
		case xhtml.RegionTag:
			segment, err = rewriteTagReferences(segment, oldDocument, newDocument, pathMap, knownFiles)
		case xhtml.RegionStyle:
			if strings.Contains(segment, "&") {
				segment, err = rewriteCSSWithEntityMap(segment, oldDocument, 0, func(uri string) string {
					return rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles)
				})
			} else {
				segment, err = rewriteCSSOnly(segment, oldDocument, newDocument, pathMap, knownFiles)
			}
		case xhtml.RegionStylesheetPI:
			segment = rewriteStylesheetPIReference(segment, oldDocument, newDocument, pathMap, knownFiles)
		}
		if err != nil {
			return "", err
		}
		out.WriteString(segment)
		last = r.End
	}
	out.WriteString(text[last:])
	return out.String(), nil
}

// rewriteTagReferences 在单个标签的字节内重写 srcset、URI 属性与内联
// style 属性里的 url()/@import。
func rewriteTagReferences(tag, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) (string, error) {
	tag = rewriteSrcset(tag, oldDocument, newDocument, pathMap, knownFiles)
	var attrErr error
	tag = rewriteQuotedTagAttrs(tag, uriAttrNames, func(_ string, quote byte, raw string) string {
		uri := raw
		if strings.Contains(raw, "&") {
			decoded, _, err := xhtml.DecodeAttrWithMap(raw)
			if err != nil {
				attrErr = fmt.Errorf("%s: URI attribute entity decode: %w", oldDocument, err)
				return raw
			}
			uri = decoded
		}
		updated := rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles)
		if updated == uri {
			return raw
		}
		return attrEscapeFor(quote, updated)
	})
	if attrErr != nil {
		return "", attrErr
	}
	return rewriteInlineStyleReferences(tag, oldDocument, newDocument, pathMap, knownFiles)
}

// rewriteInlineStyleReferences 只在 style="…" 属性值内部做 CSS url()/@import
// 重写。整段标签跑 CSS 重写会连 title=""、alt="" 这类读者可见文本一起改
// （`<div title="url(a.png)">`），那是正文损坏；而内联 style 的 url() 是真
// 标记，资源搬家后必须跟着改，不能整体放弃。
func rewriteInlineStyleReferences(tag, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) (string, error) {
	var scanErr error
	updated := rewriteQuotedTagAttrs(tag, []string{"style"}, func(_ string, quote byte, value string) string {
		var result string
		var err error
		if strings.Contains(value, "&") {
			result, err = rewriteCSSWithEntityMap(value, oldDocument, quote, func(uri string) string {
				return rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles)
			})
		} else {
			result, err = rewriteCSSOnly(value, oldDocument, newDocument, pathMap, knownFiles)
		}
		if err != nil {
			scanErr = err
			return value
		}
		return result
	})
	if scanErr != nil {
		return "", scanErr
	}
	return updated, nil
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

func rewriteQuotedTagAttrs(tag string, names []string, rewrite func(name string, quote byte, value string) string) string {
	_, _, closing := xhtml.TagParts(tag)
	if closing {
		return tag
	}
	attrs, ok := xhtml.TagAttrs(tag)
	if !ok {
		return tag
	}
	var out strings.Builder
	last := 0
	changed := false
	for _, attr := range attrs {
		if attr.Quote == 0 || !hasAttrName(names, attr.Name) {
			continue
		}
		value := tag[attr.ValueSpan.Start:attr.ValueSpan.End]
		updated := rewrite(attr.Name, attr.Quote, value)
		if updated == value {
			continue
		}
		out.WriteString(tag[last:attr.ValueSpan.Start])
		out.WriteString(updated)
		last = attr.ValueSpan.End
		changed = true
	}
	if !changed {
		return tag
	}
	out.WriteString(tag[last:])
	return out.String()
}

func hasAttrName(names []string, candidate string) bool {
	for _, name := range names {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

// rewriteStylesheetPIReference 只重写 <?xml-stylesheet …?> 的 href 伪属性。
// PI 不是标签：type/media/title 伪属性与 CSS url() 语法都不参与重写。
func rewriteStylesheetPIReference(pi, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	return subNameQuoteURI(pi, []string{"href"}, func(prefix, quote, uri string) string {
		return prefix + quote + rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles) + quote
	})
}

// rewriteCSSOnly 只做 CSS url()/@import 重写（不含 srcset / URI 属性），
// 用于 <style> 元素内容与 style="…" 属性值——这两处都已经确定是 CSS 语义，
// 不需要也不应该再跑属性名匹配。
func rewriteCSSOnly(text, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) (string, error) {
	edits, err := css.ReferenceEdits(oldDocument, []byte(text), func(uri string) string {
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
