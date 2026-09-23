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
	refs, err := collectMarkupReferences(data)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ref := range refs {
		out = append(out, ref.uri)
	}
	return out, nil
}

func collectMarkupReferences(data []byte) ([]resourceReference, error) {
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return nil, err
	}
	var out []resourceReference
	for _, node := range root.Walk() {
		for _, attr := range node.Attrs {
			if attr.Name.Space == "" && attr.Name.Local == "style" {
				uris, cerr := collectCSSReferences(attr.Value)
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
					out = append(out, resourceReference{uri: candidate.url})
				}
				continue
			}
			out = append(out, resourceReference{uri: attr.Value})
		}
		if node.Name.Local == "style" {
			uris, cerr := collectCSSReferences(node.IterText())
			if cerr != nil {
				return nil, fmt.Errorf("style element: %w", cerr)
			}
			out = append(out, uris...)
		}
	}
	return out, nil
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
