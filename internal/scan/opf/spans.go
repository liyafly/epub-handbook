// spans.go 提供 OPF/XML 的**字节区间**只读扫描（INV-2：只产出区间信息，
// 不产出整文档字节）。它是 metadata/cover/split 等 capability 做
// 字节区间编辑的定位层：解析树与 scripts/epub_lib.py 的 ElementTree
// 语义一致（命名空间、实体、EOL 归一），同时保留每个元素/属性在原文
// 中的字节 span。
package opf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/ianaindex"
)

// Span 是原文中的字节区间 [Start, End)。
type Span struct {
	Start int
	End   int
}

// IsZero 报告区间是否未被赋值。
func (s Span) IsZero() bool { return s.Start == 0 && s.End == 0 }

// Len 返回区间长度。
func (s Span) Len() int { return s.End - s.Start }

// SpanAttr 是元素上的一个属性（值为解码后文本）。
type SpanAttr struct {
	Name  xml.Name
	Value string
}

// SpanNode 是一个元素的区间投影。
//
//   - Open 覆盖整个开标签（含 < 与 >，自闭合含 "/>"）；
//   - Close 覆盖结束标签；自闭合或缺失时为零值；
//   - Text/TextSpan 是开标签到第一个子元素（或结束标签）之间的首段文本；
//   - Tail/TailSpan 是本元素结束后到下一个 '<' 之间的文本（对齐 ET 的 tail）。
type SpanNode struct {
	Name      xml.Name
	Open      Span
	Close     Span
	SelfClose bool
	Attrs     []SpanAttr
	Kids      []*SpanNode
	Parent    *SpanNode
	Text      string
	TextSpan  Span
	Tail      string
	TailSpan  Span
}

// AttrByLocal 返回命名空间与 local 名都精确匹配的属性值。
// spaceURI 为空串表示无命名空间属性（对齐 Python attrib.get("id")）。
func (n *SpanNode) AttrByLocal(spaceURI, local string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Space == spaceURI && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// ChildByLocal 返回第一个命名空间与 local 名都精确匹配的直接子元素。
func (n *SpanNode) ChildByLocal(spaceURI, local string) *SpanNode {
	for _, c := range n.Kids {
		if c.Name.Space == spaceURI && c.Name.Local == local {
			return c
		}
	}
	return nil
}

// ChildrenByLocal 按文档序返回全部命名空间与 local 名匹配的直接子元素。
func (n *SpanNode) ChildrenByLocal(spaceURI, local string) []*SpanNode {
	var out []*SpanNode
	for _, c := range n.Kids {
		if c.Name.Space == spaceURI && c.Name.Local == local {
			out = append(out, c)
		}
	}
	return out
}

// ChildByAnyNS 返回第一个 local 名匹配（忽略命名空间）的直接子元素，
// 对齐 Python 的 local_name(child.tag) == wanted 判定。
func (n *SpanNode) ChildByAnyNS(local string) *SpanNode {
	for _, c := range n.Kids {
		if c.Name.Local == local {
			return c
		}
	}
	return nil
}

// Walk 按前序（文档序）返回本元素及其全部后代。
func (n *SpanNode) Walk() []*SpanNode {
	var out []*SpanNode
	var visit func(e *SpanNode)
	visit = func(e *SpanNode) {
		out = append(out, e)
		for _, c := range e.Kids {
			visit(c)
		}
	}
	visit(n)
	return out
}

// IterText 返回子树的全部文本拼接（对齐 ET 的 "".join(elem.itertext())）。
func (n *SpanNode) IterText() string {
	var b strings.Builder
	var visit func(e *SpanNode)
	visit = func(e *SpanNode) {
		b.WriteString(e.Text)
		for _, c := range e.Kids {
			visit(c)
		}
		b.WriteString(e.Tail)
	}
	visit(n)
	return b.String()
}

// End 返回元素在原文中的结束位置（结束标签尾或自闭合开标签尾）。
func (n *SpanNode) End() int {
	if !n.Close.IsZero() {
		return n.Close.End
	}
	return n.Open.End
}

// TailAfter 计算本元素的 tail 区间：元素结束后到下一个 '<'（或 EOF）。
// Python 侧删除元素时 tail 一并消失，这里给出同样的字节范围。
func (n *SpanNode) TailAfter(data []byte) Span {
	start := n.End()
	i := bytes.IndexByte(data[start:], '<')
	if i < 0 {
		return Span{Start: start, End: len(data)}
	}
	return Span{Start: start, End: start + i}
}

// RawAttr 是开标签里一个属性的原文投影。
type RawAttr struct {
	RawName   string
	ValueSpan Span // 引号内的原文区间（无引号属性为零值）
	Quote     byte // 0 表示无引号
}

// RawAttrsIn 对 [open.Start, open.End) 的开标签做 quote 感知的原文扫描，
// 按文档序返回全部属性。返回条目数与 SpanNode.Attrs 一致（xmlns 声明
// 两侧都跳过），可按下标配对。
func RawAttrsIn(data []byte, open Span) []RawAttr {
	lo, hi := open.Start, open.End
	if lo < 0 || hi > len(data) || hi-lo < 3 {
		return nil
	}
	s := data[lo:hi]
	i := 1 // 跳过 '<'
	// 跳过元素名。
	for i < len(s) && !isASCIISpaceByte(s[i]) && s[i] != '>' && s[i] != '/' {
		i++
	}
	var out []RawAttr
	for i < len(s) {
		for i < len(s) && isASCIISpaceByte(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '>' {
			break
		}
		if s[i] == '/' { // "/>"
			break
		}
		// 属性名。
		nameStart := i
		for i < len(s) && !isASCIISpaceByte(s[i]) && s[i] != '=' && s[i] != '>' && s[i] != '/' {
			i++
		}
		rawName := string(s[nameStart:i])
		if rawName == "" {
			i++
			continue
		}
		j := i
		for j < len(s) && isASCIISpaceByte(s[j]) {
			j++
		}
		if j >= len(s) || s[j] != '=' {
			// 无值属性。
			out = append(out, RawAttr{RawName: rawName})
			i = j
			continue
		}
		j++
		for j < len(s) && isASCIISpaceByte(s[j]) {
			j++
		}
		if j >= len(s) || (s[j] != '"' && s[j] != '\'') {
			// 非法形态：记为无值属性，保持与解码侧一一对应。
			out = append(out, RawAttr{RawName: rawName})
			i = j
			continue
		}
		quote := s[j]
		vs := j + 1
		ve := bytes.IndexByte(s[vs:], quote)
		if ve < 0 {
			out = append(out, RawAttr{RawName: rawName})
			i = len(s)
			continue
		}
		ve += vs
		isXMLNS := rawName == "xmlns" || strings.HasPrefix(rawName, "xmlns:")
		if !isXMLNS {
			out = append(out, RawAttr{RawName: rawName, ValueSpan: Span{Start: lo + vs, End: lo + ve}, Quote: quote})
		}
		i = ve + 1
	}
	return out
}

// RawAttrValueSpan 返回第 index 个属性（与 SpanNode.Attrs 对齐）的原文值
// 区间与引号字符。
func RawAttrValueSpan(data []byte, n *SpanNode, index int) (Span, byte, bool) {
	if index < 0 || index >= len(n.Attrs) {
		return Span{}, 0, false
	}
	raws := RawAttrsIn(data, n.Open)
	if index >= len(raws) {
		return Span{}, 0, false
	}
	r := raws[index]
	if r.Quote == 0 {
		return Span{}, 0, false
	}
	return r.ValueSpan, r.Quote, true
}

// AttrIndex 返回命名空间与 local 名精确匹配的属性下标。
func (n *SpanNode) AttrIndex(spaceURI, local string) int {
	for i, a := range n.Attrs {
		if a.Name.Space == spaceURI && a.Name.Local == local {
			return i
		}
	}
	return -1
}

// ScanSpanTree 解析 XML 字节为区间树（根元素）。解析语义对齐
// ET.fromstring：注释 / PI / DOCTYPE 丢弃、实体解码、EOL 归一；
// 声明的非 UTF-8 编码先转换为 UTF-8。
func ScanSpanTree(data []byte) (*SpanNode, error) {
	src, err := xmlSourceToUTF8(data)
	if err != nil {
		return nil, err
	}
	d := xml.NewDecoder(strings.NewReader(src))
	d.Strict = true
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // 已按声明编码转换为 UTF-8
	}

	var root *SpanNode
	var stack []*SpanNode
	cur := func() *SpanNode {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}

	// 注释 / PI / 指令不中断文本累积（ET 的 text 合并语义）。
	appendText := func(text string, span Span) {
		if len(stack) == 0 {
			return // prolog / epilog 文本
		}
		top := cur()
		if len(top.Kids) == 0 {
			if top.TextSpan.IsZero() {
				top.TextSpan = span
			} else {
				top.TextSpan.End = span.End
			}
			top.Text += text
			return
		}
		last := top.Kids[len(top.Kids)-1]
		if last.TailSpan.IsZero() {
			last.TailSpan = span
		} else {
			last.TailSpan.End = span.End
		}
		last.Tail += text
	}

	for {
		prev := int(d.InputOffset())
		tok, err := d.Token()
		curOff := int(d.InputOffset())
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parse failed: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			attrs := make([]SpanAttr, 0, len(t.Attr))
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				attrs = append(attrs, SpanAttr{Name: a.Name, Value: a.Value})
			}
			node := &SpanNode{
				Name:  t.Name,
				Open:  Span{Start: prev, End: curOff},
				Attrs: attrs,
				Text:  "",
			}
			node.SelfClose = curOff-prev >= 2 && src[curOff-2] == '/' && src[curOff-1] == '>'
			if len(stack) > 0 {
				p := cur()
				node.Parent = p
				p.Kids = append(p.Kids, node)
			} else if root == nil {
				root = node
			} else {
				return nil, fmt.Errorf("XML parse failed: multiple root elements")
			}
			if node.SelfClose {
				// 不入栈；合成的空 EndElement 由下方 prev==cur 分支吞掉。
			} else {
				stack = append(stack, node)
			}
		case xml.EndElement:
			if prev == curOff {
				// 合成的自闭合收尾（空 span），栈里没有对应节点。
				continue
			}
			if len(stack) == 0 {
				return nil, fmt.Errorf("XML parse failed: unexpected end element")
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			top.Close = Span{Start: prev, End: curOff}
		case xml.CharData:
			appendText(string(t), Span{Start: prev, End: curOff})
		default:
			// Comment / ProcInst / Directive：跳过，文本累积不断开。
		}
	}
	if root == nil {
		return nil, fmt.Errorf("XML parse failed: no element found")
	}
	return root, nil
}

func isASCIISpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// xmlSourceToUTF8 按 BOM / XML 声明把 XML 字节转换为 UTF-8 文本
// （对齐 expat 的声明编码处理；未知编码报错）。
func xmlSourceToUTF8(data []byte) (string, error) {
	body := data
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		body = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return "", fmt.Errorf("XML parse failed: UTF-16 input is not supported here")
	}
	if m := xmlEncodingRe.FindSubmatch(body[:min(len(body), 256)]); m != nil {
		declared := string(m[1])
		switch strings.ToLower(declared) {
		case "utf-8", "utf8", "ascii", "us-ascii":
			// 直接按 UTF-8 解析。
		default:
			enc, err := ianaindex.IANA.Encoding(declared)
			if err != nil || enc == nil {
				if enc2, err2 := ianaindex.MIME.Encoding(declared); err2 == nil && enc2 != nil {
					enc = enc2
				} else {
					return "", fmt.Errorf("XML parse failed: unknown encoding: %s", declared)
				}
			}
			out, derr := enc.NewDecoder().Bytes(body)
			if derr != nil {
				return "", fmt.Errorf("XML parse failed: cannot decode document: %v", derr)
			}
			body = out
		}
	}
	return string(body), nil
}
