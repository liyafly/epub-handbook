// xhtml_blocks.go 以 encoding/xml 流式扫描复刻 _extract_xhtml_blocks
// （ElementTree DOM 的文档序投影），不做任何 DOM 重序列化（INV-2）。
package contentanalyze

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// textBlock 对齐 Python TextBlock dataclass。
type textBlock struct {
	source       string
	locator      string
	tag          string
	classes      []string
	ancestorTags []string
	epubTypes    []string
	language     *string
	text         string
	previousTag  *string
	nextTag      *string
}

// xaFrame 是流式扫描中一个打开的元素。
type xaFrame struct {
	tag        string // local 名，小写
	inBlock    bool
	blockChild bool // 直接子元素里出现过块级标签（pre/blockquote 跳过判据）
	langPlain  string
	langXML    string
	classes    string
	epubTypes  string
	idx        int // 同名（小写）兄弟中的序号，1 起
	buf        strings.Builder
	counts     map[string]int
	finalText  string     // 闭合时回填
	skipped    bool       // pre/blockquote 嵌套块跳过或无文本
	ancestors  []*xaFrame // 推入时的祖先链快照（供 pre-order 组装路径）
}

// AnalyzeXHTML 对齐 analyze_xhtml：解析 XHTML 并产出公开报告块。
func AnalyzeXHTML(source, content string, includeSnippets bool) ([]legacyBlock, error) {
	blocks, err := extractXHTMLBlocks(source, content)
	if err != nil {
		return nil, err
	}
	out := make([]legacyBlock, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, publicize(b, includeSnippets))
	}
	return out, nil
}

// extractXHTMLBlocks 复刻 _extract_xhtml_blocks。
func extractXHTMLBlocks(source, content string) ([]textBlock, error) {
	d := xml.NewDecoder(strings.NewReader(content))
	d.Strict = true // ElementTree 同为严格 XML：未定义实体/错配标签均报错
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // 输入已按 UTF-8 严格解码
	}

	var (
		stack  []*xaFrame
		raw    []textBlock
		brNest int // <br> 打开期间丢弃字符数据（ET 中 br 的子文本也会被 _element_text 丢弃）
		// order 按 pre-order 记录块级元素（对齐 Python 的文档序产出：
		// 外层 li 先于内层），文本在闭合时回填。
		order []*xaFrame
	)
	finishBlock := func(f *xaFrame) {
		// pre/blockquote 含直接块级子元素时整块跳过（_has_nested_block）。
		if (f.tag == "pre" || f.tag == "blockquote") && f.blockChild {
			f.skipped = true
			return
		}
		text := ""
		if f.tag != "hr" {
			text = pyTrimSpace(f.buf.String())
		}
		if text == "" && f.tag != "hr" {
			f.skipped = true
			return
		}
		f.finalText = text
	}

	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: XHTML/XML parse failed: %v", source, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			f := newFrame(t, stack)
			if f.tag == "br" {
				brNest++
				for _, fr := range stack {
					if fr.inBlock {
						fr.buf.WriteByte('\n')
					}
				}
				continue // br 不入栈：它对路径/上下文无贡献
			}
			stack = append(stack, f)
			if f.inBlock {
				f.ancestors = append([]*xaFrame(nil), stack[:len(stack)-1]...)
				order = append(order, f)
			}
		case xml.CharData:
			if brNest > 0 {
				continue
			}
			for _, fr := range stack {
				if fr.inBlock {
					fr.buf.Write(t)
				}
			}
		case xml.EndElement:
			tag := strings.ToLower(t.Name.Local)
			if tag == "br" {
				if brNest > 0 {
					brNest--
				}
				continue
			}
			if len(stack) > 0 && stack[len(stack)-1].tag == tag {
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if f.inBlock {
					finishBlock(f)
				}
			}
		}
	}
	for _, f := range order {
		if f.skipped {
			continue
		}
		frames := append(append([]*xaFrame(nil), f.ancestors...), f)
		raw = append(raw, buildBlock(source, frames, f.finalText))
	}
	return withNeighbors(raw), nil
}

// newFrame 对齐流式下每个开元素需要的投影：小写标签、语言/class/epub:type
// 属性、以及由父计数器推出的同名兄弟序号（_node_path 的索引）。
func newFrame(t xml.StartElement, stack []*xaFrame) *xaFrame {
	f := &xaFrame{tag: strings.ToLower(t.Name.Local)}
	for _, a := range t.Attr {
		switch {
		case a.Name.Space == "" && a.Name.Local == "class":
			f.classes = a.Value
		case a.Name.Space == "" && a.Name.Local == "lang":
			f.langPlain = a.Value
		case a.Name.Space == opf.XMLURI && a.Name.Local == "lang":
			f.langXML = a.Value
		case (a.Name.Space == opf.OPSURI && a.Name.Local == "type") ||
			(a.Name.Space == "" && a.Name.Local == "epub:type") ||
			(a.Name.Space == "epub" && a.Name.Local == "type"):
			// Python: attrib.get(EPUB_TYPE) or attrib.get("epub:type") or ""
			if f.epubTypes == "" {
				f.epubTypes = a.Value
			}
		}
	}
	if n := len(stack); n > 0 {
		parent := stack[n-1]
		if parent.counts == nil {
			parent.counts = map[string]int{}
		}
		parent.counts[f.tag]++
		f.idx = parent.counts[f.tag]
		if blockTags[f.tag] {
			parent.blockChild = true
		}
	} else {
		f.idx = 1
	}
	f.inBlock = blockTags[f.tag]
	return f
}

// buildBlock 组装一个块：路径、祖先标签、去重 class / epub:type、最近语言。
func buildBlock(source string, frames []*xaFrame, text string) textBlock {
	var path strings.Builder
	classes := []string{}
	seenClass := map[string]bool{}
	epubTypes := []string{}
	seenType := map[string]bool{}
	ancestors := make([]string, 0, len(frames))
	for i, fr := range frames {
		if i > 0 {
			path.WriteByte('/')
		}
		fmt.Fprintf(&path, "%s[%d]", fr.tag, fr.idx)
		ancestors = append(ancestors, fr.tag)
		for _, c := range splitPyFields(fr.classes) {
			if !seenClass[c] {
				seenClass[c] = true
				classes = append(classes, c)
			}
		}
		for _, e := range splitPyFields(fr.epubTypes) {
			if !seenType[e] {
				seenType[e] = true
				epubTypes = append(epubTypes, e)
			}
		}
	}
	// _nearest_language：从块本身向上找，每层先 lang 再 xml:lang。
	var language *string
	for i := len(frames) - 1; i >= 0 && language == nil; i-- {
		if v := frames[i].langPlain; v != "" {
			language = &v
		} else if v := frames[i].langXML; v != "" {
			language = &v
		}
	}
	return textBlock{
		source:       source,
		locator:      "/" + strings.TrimPrefix(path.String(), "/"),
		tag:          frames[len(frames)-1].tag,
		classes:      classes,
		ancestorTags: ancestors,
		epubTypes:    epubTypes,
		language:     language,
		text:         text,
	}
}

// withNeighbors 对齐 _with_neighbors：填前驱/后继标签。
func withNeighbors(blocks []textBlock) []textBlock {
	out := make([]textBlock, 0, len(blocks))
	for i := range blocks {
		b := blocks[i]
		if i > 0 {
			pt := blocks[i-1].tag
			b.previousTag = &pt
		}
		if i+1 < len(blocks) {
			nt := blocks[i+1].tag
			b.nextTag = &nt
		}
		out = append(out, b)
	}
	return out
}
