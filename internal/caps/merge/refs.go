// Reference rewriting preserves markup regions and uses lossless CSS token spans.
package merge

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

// rewriteURI 复刻 core.rewrite_uri（静默失败：解析失败或目标未知时原样返回）。
func rewriteURI(uri, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) string {
	if uri == "" || strings.HasPrefix(uri, "#") || pypath.IsExternalURI(uri) {
		return uri
	}
	parts := pypath.URLSplit(uri)
	if parts.Path == "" {
		return uri
	}
	oldTarget, err := pypath.ResolveRelativePath(oldDocument, parts.Path)
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
	newPath := pypath.RelativeURI(newDocument, target)
	return pypath.URLUnsplitPath(newPath, parts.Query, parts.Fragment)
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
				return "", fmt.Errorf("%s: character references in embedded CSS require explicit review", oldDocument)
			}
			segment, err = rewriteCSSOnly(segment, oldDocument, newDocument, pathMap, knownFiles)
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
	tag = subNameQuoteURI(tag, uriAttrNames, func(prefix, quote, uri string) string {
		return prefix + quote + rewriteURI(uri, oldDocument, newDocument, pathMap, knownFiles) + quote
	})
	return rewriteInlineStyleReferences(tag, oldDocument, newDocument, pathMap, knownFiles)
}

// rewriteInlineStyleReferences 只在 style="…" 属性值内部做 CSS url()/@import
// 重写。整段标签跑 CSS 重写会连 title=""、alt="" 这类读者可见文本一起改
// （`<div title="url(a.png)">`），那是正文损坏；而内联 style 的 url() 是真
// 标记，资源搬家后必须跟着改，不能整体放弃。
func rewriteInlineStyleReferences(tag, oldDocument, newDocument string, pathMap map[string]string, knownFiles map[string]bool) (string, error) {
	var scanErr error
	updated := subNameQuoteURI(tag, []string{"style"}, func(prefix, quote, value string) string {
		if strings.Contains(value, "&") {
			scanErr = fmt.Errorf("%s: character references in inline CSS require explicit review", oldDocument)
			return prefix + quote + value + quote
		}
		result, err := rewriteCSSOnly(value, oldDocument, newDocument, pathMap, knownFiles)
		if err != nil {
			scanErr = err
			return prefix + quote + value + quote
		}
		return prefix + quote + result + quote
	})
	if scanErr != nil {
		return "", scanErr
	}
	return updated, nil
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

// transformResource 复刻 core.transform_resource：仅 CSS / 标记类参与重写，
// 非 UTF-8 字节原样返回。独立 .css 文件按 CSS token/span 扫描；markupExtensions（XHTML/NCX/OPF 同族标记文件）改用区域感知重写，
// 避免字符数据里的转义示例文本被当成标记误改（见上方注释）。
func transformResource(data []byte, oldPath, newPath string, pathMap map[string]string, knownFiles map[string]bool, warn func(format string, a ...any)) ([]byte, error) {
	ext := strings.ToLower(pypath.PathExt(oldPath))
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

// ---- 通用扫描器（对齐 Python re 语义） ----

type uriMatch struct {
	start, end int
	prefix     string
	quote      byte // 0 表示无引号
	uri        string
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
