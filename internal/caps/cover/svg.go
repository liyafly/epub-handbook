// svg.go 复刻 core.resize_svg_cover_pages / set_xml_tag_attribute /
// uri_targets_archive 及其正则（SVG_BLOCK_RE / SVG_IMAGE_RE）。
package cover

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// resizeSVGCoverPages 复刻 core.resize_svg_cover_pages：把内联 SVG 封面
// 包裹的 <image> 与 viewBox 对齐到替换栅格的尺寸。
func resizeSVGCoverPages(data []byte, documentPath, coverPath string, width, height int) []byte {
	if !markupExtensions[strings.ToLower(pathExt(documentPath))] {
		return data
	}
	if !utf8.Valid(data) {
		return data
	}
	text := string(data)
	return []byte(rewriteSVGBlocks(text, documentPath, coverPath, width, height))
}

// rewriteSVGBlocks 实现 SVG_BLOCK_RE.sub(replace_svg)。
func rewriteSVGBlocks(text, documentPath, coverPath string, width, height int) string {
	var out strings.Builder
	pos := 0
	for {
		openStart, openEnd, ok := findSVGOpenTag(text, pos)
		if !ok {
			break
		}
		closeStart, closeEnd, found := findSVGCloseTag(text, openEnd)
		if !found {
			// 整体匹配失败，从下一个字节继续找 <svg。
			pos = openStart + 1
			continue
		}
		body := text[openEnd:closeStart]
		newBody, hasCoverImage := rewriteSVGImages(body, documentPath, coverPath, width, height)
		if !hasCoverImage {
			out.WriteString(text[pos:closeEnd])
			pos = closeEnd
			continue
		}
		opening := setXMLTagAttribute(text[openStart:openEnd], "viewBox", fmt.Sprintf("0 0 %d %d", width, height))
		out.WriteString(text[pos:openStart])
		out.WriteString(opening)
		out.WriteString(newBody)
		out.WriteString(text[closeStart:closeEnd])
		pos = closeEnd
	}
	out.WriteString(text[pos:])
	return out.String()
}

// findSVGOpenTag 实现 `<svg\b[^>]*>`（大小写不敏感）。
func findSVGOpenTag(text string, from int) (int, int, bool) {
	lower := strings.ToLower(text)
	for i := from; i+4 <= len(text); {
		j := strings.Index(lower[i:], "<svg")
		if j < 0 {
			return 0, 0, false
		}
		i += j
		after := i + 4
		if after < len(text) && isWordRune(rune(text[after])) {
			i++ // \b 不成立
			continue
		}
		end := strings.IndexByte(text[i:], '>')
		if end < 0 {
			return 0, 0, false
		}
		return i, i + end + 1, true
	}
	return 0, 0, false
}

// findSVGCloseTag 实现 `</svg\s*>`（大小写不敏感）；正则的 .*? 允许
// 跳过不成立的 "</svg" 候选继续向后找。
func findSVGCloseTag(text string, from int) (int, int, bool) {
	lower := strings.ToLower(text)
	i := from
	for i+5 <= len(text) {
		j := strings.Index(lower[i:], "</svg")
		if j < 0 {
			return 0, 0, false
		}
		i += j
		k := i + 5
		for k < len(text) && isASCIISpace(text[k]) {
			k++
		}
		if k < len(text) && text[k] == '>' {
			return i, k + 1, true
		}
		i++ // 该候选不成立，尝试下一个 "</svg"
	}
	return 0, 0, false
}

func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

// rewriteSVGImages 实现 SVG_IMAGE_RE.sub(replace_image)。
func rewriteSVGImages(body, documentPath, coverPath string, width, height int) (string, bool) {
	var out strings.Builder
	pos := 0
	hasCoverImage := false
	for {
		start, end, ok := findImageTag(body, pos)
		if !ok {
			break
		}
		tag := body[start:end]
		replaced := ""
		for _, m := range findNameQuoteMatches(tag, 0, uriAttrNames) {
			if uriTargetsArchive(m.uri, documentPath, coverPath) {
				hasCoverImage = true
				replaced = setXMLTagAttribute(setXMLTagAttribute(tag, "width", itoa(width)), "height", itoa(height))
				break
			}
		}
		out.WriteString(body[pos:start])
		if replaced != "" {
			out.WriteString(replaced)
		} else {
			out.WriteString(tag)
		}
		pos = end
	}
	out.WriteString(body[pos:])
	return out.String(), hasCoverImage
}

// findImageTag 实现 `<image\b[^>]*>`（大小写不敏感）。
func findImageTag(body string, from int) (int, int, bool) {
	lower := strings.ToLower(body)
	for i := from; i+6 <= len(body); {
		j := strings.Index(lower[i:], "<image")
		if j < 0 {
			return 0, 0, false
		}
		i += j
		after := i + 6
		if after < len(body) && isWordRune(rune(body[after])) {
			i++
			continue
		}
		end := strings.IndexByte(body[i:], '>')
		if end < 0 {
			return 0, 0, false
		}
		return i, i + end + 1, true
	}
	return 0, 0, false
}

// uriTargetsArchive 复刻 core.uri_targets_archive。
func uriTargetsArchive(uri, documentPath, targetPath string) bool {
	if uri == "" || pyIsExternalURI(uri) {
		return false
	}
	path := pyURLSplit(uri).path
	if path == "" {
		return false
	}
	resolved, err := resolveRelativePath(documentPath, path)
	if err != nil {
		return false
	}
	return resolved == targetPath
}

// setXMLTagAttribute 复刻 core.set_xml_tag_attribute：
// 先按 `(\s{name}\s*=\s*)(["']).*?\2`（IGNORECASE|DOTALL）替换首个命中值；
// 不存在则在收尾（">" 或 "/>"）前插入 ` {name}="{value}"`。值不经转义
// （与 Python f-string 一致，调用方只传数字）。
func setXMLTagAttribute(tag, name, value string) string {
	if idx, preEnd, quote, ok := findTagAttr(tag, name); ok {
		return tag[:preEnd] + value + string(quote) + tag[idx+1:]
	}
	closing := ">"
	trimmed := strings.TrimRight(tag, " \t\r\n\f\v")
	if strings.HasSuffix(trimmed, "/>") {
		closing = "/>"
	}
	return tag[:len(tag)-len(closing)] + ` ` + name + `="` + value + `"` + tag[len(tag)-len(closing):]
}

// findTagAttr 在 tag 里找 `\s{name}\s*=\s*(["'])(.*?)\1` 的首个命中，
// 返回（值内容结束位置=闭引号下标, 值起始, 引号）。
func findTagAttr(tag, name string) (int, int, byte, bool) {
	lowerName := strings.ToLower(name)
	for i := 0; i < len(tag); {
		if tag[i] != ' ' && tag[i] != '\t' && tag[i] != '\n' && tag[i] != '\r' && tag[i] != '\f' && tag[i] != '\v' {
			_, size := utf8.DecodeRuneInString(tag[i:])
			if size == 0 {
				break
			}
			i += size
			continue
		}
		wsStart := i
		j := skipASCIISpace(tag, i)
		if j+len(lowerName) <= len(tag) && strings.EqualFold(tag[j:j+len(lowerName)], lowerName) {
			k := skipASCIISpace(tag, j+len(lowerName))
			if k < len(tag) && tag[k] == '=' {
				k = skipASCIISpace(tag, k+1)
				if k < len(tag) && (tag[k] == '"' || tag[k] == '\'') {
					quote := tag[k]
					vs := k + 1
					vEnd := strings.IndexByte(tag[vs:], quote)
					if vEnd < 0 {
						return 0, 0, 0, false
					}
					return vs + vEnd, vs, quote, true
				}
			}
		}
		_ = wsStart
		_, size := utf8.DecodeRuneInString(tag[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return 0, 0, 0, false
}

func skipASCIISpace(s string, i int) int {
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			i++
		default:
			return i
		}
	}
	return i
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
