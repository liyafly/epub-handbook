// svg.go 复刻 core.resize_svg_cover_pages / set_xml_tag_attribute /
// uri_targets_archive 及其正则（SVG_BLOCK_RE / SVG_IMAGE_RE）。
package cover

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// resizeSVGCoverPages 复刻 core.resize_svg_cover_pages：把内联 SVG 封面
// 包裹的 <image> 与 viewBox 对齐到替换栅格的尺寸。
//
// 与 refs.go 的引用重写同样是区域感知的：`<svg` / `</svg` / `<image` 只在
// xhtml.ScanRegions 认定的真实标签区间上成立。裸 strings.Index 扫描虽然要求
// 字面尖括号（转义正文 `&lt;svg&gt;` 命中不了），但注释、CDATA 与 <script>
// 里未转义的示例 SVG 片段仍会被改写 —— 那是作者正文。
// warnf 可以为 nil（老调用点与测试）；区域扫描截断时上报文件名与偏移。
func resizeSVGCoverPages(data []byte, documentPath, coverPath string, width, height int, warnf func(string, ...any)) []byte {
	if !markupExtensions[strings.ToLower(pathExt(documentPath))] {
		return data
	}
	if !utf8.Valid(data) {
		return data
	}
	text := string(data)
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete && warnf != nil {
		warnf("%s: markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); inline SVG cover pages after this offset left unresized", documentPath, stop)
	}
	// 真实标签的 start→end 表。既用来判定候选是否真的是标签，也直接给出
	// 标签结束位置 —— 比原来找第一个 '>' 更准（属性值里的 '>' 不再截断标签）。
	tags := make(map[int]int, len(regions))
	for _, r := range regions {
		if r.Kind == xhtml.RegionTag {
			tags[r.Start] = r.End
		}
	}
	lower := asciiLower(text)
	return []byte(rewriteSVGBlocks(text, lower, documentPath, coverPath, width, height, tags))
}

// rewriteSVGBlocks 实现 SVG_BLOCK_RE.sub(replace_svg)。
func rewriteSVGBlocks(text, lower, documentPath, coverPath string, width, height int, tags map[int]int) string {
	var out strings.Builder
	pos := 0
	for {
		openStart, openEnd, ok := findSVGOpenTag(text, lower, pos, tags)
		if !ok {
			break
		}
		closeStart, closeEnd, found := findSVGCloseTag(text, lower, openEnd, tags)
		if !found {
			// 整体匹配失败，从下一个字节继续找 <svg。
			pos = openStart + 1
			continue
		}
		body := text[openEnd:closeStart]
		newBody, hasCoverImage := rewriteSVGImages(body, lower[openEnd:closeStart], documentPath, coverPath, width, height, tags, openEnd)
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

// findSVGOpenTag 实现 `<svg\b[^>]*>`（大小写不敏感），且只接受落在真实
// 标签区间上的候选（tags 为 nil 时退回纯字面扫描，供不关心区域的调用方）。
func findSVGOpenTag(text, lower string, from int, tags map[int]int) (int, int, bool) {
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
		if tags != nil {
			end, isTag := tags[i]
			if !isTag {
				i++ // 注释 / CDATA / script 里的字面 <svg，不是标记
				continue
			}
			return i, end, true
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
func findSVGCloseTag(text, lower string, from int, tags map[int]int) (int, int, bool) {
	i := from
	for i+5 <= len(text) {
		j := strings.Index(lower[i:], "</svg")
		if j < 0 {
			return 0, 0, false
		}
		i += j
		if tags != nil {
			if end, isTag := tags[i]; isTag {
				return i, end, true
			}
			i++ // 非标记位置的字面 "</svg"
			continue
		}
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
func rewriteSVGImages(body, lowerBody, documentPath, coverPath string, width, height int, tags map[int]int, base int) (string, bool) {
	var out strings.Builder
	pos := 0
	hasCoverImage := false
	for {
		start, end, ok := findImageTag(body, lowerBody, pos, tags, base)
		if !ok {
			break
		}
		tag := body[start:end]
		replaced := ""
		attrs, ok := xhtml.TagAttrs(tag)
		for _, attr := range attrs {
			if !ok || attr.Quote == 0 || !hasAttrName(uriAttrNames, attr.Name) {
				continue
			}
			uri := tag[attr.ValueSpan.Start:attr.ValueSpan.End]
			if uriTargetsArchive(uri, documentPath, coverPath) {
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

// findImageTag 实现 `<image\b[^>]*>`（大小写不敏感）。body 是 <svg> 元素的
// 内容，base 是它在整份文档里的起始偏移 —— tags 的键是文档绝对坐标，比对前
// 必须加回 base。tags 为 nil 时退回纯字面扫描。
func findImageTag(body, lower string, from int, tags map[int]int, base int) (int, int, bool) {
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
		if tags != nil {
			end, isTag := tags[base+i]
			if !isTag {
				i++ // 注释 / CDATA / script 里的字面 <image，不是标记
				continue
			}
			return i, end - base, true
		}
		end := strings.IndexByte(body[i:], '>')
		if end < 0 {
			return 0, 0, false
		}
		return i, i + end + 1, true
	}
	return 0, 0, false
}

func asciiLower(s string) string {
	lower := []byte(s)
	for i, b := range lower {
		if b >= 'A' && b <= 'Z' {
			lower[i] = b + ('a' - 'A')
		}
	}
	return string(lower)
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
	attrs, ok := xhtml.TagAttrs(tag)
	if !ok {
		return 0, 0, 0, false
	}
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, name) && attr.Quote != 0 {
			return attr.ValueSpan.End, attr.ValueSpan.Start, attr.Quote, true
		}
	}
	return 0, 0, 0, false
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
